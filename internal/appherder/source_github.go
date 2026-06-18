package appherder

import (
	"context"
	"fmt"
	"net/http"
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

func (githubReleaseSource) Kind() string { return "github" }

func (s githubReleaseSource) Latest(ctx context.Context) (Release, error) {
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

	desc := fmt.Sprintf("query GitHub releases for %s/%s", s.owner, s.repo)
	resp, err := httpGetOK(ctx, endpoint, desc, func(req *http.Request) {
		req.Header.Set("Accept", "application/vnd.github+json")
		if tok := githubToken(); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	})
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()

	rel, err := decodeJSON[ghRelease](resp.Body, fmt.Sprintf("decode GitHub release for %s/%s", s.owner, s.repo))
	if err != nil {
		return Release{}, err
	}

	asset, err := matchByName(rel.Assets, s.pattern, func(a ghAsset) string { return a.Name }, "GitHub")
	if err != nil {
		return Release{}, err
	}
	return Release{
		Version: rel.TagName,
		URL:     asset.URL,
		SHA256:  strings.TrimPrefix(asset.Digest, "sha256:"),
		Size:    asset.Size,
	}, nil
}

// githubToken returns a personal access token from the environment, used to
// raise the API rate limit. It is never sent to asset download URLs.
func githubToken() string {
	return tokenFromEnv("GH_TOKEN", "GITHUB_TOKEN")
}
