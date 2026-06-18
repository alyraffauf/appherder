package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseUpdateInfoGitLabStripsZsyncSuffix(t *testing.T) {
	src, err := parseUpdateInfo("gl-releases-zsync|gitlab.com|mygroup/myapp|latest|MyApp-*-x86_64.AppImage.zsync")
	if err != nil {
		t.Fatal(err)
	}
	gl, ok := src.(gitlabReleaseSource)
	if !ok {
		t.Fatalf("got %T, want gitlabReleaseSource", src)
	}
	if gl.host != "gitlab.com" || gl.project != "mygroup/myapp" || gl.tag != "latest" {
		t.Fatalf("parsed = %+v", gl)
	}
	if gl.pattern != "MyApp-*-x86_64.AppImage" {
		t.Fatalf("pattern = %q, want .zsync stripped", gl.pattern)
	}
}

func TestParseUpdateInfoRejectsMalformedGitLab(t *testing.T) {
	if _, err := parseUpdateInfo("gl-releases-zsync|too|few"); err == nil {
		t.Fatal("expected error for malformed gl-releases info")
	}
}

func TestGitLabReleaseSourceLatest(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dl/MyApp-x86_64.AppImage.zsync":
			fmt.Fprint(w, sampleZsync)
		default: // the releases API
			if !strings.Contains(r.URL.EscapedPath(), "mygroup%2Fmyapp") {
				t.Errorf("project not encoded in path: %s", r.URL.EscapedPath())
			}
			fmt.Fprintf(w, `{
				"tag_name": "v2.0.0",
				"assets": {"links": [
					{"name": "MyApp-x86_64.AppImage", "url": "%[1]s/x", "direct_asset_url": "%[1]s/dl/MyApp-x86_64.AppImage"},
					{"name": "MyApp-x86_64.AppImage.zsync", "url": "%[1]s/x.zsync", "direct_asset_url": "%[1]s/dl/MyApp-x86_64.AppImage.zsync"}
				]}
			}`, srv.URL)
		}
	}))
	defer srv.Close()

	src := gitlabReleaseSource{host: "gitlab.com", project: "mygroup/myapp", tag: "latest", pattern: "MyApp-*.AppImage", api: srv.URL}
	rel, err := src.latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.version != "v2.0.0" {
		t.Fatalf("version = %q", rel.version)
	}
	if rel.url != srv.URL+"/dl/MyApp-x86_64.AppImage" {
		t.Fatalf("url = %q, want direct_asset_url", rel.url)
	}
	if rel.sha1 != "da39a3ee5e6b4b0d3255bfef95601890afd80709" {
		t.Fatalf("sha1 = %q, want value from sibling .zsync", rel.sha1)
	}
	if rel.size != 4096 {
		t.Fatalf("size = %d, want length from sibling .zsync", rel.size)
	}
}

func TestGitLabReleaseSourceUsesTagEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/releases/v9") {
			t.Errorf("path = %s, want tag endpoint", r.URL.Path)
		}
		fmt.Fprint(w, `{"tag_name":"v9","assets":{"links":[{"name":"a.AppImage","url":"u"}]}}`)
	}))
	defer srv.Close()

	src := gitlabReleaseSource{host: "h", project: "o/r", tag: "v9", pattern: "*.AppImage", api: srv.URL}
	rel, err := src.latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.url != "u" { // no direct_asset_url, falls back to url
		t.Fatalf("url = %q", rel.url)
	}
}

func TestGitLabReleaseSourceErrorsOnNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	src := gitlabReleaseSource{host: "h", project: "o/r", tag: "latest", pattern: "*.AppImage", api: srv.URL}
	if _, err := src.latest(context.Background()); err == nil {
		t.Fatal("expected error on 404")
	}
}
