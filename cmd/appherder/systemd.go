package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed systemd/*
var unitTemplates embed.FS

var syncUnits = []string{
	"appherder-sync.path",
	"appherder-sync.service",
}

var upgradeUnits = []string{
	"appherder-upgrade.timer",
	"appherder-upgrade.service",
}

func enableUnits(units []string) error {
	if err := writeUnitFiles(units); err != nil {
		return err
	}
	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	return runSystemctl("enable", "--now", units[0])
}

func disableUnits(units []string) error {
	if err := runSystemctl("disable", "--now", units[0]); err != nil {
		return err
	}
	if err := removeUnitFiles(units); err != nil {
		return err
	}
	return runSystemctl("daemon-reload")
}

func binaryPath() (string, error) {
	bin, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve appherder binary: %w", err)
	}
	if bin, err = filepath.EvalSymlinks(bin); err != nil {
		return "", fmt.Errorf("resolve appherder binary: %w", err)
	}
	return bin, nil
}

// writeUnitFiles renders the named templates with the appherder binary path
// and writes them to the systemd user directory.
func writeUnitFiles(names []string) error {
	bin, err := binaryPath()
	if err != nil {
		return err
	}
	userDir, err := systemdUserDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return fmt.Errorf("create systemd user directory: %w", err)
	}
	for _, name := range names {
		data, err := unitTemplates.ReadFile("systemd/" + name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		rendered := strings.ReplaceAll(string(data), "{{BIN}}", bin)
		dest := filepath.Join(userDir, name)
		if err := os.WriteFile(dest, []byte(rendered), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
	}
	return nil
}

func removeUnitFiles(names []string) error {
	userDir, err := systemdUserDir()
	if err != nil {
		return err
	}
	for _, name := range names {
		_ = os.Remove(filepath.Join(userDir, name))
	}
	return nil
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
