package appherder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a App) linkPath(appName string) string {
	return filepath.Join(a.binDir, appName)
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
	if err := os.MkdirAll(a.binDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", a.binDir, err)
	}
	dst := a.linkPath(appName)

	if target, err := os.Readlink(dst); err == nil {
		if target == src {
			return nil
		}
		fmt.Fprintf(os.Stderr, "Warning: %s is already linked to a different target\n", dst)
		return nil
	}
	if _, err := os.Lstat(dst); err == nil {
		fmt.Fprintf(os.Stderr, "Warning: %s already exists, skipping\n", dst)
		return nil
	}

	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("link %s -> %s: %w", dst, src, err)
	}
	return nil
}

func (a App) Unlink(appName string) error {
	appName = NormalizeAppName(appName)
	dst := a.linkPath(appName)

	target, err := os.Readlink(dst)
	if err != nil {
		return fmt.Errorf("%s is not a link managed by appherder", dst)
	}
	if !strings.HasPrefix(target, a.appimagesDir+string(filepath.Separator)) {
		return fmt.Errorf("%s is not a link managed by appherder", dst)
	}

	if err := os.Remove(dst); err != nil {
		return fmt.Errorf("remove link %s: %w", dst, err)
	}
	return nil
}
