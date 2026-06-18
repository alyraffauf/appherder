package appherder

import (
	"fmt"
	"os/exec"
)

// Launch starts the installed AppImage for appid and returns immediately.
func (a App) Launch(appid string) error {
	path, err := findAppImagePath(a.appimagesDir, appid)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("appimage %s is not installed", appid)
	}
	cmd := exec.Command(path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch %s: %w", appid, err)
	}
	return cmd.Process.Release()
}
