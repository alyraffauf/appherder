package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExtractAppImageUsesDestinationAsWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.appimage")
	extractDest := filepath.Join(dir, "extract")
	if err := os.WriteFile(source, []byte("appimage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(extractDest, 0o755); err != nil {
		t.Fatal(err)
	}

	var gotName string
	var gotArgs []string
	var gotDir string
	a := app{
		run: func(name string, args []string, dir string) error {
			gotName = name
			gotArgs = args
			gotDir = dir
			return nil
		},
	}

	extracted, err := a.extractAppImage(source, extractDest)
	if err != nil {
		t.Fatal(err)
	}

	absSource, err := filepath.Abs(source)
	if err != nil {
		t.Fatal(err)
	}
	if gotName != absSource {
		t.Fatalf("command name = %q, want %q", gotName, absSource)
	}
	if !reflect.DeepEqual(gotArgs, []string{"--appimage-extract"}) {
		t.Fatalf("command args = %#v", gotArgs)
	}
	if gotDir != extractDest {
		t.Fatalf("command dir = %q, want %q", gotDir, extractDest)
	}
	if extracted != filepath.Join(extractDest, "squashfs-root") {
		t.Fatalf("extracted = %q", extracted)
	}
	if mode := fileMode(source); mode != 0o755 {
		t.Fatalf("source mode = %#o, want 0755", mode)
	}
}

func TestInstallAppImageCopiesToAppImagesDir(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	source := filepath.Join(dir, "source.appimage")
	if err := os.WriteFile(source, []byte("appimage"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := app{homeDir: func() (string, error) { return home, nil }}
	dest, err := a.installAppImage(source, "test")
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(home, "AppImages", "test.appimage")
	if dest != want {
		t.Fatalf("dest = %q, want %q", dest, want)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "appimage" {
		t.Fatalf("installed data = %q", data)
	}
	if mode := fileMode(dest); mode&0o100 == 0 {
		t.Fatalf("installed file is not user-executable: %#o", mode)
	}
}

func TestInstallAppImageReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	appimages := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}

	existing := filepath.Join(appimages, "test.appimage")
	if err := os.WriteFile(existing, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "source.appimage")
	if err := os.WriteFile(source, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := app{homeDir: func() (string, error) { return home, nil }}
	dest, err := a.installAppImage(source, "test")
	if err != nil {
		t.Fatal(err)
	}
	if dest != existing {
		t.Fatalf("dest = %q, want %q", dest, existing)
	}
	data, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("replacement data = %q", data)
	}
	matches, err := filepath.Glob(filepath.Join(appimages, "*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %#v", matches)
	}
}
