package appherder

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncRemovesOrphanedManagedLauncher(t *testing.T) {
	a, home := newTestApp(t)
	if err := os.MkdirAll(filepath.Join(home, "AppImages"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeManagedDesktop(t, home, "gone")

	result, err := a.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(home, ".local", "share", "applications", "gone.desktop")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphaned launcher should be removed, stat err: %v", err)
	}
	if !removalSucceeded(result, "gone") {
		t.Fatalf("sync result = %+v, want removal of gone", result)
	}
}

func TestSyncKeepsManagedLauncherWhenAppImagePresent(t *testing.T) {
	a, home := newTestApp(t)
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

	result, err := a.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(home, ".local", "share", "applications", "present.desktop")); err != nil {
		t.Fatalf("launcher for a present (if unparseable) AppImage should be kept: %v", err)
	}
	if !installFailed(result, "present.appimage") {
		t.Fatalf("sync result = %+v, want a failed install for present.appimage", result)
	}
}

func TestSyncKeepsUnmanagedLauncherEvenWhenAppImageAbsent(t *testing.T) {
	a, home := newTestApp(t)
	if err := os.MkdirAll(filepath.Join(home, "AppImages"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// No TryExec under ~/AppImages: --force must leave it alone.
	handmade := filepath.Join(dir, "handmade.desktop")
	if err := os.WriteFile(handmade, []byte("[Desktop Entry]\nName=Mine\nTryExec=/usr/bin/foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, force := range []bool{false, true} {
		if _, err := a.Sync(context.Background(), force); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(handmade); err != nil {
			t.Fatalf("force=%v: unmanaged launcher must not be touched, stat err: %v", force, err)
		}
	}
}

func TestSyncForceRemovesAppImageBackedOrphan(t *testing.T) {
	a, home := newTestApp(t)
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Unmanaged launcher pointing at a missing AppImage: --force removes it.
	target := filepath.Join(appimages, "gone.appimage")
	if err := os.WriteFile(filepath.Join(dir, "gone.desktop"),
		[]byte("[Desktop Entry]\nName=Gone\nTryExec="+target+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := a.Sync(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "gone.desktop")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("--force should remove the AppImage-backed orphan, stat err: %v", err)
	}
	if !removalSucceeded(result, "gone") {
		t.Fatalf("sync result = %+v, want removal reported", result)
	}
}

func TestSyncForceKeepsOrphanWhenAppImageStillPresent(t *testing.T) {
	a, home := newTestApp(t)
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Present (if unparseable) AppImage: --force keeps it; install adopts it.
	target := filepath.Join(appimages, "present.appimage")
	if err := os.WriteFile(target, []byte("not an appimage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "present.desktop"),
		[]byte("[Desktop Entry]\nName=Present\nTryExec="+target+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := a.Sync(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "present.desktop")); err != nil {
		t.Fatalf("--force must keep a launcher whose AppImage is still present: %v", err)
	}
}

func TestSyncIgnoresHiddenAndTempFiles(t *testing.T) {
	a, home := newTestApp(t)
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

	result, err := a.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installs) != 0 || len(result.Removals) != 0 {
		t.Fatalf("sync result = %+v, want no activity for hidden/temp files", result)
	}
}

func TestSyncHandlesMissingAppImagesDir(t *testing.T) {
	a, home := newTestApp(t)
	writeManagedDesktop(t, home, "orphan")

	if _, err := a.Sync(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "applications", "orphan.desktop")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing ~/AppImages should still reconcile orphans away, stat err: %v", err)
	}
}

func TestSyncReportsSkipsInInputOrder(t *testing.T) {
	a, home := newTestApp(t)
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	// Concurrent installs must still produce results in input order.
	for _, name := range []string{"charlie.appimage", "alpha.appimage", "bravo.appimage"} {
		if err := os.WriteFile(filepath.Join(appimages, name), []byte("not an appimage"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := a.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"alpha.appimage", "bravo.appimage", "charlie.appimage"}
	var failed []string
	for _, inst := range result.Installs {
		if inst.Err != nil {
			failed = append(failed, filepath.Base(inst.File))
		}
	}
	if len(failed) != len(want) {
		t.Fatalf("failed installs = %v, want %d", failed, len(want))
	}
	for i, name := range want {
		if failed[i] != name {
			t.Fatalf("failed[%d] = %s, want %s (order must be sorted)", i, failed[i], name)
		}
	}
}

// removalSucceeded reports whether result includes a successful removal of appid.
func removalSucceeded(result SyncResult, appid string) bool {
	for _, rem := range result.Removals {
		if rem.AppName == appid && rem.Err == nil {
			return true
		}
	}
	return false
}

// installFailed reports whether result includes a failed install whose File
// basename ends with filename.
func installFailed(result SyncResult, filename string) bool {
	for _, inst := range result.Installs {
		if inst.Err != nil && filepath.Base(inst.File) == filename {
			return true
		}
	}
	return false
}
