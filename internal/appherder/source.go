package appherder

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

// Release describes the newest available build a source knows about.
type Release struct {
	Version string    // human label, e.g. a release tag
	URL     string    // download URL for the AppImage
	SHA256  string    // hex sha256 of the asset, "" when unavailable
	SHA1    string    // hex sha1 (zsync's hash), "" when unavailable
	Size    int64     // content length, 0 when unavailable
	ModTime time.Time // server Last-Modified, for checksumless sources
}

// checksum returns the strongest hash the release carries with a hasher to
// match; ok is false when the source provided no cryptographic hash.
func (r Release) checksum() (want string, hasher hash.Hash, ok bool) {
	switch {
	case r.SHA256 != "":
		return r.SHA256, sha256.New(), true
	case r.SHA1 != "":
		return r.SHA1, sha1.New(), true
	}
	return "", nil, false
}

// localMatches reports whether file already equals this release, by the
// strongest available signal: checksum, then mtime, then size.
func (r Release) localMatches(file string) (bool, error) {
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
	if !r.ModTime.IsZero() {
		return !info.ModTime().Before(r.ModTime), nil
	}
	if r.Size > 0 {
		return info.Size() == r.Size, nil
	}
	return false, nil
}

// verifyDownload checks a freshly downloaded file against the release's
// checksum. With no checksum there is nothing to verify and it returns nil.
func (r Release) verifyDownload(file string) error {
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

// Source resolves the latest available build of an installed app.
type Source interface {
	Latest(ctx context.Context) (Release, error)
	Kind() string
}

// ReadUpdateInfo returns the AppImage's embedded update-information string from
// its .upd_info ELF section, or "" when absent or empty.
func ReadUpdateInfo(path string) (string, error) {
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

// SourceForAppImage resolves an update source from the AppImage's embedded
// update info. It returns (nil, nil) when the AppImage carries none.
func SourceForAppImage(file string) (Source, error) {
	info, err := ReadUpdateInfo(file)
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
func parseUpdateInfo(info string) (Source, error) {
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

// tokenFromEnv returns the first non-empty environment variable from keys.
func tokenFromEnv(keys ...string) string {
	for _, key := range keys {
		if token := os.Getenv(key); token != "" {
			return token
		}
	}
	return ""
}

// matchByName picks the first item whose name matches pattern (a glob), sorting
// matches by name for determinism.
func matchByName[T any](items []T, pattern string, name func(T) string, kind string) (T, error) {
	var matches []T
	for _, item := range items {
		if ok, _ := path.Match(pattern, name(item)); ok {
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		var zero T
		return zero, fmt.Errorf("no %s release asset matches %q", kind, pattern)
	}
	sort.Slice(matches, func(i, j int) bool { return name(matches[i]) < name(matches[j]) })
	return matches[0], nil
}

// decodeJSON decodes a JSON value from r, wrapping any error with desc.
func decodeJSON[T any](r io.Reader, desc string) (T, error) {
	var value T
	if err := json.NewDecoder(r).Decode(&value); err != nil {
		return value, fmt.Errorf("%s: %w", desc, err)
	}
	return value, nil
}
