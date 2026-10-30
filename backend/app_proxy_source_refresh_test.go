package backend

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func newTestIDGenerator() func() string {
	counter := 0
	return func() string {
		counter++
		return fmt.Sprintf("proxy-new-%d", counter)
	}
}

func testClashNode(t *testing.T, name string, fields map[string]interface{}) clashNode {
	t.Helper()
	if fields == nil {
		fields = map[string]interface{}{}
	}
	fields["name"] = name
	if _, ok := fields["server"]; !ok {
		fields["server"] = "1.2.3.4"
	}
	if _, ok := fields["port"]; !ok {
		fields["port"] = 443
	}
	return clashNode{Name: name, Fields: fields}
}

func testProxyRow(proxyID string, name string, node clashNode, sourceID string) BrowserProxy {
	configText, err := clashNodeConfigYAML(node)
	if err != nil {
		panic(err)
	}
	return BrowserProxy{
		ProxyId:     proxyID,
		ProxyName:   name,
		ProxyConfig: configText,
		SourceID:    sourceID,
		SourceURL:   "https://sub.example.com/list",
	}
}

func findRowByID(proxies []BrowserProxy, proxyID string) (BrowserProxy, bool) {
	for _, row := range proxies {
		if strings.EqualFold(row.ProxyId, proxyID) {
			return row, true
		}
	}
	return BrowserProxy{}, false
}

func containsProxyID(proxies []BrowserProxy, proxyID string) bool {
	_, ok := findRowByID(proxies, proxyID)
	return ok
}

func TestInferCountryCodeFromName(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"美国 家宽 01", "US"},
		{"US-IAD-1", "US"},
		{"🇺🇸 Los Angeles", "US"},
		{"香港 IEPL", "HK"},
		{"russia 01", "RU"},
		{"austria 01", ""}, // "au" 不得命中 austria/australia
		{"australia 01", "AU"},
		{"节点 1", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := inferCountryCodeFromName(tc.name); got != tc.want {
			t.Fatalf("inferCountryCodeFromName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestBuildRefreshedProxyPoolNameMatchKeepsID(t *testing.T) {
	oldNode := testClashNode(t, "美国 01", map[string]interface{}{"type": "ss", "cipher": "aes-128-gcm", "password": "p1"})
	newNode := testClashNode(t, "美国 01", map[string]interface{}{"type": "ss", "cipher": "aes-128-gcm", "password": "p1", "server": "5.6.7.8"})
	meta := proxySourceMeta{SourceID: "src-1", SourceURL: "https://sub.example.com/list"}
	pool := []BrowserProxy{testProxyRow("proxy-old-1", "美国 01", oldNode, "src-1")}
	pool[0].PreferredKernel = "sing-box"

	plan := buildRefreshedProxyPool(pool, meta, []clashNode{newNode}, nil, newTestIDGenerator(), time.Unix(1700000000, 0))

	if plan.result.NameMatched != 1 || plan.result.Added != 0 || plan.result.Total != 1 {
		t.Fatalf("result = %+v, want nameMatched=1 total=1", plan.result)
	}
	row, ok := findRowByID(plan.merged, "proxy-old-1")
	if !ok {
		t.Fatalf("名称匹配后必须沿用旧 proxyId: %#v", plan.merged)
	}
	if !strings.Contains(row.ProxyConfig, "5.6.7.8") {
		t.Fatalf("配置未更新: %s", row.ProxyConfig)
	}
	if row.PreferredKernel != "sing-box" {
		t.Fatalf("同协议刷新不应清空 preferred_kernel: %q", row.PreferredKernel)
	}
	if row.SourceMissing {
		t.Fatal("匹配成功不应标记异常")
	}
	if row.SourceLastRefreshAt == "" {
		t.Fatal("刷新时间未更新")
	}
}

func TestBuildRefreshedProxyPoolCountryFallbackReplacesByID(t *testing.T) {
	oldNode := testClashNode(t, "美国 旧", map[string]interface{}{"type": "trojan"})
	meta := proxySourceMeta{SourceID: "src-1", SourceURL: "https://sub.example.com/list"}
	pool := []BrowserProxy{testProxyRow("proxy-old-1", "美国 旧", oldNode, "src-1")}
	pool[0].PreferredKernel = "mihomo"

	newUS := testClashNode(t, "US-NewYork-07", map[string]interface{}{"type": "vless"})
	newJP := testClashNode(t, "JP-01", map[string]interface{}{"type": "vless"})

	plan := buildRefreshedProxyPool(pool, meta, []clashNode{newUS, newJP}, nil, newTestIDGenerator(), time.Now())

	if plan.result.CountryMatched != 1 || plan.result.Added != 1 || plan.result.MarkedMissing != 0 {
		t.Fatalf("result = %+v, want countryMatched=1 added=1", plan.result)
	}
	row, ok := findRowByID(plan.merged, "proxy-old-1")
	if !ok {
		t.Fatalf("同国家替换应沿用旧 proxyId（实例绑定不变）: %#v", plan.merged)
	}
	if row.ProxyName != "US-NewYork-07" || !strings.Contains(row.ProxyConfig, "vless") {
		t.Fatalf("应替换为同国家新节点: %s / %s", row.ProxyName, row.ProxyConfig)
	}
	if row.PreferredKernel != "" {
		t.Fatalf("协议变化后 preferred_kernel 必须清空（跨栈指定会被拒绝）: %q", row.PreferredKernel)
	}
	if !containsProxyID(plan.merged, "proxy-new-1") && !containsProxyID(plan.merged, "proxy-new-2") {
		t.Fatalf("JP 新节点应正常新增: %#v", plan.merged)
	}
}

func TestBuildRefreshedProxyPoolMarksMissingWithoutMatch(t *testing.T) {
	oldNode := testClashNode(t, "神秘节点 X", map[string]interface{}{"type": "ss"})
	meta := proxySourceMeta{SourceID: "src-1", SourceURL: "https://sub.example.com/list"}
	pool := []BrowserProxy{testProxyRow("proxy-old-1", "神秘节点 X", oldNode, "src-1")}

	newUS := testClashNode(t, "US-01", map[string]interface{}{"type": "ss"})
	plan := buildRefreshedProxyPool(pool, meta, []clashNode{newUS}, nil, newTestIDGenerator(), time.Unix(1700000000, 0))

	if plan.result.MarkedMissing != 1 || plan.result.Added != 1 {
		t.Fatalf("result = %+v, want markedMissing=1 added=1", plan.result)
	}
	row, ok := findRowByID(plan.merged, "proxy-old-1")
	if !ok {
		t.Fatal("匹配不上的旧节点必须保留在池内（实例绑定不动）")
	}
	if !row.SourceMissing || row.SourceMissingSince == "" {
		t.Fatalf("旧节点应标记异常: %+v", row)
	}
	if row.ProxyName != "神秘节点 X" {
		t.Fatal("不应替换节点内容")
	}
}

func TestBuildRefreshedProxyPoolMissingRecoversByName(t *testing.T) {
	oldNode := testClashNode(t, "美国 01", map[string]interface{}{"type": "ss"})
	meta := proxySourceMeta{SourceID: "src-1", SourceURL: "https://sub.example.com/list"}
	pool := []BrowserProxy{testProxyRow("proxy-old-1", "美国 01", oldNode, "src-1")}
	pool[0].SourceMissing = true
	pool[0].SourceMissingSince = "2026-01-01T00:00:00Z"

	plan := buildRefreshedProxyPool(pool, meta, []clashNode{testClashNode(t, "美国 01", map[string]interface{}{"type": "ss"})}, nil, newTestIDGenerator(), time.Now())

	if plan.result.MarkedMissing != 0 {
		t.Fatalf("result = %+v", plan.result)
	}
	row, _ := findRowByID(plan.merged, "proxy-old-1")
	if row.SourceMissing || row.SourceMissingSince != "" {
		t.Fatalf("重新出现后应清除异常标记: %+v", row)
	}
}

func TestBuildRefreshedProxyPoolAppliesIgnoredNames(t *testing.T) {
	meta := proxySourceMeta{SourceID: "src-1", SourceURL: "https://sub.example.com/list"}
	oldNode := testClashNode(t, "美国 01", map[string]interface{}{"type": "ss"})
	pool := []BrowserProxy{testProxyRow("proxy-old-1", "美国 01", oldNode, "src-1")}

	nodes := []clashNode{
		testClashNode(t, "美国 01", map[string]interface{}{"type": "ss"}),
		testClashNode(t, "HK-01", map[string]interface{}{"type": "ss"}),
	}
	plan := buildRefreshedProxyPool(pool, meta, nodes, []string{"HK-01"}, newTestIDGenerator(), time.Now())

	if plan.result.Fetched != 1 {
		t.Fatalf("忽略名单未生效: %+v", plan.result)
	}
	if containsProxyID(plan.merged, "proxy-new-2") {
		t.Fatal("被忽略节点不应入库")
	}
}

// 回归：只有 source_url 没有 source_id 的订阅行（手工配置/旧数据），
// 分组键退回 URL，刷新时必须仍能按名称匹配，而不是把旧行当新节点重复堆积。
func TestBuildRefreshedProxyPoolURLOnlySource(t *testing.T) {
	oldNode := testClashNode(t, "美国 01", map[string]interface{}{"type": "ss"})
	row := testProxyRow("proxy-old-1", "美国 01", oldNode, "")
	row.SourceID = ""
	row.SourceURL = "https://sub.example.com/list"
	pool := []BrowserProxy{row}

	meta := proxySourceMeta{SourceID: "", SourceURL: "https://sub.example.com/list"}
	plan := buildRefreshedProxyPool(pool, meta, []clashNode{testClashNode(t, "美国 01", map[string]interface{}{"type": "ss"})}, nil, newTestIDGenerator(), time.Now())

	if plan.result.NameMatched != 1 || plan.result.Added != 0 || plan.result.Total != 1 {
		t.Fatalf("result = %+v, want nameMatched=1 added=0 total=1", plan.result)
	}
	count := 0
	for _, item := range plan.merged {
		if strings.EqualFold(item.ProxyName, "美国 01") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("同名节点应只有 1 条, got %d", count)
	}
	_, ok := findRowByID(plan.merged, "proxy-old-1")
	if !ok {
		t.Fatal("URL 键来源的旧行应沿用旧 proxyId")
	}
}

func TestBuildRefreshedProxyPoolKeepsOtherSourcesUntouched(t *testing.T) {
	meta := proxySourceMeta{SourceID: "src-1", SourceURL: "https://sub.example.com/list"}
	otherNode := testClashNode(t, "手工节点", map[string]interface{}{"type": "ss"})
	other := testProxyRow("manual-1", "手工节点", otherNode, "")
	other.SourceID = ""
	other.SourceURL = ""

	oldNode := testClashNode(t, "美国 旧", map[string]interface{}{"type": "ss"})
	pool := []BrowserProxy{other, testProxyRow("proxy-old-1", "美国 旧", oldNode, "src-1")}

	plan := buildRefreshedProxyPool(pool, meta, []clashNode{testClashNode(t, "US-01", map[string]interface{}{"type": "ss"})}, nil, newTestIDGenerator(), time.Now())

	row, ok := findRowByID(plan.merged, "manual-1")
	if !ok {
		t.Fatalf("非订阅源节点必须原样保留: %#v", plan.merged)
	}
	if row.SourceMissing {
		t.Fatal("非订阅源节点不应被标记异常")
	}
}

func TestCollectProxySourceMetasGroupsAndAggregates(t *testing.T) {
	now := time.Unix(1700000000, 0)
	a1 := now.Add(-30 * time.Minute).Format(time.RFC3339)
	a2 := now.Add(-10 * time.Minute).Format(time.RFC3339)

	row1 := BrowserProxy{ProxyId: "p1", ProxyName: "n1", ProxyConfig: "ss://x", SourceID: "src-1", SourceURL: "https://s1", SourceAutoRefresh: false, SourceRefreshIntervalM: 30, SourceLastRefreshAt: a1, GroupName: "G1", DnsServers: "dns"}
	row2 := BrowserProxy{ProxyId: "p2", ProxyName: "n2", ProxyConfig: "ss://x", SourceID: "src-1", SourceURL: "https://s1", SourceAutoRefresh: true, SourceLastRefreshAt: a2}
	row3 := BrowserProxy{ProxyId: "p3", ProxyName: "n3", ProxyConfig: "ss://x", SourceID: "src-2", SourceURL: "https://s2", SourceAutoRefresh: true, SourceRefreshIntervalM: 15}

	metas := collectProxySourceMetas([]BrowserProxy{row1, row2, row3})
	if len(metas) != 2 {
		t.Fatalf("metas = %+v, want 2 个订阅源", metas)
	}
	first := metas[0]
	if first.SourceID != "src-1" || first.IntervalM != 30 || !first.AutoRefresh {
		t.Fatalf("src-1 聚合错误: %+v", first)
	}
	if first.GroupName != "G1" || first.DnsServers != "dns" {
		t.Fatalf("元数据取首个非空: %+v", first)
	}
	if parseProxyTimestampMs(first.LastRefreshAt) != parseProxyTimestampMs(a2) {
		t.Fatalf("LastRefreshAt 应取最新: %q", first.LastRefreshAt)
	}
	if metas[1].IntervalM != 15 {
		t.Fatalf("src-2 间隔 = %d, want 15", metas[1].IntervalM)
	}
}

func TestParseClashNodesFromPayload(t *testing.T) {
	payload := map[string]interface{}{
		"proxies": []interface{}{
			map[string]interface{}{"name": "US-01", "type": "ss", "server": "1.1.1.1", "port": 443},
			map[string]interface{}{"type": "ss", "server": "2.2.2.2", "port": 443}, // 无名称 → 兜底命名
			"garbage",
		},
	}
	nodes := parseClashNodes(payload)
	if len(nodes) != 2 {
		t.Fatalf("nodes = %+v, want 2", nodes)
	}
	if nodes[0].Name != "US-01" {
		t.Fatalf("node[0].Name = %q", nodes[0].Name)
	}
	if nodes[1].Name == "" {
		t.Fatal("无名称节点应有兜底命名")
	}
}

func TestClashConfigMatchesNodeIgnoresKeyOrder(t *testing.T) {
	node := testClashNode(t, "US-01", map[string]interface{}{"type": "ss", "cipher": "aes-128-gcm", "password": "p"})
	configText, err := clashNodeConfigYAML(node)
	if err != nil {
		t.Fatal(err)
	}
	if !clashConfigMatchesNode(configText, node) {
		t.Fatal("同一节点的 YAML 文本与结构应语义一致")
	}
	changed := testClashNode(t, "US-01", map[string]interface{}{"type": "ss", "cipher": "aes-128-gcm", "password": "p", "server": "9.9.9.9"})
	if clashConfigMatchesNode(configText, changed) {
		t.Fatal("服务器变化不应视为同一节点")
	}
}

func TestProxySourceRefreshSchedulerDueWindow(t *testing.T) {
	now := time.Now()
	stale := now.Add(-90 * time.Minute).Format(time.RFC3339)
	fresh := now.Add(-10 * time.Minute).Format(time.RFC3339)
	pool := []BrowserProxy{
		{ProxyId: "due", ProxyName: "due", ProxyConfig: "ss://a", SourceID: "src-due", SourceURL: "https://due", SourceAutoRefresh: true, SourceLastRefreshAt: stale},
		{ProxyId: "fresh", ProxyName: "fresh", ProxyConfig: "ss://b", SourceID: "src-fresh", SourceURL: "https://fresh", SourceAutoRefresh: true, SourceLastRefreshAt: fresh},
	}
	due := selectDueProxySources(pool, now)
	if len(due) != 1 {
		t.Fatalf("due = %d, want 1（60 分钟间隔内不应重复刷新）", len(due))
	}
	if due[0].SourceID != "src-due" {
		t.Fatalf("到期源 = %q, want src-due", due[0].SourceID)
	}
}

// 导入弹窗的"自动更新订阅"开关必须真正生效：关掉的源不参与定时拉取。
func TestSelectDueProxySourcesRespectsAutoRefresh(t *testing.T) {
	now := time.Now()
	pool := []BrowserProxy{
		{ProxyId: "auto", ProxyName: "auto", ProxyConfig: "ss://a", SourceID: "src-auto", SourceURL: "https://auto", SourceAutoRefresh: true, SourceLastRefreshAt: now.Add(-90 * time.Minute).Format(time.RFC3339)},
		{ProxyId: "manual", ProxyName: "manual", ProxyConfig: "ss://b", SourceID: "src-manual", SourceURL: "https://manual", SourceAutoRefresh: false, SourceLastRefreshAt: now.Add(-90 * time.Minute).Format(time.RFC3339)},
	}
	due := selectDueProxySources(pool, now)
	if len(due) != 1 || due[0].SourceID != "src-auto" {
		t.Fatalf("due = %+v, want 仅 src-auto（关闭自动更新的源不参与定时拉取）", due)
	}
}
