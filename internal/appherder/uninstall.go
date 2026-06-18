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
			// Leave launchers we didn't install (a name clash, or a pre-marker
			// install); --force overrides this.
			if !managed {
				continue
			}
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}

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
