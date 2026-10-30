package backend

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"

	"gopkg.in/yaml.v3"
)

// ProxySourceRefreshResult 一次订阅源刷新的结果统计。
type ProxySourceRefreshResult struct {
	SourceId       string `json:"sourceId"`
	SourceURL      string `json:"sourceUrl"`
	Fetched        int    `json:"fetched"`        // 订阅解析出的节点数（已过滤忽略名单）
	NameMatched    int    `json:"nameMatched"`    // 按名称匹配沿用旧 ID
	CountryMatched int    `json:"countryMatched"` // 名称匹配不上时按同国家替换
	Added          int    `json:"added"`          // 全新节点
	MarkedMissing  int    `json:"markedMissing"`  // 名称/国家都匹配不上，标记为异常的旧节点
	Total          int    `json:"total"`          // 该源最终保留的节点数
}

// proxySourceMeta 从代理行聚合出的订阅源元数据。
type proxySourceMeta struct {
	SourceID      string
	SourceURL     string
	NamePrefix    string
	GroupName     string
	DnsServers    string
	AutoRefresh   bool
	IntervalM     int
	LastRefreshAt string
}

// clashNode 订阅解析出的单个节点：原始名称 + 字段集。
type clashNode struct {
	Name   string
	Fields map[string]interface{}
}

// proxySourceKey 订阅源的分组键：优先 sourceId，缺失时退回 source_url。
func proxySourceKey(row BrowserProxy) string {
	if id := strings.TrimSpace(row.SourceID); id != "" {
		return id
	}
	return strings.TrimSpace(row.SourceURL)
}

// proxySourceMetaKey 与 proxySourceKey 同一口径的订阅源分组键。
func proxySourceMetaKey(meta proxySourceMeta) string {
	if id := strings.TrimSpace(meta.SourceID); id != "" {
		return id
	}
	return strings.TrimSpace(meta.SourceURL)
}

// collectProxySourceMetas 聚合代理池里全部 URL 订阅源。
func collectProxySourceMetas(proxies []BrowserProxy) []proxySourceMeta {
	type accumulator struct {
		meta      proxySourceMeta
		lastMs    int64
		hasRow    bool
		intervalM int
	}
	groups := make(map[string]*accumulator)
	for _, row := range proxies {
		sourceURL := strings.TrimSpace(row.SourceURL)
		key := proxySourceKey(row)
		if key == "" || sourceURL == "" {
			continue
		}
		item := groups[key]
		if item == nil {
			item = &accumulator{meta: proxySourceMeta{
				SourceID:    strings.TrimSpace(row.SourceID),
				SourceURL:   sourceURL,
				AutoRefresh: row.SourceAutoRefresh,
			}}
			groups[key] = item
		}
		if prefix := strings.TrimSpace(row.SourceNamePrefix); item.meta.NamePrefix == "" {
			item.meta.NamePrefix = prefix
		}
		if item.meta.GroupName == "" {
			item.meta.GroupName = strings.TrimSpace(row.GroupName)
		}
		if item.meta.DnsServers == "" {
			item.meta.DnsServers = strings.TrimSpace(row.DnsServers)
		}
		if row.SourceAutoRefresh {
			item.meta.AutoRefresh = true
		}
		if row.SourceRefreshIntervalM > item.intervalM {
			item.intervalM = row.SourceRefreshIntervalM
		}
		if ms := parseProxyTimestampMs(row.SourceLastRefreshAt); ms > item.lastMs {
			item.lastMs = ms
			item.meta.LastRefreshAt = strings.TrimSpace(row.SourceLastRefreshAt)
		}
		item.hasRow = true
	}

	result := make([]proxySourceMeta, 0, len(groups))
	for _, item := range groups {
		if !item.hasRow {
			continue
		}
		item.meta.IntervalM = item.intervalM
		result = append(result, item.meta)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].SourceID < result[j].SourceID })
	return result
}

func parseProxyTimestampMs(value string) int64 {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return 0
	}
	return parsed.UnixMilli()
}

// parseClashNodes 从 Clash YAML payload 提取节点列表。
func parseClashNodes(payload interface{}) []clashNode {
	m := toStringMap(payload)
	if m == nil {
		return nil
	}
	arr, ok := m["proxies"].([]interface{})
	if !ok {
		return nil
	}
	nodes := make([]clashNode, 0, len(arr))
	for index, item := range arr {
		fields := toStringMap(item)
		if fields == nil {
			continue
		}
		name := strings.TrimSpace(getMapString(fields, "name"))
		if name == "" {
			name = fmt.Sprintf("导入代理 %d", index+1)
			fields["name"] = name
		}
		nodes = append(nodes, clashNode{Name: name, Fields: fields})
	}
	return nodes
}

// clashNodeConfigYAML 生成与前端 proxyToYaml 一致的单节点 YAML（单元素序列）。
func clashNodeConfigYAML(node clashNode) (string, error) {
	data, err := yaml.Marshal([]interface{}{node.Fields})
	if err != nil {
		return "", fmt.Errorf("序列化节点 %s 失败: %w", node.Name, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// clashConfigMatchesNode 现有 ProxyConfig 是否与订阅节点语义一致（忽略键序/格式差异）。
func clashConfigMatchesNode(configText string, node clashNode) bool {
	parsed, ok := parseClashPayload(configText)
	if !ok {
		return false
	}
	arr, ok := parsed.([]interface{})
	if !ok || len(arr) != 1 {
		return false
	}
	item := toStringMap(arr[0])
	if item == nil {
		return false
	}
	return canonicalClashJSON(item) == canonicalClashJSON(node.Fields)
}

// clashConfigType 提取节点协议类型（type 字段），用于判断 preferred_kernel 是否可以沿用。
func clashConfigType(configText string) string {
	parsed, parsedOK := parseClashPayload(configText)
	if !parsedOK {
		return ""
	}
	if arr, ok := parsed.([]interface{}); ok && len(arr) == 1 {
		if item := toStringMap(arr[0]); item != nil {
			return strings.ToLower(strings.TrimSpace(getMapString(item, "type")))
		}
	}
	return ""
}

func canonicalClashJSON(value interface{}) string {
	data, err := json.Marshal(normalizeYAMLValue(value))
	if err != nil {
		return ""
	}
	return string(data)
}

func normalizeYAMLValue(value interface{}) interface{} {
	switch item := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(item))
		for key, val := range item {
			out[key] = normalizeYAMLValue(val)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(item))
		for key, val := range item {
			out[fmt.Sprint(key)] = normalizeYAMLValue(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(item))
		for i, val := range item {
			out[i] = normalizeYAMLValue(val)
		}
		return out
	default:
		return value
	}
}

// resolveProxyRowCountryCode 旧节点的国家码：优先按名称推断（与订阅同源的口径），退回 IP 健康结果。
func resolveProxyRowCountryCode(row BrowserProxy) string {
	if code := inferCountryCodeFromName(row.ProxyName); code != "" {
		return code
	}
	if trimmed := strings.TrimSpace(row.LastIPHealthJSON); trimmed != "" {
		var health ProxyIPHealthResult
		if err := json.Unmarshal([]byte(trimmed), &health); err == nil {
			if code := resolveProxyLocationCountryCode(health); code != "" {
				return code
			}
		}
	}
	return ""
}

// proxyNameCountryTokens 节点名 → ISO2 国家码。ASCII 词按词边界匹配，中文/旗帜按包含匹配。
var proxyNameCountryTokens = []struct {
	token string
	code  string
}{
	{"united states", "US"}, {"america", "US"}, {"los angeles", "US"}, {"san jose", "US"},
	{"hong kong", "HK"}, {"hongkong", "HK"},
	{"united kingdom", "GB"}, {"great britain", "GB"}, {"london", "GB"},
	{"south korea", "KR"}, {"korea", "KR"},
	{"new york", "US"}, {"seattle", "US"}, {"dallas", "US"}, {"chicago", "US"},
	{"singapore", "SG"}, {"taiwan", "TW"}, {"japan", "JP"}, {"germany", "DE"},
	{"france", "FR"}, {"netherlands", "NL"}, {"canada", "CA"}, {"australia", "AU"},
	{"russia", "RU"}, {"brazil", "BR"}, {"india", "IN"}, {"turkey", "TR"},
	{"türkiye", "TR"}, {"vietnam", "VN"}, {"thailand", "TH"}, {"malaysia", "MY"},
	{"philippines", "PH"}, {"indonesia", "ID"}, {"argentina", "AR"}, {"mexico", "MX"},
	{"italy", "IT"}, {"spain", "ES"}, {"sweden", "SE"}, {"switzerland", "CH"},
	{"poland", "PL"}, {"ukraine", "UA"}, {"egypt", "EG"}, {"macau", "MO"}, {"macao", "MO"},
	{"dubai", "AE"}, {"emirates", "AE"}, {"usa", "US"}, {"britain", "GB"},
	{"us", "US"}, {"uk", "GB"}, {"hk", "HK"}, {"tw", "TW"}, {"jp", "JP"}, {"kr", "KR"},
	{"sg", "SG"}, {"de", "DE"}, {"fr", "FR"}, {"nl", "NL"}, {"ca", "CA"}, {"au", "AU"},
	{"ru", "RU"}, {"br", "BR"}, {"in", "IN"}, {"tr", "TR"}, {"vn", "VN"}, {"th", "TH"},
	{"my", "MY"}, {"ph", "PH"}, {"id", "ID"}, {"ar", "AR"}, {"mx", "MX"}, {"it", "IT"},
	{"es", "ES"}, {"se", "SE"}, {"ch", "CH"}, {"pl", "PL"}, {"ua", "UA"}, {"ae", "AE"},
	{"gb", "GB"}, {"mo", "MO"}, {"eg", "EG"},
	{"美国", "US"}, {"香港", "HK"}, {"台湾", "TW"}, {"日本", "JP"}, {"韩国", "KR"},
	{"新加坡", "SG"}, {"英国", "GB"}, {"德国", "DE"}, {"法国", "FR"}, {"荷兰", "NL"},
	{"加拿大", "CA"}, {"澳大利亚", "AU"}, {"澳洲", "AU"}, {"俄罗斯", "RU"}, {"巴西", "BR"},
	{"印度", "IN"}, {"土耳其", "TR"}, {"越南", "VN"}, {"泰国", "TH"}, {"马来西亚", "MY"},
	{"菲律宾", "PH"}, {"印尼", "ID"}, {"印度尼西亚", "ID"}, {"阿根廷", "AR"}, {"墨西哥", "MX"},
	{"意大利", "IT"}, {"西班牙", "ES"}, {"瑞典", "SE"}, {"瑞士", "CH"}, {"波兰", "PL"},
	{"乌克兰", "UA"}, {"埃及", "EG"}, {"澳门", "MO"}, {"阿联酋", "AE"}, {"迪拜", "AE"},
	{"🇺🇸", "US"}, {"🇭🇰", "HK"}, {"🇹🇼", "TW"}, {"🇯🇵", "JP"}, {"🇰🇷", "KR"},
	{"🇸🇬", "SG"}, {"🇬🇧", "GB"}, {"🇩🇪", "DE"}, {"🇫🇷", "FR"}, {"🇳🇱", "NL"},
	{"🇨🇦", "CA"}, {"🇦🇺", "AU"}, {"🇷🇺", "RU"}, {"🇧🇷", "BR"}, {"🇮🇳", "IN"},
	{"🇹🇷", "TR"}, {"🇻🇳", "VN"}, {"🇹🇭", "TH"}, {"🇲🇾", "MY"}, {"🇵🇭", "PH"},
	{"🇮🇩", "ID"}, {"🇦🇷", "AR"}, {"🇲🇽", "MX"}, {"🇮🇹", "IT"}, {"🇪🇸", "ES"},
	{"🇸🇪", "SE"}, {"🇨🇭", "CH"}, {"🇵🇱", "PL"}, {"🇺🇦", "UA"}, {"🇲🇴", "MO"},
	{"🇦🇪", "AE"}, {"🇪🇬", "EG"},
}

// inferCountryCodeFromName 从节点名推断 ISO2 国家码。
func inferCountryCodeFromName(name string) string {
	lower := strings.ToLower(name)
	if lower == "" {
		return ""
	}
	// 长 token 优先，避免短词误命中。
	tokens := make([]struct {
		token string
		code  string
	}, len(proxyNameCountryTokens))
	copy(tokens, proxyNameCountryTokens)
	sort.SliceStable(tokens, func(i, j int) bool { return len(tokens[i].token) > len(tokens[j].token) })
	for _, entry := range tokens {
		if proxyNameContainsToken(lower, entry.token) {
			return entry.code
		}
	}
	return ""
}

func proxyNameContainsToken(lowerName string, token string) bool {
	if token == "" {
		return false
	}
	if isASCIILowerToken(token) {
		return asciiWordContains(lowerName, token)
	}
	return strings.Contains(lowerName, token)
}

func isASCIILowerToken(token string) bool {
	for _, r := range token {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// asciiWordContains 判断 token 是否作为独立词出现（前后不能是字母/数字），避免 "us" 命中 "russia"。
func asciiWordContains(text string, token string) bool {
	if len(token) == 0 || len(token) > len(text) {
		return false
	}
	isWordByte := func(b byte) bool {
		return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
	}
	for i := 0; i+len(token) <= len(text); i++ {
		if text[i:i+len(token)] != token {
			continue
		}
		beforeOK := i == 0 || !isWordByte(text[i-1])
		after := i + len(token)
		afterOK := after >= len(text) || !isWordByte(text[after])
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}

// proxySourceIgnoredNames 读取订阅源忽略名单（导入预览取消勾选的节点，不再导入）。
// 名单 map 同时被调度器 goroutine 读取、Wails 调用写入，必须加锁防并发读写 panic。
func (a *App) proxySourceIgnoredNames(sourceID string) []string {
	if a == nil || a.config == nil {
		return nil
	}
	a.proxySourceIgnoredMu.Lock()
	names := append([]string{}, a.config.Browser.ProxySourceIgnoredNames[strings.TrimSpace(sourceID)]...)
	a.proxySourceIgnoredMu.Unlock()
	result := make([]string, 0, len(names))
	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// setProxySourceIgnoredNamesLocked 在持锁状态下更新内存中的忽略名单。
func (a *App) setProxySourceIgnoredNamesLocked(sourceID string, names []string) {
	if a.config.Browser.ProxySourceIgnoredNames == nil {
		a.config.Browser.ProxySourceIgnoredNames = make(map[string][]string)
	}
	if len(names) == 0 {
		delete(a.config.Browser.ProxySourceIgnoredNames, sourceID)
	} else {
		a.config.Browser.ProxySourceIgnoredNames[sourceID] = names
	}
}

// refreshedSourcePlan 订阅刷新的合并计划。
type refreshedSourcePlan struct {
	merged []BrowserProxy
	result ProxySourceRefreshResult
}

// buildRefreshedProxyPool 纯函数：按"名称 → 同国家 → 标记异常"三级规则把订阅节点合并进代理池。
//
//   - 名称匹配：沿用旧 proxyId（实例绑定/虚拟分组/测速结果全部无损延续），更新配置；
//   - 同国家匹配：名称对不上时，用同源同国家的新节点顶替旧节点的 proxyId，实例自动切到新节点；
//   - 都匹配不上：旧节点保留在池内（实例绑定不动），仅标记 SourceMissing 异常；
//   - 订阅里的全新节点正常新增。
func buildRefreshedProxyPool(
	pool []BrowserProxy,
	meta proxySourceMeta,
	nodes []clashNode,
	ignoredNames []string,
	generateID func() string,
	now time.Time,
) refreshedSourcePlan {
	refreshedAt := now.Format(time.RFC3339)
	result := ProxySourceRefreshResult{SourceId: meta.SourceID, SourceURL: meta.SourceURL}
	// 分组键必须与 collectProxySourceMetas 一致：sourceId 缺失时退回 source_url，
	// 否则"只有 URL 没有 ID"的订阅行永远匹配不上，每次刷新都会重复新增节点。
	metaKey := proxySourceMetaKey(meta)

	ignored := make(map[string]int, len(ignoredNames))
	for _, name := range ignoredNames {
		key := strings.TrimSpace(name)
		if key != "" {
			ignored[key]++
		}
	}

	type nodePlan struct {
		node      clashNode
		finalName string
		config    string
		oldIndex  int // >=0 表示沿用该旧行（名称或国家匹配）
	}
	plans := make([]nodePlan, 0, len(nodes))
	for _, node := range nodes {
		finalName := node.Name
		if meta.NamePrefix != "" {
			finalName = meta.NamePrefix + "-" + node.Name
		}
		if count := ignored[finalName]; count > 0 {
			ignored[finalName] = count - 1
			continue
		}
		configText, err := clashNodeConfigYAML(node)
		if err != nil {
			continue
		}
		plans = append(plans, nodePlan{node: node, finalName: finalName, config: configText, oldIndex: -1})
	}
	result.Fetched = len(plans)

	// 旧行索引（仅当前订阅源的行）。
	oldIndexes := make([]int, 0, len(pool))
	for index, row := range pool {
		if proxySourceKey(row) == metaKey {
			oldIndexes = append(oldIndexes, index)
		}
	}

	claimed := make(map[int]bool, len(oldIndexes)) // 旧行下标 → 已被名称匹配占用
	nameBuckets := make(map[string][]int)
	for _, oi := range oldIndexes {
		nameBuckets[strings.ToLower(strings.TrimSpace(pool[oi].ProxyName))] = append(nameBuckets[strings.ToLower(strings.TrimSpace(pool[oi].ProxyName))], oi)
	}

	entriesByOld := make(map[int]BrowserProxy, len(oldIndexes))
	fresh := make([]int, 0, len(plans)) // 无主新节点的 plans 下标

	// 第一级：名称匹配（先要求配置也一致，再按出现顺序 FIFO）。
	for pi := range plans {
		plan := &plans[pi]
		candidates := make([]int, 0, 2)
		for _, oi := range nameBuckets[strings.ToLower(plan.finalName)] {
			if !claimed[oi] {
				candidates = append(candidates, oi)
			}
		}
		chosen := -1
		for _, oi := range candidates {
			if clashConfigMatchesNode(pool[oi].ProxyConfig, plan.node) {
				chosen = oi
				break
			}
		}
		if chosen < 0 && len(candidates) > 0 {
			chosen = candidates[0]
		}
		if chosen < 0 {
			fresh = append(fresh, pi)
			continue
		}
		claimed[chosen] = true
		plan.oldIndex = chosen
		entry := pool[chosen]
		if clashConfigType(entry.ProxyConfig) != clashConfigType(plan.config) {
			// 协议变化时不得沿用 preferred_kernel：跨栈指定会被拒绝。
			entry.PreferredKernel = ""
		}
		entry.ProxyName = plan.finalName
		entry.ProxyConfig = plan.config
		entry.SourceLastRefreshAt = refreshedAt
		entry.SourceMissing = false
		entry.SourceMissingSince = ""
		entriesByOld[chosen] = entry
		result.NameMatched++
	}

	// 第二级：同国家匹配（1:1，按列表顺序配对）。
	freshUsed := make(map[int]bool, len(fresh))
	for _, oi := range oldIndexes {
		if claimed[oi] {
			continue
		}
		countryCode := resolveProxyRowCountryCode(pool[oi])
		if countryCode == "" {
			continue
		}
		pick := -1
		for _, pi := range fresh {
			if freshUsed[pi] {
				continue
			}
			if inferCountryCodeFromName(plans[pi].node.Name) == countryCode {
				pick = pi
				break
			}
		}
		if pick < 0 {
			continue
		}
		freshUsed[pick] = true
		plan := &plans[pick]
		plan.oldIndex = oi
		entry := pool[oi]
		// 新节点协议可能与旧节点不同，跨栈内核指定必须清空回 auto。
		entry.PreferredKernel = ""
		entry.ProxyName = plan.finalName
		entry.ProxyConfig = plan.config
		entry.SourceLastRefreshAt = refreshedAt
		entry.SourceMissing = false
		entry.SourceMissingSince = ""
		entriesByOld[oi] = entry
		result.CountryMatched++
	}

	// 第三级：无匹配的旧节点保留并标记异常；仍无主的新节点正常新增。
	merged := make([]BrowserProxy, 0, len(pool)+len(plans))
	appendedFresh := make(map[int]bool, len(fresh))
	for index, row := range pool {
		if entry, ok := entriesByOld[index]; ok {
			merged = append(merged, entry)
			continue
		}
		if claimed[index] {
			continue
		}
		if proxySourceKey(row) != metaKey {
			merged = append(merged, row)
			continue
		}
		// 名称与国家都匹配不上：保留原行，标记异常；不改动 SourceLastRefreshAt。
		if !row.SourceMissing {
			row.SourceMissing = true
			row.SourceMissingSince = refreshedAt
			result.MarkedMissing++
		}
		merged = append(merged, row)
	}
	for _, pi := range fresh {
		if freshUsed[pi] || appendedFresh[pi] {
			continue
		}
		plan := &plans[pi]
		proxyID := strings.TrimSpace(generateID())
		merged = append(merged, BrowserProxy{
			ProxyId:                proxyID,
			ProxyName:              plan.finalName,
			ProxyConfig:            plan.config,
			DnsServers:             meta.DnsServers,
			GroupName:              meta.GroupName,
			SourceID:               meta.SourceID,
			SourceURL:              meta.SourceURL,
			SourceNamePrefix:       meta.NamePrefix,
			SourceAutoRefresh:      meta.AutoRefresh,
			SourceRefreshIntervalM: meta.IntervalM,
			SourceLastRefreshAt:    refreshedAt,
		})
		appendedFresh[pi] = true
		result.Added++
	}

	normalized := proxy.NormalizeBrowserProxies(merged, generateID)
	for _, row := range normalized {
		if proxySourceKey(row) == metaKey {
			result.Total++
		}
	}
	return refreshedSourcePlan{merged: normalized, result: result}
}

// BrowserProxyRefreshSource 手动触发一次订阅源刷新（前端“刷新订阅”按钮与定时调度共用同一套匹配逻辑）。
func (a *App) BrowserProxyRefreshSource(sourceId string, fetchProxyId string) (ProxySourceRefreshResult, error) {
	sourceId = strings.TrimSpace(sourceId)
	if sourceId == "" {
		return ProxySourceRefreshResult{}, fmt.Errorf("订阅源 ID 不能为空")
	}
	pool := a.getLatestProxies()
	var meta *proxySourceMeta
	for _, item := range collectProxySourceMetas(pool) {
		if item.SourceID == sourceId || strings.TrimSpace(item.SourceURL) == sourceId {
			copyMeta := item
			meta = &copyMeta
			break
		}
	}
	if meta == nil {
		return ProxySourceRefreshResult{}, fmt.Errorf("订阅源不存在：%s", sourceId)
	}
	return a.refreshProxySourceWithMeta(pool, *meta, fetchProxyId)
}

func (a *App) refreshProxySourceWithMeta(pool []BrowserProxy, meta proxySourceMeta, fetchProxyID string) (ProxySourceRefreshResult, error) {
	fetched, err := a.browserProxyFetchClashByURL(meta.SourceURL, strings.TrimSpace(fetchProxyID))
	if err != nil {
		return ProxySourceRefreshResult{}, err
	}
	content, _ := fetched["content"].(string)
	payload, ok := parseClashPayload(content)
	if !ok {
		return ProxySourceRefreshResult{}, fmt.Errorf("订阅内容不是有效的 Clash YAML")
	}
	nodes := parseClashNodes(payload)
	if len(nodes) == 0 {
		return ProxySourceRefreshResult{}, fmt.Errorf("订阅内容未解析到可用代理")
	}

	plan := buildRefreshedProxyPool(pool, meta, nodes, a.proxySourceIgnoredNames(meta.SourceID), generateUUID, time.Now())
	if err := a.SaveBrowserProxies(plan.merged); err != nil {
		return plan.result, err
	}
	logger.New("ProxySource").Info("订阅源刷新完成",
		logger.F("source_id", meta.SourceID),
		logger.F("source_url", meta.SourceURL),
		logger.F("fetched", plan.result.Fetched),
		logger.F("name_matched", plan.result.NameMatched),
		logger.F("country_matched", plan.result.CountryMatched),
		logger.F("added", plan.result.Added),
		logger.F("marked_missing", plan.result.MarkedMissing),
		logger.F("total", plan.result.Total),
	)
	a.emitRuntimeEvent("proxy:sources:refreshed", map[string]interface{}{
		"sourceId":       plan.result.SourceId,
		"sourceUrl":      plan.result.SourceURL,
		"fetched":        plan.result.Fetched,
		"nameMatched":    plan.result.NameMatched,
		"countryMatched": plan.result.CountryMatched,
		"added":          plan.result.Added,
		"markedMissing":  plan.result.MarkedMissing,
		"total":          plan.result.Total,
		"at":             time.Now().Format(time.RFC3339),
	})
	return plan.result, nil
}

// BrowserProxySetSourceIgnoredNames 持久化订阅源忽略名单，后端定时刷新不再导入这些节点。
func (a *App) BrowserProxySetSourceIgnoredNames(sourceId string, names []string) error {
	if a == nil || a.config == nil {
		return fmt.Errorf("应用配置未初始化")
	}
	sourceId = strings.TrimSpace(sourceId)
	if sourceId == "" {
		return fmt.Errorf("订阅源 ID 不能为空")
	}
	cleaned := make([]string, 0, len(names))
	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	a.proxySourceIgnoredMu.Lock()
	a.setProxySourceIgnoredNamesLocked(sourceId, cleaned)
	a.proxySourceIgnoredMu.Unlock()
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		return fmt.Errorf("保存订阅源忽略名单失败: %w", err)
	}
	return nil
}

// proxySourceRefreshEnabled 后端订阅刷新调度开关。
func (a *App) proxySourceRefreshEnabled() bool {
	if a == nil || a.config == nil {
		return false
	}
	return a.config.ProxySourceRefresh.IsEnabled()
}
