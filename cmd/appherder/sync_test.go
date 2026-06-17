package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeManagedDesktop stamps a launcher with appherder's ownership marker.
func writeManagedDesktop(t *testing.T, home, appid string) {
	t.Helper()
	dir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, appid+".desktop"), []byte(managedDesktop), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSyncRemovesOrphanedManagedLauncher(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "AppImages"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeManagedDesktop(t, home, "gone")

	var out bytes.Buffer
	a := app{homeDir: func() (string, error) { return home, nil }}
	if err := a.sync(&out); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(home, ".local", "share", "applications", "gone.desktop")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphaned launcher should be removed, stat err: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("removed gone")) {
		t.Fatalf("sync output = %q, want it to report removal of gone", out.String())
	}
}

func TestSyncKeepsManagedLauncherWhenAppImagePresent(t *testing.T) {
	home := t.TempDir()
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	// Present but unparseable: install skips it, and the orphan pass must
	// still see it as present and keep the launcher.
	if err := os.WriteFile(filepath.Join(appimages, "present.appimage"), []byte("not an appimage"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeManagedDesktop(t, home, "present")

	var out bytes.Buffer
	a := app{homeDir: func() (string, error) { return home, nil }}
	if err := a.sync(&out); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(home, ".local", "share", "applications", "present.desktop")); err != nil {
		t.Fatalf("launcher for a present (if unparseable) AppImage should be kept: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("skip present.appimage:")) {
		t.Fatalf("sync output = %q, want a skip report for the bad file", out.String())
	}
}

func TestSyncKeepsUnmanagedLauncherEvenWhenAppImageAbsent(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "AppImages"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "handmade.desktop"), []byte("[Desktop Entry]\nName=Mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	a := app{homeDir: func() (string, error) { return home, nil }}
	if err := a.sync(&out); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "handmade.desktop")); err != nil {
		t.Fatalf("unmanaged launcher must not be touched by sync: %v", err)
	}
}

func TestSyncIgnoresHiddenAndTempFiles(t *testing.T) {
	home := t.TempDir()
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	// Partial download and editor temp: the *.appimage glob must not match.
	for _, name := range []string{".foo.appimage.part", ".appimagetool.appimage.swp"} {
		if err := os.WriteFile(filepath.Join(appimages, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var out bytes.Buffer
	a := app{homeDir: func() (string, error) { return home, nil }}
	if err := a.sync(&out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("sync output = %q, want no activity for hidden/temp files", out.String())
	}
}

func TestSyncHandlesMissingAppImagesDir(t *testing.T) {
	home := t.TempDir()
	writeManagedDesktop(t, home, "orphan")

	var out bytes.Buffer
	a := app{homeDir: func() (string, error) { return home, nil }}
	if err := a.sync(&out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "applications", "orphan.desktop")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing ~/AppImages should still reconcile orphans away, stat err: %v", err)
	}
}
