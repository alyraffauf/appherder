package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// resolveIcon prefers .DirIcon, then any icon at the AppImage root, then icons
// in the standard theme directories. It returns "" when none is found.
func resolveIcon(extracted string) string {
	dirIcon := filepath.Join(extracted, ".DirIcon")
	if info, err := os.Stat(dirIcon); err == nil && !info.IsDir() {
		return dirIcon
	}
	return findFallbackIcon(extracted)
}

func findFallbackIcon(extracted string) string {
	best := ""
	bestRank := -1
	var bestSize int64
	consider := func(path string, entry fs.DirEntry) {
		rank := iconRank(path)
		if rank < 0 {
			return
		}
		info, err := entry.Info()
		if err != nil {
			return
		}
		// Prefer the higher-ranked extension, then the larger (higher-res) file.
		if size := info.Size(); rank > bestRank || (rank == bestRank && size > bestSize) {
			best, bestRank, bestSize = path, rank, size
		}
	}

	if entries, err := os.ReadDir(extracted); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				consider(filepath.Join(extracted, entry.Name()), entry)
			}
		}
	}

	for _, dir := range []string{
		filepath.Join(extracted, "usr", "share", "icons"),
		filepath.Join(extracted, "usr", "share", "pixmaps"),
	} {
		_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			consider(path, entry)
			return nil
		})
	}

	return best
}

// iconRank ranks icon extensions (higher preferred); -1 means not an icon.
func iconRank(path string) int {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".svg":
		return 2
	case ".png":
		return 1
	case ".xpm":
		return 0
	default:
		return -1
	}
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
