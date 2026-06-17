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
	if err := writeAtomic(dest, 0o755, func(w io.Writer) error {
		return copyTo(file, w)
	}); err != nil {
		return "", fmt.Errorf("install AppImage %s to %s: %w", file, dest, err)
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
