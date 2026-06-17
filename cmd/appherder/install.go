package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a app) install(appimage string) (err error) {
	appimage, err = filepath.Abs(appimage)
	if err != nil {
		return fmt.Errorf("resolve AppImage path %q: %w", appimage, err)
	}

	fsys, closeAppImage, err := openAppImage(appimage)
	if err != nil {
		return err
	}
	defer closeAppImage()

	// The squashfs reader parses untrusted input; turn any panic into an error.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("read AppImage %s: %v", appimage, r)
		}
	}()

	desktop, desktopName, err := findDesktopFile(fsys)
	if err != nil {
		return fmt.Errorf("find desktop file: %w", err)
	}

	icon := resolveIcon(fsys)
	appName := deriveAppName(desktopName, appimage)

	// Patch in memory before any filesystem writes so a failure here installs nothing.
	if desktop != nil {
		if err := a.patchDesktopFile(desktop, appName, icon != ""); err != nil {
			return err
		}
	}

	// Roll back written files on a later failure rather than leaving a half-installed app.
	var installed []string
	rollback := func() {
		for _, path := range installed {
			_ = os.Remove(path)
		}
	}

	if icon != "" {
		dest, err := a.installIcon(fsys, icon, appName)
		if err != nil {
			rollback()
			return err
		}
		installed = append(installed, dest)
	}

	dest, err := a.installAppImage(appimage, appName)
	if err != nil {
		rollback()
		return err
	}
	installed = append(installed, dest)

	if desktop != nil {
		dest, err := a.installDesktopFile(desktop, appName)
		if err != nil {
			rollback()
			return err
		}
		installed = append(installed, dest)
	}

	return nil
}

// deriveAppName prefers the AppImage's desktop-file id, which is stable across
// versions, and falls back to the source filename when no desktop entry ships.
func deriveAppName(desktopName string, appimagePath string) string {
	if name := strings.TrimSuffix(desktopName, ".desktop"); name != "" {
		return name
	}
	return appNameFromPath(appimagePath)
}

func appNameFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
