package main

import (
	"fmt"
	"os"
)

func main() {
	cmd := newRootCommand(newApp(), os.Stdout, os.Stderr)
	cmd.SetArgs(os.Args[1:])
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
