package appherder

import (
	"fmt"
	"os"
	"path/filepath"
)

func (a App) localBinDir() string {
	return filepath.Join(filepath.Dir(filepath.Dir(a.applicationsDir)), "bin")
}

func (a App) linkPath(appName string) string {
	return filepath.Join(a.localBinDir(), appName)
}

func (a App) appImagePath(appName string) string {
	return filepath.Join(a.appimagesDir, appName+".appimage")
}

func (a App) Link(appName string) error {
	appName = NormalizeAppName(appName)
	src := a.appImagePath(appName)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("%s is not installed", appName)
	}
	binDir := a.localBinDir()
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", binDir, err)
	}
	dst := a.linkPath(appName)
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing link %s: %w", dst, err)
	}
	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("link %s -> %s: %w", dst, src, err)
	}
	return nil
}

func (a App) Unlink(appName string) error {
	appName = NormalizeAppName(appName)
	dst := a.linkPath(appName)
	if err := os.Remove(dst); err != nil {
		return fmt.Errorf("remove link %s: %w", dst, err)
	}
	return nil
}
