package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

func main() {
	cfg, err := parseArgs(os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if err := newApp().install(cfg.install); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
