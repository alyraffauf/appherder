package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a app) uninstall(name string) error {
	appName := normalizeAppName(name)
	home, err := a.homeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	for _, path := range installedPaths(home, appName) {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}

	return nil
}

func normalizeAppName(name string) string {
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
