package main

import (
	"testing"
	"testing/fstest"
)

func TestResolveIconPrefersDirIcon(t *testing.T) {
	fsys := fstest.MapFS{
		".DirIcon":              {Data: []byte("dir")},
		"app.png":               {Data: []byte("png")},
		"usr/share/icons/a.svg": {Data: []byte("svg")},
	}
	if got := resolveIcon(fsys); got != ".DirIcon" {
		t.Fatalf("resolveIcon = %q, want .DirIcon", got)
	}
}

func TestResolveIconFallsBackToHighestRankedExtension(t *testing.T) {
	fsys := fstest.MapFS{
		"app.png":               {Data: []byte("0123456789")},
		"usr/share/icons/a.svg": {Data: []byte("svg")},
	}
	if got := resolveIcon(fsys); got != "usr/share/icons/a.svg" {
		t.Fatalf("resolveIcon = %q, want usr/share/icons/a.svg", got)
	}
}

func TestResolveIconPrefersLargerImageOfSameType(t *testing.T) {
	fsys := fstest.MapFS{
		"small.png": {Data: []byte("a")},
		"big.png":   {Data: []byte("aaaaaaaaaa")},
	}
	if got := resolveIcon(fsys); got != "big.png" {
		t.Fatalf("resolveIcon = %q, want big.png", got)
	}
}

func TestResolveIconReturnsEmptyWhenNoIcon(t *testing.T) {
	fsys := fstest.MapFS{
		"AppRun":      {Data: []byte("x")},
		"app.desktop": {Data: []byte("x")},
	}
	if got := resolveIcon(fsys); got != "" {
		t.Fatalf("resolveIcon = %q, want empty", got)
	}
}
