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
// the folder, remove managed launchers whose AppImage is gone. Per-file errors
// are reported and skipped so one bad file does not abort the pass.
func (a app) sync(out io.Writer) error {
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

	managed, err := managedApps(home)
	if err != nil {
		return err
	}
	for _, appid := range managed {
		// Look for the managed app's AppImage under any extension casing.
		present, err := appImagePresent(appimagesDir, appid)
		if err != nil {
			return err
		}
		if present {
			continue
		}
		if err := a.uninstall(appid, false); err != nil {
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
// extension case-insensitively. Install normalizes the name to <appid>.appimage
// on success; on install failure the original casing may remain.
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
