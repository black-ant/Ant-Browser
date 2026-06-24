package backend

import "testing"

func TestTimezoneOverrideFromLaunchArgs(t *testing.T) {
	t.Parallel()

	timezone := timezoneOverrideFromLaunchArgs([]string{
		"--lang=en-SG",
		"--timezone=Asia/Shanghai",
		"--timezone=Asia/Singapore",
	})

	if timezone.TimezoneID != "Asia/Singapore" {
		t.Fatalf("expected latest timezone arg, got %+v", timezone)
	}
	if timezone.Source != "proxy" {
		t.Fatalf("expected trailing injected timezone to be proxy-sourced, got %+v", timezone)
	}
}

func TestTimezoneOverrideFromLaunchArgsSkipsEmpty(t *testing.T) {
	t.Parallel()

	timezone := timezoneOverrideFromLaunchArgs([]string{
		"--timezone=",
		"--lang=en-US",
	})

	if timezone.shouldApply() {
		t.Fatalf("empty timezone should not apply: %+v", timezone)
	}
}
