package backend

import "testing"

// keepAliveShouldInject 抽出"是否对该实例注入"的判定,便于单测(纯函数,不触发 CDP)。
func TestKeepAliveShouldInjectRespectsPerProfileSwitch(t *testing.T) {
	cases := []struct {
		name    string
		running bool
		ready   bool
		port    int
		enabled bool
		want    bool
	}{
		{"正常且开", true, true, 9222, true, true},
		{"每实例关", true, true, 9222, false, false},
		{"未就绪", true, false, 9222, true, false},
		{"未运行", false, true, 9222, true, false},
		{"无端口", true, true, 0, true, false},
	}
	for _, c := range cases {
		got := keepAliveShouldInject(c.running, c.ready, c.port, c.enabled)
		if got != c.want {
			t.Fatalf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
