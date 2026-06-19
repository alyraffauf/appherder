package appherder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Rollback restores a saved version of the app. When version is "", it picks
// the most recently saved one (by mtime).
func (a App) Rollback(appName string, version string) error {
	appName = NormalizeAppName(appName)
	versionsDir := filepath.Join(a.appimagesDir, ".versions", appName)
	current := filepath.Join(a.appimagesDir, appName+".appimage")

	if version == "" {
		var err error
		version, err = newestVersion(versionsDir)
		if err != nil {
			return err
		}
	}

	saved := filepath.Join(versionsDir, version+".appimage")
	if _, err := os.Stat(saved); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("version %q not found for %s", version, appName)
		}
		return fmt.Errorf("check saved version %q: %w", version, err)
	}

	// Track where the current binary was moved so we can put it back if the
	// second rename fails.
	var movedCurrentTo string
	if _, err := os.Stat(current); err == nil {
		currentVersion := readAppImageVersion(current)
		currentSaved := filepath.Join(versionsDir, currentVersion+".appimage")
		if currentSaved != saved {
			if err := os.Rename(current, currentSaved); err != nil {
				return fmt.Errorf("save current version before rollback: %w", err)
			}
			movedCurrentTo = currentSaved
		}
	}

	if err := os.Rename(saved, current); err != nil {
		if movedCurrentTo != "" {
			os.Rename(movedCurrentTo, current) //nolint:errcheck
		}
		return fmt.Errorf("restore version %s: %w", version, err)
	}
	if err := os.Chmod(current, 0o755); err != nil {
		return fmt.Errorf("make AppImage executable: %w", err)
	}
	return nil
}

// newestVersion returns the name (without .appimage) of the most recently
// modified saved version.
func newestVersion(versionsDir string) (string, error) {
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no saved versions in %s", filepath.Dir(versionsDir))
		}
		return "", err
	}
	var newest string
	var newestTime time.Time
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".appimage") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newestTime) {
			newest = entry.Name()
			newestTime = info.ModTime()
		}
	}
	if newest == "" {
		return "", fmt.Errorf("no saved versions in %s", filepath.Dir(versionsDir))
	}
	return strings.TrimSuffix(newest, ".appimage"), nil
}
