package appherder

import (
	"bytes"
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
	var picker iconPicker
	for _, entry := range entries {
		if !entry.IsDir() {
			picker.consider(entry.Name())
		}
	}
	return picker.best
}

func bestThemedIcon(fsys fs.FS) string {
	var picker iconPicker
	for _, dir := range []string{"usr/share/icons", "usr/share/pixmaps"} {
		_ = fs.WalkDir(fsys, dir, func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			picker.consider(name)
			return nil
		})
	}
	return picker.best
}

// iconPicker keeps the best icon seen, preferring svg > png > xpm.
type iconPicker struct {
	best string
	rank int
}

func (p *iconPicker) consider(name string) {
	rank := iconRank(name)
	if rank < 0 || (p.best != "" && rank <= p.rank) {
		return
	}
	p.best, p.rank = name, rank
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

type iconInstall struct {
	source  string
	path    string
	content []byte
}

func (a App) installIcon(fsys fs.FS, icon string, appName string) (string, error) {
	preparedIcon, err := a.prepareIconInstall(fsys, icon, appName)
	if err != nil {
		return "", err
	}
	return a.installPreparedIcon(preparedIcon, appName)
}

func (a App) prepareIconInstall(fsys fs.FS, icon string, appName string) (iconInstall, error) {
	content, err := fs.ReadFile(fsys, icon)
	if err != nil {
		return iconInstall{}, fmt.Errorf("read icon %s: %w", icon, err)
	}
	ext := iconExt(icon, content)
	if ext == "" {
		return iconInstall{}, fmt.Errorf("unsupported icon format %s", icon)
	}
	return iconInstall{
		source:  icon,
		path:    filepath.Join(a.iconsDir, appName+ext),
		content: content,
	}, nil
}

func (a App) installPreparedIcon(preparedIcon iconInstall, appName string) (string, error) {
	if err := os.MkdirAll(a.iconsDir, 0o755); err != nil {
		return "", fmt.Errorf("create icon directory %s: %w", a.iconsDir, err)
	}
	if err := writeIfChanged(preparedIcon.path, 0o644, preparedIcon.content); err != nil {
		return "", fmt.Errorf("install icon %s to %s: %w", preparedIcon.source, preparedIcon.path, err)
	}
	for _, path := range a.installedIconPaths(appName) {
		if path != preparedIcon.path {
			_ = os.Remove(path)
		}
	}
	_ = os.Remove(filepath.Join(a.iconsDir, appName))
	return preparedIcon.path, nil
}

func iconExt(name string, content []byte) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".svg":
		return ".svg"
	case ".png":
		return ".png"
	case ".xpm":
		return ".xpm"
	}
	if bytes.HasPrefix(content, []byte("\x89PNG\r\n\x1a\n")) {
		return ".png"
	}
	head := bytes.ToLower(bytes.TrimSpace(content[:min(len(content), 1024)]))
	if bytes.Contains(head, []byte("<svg")) {
		return ".svg"
	}
	if bytes.HasPrefix(head, []byte("/* xpm */")) {
		return ".xpm"
	}
	return ""
}
