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

const DefaultIPHealthURL = "https://my.ippure.com/v1/info"

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
	targetURL := strings.TrimSpace(cfg.URL)
	if targetURL == "" {
		targetURL = DefaultIPHealthURL
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	source := resolveIPHealthSource(cfg, targetURL)
	parser := resolveIPHealthParser(cfg.Parser)
	meta := map[string]interface{}{
		"_source":    source,
		"_targetUrl": targetURL,
		"_parser":    parser,
	}
	if targetURL == "" {
		meta["error"] = "IP 健康检测目标 URL 为空"
		return meta, fmt.Errorf("IP 健康检测目标 URL 为空")
	}

	src := resolveProxyConfig("", proxies, proxyId)
	if src == "" {
		meta["error"] = "未找到代理配置"
		return meta, fmt.Errorf("未找到代理配置")
	}

	client, err := buildIPHealthHTTPClient(src, proxyId, proxies, xrayMgr, singboxMgr, clashMgr, connectorType, timeout)
	if err != nil {
		meta["error"] = err.Error()
		return meta, fmt.Errorf("创建 IP 健康检测客户端失败（source=%s）: %w", source, err)
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

	result, err := parseIPHealthBody(body, cfg.Parser)
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
