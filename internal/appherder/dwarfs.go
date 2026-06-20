package appherder

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"time"
)

// openDwarFS extracts the DwarFS payload from an AppImage using dwarfsextract.
// It intentionally does not fall back to --appimage-extract, because that
// executes the AppImage runtime.
func openDwarFS(ctx context.Context, appimagePath string) (fs.FS, func(), error) {
	dir, err := os.MkdirTemp("", "appherder-dwarfs")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp directory: %w", err)
	}
	cleanup := func() { os.RemoveAll(dir) }

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd, err := extractCommand(ctx, appimagePath, dir)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("extract DwarFS AppImage: %w\n%s", err, out)
	}

	return os.DirFS(dir), cleanup, nil
}

func extractCommand(ctx context.Context, appimagePath, destDir string) (*exec.Cmd, error) {
	extract, err := exec.LookPath("dwarfsextract")
	if err != nil {
		return nil, fmt.Errorf("find dwarfsextract: %w", err)
	}
	return exec.CommandContext(ctx, extract,
		"--input="+appimagePath,
		"--output="+destDir,
		"--pattern=**.desktop",
		"--pattern=**.png",
		"--pattern=**.svg",
		"--pattern=.DirIcon",
	), nil
}

// isDwarFS reports whether the payload at offset starts with the DwarFS magic.
func isDwarFS(file readerAt, offset int64) bool {
	magic := make([]byte, 6)
	_, err := file.ReadAt(magic, offset)
	return err == nil && string(magic) == "DWARFS"
}
