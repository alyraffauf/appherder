package appherder

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
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

// byteRange is a half-open [start, start+length) span of a file.
type byteRange struct{ start, length int64 }

// expectedChecksum is a source-advertised hash to verify a download against. The
// zero value means the source provided none.
type expectedChecksum struct {
	hex    string
	hasher hash.Hash
}

func (c expectedChecksum) set() bool { return c.hex != "" }

// matches reports whether the bytes fed to c.hasher hash to the advertised value.
func (c expectedChecksum) matches() bool {
	return strings.EqualFold(hex.EncodeToString(c.hasher.Sum(nil)), c.hex)
}

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
// byte ranges they occupy, which the signing digest zeroes.
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

// hashFile reads file once, writing each chunk to every non-nil hasher: raw
// receives the unmodified bytes (for a checksum), signed receives them with the
// zero ranges NUL'd (for appimagetool's signing digest).
func hashFile(file string, zero []byteRange, raw, signed hash.Hash) error {
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("open AppImage %s: %w", file, err)
	}
	defer f.Close()

	buf := make([]byte, 64*1024)
	var pos int64
	for {
		n, err := f.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if raw != nil {
				raw.Write(chunk)
			}
			if signed != nil {
				zeroOverlaps(chunk, pos, zero)
				signed.Write(chunk)
			}
			pos += int64(n)
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read AppImage %s: %w", file, err)
		}
	}
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

// verifyAppImage checks a freshly obtained AppImage before install, in a single
// read pass: download integrity against want (the source's advertised checksum,
// when present) and authenticity against the key pinned for the app. It returns
// the fingerprint to record going forward.
//
// Trust policy: with no pin, the first valid signature is pinned (trust on first
// use). Once pinned, every update must be signed by that key; an unsigned,
// invalid, or re-keyed AppImage is refused, and changing trust means uninstall
// then reinstall.
func verifyAppImage(file, pinned string, want expectedChecksum) (fingerprint string, err error) {
	sig, key, zero, err := readSignatureSections(file)
	if err != nil {
		return "", err
	}
	signed := len(bytes.TrimSpace(sig)) > 0

	// Single read pass: hash only what we check. rawHash takes the unmodified
	// bytes for the checksum, signHash the section-zeroed bytes for the digest.
	var rawHash, signHash hash.Hash
	if want.set() {
		rawHash = want.hasher
	}
	if signed {
		signHash = sha256.New()
	}
	if rawHash != nil || signHash != nil {
		if err := hashFile(file, zero, rawHash, signHash); err != nil {
			return "", err
		}
	}

	if want.set() && !want.matches() {
		return "", errors.New("downloaded AppImage failed checksum verification")
	}

	if !signed {
		if pinned != "" {
			return "", fmt.Errorf("refusing unsigned AppImage: a signing key is pinned (%s); uninstall and reinstall to trust a different build", pinned)
		}
		return "", nil
	}
	if len(bytes.TrimSpace(key)) == 0 {
		return "", errors.New("AppImage is signed but carries no public key")
	}
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(key))
	if err != nil {
		return "", fmt.Errorf("read embedded signing key: %w", err)
	}
	digest := hex.EncodeToString(signHash.Sum(nil))
	signer, err := openpgp.CheckArmoredDetachedSignature(keyring, strings.NewReader(digest), bytes.NewReader(sig), nil)
	if err != nil {
		return "", fmt.Errorf("invalid AppImage signature: %w", err)
	}
	fingerprint = fmt.Sprintf("%X", signer.PrimaryKey.Fingerprint)
	if pinned != "" && !strings.EqualFold(pinned, fingerprint) {
		return "", fmt.Errorf("refusing AppImage: signing key changed (pinned %s, got %s); uninstall and reinstall to trust the new key", pinned, fingerprint)
	}
	return fingerprint, nil
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
