package appherder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const lastModified = "Mon, 02 Jun 2025 12:00:00 GMT"

func TestParseUpdateInfoStatic(t *testing.T) {
	src, err := parseUpdateInfo("static|https://host/App-latest.AppImage")
	if err != nil {
		t.Fatal(err)
	}
	s, ok := src.(staticURLSource)
	if !ok {
		t.Fatalf("got %T, want staticURLSource", src)
	}
	if s.url != "https://host/App-latest.AppImage" {
		t.Fatalf("url = %q", s.url)
	}
}

func TestParseUpdateInfoRejectsMalformedStatic(t *testing.T) {
	if _, err := parseUpdateInfo("static|"); err == nil {
		t.Fatal("expected error for empty static URL")
	}
}

func TestStaticURLSourceLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method = %s, want HEAD", r.Method)
		}
		w.Header().Set("Last-Modified", lastModified)
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rel, err := staticURLSource{url: srv.URL}.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.URL != srv.URL {
		t.Fatalf("url = %q", rel.URL)
	}
	if rel.Size != 4096 {
		t.Fatalf("size = %d", rel.Size)
	}
	if rel.Version != lastModified {
		t.Fatalf("version = %q", rel.Version)
	}
	if want, _ := http.ParseTime(lastModified); !rel.ModTime.Equal(want) {
		t.Fatalf("modTime = %v, want %v", rel.ModTime, want)
	}
}

func TestStaticURLSourceFallsBackToGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			http.Error(w, "no HEAD", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Length", "10")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rel, err := staticURLSource{url: srv.URL}.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Size != 10 {
		t.Fatalf("size = %d, want value from GET fallback", rel.Size)
	}
}

func TestReleaseLocalMatchesModTime(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "app.AppImage")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	server, _ := http.ParseTime(lastModified)
	chtime := func(mtime time.Time) {
		if err := os.Chtimes(file, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}

	// Installed after the server's copy: current.
	chtime(server.Add(time.Hour))
	if ok, err := (Release{ModTime: server}).localMatches(file); err != nil || !ok {
		t.Fatalf("newer install: matches = %v, %v; want true", ok, err)
	}

	// Server published something newer: stale.
	chtime(server.Add(-time.Hour))
	if ok, _ := (Release{ModTime: server}).localMatches(file); ok {
		t.Fatal("older install: matches = true, want false")
	}
}
