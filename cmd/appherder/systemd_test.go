package main

import (
	"testing"

	"github.com/alyraffauf/appherder/internal/appherder"
)

func TestRenderUnitTemplateUsesConfiguredPaths(t *testing.T) {
	app := appherder.NewAppWithDirs(
		"/tmp/App Images/%apps",
		"/tmp/data/applications",
		"/tmp/App Images/%apps/.icons",
		"/tmp/bin dir",
	)
	template := "ExecStart={{BIN}} sync\n" +
		"PathChanged={{APPIMAGES_DIR}}\n" +
		"ReadWritePaths={{READ_WRITE_PATHS}}"

	got, err := renderUnitTemplate(template, "/opt/app%herder", app)
	if err != nil {
		t.Fatalf("renderUnitTemplate: %v", err)
	}

	want := "ExecStart=/opt/app%%herder sync\n" +
		`PathChanged="/tmp/App Images/%%apps"` + "\n" +
		`ReadWritePaths="/tmp/App Images/%%apps" /tmp/data/applications "/tmp/bin dir"`
	if got != want {
		t.Fatalf("rendered unit = %q, want %q", got, want)
	}
}

func TestRenderUnitTemplateRejectsNewlinePaths(t *testing.T) {
	app := appherder.NewAppWithDirs(
		"/tmp/AppImages\nbad",
		"/tmp/data/applications",
		"/tmp/AppImages/.icons",
		"/tmp/bin",
	)

	if _, err := renderUnitTemplate("PathChanged={{APPIMAGES_DIR}}", "/bin/appherder", app); err == nil {
		t.Fatal("expected newline path error")
	}
}
