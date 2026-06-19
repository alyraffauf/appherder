//go:build gtk

package main

import (
	"os"

	"github.com/alyraffauf/appherder/internal/appherder"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
)

var version = "dev"

func main() {
	app := appherder.NewApp()

	appID := "io.github.alyraffauf.AppHerder"
	gtkApp := adw.NewApplication(appID, gio.ApplicationFlagsNone)
	gtkApp.SetVersion(version)

	gtkApp.ConnectActivate(func() {
		window := newMainWindow(gtkApp, app)
		window.Present()
	})

	if code := gtkApp.Run(nil); code > 0 {
		os.Exit(code)
	}
}
