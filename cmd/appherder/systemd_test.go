package main

import (
	"strings"
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
	template := strings.Join([]string{
		"ExecStart={{BIN}} sync",
		"PathChanged={{APPIMAGES_DIR}}",
		"ReadWritePaths={{READ_WRITE_PATHS}}",
	}, "\n")

	got, err := renderUnitTemplate(template, "/opt/app%herder", app)
	if err != nil {
		t.Fatalf("renderUnitTemplate: %v", err)
	}

	for _, want := range []string{
		"ExecStart=/opt/app%%herder sync",
		`PathChanged="/tmp/App Images/%%apps"`,
		`ReadWritePaths="/tmp/App Images/%%apps" /tmp/data/applications "/tmp/bin dir"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered unit missing %q:\n%s", want, got)
		}
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
