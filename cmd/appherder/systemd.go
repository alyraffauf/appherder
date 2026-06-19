package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alyraffauf/appherder/internal/appherder"
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

func enableUnits(a appherder.App, units []string) error {
	if err := ensureServiceWritePaths(a); err != nil {
		return err
	}
	if err := writeUnitFiles(a, units); err != nil {
		return err
	}
	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	return runSystemctl("enable", "--now", units[0])
}

func ensureServiceWritePaths(a appherder.App) error {
	for _, path := range a.ServiceWritePaths() {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("create service write directory %s: %w", path, err)
		}
	}
	return nil
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
	if appimage := os.Getenv("APPIMAGE"); appimage != "" {
		return appimage, nil
	}
	bin, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve appherder binary: %w", err)
	}
	if bin, err = filepath.EvalSymlinks(bin); err != nil {
		return "", fmt.Errorf("resolve appherder binary: %w", err)
	}
	return bin, nil
}

// writeUnitFiles renders the named templates with the appherder binary and
// configured directories, then writes them to the systemd user directory.
func writeUnitFiles(a appherder.App, names []string) error {
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
		rendered, err := renderUnitTemplate(string(data), bin, a)
		if err != nil {
			return fmt.Errorf("render %s: %w", name, err)
		}
		dest := filepath.Join(userDir, name)
		if err := os.WriteFile(dest, []byte(rendered), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
	}
	return nil
}

func renderUnitTemplate(template, bin string, a appherder.App) (string, error) {
	appimagesDir, err := systemdPathValue(a.AppImagesDir())
	if err != nil {
		return "", err
	}
	writePaths, err := systemdPathList(a.ServiceWritePaths())
	if err != nil {
		return "", err
	}

	rendered := strings.ReplaceAll(template, "{{BIN}}", systemdSpecifierEscape(bin))
	rendered = strings.ReplaceAll(rendered, "{{APPIMAGES_DIR}}", appimagesDir)
	rendered = strings.ReplaceAll(rendered, "{{READ_WRITE_PATHS}}", writePaths)
	return rendered, nil
}

func systemdPathList(paths []string) (string, error) {
	seen := make(map[string]bool, len(paths))
	values := make([]string, 0, len(paths))
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		value, err := systemdPathValue(path)
		if err != nil {
			return "", err
		}
		values = append(values, value)
	}
	return strings.Join(values, " "), nil
}

func systemdPathValue(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.ContainsAny(path, "\n\r") {
		return "", fmt.Errorf("path contains a newline: %q", path)
	}
	escaped := systemdSpecifierEscape(path)
	if strings.ContainsAny(escaped, " \t\"'\\") {
		return strconv.Quote(escaped), nil
	}
	return escaped, nil
}

func systemdSpecifierEscape(value string) string {
	return strings.ReplaceAll(value, "%", "%%")
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
