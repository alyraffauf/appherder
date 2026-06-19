package appherder

import (
	"path/filepath"

	"github.com/adrg/xdg"
)

// App is the core engine for managing AppImages. It holds no CLI or I/O
// state; all output formatting is the caller's responsibility.
type App struct {
	appimagesDir    string
	applicationsDir string
	iconsDir        string
	binDir          string
	progress        Progress
	config          Config
}

// NewApp returns an App wired to the current user's home directory and
// ~/.config/appherder/config.toml. The applications directory honors
// XDG_DATA_HOME.
func NewApp() (App, error) {
	cfg := loadConfig()
	return NewAppWithDirs(
		cfg.AppImagesDir,
		filepath.Join(xdg.DataHome, "applications"),
		filepath.Join(cfg.AppImagesDir, ".icons"),
		cfg.BinDir,
	).withConfig(cfg), nil
}

// NewAppWithDirs returns an App that uses the given directories directly,
// for tests or non-standard layouts. Config defaults are used.
func NewAppWithDirs(appimagesDir, applicationsDir, iconsDir, binDir string) App {
	return App{
		appimagesDir:    appimagesDir,
		applicationsDir: applicationsDir,
		iconsDir:        iconsDir,
		binDir:          binDir,
		config: Config{
			AppImagesDir:     appimagesDir,
			MaxSavedVersions: 3,
			BinDir:           binDir,
		},
	}
}

func (a App) withConfig(cfg Config) App {
	a.config = cfg
	return a
}

// WithProgress returns a copy of App that reports download progress to p.
func (a App) WithProgress(p Progress) App {
	a.progress = p
	return a
}
