package appherder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a App) Uninstall(name string, force bool) error {
	appName := NormalizeAppName(name)

	for _, path := range a.installedPaths(appName) {
		if strings.HasSuffix(path, ".desktop") && !force {
			managed, err := isManagedDesktop(path)
			if err != nil {
				return err
			}
			if !managed {
				continue
			}
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}

	_ = os.RemoveAll(filepath.Join(a.appimagesDir, ".versions", appName))

	return nil
}

// NormalizeAppName strips directory and .appimage extension from name.
func NormalizeAppName(name string) string {
	name = filepath.Base(name)
	if ext := filepath.Ext(name); strings.EqualFold(ext, ".appimage") {
		name = strings.TrimSuffix(name, ext)
	}
	return name
}

func (a App) installedPaths(appName string) []string {
	return []string{
		filepath.Join(a.appimagesDir, appName+".appimage"),
		filepath.Join(a.iconsDir, appName),
		filepath.Join(a.applicationsDir, appName+".desktop"),
	}
}
