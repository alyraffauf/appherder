package appherder

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/alyraffauf/goxdgdesktop/desktopfile"
)

// AppInfo is one managed app's display metadata, as returned by List.
type AppInfo struct {
	AppID    string
	Name     string // desktop Name= field, falls back to filename
	Filename string // basename of the AppImage in ~/AppImages, "" when missing
	Version  string // desktop X-AppImage-Version=
	Size     int64
	Source   string // update source kind, "none" when no info
}

// List returns display metadata for every app appherder manages. Offline and
// instant; use CheckUpgrades to see what's stale.
func (a App) List() ([]AppInfo, error) {
	appids, err := managedApps(a.applicationsDir)
	if err != nil {
		return nil, err
	}

	infos := make([]AppInfo, 0, len(appids))
	for _, appid := range appids {
		infos = append(infos, gatherAppInfo(a.applicationsDir, a.appimagesDir, appid))
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

// gatherAppInfo collects display metadata for appid from its installed desktop
// file and AppImage.
func gatherAppInfo(appsDir, appimagesDir, appid string) AppInfo {
	info := AppInfo{AppID: appid, Source: "none"}

	if desktop, err := desktopfile.Read(filepath.Join(appsDir, appid+".desktop")); err == nil {
		if name, ok := desktop.Get(desktopEntrySection, "Name"); ok && name != "" {
			info.Name = name
		}
		if version, ok := desktop.Get(desktopEntrySection, "X-AppImage-Version"); ok {
			info.Version = version
		}
	}

	if path, err := findAppImagePath(appimagesDir, appid); err == nil && path != "" {
		info.Filename = filepath.Base(path)
		if stat, err := os.Stat(path); err == nil {
			info.Size = stat.Size()
		}
		if src, err := SourceForAppImage(path); err == nil && src != nil {
			info.Source = src.Kind()
		}
	}

	if info.Name == "" {
		if info.Filename != "" {
			info.Name = info.Filename
		} else {
			info.Name = appid
		}
	}
	return info
}
