package main

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeExtract simulates `appimage --appimage-extract` by populating a
// squashfs-root directory with a desktop file and icon, mirroring what a real
// AppImage runtime writes during extraction.
func fakeExtract(t *testing.T, desktop string, icon []byte) func(name string, args []string, dir string) error {
	t.Helper()
	return func(name string, args []string, dir string) error {
		root := filepath.Join(dir, "squashfs-root")
		if err := os.MkdirAll(root, 0o755); err != nil {
			return err
		}
		if desktop != "" {
			if err := os.WriteFile(filepath.Join(root, "example.desktop"), []byte(desktop), 0o644); err != nil {
				return err
			}
		}
		if icon != nil {
			if err := os.WriteFile(filepath.Join(root, ".DirIcon"), icon, 0o644); err != nil {
				return err
			}
		}
		return nil
	}
}

func TestInstallWritesAppImageIconAndDesktopFile(t *testing.T) {
	home := t.TempDir()
	src := t.TempDir()
	appimage := filepath.Join(src, "Example.AppImage")
	if err := os.WriteFile(appimage, []byte("appimage-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	iconBytes := []byte("icon-bytes")
	a := app{
		homeDir: func() (string, error) { return home, nil },
		run:     fakeExtract(t, sampleDesktopFile, iconBytes),
	}

	if err := a.install(appimage); err != nil {
		t.Fatalf("install: %v", err)
	}

	// AppImage copied into ~/AppImages and made executable.
	installedAppImage := filepath.Join(home, "AppImages", "Example.appimage")
	info, err := os.Stat(installedAppImage)
	if err != nil {
		t.Fatalf("stat installed appimage: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("installed appimage is not executable: %v", info.Mode())
	}
	if got, _ := os.ReadFile(installedAppImage); string(got) != "appimage-bytes" {
		t.Fatalf("installed appimage contents = %q", got)
	}

	// Icon copied into ~/AppImages/.icons/<app>.
	installedIcon := filepath.Join(home, "AppImages", ".icons", "Example")
	if got, _ := os.ReadFile(installedIcon); string(got) != string(iconBytes) {
		t.Fatalf("installed icon contents = %q", got)
	}

	// Desktop file written and patched to point at the installed paths.
	installedDesktop := filepath.Join(home, ".local", "share", "applications", "Example.desktop")
	desktop, err := readDesktopFile(installedDesktop)
	if err != nil {
		t.Fatalf("read installed desktop file: %v", err)
	}
	assertDesktopValue(t, desktop, desktopEntrySection, "TryExec", installedAppImage)
	assertDesktopValue(t, desktop, desktopEntrySection, "Icon", installedIcon)
	assertDesktopExec(t, desktop, desktopEntrySection,
		[]string{"env", "FOO=bar", "DESKTOPINTEGRATION=1", installedAppImage, "%U"})
}

func TestInstallWithoutDesktopFileStillInstallsAppImage(t *testing.T) {
	home := t.TempDir()
	src := t.TempDir()
	appimage := filepath.Join(src, "Example.AppImage")
	if err := os.WriteFile(appimage, []byte("appimage-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := app{
		homeDir: func() (string, error) { return home, nil },
		run:     fakeExtract(t, "", nil),
	}

	if err := a.install(appimage); err != nil {
		t.Fatalf("install: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, "AppImages", "Example.appimage")); err != nil {
		t.Fatalf("stat installed appimage: %v", err)
	}
	// No desktop file in the extracted AppImage means none is written.
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "applications", "Example.desktop")); !os.IsNotExist(err) {
		t.Fatalf("expected no desktop file, stat err: %v", err)
	}
}
