package appherder

import (
	"reflect"
	"testing"

	"github.com/kballard/go-shellquote"
)

func TestPatchExecCommandDropsOriginalEnvWrappedExecutable(t *testing.T) {
	got := mustSplit(t, patchExecCommand(
		"env FOO=bar upstream-app --new-window %U",
		"/home/test/AppImages/example.appimage",
	))
	want := []string{"env", "FOO=bar", "DESKTOPINTEGRATION=1", "/home/test/AppImages/example.appimage", "--new-window", "%U"}
	assertTokens(t, got, want)
}

func TestPatchExecCommandStripsMetadataFieldCodes(t *testing.T) {
	got := mustSplit(t, patchExecCommand(
		"upstream-app --open %i %c %k %F",
		"/home/test/AppImages/example.appimage",
	))
	want := []string{"env", "DESKTOPINTEGRATION=1", "/home/test/AppImages/example.appimage", "--open", "%F"}
	assertTokens(t, got, want)
}

func TestPatchExecCommandPreservesQuotedArguments(t *testing.T) {
	got := mustSplit(t, patchExecCommand(
		"upstream-app --name 'Example App' --flag=\"two words\" %U",
		"/home/test/AppImages/example.appimage",
	))
	want := []string{"env", "DESKTOPINTEGRATION=1", "/home/test/AppImages/example.appimage", "--name", "Example App", "--flag=two words", "%U"}
	assertTokens(t, got, want)
}

func TestPatchExecCommandDoesNotDuplicateDesktopIntegration(t *testing.T) {
	got := mustSplit(t, patchExecCommand(
		"env DESKTOPINTEGRATION=0 FOO=bar upstream-app %U",
		"/home/test/AppImages/example.appimage",
	))
	want := []string{"env", "DESKTOPINTEGRATION=0", "FOO=bar", "/home/test/AppImages/example.appimage", "%U"}
	assertTokens(t, got, want)
}

func TestPatchExecCommandKeepsPositionalKeyValueArgument(t *testing.T) {
	got := mustSplit(t, patchExecCommand(
		"upstream-app --set key=value %U",
		"/home/test/AppImages/example.appimage",
	))
	want := []string{"env", "DESKTOPINTEGRATION=1", "/home/test/AppImages/example.appimage", "--set", "key=value", "%U"}
	assertTokens(t, got, want)
}

func mustSplit(t *testing.T, cmd string) []string {
	t.Helper()

	tokens, err := shellquote.Split(cmd)
	if err != nil {
		t.Fatalf("split %q: %v", cmd, err)
	}
	return tokens
}

func assertTokens(t *testing.T, got []string, want []string) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens = %#v, want %#v", got, want)
	}
}
