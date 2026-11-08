package browser

import "testing"

// 绑定里存在但插件记录已丢失的 ID 必须被识别出来（顺序保持、去重、忽略空白）。
func TestMissingExtensionIDs(t *testing.T) {
	loaded := []Extension{
		{ExtensionID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{ExtensionID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	bound := []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		" cccccccccccccccccccccccccccccccc ",
		"cccccccccccccccccccccccccccccccc",
		"",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	missing := missingExtensionIDs(bound, loaded)
	if len(missing) != 1 || missing[0] != "cccccccccccccccccccccccccccccccc" {
		t.Fatalf("missing = %#v, want 仅缺失的 c 插件", missing)
	}
	if got := missingExtensionIDs(nil, loaded); got != nil {
		t.Fatalf("无绑定时应返回 nil: %#v", got)
	}
	if got := missingExtensionIDs(bound, nil); len(got) != 3 {
		t.Fatalf("全部记录丢失时应返回全部绑定: %#v", got)
	}
}
