package appherder

import (
	"fmt"
	"os"
	"path/filepath"
)

// App is the core engine for managing AppImages. It holds no CLI or I/O
// state; all output formatting is the caller's responsibility.
type App struct {
	appimagesDir    string
	applicationsDir string
	iconsDir        string
}

// NewApp returns an App wired to the current user's home directory.
func NewApp() (App, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return App{}, fmt.Errorf("resolve home directory: %w", err)
	}
	return NewAppWithDirs(
		filepath.Join(home, "AppImages"),
		filepath.Join(home, ".local", "share", "applications"),
		filepath.Join(home, "AppImages", ".icons"),
	), nil
}

// NewAppWithDirs returns an App that uses the given directories directly,
// for tests or non-standard layouts.
func NewAppWithDirs(appimagesDir, applicationsDir, iconsDir string) App {
	return App{
		appimagesDir:    appimagesDir,
		applicationsDir: applicationsDir,
		iconsDir:        iconsDir,
	}
}
