package browser

import (
	"reflect"
	"testing"
)

func TestBuildLaunchArgsAppendsConfiguredStartURLs(t *testing.T) {
	t.Parallel()

	baseArgs := []string{"--disable-sync"}
	got := BuildLaunchArgs(append([]string{}, baseArgs...), []string{"https://example.com/", ""})
	want := []string{
		"--disable-sync",
		"https://example.com/",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildLaunchArgs 结果错误:\n got=%v\nwant=%v", got, want)
	}
}

func TestBuildLaunchArgsEmptyStartURLsNoop(t *testing.T) {
	t.Parallel()

	baseArgs := []string{"--disable-sync"}
	got := BuildLaunchArgs(append([]string{}, baseArgs...), nil)
	want := []string{"--disable-sync"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildLaunchArgs 空起始页应原样返回:\n got=%v\nwant=%v", got, want)
	}
}
