package backend

import (
	"reflect"
	"testing"
)

func TestBrowserProcessLanguageEnvDerivesLanguageFromLangArg(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{name: "region tag", args: []string{"--fingerprint=1", "--lang=en-US"}, want: []string{"LANGUAGE=en_US:en"}},
		{name: "chinese", args: []string{"--lang=zh-CN", "--accept-lang=zh-CN,zh"}, want: []string{"LANGUAGE=zh_CN:zh"}},
		{name: "base only", args: []string{"--lang=de"}, want: []string{"LANGUAGE=de"}},
		{name: "last wins", args: []string{"--lang=zh-CN", "--lang=en-GB"}, want: []string{"LANGUAGE=en_GB:en"}},
		{name: "no lang", args: []string{"--fingerprint=1", "--accept-lang=en-US,en"}, want: nil},
		{name: "empty lang", args: []string{"--lang="}, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := browserProcessLanguageEnv("linux", tc.args)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("browserProcessLanguageEnv(linux, %v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestBrowserProcessLanguageEnvOnlyAppliesToLinux(t *testing.T) {
	args := []string{"--lang=en-US"}
	for _, hostOS := range []string{"windows", "darwin", "", "freebsd"} {
		if got := browserProcessLanguageEnv(hostOS, args); got != nil {
			t.Fatalf("browserProcessLanguageEnv(%q) = %v, want nil", hostOS, got)
		}
	}
}

func TestBrowserProcessEnvReplacesExistingKeysAndKeepsOthers(t *testing.T) {
	base := []string{"PATH=/usr/bin", "LANGUAGE=zh_CN:zh", "LANG=zh_CN.UTF-8", "NOEQUALS"}
	got := browserProcessEnv(base, []string{"LANGUAGE=en_US:en"})
	want := []string{"PATH=/usr/bin", "LANG=zh_CN.UTF-8", "NOEQUALS", "LANGUAGE=en_US:en"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("browserProcessEnv = %v, want %v", got, want)
	}
}

func TestBrowserProcessEnvReturnsNilWithoutOverrides(t *testing.T) {
	if got := browserProcessEnv([]string{"PATH=/usr/bin"}, nil); got != nil {
		t.Fatalf("browserProcessEnv without overrides = %v, want nil so exec.Cmd inherits the parent environment", got)
	}
}
