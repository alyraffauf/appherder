package appherder

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
)

// gitlabReleaseSource resolves updates from a project's GitLab releases. The
// gl-releases-* update-info token is our own convention, and since GitLab's API
// carries no asset digest, the checksum comes from the sibling .zsync asset.
type gitlabReleaseSource struct {
	host, project, tag, pattern string
	api                         string // base URL; "" means https://{host} (overridable for tests)
}

type glLink struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	DirectAssetURL string `json:"direct_asset_url"`
}

type glRelease struct {
	TagName string `json:"tag_name"`
	Assets  struct {
		Links []glLink `json:"links"`
	} `json:"assets"`
}

func (gitlabReleaseSource) Kind() string { return "gitlab" }

func (s gitlabReleaseSource) Latest(ctx context.Context) (Release, error) {
	base := s.api
	if base == "" {
		base = "https://" + s.host
	}
	// The project path goes in one segment, so its slashes must be encoded.
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/releases/", base, url.PathEscape(s.project))
	if s.tag == "" || s.tag == "latest" {
		endpoint += "permalink/latest"
	} else {
		endpoint += url.PathEscape(s.tag)
	}

	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Release{}, err
	}
	if tok := gitlabToken(); tok != "" {
		req.Header.Set("PRIVATE-TOKEN", tok)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("query GitLab releases for %s: %w", s.project, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("query GitLab releases for %s: %s", s.project, resp.Status)
	}

	var rel glRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, fmt.Errorf("decode GitLab release for %s: %w", s.project, err)
	}

	asset, err := matchLink(rel.Assets.Links, s.pattern)
	if err != nil {
		return Release{}, err
	}
	out := Release{Version: rel.TagName, URL: linkURL(asset)}

	// No digest from the API: take the checksum from the sibling .zsync asset
	// when present, else comparison falls back to size.
	if zsyncLink, ok := findLink(rel.Assets.Links, asset.Name+".zsync"); ok {
		header, err := fetchZsyncHeader(ctx, linkURL(zsyncLink))
		if err != nil {
			return Release{}, err
		}
		out.SHA1 = header["sha-1"]
		if size, err := strconv.ParseInt(header["length"], 10, 64); err == nil {
			out.Size = size
		}
	}
	return out, nil
}

// matchLink picks the release asset whose name matches pattern (a glob), taking
// the first by name when several match, for determinism.
func matchLink(links []glLink, pattern string) (glLink, error) {
	var matches []glLink
	for _, link := range links {
		if ok, _ := path.Match(pattern, link.Name); ok {
			matches = append(matches, link)
		}
	}
	if len(matches) == 0 {
		return glLink{}, fmt.Errorf("no GitLab release asset matches %q", pattern)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Name < matches[j].Name })
	return matches[0], nil
}

func findLink(links []glLink, name string) (glLink, bool) {
	for _, link := range links {
		if link.Name == name {
			return link, true
		}
	}
	return glLink{}, false
}

// linkURL prefers the permalinked direct_asset_url, falling back to the raw url.
func linkURL(link glLink) string {
	if link.DirectAssetURL != "" {
		return link.DirectAssetURL
	}
	return link.URL
}

// gitlabToken returns a token from the environment, used for private projects
// and to raise rate limits. It is never sent to asset download URLs.
func gitlabToken() string {
	for _, key := range []string{"GL_TOKEN", "GITLAB_TOKEN"} {
		if token := os.Getenv(key); token != "" {
			return token
		}
	}
	return ""
}
