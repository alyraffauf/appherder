package appherder

import (
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/alyraffauf/goxdgdesktop/desktopfile"
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

func TestPatchDesktopFilePreservesDesktopActions(t *testing.T) {
	app, home := newTestApp(t)

	desktop := desktopfile.Parse([]byte(sampleDesktopFile))
	iconPath := filepath.Join(home, "AppImages", ".icons", "example.png")
	if err := app.patchDesktopFile(desktop, "example", iconPath); err != nil {
		t.Fatal(err)
	}

	assertDesktopValue(t, desktop, desktopEntrySection, desktopOwnerKey, "true")
	assertDesktopValue(t, desktop, desktopEntrySection, "Icon", iconPath)
	assertDesktopValue(t, desktop, desktopEntrySection, "TryExec", filepath.Join(home, "AppImages", "example.appimage"))
	assertDesktopExec(t, desktop, desktopEntrySection, []string{"env", "FOO=bar", "DESKTOPINTEGRATION=1", filepath.Join(home, "AppImages", "example.appimage"), "%U"})
	assertDesktopExec(t, desktop, "Desktop Action new-window", []string{"env", "DESKTOPINTEGRATION=1", filepath.Join(home, "AppImages", "example.appimage"), "--new-window", "%U"})
	assertDesktopExec(t, desktop, "Desktop Action new-private-window", []string{"env", "DESKTOPINTEGRATION=1", filepath.Join(home, "AppImages", "example.appimage"), "--private-window", "%U"})
	assertDesktopValue(t, desktop, "Desktop Action new-private-window", "Name", "New Private Window")
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
		desktop      *desktopfile.File
		desktopName  string
		appimagePath string
		want         string
	}{
		{"Name with hyphen", desktopfile.Parse([]byte("[Desktop Entry]\nName=ES-DE\n")), "org.es_de.frontend.desktop", "", "esde"},
		{"Name with spaces", desktopfile.Parse([]byte("[Desktop Entry]\nName=Visual Studio Code\n")), "code.desktop", "", "visual_studio_code"},
		{"Name lowercased", desktopfile.Parse([]byte("[Desktop Entry]\nName=Krita\n")), "krita.desktop", "", "krita"},
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
	app, home := newTestApp(t)
	desktop := desktopfile.Parse([]byte(
		"[Desktop Entry]\nType=Application\nName=Foo\nTerminal=true\n",
	))
	if err := app.patchDesktopFile(desktop, "foo", ""); err != nil {
		t.Fatal(err)
	}
	assertDesktopValue(t, desktop, desktopEntrySection, "Terminal", "true")
	assertDesktopValue(t, desktop, desktopEntrySection, "TryExec", filepath.Join(home, "AppImages", "foo.appimage"))
	assertDesktopValue(t, desktop, desktopEntrySection, "Exec", filepath.Join(home, "AppImages", "foo.appimage"))
	assertDesktopValue(t, desktop, desktopEntrySection, desktopOwnerKey, "true")
}

func assertDesktopValue(t *testing.T, desktop *desktopfile.File, section string, key string, want string) {
	t.Helper()

	got, ok := desktop.Get(section, key)
	if !ok {
		t.Fatalf("missing %s in section %s", key, section)
	}
	if got != want {
		t.Fatalf("%s/%s = %q, want %q", section, key, got, want)
	}
}

func assertDesktopExec(t *testing.T, desktop *desktopfile.File, section string, want []string) {
	t.Helper()

	execCmd, ok := desktop.Get(section, "Exec")
	if !ok {
		t.Fatalf("missing Exec in section %s", section)
	}
	assertTokens(t, mustSplit(t, execCmd), want)
}
