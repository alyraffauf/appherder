package appherder

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestApp returns an App whose directories live under a temp dir.
func newTestApp(t *testing.T) (App, string) {
	t.Helper()
	home := t.TempDir()
	return NewAppWithDirs(
		filepath.Join(home, "AppImages"),
		filepath.Join(home, ".local", "share", "applications"),
		filepath.Join(home, "AppImages", ".icons"),
		filepath.Join(home, ".local", "bin"),
	), home
}

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
