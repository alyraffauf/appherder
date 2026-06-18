package appherder

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/alyraffauf/goxdgdesktop/desktopfile"
)

// appimagetool stores an AppImage's optional OpenPGP signature in two ELF
// sections: .sha256_sig (armored detached signature) and .sig_key (armored
// public key). The signed message is the lowercase hex SHA-256 of the whole
// file with both sections zeroed.
const (
	sigSection = ".sha256_sig"
	keySection = ".sig_key"

	// desktopSigningKey pins an app's trusted signing-key fingerprint in its
	// managed launcher, recorded on the first signed install (trust on first use).
	desktopSigningKey = "X-AppHerder-SigningKey"
)

// errUnsigned reports that an AppImage carries no usable embedded signature.
var errUnsigned = errors.New("AppImage is not signed")

// byteRange is a half-open [start, start+length) span of a file.
type byteRange struct{ start, length int64 }

// sectionData returns the named ELF section's contents with NUL padding trimmed,
// and the byte range it occupies. ok is false when the section is absent or
// holds no file bytes.
func sectionData(f *elf.File, name string) (data []byte, span byteRange, ok bool, err error) {
	section := f.Section(name)
	if section == nil || section.Type == elf.SHT_NOBITS {
		return nil, byteRange{}, false, nil
	}
	data, err = section.Data()
	if err != nil {
		return nil, byteRange{}, false, fmt.Errorf("read %s: %w", name, err)
	}
	return bytes.TrimRight(data, "\x00"), byteRange{int64(section.Offset), int64(section.Size)}, true, nil
}

// readSignatureSections returns the .sha256_sig and .sig_key contents and the
// byte ranges they occupy, which the digest zeroes.
func readSignatureSections(file string) (sig, key []byte, zero []byteRange, err error) {
	elfFile, err := elf.Open(file)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open AppImage %s: %w", file, err)
	}
	defer elfFile.Close()

	sig, sigSpan, sigOK, err := sectionData(elfFile, sigSection)
	if err != nil {
		return nil, nil, nil, err
	}
	key, keySpan, keyOK, err := sectionData(elfFile, keySection)
	if err != nil {
		return nil, nil, nil, err
	}
	if sigOK {
		zero = append(zero, sigSpan)
	}
	if keyOK {
		zero = append(zero, keySpan)
	}
	return sig, key, zero, nil
}

// signedDigest returns the lowercase hex SHA-256 that appimagetool signs: the
// file hashed with the bytes in zero replaced by NULs.
func signedDigest(file string, zero []byteRange) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", fmt.Errorf("open AppImage %s: %w", file, err)
	}
	defer f.Close()

	hasher := sha256.New()
	buf := make([]byte, 64*1024)
	var pos int64
	for {
		n, err := f.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			zeroOverlaps(chunk, pos, zero)
			hasher.Write(chunk)
			pos += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read AppImage %s: %w", file, err)
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// zeroOverlaps NULs the bytes of chunk (which begins at file offset pos) that
// fall within any range in zero.
func zeroOverlaps(chunk []byte, pos int64, zero []byteRange) {
	end := pos + int64(len(chunk))
	for _, r := range zero {
		for i := max(pos, r.start); i < min(end, r.start+r.length); i++ {
			chunk[i-pos] = 0
		}
	}
}

// verifyAppImageSignature verifies an AppImage's embedded signature and returns
// the signer's fingerprint (uppercase hex). It returns errUnsigned when no
// signature is present, or an error when one is present but invalid.
func verifyAppImageSignature(file string) (fingerprint string, err error) {
	sig, key, zero, err := readSignatureSections(file)
	if err != nil {
		return "", err
	}
	if len(bytes.TrimSpace(sig)) == 0 {
		return "", errUnsigned
	}
	if len(bytes.TrimSpace(key)) == 0 {
		return "", errors.New("AppImage is signed but carries no public key")
	}

	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(key))
	if err != nil {
		return "", fmt.Errorf("read embedded signing key: %w", err)
	}
	digest, err := signedDigest(file, zero)
	if err != nil {
		return "", err
	}
	signer, err := openpgp.CheckArmoredDetachedSignature(keyring, strings.NewReader(digest), bytes.NewReader(sig), nil)
	if err != nil {
		return "", fmt.Errorf("invalid AppImage signature: %w", err)
	}
	return fmt.Sprintf("%X", signer.PrimaryKey.Fingerprint), nil
}

// pinnedSigningKey returns the fingerprint appherder pinned for appName, or "".
func (a App) pinnedSigningKey(appName string) string {
	desktop, err := desktopfile.Read(filepath.Join(a.applicationsDir, appName+".desktop"))
	if err != nil {
		return ""
	}
	fingerprint, _ := desktop.Get(desktopEntrySection, desktopSigningKey)
	return fingerprint
}

// checkSignature applies the trust policy to an incoming AppImage given the
// fingerprint pinned for the app ("" if none), returning the fingerprint to keep
// going forward. An app with no pin accepts anything and pins the first valid
// signature (trust on first use). Once pinned, every update must be signed by
// the pinned key: an unsigned, invalid, or differently-keyed AppImage is
// refused. Changing trust is a deliberate act — uninstall, then reinstall.
func checkSignature(file, pinned string) (fingerprint string, err error) {
	fpr, err := verifyAppImageSignature(file)
	switch {
	case errors.Is(err, errUnsigned):
		if pinned != "" {
			return "", fmt.Errorf("refusing unsigned AppImage: a signing key is pinned (%s); uninstall and reinstall to trust a different build", pinned)
		}
		return "", nil
	case err != nil:
		return "", err
	}
	if pinned != "" && !strings.EqualFold(pinned, fpr) {
		return "", fmt.Errorf("refusing AppImage: signing key changed (pinned %s, got %s); uninstall and reinstall to trust the new key", pinned, fpr)
	}
	return fpr, nil
}
