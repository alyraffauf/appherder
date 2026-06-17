package main

import (
	"bytes"
	"testing"
)

func TestInstallCommandRequiresAppImage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newRootCommand(app{}, &stdout, &stderr)
	cmd.SetArgs([]string{"install"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected install without APPIMAGE to fail")
	}
}

func TestRootHelpMentionsInstallCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newRootCommand(app{}, &stdout, &stderr)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("install")) {
		t.Fatalf("help output did not mention install command:\n%s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("completion")) {
		t.Fatalf("help output did not mention completion command:\n%s", stdout.String())
	}
}
