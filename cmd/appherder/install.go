package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a app) install(appimage string) error {
	appimage, err := filepath.Abs(appimage)
	if err != nil {
		return fmt.Errorf("resolve AppImage path %q: %w", appimage, err)
	}

	tmp, err := os.MkdirTemp("", "appherder-")
	if err != nil {
		return fmt.Errorf("create extraction directory: %w", err)
	}
	defer os.RemoveAll(tmp)

	extracted, err := a.extractAppImage(appimage, tmp)
	if err != nil {
		return err
	}

	desktop, err := findDesktopFile(extracted)
	if err != nil {
		return fmt.Errorf("find desktop file in %s: %w", extracted, err)
	}

	icon := resolveIcon(extracted)
	appName := appNameFromPath(appimage)

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
		dest, err := a.installIcon(icon, appName)
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

func appNameFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
