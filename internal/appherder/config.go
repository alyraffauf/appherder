package appherder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	AppImagesDir     string `toml:"appimages_dir"`
	MaxSavedVersions int    `toml:"max_saved_versions"`
	BinDir           string `toml:"bin_dir"`
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
