package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// zsyncURLSource resolves updates from a .zsync control file at a fixed URL,
// the generic (non-GitHub) AppImage update mechanism. We read the control
// file's header for the target's size, checksum, and URL, then download the
// whole file; zsync's block-level delta transfer isn't implemented yet.
type zsyncURLSource struct {
	url string
}

func (s zsyncURLSource) latest(ctx context.Context) (release, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return release{}, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return release{}, fmt.Errorf("fetch zsync control file %s: %w", s.url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("fetch zsync control file %s: %s", s.url, resp.Status)
	}

	header, err := parseZsyncHeader(resp.Body)
	if err != nil {
		return release{}, fmt.Errorf("parse zsync control file %s: %w", s.url, err)
	}

	target, err := resolveZsyncURL(s.url, header["url"])
	if err != nil {
		return release{}, err
	}

	rel := release{
		url:     target,
		sha1:    header["sha-1"],
		version: zsyncVersion(header),
	}
	if n, err := strconv.ParseInt(header["length"], 10, 64); err == nil {
		rel.size = n
	}
	return rel, nil
}

// parseZsyncHeader reads a .zsync file's text header: "Key: Value" lines ending
// at the blank line that separates them from the binary checksum block. Keys
// are lowercased. It stops at the blank line so the body isn't consumed.
func parseZsyncHeader(r io.Reader) (map[string]string, error) {
	header := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" { // header ends; binary data follows
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("malformed header line %q", line)
		}
		header[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if header["url"] == "" {
		return nil, fmt.Errorf("control file has no URL field")
	}
	return header, nil
}

// resolveZsyncURL turns the control file's URL field, which is often relative,
// into an absolute download URL against the .zsync file's own location.
func resolveZsyncURL(zsyncURL, target string) (string, error) {
	base, err := url.Parse(zsyncURL)
	if err != nil {
		return "", fmt.Errorf("parse zsync URL %s: %w", zsyncURL, err)
	}
	ref, err := url.Parse(target)
	if err != nil {
		return "", fmt.Errorf("parse target URL %q: %w", target, err)
	}
	return base.ResolveReference(ref).String(), nil
}

// zsyncVersion picks a human label for the build. zsync carries no version, so
// MTime (which changes per build) is the most useful, then the filename.
func zsyncVersion(header map[string]string) string {
	if v := header["mtime"]; v != "" {
		return v
	}
	if v := header["filename"]; v != "" {
		return v
	}
	return "latest"
}
