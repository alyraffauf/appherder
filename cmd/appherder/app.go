package main

import "os"

type app struct {
	homeDir func() (string, error)
}

func newApp() app {
	return app{
		homeDir: os.UserHomeDir,
	}
}
