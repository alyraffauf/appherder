package appherder

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
)

// elfLayout is a minimal ELF carrying empty .sha256_sig and .sig_key sections,
// ready to have a signature embedded.
type elfLayout struct {
	bytes          []byte
	sigOff, sigCap int64
	keyOff, keyCap int64
	textOff        int64 // a content byte safe to tamper with
}

// buildSignableELF lays out a 64-bit little-endian ELF with zero-filled
// signature sections, mirroring what appimagetool reserves before signing.
func buildSignableELF(sigCap, keyCap int) elfLayout {
	le := binary.LittleEndian
	const hdr, shentsize = 64, 64
	textData := bytes.Repeat([]byte{0xAA}, 16)

	names := []string{"", ".text", ".sha256_sig", ".sig_key", ".shstrtab"}
	var shstr bytes.Buffer
	nameOff := map[string]uint32{}
	for _, name := range names {
		nameOff[name] = uint32(shstr.Len())
		shstr.WriteString(name)
		shstr.WriteByte(0)
	}

	off := int64(hdr)
	textOff := off
	off += int64(len(textData))
	sigOff := off
	off += int64(sigCap)
	keyOff := off
	off += int64(keyCap)
	shstrOff := off
	off += int64(shstr.Len())
	if rem := off % 8; rem != 0 {
		off += 8 - rem
	}
	shoff := off

	sections := []struct {
		name      string
		typ       uint32
		off, size int64
	}{
		{"", 0, 0, 0},
		{".text", 1, textOff, int64(len(textData))},    // SHT_PROGBITS
		{".sha256_sig", 1, sigOff, int64(sigCap)},      // SHT_PROGBITS
		{".sig_key", 1, keyOff, int64(keyCap)},         // SHT_PROGBITS
		{".shstrtab", 3, shstrOff, int64(shstr.Len())}, // SHT_STRTAB
	}

	buf := make([]byte, shoff+int64(len(sections))*shentsize)
	copy(buf, []byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0})
	le.PutUint16(buf[16:], 2)    // ET_EXEC
	le.PutUint16(buf[18:], 0x3e) // x86-64
	le.PutUint32(buf[20:], 1)    // version
	le.PutUint64(buf[40:], uint64(shoff))
	le.PutUint16(buf[52:], hdr)
	le.PutUint16(buf[58:], shentsize)
	le.PutUint16(buf[60:], uint16(len(sections)))
	le.PutUint16(buf[62:], 4) // shstrndx -> .shstrtab

	copy(buf[textOff:], textData)
	copy(buf[shstrOff:], shstr.Bytes())
	for i, s := range sections {
		base := shoff + int64(i)*shentsize
		le.PutUint32(buf[base:], nameOff[s.name])
		le.PutUint32(buf[base+4:], s.typ)
		le.PutUint64(buf[base+24:], uint64(s.off))
		le.PutUint64(buf[base+32:], uint64(s.size))
	}
	return elfLayout{buf, sigOff, int64(sigCap), keyOff, int64(keyCap), textOff}
}

// embed writes sig and key into the reserved sections (zero-padded).
func (l elfLayout) embed(sig, key []byte) []byte {
	out := append([]byte(nil), l.bytes...)
	copy(out[l.sigOff:l.sigOff+l.sigCap], sig)
	copy(out[l.keyOff:l.keyOff+l.keyCap], key)
	return out
}

// signWith signs the layout's signed digest with entity and returns the armored
// detached signature plus the armored public key.
func signWith(t *testing.T, l elfLayout, entity *openpgp.Entity) (sig, key []byte) {
	t.Helper()
	sum := sha256.Sum256(l.bytes) // sections are still zero-filled
	digest := hex.EncodeToString(sum[:])

	var sigBuf bytes.Buffer
	if err := openpgp.ArmoredDetachSign(&sigBuf, entity, strings.NewReader(digest), nil); err != nil {
		t.Fatalf("sign: %v", err)
	}
	var keyBuf bytes.Buffer
	w, err := armor.Encode(&keyBuf, openpgp.PublicKeyType, nil)
	if err != nil {
		t.Fatalf("armor key: %v", err)
	}
	if err := entity.Serialize(w); err != nil {
		t.Fatalf("serialize key: %v", err)
	}
	w.Close()
	return sigBuf.Bytes(), keyBuf.Bytes()
}

func newTestEntity(t *testing.T) *openpgp.Entity {
	t.Helper()
	entity, err := openpgp.NewEntity("AppHerder Test", "", "test@appherder.local", nil)
	if err != nil {
		t.Fatalf("new entity: %v", err)
	}
	return entity
}

func writeAppImage(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "App.AppImage")
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestVerifyAppImageSignatureValid(t *testing.T) {
	entity := newTestEntity(t)
	layout := buildSignableELF(2048, 4096)
	sig, key := signWith(t, layout, entity)
	path := writeAppImage(t, layout.embed(sig, key))

	got, err := verifyAppImageSignature(path)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	want := strings.ToUpper(hex.EncodeToString(entity.PrimaryKey.Fingerprint))
	if got != want {
		t.Fatalf("fingerprint = %s, want %s", got, want)
	}
}

func TestVerifyAppImageSignatureUnsigned(t *testing.T) {
	// Sections present but empty.
	path := writeAppImage(t, buildSignableELF(2048, 4096).bytes)
	if _, err := verifyAppImageSignature(path); !errors.Is(err, errUnsigned) {
		t.Fatalf("err = %v, want errUnsigned", err)
	}
}

func TestVerifyAppImageSignatureTampered(t *testing.T) {
	entity := newTestEntity(t)
	layout := buildSignableELF(2048, 4096)
	sig, key := signWith(t, layout, entity)
	data := layout.embed(sig, key)
	data[layout.textOff] ^= 0xFF // flip a byte outside the signature sections

	path := writeAppImage(t, data)
	if _, err := verifyAppImageSignature(path); err == nil || errors.Is(err, errUnsigned) {
		t.Fatalf("err = %v, want invalid-signature error", err)
	}
}

func TestVerifyAppImageSignatureMissingKey(t *testing.T) {
	entity := newTestEntity(t)
	layout := buildSignableELF(2048, 4096)
	sig, _ := signWith(t, layout, entity)
	path := writeAppImage(t, layout.embed(sig, nil)) // signature but no key

	if _, err := verifyAppImageSignature(path); err == nil || errors.Is(err, errUnsigned) {
		t.Fatalf("err = %v, want missing-key error", err)
	}
}

func TestCheckSignaturePolicy(t *testing.T) {
	entity := newTestEntity(t)
	layout := buildSignableELF(2048, 4096)
	sig, key := signWith(t, layout, entity)
	fpr := strings.ToUpper(hex.EncodeToString(entity.PrimaryKey.Fingerprint))

	signed := writeAppImage(t, layout.embed(sig, key))
	unsigned := writeAppImage(t, buildSignableELF(2048, 4096).bytes)

	t.Run("signed with no pin establishes trust", func(t *testing.T) {
		got, err := checkSignature(signed, "")
		if err != nil || got != fpr {
			t.Fatalf("got (%q, %v), want (%q, nil)", got, err, fpr)
		}
	})
	t.Run("signed matching pin is accepted", func(t *testing.T) {
		got, err := checkSignature(signed, fpr)
		if err != nil || got != fpr {
			t.Fatalf("got (%q, %v), want (%q, nil)", got, err, fpr)
		}
	})
	t.Run("signed with different pin is refused", func(t *testing.T) {
		if _, err := checkSignature(signed, "DEADBEEF"); err == nil {
			t.Fatal("expected refusal on key change")
		}
	})
	t.Run("unsigned is refused once a key is pinned", func(t *testing.T) {
		if _, err := checkSignature(unsigned, fpr); err == nil {
			t.Fatal("expected refusal of unsigned update over a pinned key")
		}
	})
	t.Run("unsigned with no pin stays unpinned", func(t *testing.T) {
		got, err := checkSignature(unsigned, "")
		if err != nil || got != "" {
			t.Fatalf("got (%q, %v), want (\"\", nil)", got, err)
		}
	})
}

// TestVerifyAppImageSignatureGPGInterop proves appherder verifies signatures
// produced the same way appimagetool produces them (gpg/gpgme: an armored
// detached signature over the lowercase hex digest). It is skipped where gpg is
// unavailable.
func TestVerifyAppImageSignatureGPGInterop(t *testing.T) {
	gpg, err := exec.LookPath("gpg")
	if err != nil {
		t.Skip("gpg not installed")
	}
	home := t.TempDir()
	run := func(input []byte, args ...string) ([]byte, error) {
		cmd := exec.Command(gpg, append([]string{"--batch", "--no-tty"}, args...)...)
		cmd.Env = append(os.Environ(), "GNUPGHOME="+home)
		if input != nil {
			cmd.Stdin = bytes.NewReader(input)
		}
		return cmd.Output()
	}
	if _, err := run(nil, "--pinentry-mode", "loopback", "--passphrase", "",
		"--quick-generate-key", "AppHerder Interop <interop@appherder.local>", "rsa2048", "sign", "0"); err != nil {
		t.Skipf("gpg key generation failed: %v", err)
	}

	colons, err := run(nil, "--list-keys", "--with-colons")
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	var fpr string
	for line := range strings.SplitSeq(string(colons), "\n") {
		if fields := strings.Split(line, ":"); fields[0] == "fpr" {
			fpr = fields[9]
			break
		}
	}

	layout := buildSignableELF(2048, 4096)
	sum := sha256.Sum256(layout.bytes)
	sig, err := run([]byte(hex.EncodeToString(sum[:])), "--pinentry-mode", "loopback", "--passphrase", "",
		"--armor", "--detach-sign", "--output", "-")
	if err != nil {
		t.Fatalf("gpg sign: %v", err)
	}
	key, err := run(nil, "--armor", "--export")
	if err != nil {
		t.Fatalf("gpg export: %v", err)
	}

	path := writeAppImage(t, layout.embed(sig, key))
	got, err := verifyAppImageSignature(path)
	if err != nil {
		t.Fatalf("verify gpg-signed AppImage: %v", err)
	}
	if got != fpr {
		t.Fatalf("fingerprint = %s, want %s", got, fpr)
	}
}
