package appherder

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	a, home := newTestApp(t)
	src := filepath.Join(t.TempDir(), "Foo-1.0-x86_64.AppImage")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

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
	a, home := newTestApp(t)
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(appimages, "Foo-1.0-x86_64.appimage")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

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
	a, home := newTestApp(t)
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
	a, home := newTestApp(t)
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
	a, home := newTestApp(t)
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(appimages, "foo.appimage")
	if err := os.WriteFile(dest, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := a.installAppImage(dest, "foo")
	if err != nil {
		t.Fatal(err)
	}
	if got != dest {
		t.Fatalf("dest = %q, want %q", got, dest)
	}
	assertExecutableFile(t, dest, "payload")
}

func TestSanitizeVersionForFilename(t *testing.T) {
	tests := []struct{ in, want string }{
		{"v1.2.3", "v1.2.3"},
		{"2024/05/01", "2024_05_01"},
		{"path\\separator", "path_separator"},
		{"mixed/path\\thing", "mixed_path_thing"},
	}
	for _, tc := range tests {
		if got := sanitizeVersionForFilename(tc.in); got != tc.want {
			t.Fatalf("sanitizeVersionForFilename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	long := ""
	for i := 0; i < 200; i++ {
		long += "x"
	}
	if got := sanitizeVersionForFilename(long); len(got) != 100 {
		t.Fatalf("expected truncated to 100, got %d", len(got))
	}
}

func TestReadAppImageVersionFallsBackToMtime(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-an-appimage")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(file, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	if got := readAppImageVersion(file); got != "2026-06-18T120000" {
		t.Fatalf("readAppImageVersion = %q, want 2026-06-18T120000", got)
	}
}

func TestSaveToVersionsHardlinksFile(t *testing.T) {
	a, home := newTestApp(t)
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(appimages, "foo.appimage")
	if err := os.WriteFile(current, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.saveToVersions(current, "foo"); err != nil {
		t.Fatal(err)
	}

	versionsDir := filepath.Join(appimages, ".versions", "foo")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 saved version, got %d", len(entries))
	}
	got, _ := os.ReadFile(filepath.Join(versionsDir, entries[0].Name()))
	if string(got) != "v1" {
		t.Fatalf("saved content = %q, want v1", string(got))
	}
}

func TestSaveToVersionsNoopOnMissingFile(t *testing.T) {
	a, _ := newTestApp(t)
	if err := a.saveToVersions("/nonexistent/foo.appimage", "foo"); err != nil {
		t.Fatal(err)
	}
}

func TestInstallAppImageSavesPreviousVersion(t *testing.T) {
	a, home := newTestApp(t)
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}

	current := filepath.Join(appimages, "foo.appimage")
	if err := os.WriteFile(current, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(t.TempDir(), "Foo-2.0.AppImage")
	if err := os.WriteFile(src, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := a.installAppImage(src, "foo")
	if err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(current)
	if string(got) != "v2" {
		t.Fatalf("current = %q, want v2", string(got))
	}

	versionsDir := filepath.Join(appimages, ".versions", "foo")
	entries, _ := os.ReadDir(versionsDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 saved version, got %d", len(entries))
	}
	savedContent, _ := os.ReadFile(filepath.Join(versionsDir, entries[0].Name()))
	if string(savedContent) != "v1" {
		t.Fatalf("saved = %q, want v1", string(savedContent))
	}
}

func TestInstallAppImageSavesPreviousVersionInFolder(t *testing.T) {
	a, home := newTestApp(t)
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}

	current := filepath.Join(appimages, "foo.appimage")
	if err := os.WriteFile(current, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(appimages, "Foo-2.0.appimage")
	if err := os.WriteFile(src, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := a.installAppImage(src, "foo")
	if err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(current)
	if string(got) != "v2" {
		t.Fatalf("current = %q, want v2", string(got))
	}

	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("versioned source should be moved away")
	}

	versionsDir := filepath.Join(appimages, ".versions", "foo")
	entries, _ := os.ReadDir(versionsDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 saved version, got %d", len(entries))
	}
}

func TestSaveToVersionsPrunesOldestVersions(t *testing.T) {
	a, home := newTestApp(t)
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	versionsDir := filepath.Join(appimages, ".versions", "foo")
	if err := os.MkdirAll(versionsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Seed 3 existing versions with distinct mtimes.
	t1 := time.Now().Add(-3 * time.Hour)
	t2 := time.Now().Add(-2 * time.Hour)
	t3 := time.Now().Add(-1 * time.Hour)
	for _, spec := range []struct {
		name  string
		mtime time.Time
	}{
		{"v1.appimage", t1},
		{"v2.appimage", t2},
		{"v3.appimage", t3},
	} {
		path := filepath.Join(versionsDir, spec.name)
		os.WriteFile(path, []byte(spec.name), 0o644)
		os.Chtimes(path, spec.mtime, spec.mtime)
	}

	current := filepath.Join(appimages, "foo.appimage")
	if err := os.WriteFile(current, []byte("current"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Saving a 4th version should prune v1 (oldest)
	if err := a.saveToVersions(current, "foo"); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(versionsDir)
	if len(entries) != 3 {
		t.Fatalf("expected 3 versions after prune, got %d", len(entries))
	}
	for _, entry := range entries {
		if entry.Name() == "v1.appimage" {
			t.Fatal("v1.appimage should have been pruned")
		}
	}
}
