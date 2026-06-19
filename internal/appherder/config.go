package appherder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	AppImagesDir     string                  `toml:"appimages_dir"`
	MaxSavedVersions int                     `toml:"max_saved_versions"`
	BinDir           string                  `toml:"bin_dir"`
	Sources          map[string]SourceConfig `toml:"sources"`
}

type SourceConfig struct {
	Type    string `toml:"type"`
	Owner   string `toml:"owner"`
	Repo    string `toml:"repo"`
	Host    string `toml:"host"`
	Project string `toml:"project"`
	Tag     string `toml:"tag"`
	Pattern string `toml:"pattern"`
	URL     string `toml:"url"`
}

func (sc SourceConfig) ToSource() (Source, error) {
	switch sc.Type {
	case "github":
		return githubReleaseSource{
			owner:   sc.Owner,
			repo:    sc.Repo,
			tag:     sc.Tag,
			pattern: sc.Pattern,
		}, nil
	case "gitlab":
		return gitlabReleaseSource{
			host:    sc.Host,
			project: sc.Project,
			tag:     sc.Tag,
			pattern: sc.Pattern,
		}, nil
	case "zsync":
		return zsyncURLSource{url: sc.URL}, nil
	case "static":
		return staticURLSource{url: sc.URL}, nil
	default:
		return nil, fmt.Errorf("unknown source type %q (expected github, gitlab, zsync, or static)", sc.Type)
	}
}

func configPath() string {
	return filepath.Join(xdg.ConfigHome, "appherder", "config.toml")
}

func loadConfig() Config {
	home, _ := os.UserHomeDir()
	cfg := Config{
		AppImagesDir:     filepath.Join(home, "AppImages"),
		MaxSavedVersions: 3,
		BinDir:           filepath.Join(home, ".local", "bin"),
	}

	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to parse config file: %v\n", err)
		fmt.Fprintf(os.Stderr, "Using default configuration\n")
		return Config{
			AppImagesDir:     filepath.Join(home, "AppImages"),
			MaxSavedVersions: 3,
			BinDir:           filepath.Join(home, ".local", "bin"),
		}
	}

	if cfg.MaxSavedVersions < 1 {
		cfg.MaxSavedVersions = 1
	}

	return cfg
}
