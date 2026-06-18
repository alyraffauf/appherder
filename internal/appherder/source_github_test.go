package appherder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubReleaseSourceLatest(t *testing.T) {
	const body = `{
		"tag_name": "v1.2.3",
		"assets": [
			{"name": "MyApp-1.2.3-x86_64.AppImage", "browser_download_url": "https://example.com/MyApp.AppImage", "size": 1234, "digest": "sha256:deadbeef"},
			{"name": "MyApp-1.2.3-x86_64.AppImage.zsync", "browser_download_url": "https://example.com/MyApp.AppImage.zsync", "size": 99},
			{"name": "source.tar.gz", "browser_download_url": "https://example.com/src.tgz", "size": 5}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/myorg/myapp/releases/latest" {
			t.Errorf("path = %s, want /repos/myorg/myapp/releases/latest", r.URL.Path)
		}
		w.Write([]byte(body))
	}))
	defer srv.Close()

	src := githubReleaseSource{owner: "myorg", repo: "myapp", tag: "latest", pattern: "MyApp-*-x86_64.AppImage", api: srv.URL}
	rel, err := src.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "v1.2.3" {
		t.Fatalf("version = %q", rel.Version)
	}
	if rel.URL != "https://example.com/MyApp.AppImage" {
		t.Fatalf("url = %q", rel.URL)
	}
	if rel.SHA256 != "deadbeef" {
		t.Fatalf("sha256 = %q, want digest with sha256: prefix stripped", rel.SHA256)
	}
	if rel.Size != 1234 {
		t.Fatalf("size = %d", rel.Size)
	}
}

func TestGitHubReleaseSourceUsesTagEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/releases/tags/v9" {
			t.Errorf("path = %s, want /repos/o/r/releases/tags/v9", r.URL.Path)
		}
		w.Write([]byte(`{"tag_name":"v9","assets":[{"name":"a.AppImage","browser_download_url":"u","size":1,"digest":"sha256:ab"}]}`))
	}))
	defer srv.Close()

	src := githubReleaseSource{owner: "o", repo: "r", tag: "v9", pattern: "*.AppImage", api: srv.URL}
	if _, err := src.Latest(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubReleaseSourceErrorsOnNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	src := githubReleaseSource{owner: "o", repo: "r", tag: "latest", pattern: "*.AppImage", api: srv.URL}
	if _, err := src.Latest(context.Background()); err == nil {
		t.Fatal("expected error on 404")
	}
}
