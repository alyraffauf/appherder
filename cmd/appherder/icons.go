package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// resolveIcon picks the AppImage's icon by location first: .DirIcon, then any
// icon at the root, then the themed directories. Format (svg > png > xpm) and
// size only break ties within a tier. It returns "" when none is found.
func resolveIcon(fsys fs.FS) string {
	if info, err := fs.Stat(fsys, ".DirIcon"); err == nil && !info.IsDir() {
		return ".DirIcon"
	}
	if icon := bestRootIcon(fsys); icon != "" {
		return icon
	}
	return bestThemedIcon(fsys)
}

func bestRootIcon(fsys fs.FS) string {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return ""
	}
	var p iconPicker
	for _, entry := range entries {
		if !entry.IsDir() {
			p.consider(entry.Name(), entry)
		}
	}
	return p.best
}

func bestThemedIcon(fsys fs.FS) string {
	var p iconPicker
	for _, dir := range []string{"usr/share/icons", "usr/share/pixmaps"} {
		_ = fs.WalkDir(fsys, dir, func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			p.consider(name, entry)
			return nil
		})
	}
	return p.best
}

// iconPicker keeps the best icon seen, preferring format rank then larger size.
type iconPicker struct {
	best string
	rank int
	size int64
}

func (p *iconPicker) consider(name string, entry fs.DirEntry) {
	rank := iconRank(name)
	if rank < 0 {
		return
	}
	info, err := entry.Info()
	if err != nil {
		return
	}
	if p.best == "" || rank > p.rank || (rank == p.rank && info.Size() > p.size) {
		p.best, p.rank, p.size = name, rank, info.Size()
	}
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
