package appherder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alyraffauf/goxdgdesktop/desktopfile"
)

// installConcurrency caps parallel installs: enough to overlap metadata I/O
// and squashfs decompression without copying several 200 MB binaries at once.
const installConcurrency = 4

// SyncInstall is one AppImage's outcome from the install phase of Sync.
type SyncInstall struct {
	File    string // source AppImage path
	AppName string // resolved install name; "" on error
	New     bool   // false if the app was already managed
	Err     error  // nil means success
}

// SyncRemoval is one launcher's outcome from the removal phase of Sync.
type SyncRemoval struct {
	AppName string
	Err     error // nil means removed
}

// SyncResult is the structured outcome of Sync: what was installed and what
// was removed, with per-item errors so the caller can format them.
type SyncResult struct {
	Installs []SyncInstall
	Removals []SyncRemoval
}

// Sync reconciles ~/AppImages with installed state: install every AppImage in
// the folder, remove launchers whose AppImage is gone. With force, also remove
// unmanaged launchers whose TryExec/Exec points at a missing file in ~/AppImages
// (entries left by another tool). Per-file errors are included in the result
// rather than aborting the pass.
func (a App) Sync(ctx context.Context, force bool) (SyncResult, error) {
	existing, err := managedApps(a.applicationsDir)
	if err != nil {
		return SyncResult{}, err
	}
	managed := make(map[string]bool, len(existing))
	for _, appid := range existing {
		managed[appid] = true
	}

	files, err := listAppImages(a.appimagesDir)
	if err != nil {
		return SyncResult{}, err
	}

	// parallelMap preserves input order, so results stay deterministic.
	installResults := parallelMap(ctx, files, installConcurrency, func(ctx context.Context, f string) SyncInstall {
		name, err := a.install(ctx, f, expectedChecksum{})
		return SyncInstall{File: f, AppName: name, Err: err}
	})

	var result SyncResult
	for _, inst := range installResults {
		if inst.Err == nil && !managed[inst.AppName] {
			inst.New = true
		}
		result.Installs = append(result.Installs, inst)
	}

	candidates := existing
	if force {
		extra, err := appImageBackedOrphans(a.applicationsDir, a.appimagesDir)
		if err != nil {
			return result, err
		}
		candidates = append(candidates, extra...)
	}
	for _, appid := range candidates {
		present, err := appImagePresent(a.appimagesDir, appid)
		if err != nil {
			return result, err
		}
		if present {
			continue
		}
		if err := a.Uninstall(appid, force); err != nil {
			result.Removals = append(result.Removals, SyncRemoval{AppName: appid, Err: err})
			continue
		}
		result.Removals = append(result.Removals, SyncRemoval{AppName: appid})
	}
	return result, nil
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

// appImageBackedOrphans returns appids of unmanaged desktop entries whose
// TryExec or Exec points at a missing file inside appimagesDir; launchers left
// by another tool after their AppImage was deleted.
func appImageBackedOrphans(applicationsDir, appimagesDir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(applicationsDir, "*.desktop"))
	if err != nil {
		return nil, err
	}
	prefix := appimagesDir + string(filepath.Separator)
	var orphans []string
	for _, path := range matches {
		desktop, err := desktopfile.Read(path)
		if err != nil {
			return nil, fmt.Errorf("read desktop file %s: %w", path, err)
		}
		if value, ok := desktop.Get(desktopEntrySection, desktopOwnerKey); ok && value == "true" {
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
func desktopTarget(desktop *desktopfile.File) string {
	if tryExec, ok := desktop.Get(desktopEntrySection, "TryExec"); ok && tryExec != "" {
		return tryExec
	}
	if exec, ok := desktop.Get(desktopEntrySection, "Exec"); ok {
		return execPath(exec)
	}
	return ""
}
