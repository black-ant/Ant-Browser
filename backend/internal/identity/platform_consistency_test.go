package identity

import (
	"strings"
	"testing"
)

// 锁定:每条 macos 模板生成的身份,LaunchArgs 恰有一个 --fingerprint-platform 且为 macos,
// 且 UA 含 Mac 标记 —— 平台/UA 自洽,不会与全局 windows 混淆。
func TestMacIdentitiesArePlatformConsistent(t *testing.T) {
	recs, err := EmbeddedPoolRecords()
	if err != nil {
		t.Fatalf("EmbeddedPoolRecords: %v", err)
	}
	checked := 0
	for _, r := range recs {
		if r.Platform != "macos" {
			continue
		}
		checked++
		args := BuildIdentity(r, 12345).LaunchArgs()
		var platCount int
		var uaFull string
		for _, a := range args {
			if strings.HasPrefix(a, "--fingerprint-platform=") {
				platCount++
				if a != "--fingerprint-platform=macos" {
					t.Fatalf("macos 模板平台参数错误: %s", a)
				}
			}
			if strings.HasPrefix(a, "--user-agent=") {
				uaFull = a
			}
		}
		if platCount != 1 {
			t.Fatalf("应恰有 1 个平台参数,实际 %d(rec=%s)", platCount, r.UAFull)
		}
		if uaFull != "" && !strings.Contains(uaFull, "Mac") {
			t.Fatalf("macos 身份 UA 应含 Mac: %s", uaFull)
		}
	}
	if checked == 0 {
		t.Skip("内嵌池无 macos 模板,跳过")
	}
}
