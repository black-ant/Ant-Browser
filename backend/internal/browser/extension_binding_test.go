package browser

import "testing"

// 内置插件的逻辑 ID 不是 32 位合法 Chrome ID，绑定列表必须原样保留，不能被格式过滤丢弃。
func TestDedupeExtensionIDsKeepsBuiltinLogicalIDs(t *testing.T) {
	const builtinID = "ababababcdcdcdefefefefghghghgh"
	const normalID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	ids := dedupeExtensionIDs([]string{" " + builtinID + " ", "", builtinID, normalID})
	if len(ids) != 2 {
		t.Fatalf("去重结果 = %#v, want 2 项", ids)
	}
	if ids[0] != builtinID {
		t.Fatalf("内置插件逻辑 ID 被过滤或顺序错乱: %#v", ids)
	}
	if ids[1] != normalID {
		t.Fatalf("普通 ID 顺序错误: %#v", ids)
	}
}

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
