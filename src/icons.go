package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func resolveIcon(extracted string) string {
	icon := filepath.Join(extracted, ".DirIcon")
	if _, err := os.Stat(icon); err == nil {
		return icon
	}
	return ""
}

func (a app) installIcon(icon string, appName string) (string, error) {
	home, err := a.homeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	iconsDir := filepath.Join(home, "AppImages", ".icons")
	if err := os.MkdirAll(iconsDir, 0o755); err != nil {
		return "", fmt.Errorf("create icon directory %s: %w", iconsDir, err)
	}

	dest := filepath.Join(iconsDir, appName)
	if err := copyFile(icon, dest); err != nil {
		return "", fmt.Errorf("install icon %s to %s: %w", icon, dest, err)
	}
	return dest, nil
}
