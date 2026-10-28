package main

import "testing"

func TestPreviousRelease(t *testing.T) {
	versions := []string{"v0.2.1", "v0.10.0", "v0.9.0", "v0.28.1", "v0.29.1", "v0.29.0-rc.1", "v0.30.0", "junk"}
	for v, want := range map[string]string{
		"v0.29.1":                               "v0.28.1", // the newest release before, not v0.29.0-rc.1
		"v0.30.0":                               "v0.29.1",
		"v0.10.0":                               "v0.9.0",  // numeric, not lexical
		"v0.29.2-0.20261027000000-abcdef123456": "v0.29.1", // a pseudo-version of main
		"v0.2.1":                                "",
	} {
		if got := previousRelease(v, versions); got != want {
			t.Errorf("previousRelease(%s) = %q, want %q", v, got, want)
		}
	}
	if !semverLess("v1.0.0-rc.1", "v1.0.0") || !semverLess("v1.0.0-rc.2", "v1.0.0-rc.10") || semverLess("v1.0.0", "v1.0.0") {
		t.Error("pre-release precedence is wrong")
	}
}
