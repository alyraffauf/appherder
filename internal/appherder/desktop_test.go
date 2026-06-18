package appherder

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

	a := App{homeDir: func() (string, error) { return "/home/test", nil }}

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
	tests := []struct {
		name         string
		desktop      *desktopFile
		desktopName  string
		appimagePath string
		want         string
	}{
		{"Name with hyphen", parseDesktopFile([]byte("[Desktop Entry]\nName=ES-DE\n")), "org.es_de.frontend.desktop", "", "esde"},
		{"Name with spaces", parseDesktopFile([]byte("[Desktop Entry]\nName=Visual Studio Code\n")), "code.desktop", "", "visual_studio_code"},
		{"Name lowercased", parseDesktopFile([]byte("[Desktop Entry]\nName=Krita\n")), "krita.desktop", "", "krita"},
		{"desktop id fallback", nil, "org.kde.krita.desktop", "", "org.kde.krita"},
		{"filename fallback", nil, "", "/dl/Krita-5.2.0-x86_64.AppImage", "krita5.2.0x86_64"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveAppName(tc.desktop, tc.desktopName, tc.appimagePath); got != tc.want {
				t.Fatalf("deriveAppName = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPatchDesktopFileSetsExecWhenMissing(t *testing.T) {
	a := App{homeDir: func() (string, error) { return "/home/test", nil }}
	desktop := parseDesktopFile([]byte(
		"[Desktop Entry]\nType=Application\nName=Foo\nTerminal=true\n",
	))
	if err := a.patchDesktopFile(desktop, "foo", false); err != nil {
		t.Fatal(err)
	}
	assertDesktopValue(t, desktop, desktopEntrySection, "Terminal", "true")
	assertDesktopValue(t, desktop, desktopEntrySection, "TryExec", "/home/test/AppImages/foo.appimage")
	assertDesktopValue(t, desktop, desktopEntrySection, "Exec", "/home/test/AppImages/foo.appimage")
	assertDesktopValue(t, desktop, desktopEntrySection, desktopOwnerKey, "true")
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
