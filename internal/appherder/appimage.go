package appherder

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CalebQ42/squashfs"
)

// openAppImage exposes a type-2 AppImage's filesystem as an fs.FS. Supports
// SquashFS (in-process) and DwarFS (extracted via the runtime). Caller must
// invoke the returned closer to release resources.
func openAppImage(ctx context.Context, path string) (fs.FS, func(), error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open AppImage %s: %w", path, err)
	}

	offset, err := fileSystemOffset(file)
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("read AppImage %s: %w", path, err)
	}

	if isDwarFS(file, offset) {
		file.Close()
		return openDwarFS(ctx, path)
	}

	if isSquashFS(file, offset) {
		return openSquashFS(file, offset)
	}

	// Not at the expected offset; try scanning forward.
	if scanned, ok := scanForSquashFS(file, offset); ok {
		return openSquashFS(file, scanned)
	}

	file.Close()
	return nil, nil, fmt.Errorf("read AppImage %s: unknown or unsupported filesystem", path)
}

func openSquashFS(file *os.File, offset int64) (fs.FS, func(), error) {
	reader, err := squashfs.NewReaderAtOffset(file, offset)
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("read SquashFS at offset %d: %w", offset, err)
	}
	return squashFS{root: &reader.FS}, func() { file.Close() }, nil
}

// squashFS adapts a squashfs reader to fs.FS, resolving symlinks on Open since
// the underlying reader returns them unfollowed. AppImages routinely symlink
// the root .desktop and .DirIcon into usr/share.
type squashFS struct {
	root *squashfs.FS
}

func (s squashFS) Open(name string) (fs.File, error) {
	file, err := s.root.OpenFile(name)
	if err != nil {
		return nil, err
	}
	for depth := 0; file.IsSymlink(); depth++ {
		if depth >= 40 {
			file.Close()
			return nil, fmt.Errorf("%s: too many levels of symbolic links", name)
		}
		next, ok := file.GetSymlinkFile().(*squashfs.File)
		file.Close()
		if !ok {
			return nil, fmt.Errorf("%s: broken symlink", name)
		}
		file = next
	}
	return file, nil
}

func (s squashFS) ReadDir(name string) ([]fs.DirEntry, error) { return s.root.ReadDir(name) }
func (s squashFS) Glob(pattern string) ([]string, error)      { return s.root.Glob(pattern) }
func (s squashFS) Stat(name string) (fs.FileInfo, error)      { return s.root.Stat(name) }

// fileSystemOffset returns the byte offset where the AppImage's payload
// filesystem begins, computed from the ELF section header table.
func fileSystemOffset(file io.ReaderAt) (int64, error) {
	var header [64]byte
	if _, err := file.ReadAt(header[:], 0); err != nil {
		return 0, fmt.Errorf("read ELF header: %w", err)
	}
	if header[0] != 0x7f || header[1] != 'E' || header[2] != 'L' || header[3] != 'F' {
		return 0, errors.New("not an AppImage (missing ELF header)")
	}

	// Byte 8-10: AppImage type. 1 = ISO 9660 (unsupported), 2 = appended filesystem.
	if header[8] == 'A' && header[9] == 'I' && header[10] == 1 {
		return 0, errors.New("type-1 AppImages are not supported")
	}

	var endian binary.ByteOrder = binary.LittleEndian
	if header[5] == 2 {
		endian = binary.BigEndian
	}

	// e_shoff + e_shnum * e_shentsize = end of the section header table.
	switch header[4] {
	case 1: // 32-bit
		tableStart := int64(endian.Uint32(header[32:36]))
		entrySize := int64(endian.Uint16(header[46:48]))
		entryCount := int64(endian.Uint16(header[48:50]))
		return tableStart + entrySize*entryCount, nil
	case 2: // 64-bit
		tableStart := int64(endian.Uint64(header[40:48]))
		entrySize := int64(endian.Uint16(header[58:60]))
		entryCount := int64(endian.Uint16(header[60:62]))
		return tableStart + entrySize*entryCount, nil
	default:
		return 0, errors.New("unknown ELF class")
	}
}

// readerAt is the subset of io.ReaderAt used by filesystem detection helpers.
type readerAt interface {
	ReadAt([]byte, int64) (int, error)
}

// isSquashFS reports whether a SquashFS superblock begins at offset.
func isSquashFS(file readerAt, offset int64) bool {
	magic := make([]byte, 4)
	_, err := file.ReadAt(magic, offset)
	return err == nil && string(magic) == "hsqs"
}

// scanForSquashFS searches for a SquashFS superblock in 4096-byte steps over
// the next 64 MiB starting from offset. Returns the found position or false.
func scanForSquashFS(file readerAt, offset int64) (int64, bool) {
	const window = 64 * 1024 * 1024
	buf := make([]byte, 4096)
	for pos := offset; pos < offset+window; pos += 4096 {
		bytesRead, err := file.ReadAt(buf, pos)
		if err != nil {
			return 0, false
		}
		for i := range bytesRead - 4 {
			if buf[i] == 'h' && buf[i+1] == 's' && buf[i+2] == 'q' && buf[i+3] == 's' {
				return pos + int64(i), true
			}
		}
	}
	return 0, false
}

// listAppImages returns *.appimage files in dir, case-insensitive (the
// AppImage spec uses .AppImage, but .appimage is common).
func listAppImages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".appimage") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

// findAppImagePath returns the full path of <appid>.appimage in dir, matching
// the extension case-insensitively, or "" when absent.
func findAppImagePath(dir, appid string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(entry.Name(), appid+".appimage") {
			return filepath.Join(dir, entry.Name()), nil
		}
	}
	return "", nil
}
