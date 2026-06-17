package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// sync reconciles ~/AppImages with installed state: install every AppImage in
// the folder, remove launchers whose AppImage is gone. With force, also remove
// unmanaged launchers whose TryExec/Exec points at a missing file in ~/AppImages
// (entries left by another tool). Per-file errors are reported and skipped so
// one bad file does not abort the pass.
func (a app) sync(out io.Writer, force bool) error {
	home, err := a.homeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	appimagesDir := filepath.Join(home, "AppImages")

	files, err := listAppImages(appimagesDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if err := a.install(f); err != nil {
			fmt.Fprintf(out, "skip %s: %v\n", filepath.Base(f), err)
			continue
		}
	}

	candidates, err := managedApps(home)
	if err != nil {
		return err
	}
	if force {
		extra, err := appImageBackedOrphans(home, appimagesDir)
		if err != nil {
			return err
		}
		candidates = append(candidates, extra...)
	}
	for _, appid := range candidates {
		present, err := appImagePresent(appimagesDir, appid)
		if err != nil {
			return err
		}
		if present {
			continue
		}
		if err := a.uninstall(appid, force); err != nil {
			fmt.Fprintf(out, "skip remove %s: %v\n", appid, err)
			continue
		}
		fmt.Fprintf(out, "removed %s\n", appid)
	}
	return nil
}

// listAppImages returns *.appimage files in dir, case-insensitive (the
// AppImage spec uses .AppImage, but .appimage is common).
func listAppImages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if !strings.EqualFold(filepath.Ext(e.Name()), ".appimage") {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	return files, nil
}

// appImagePresent reports whether <appid>.appimage exists in dir, matching the
// extension case-insensitively.
func appImagePresent(dir, appid string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(e.Name(), appid+".appimage") {
			return true, nil
		}
	}
	return false, nil
}

// appImageBackedOrphans returns appids of unmanaged desktop entries whose
// TryExec or Exec points at a missing file inside appimagesDir — launchers left
// by another tool after their AppImage was deleted.
func appImageBackedOrphans(home, appimagesDir string) ([]string, error) {
	appsDir := filepath.Join(home, ".local", "share", "applications")
	matches, err := filepath.Glob(filepath.Join(appsDir, "*.desktop"))
	if err != nil {
		return nil, err
	}
	prefix := appimagesDir + string(filepath.Separator)
	var orphans []string
	for _, path := range matches {
		desktop, err := readDesktopFile(path)
		if err != nil {
			return nil, fmt.Errorf("read desktop file %s: %w", path, err)
		}
		if v, ok := desktop.get(desktopOwnerKey, desktopEntrySection); ok && v == "true" {
			continue
		}
		target := desktopTarget(desktop)
		if target == "" || !strings.HasPrefix(target, prefix) {
			continue
		}
		if _, err := os.Stat(target); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		orphans = append(orphans, strings.TrimSuffix(filepath.Base(path), ".desktop"))
	}
	return orphans, nil
}

// desktopTarget returns the executable path a launcher points at, preferring
// TryExec.
func desktopTarget(desktop *desktopFile) string {
	if tryExec, ok := desktop.get("TryExec", desktopEntrySection); ok && tryExec != "" {
		return tryExec
	}
	if exec, ok := desktop.get("Exec", desktopEntrySection); ok {
		return execPath(exec)
	}
	return ""
}
