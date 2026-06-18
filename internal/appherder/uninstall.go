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
	home, err := a.homeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	for _, path := range installedPaths(home, appName) {
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

func installedPaths(home string, appName string) []string {
	return []string{
		filepath.Join(home, "AppImages", appName+".appimage"),
		filepath.Join(home, "AppImages", ".icons", appName),
		filepath.Join(home, ".local", "share", "applications", appName+".desktop"),
	}
}
