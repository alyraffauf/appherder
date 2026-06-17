package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// resolveIcon prefers .DirIcon, then any icon at the AppImage root, then icons
// in the standard theme directories. It returns the in-archive path, or "" when
// none is found.
func resolveIcon(fsys fs.FS) string {
	if info, err := fs.Stat(fsys, ".DirIcon"); err == nil && !info.IsDir() {
		return ".DirIcon"
	}
	return findFallbackIcon(fsys)
}

func findFallbackIcon(fsys fs.FS) string {
	best := ""
	bestRank := -1
	var bestSize int64
	consider := func(name string, entry fs.DirEntry) {
		rank := iconRank(name)
		if rank < 0 {
			return
		}
		info, err := entry.Info()
		if err != nil {
			return
		}
		// Prefer the higher-ranked extension, then the larger (higher-res) file.
		if size := info.Size(); rank > bestRank || (rank == bestRank && size > bestSize) {
			best, bestRank, bestSize = name, rank, size
		}
	}

	if entries, err := fs.ReadDir(fsys, "."); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				consider(entry.Name(), entry)
			}
		}
	}

	for _, dir := range []string{"usr/share/icons", "usr/share/pixmaps"} {
		_ = fs.WalkDir(fsys, dir, func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			consider(name, entry)
			return nil
		})
	}

	return best
}

// iconRank ranks icon extensions (higher preferred); -1 means not an icon.
func iconRank(name string) int {
	switch strings.ToLower(path.Ext(name)) {
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

func (a app) installIcon(fsys fs.FS, icon string, appName string) (string, error) {
	home, err := a.homeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	iconsDir := filepath.Join(home, "AppImages", ".icons")
	if err := os.MkdirAll(iconsDir, 0o755); err != nil {
		return "", fmt.Errorf("create icon directory %s: %w", iconsDir, err)
	}

	dest := filepath.Join(iconsDir, appName)
	if err := copyFromFS(fsys, icon, dest); err != nil {
		return "", fmt.Errorf("install icon %s to %s: %w", icon, dest, err)
	}
	return dest, nil
}
