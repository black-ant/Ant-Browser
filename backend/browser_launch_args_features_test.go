package backend

import (
	"reflect"
	"testing"
)

func TestMergeFeatureSwitchesCombinesDuplicates(t *testing.T) {
	in := []string{
		"--user-data-dir=/x",
		"--disable-features=Translate,MediaRouter",
		"--fingerprint=7",
		"--disable-features=MediaRouter,LiveCaption",
		"--enable-features=Foo",
		"--enable-features=Bar",
	}
	got := mergeFeatureSwitches(in)
	want := []string{
		"--user-data-dir=/x",
		"--disable-features=Translate,MediaRouter,LiveCaption",
		"--fingerprint=7",
		"--enable-features=Foo,Bar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestMergeFeatureSwitchesNoOpWithoutDuplicates(t *testing.T) {
	in := []string{"--disable-features=Translate", "--mute-audio"}
	if got := mergeFeatureSwitches(in); !reflect.DeepEqual(got, in) {
		t.Fatalf("got %v want %v", got, in)
	}
}

func TestMergeFeatureSwitchesDropsEmptyValueSwitch(t *testing.T) {
	in := []string{"--disable-features=", "--mute-audio"}
	want := []string{"--mute-audio"}
	if got := mergeFeatureSwitches(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
