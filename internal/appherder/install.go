package appherder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alyraffauf/goxdgdesktop/desktopfile"
)

func (a App) Install(appimage string) (string, error) {
	return a.install(context.Background(), appimage, expectedChecksum{})
}

func (a App) install(ctx context.Context, appimage string, want expectedChecksum) (appName string, err error) {
	appimage, err = filepath.Abs(appimage)
	if err != nil {
		return "", fmt.Errorf("resolve AppImage path %q: %w", appimage, err)
	}

	// Verify before openAppImage: the DwarFS fallback executes the AppImage to
	// extract it, so bad images must be refused first. Pinned-key check is
	// deferred until the app name is known from the desktop file inside.
	fingerprint, err := verifyAppImage(appimage, "", want)
	if err != nil {
		return "", err
	}

	fsys, closeAppImage, err := openAppImage(ctx, appimage)
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

	// Pinned-key check deferred from above; app name now known.
	pinned := a.pinnedSigningKey(appName)
	if pinned != "" {
		if fingerprint == "" {
			return "", fmt.Errorf("refusing unsigned AppImage: a signing key is pinned (%s); uninstall and reinstall to trust a different build", pinned)
		}
		if !strings.EqualFold(pinned, fingerprint) {
			return "", fmt.Errorf("refusing AppImage: signing key changed (pinned %s, got %s); uninstall and reinstall to trust the new key", pinned, fingerprint)
		}
	}
	pin := fingerprint

	// No desktop file inside the AppImage: synthesize a terminal launcher so
	// CLI apps still get a menu entry and are tracked by managedApps.
	if desktop == nil {
		desktop = desktopfile.Parse([]byte(fmt.Sprintf(
			"[Desktop Entry]\nType=Application\nName=%s\nTerminal=true\n",
			appNameFromPath(appimage),
		)))
	}

	// Patch in memory before any filesystem writes so a failure here installs nothing.
	if err := a.patchDesktopFile(desktop, appName, icon != ""); err != nil {
		return "", err
	}
	if pin != "" {
		desktop.Set(desktopEntrySection, desktopSigningKey, pin)
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

	dest, err := a.installDesktopFile(desktop, appName)
	if err != nil {
		rollback()
		return "", err
	}
	installed = append(installed, dest)

	// Materialize the AppImage last: when the source is already in ~/AppImages
	// it gets moved, so an earlier failure must not roll back over the user's
	// file.
	if _, err := a.installAppImage(appimage, appName); err != nil {
		rollback()
		return "", err
	}

	if isTerminalApp(desktop) {
		_ = a.Link(appName)
	}

	return appName, nil
}

// InstallFromURL downloads an AppImage from url and installs it.
func (a App) InstallFromURL(ctx context.Context, url string) (string, error) {
	tmpName, err := downloadToTemp(ctx, url, "appherder-install", a.progress, url)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpName)
	return a.install(ctx, tmpName, expectedChecksum{})
}

// deriveAppName picks the canonical install name: the desktop entry's Name
// field (e.g. "ES-DE" -> "esde"), then the desktop-file id, then the source
// filename.
func deriveAppName(desktop *desktopfile.File, desktopName string, appimagePath string) string {
	if desktop != nil {
		if name, ok := desktop.Get(desktopEntrySection, "Name"); ok && name != "" {
			return sanitizeAppName(name)
		}
	}
	if name := strings.TrimSuffix(desktopName, ".desktop"); name != "" {
		return sanitizeAppName(name)
	}
	return sanitizeAppName(appNameFromPath(appimagePath))
}

// sanitizeAppName lowercases s, turns spaces into underscores, and drops any
// character that isn't alphanumeric, underscore, or dot.
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

func isTerminalApp(desktop *desktopfile.File) bool {
	if desktop == nil {
		return false
	}
	val, ok := desktop.Get(desktopEntrySection, "Terminal")
	return ok && strings.EqualFold(val, "true")
}

// NormalizeAppName strips directory and .appimage extension from name.
func NormalizeAppName(name string) string {
	name = filepath.Base(name)
	if ext := filepath.Ext(name); strings.EqualFold(ext, ".appimage") {
		name = strings.TrimSuffix(name, ext)
	}
	return name
}
