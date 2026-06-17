package main

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
func openAppImage(file string) (fs.FS, func(), error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, nil, fmt.Errorf("open AppImage %s: %w", file, err)
	}

	offset, err := appImageSquashfsOffset(f)
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("read AppImage %s: %w", file, err)
	}

	reader, err := squashfs.NewReaderAtOffset(f, offset)
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("read AppImage filesystem %s: %w", file, err)
	}

	return squashFS{root: &reader.FS}, func() { f.Close() }, nil
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
func appImageSquashfsOffset(r io.ReaderAt) (int64, error) {
	var h [64]byte
	if _, err := r.ReadAt(h[:], 0); err != nil {
		return 0, fmt.Errorf("read ELF header: %w", err)
	}
	if h[0] != 0x7f || h[1] != 'E' || h[2] != 'L' || h[3] != 'F' {
		return 0, errors.New("not an AppImage (missing ELF header)")
	}
	// AppImages record their type in the otherwise-unused ELF padding bytes.
	if h[8] == 'A' && h[9] == 'I' && h[10] == 1 {
		return 0, errors.New("type-1 AppImages are not supported")
	}

	bo := binary.ByteOrder(binary.LittleEndian)
	if h[5] == 2 {
		bo = binary.BigEndian
	}
	// The squashfs starts where the ELF ends: e_shoff + e_shnum*e_shentsize.
	switch h[4] {
	case 1: // 32-bit ELF: e_shoff@32, e_shentsize@46, e_shnum@48
		return int64(bo.Uint32(h[32:36])) + int64(bo.Uint16(h[46:48]))*int64(bo.Uint16(h[48:50])), nil
	case 2: // 64-bit ELF: e_shoff@40, e_shentsize@58, e_shnum@60
		return int64(bo.Uint64(h[40:48])) + int64(bo.Uint16(h[58:60]))*int64(bo.Uint16(h[60:62])), nil
	default:
		return 0, errors.New("unknown ELF class")
	}
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
	inFolder := samePath(filepath.Dir(file), appimagesDir)

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
			if err := writeAtomic(dest, 0o755, func(w io.Writer) error {
				return copyTo(file, w)
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
func samePath(a, b string) bool {
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ai, bi)
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
