package main

import (
	"context"
	"debug/elf"
	"fmt"
	"strings"
)

// release describes the newest available build a source knows about.
type release struct {
	version string // human label, e.g. a release tag
	url     string // download URL for the AppImage
	sha256  string // hex sha256 of the asset, "" when the source can't provide it
	size    int64
}

// source resolves the latest available build of an installed app.
type source interface {
	latest(ctx context.Context) (release, error)
}

// readUpdateInfo returns the AppImage's embedded update-information string from
// its .upd_info ELF section, or "" when absent or empty.
func readUpdateInfo(file string) (string, error) {
	f, err := elf.Open(file)
	if err != nil {
		return "", fmt.Errorf("open AppImage %s: %w", file, err)
	}
	defer f.Close()

	section := f.Section(".upd_info")
	if section == nil {
		return "", nil
	}
	data, err := section.Data()
	if err != nil {
		return "", fmt.Errorf("read .upd_info from %s: %w", file, err)
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
	default:
		return nil, fmt.Errorf("unsupported update source %q", fields[0])
	}
}
