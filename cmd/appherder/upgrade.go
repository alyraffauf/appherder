package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// checkConcurrency caps concurrent update checks: enough to overlap network
// latency without hammering the API.
const checkConcurrency = 8

// upgradeCheck is the per-app outcome of the (concurrent) check phase.
type upgradeCheck struct {
	name      string
	release   release
	available bool  // a newer build is available
	noSource  bool  // no embedded update info; skip silently
	err       error // the check itself failed
}

// upgrade installs available updates for AppImages in ~/AppImages; checkOnly
// reports them without downloading. Apps with no update info or already current
// are skipped silently, and per-app errors don't abort the run.
func (a app) upgrade(ctx context.Context, out io.Writer, checkOnly bool) error {
	home, err := a.homeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	files, err := listAppImages(filepath.Join(home, "AppImages"))
	if err != nil {
		return err
	}

	// Check every app concurrently; results come back in input order.
	checks := parallelMap(ctx, files, checkConcurrency, func(ctx context.Context, file string) upgradeCheck {
		return checkOne(ctx, file)
	})

	// Apply sequentially: bandwidth/disk-bound, and keeps output ordered.
	available := 0
	for _, c := range checks {
		switch {
		case c.err != nil:
			fmt.Fprintf(out, "skip %s: %v\n", c.name, c.err)
			continue
		case c.noSource || !c.available:
			continue
		}

		available++
		if checkOnly {
			fmt.Fprintf(out, "%s: update available (%s)\n", c.name, c.release.version)
			continue
		}

		if err := a.applyUpgrade(ctx, c.release); err != nil {
			fmt.Fprintf(out, "skip %s: %v\n", c.name, err)
			continue
		}
		fmt.Fprintf(out, "upgraded %s to %s\n", c.name, c.release.version)
	}

	if available == 0 {
		fmt.Fprintln(out, "everything is up to date")
	}
	return nil
}

// checkOne resolves an AppImage's source and reports whether an update exists.
func checkOne(ctx context.Context, file string) upgradeCheck {
	name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))

	src, err := sourceForAppImage(file)
	if err != nil {
		return upgradeCheck{name: name, err: err}
	}
	if src == nil {
		return upgradeCheck{name: name, noSource: true}
	}

	rel, err := src.latest(ctx)
	if err != nil {
		return upgradeCheck{name: name, err: err}
	}

	current, err := rel.localMatches(file)
	if err != nil {
		return upgradeCheck{name: name, err: err}
	}
	return upgradeCheck{name: name, release: rel, available: !current}
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

func (a app) applyUpgrade(ctx context.Context, rel release) error {
	tmp, err := os.CreateTemp("", "appherder-upgrade-*.appimage")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := download(ctx, rel.url, tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close download: %w", err)
	}

	if err := rel.verifyDownload(tmpName); err != nil {
		return err
	}

	return a.install(tmpName)
}

func download(ctx context.Context, url string, w io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	_, err = io.Copy(w, newIdleTimeoutReader(resp.Body, downloadIdleTimeout, cancel))
	return err
}
