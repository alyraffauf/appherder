package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// installConcurrency caps parallel installs: enough to overlap metadata I/O
// and squashfs decompression without copying several 200 MB binaries at once.
const installConcurrency = 4

type syncResult struct {
	file    string
	appName string
	err     error
}

// sync reconciles ~/AppImages with installed state: install every AppImage in
// the folder, remove launchers whose AppImage is gone. With force, also remove
// unmanaged launchers whose TryExec/Exec points at a missing file in ~/AppImages
// (entries left by another tool). Per-file errors are reported and skipped so
// one bad file does not abort the pass.
func (a app) sync(ctx context.Context, out io.Writer, force bool) error {
	home, err := a.homeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	appimagesDir := filepath.Join(home, "AppImages")

	// Snapshot managed apps before installing, so we can report only newly
	// installed ones instead of every already-present app.
	existing, err := managedApps(home)
	if err != nil {
		return err
	}
	managed := make(map[string]bool, len(existing))
	for _, appid := range existing {
		managed[appid] = true
	}

	files, err := listAppImages(appimagesDir)
	if err != nil {
		return err
	}

	// parallelMap preserves input order, so output stays deterministic.
	results := parallelMap(ctx, files, installConcurrency, func(_ context.Context, f string) syncResult {
		name, err := a.install(f)
		return syncResult{file: f, appName: name, err: err}
	})
	for _, result := range results {
		if result.err != nil {
			fmt.Fprintf(out, "skipped %s: %v\n", filepath.Base(result.file), result.err)
			continue
		}
		if !managed[result.appName] {
			fmt.Fprintf(out, "installed %s\n", result.appName)
		}
	}

	candidates := existing
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
			fmt.Fprintf(out, "skipped removing %s: %v\n", appid, err)
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
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".appimage") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

// appImagePresent reports whether <appid>.appimage exists in dir, matching the
// extension case-insensitively.
func appImagePresent(dir, appid string) (bool, error) {
	path, err := findAppImagePath(dir, appid)
	if err != nil {
		return false, err
	}
	return path != "", nil
}

// findAppImagePath returns the full path of <appid>.appimage in dir, matching
// the extension case-insensitively, or "" when absent.
func findAppImagePath(dir, appid string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(entry.Name(), appid+".appimage") {
			return filepath.Join(dir, entry.Name()), nil
		}
	}
	return "", nil
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
		if value, ok := desktop.get(desktopOwnerKey, desktopEntrySection); ok && value == "true" {
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
