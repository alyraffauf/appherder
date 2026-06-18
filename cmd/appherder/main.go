package main

import (
	"fmt"
	"os"

	"github.com/alyraffauf/appherder/internal/appherder"
)

func main() {
	cmd := newRootCommand(appherder.NewApp(), os.Stdout, os.Stderr)
	cmd.SetArgs(os.Args[1:])
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
