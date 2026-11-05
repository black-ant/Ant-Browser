package backend

import (
	"fmt"
	"strings"
	"sync"
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
	oldNode := testClashNode(t, "神秘节点 X", map[string]interface{}{"type": "ss", "server": "9.9.9.9", "port": 1})
	meta := proxySourceMeta{SourceID: "src-1", SourceURL: "https://sub.example.com/list"}
	pool := []BrowserProxy{testProxyRow("proxy-old-1", "神秘节点 X", oldNode, "src-1")}

	newUS := testClashNode(t, "US-01", map[string]interface{}{"type": "ss", "server": "8.8.8.8", "port": 2})
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

func TestCollectProxySourceMetasAggregatesFetchProxy(t *testing.T) {
	row1 := BrowserProxy{ProxyId: "p1", ProxyName: "n1", ProxyConfig: "ss://x", SourceID: "src-1", SourceURL: "https://s1"}
	row2 := BrowserProxy{ProxyId: "p2", ProxyName: "n2", ProxyConfig: "ss://x", SourceID: "src-1", SourceURL: "https://s1", SourceFetchProxyID: "proxy-fetch-1"}

	metas := collectProxySourceMetas([]BrowserProxy{row1, row2})
	if len(metas) != 1 {
		t.Fatalf("metas = %+v, want 1 个订阅源", metas)
	}
	if metas[0].SourceFetchProxy != "proxy-fetch-1" {
		t.Fatalf("SourceFetchProxy 应取首个非空: %q", metas[0].SourceFetchProxy)
	}

	empty := collectProxySourceMetas([]BrowserProxy{row1})
	if empty[0].SourceFetchProxy != "" {
		t.Fatalf("未配置拉取代理时应为空(直连): %q", empty[0].SourceFetchProxy)
	}
}

func TestBuildRefreshedProxyPoolClearsRefreshError(t *testing.T) {
	meta := proxySourceMeta{SourceID: "src-1", SourceURL: "https://sub.example.com/list", SourceFetchProxy: "proxy-fetch-1"}

	// 名称匹配行：刷新成功后失败记录必须清除。
	matched := testProxyRow("proxy-old-1", "美国 01", testClashNode(t, "美国 01", map[string]interface{}{"type": "ss"}), "src-1")
	matched.SourceLastRefreshError = "上次的错误"

	// 无匹配行：保留并标记异常，但失败记录同样清除（订阅已成功拉取）。
	// 端点与上面两个新节点都不同，避免被端点匹配级合并。
	missing := testProxyRow("proxy-old-2", "神秘节点 X", testClashNode(t, "神秘节点 X", map[string]interface{}{"type": "ss", "server": "9.9.9.9", "port": 1}), "src-1")
	missing.SourceLastRefreshError = "上次的错误"

	plan := buildRefreshedProxyPool(
		[]BrowserProxy{matched, missing},
		meta,
		[]clashNode{
			testClashNode(t, "美国 01", map[string]interface{}{"type": "ss"}),
			testClashNode(t, "美国 02", map[string]interface{}{"type": "ss"}),
		},
		nil,
		newTestIDGenerator(),
		time.Now(),
	)

	matchedRow, ok := findRowByID(plan.merged, "proxy-old-1")
	if !ok {
		t.Fatalf("名称匹配行必须沿用旧 proxyId: %#v", plan.merged)
	}
	if matchedRow.SourceLastRefreshError != "" {
		t.Fatalf("刷新成功后名称匹配行的失败记录应清除: %q", matchedRow.SourceLastRefreshError)
	}

	missingRow, ok := findRowByID(plan.merged, "proxy-old-2")
	if !ok || !missingRow.SourceMissing {
		t.Fatalf("无匹配行应保留并标记异常: %#v", plan.merged)
	}
	if missingRow.SourceLastRefreshError != "" {
		t.Fatalf("刷新成功后异常保留行的失败记录应清除: %q", missingRow.SourceLastRefreshError)
	}

	addedCount := 0
	for _, row := range plan.merged {
		if row.SourceID == "src-1" && row.ProxyName == "美国 02" {
			addedCount++
			if row.SourceFetchProxyID != "proxy-fetch-1" {
				t.Fatalf("新增行应继承订阅源拉取代理: %q", row.SourceFetchProxyID)
			}
		}
	}
	if addedCount == 0 {
		t.Fatalf("订阅节点应正常新增: %#v", plan.merged)
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

// 忽略名单同时被调度器读取与保存动作写入，必须无数据竞争（需在 go test -race 下验证）。
func TestProxySourceIgnoredNamesConcurrentAccess(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = app.proxySourceIgnoredNames("src-share")
			}
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				_ = app.BrowserProxySetSourceIgnoredNames("src-share", []string{fmt.Sprintf("节点-%d-%d", n, j)})
			}
		}(i)
	}
	wg.Wait()
}

// 定时刷新间隔按每个源自己设置：24 小时间隔下，23 小时前刷过不算到期，25 小时前刷过的懒加载补拉。
func TestSelectDueProxySourcesHonorsSourceInterval(t *testing.T) {
	now := time.Now()
	pool := []BrowserProxy{
		{ProxyId: "recent24h", ProxyName: "recent24h", ProxyConfig: "ss://a", SourceID: "src-recent", SourceURL: "https://recent", SourceAutoRefresh: true, SourceRefreshIntervalM: 1440, SourceLastRefreshAt: now.Add(-23 * time.Hour).Format(time.RFC3339)},
		{ProxyId: "overdue24h", ProxyName: "overdue24h", ProxyConfig: "ss://b", SourceID: "src-overdue", SourceURL: "https://overdue", SourceAutoRefresh: true, SourceRefreshIntervalM: 1440, SourceLastRefreshAt: now.Add(-25 * time.Hour).Format(time.RFC3339)},
	}
	due := selectDueProxySources(pool, now)
	if len(due) != 1 || due[0].SourceID != "src-overdue" {
		t.Fatalf("due = %+v, want 仅 src-overdue（24 小时内不应重复拉取，超过的懒加载补拉）", due)
	}
}

// 端到端（真库）：导入写入的定时刷新间隔必须原样持久化；
// 超过间隔的源到期补拉（懒加载），补拉刷新后不再重复到期。
func TestProxySourceRefreshIntervalRoundTripAndDue(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	imported := []BrowserProxy{{
		ProxyId:                "p1",
		ProxyName:              "share 01",
		ProxyConfig:            "ss://a",
		SourceID:               "src-share",
		SourceURL:              "https://sub.example.com/list",
		SourceNamePrefix:       "share",
		SourceAutoRefresh:      true,
		SourceRefreshIntervalM: 1440,
		SourceLastRefreshAt:    time.Now().Add(-25 * time.Hour).Format(time.RFC3339),
	}}
	if err := app.SaveBrowserProxies(imported); err != nil {
		t.Fatalf("SaveBrowserProxies failed: %v", err)
	}

	pool := app.getLatestProxies()
	row, ok := findRowByID(pool, "p1")
	if !ok {
		t.Fatalf("导入的节点应持久化: %#v", pool)
	}
	if !row.SourceAutoRefresh || row.SourceRefreshIntervalM != 1440 {
		t.Fatalf("24 小时间隔应原样落库: %+v", row)
	}

	// 导入后已超 24 小时未刷（懒加载场景：应用期间没开）→ 启动后应到期补拉。
	if due := selectDueProxySources(app.getLatestProxies(), time.Now()); len(due) != 1 || due[0].SourceID != "src-share" {
		t.Fatalf("due = %+v, want src-share 到期补拉", due)
	}

	// 模拟补拉完成：刷新时间拨回现在 → 不应再次到期（避免重复拉取）。
	refreshed := append([]BrowserProxy{}, pool...)
	for i := range refreshed {
		if strings.EqualFold(refreshed[i].ProxyId, "p1") {
			refreshed[i].SourceLastRefreshAt = time.Now().Format(time.RFC3339)
		}
	}
	if err := app.SaveBrowserProxies(refreshed); err != nil {
		t.Fatalf("回写刷新时间失败: %v", err)
	}
	if due := selectDueProxySources(app.getLatestProxies(), time.Now()); len(due) != 0 {
		t.Fatalf("刚刷新过的源不应再次到期: %+v", due)
	}
}

// 更新管理弹窗的"编辑订阅源"：开关/间隔/拉取代理按 sourceId 批量落到该源所有行，
// 其他源不受影响；间隔钳制到 [5,1440]；拉取代理必须存在。
func TestBrowserProxyUpdateSourceConfig(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	imported := []BrowserProxy{
		{ProxyId: "p1", ProxyName: "share 01", ProxyConfig: "ss://a", SourceID: "src-share", SourceURL: "https://s1", SourceAutoRefresh: false, SourceRefreshIntervalM: 0},
		{ProxyId: "p2", ProxyName: "share 02", ProxyConfig: "ss://b", SourceID: "src-share", SourceURL: "https://s1", SourceAutoRefresh: true, SourceRefreshIntervalM: 30},
		{ProxyId: "p3", ProxyName: "other 01", ProxyConfig: "ss://c", SourceID: "src-other", SourceURL: "https://s2", SourceAutoRefresh: true, SourceRefreshIntervalM: 15},
	}
	if err := app.SaveBrowserProxies(imported); err != nil {
		t.Fatalf("SaveBrowserProxies failed: %v", err)
	}

	// 拉取代理不存在必须拒绝。
	if err := app.BrowserProxyUpdateSourceConfig("src-share", true, 30, "proxy-missing"); err == nil {
		t.Fatal("拉取代理不存在应报错")
	}

	// 间隔低于下限应钳制到 5 分钟；拉取代理指向池内节点。
	if err := app.BrowserProxyUpdateSourceConfig("src-share", true, 3, "p3"); err != nil {
		t.Fatalf("BrowserProxyUpdateSourceConfig failed: %v", err)
	}
	pool := app.getLatestProxies()
	for _, id := range []string{"p1", "p2"} {
		row, ok := findRowByID(pool, id)
		if !ok {
			t.Fatalf("行 %s 应存在: %#v", id, pool)
		}
		if !row.SourceAutoRefresh || row.SourceRefreshIntervalM != 5 || row.SourceFetchProxyID != "p3" {
			t.Fatalf("%s 配置应更新且间隔钳制到 5: %+v", id, row)
		}
	}
	otherRow, _ := findRowByID(pool, "p3")
	if !otherRow.SourceAutoRefresh || otherRow.SourceRefreshIntervalM != 15 || otherRow.SourceFetchProxyID != "" {
		t.Fatalf("其他源不应被波及: %+v", otherRow)
	}

	// 关闭自动更新：normalize 会把间隔清零。
	if err := app.BrowserProxyUpdateSourceConfig("src-share", false, 30, ""); err != nil {
		t.Fatalf("关闭自动更新失败: %v", err)
	}
	row, _ := findRowByID(app.getLatestProxies(), "p1")
	if row.SourceAutoRefresh || row.SourceRefreshIntervalM != 0 || row.SourceFetchProxyID != "" {
		t.Fatalf("关闭后应回落手动刷新: %+v", row)
	}

	// 支持用 source_url 定位源（与刷新入口同口径）。
	if err := app.BrowserProxyUpdateSourceConfig("https://s1", true, 60, ""); err != nil {
		t.Fatalf("按 URL 定位源失败: %v", err)
	}
	row, _ = findRowByID(app.getLatestProxies(), "p2")
	if !row.SourceAutoRefresh || row.SourceRefreshIntervalM != 60 {
		t.Fatalf("按 URL 更新应生效: %+v", row)
	}

	// 不存在的源必须报错。
	if err := app.BrowserProxyUpdateSourceConfig("src-none", true, 30, ""); err == nil {
		t.Fatal("订阅源不存在应报错")
	}
}

// 安全控制配套·自动对齐：订阅方改节点名但端点不变时，刷新应自动合并（旧 ID 保留、名字/配置更新），
// 不再走"旧标异常+新加入"的老路。
func TestBuildRefreshedProxyPoolEndpointMatch(t *testing.T) {
	meta := proxySourceMeta{SourceID: "src-1", SourceURL: "https://sub.example.com/list", NamePrefix: "自用-"}
	oldRow := testProxyRow("proxy-old-1", "自用--old-node", testClashNode(t, "old-node", map[string]interface{}{"type": "hysteria2", "server": "1.2.3.4", "port": 50000}), "src-1")

	plan := buildRefreshedProxyPool(
		[]BrowserProxy{oldRow},
		meta,
		[]clashNode{
			testClashNode(t, "renamed-node", map[string]interface{}{"type": "hysteria2", "server": "1.2.3.4", "port": 50000}),
			testClashNode(t, "brand-new", map[string]interface{}{"type": "ss", "server": "5.6.7.8", "port": 8443}),
		},
		nil,
		newTestIDGenerator(),
		time.Now(),
	)

	if plan.result.EndpointMatched != 1 {
		t.Fatalf("端点匹配应命中 1 次: %+v", plan.result)
	}
	row, ok := findRowByID(plan.merged, "proxy-old-1")
	if !ok {
		t.Fatalf("合并必须沿用旧 proxyId: %#v", plan.merged)
	}
	if row.ProxyName != "自用--renamed-node" {
		t.Fatalf("合并后名字应为前缀+新名: %q", row.ProxyName)
	}
	if row.SourceMissing {
		t.Fatal("合并后不应再标记异常")
	}
	hasNew := false
	for _, r := range plan.merged {
		if r.ProxyName == "自用--brand-new" && r.ProxyId != "proxy-old-1" {
			hasNew = true
		}
	}
	if !hasNew {
		t.Fatal("无主新节点应正常新增")
	}
	var endpointPair *ProxyRefreshPair
	for i := range plan.result.Pairs {
		if plan.result.Pairs[i].Match == "endpoint" {
			endpointPair = &plan.result.Pairs[i]
		}
	}
	if endpointPair == nil || endpointPair.OldName != "自用--old-node" || endpointPair.NewName != "自用--renamed-node" {
		t.Fatalf("对齐明细应记录端点配对: %+v", plan.result.Pairs)
	}
}
