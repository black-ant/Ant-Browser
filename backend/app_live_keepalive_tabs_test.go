package backend

import "testing"

// 回归:保活必须覆盖实例的所有 page 标签(逐标签空闲计时器都要重置),而非只取第一个。
func TestPageTargetWSURLsSelectsAllPages(t *testing.T) {
	body := []byte(`[
		{"type":"page","webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/page/A"},
		{"type":"service_worker","webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/worker/X"},
		{"type":"page","webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/page/B"},
		{"type":"page","webSocketDebuggerUrl":""},
		{"type":"background_page","webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/bg/Y"}
	]`)
	got := pageTargetWSURLs(body)
	want := []string{
		"ws://127.0.0.1:9222/devtools/page/A",
		"ws://127.0.0.1:9222/devtools/page/B",
	}
	if len(got) != len(want) {
		t.Fatalf("应选出 %d 个 page 标签,实际 %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个不符: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestPageTargetWSURLsHandlesBadJSON(t *testing.T) {
	if got := pageTargetWSURLs([]byte("not json")); got != nil {
		t.Fatalf("坏 JSON 应返回 nil,实际 %v", got)
	}
	if got := pageTargetWSURLs([]byte("[]")); len(got) != 0 {
		t.Fatalf("空数组应返回空,实际 %v", got)
	}
}
