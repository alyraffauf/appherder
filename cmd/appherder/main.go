package main

import (
	"fmt"
	"os"

	"github.com/alyraffauf/appherder/internal/appherder"
)

var version = "dev"

func main() {
	app, err := appherder.NewApp()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cmd := newRootCommand(app, os.Stdout, os.Stderr)
	cmd.SetArgs(os.Args[1:])
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
