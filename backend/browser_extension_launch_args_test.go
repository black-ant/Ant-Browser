package backend

import (
	"slices"
	"testing"
)

func TestBuildBrowserLaunchArgsDoesNotLoadExtensionsFromStartupFlags(t *testing.T) {
	args := buildBrowserLaunchArgs("profile-dir", 9222, "direct://", nil, nil, nil, nil, false)
	if slices.Contains(args, "--load-extension") || slices.Contains(args, "--disable-extensions-except") {
		t.Fatalf("args = %#v, production extensions must not be loaded from startup flags", args)
	}
}
