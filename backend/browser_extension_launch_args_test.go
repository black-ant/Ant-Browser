package backend

import (
	"slices"
	"testing"
)

func TestBuildBrowserLaunchArgsDoesNotLoadProductionExtensionsByDefault(t *testing.T) {
	args := buildBrowserLaunchArgs("profile-dir", 9222, "direct://", nil, nil, nil, nil, nil, false)
	if slices.Contains(args, "--load-extension") || slices.Contains(args, "--disable-extensions-except") {
		t.Fatalf("args = %#v, production extensions must not be loaded from startup flags", args)
	}
}

func TestBuildBrowserLaunchArgsKeepsDevelopmentExtensionLoading(t *testing.T) {
	args := buildBrowserLaunchArgs("profile-dir", 9222, "direct://", []string{"C:\\dev-extension"}, nil, nil, nil, nil, false)
	if !slices.Contains(args, "--load-extension=C:\\dev-extension") {
		t.Fatalf("args = %#v, want development extension load flag", args)
	}
	if slices.Contains(args, "--disable-extensions-except=C:\\dev-extension") {
		t.Fatalf("args = %#v, development extension must not disable persistent extensions", args)
	}
}
