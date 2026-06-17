package main

import (
	"io"
	"os"
	"path/filepath"
)

// writeAtomic writes via a temp file beside path and renames it into place, so
// path never holds a partially written file.
func writeAtomic(path string, perm os.FileMode, write func(io.Writer) error) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			os.Remove(tmpName)
		}
	}()

	if err = write(tmp); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func copyFile(src string, dest string) error {
	return writeAtomic(dest, 0o644, func(w io.Writer) error {
		return copyTo(src, w)
	})
}
