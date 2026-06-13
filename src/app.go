package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const extractTimeout = 2 * time.Minute

type app struct {
	homeDir func() (string, error)
	run     func(name string, args []string, dir string) error
}

func newApp() app {
	return app{
		homeDir: os.UserHomeDir,
		run:     runCommand,
	}
}

func runCommand(name string, args []string, dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), extractTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("timed out after %s", extractTimeout)
	}
	if err != nil && len(output) > 0 {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return err
}
