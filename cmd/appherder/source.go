package main

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"strings"
	"time"
)

// release describes the newest available build a source knows about.
type release struct {
	version string    // human label, e.g. a release tag
	url     string    // download URL for the AppImage
	sha256  string    // hex sha256 of the asset, "" when unavailable
	sha1    string    // hex sha1 (zsync's hash), "" when unavailable
	size    int64     // content length, 0 when unavailable
	modTime time.Time // server Last-Modified, for checksumless sources
}

// checksum returns the strongest hash the release carries with a hasher to
// match; ok is false when the source provided no cryptographic hash.
func (r release) checksum() (want string, hasher hash.Hash, ok bool) {
	switch {
	case r.sha256 != "":
		return r.sha256, sha256.New(), true
	case r.sha1 != "":
		return r.sha1, sha1.New(), true
	}
	return "", nil, false
}

// localMatches reports whether file already equals this release, by the
// strongest available signal: checksum, then mtime, then size.
func (r release) localMatches(file string) (bool, error) {
	if want, hasher, ok := r.checksum(); ok {
		sum, err := fileSum(file, hasher)
		if err != nil {
			return false, err
		}
		return strings.EqualFold(hex.EncodeToString(sum), want), nil
	}

	info, err := os.Stat(file)
	if err != nil {
		return false, err
	}
	// No checksum: current if the install is at least as new as the server's
	// copy, else fall back to size.
	if !r.modTime.IsZero() {
		return !info.ModTime().Before(r.modTime), nil
	}
	if r.size > 0 {
		return info.Size() == r.size, nil
	}
	return false, nil
}

// verifyDownload checks a freshly downloaded file against the release's
// checksum. With no checksum there is nothing to verify and it returns nil.
func (r release) verifyDownload(file string) error {
	want, hasher, ok := r.checksum()
	if !ok {
		return nil
	}
	sum, err := fileSum(file, hasher)
	if err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(sum), want) {
		return fmt.Errorf("downloaded AppImage failed checksum verification")
	}
	return nil
}

// source resolves the latest available build of an installed app.
type source interface {
	latest(ctx context.Context) (release, error)
	kind() string
}

// readUpdateInfo returns the AppImage's embedded update-information string from
// its .upd_info ELF section, or "" when absent or empty.
func readUpdateInfo(path string) (string, error) {
	file, err := elf.Open(path)
	if err != nil {
		return "", fmt.Errorf("open AppImage %s: %w", path, err)
	}
	defer file.Close()

	section := file.Section(".upd_info")
	if section == nil {
		return "", nil
	}
	data, err := section.Data()
	if err != nil {
		return "", fmt.Errorf("read .upd_info from %s: %w", path, err)
	}
	return strings.TrimRight(string(data), "\x00"), nil
}

// sourceForAppImage resolves an update source from the AppImage's embedded
// update info. It returns (nil, nil) when the AppImage carries none.
func sourceForAppImage(file string) (source, error) {
	info, err := readUpdateInfo(file)
	if err != nil {
		return nil, err
	}
	if info == "" {
		return nil, nil
	}
	return parseUpdateInfo(info)
}

// parseUpdateInfo turns an AppImage update-info string (the "type|a|b|..." form)
// into a concrete source.
func parseUpdateInfo(info string) (source, error) {
	fields := strings.Split(info, "|")
	switch fields[0] {
	case "gh-releases-zsync", "gh-releases-direct":
		// gh-releases-zsync|owner|repo|tag|pattern.zsync
		if len(fields) != 5 {
			return nil, fmt.Errorf("malformed GitHub update info %q", info)
		}
		return githubReleaseSource{
			owner:   fields[1],
			repo:    fields[2],
			tag:     fields[3],
			pattern: strings.TrimSuffix(fields[4], ".zsync"),
		}, nil
	case "gl-releases-zsync", "gl-releases-direct":
		// gl-releases-zsync|host|project|tag|pattern.zsync (our convention)
		if len(fields) != 5 {
			return nil, fmt.Errorf("malformed GitLab update info %q", info)
		}
		return gitlabReleaseSource{
			host:    fields[1],
			project: fields[2],
			tag:     fields[3],
			pattern: strings.TrimSuffix(fields[4], ".zsync"),
		}, nil
	case "zsync":
		// zsync|https://host/path/App-latest.AppImage.zsync
		if len(fields) != 2 || fields[1] == "" {
			return nil, fmt.Errorf("malformed zsync update info %q", info)
		}
		return zsyncURLSource{url: fields[1]}, nil
	case "static":
		// static|https://host/App-latest.AppImage (our convention)
		if len(fields) != 2 || fields[1] == "" {
			return nil, fmt.Errorf("malformed static update info %q", info)
		}
		return staticURLSource{url: fields[1]}, nil
	default:
		return nil, fmt.Errorf("unsupported update source %q", fields[0])
	}
}
