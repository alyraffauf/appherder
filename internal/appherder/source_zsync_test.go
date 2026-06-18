package appherder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleZsync = "zsync: 0.6.2\n" +
	"Filename: MyApp-x86_64.AppImage\n" +
	"MTime: Wed, 01 Jan 2025 00:00:00 +0000\n" +
	"Blocksize: 2048\n" +
	"Length: 4096\n" +
	"Hash-Lengths: 2,2,4\n" +
	"URL: MyApp-x86_64.AppImage\n" +
	"SHA-1: da39a3ee5e6b4b0d3255bfef95601890afd80709\n" +
	"\n\x00\x01\x02\x03binary checksum block"

func TestParseUpdateInfoZsync(t *testing.T) {
	src, err := parseUpdateInfo("zsync|https://host/path/App.AppImage.zsync")
	if err != nil {
		t.Fatal(err)
	}
	z, ok := src.(zsyncURLSource)
	if !ok {
		t.Fatalf("got %T, want zsyncURLSource", src)
	}
	if z.url != "https://host/path/App.AppImage.zsync" {
		t.Fatalf("url = %q", z.url)
	}
}

func TestParseUpdateInfoRejectsMalformedZsync(t *testing.T) {
	if _, err := parseUpdateInfo("zsync|"); err == nil {
		t.Fatal("expected error for empty zsync URL")
	}
}

func TestParseZsyncHeaderStopsAtBlankLine(t *testing.T) {
	h, err := parseZsyncHeader(strings.NewReader(sampleZsync))
	if err != nil {
		t.Fatal(err)
	}
	if h["length"] != "4096" {
		t.Fatalf("length = %q", h["length"])
	}
	if h["sha-1"] != "da39a3ee5e6b4b0d3255bfef95601890afd80709" {
		t.Fatalf("sha-1 = %q", h["sha-1"])
	}
	if h["mtime"] != "Wed, 01 Jan 2025 00:00:00 +0000" { // value keeps its colons
		t.Fatalf("mtime = %q", h["mtime"])
	}
}

func TestParseZsyncHeaderRequiresURL(t *testing.T) {
	if _, err := parseZsyncHeader(strings.NewReader("Length: 1\n\n")); err == nil {
		t.Fatal("expected error when URL field is missing")
	}
}

func TestResolveZsyncURL(t *testing.T) {
	got, err := resolveZsyncURL("https://host/dir/App.AppImage.zsync", "App-x86_64.AppImage")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://host/dir/App-x86_64.AppImage" {
		t.Fatalf("relative resolution = %q", got)
	}

	got, err = resolveZsyncURL("https://host/dir/App.zsync", "https://cdn.example/App.AppImage")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://cdn.example/App.AppImage" {
		t.Fatalf("absolute URL = %q", got)
	}
}

func TestZsyncURLSourceLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app/MyApp.AppImage.zsync" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(sampleZsync))
	}))
	defer srv.Close()

	src := zsyncURLSource{url: srv.URL + "/app/MyApp.AppImage.zsync"}
	rel, err := src.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.URL != srv.URL+"/app/MyApp-x86_64.AppImage" {
		t.Fatalf("url = %q, want target resolved against control file", rel.URL)
	}
	if rel.SHA1 != "da39a3ee5e6b4b0d3255bfef95601890afd80709" {
		t.Fatalf("sha1 = %q", rel.SHA1)
	}
	if rel.Size != 4096 {
		t.Fatalf("size = %d", rel.Size)
	}
	if rel.Version != "Wed, 01 Jan 2025 00:00:00 +0000" {
		t.Fatalf("version = %q", rel.Version)
	}
}
