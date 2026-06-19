package appherder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestResolveIconPrefersDirIcon(t *testing.T) {
	fsys := fstest.MapFS{
		".DirIcon":              {Data: []byte("dir")},
		"app.png":               {Data: []byte("png")},
		"usr/share/icons/a.svg": {Data: []byte("svg")},
	}
	if got := resolveIcon(fsys); got != ".DirIcon" {
		t.Fatalf("resolveIcon = %q, want .DirIcon", got)
	}
}

func TestResolveIconPrefersRootOverThemedEvenIfSmaller(t *testing.T) {
	fsys := fstest.MapFS{
		"app.png":                 {Data: []byte("x")},                // root, tiny png
		"usr/share/icons/big.svg": {Data: []byte("a larger payload")}, // themed, higher format and larger
	}
	if got := resolveIcon(fsys); got != "app.png" {
		t.Fatalf("resolveIcon = %q, want app.png (root beats themed)", got)
	}
}

func TestResolveIconPrefersSvgWithinTier(t *testing.T) {
	fsys := fstest.MapFS{
		"app.png": {Data: []byte("0123456789")}, // larger png
		"app.svg": {Data: []byte("s")},          // smaller, but higher format
	}
	if got := resolveIcon(fsys); got != "app.svg" {
		t.Fatalf("resolveIcon = %q, want app.svg", got)
	}
}

func TestResolveIconPrefersLargerWithinSameFormat(t *testing.T) {
	fsys := fstest.MapFS{
		"small.png": {Data: []byte("a")},
		"big.png":   {Data: []byte("aaaaaaaaaa")},
	}
	if got := resolveIcon(fsys); got != "big.png" {
		t.Fatalf("resolveIcon = %q, want big.png", got)
	}
}

func TestResolveIconUsesThemedWhenNoRootIcon(t *testing.T) {
	fsys := fstest.MapFS{
		"usr/share/icons/a.png": {Data: []byte("png")},
		"usr/share/icons/b.svg": {Data: []byte("svg")},
	}
	if got := resolveIcon(fsys); got != "usr/share/icons/b.svg" {
		t.Fatalf("resolveIcon = %q, want usr/share/icons/b.svg", got)
	}
}

func TestResolveIconReturnsEmptyWhenNoIcon(t *testing.T) {
	fsys := fstest.MapFS{
		"AppRun":      {Data: []byte("x")},
		"app.desktop": {Data: []byte("x")},
	}
	if got := resolveIcon(fsys); got != "" {
		t.Fatalf("resolveIcon = %q, want empty", got)
	}
}

func TestInstallIconPreservesExtension(t *testing.T) {
	app, home := newTestApp(t)
	fsys := fstest.MapFS{
		"keepassxc.png": {Data: []byte("\x89PNG\r\n\x1a\npng")},
	}

	got, err := app.installIcon(fsys, "keepassxc.png", "keepassxc")
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(home, "AppImages", ".icons", "keepassxc.png")
	if got != want {
		t.Fatalf("installIcon = %q, want %q", got, want)
	}
}

func TestInstallIconDetectsDirIconPNG(t *testing.T) {
	app, home := newTestApp(t)
	fsys := fstest.MapFS{
		".DirIcon": {Data: []byte("\x89PNG\r\n\x1a\npng")},
	}

	got, err := app.installIcon(fsys, ".DirIcon", "keepassxc")
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(home, "AppImages", ".icons", "keepassxc.png")
	if got != want {
		t.Fatalf("installIcon = %q, want %q", got, want)
	}
}

func TestInstallIconRemovesStaleSiblings(t *testing.T) {
	app, home := newTestApp(t)
	iconDir := filepath.Join(home, "AppImages", ".icons")
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(iconDir, "keepassxc.svg")
	if err := os.WriteFile(stale, []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{
		"keepassxc.png": {Data: []byte("\x89PNG\r\n\x1a\npng")},
	}

	if _, err := app.installIcon(fsys, "keepassxc.png", "keepassxc"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("expected stale icon to be removed, stat err: %v", err)
	}
}

func TestInstallAppImageWritesExtensionedDirIconPath(t *testing.T) {
	if _, err := exec.LookPath("mksquashfs"); err != nil {
		t.Skip("mksquashfs not installed")
	}

	app, home := newTestApp(t)
	appimage := makeAppImageFixture(t, map[string][]byte{
		".DirIcon": []byte("\x89PNG\r\n\x1a\npng"),
		"keepassxc.desktop": []byte(
			"[Desktop Entry]\n" +
				"Type=Application\n" +
				"Name=KeePassXC\n" +
				"Exec=keepassxc %U\n",
		),
	})

	name, err := app.Install(appimage)
	if err != nil {
		t.Fatal(err)
	}
	if name != "keepassxc" {
		t.Fatalf("installed name = %q, want keepassxc", name)
	}

	iconPath := filepath.Join(home, "AppImages", ".icons", "keepassxc.png")
	if _, err := os.Stat(iconPath); err != nil {
		t.Fatalf("installed icon missing: %v", err)
	}
	desktop, err := os.ReadFile(filepath.Join(home, ".local", "share", "applications", "keepassxc.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	want := "Icon=" + iconPath + "\n"
	if !bytes.Contains(desktop, []byte(want)) {
		t.Fatalf("desktop file missing %q:\n%s", want, desktop)
	}
}

func makeAppImageFixture(t *testing.T, files map[string][]byte) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	squashfs := filepath.Join(t.TempDir(), "payload.squashfs")
	cmd := exec.Command("mksquashfs", root, squashfs, "-noappend", "-quiet")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mksquashfs: %v\n%s", err, out)
	}
	payload, err := os.ReadFile(squashfs)
	if err != nil {
		t.Fatal(err)
	}

	layout := buildSignableELF(0, 0)
	appimage := filepath.Join(t.TempDir(), "fixture.AppImage")
	if err := os.WriteFile(appimage, append(layout.bytes, payload...), 0o755); err != nil {
		t.Fatal(err)
	}
	return appimage
}
