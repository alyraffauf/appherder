package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed systemd/*.path systemd/*.service
var unitTemplates embed.FS

const autosyncUnitBase = "appherder-sync"

// enableAutosync writes the systemd user units with the current binary path and
// enables the watcher.
func enableAutosync() error {
	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve appherder binary: %w", err)
	}
	if binPath, err = filepath.EvalSymlinks(binPath); err != nil {
		return fmt.Errorf("resolve appherder binary: %w", err)
	}

	userDir, err := systemdUserDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return fmt.Errorf("create systemd user directory: %w", err)
	}

	entries, err := fs.ReadDir(unitTemplates, "systemd")
	if err != nil {
		return fmt.Errorf("read embedded units: %w", err)
	}
	for _, entry := range entries {
		data, err := unitTemplates.ReadFile("systemd/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		rendered := strings.ReplaceAll(string(data), "{{BIN}}", binPath)
		dest := filepath.Join(userDir, entry.Name())
		if err := os.WriteFile(dest, []byte(rendered), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
	}

	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	return runSystemctl("enable", "--now", autosyncUnitBase+".path")
}

// disableAutosync stops and removes the systemd user units.
func disableAutosync() error {
	if err := runSystemctl("disable", "--now", autosyncUnitBase+".path"); err != nil {
		return err
	}
	userDir, err := systemdUserDir()
	if err != nil {
		return err
	}
	entries, err := fs.ReadDir(unitTemplates, "systemd")
	if err != nil {
		return fmt.Errorf("read embedded units: %w", err)
	}
	for _, entry := range entries {
		_ = os.Remove(filepath.Join(userDir, entry.Name()))
	}
	return runSystemctl("daemon-reload")
}

func systemdUserDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}
