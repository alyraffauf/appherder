package appherder

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

func TestResolveIconPrefersRootOverThemedEvenIfSmaller(t *testing.T) {
	fsys := fstest.MapFS{
		"app.png":                 {Data: []byte("x")},                // root, tiny png
		"usr/share/icons/big.svg": {Data: []byte("a larger payload")}, // themed, higher format and larger
	}
	if got := resolveIcon(fsys); got != "app.png" {
		t.Fatalf("resolveIcon = %q, want app.png (root beats themed)", got)
	}
}

func TestResolveIconPrefersSvgWithinTier(t *testing.T) {
	fsys := fstest.MapFS{
		"app.png": {Data: []byte("0123456789")}, // larger png
		"app.svg": {Data: []byte("s")},          // smaller, but higher format
	}
	if got := resolveIcon(fsys); got != "app.svg" {
		t.Fatalf("resolveIcon = %q, want app.svg", got)
	}
}

func TestResolveIconPrefersLargerWithinSameFormat(t *testing.T) {
	fsys := fstest.MapFS{
		"small.png": {Data: []byte("a")},
		"big.png":   {Data: []byte("aaaaaaaaaa")},
	}
	if got := resolveIcon(fsys); got != "big.png" {
		t.Fatalf("resolveIcon = %q, want big.png", got)
	}
}

func TestResolveIconUsesThemedWhenNoRootIcon(t *testing.T) {
	fsys := fstest.MapFS{
		"usr/share/icons/a.png": {Data: []byte("png")},
		"usr/share/icons/b.svg": {Data: []byte("svg")},
	}
	if got := resolveIcon(fsys); got != "usr/share/icons/b.svg" {
		t.Fatalf("resolveIcon = %q, want usr/share/icons/b.svg", got)
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
