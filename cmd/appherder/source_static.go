package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

// staticURLSource tracks a fixed URL that always serves the latest AppImage.
// There's no version or checksum, so freshness leans on the server's
// Last-Modified (vs. the installed file's mtime), then Content-Length.
type staticURLSource struct {
	url string
}

func (staticURLSource) kind() string { return "static" }

func (s staticURLSource) latest(ctx context.Context) (release, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	resp, err := s.probe(ctx)
	if err != nil {
		return release{}, err
	}
	defer resp.Body.Close()

	rel := release{url: s.url, version: "latest"}
	if lastModified := resp.Header.Get("Last-Modified"); lastModified != "" {
		rel.version = lastModified
		if modTime, err := http.ParseTime(lastModified); err == nil {
			rel.modTime = modTime
		}
	}
	if size, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); err == nil {
		rel.size = size
	}
	return rel, nil
}

// probe reads the URL's headers via HEAD, falling back to GET for servers that
// reject it. The caller closes the body; we never read it.
func (s staticURLSource) probe(ctx context.Context) (*http.Response, error) {
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		req, err := http.NewRequestWithContext(ctx, method, s.url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("probe %s: %w", s.url, err)
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		resp.Body.Close()
		if method == http.MethodHead && (resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented) {
			continue
		}
		return nil, fmt.Errorf("probe %s: %s", s.url, resp.Status)
	}
	return nil, fmt.Errorf("probe %s: no usable response", s.url)
}
