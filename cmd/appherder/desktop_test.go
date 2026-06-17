package main

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

const sampleDesktopFile = `# leading comment
[Desktop Entry]
Name=Example App
Exec=env FOO=bar upstream-app %U
Icon=example-app
Actions=new-window;new-private-window;

[Desktop Action new-window]
Name=New Window
Exec=upstream-app --new-window %U

[Desktop Action new-private-window]
Name=New Private Window
Exec=upstream-app --private-window %U
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
	if err := a.patchDesktopFile(desktop, "example", true); err != nil {
		t.Fatal(err)
	}
	if err := desktop.write(output); err != nil {
		t.Fatal(err)
	}

	patched, err := readDesktopFile(output)
	if err != nil {
		t.Fatal(err)
	}

	assertDesktopValue(t, patched, desktopEntrySection, desktopOwnerKey, "true")
	assertDesktopValue(t, patched, desktopEntrySection, "Icon", "/home/test/AppImages/.icons/example")
	assertDesktopValue(t, patched, desktopEntrySection, "TryExec", "/home/test/AppImages/example.appimage")
	assertDesktopExec(t, patched, desktopEntrySection, []string{"env", "FOO=bar", "DESKTOPINTEGRATION=1", "/home/test/AppImages/example.appimage", "%U"})
	assertDesktopExec(t, patched, "Desktop Action new-window", []string{"env", "DESKTOPINTEGRATION=1", "/home/test/AppImages/example.appimage", "--new-window", "%U"})
	assertDesktopExec(t, patched, "Desktop Action new-private-window", []string{"env", "DESKTOPINTEGRATION=1", "/home/test/AppImages/example.appimage", "--private-window", "%U"})
	assertDesktopValue(t, patched, "Desktop Action new-private-window", "Name", "New Private Window")
}

func TestFindDesktopFileSkipsDefault(t *testing.T) {
	fsys := fstest.MapFS{
		"default.desktop": {Data: []byte("[Desktop Entry]\nName=Default\n")},
		"app.desktop":     {Data: []byte("[Desktop Entry]\nName=App\n")},
	}
	desktop, name, err := findDesktopFile(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if desktop == nil {
		t.Fatal("expected a desktop file")
	}
	if name != "app.desktop" {
		t.Fatalf("desktop name = %q, want app.desktop", name)
	}
	assertDesktopValue(t, desktop, desktopEntrySection, "Name", "App")
}

func TestFindDesktopFileReturnsNilWhenOnlyDefault(t *testing.T) {
	fsys := fstest.MapFS{
		"default.desktop": {Data: []byte("[Desktop Entry]\nName=Default\n")},
	}
	desktop, name, err := findDesktopFile(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if desktop != nil || name != "" {
		t.Fatalf("expected no desktop file, got %v %q", desktop, name)
	}
}

func TestDeriveAppName(t *testing.T) {
	if got := deriveAppName("org.kde.krita.desktop", "/dl/krita-5.2.0-x86_64.appimage"); got != "org.kde.krita" {
		t.Fatalf("deriveAppName with desktop = %q, want org.kde.krita", got)
	}
	if got := deriveAppName("", "/dl/Krita-5.2.0-x86_64.AppImage"); got != "Krita-5.2.0-x86_64" {
		t.Fatalf("deriveAppName fallback = %q, want Krita-5.2.0-x86_64", got)
	}
}

func assertDesktopValue(t *testing.T, desktop *desktopFile, section string, key string, want string) {
	t.Helper()

	got, ok := desktop.get(key, section)
	if !ok {
		t.Fatalf("missing %s in section %s", key, section)
	}
	if got != want {
		t.Fatalf("%s/%s = %q, want %q", section, key, got, want)
	}
}

func assertDesktopExec(t *testing.T, desktop *desktopFile, section string, want []string) {
	t.Helper()

	execCmd, ok := desktop.get("Exec", section)
	if !ok {
		t.Fatalf("missing Exec in section %s", section)
	}
	assertTokens(t, mustSplit(t, execCmd), want)
}
