package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleDesktopFile = `# leading comment
[Desktop Entry]
Name=Helium
Exec=env FOO=bar helium %U
Icon=helium
Actions=new-window;new-private-window;

[Desktop Action new-window]
Name=New Window
Exec=helium --new-window %U

[Desktop Action new-private-window]
Name=New Private Window
Exec=helium --private-window %U
`

func TestDesktopFileRoundTripMatchesInput(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.desktop")
	output := filepath.Join(dir, "output.desktop")
	if err := os.WriteFile(source, []byte(sampleDesktopFile), 0o644); err != nil {
		t.Fatal(err)
	}

	desktop, err := readDesktopFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := desktop.write(output); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != sampleDesktopFile {
		t.Fatalf("round trip mismatch:\n%s", got)
	}
}

func TestPatchDesktopFilePreservesDesktopActions(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.desktop")
	output := filepath.Join(dir, "output.desktop")
	if err := os.WriteFile(source, []byte(sampleDesktopFile), 0o644); err != nil {
		t.Fatal(err)
	}

	a := app{homeDir: func() (string, error) { return "/home/test", nil }}

	desktop, err := readDesktopFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.patchDesktopFile(desktop, "helium"); err != nil {
		t.Fatal(err)
	}
	if err := desktop.write(output); err != nil {
		t.Fatal(err)
	}

	written, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	text := string(written)
	for _, expected := range []string{
		"[Desktop Action new-window]",
		"[Desktop Action new-private-window]",
		"Name=New Private Window",
		"Exec=env FOO=bar DESKTOPINTEGRATION=1 /home/test/AppImages/helium.appimage %U",
		"Exec=env DESKTOPINTEGRATION=1 /home/test/AppImages/helium.appimage --new-window %U",
		"Exec=env DESKTOPINTEGRATION=1 /home/test/AppImages/helium.appimage --private-window %U",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("patched desktop file missing %q:\n%s", expected, text)
		}
	}
}
