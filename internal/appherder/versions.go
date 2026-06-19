package appherder

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (a App) installAppImage(file string, appName string) (string, error) {
	if err := os.MkdirAll(a.appimagesDir, 0o755); err != nil {
		return "", fmt.Errorf("create AppImages directory %s: %w", a.appimagesDir, err)
	}

	dest := filepath.Join(a.appimagesDir, appName+".appimage")
	inFolder := samePath(filepath.Dir(file), a.appimagesDir)

	if !samePath(file, dest) {
		same, err := sameContent(file, dest)
		if err != nil {
			return "", fmt.Errorf("compare AppImage %s with %s: %w", file, dest, err)
		}
		switch {
		case same:
			if inFolder {
				if err := os.Remove(file); err != nil {
					return "", fmt.Errorf("remove duplicate AppImage %s: %w", file, err)
				}
			}
		case inFolder:
			if err := a.saveToVersions(dest, appName); err != nil {
				return "", err
			}
			if err := os.Rename(file, dest); err != nil {
				return "", fmt.Errorf("move AppImage %s to %s: %w", file, dest, err)
			}
		default:
			if err := a.saveToVersions(dest, appName); err != nil {
				return "", err
			}
			if err := writeAtomic(dest, 0o755, func(writer io.Writer) error {
				return copyTo(file, writer)
			}); err != nil {
				return "", fmt.Errorf("install AppImage %s to %s: %w", file, dest, err)
			}
		}
	}

	if err := os.Chmod(dest, 0o755); err != nil {
		return "", fmt.Errorf("make AppImage executable %s: %w", dest, err)
	}
	return dest, nil
}

// samePath reports whether a and b resolve to the same file, accounting for
// symlinks and mounts (e.g. /home -> /var/home).
func samePath(pathA, pathB string) bool {
	infoA, err := os.Stat(pathA)
	if err != nil {
		return false
	}
	infoB, err := os.Stat(pathB)
	if err != nil {
		return false
	}
	return os.SameFile(infoA, infoB)
}

func copyTo(src string, dest io.Writer) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source %s: %w", src, err)
	}
	defer in.Close()

	_, err = io.Copy(dest, in)
	return err
}

// saveToVersions hardlinks src into .versions/appName/<version>.appimage,
// pruning older versions to keep at most MaxSavedVersions from config.
func (a App) saveToVersions(src, appName string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}
	version := readAppImageVersion(src)
	versionsDir := filepath.Join(a.appimagesDir, ".versions", appName)
	if err := os.MkdirAll(versionsDir, 0o755); err != nil {
		return fmt.Errorf("create versions directory: %w", err)
	}

	a.pruneVersions(versionsDir, a.config.MaxSavedVersions-1)

	dest := filepath.Join(versionsDir, version+".appimage")
	os.Remove(dest)
	if err := os.Link(src, dest); err != nil {
		return fmt.Errorf("save current version %s: %w", version, err)
	}
	return nil
}

// pruneVersions removes the oldest saved versions when the directory holds
// more than keep files, sorting by mtime.
func (a App) pruneVersions(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var files []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".appimage") {
			files = append(files, entry)
		}
	}
	if len(files) <= keep {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		infoI, errI := files[i].Info()
		infoJ, errJ := files[j].Info()
		if errI != nil || errJ != nil {
			return false
		}
		return infoI.ModTime().Before(infoJ.ModTime())
	})
	for _, entry := range files[:len(files)-keep] {
		os.Remove(filepath.Join(dir, entry.Name()))
	}
}

// readAppImageVersion returns the embedded version string from the AppImage's
// desktop file, falling back to the file's mtime.
func readAppImageVersion(path string) string {
	if fsys, closeFs, err := openAppImage(context.Background(), path); err == nil {
		defer closeFs()
		if desktop, _, err := findDesktopFile(fsys); err == nil && desktop != nil {
			if version, ok := desktop.Get(desktopEntrySection, "X-AppImage-Version"); ok && version != "" {
				return sanitizeVersionForFilename(version)
			}
		}
	}
	if info, err := os.Stat(path); err == nil {
		return info.ModTime().UTC().Format("2006-01-02T150405")
	}
	return "unknown"
}

// sanitizeVersionForFilename replaces path separators so the version string is
// safe for a filename.
func sanitizeVersionForFilename(version string) string {
	version = strings.ReplaceAll(version, "/", "_")
	version = strings.ReplaceAll(version, "\\", "_")
	if len(version) > 100 {
		version = version[:100]
	}
	return version
}
