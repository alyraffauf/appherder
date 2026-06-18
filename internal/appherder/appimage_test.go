package appherder

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAppImageSquashfsOffset(t *testing.T) {
	// First 64 bytes of a real type-2 ELF64 AppImage (appimagetool), whose
	// squashfs payload begins at the offset asserted below.
	header := []byte{
		0x7f, 0x45, 0x4c, 0x46, 0x02, 0x01, 0x01, 0x00, 0x41, 0x49, 0x02, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x3e, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x87, 0xae, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x78, 0x62, 0x0e, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x40, 0x00, 0x38, 0x00, 0x0a, 0x00, 0x40, 0x00,
		0x1e, 0x00, 0x1d, 0x00,
	}
	got, err := appImageSquashfsOffset(bytes.NewReader(header))
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(944632); got != want {
		t.Fatalf("offset = %d, want %d", got, want)
	}
}

func TestAppImageSquashfsOffsetRejectsNonELF(t *testing.T) {
	if _, err := appImageSquashfsOffset(bytes.NewReader(make([]byte, 64))); err == nil {
		t.Fatal("expected error for non-ELF input")
	}
}

func TestAppImageSquashfsOffsetRejectsType1(t *testing.T) {
	h := make([]byte, 64)
	copy(h, []byte{0x7f, 'E', 'L', 'F'})
	h[8], h[9], h[10] = 'A', 'I', 1
	if _, err := appImageSquashfsOffset(bytes.NewReader(h)); err == nil {
		t.Fatal("expected error for type-1 AppImage")
	}
}

func assertExecutableFile(t *testing.T, path, want string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("%s is not executable: %v", path, info.Mode())
	}
	if got, _ := os.ReadFile(path); string(got) != want {
		t.Fatalf("%s contents = %q, want %q", path, got, want)
	}
}

func TestInstallAppImageCopiesFromElsewhere(t *testing.T) {
	home := t.TempDir()
	src := filepath.Join(t.TempDir(), "Foo-1.0-x86_64.AppImage")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := App{homeDir: func() (string, error) { return home, nil }}

	dest, err := a.installAppImage(src, "foo")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, "AppImages", "foo.appimage"); dest != want {
		t.Fatalf("dest = %q, want %q", dest, want)
	}
	assertExecutableFile(t, dest, "payload")
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source should remain after copy: %v", err)
	}
}

func TestInstallAppImageMovesVersionedFileInPlace(t *testing.T) {
	home := t.TempDir()
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(appimages, "Foo-1.0-x86_64.appimage")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := App{homeDir: func() (string, error) { return home, nil }}

	dest, err := a.installAppImage(src, "foo")
	if err != nil {
		t.Fatal(err)
	}
	assertExecutableFile(t, dest, "payload")
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("versioned source should be moved away, stat err: %v", err)
	}
}

func TestInstallAppImageSkipsCopyWhenIdentical(t *testing.T) {
	home := t.TempDir()
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(appimages, "foo.appimage")
	if err := os.WriteFile(dest, []byte("payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "Foo-1.0.AppImage")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	before, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	a := App{homeDir: func() (string, error) { return home, nil }}
	if _, err := a.installAppImage(src, "foo"); err != nil {
		t.Fatal(err)
	}

	after, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("dest was rewritten despite identical content")
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("external source should remain: %v", err)
	}
}

func TestInstallAppImageRemovesIdenticalDuplicateInFolder(t *testing.T) {
	home := t.TempDir()
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(appimages, "foo.appimage")
	if err := os.WriteFile(dest, []byte("payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	dup := filepath.Join(appimages, "Foo-1.0.appimage")
	if err := os.WriteFile(dup, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	before, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	a := App{homeDir: func() (string, error) { return home, nil }}
	if _, err := a.installAppImage(dup, "foo"); err != nil {
		t.Fatal(err)
	}

	after, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("canonical AppImage was rewritten despite identical content")
	}
	if _, err := os.Stat(dup); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("identical duplicate should be removed, stat err: %v", err)
	}
}

func TestInstallAppImageNoOpWhenAlreadyCanonical(t *testing.T) {
	home := t.TempDir()
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(appimages, "foo.appimage")
	if err := os.WriteFile(dest, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := App{homeDir: func() (string, error) { return home, nil }}

	got, err := a.installAppImage(dest, "foo")
	if err != nil {
		t.Fatal(err)
	}
	if got != dest {
		t.Fatalf("dest = %q, want %q", got, dest)
	}
	assertExecutableFile(t, dest, "payload")
}
