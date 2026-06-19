package appherder

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// checkConcurrency caps concurrent update checks: enough to overlap network
// latency without hammering the API.
const checkConcurrency = 8

// UpgradeCheck is the per-app outcome of checking for an update.
type UpgradeCheck struct {
	Name      string
	Release   Release
	Available bool  // a newer build is available
	NoSource  bool  // no embedded update info; skip silently
	Err       error // the check itself failed
}

// UpgradeApplied is the per-app outcome of downloading and installing an update.
type UpgradeApplied struct {
	Name    string
	Version string
	Err     error
}

// CheckUpgrades checks appherder-managed AppImages for available updates.
// Only apps whose desktop file carries the X-AppHerder=true marker are
// included. Results come back in sorted filename order. Apps with no update
// info or already current are included with NoSource/Available=false so the
// caller can decide what to show.
func (a App) CheckUpgrades(ctx context.Context) ([]UpgradeCheck, error) {
	files, err := listAppImages(a.appimagesDir)
	if err != nil {
		return nil, err
	}

	var managed []string
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		desktop := filepath.Join(a.applicationsDir, name+".desktop")
		if owned, _ := isManagedDesktop(desktop); owned {
			managed = append(managed, file)
		}
	}

	return parallelMap(ctx, managed, checkConcurrency, a.checkOne), nil
}

// ApplyUpgrades downloads and installs updates for the given checks, processing
// only those with Available=true. Apply is sequential (bandwidth-bound); per-app
// errors are included in the result rather than aborting the run.
func (a App) ApplyUpgrades(ctx context.Context, checks []UpgradeCheck) []UpgradeApplied {
	var applied []UpgradeApplied
	for _, check := range checks {
		if check.Err != nil || check.NoSource || !check.Available {
			continue
		}
		err := a.applyUpgrade(ctx, check.Name, check.Release)
		applied = append(applied, UpgradeApplied{
			Name:    check.Name,
			Version: check.Release.Version,
			Err:     err,
		})
	}
	return applied
}

func (a App) checkOne(ctx context.Context, file string) UpgradeCheck {
	name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))

	src, err := a.SourceForAppImage(file)
	if err != nil {
		return UpgradeCheck{Name: name, Err: err}
	}
	if src == nil {
		return UpgradeCheck{Name: name, NoSource: true}
	}

	rel, err := src.Latest(ctx)
	if err != nil {
		return UpgradeCheck{Name: name, Err: err}
	}

	current, err := rel.localMatches(file)
	if err != nil {
		return UpgradeCheck{Name: name, Err: err}
	}
	return UpgradeCheck{Name: name, Release: rel, Available: !current}
}

// parallelMap applies fn to each item with at most `limit` concurrent calls,
// returning results in input order.
func parallelMap[T, R any](ctx context.Context, items []T, limit int, fn func(context.Context, T) R) []R {
	if limit < 1 {
		limit = 1
	}
	results := make([]R, len(items))
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, item T) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = fn(ctx, item)
		}(i, item)
	}
	wg.Wait()
	return results
}

func (a App) applyUpgrade(ctx context.Context, name string, rel Release) error {
	tmpName, err := downloadToTemp(ctx, rel.URL, "appherder-upgrade", a.progress, name)
	if err != nil {
		return err
	}
	defer os.Remove(tmpName)

	_, err = a.install(ctx, tmpName, rel.expectedChecksum())
	return err
}

// downloadToTemp downloads url to a temporary file and returns its path. The
// caller must remove the file.
func downloadToTemp(ctx context.Context, url, prefix string, progress Progress, name string) (string, error) {
	tmp, err := os.CreateTemp("", prefix+"-*.appimage")
	if err != nil {
		return "", fmt.Errorf("create temporary file: %w", err)
	}
	tmpName := tmp.Name()
	if err := download(ctx, url, tmp, progress, name); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("close download: %w", err)
	}
	return tmpName, nil
}

func download(ctx context.Context, url string, writer io.Writer, progress Progress, name string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resp, err := httpGetOK(ctx, url, fmt.Sprintf("download %s", url), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body := io.Reader(newIdleTimeoutReader(resp.Body, downloadIdleTimeout, cancel))
	body = newProgressReader(body, progress, name, resp.ContentLength)
	_, err = io.Copy(writer, body)
	return err
}
