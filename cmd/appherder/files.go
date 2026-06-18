package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// writeIfChanged writes content to path atomically, but skips when path
// already holds identical bytes so the file's mtime stays stable.
func writeIfChanged(path string, perm os.FileMode, content []byte) error {
	if existing, err := os.ReadFile(path); err == nil {
		if bytes.Equal(existing, content) {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeAtomic(path, perm, func(w io.Writer) error {
		_, err := w.Write(content)
		return err
	})
}

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

// sameContent reports whether a and b are byte-identical, comparing size first
// to avoid hashing files that obviously differ. A missing b reports false.
func sameContent(pathA, pathB string) (bool, error) {
	infoA, err := os.Stat(pathA)
	if err != nil {
		return false, err
	}
	infoB, err := os.Stat(pathB)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if infoA.Size() != infoB.Size() {
		return false, nil
	}

	hashA, err := fileHash(pathA)
	if err != nil {
		return false, err
	}
	hashB, err := fileHash(pathB)
	if err != nil {
		return false, err
	}
	return bytes.Equal(hashA, hashB), nil
}

func fileHash(path string) ([]byte, error) {
	return fileSum(path, sha256.New())
}

// fileSum returns h's checksum over the file's contents.
func fileSum(path string, hasher hash.Hash) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if _, err := io.Copy(hasher, file); err != nil {
		return nil, err
	}
	return hasher.Sum(nil), nil
}

func copyFromFS(fsys fs.FS, name string, dest string) error {
	content, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	return writeIfChanged(dest, 0o644, content)
}
