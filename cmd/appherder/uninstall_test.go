package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

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
		if err := os.WriteFile(path, []byte("owned"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	a := app{homeDir: func() (string, error) { return home, nil }}
	if err := a.uninstall(filepath.Join(home, "AppImages", "example.AppImage")); err != nil {
		t.Fatal(err)
	}

	for _, path := range installedPaths(home, "example") {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected %s to be removed, stat err: %v", path, err)
		}
	}
}
