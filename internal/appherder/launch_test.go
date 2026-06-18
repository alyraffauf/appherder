package appherder

import "testing"

func TestLaunchMissingApp(t *testing.T) {
	a, _ := newTestApp(t)
	if err := a.Launch("missing"); err == nil {
		t.Fatal("Launch missing app succeeded, want error")
	}
}
