package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func (a app) extractAppImage(file string, dest string) (string, error) {
	file, err := filepath.Abs(file)
	if err != nil {
		return "", fmt.Errorf("resolve AppImage path %q: %w", file, err)
	}
	if err := os.Chmod(file, 0o755); err != nil {
		return "", fmt.Errorf("make AppImage executable %s: %w", file, err)
	}
	if err := a.run(file, []string{"--appimage-extract"}, dest); err != nil {
		return "", fmt.Errorf("extract AppImage %s into %s: %w", file, dest, err)
	}
	return filepath.Join(dest, "squashfs-root"), nil
}

func (a app) installAppImage(file string, appName string) (string, error) {
	home, err := a.homeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	appimagesDir := filepath.Join(home, "AppImages")
	if err := os.MkdirAll(appimagesDir, 0o755); err != nil {
		return "", fmt.Errorf("create AppImages directory %s: %w", appimagesDir, err)
	}

	dest := filepath.Join(appimagesDir, appName+".appimage")
	tmp, err := os.CreateTemp(appimagesDir, "."+appName+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary AppImage in %s: %w", appimagesDir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := copyTo(file, tmp); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("copy AppImage %s to %s: %w", file, tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temporary AppImage %s: %w", tmpName, err)
	}

	if err := os.Chmod(tmpName, 0o755); err != nil {
		return "", fmt.Errorf("make temporary AppImage executable %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return "", fmt.Errorf("replace AppImage %s: %w", dest, err)
	}
	if err := os.Chmod(dest, 0o755); err != nil {
		return "", fmt.Errorf("make installed AppImage executable %s: %w", dest, err)
	}

	return dest, nil
}

func copyTo(src string, dest io.Writer) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source %s: %w", src, err)
	}
	defer in.Close()

	_, err = io.Copy(dest, in)
	return err
}
