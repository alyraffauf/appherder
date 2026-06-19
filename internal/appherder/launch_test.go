package appherder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLaunchExecutesInstalledAppImage(t *testing.T) {
	app, home := newTestApp(t)
	marker := filepath.Join(home, "launched")
	script := "#!/bin/sh\nprintf launched > " + shellQuote(marker) + "\n"
	if err := os.MkdirAll(filepath.Join(home, "AppImages"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "AppImages", "foo.appimage"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := app.Launch("foo"); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for {
		if got, err := os.ReadFile(marker); err == nil && string(got) == "launched" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("launched app did not write marker")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
