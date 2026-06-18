package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"
)

// appInfo is one row in the `list` output.
type appInfo struct {
	appid    string
	name     string // desktop Name= field, falls back to filename
	filename string // basename of the AppImage in ~/AppImages
	version  string // desktop X-AppImage-Version=
	size     int64
	source   string
}

// list prints appherder's managed apps: display name, version, file size, and
// update source. Offline and instant; use `upgrade --check` to see what's
// stale. A missing AppImage shows as empty filename and size.
func (a app) list(out io.Writer) error {
	home, err := a.homeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	appids, err := managedApps(home)
	if err != nil {
		return err
	}

	appimagesDir := filepath.Join(home, "AppImages")
	appsDir := filepath.Join(home, ".local", "share", "applications")

	infos := make([]appInfo, 0, len(appids))
	for _, appid := range appids {
		infos = append(infos, gatherAppInfo(appsDir, appimagesDir, appid))
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].name < infos[j].name })

	tabWriter := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tabWriter, "NAME\tFILENAME\tVERSION\tSIZE\tSOURCE")
	for _, info := range infos {
		size := "-"
		if info.size > 0 {
			size = humanSize(info.size)
		}
		fmt.Fprintf(tabWriter, "%s\t%s\t%s\t%s\t%s\n",
			info.name, orDash(info.filename), orDash(info.version), size, orDash(info.source))
	}
	return tabWriter.Flush()
}

// orDash returns s, or "-" when empty, for table cells.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// gatherAppInfo collects display metadata for appid from its installed desktop
// file and AppImage.
func gatherAppInfo(appsDir, appimagesDir, appid string) appInfo {
	info := appInfo{appid: appid, source: "none"}

	if desktop, err := readDesktopFile(filepath.Join(appsDir, appid+".desktop")); err == nil {
		if name, ok := desktop.get("Name", desktopEntrySection); ok && name != "" {
			info.name = name
		}
		if version, ok := desktop.get("X-AppImage-Version", desktopEntrySection); ok {
			info.version = version
		}
	}

	if path, err := findAppImagePath(appimagesDir, appid); err == nil && path != "" {
		info.filename = filepath.Base(path)
		if stat, err := os.Stat(path); err == nil {
			info.size = stat.Size()
		}
		if src, err := sourceForAppImage(path); err == nil && src != nil {
			info.source = src.kind()
		}
	}

	if info.name == "" {
		if info.filename != "" {
			info.name = info.filename
		} else {
			info.name = appid
		}
	}
	return info
}

// humanSize formats bytes in binary units with one decimal place (e.g. "354 MB").
func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for scaled := bytes / unit; scaled >= unit; scaled /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
