package appherder

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteIfChangedSkipsIdenticalContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	content := []byte("hello")

	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	// Pin an old mtime so a skipped write is detectable.
	old := time.Unix(1000, 0)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	if err := writeIfChanged(path, 0o644, content); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Fatalf("mtime changed on identical write: got %v, want %v", info.ModTime(), old)
	}
}

func TestWriteIfChangedWritesWhenDifferent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeIfChanged(path, 0o644, []byte("new")); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("content = %q, want new", got)
	}
}

func TestWriteIfChangedCreatesWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")

	if err := writeIfChanged(path, 0o644, []byte("fresh")); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "fresh" {
		t.Fatalf("content = %q, want fresh", got)
	}
}
