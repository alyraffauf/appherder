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

	if icon != "" {
		if _, err := a.installIcon(icon, appName); err != nil {
			return err
		}
	}

	if _, err := a.installAppImage(appimage, appName); err != nil {
		return err
	}

	if desktop != nil {
		if err := a.patchDesktopFile(desktop, appName); err != nil {
			return err
		}
		if _, err := a.installDesktopFile(desktop, appName); err != nil {
			return err
		}
	}

	return nil
}

func appNameFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
