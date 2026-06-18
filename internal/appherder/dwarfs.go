package appherder

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// openDwarFS extracts the DwarFS payload from an AppImage. Tries the system
// dwarfsextract tool first; falls back to the AppImage's own --appimage-extract
// as a last resort. Returns the extracted filesystem and a cleanup function.
func openDwarFS(appimagePath string) (fs.FS, func(), error) {
	dir, err := os.MkdirTemp("", "appherder-dwarfs")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp directory: %w", err)
	}
	cleanup := func() { os.RemoveAll(dir) }

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := extractCommand(ctx, appimagePath, dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("extract DwarFS AppImage: %w\n%s", err, out)
	}

	// dwarfsextract -o extracts directly; --appimage-extract nests under squashfs-root.
	root := dir
	if _, err := os.Stat(filepath.Join(dir, "squashfs-root")); err == nil {
		root = filepath.Join(dir, "squashfs-root")
	}
	return os.DirFS(root), cleanup, nil
}

func extractCommand(ctx context.Context, appimagePath, destDir string) *exec.Cmd {
	extract, err := exec.LookPath("dwarfsextract")
	if err == nil {
		return exec.CommandContext(ctx, extract,
			"--input="+appimagePath,
			"--output="+destDir,
			"--pattern=**.desktop",
			"--pattern=**.png",
			"--pattern=**.svg",
			"--pattern=.DirIcon",
		)
	}
	// Fall back to the AppImage's own --appimage-extract. The file must be
	// executable for this to work.
	os.Chmod(appimagePath, 0o755)
	cmd := exec.CommandContext(ctx, appimagePath, "--appimage-extract")
	cmd.Dir = destDir
	return cmd
}

// isDwarFS reports whether the payload at offset starts with the DwarFS magic.
func isDwarFS(file readerAt, offset int64) bool {
	magic := make([]byte, 6)
	_, err := file.ReadAt(magic, offset)
	return err == nil && string(magic) == "DWARFS"
}
