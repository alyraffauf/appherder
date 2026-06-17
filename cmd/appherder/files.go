package main

import (
	"io"
	"io/fs"
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

func copyFromFS(fsys fs.FS, name string, dest string) error {
	in, err := fsys.Open(name)
	if err != nil {
		return err
	}
	defer in.Close()

	return writeAtomic(dest, 0o644, func(w io.Writer) error {
		_, err := io.Copy(w, in)
		return err
	})
}
