package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

type config struct {
	install string
}

func parseArgs(args []string, output io.Writer) (config, error) {
	fs := flag.NewFlagSet("appherder", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: appherder -install APPIMAGE")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Manage AppImages with desktop integration.")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Options:")
		fs.PrintDefaults()
	}

	var cfg config
	fs.StringVar(&cfg.install, "install", "", "Install an AppImage")
	fs.StringVar(&cfg.install, "i", "", "Install an AppImage")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if cfg.install == "" {
		return cfg, errors.New("missing required -install APPIMAGE")
	}
	return cfg, nil
}
