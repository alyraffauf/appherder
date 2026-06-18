package appherder

import (
	"fmt"
	"os"
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
}

// NewApp returns an App wired to the current user's home directory. The
// applications directory honors XDG_DATA_HOME; the user bin directory
// (~/.local/bin) is not covered by the XDG spec and is derived from home.
func NewApp() (App, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return App{}, fmt.Errorf("resolve home directory: %w", err)
	}
	return NewAppWithDirs(
		filepath.Join(home, "AppImages"),
		filepath.Join(xdg.DataHome, "applications"),
		filepath.Join(home, "AppImages", ".icons"),
		filepath.Join(home, ".local", "bin"),
	), nil
}

// NewAppWithDirs returns an App that uses the given directories directly,
// for tests or non-standard layouts.
func NewAppWithDirs(appimagesDir, applicationsDir, iconsDir, binDir string) App {
	return App{
		appimagesDir:    appimagesDir,
		applicationsDir: applicationsDir,
		iconsDir:        iconsDir,
		binDir:          binDir,
	}
}

// WithProgress returns a copy of App that reports download progress to p.
func (a App) WithProgress(p Progress) App {
	a.progress = p
	return a
}
