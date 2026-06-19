package appherder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestInstallFromURLDownloadsAndCleansUp(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not an appimage"))
	}))
	defer srv.Close()

	a, _ := newTestApp(t)
	_, err := a.InstallFromURL(context.Background(), srv.URL+"/Foo.AppImage")
	if err == nil {
		t.Fatal("expected error for non-AppImage content")
	}

	matches, _ := filepath.Glob(filepath.Join(tmp, "appherder-install-*.appimage"))
	if len(matches) > 0 {
		t.Fatalf("temp files left behind: %v", matches)
	}
}

func TestInstallFromURLHandlesDownloadFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	a, _ := newTestApp(t)
	if _, err := a.InstallFromURL(context.Background(), srv.URL+"/Foo.AppImage"); err == nil {
		t.Fatal("expected error for 404 download")
	}
}
