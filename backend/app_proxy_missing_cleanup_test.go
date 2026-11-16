package backend

import "testing"

func cleanupTestRow(proxyID string, name string, sourceID string, sourceURL string, missing bool) BrowserProxy {
	return BrowserProxy{
		ProxyId:       proxyID,
		ProxyName:     name,
		SourceID:      sourceID,
		SourceURL:     sourceURL,
		SourceMissing: missing,
	}
}

func TestPlanProxyMissingCleanupDeletesUnreferenced(t *testing.T) {
	pool := []BrowserProxy{
		cleanupTestRow("proxy-keep", "正常节点", "src-1", "https://sub.example.com/list?a=1", false),
		cleanupTestRow("proxy-miss-1", "异常节点 1", "src-1", "https://sub.example.com/list?a=1", true),
		cleanupTestRow("proxy-miss-2", "异常节点 2", "src-1", "https://sub.example.com/list?a=1", true),
	}

	remaining, result := planProxyMissingCleanup(pool, "", map[string]string{}, map[string]string{}, map[string]string{})

	if result.Scanned != 2 || result.Deletable != 2 || len(result.Skipped) != 0 {
		t.Fatalf("result = %+v", result)
	}
	if len(remaining) != 1 || remaining[0].ProxyId != "proxy-keep" {
		t.Fatalf("remaining = %+v", remaining)
	}
}

func TestPlanProxyMissingCleanupSkipsInstanceReference(t *testing.T) {
	pool := []BrowserProxy{
		cleanupTestRow("proxy-miss-1", "异常节点 1", "src-1", "https://sub.example.com/list", true),
		cleanupTestRow("proxy-miss-2", "异常节点 2", "src-1", "https://sub.example.com/list", true),
	}
	instanceRefs := map[string]string{"proxy-miss-1": "Muse"}

	remaining, result := planProxyMissingCleanup(pool, "", instanceRefs, map[string]string{}, map[string]string{})

	if result.Scanned != 2 || result.Deletable != 1 || len(result.Skipped) != 1 {
		t.Fatalf("result = %+v", result)
	}
	skip := result.Skipped[0]
	if skip.ProxyId != "proxy-miss-1" || skip.Reason != proxyMissingSkipInstance || skip.Detail != "Muse" {
		t.Fatalf("skip = %+v", skip)
	}
	if len(remaining) != 1 || remaining[0].ProxyId != "proxy-miss-1" {
		t.Fatalf("被实例引用的异常节点必须保留: %+v", remaining)
	}
}

func TestPlanProxyMissingCleanupSkipsVirtualGroupReference(t *testing.T) {
	pool := []BrowserProxy{
		cleanupTestRow("PROXY-MISS-1", "异常节点 1", "src-1", "https://sub.example.com/list", true),
	}
	// 引用键按小写匹配，代理行 ID 大写也必须命中。
	groupRefs := map[string]string{"proxy-miss-1": "US"}

	remaining, result := planProxyMissingCleanup(pool, "", map[string]string{}, groupRefs, map[string]string{})

	if result.Deletable != 0 || len(result.Skipped) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if result.Skipped[0].Reason != proxyMissingSkipVirtualGroup || result.Skipped[0].Detail != "US" {
		t.Fatalf("skip = %+v", result.Skipped[0])
	}
	if len(remaining) != 1 {
		t.Fatalf("被虚拟分组引用的异常节点必须保留: %+v", remaining)
	}
}

func TestPlanProxyMissingCleanupFiltersBySource(t *testing.T) {
	pool := []BrowserProxy{
		cleanupTestRow("proxy-a", "源 A 异常", "src-a", "https://a.example.com/list", true),
		cleanupTestRow("proxy-b", "源 B 异常", "src-b", "https://b.example.com/list", true),
	}

	remaining, result := planProxyMissingCleanup(pool, "src-a", map[string]string{}, map[string]string{}, map[string]string{})

	if result.Scanned != 1 || result.Deletable != 1 {
		t.Fatalf("result = %+v", result)
	}
	if len(remaining) != 1 || remaining[0].ProxyId != "proxy-b" {
		t.Fatalf("未选中源的异常行必须保留: %+v", remaining)
	}

	// 用订阅 URL 定位源，与前端传 sourceUrl 的口径一致。
	remaining, result = planProxyMissingCleanup(pool, "https://b.example.com/list", map[string]string{}, map[string]string{}, map[string]string{})
	if result.Scanned != 1 || result.Deletable != 1 {
		t.Fatalf("by url result = %+v", result)
	}
	if len(remaining) != 1 || remaining[0].ProxyId != "proxy-a" {
		t.Fatalf("by url remaining = %+v", remaining)
	}
}

func TestPlanProxyMissingCleanupKeepsRowsWithoutID(t *testing.T) {
	pool := []BrowserProxy{
		cleanupTestRow("", "无主键异常行", "src-1", "https://sub.example.com/list", true),
	}

	remaining, result := planProxyMissingCleanup(pool, "", map[string]string{}, map[string]string{}, map[string]string{})

	if result.Scanned != 1 || result.Deletable != 0 {
		t.Fatalf("result = %+v", result)
	}
	if len(remaining) != 1 {
		t.Fatalf("缺少 proxyId 的行不得删除: %+v", remaining)
	}
}

func TestPlanProxyMissingCleanupSkipsFetchProxy(t *testing.T) {
	pool := []BrowserProxy{
		cleanupTestRow("proxy-miss-1", "订阅拉取代理", "src-1", "https://sub.example.com/list", true),
	}
	pool[0].SourceFetchProxyID = "proxy-miss-1"
	fetchRefs := map[string]string{"proxy-miss-1": "src-1"}

	remaining, result := planProxyMissingCleanup(pool, "", map[string]string{}, map[string]string{}, fetchRefs)

	if result.Deletable != 0 || len(result.Skipped) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if result.Skipped[0].Reason != proxyMissingSkipFetchProxy || result.Skipped[0].Detail != "src-1" {
		t.Fatalf("skip = %+v", result.Skipped[0])
	}
	if len(remaining) != 1 {
		t.Fatalf("订阅拉取代理必须保留: %+v", remaining)
	}
}
