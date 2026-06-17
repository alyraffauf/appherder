package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// elfHeader builds a minimal little-endian ELF header with the given class
// (1=32-bit, 2=64-bit) and section-header-table fields.
func elfHeader(class byte, shoff uint64, shentsize, shnum uint16) []byte {
	h := make([]byte, 64)
	copy(h, []byte{0x7f, 'E', 'L', 'F'})
	h[4] = class
	h[5] = 1 // little-endian
	if class == 2 {
		binary.LittleEndian.PutUint64(h[40:48], shoff)
		binary.LittleEndian.PutUint16(h[58:60], shentsize)
		binary.LittleEndian.PutUint16(h[60:62], shnum)
	} else {
		binary.LittleEndian.PutUint32(h[32:36], uint32(shoff))
		binary.LittleEndian.PutUint16(h[46:48], shentsize)
		binary.LittleEndian.PutUint16(h[48:50], shnum)
	}
	return h
}

func TestAppImageSquashfsOffset(t *testing.T) {
	for _, class := range []byte{1, 2} {
		got, err := appImageSquashfsOffset(bytes.NewReader(elfHeader(class, 1000, 64, 10)))
		if err != nil {
			t.Fatalf("class %d: %v", class, err)
		}
		if want := int64(1000 + 64*10); got != want {
			t.Fatalf("class %d: offset = %d, want %d", class, got, want)
		}
	}
}

func TestAppImageSquashfsOffsetRejectsNonELF(t *testing.T) {
	if _, err := appImageSquashfsOffset(bytes.NewReader(make([]byte, 64))); err == nil {
		t.Fatal("expected error for non-ELF input")
	}
}

func TestAppImageSquashfsOffsetRejectsType1(t *testing.T) {
	h := elfHeader(2, 1000, 64, 10)
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
	a := app{homeDir: func() (string, error) { return home, nil }}

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
	a := app{homeDir: func() (string, error) { return home, nil }}

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
	a := app{homeDir: func() (string, error) { return home, nil }}
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
	a := app{homeDir: func() (string, error) { return home, nil }}
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
	a := app{homeDir: func() (string, error) { return home, nil }}

	got, err := a.installAppImage(dest, "foo")
	if err != nil {
		t.Fatal(err)
	}
	if got != dest {
		t.Fatalf("dest = %q, want %q", got, dest)
	}
	assertExecutableFile(t, dest, "payload")
}
