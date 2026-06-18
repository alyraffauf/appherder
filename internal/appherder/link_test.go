package appherder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLink(t *testing.T) {
	app, home := newTestApp(t)
	mustWrite(t, filepath.Join(home, "AppImages", "foo.appimage"), []byte("fake appimage"))

	if err := app.Link("foo"); err != nil {
		t.Fatalf("Link: %v", err)
	}

	link := filepath.Join(home, ".local", "bin", "foo")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	want := filepath.Join(home, "AppImages", "foo.appimage")
	if target != want {
		t.Errorf("link target = %q, want %q", target, want)
	}
}

func TestLinkUninstalled(t *testing.T) {
	app, _ := newTestApp(t)
	err := app.Link("nope")
	if err == nil {
		t.Fatal("expected error linking uninstalled app")
	}
}

func TestLinkNormalizesName(t *testing.T) {
	app, home := newTestApp(t)
	mustWrite(t, filepath.Join(home, "AppImages", "foo.appimage"), []byte("x"))

	if err := app.Link("foo.appimage"); err != nil {
		t.Fatalf("Link: %v", err)
	}

	link := filepath.Join(home, ".local", "bin", "foo")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != filepath.Join(home, "AppImages", "foo.appimage") {
		t.Errorf("unexpected target: %s", target)
	}
}

func TestUnlink(t *testing.T) {
	app, home := newTestApp(t)
	mustWrite(t, filepath.Join(home, "AppImages", "foo.appimage"), []byte("x"))
	binDir := filepath.Join(home, ".local", "bin")
	mustMkdir(t, binDir)
	mustSymlink(t, filepath.Join(home, "AppImages", "foo.appimage"), filepath.Join(binDir, "foo"))

	if err := app.Unlink("foo"); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if _, err := os.Readlink(filepath.Join(binDir, "foo")); !os.IsNotExist(err) {
		t.Fatal("link still exists after unlink")
	}
}

func TestUnlinkNotLinked(t *testing.T) {
	app, _ := newTestApp(t)
	err := app.Unlink("nope")
	if err == nil {
		t.Fatal("expected error unlinking nonexistent link")
	}
}

func TestUnlinkNormalizesName(t *testing.T) {
	app, home := newTestApp(t)
	mustWrite(t, filepath.Join(home, "AppImages", "foo.appimage"), []byte("x"))
	binDir := filepath.Join(home, ".local", "bin")
	mustMkdir(t, binDir)
	mustSymlink(t, filepath.Join(home, "AppImages", "foo.appimage"), filepath.Join(binDir, "foo"))

	if err := app.Unlink("foo.appimage"); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if _, err := os.Readlink(filepath.Join(binDir, "foo")); !os.IsNotExist(err) {
		t.Fatal("link still exists after unlink")
	}
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}
