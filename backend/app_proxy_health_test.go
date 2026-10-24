package backend

import "testing"

func TestProxySpeedBatchConcurrencyClamp(t *testing.T) {
	app := NewApp("")
	cases := []struct {
		name      string
		requested int
		total     int
		want      int
	}{
		{"frontend default 20 runs at 20", 20, 100, 20},
		{"zero falls back to default 5", 0, 100, 5},
		{"oversized request clamps to 20", 100, 100, 20},
		{"never exceeds total", 20, 3, 3},
		{"empty total", 20, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := app.proxySpeedBatchConcurrency(tc.requested, tc.total); got != tc.want {
				t.Fatalf("proxySpeedBatchConcurrency(%d, %d) = %d, want %d", tc.requested, tc.total, got, tc.want)
			}
		})
	}
}
