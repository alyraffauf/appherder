package main

import "testing"

func TestPatchExecCommandDropsOriginalEnvWrappedExecutable(t *testing.T) {
	got := patchExecCommand(
		"env FOO=bar helium --new-window %U",
		"/home/test/AppImages/helium.appimage",
	)
	want := "env FOO=bar DESKTOPINTEGRATION=1 /home/test/AppImages/helium.appimage --new-window %U"
	if got != want {
		t.Fatalf("patchExecCommand() = %q, want %q", got, want)
	}
}

func TestPatchExecCommandStripsMetadataFieldCodes(t *testing.T) {
	got := patchExecCommand(
		"helium --open %i %c %k %F",
		"/home/test/AppImages/helium.appimage",
	)
	want := "env DESKTOPINTEGRATION=1 /home/test/AppImages/helium.appimage --open %F"
	if got != want {
		t.Fatalf("patchExecCommand() = %q, want %q", got, want)
	}
}

func TestPatchExecCommandPreservesQuotedArguments(t *testing.T) {
	got := patchExecCommand(
		"helium --name 'Helium Browser' --flag=\"two words\" %U",
		"/home/test/AppImages/helium.appimage",
	)
	want := "env DESKTOPINTEGRATION=1 /home/test/AppImages/helium.appimage --name 'Helium Browser' '--flag=two words' %U"
	if got != want {
		t.Fatalf("patchExecCommand() = %q, want %q", got, want)
	}
}
