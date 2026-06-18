package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
)

type githubReleaseSource struct {
	owner, repo, tag, pattern string
	api                       string // base API URL; "" means api.github.com (overridable for tests)
}

type ghAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"` // e.g. "sha256:abc…", may be empty
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

func (s githubReleaseSource) latest(ctx context.Context) (release, error) {
	base := s.api
	if base == "" {
		base = "https://api.github.com"
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/releases/", base, s.owner, s.repo)
	if s.tag == "" || s.tag == "latest" {
		endpoint += "latest"
	} else {
		endpoint += "tags/" + s.tag
	}

	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := githubToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return release{}, fmt.Errorf("query GitHub releases for %s/%s: %w", s.owner, s.repo, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("query GitHub releases for %s/%s: %s", s.owner, s.repo, resp.Status)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return release{}, fmt.Errorf("decode GitHub release for %s/%s: %w", s.owner, s.repo, err)
	}

	asset, err := matchAsset(rel.Assets, s.pattern)
	if err != nil {
		return release{}, err
	}
	return release{
		version: rel.TagName,
		url:     asset.URL,
		sha256:  strings.TrimPrefix(asset.Digest, "sha256:"),
		size:    asset.Size,
	}, nil
}

// matchAsset picks the release asset whose name matches pattern (a glob). On
// multiple matches it takes the first by name, for determinism.
func matchAsset(assets []ghAsset, pattern string) (ghAsset, error) {
	var matches []ghAsset
	for _, asset := range assets {
		if ok, _ := path.Match(pattern, asset.Name); ok {
			matches = append(matches, asset)
		}
	}
	if len(matches) == 0 {
		return ghAsset{}, fmt.Errorf("no GitHub release asset matches %q", pattern)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Name < matches[j].Name })
	return matches[0], nil
}

// githubToken returns a personal access token from the environment, used to
// raise the API rate limit. It is never sent to asset download URLs.
func githubToken() string {
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if token := os.Getenv(key); token != "" {
			return token
		}
	}
	return ""
}
