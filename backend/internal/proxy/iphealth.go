package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
)

const DefaultIPHealthURL = "http://ip-api.com/json"

// ipHealthDefaultTarget IP 健康检测回退链中的一个目标(URL + 解析器)。
type ipHealthDefaultTarget struct {
	URL    string
	Parser string
}

// defaultIPHealthChain IP 健康检测回退链(用户未配置目标时使用):
// 全部为"返回标准 countryCode/city、且中国出口代理可达"的目标,避免默认走 my.ippure.com
// 这类海外站被中国出口代理墙掉 → 检测失败 → 无法按代理匹配定位。
// 顺序:ip-api(返回 countryCode+city,最标准)→ ipinfo(country 为2位码+city)→ cloudflare trace(loc=CC,parser 已映射 countryCode)。
var defaultIPHealthChain = []ipHealthDefaultTarget{
	{URL: "http://ip-api.com/json", Parser: "json"},
	{URL: "https://ipinfo.io/json", Parser: "json"},
	{URL: "https://www.cloudflare.com/cdn-cgi/trace", Parser: "cloudflare_trace"},
}

type IPHealthConfig struct {
	URL     string
	Source  string
	Parser  string
	Timeout time.Duration
}

// FetchDefaultIPHealthInfo 使用传入的检测目标查询出口 IP 健康信息。
// 返回值为第三方接口原始 JSON（map 形式），不做本地评分计算。
func FetchDefaultIPHealthInfo(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
) (map[string]interface{}, error) {
	return FetchIPHealthInfo(proxyId, proxies, xrayMgr, singboxMgr, nil, config.BrowserConnectorXray, nil)
}

func FetchIPHealthInfo(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	clashMgr *ClashManager,
	connectorType string,
	cfg *IPHealthConfig,
) (map[string]interface{}, error) {
	if cfg == nil {
		cfg = &IPHealthConfig{}
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}

	src := resolveProxyConfig("", proxies, proxyId)
	if src == "" {
		return map[string]interface{}{"_source": "ip_health", "error": "未找到代理配置"}, fmt.Errorf("未找到代理配置")
	}

	client, err := buildIPHealthHTTPClient(src, proxyId, proxies, xrayMgr, singboxMgr, clashMgr, connectorType, timeout)
	if err != nil {
		return map[string]interface{}{"_source": "ip_health", "error": err.Error()},
			fmt.Errorf("创建 IP 健康检测客户端失败: %w", err)
	}

	// 构造本轮要尝试的目标列表:用户显式配置了 URL 就只试这一个;否则走默认回退链。
	type target struct {
		URL    string
		Parser string
	}
	var targets []target
	if u := strings.TrimSpace(cfg.URL); u != "" {
		targets = []target{{URL: u, Parser: cfg.Parser}}
	} else {
		for _, t := range defaultIPHealthChain {
			targets = append(targets, target{URL: t.URL, Parser: t.Parser})
		}
	}

	var lastMeta map[string]interface{}
	var lastErr error
	for _, t := range targets {
		meta, perr := fetchIPHealthTarget(client, t.URL, t.Parser, cfg, timeout)
		if perr == nil && meta != nil {
			// 成功:要求至少解析出 IP,否则视为该目标不可用、继续回退。
			if mapString(meta, "ip") != "" {
				return meta, nil
			}
		}
		lastMeta = meta
		lastErr = perr
	}
	if lastMeta != nil {
		return lastMeta, lastErr
	}
	msg := "所有 IP 健康检测目标均失败"
	if lastErr != nil {
		msg = lastErr.Error()
	}
	return map[string]interface{}{"_source": "ip_health", "error": msg}, fmt.Errorf("%s", msg)
}

// fetchIPHealthTarget 用已建好的 HTTP 客户端对单个目标发一次 IP 健康检测请求并解析。
// 成功返回含 ip/_source/_targetUrl/_parser 的 map;失败返回带 error 字段的 meta + error。
func fetchIPHealthTarget(client *http.Client, targetURL string, parser string, cfg *IPHealthConfig, timeout time.Duration) (map[string]interface{}, error) {
	targetURL = strings.TrimSpace(targetURL)
	source := resolveIPHealthSource(cfg, targetURL)
	parser = resolveIPHealthParser(parser)
	meta := map[string]interface{}{
		"_source":    source,
		"_targetUrl": targetURL,
		"_parser":    parser,
	}
	if targetURL == "" {
		meta["error"] = "IP 健康检测目标 URL 为空"
		return meta, fmt.Errorf("IP 健康检测目标 URL 为空")
	}
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		meta["error"] = err.Error()
		return meta, fmt.Errorf("创建 IP 健康检测请求失败（source=%s）: %w", source, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ZwBrowser/1.0")

	resp, err := client.Do(req)
	if err != nil {
		meta["error"] = err.Error()
		return meta, fmt.Errorf("调用 IP 健康检测接口失败（source=%s）: %w", source, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		meta["error"] = err.Error()
		return meta, fmt.Errorf("读取 IP 健康检测响应失败（source=%s）: %w", source, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := bodySnippet(body, 180)
		meta["error"] = fmt.Sprintf("HTTP %d", resp.StatusCode)
		meta["_statusCode"] = resp.StatusCode
		if snippet != "" {
			meta["_bodySnippet"] = snippet
		}
		return meta, fmt.Errorf("IP 健康检测 HTTP %d（source=%s）: %s", resp.StatusCode, source, snippet)
	}

	result, err := parseIPHealthBody(body, parser)
	if err != nil {
		snippet := bodySnippet(body, 180)
		meta["error"] = err.Error()
		if snippet != "" {
			meta["_bodySnippet"] = snippet
		}
		return meta, fmt.Errorf("IP 健康检测响应解析失败（source=%s, parser=%s）: %w", source, parser, err)
	}
	result["_source"] = source
	result["_targetUrl"] = targetURL
	result["_parser"] = parser
	return result, nil
}

func parseIPHealthBody(body []byte, parser string) (map[string]interface{}, error) {
	if strings.EqualFold(strings.TrimSpace(parser), "cloudflare_trace") {
		result := map[string]interface{}{}
		for _, line := range strings.Split(string(body), "\n") {
			key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
			if ok && strings.TrimSpace(key) != "" {
				result[strings.TrimSpace(key)] = strings.TrimSpace(value)
			}
		}
		if ip := mapString(result, "ip"); ip != "" {
			result["ip"] = ip
		}
		// cloudflare trace 的国家码在 loc=CC 字段;normalize 到标准 countryCode 键,
		// 使下游 resolveProxyLocationCountryCode 能识别 → cloudflare trace 目标也能按代理匹配定位。
		if loc := mapString(result, "loc"); loc != "" {
			result["countryCode"] = loc
		}
		return result, nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func mapString(data map[string]interface{}, key string) string {
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func buildIPHealthHTTPClient(
	src string,
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	clashMgr *ClashManager,
	connectorType string,
	timeout time.Duration,
) (*http.Client, error) {
	return buildProxyHTTPClient(src, proxyId, proxies, xrayMgr, singboxMgr, clashMgr, connectorType, timeout)
}

func resolveIPHealthSource(cfg *IPHealthConfig, targetURL string) string {
	if cfg != nil {
		if source := strings.TrimSpace(cfg.Source); source != "" {
			return source
		}
		if parser := strings.TrimSpace(cfg.Parser); parser != "" {
			return parser
		}
	}
	if DefaultIPHealthURL != "" && strings.EqualFold(strings.TrimSpace(targetURL), DefaultIPHealthURL) {
		return "ip_health"
	}
	if parsed, err := url.Parse(strings.TrimSpace(targetURL)); err == nil {
		if host := strings.ToLower(strings.TrimSpace(parsed.Hostname())); host != "" {
			return host
		}
	}
	return "ip_health"
}

func resolveIPHealthParser(parser string) string {
	normalized := strings.TrimSpace(parser)
	if normalized == "" {
		return "json"
	}
	return normalized
}

func bodySnippet(body []byte, max int) string {
	s := strings.TrimSpace(string(body))
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
