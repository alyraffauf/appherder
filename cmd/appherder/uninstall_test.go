package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const managedDesktop = "[Desktop Entry]\n" + desktopOwnerKey + "=true\n"

func TestNormalizeAppNameAcceptsNamesAndAppImagePaths(t *testing.T) {
	tests := map[string]string{
		"example":                               "example",
		"example.appimage":                      "example",
		"example.AppImage":                      "example",
		"example.APPIMAGE":                      "example",
		"/home/test/AppImages/example.AppImage": "example",
		"/home/test/AppImages/example.appimage": "example",
	}

	for input, want := range tests {
		if got := normalizeAppName(input); got != want {
			t.Fatalf("normalizeAppName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestInstalledPaths(t *testing.T) {
	got := installedPaths("/home/test", "example")
	want := []string{
		"/home/test/AppImages/example.appimage",
		"/home/test/AppImages/.icons/example",
		"/home/test/.local/share/applications/example.desktop",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("installedPaths() = %#v, want %#v", got, want)
	}
}

func TestUninstallRemovesInstalledFilesOnly(t *testing.T) {
	home := t.TempDir()
	for _, path := range installedPaths(home, "example") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		content := []byte("owned")
		if strings.HasSuffix(path, ".desktop") {
			content = []byte(managedDesktop)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	a := app{homeDir: func() (string, error) { return home, nil }}
	if err := a.uninstall(filepath.Join(home, "AppImages", "example.AppImage"), false); err != nil {
		t.Fatal(err)
	}

	for _, path := range installedPaths(home, "example") {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected %s to be removed, stat err: %v", path, err)
		}
	}
}

func TestUninstallKeepsUnmanagedDesktopFile(t *testing.T) {
	home := t.TempDir()
	desktop := filepath.Join(home, ".local", "share", "applications", "example.desktop")
	if err := os.MkdirAll(filepath.Dir(desktop), 0o755); err != nil {
		t.Fatal(err)
	}
	// A hand-made launcher with a colliding name and no ownership marker.
	if err := os.WriteFile(desktop, []byte("[Desktop Entry]\nName=Mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := app{homeDir: func() (string, error) { return home, nil }}
	if err := a.uninstall("example", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(desktop); err != nil {
		t.Fatalf("unmanaged desktop file should be kept: %v", err)
	}

	// --force removes it anyway.
	if err := a.uninstall("example", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(desktop); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected forced removal, stat err: %v", err)
	}
}

func TestManagedApps(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "managed.desktop"), []byte(managedDesktop), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other.desktop"), []byte("[Desktop Entry]\nName=Other\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := managedApps(home)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"managed"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("managedApps() = %#v, want %#v", got, want)
	}
}
