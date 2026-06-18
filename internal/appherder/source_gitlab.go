package appherder

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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

	desc := fmt.Sprintf("query GitLab releases for %s", s.project)
	resp, err := httpGetOK(ctx, endpoint, desc, func(req *http.Request) {
		if tok := gitlabToken(); tok != "" {
			req.Header.Set("PRIVATE-TOKEN", tok)
		}
	})
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()

	rel, err := decodeJSON[glRelease](resp.Body, fmt.Sprintf("decode GitLab release for %s", s.project))
	if err != nil {
		return Release{}, err
	}

	asset, err := matchByName(rel.Assets.Links, s.pattern, func(l glLink) string { return l.Name }, "GitLab")
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
	return tokenFromEnv("GL_TOKEN", "GITLAB_TOKEN")
}
