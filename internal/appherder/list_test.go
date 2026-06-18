package appherder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListShowsInstalledAndOrphaned(t *testing.T) {
	a, home := newTestApp(t)
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

	infos, err := a.List()
	if err != nil {
		t.Fatal(err)
	}

	byName := make(map[string]AppInfo, len(infos))
	for _, info := range infos {
		byName[info.Name] = info
	}

	if info, ok := byName["Present App"]; !ok {
		t.Fatalf("list missing Present App: %+v", infos)
	} else {
		if info.Filename != "present.appimage" {
			t.Fatalf("filename = %q, want present.appimage", info.Filename)
		}
		if info.Version != "1.2.3" {
			t.Fatalf("version = %q, want 1.2.3", info.Version)
		}
	}

	if info, ok := byName["gone"]; !ok {
		t.Fatalf("list missing gone: %+v", infos)
	} else if info.Filename != "" {
		t.Fatalf("gone filename = %q, want empty (orphaned)", info.Filename)
	}

	if _, ok := byName["Other"]; ok {
		t.Fatal("unmanaged app appeared in list")
	}
}

func TestListSignatureStatus(t *testing.T) {
	a, home := newTestApp(t)
	appimages := filepath.Join(home, "AppImages")
	appsDir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(appimages, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	layout := buildSignableELF(2048, 4096)
	sig, key := signWith(t, layout, newTestEntity(t))
	signedImage := layout.embed(sig, key)
	unsignedImage := buildSignableELF(2048, 4096).bytes

	write := func(name string, image []byte, desktop string) {
		if err := os.WriteFile(filepath.Join(appimages, name+".appimage"), image, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(appsDir, name+".desktop"), []byte(desktop), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("signed", signedImage, "[Desktop Entry]\nName=Signed\n"+desktopOwnerKey+"=true\n")
	write("pinned", signedImage, "[Desktop Entry]\nName=Pinned\n"+desktopOwnerKey+"=true\n"+desktopSigningKey+"=ABC123\n")
	write("plain", unsignedImage, "[Desktop Entry]\nName=Plain\n"+desktopOwnerKey+"=true\n")

	infos, err := a.List()
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]string, len(infos))
	for _, info := range infos {
		got[info.Name] = info.Signature
	}
	for name, want := range map[string]string{"Signed": "signed", "Pinned": "pinned", "Plain": "none"} {
		if got[name] != want {
			t.Errorf("%s signature = %q, want %q", name, got[name], want)
		}
	}
}

func TestListEmptyWhenNothingManaged(t *testing.T) {
	a, _ := newTestApp(t)
	infos, err := a.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 0 {
		t.Fatalf("expected no managed apps, got %d: %+v", len(infos), infos)
	}
}

func TestListFallsBackToFilenameForName(t *testing.T) {
	a, home := newTestApp(t)
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

	infos, err := a.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 || infos[0].Name != "noname.appimage" {
		t.Fatalf("expected filename fallback, got: %+v", infos)
	}
}
