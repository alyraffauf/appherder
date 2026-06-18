package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListShowsInstalledAndOrphaned(t *testing.T) {
	home := t.TempDir()
	appimages := filepath.Join(home, "AppImages")
	appsDir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}

	// Present AppImage with a named desktop entry.
	if err := os.WriteFile(filepath.Join(appimages, "present.appimage"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appsDir, "present.desktop"), []byte(
		"[Desktop Entry]\nName=Present App\nX-AppImage-Version=1.2.3\n"+desktopOwnerKey+"=true\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	// Missing AppImage: orphaned launcher only.
	writeManagedDesktop(t, home, "gone")

	// Unmanaged: must not appear.
	if err := os.WriteFile(
		filepath.Join(appsDir, "other.desktop"),
		[]byte("[Desktop Entry]\nName=Other\n"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	app := app{homeDir: func() (string, error) { return home, nil }}
	if err := app.list(&out); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{"Present App", "present.appimage", "1.2.3", "gone"} {
		if !strings.Contains(got, want) {
			t.Fatalf("list output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "other") {
		t.Fatalf("unmanaged app appeared in list:\n%s", got)
	}
}

func TestListEmptyWhenNothingManaged(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	app := app{homeDir: func() (string, error) { return home, nil }}
	if err := app.list(&out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "NAME") {
		t.Fatalf("expected header only, got:\n%s", out.String())
	}
}

func TestListFallsBackToFilenameForName(t *testing.T) {
	home := t.TempDir()
	appimages := filepath.Join(home, "AppImages")
	appsDir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Desktop file with no Name= field — list should fall back to the filename.
	if err := os.WriteFile(filepath.Join(appimages, "noname.appimage"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appsDir, "noname.desktop"), []byte(
		"[Desktop Entry]\n"+desktopOwnerKey+"=true\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	app := app{homeDir: func() (string, error) { return home, nil }}
	if err := app.list(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "noname.appimage") {
		t.Fatalf("expected filename fallback in output:\n%s", out.String())
	}
}
