package appherder

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/alyraffauf/goxdgdesktop/desktopfile"
)

const (
	desktopEntrySection = desktopfile.EntrySection
	// desktopOwnerKey marks launchers appherder owns.
	desktopOwnerKey = "X-AppHerder"
)

// findDesktopFile returns the AppImage's desktop entry and filename.
func findDesktopFile(fsys fs.FS) (*desktopfile.File, string, error) {
	return desktopfile.FirstInFS(fsys, "*.desktop", "default.desktop")
}

// isManagedDesktop reports whether path is owned by appherder.
func isManagedDesktop(path string) (bool, error) {
	desktop, err := desktopfile.Read(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read desktop file %s: %w", path, err)
	}
	value, ok := desktop.Get(desktopEntrySection, desktopOwnerKey)
	return ok && value == "true", nil
}

// managedApps returns the app IDs of launchers appherder installed.
func managedApps(applicationsDir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(applicationsDir, "*.desktop"))
	if err != nil {
		return nil, err
	}

	var apps []string
	for _, path := range matches {
		managed, err := isManagedDesktop(path)
		if err != nil {
			return nil, err
		}
		if managed {
			apps = append(apps, strings.TrimSuffix(filepath.Base(path), ".desktop"))
		}
	}
	return apps, nil
}

func (a App) patchDesktopFile(desktop *desktopfile.File, appName string, iconPath string) error {
	appimage := filepath.Join(a.appimagesDir, appName+".appimage")

	desktop.Set(desktopEntrySection, desktopOwnerKey, "true")
	if iconPath != "" {
		desktop.Set(desktopEntrySection, "Icon", iconPath)
	}
	desktop.Set(desktopEntrySection, "TryExec", appimage)

	if execCmd, ok := desktop.Get(desktopEntrySection, "Exec"); ok {
		desktop.Set(desktopEntrySection, "Exec", patchExecCommand(execCmd, appimage))
	} else {
		desktop.Set(desktopEntrySection, "Exec", appimage)
	}

	for _, section := range desktop.Sections() {
		if !strings.HasPrefix(section.Name, desktopfile.ActionSectionStart) {
			continue
		}
		if execCmd, ok := section.Get("Exec"); ok {
			desktop.Set(section.Name, "Exec", patchExecCommand(execCmd, appimage))
		}
	}

	return nil
}

func (a App) installDesktopFile(desktop *desktopfile.File, appName string) (string, error) {
	dest := filepath.Join(a.applicationsDir, appName+".desktop")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("create desktop file directory %s: %w", filepath.Dir(dest), err)
	}
	if err := writeIfChanged(dest, 0o644, desktop.Bytes()); err != nil {
		return "", fmt.Errorf("write desktop file %s: %w", dest, err)
	}
	return dest, nil
}
