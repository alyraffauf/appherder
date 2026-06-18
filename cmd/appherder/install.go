package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a app) install(appimage string) (appName string, err error) {
	appimage, err = filepath.Abs(appimage)
	if err != nil {
		return "", fmt.Errorf("resolve AppImage path %q: %w", appimage, err)
	}

	fsys, closeAppImage, err := openAppImage(appimage)
	if err != nil {
		return "", err
	}
	defer closeAppImage()

	// The squashfs reader parses untrusted input; turn any panic into an error.
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("read AppImage %s: %v", appimage, recovered)
		}
	}()

	desktop, desktopName, err := findDesktopFile(fsys)
	if err != nil {
		return "", fmt.Errorf("find desktop file: %w", err)
	}

	icon := resolveIcon(fsys)
	appName = deriveAppName(desktop, desktopName, appimage)

	// Patch in memory before any filesystem writes so a failure here installs nothing.
	if desktop != nil {
		if err := a.patchDesktopFile(desktop, appName, icon != ""); err != nil {
			return "", err
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
			return "", err
		}
		installed = append(installed, dest)
	}

	if desktop != nil {
		dest, err := a.installDesktopFile(desktop, appName)
		if err != nil {
			rollback()
			return "", err
		}
		installed = append(installed, dest)
	}

	// Materialize the AppImage last: when the source is already in ~/AppImages
	// it gets moved, so an earlier failure must not roll back over the user's
	// file.
	if _, err := a.installAppImage(appimage, appName); err != nil {
		rollback()
		return "", err
	}

	return appName, nil
}

// deriveAppName picks the canonical install name, matching GearLever so the
// two tools land at the same path: the desktop entry's Name field (e.g.
// "ES-DE" -> "esde"), then the desktop-file id, then the source filename.
func deriveAppName(desktop *desktopFile, desktopName string, appimagePath string) string {
	if desktop != nil {
		if name, ok := desktop.get("Name", desktopEntrySection); ok && name != "" {
			return sanitizeAppName(name)
		}
	}
	if name := strings.TrimSuffix(desktopName, ".desktop"); name != "" {
		return sanitizeAppName(name)
	}
	return sanitizeAppName(appNameFromPath(appimagePath))
}

// sanitizeAppName lowercases s, turns spaces into underscores, and drops any
// character that isn't alphanumeric, underscore, or dot — GearLever's naming
// rule.
func sanitizeAppName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	var builder strings.Builder
	builder.Grow(len(name))
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '.' {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func appNameFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
