package backend

import (
	"slices"
	"strings"
	"testing"
)

func TestBuildBrowserLaunchArgsLoadsExtensionsFromStartupFlags(t *testing.T) {
	paths := []string{`D:\data\extensions\aaa`, `D:\data\extensions\bbb`}
	args := buildBrowserLaunchArgs("profile-dir", 9222, "direct://", nil, nil, nil, paths, nil, false)

	want := "--load-extension=" + strings.Join(paths, ",")
	if !slices.Contains(args, want) {
		t.Fatalf("args = %#v, want %q", args, want)
	}
	// 不加 --disable-extensions-except，避免把实例里已有的其他插件一起禁用。
	for _, arg := range args {
		if strings.HasPrefix(arg, "--disable-extensions-except") {
			t.Fatalf("args = %#v, must not restrict other extensions", args)
		}
	}
}

func TestBuildBrowserLaunchArgsOmitsLoadExtensionWithoutPlugins(t *testing.T) {
	args := buildBrowserLaunchArgs("profile-dir", 9222, "direct://", nil, nil, nil, nil, nil, false)
	for _, arg := range args {
		if strings.HasPrefix(arg, "--load-extension") {
			t.Fatalf("args = %#v, must not load extensions when none are configured", args)
		}
	}
}
