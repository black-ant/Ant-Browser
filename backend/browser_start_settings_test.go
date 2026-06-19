package backend

import (
	"ant-chrome/backend/internal/config"
	"testing"
)

func TestBrowserStartMaxConcurrentClamp(t *testing.T) {
	mk := func(v int) *config.Config {
		c := &config.Config{}
		c.Browser.StartMaxConcurrent = v
		return c
	}

	cases := []struct {
		name string
		cfg  *config.Config
		want int
	}{
		{"nil-config-default", nil, defaultBrowserStartMaxConcurrent},
		{"zero-default", mk(0), defaultBrowserStartMaxConcurrent},
		{"negative-default", mk(-5), defaultBrowserStartMaxConcurrent},
		{"in-range", mk(4), 4},
		{"below-min", mk(0), defaultBrowserStartMaxConcurrent}, // 0 视为未配置→默认
		{"above-max", mk(999), maxBrowserStartMaxConcurrent},
		{"at-min", mk(minBrowserStartMaxConcurrent), minBrowserStartMaxConcurrent},
		{"at-max", mk(maxBrowserStartMaxConcurrent), maxBrowserStartMaxConcurrent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := browserStartMaxConcurrent(tc.cfg); got != tc.want {
				t.Errorf("browserStartMaxConcurrent(%v) = %d, want %d", tc.cfg, got, tc.want)
			}
		})
	}
}
