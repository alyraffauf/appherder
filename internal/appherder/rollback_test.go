package appherder

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRollbackWithExplicitVersion(t *testing.T) {
	a, home := newTestApp(t)
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}

	versionsDir := filepath.Join(appimages, ".versions", "foo")
	if err := os.MkdirAll(versionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versionsDir, "v1.0.appimage"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	current := filepath.Join(appimages, "foo.appimage")
	if err := os.WriteFile(current, []byte("current"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.Rollback("foo", "v1.0"); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(current)
	if string(got) != "old" {
		t.Fatalf("current = %q, want old", string(got))
	}
}

func TestRollbackPicksMostRecentByMtime(t *testing.T) {
	a, home := newTestApp(t)
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}

	versionsDir := filepath.Join(appimages, ".versions", "foo")
	if err := os.MkdirAll(versionsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	v1 := filepath.Join(versionsDir, "v1.appimage")
	v2 := filepath.Join(versionsDir, "v2.appimage")
	if err := os.WriteFile(v1, []byte("oldest"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(v2, []byte("older"), 0o644); err != nil {
		t.Fatal(err)
	}

	t1 := time.Now().Add(-2 * time.Hour)
	t2 := time.Now().Add(-1 * time.Hour)
	os.Chtimes(v1, t1, t1)
	os.Chtimes(v2, t2, t2)

	current := filepath.Join(appimages, "foo.appimage")
	if err := os.WriteFile(current, []byte("current"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.Rollback("foo", ""); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(current)
	if string(got) != "older" {
		t.Fatalf("current = %q, want older (most recent mtime)", string(got))
	}
}

func TestRollbackSavesCurrentVersionBeforeRestoring(t *testing.T) {
	a, home := newTestApp(t)
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}

	versionsDir := filepath.Join(appimages, ".versions", "foo")
	if err := os.MkdirAll(versionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versionsDir, "v1.appimage"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	current := filepath.Join(appimages, "foo.appimage")
	if err := os.WriteFile(current, []byte("current"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.Rollback("foo", "v1"); err != nil {
		t.Fatal(err)
	}

	// Current should now be "old"
	got, _ := os.ReadFile(current)
	if string(got) != "old" {
		t.Fatalf("current = %q, want old", string(got))
	}

	// Previous current should be saved; the restored version moves to active.
	entries, _ := os.ReadDir(versionsDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 saved version (pre-rollback current), got %d", len(entries))
	}
}

func TestRollbackErrorOnMissingVersion(t *testing.T) {
	a, _ := newTestApp(t)
	if err := a.Rollback("foo", "v99"); err == nil {
		t.Fatal("expected error for nonexistent version")
	}
}

func TestRollbackErrorOnNoSavedVersions(t *testing.T) {
	a, _ := newTestApp(t)
	if err := a.Rollback("foo", ""); err == nil {
		t.Fatal("expected error when no saved versions exist")
	}
}

func TestNewestVersionPicksByMtime(t *testing.T) {
	dir := t.TempDir()
	v1 := filepath.Join(dir, "v1.appimage")
	v2 := filepath.Join(dir, "v2.appimage")
	if err := os.WriteFile(v1, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(v2, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	t1 := time.Now().Add(-2 * time.Hour)
	t2 := time.Now().Add(-1 * time.Hour)
	os.Chtimes(v1, t1, t1)
	os.Chtimes(v2, t2, t2)

	got, err := newestVersion(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "v2" {
		t.Fatalf("newestVersion = %q, want v2", got)
	}
}

func TestNewestVersionSkipsDirectoriesAndNonAppImages(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "v1.appimage"), []byte("x"), 0o644)

	got, err := newestVersion(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "v1" {
		t.Fatalf("newestVersion = %q, want v1", got)
	}
}

func TestNewestVersionErrorOnEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if _, err := newestVersion(dir); err == nil {
		t.Fatal("expected error for empty directory")
	}
}

func TestNewestVersionErrorOnMissingDir(t *testing.T) {
	if _, err := newestVersion("/nonexistent"); err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestUninstallCleansUpVersions(t *testing.T) {
	a, home := newTestApp(t)
	appimages := filepath.Join(home, "AppImages")
	versionsDir := filepath.Join(appimages, ".versions", "foo")
	if err := os.MkdirAll(versionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versionsDir, "v1.appimage"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versionsDir, "v2.appimage"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Uninstall needs a managed desktop entry to succeed without --force.
	writeManagedDesktop(t, home, "foo")

	if err := a.Uninstall("foo", false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(versionsDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(".versions/ directory should be removed by uninstall")
	}
}
