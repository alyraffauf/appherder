package appherder

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/CalebQ42/squashfs"
)

// openAppImage exposes a type-2 AppImage's squashfs payload as an fs.FS without
// executing it. The caller must invoke the returned closer to release the file.
func openAppImage(path string) (fs.FS, func(), error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open AppImage %s: %w", path, err)
	}

	offset, err := appImageSquashfsOffset(file)
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("read AppImage %s: %w", path, err)
	}

	reader, err := squashfs.NewReaderAtOffset(file, offset)
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("read AppImage filesystem %s: %w", path, err)
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

// appImageSquashfsOffset returns the byte offset of the squashfs image appended
// to the AppImage's ELF runtime, computed from the ELF section header table.
func appImageSquashfsOffset(reader io.ReaderAt) (int64, error) {
	var header [64]byte
	if _, err := reader.ReadAt(header[:], 0); err != nil {
		return 0, fmt.Errorf("read ELF header: %w", err)
	}
	if header[0] != 0x7f || header[1] != 'E' || header[2] != 'L' || header[3] != 'F' {
		return 0, errors.New("not an AppImage (missing ELF header)")
	}
	// AppImages record their type in the otherwise-unused ELF padding bytes.
	if header[8] == 'A' && header[9] == 'I' && header[10] == 1 {
		return 0, errors.New("type-1 AppImages are not supported")
	}

	byteOrder := binary.ByteOrder(binary.LittleEndian)
	if header[5] == 2 {
		byteOrder = binary.BigEndian
	}
	// The squashfs starts where the ELF ends: e_shoff + e_shnum*e_shentsize.
	switch header[4] {
	case 1: // 32-bit ELF: e_shoff@32, e_shentsize@46, e_shnum@48
		return int64(byteOrder.Uint32(header[32:36])) + int64(byteOrder.Uint16(header[46:48]))*int64(byteOrder.Uint16(header[48:50])), nil
	case 2: // 64-bit ELF: e_shoff@40, e_shentsize@58, e_shnum@60
		return int64(byteOrder.Uint64(header[40:48])) + int64(byteOrder.Uint16(header[58:60]))*int64(byteOrder.Uint16(header[60:62])), nil
	default:
		return 0, errors.New("unknown ELF class")
	}
}

func (a App) installAppImage(file string, appName string) (string, error) {
	if err := os.MkdirAll(a.appimagesDir, 0o755); err != nil {
		return "", fmt.Errorf("create AppImages directory %s: %w", a.appimagesDir, err)
	}

	dest := filepath.Join(a.appimagesDir, appName+".appimage")
	inFolder := samePath(filepath.Dir(file), a.appimagesDir)

	if !samePath(file, dest) {
		same, err := sameContent(file, dest)
		if err != nil {
			return "", fmt.Errorf("compare AppImage %s with %s: %w", file, dest, err)
		}
		switch {
		case same:
			// Installed binary is already byte-identical, so skip the copy. If
			// the source is a duplicate dropped into ~/AppImages, remove it.
			if inFolder {
				if err := os.Remove(file); err != nil {
					return "", fmt.Errorf("remove duplicate AppImage %s: %w", file, err)
				}
			}
		case inFolder:
			// A differently-named AppImage already in ~/AppImages: move it into
			// place instead of leaving a duplicate.
			if err := os.Rename(file, dest); err != nil {
				return "", fmt.Errorf("move AppImage %s to %s: %w", file, dest, err)
			}
		default:
			// Source lives elsewhere: copy it in.
			if err := writeAtomic(dest, 0o755, func(writer io.Writer) error {
				return copyTo(file, writer)
			}); err != nil {
				return "", fmt.Errorf("install AppImage %s to %s: %w", file, dest, err)
			}
		}
	}

	if err := os.Chmod(dest, 0o755); err != nil {
		return "", fmt.Errorf("make AppImage executable %s: %w", dest, err)
	}
	return dest, nil
}

// samePath reports whether a and b resolve to the same file, accounting for
// symlinks and mounts (e.g. /home -> /var/home).
func samePath(pathA, pathB string) bool {
	infoA, err := os.Stat(pathA)
	if err != nil {
		return false
	}
	infoB, err := os.Stat(pathB)
	if err != nil {
		return false
	}
	return os.SameFile(infoA, infoB)
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
