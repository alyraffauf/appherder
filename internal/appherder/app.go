package appherder

import "os"

// App is the core engine for managing AppImages. It holds no CLI or I/O
// state; all output formatting is the caller's responsibility.
type App struct {
	homeDir func() (string, error)
}

// NewApp returns an App wired to the current user's home directory.
func NewApp() App {
	return App{
		homeDir: os.UserHomeDir,
	}
}
