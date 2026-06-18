package main

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseLocalMatchesSHA1(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "app.AppImage")
	content := []byte("hello zsync")
	if err := os.WriteFile(file, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha1.Sum(content)

	rel := release{sha1: hex.EncodeToString(sum[:])}
	if ok, err := rel.localMatches(file); err != nil || !ok {
		t.Fatalf("localMatches = %v, %v; want true", ok, err)
	}
	if err := rel.verifyDownload(file); err != nil {
		t.Fatalf("verifyDownload = %v; want nil", err)
	}

	stale := release{sha1: hex.EncodeToString(make([]byte, 20))}
	if ok, _ := stale.localMatches(file); ok {
		t.Fatal("localMatches = true for mismatched sha1")
	}
	if err := stale.verifyDownload(file); err == nil {
		t.Fatal("verifyDownload = nil for mismatched sha1")
	}
}

func TestParseUpdateInfoGitHubStripsZsyncSuffix(t *testing.T) {
	src, err := parseUpdateInfo("gh-releases-zsync|myorg|myapp|latest|MyApp-*-x86_64.AppImage.zsync")
	if err != nil {
		t.Fatal(err)
	}
	gh, ok := src.(githubReleaseSource)
	if !ok {
		t.Fatalf("got %T, want githubReleaseSource", src)
	}
	if gh.owner != "myorg" || gh.repo != "myapp" || gh.tag != "latest" {
		t.Fatalf("parsed = %+v", gh)
	}
	if gh.pattern != "MyApp-*-x86_64.AppImage" {
		t.Fatalf("pattern = %q, want .zsync stripped", gh.pattern)
	}
}

func TestParseUpdateInfoRejectsUnsupported(t *testing.T) {
	if _, err := parseUpdateInfo("bintray-zsync|a|b|c|d"); err == nil {
		t.Fatal("expected error for unsupported source type")
	}
}

func TestParseUpdateInfoRejectsMalformedGitHub(t *testing.T) {
	if _, err := parseUpdateInfo("gh-releases-zsync|too|few"); err == nil {
		t.Fatal("expected error for malformed gh-releases info")
	}
}

func TestMatchAssetIgnoresZsyncAndOthers(t *testing.T) {
	assets := []ghAsset{
		{Name: "MyApp-1.2.3-x86_64.AppImage.zsync"},
		{Name: "source.tar.gz"},
		{Name: "MyApp-1.2.3-x86_64.AppImage"},
	}
	got, err := matchAsset(assets, "MyApp-*-x86_64.AppImage")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "MyApp-1.2.3-x86_64.AppImage" {
		t.Fatalf("matched %q", got.Name)
	}
}

func TestMatchAssetNoMatch(t *testing.T) {
	if _, err := matchAsset([]ghAsset{{Name: "other.AppImage"}}, "MyApp-*.AppImage"); err == nil {
		t.Fatal("expected error when nothing matches")
	}
}
