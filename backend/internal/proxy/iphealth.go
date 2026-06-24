package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
)

const defaultIPPureInfoURL = "https://my.ippure.com/v1/info"

// FetchIPPureInfo 通过指定代理链路查询 IPPure 的出口 IP 健康信息。
// 返回值为第三方接口原始 JSON（map 形式），不做本地评分计算。
func FetchIPPureInfo(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
) (map[string]interface{}, error) {
	src := ""
	for _, item := range proxies {
		if strings.EqualFold(item.ProxyId, proxyId) {
			src = strings.TrimSpace(item.ProxyConfig)
			break
		}
	}
	if src == "" {
		return nil, fmt.Errorf("未找到代理配置")
	}

	return fetchIPPureInfoWithConfig(src, proxyId, proxies, xrayMgr, singboxMgr)
}

// FetchIPPureInfoByConfig 通过一段临时代理配置查询出口 IP 健康信息。
// 它用于尚未保存到代理池、没有 proxyId 的手动代理配置。
func FetchIPPureInfoByConfig(
	proxyConfig string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
) (map[string]interface{}, error) {
	src := strings.TrimSpace(proxyConfig)
	if src == "" {
		return nil, fmt.Errorf("代理配置为空")
	}
	return fetchIPPureInfoWithConfig(src, "", proxies, xrayMgr, singboxMgr)
}

func fetchIPPureInfoWithConfig(
	src string,
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
) (map[string]interface{}, error) {
	client, err := buildIPPureHTTPClient(src, proxyId, proxies, xrayMgr, singboxMgr, 20*time.Second)
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequest(http.MethodGet, defaultIPPureInfoURL, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AntChrome/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用 IPPure 接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 IPPure 响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("IPPure HTTP %d: %s", resp.StatusCode, bodySnippet(body, 180))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("IPPure JSON 解析失败: %w", err)
	}
	return result, nil
}

func buildIPPureHTTPClient(
	src string,
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	timeout time.Duration,
) (*http.Client, error) {
	return buildProxyHTTPClient(src, proxyId, proxies, xrayMgr, singboxMgr, timeout)
}

func bodySnippet(body []byte, max int) string {
	s := strings.TrimSpace(string(body))
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
