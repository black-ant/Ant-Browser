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

// IPDetectResult 多源出口 IP 检测的归一化结果（透传部分原始字段）。
type IPDetectResult struct {
	Source      string                 `json:"source"`
	Ok          bool                   `json:"ok"`
	Error       string                 `json:"error"`
	IP          string                 `json:"ip"`
	Country     string                 `json:"country"`
	CountryCode string                 `json:"countryCode"`
	Region      string                 `json:"region"`
	City        string                 `json:"city"`
	ISP         string                 `json:"isp"`
	Org         string                 `json:"org"`
	LatencyMs   int64                  `json:"latencyMs"`
	UpdatedAt   string                 `json:"updatedAt"`
	RawData     map[string]interface{} `json:"rawData"`
}

// IPDetectSource 检测源元数据，供前端下拉展示。
type IPDetectSource struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type ipDetectService struct {
	key      string
	label    string
	endpoint string
	parse    func(data map[string]interface{}, result *IPDetectResult)
}

// ipDetectServices 注册的检测源；均为无需密钥的公开 JSON 接口。
// 通过代理链路发起 GET，返回出口 IP 的地理与 ISP 信息。
var ipDetectServices = []ipDetectService{
	{
		key:      "ip-api",
		label:    "IP-API (ip-api.com)",
		endpoint: "http://ip-api.com/json/?fields=status,message,query,country,countryCode,regionName,city,isp,org,as",
		parse: func(data map[string]interface{}, r *IPDetectResult) {
			r.IP = mapString(data, "query")
			r.Country = mapString(data, "country")
			r.CountryCode = mapString(data, "countryCode")
			r.Region = mapString(data, "regionName")
			r.City = mapString(data, "city")
			r.ISP = mapString(data, "isp")
			r.Org = firstNonEmpty(mapString(data, "org"), mapString(data, "as"))
		},
	},
	{
		key:      "ipinfo",
		label:    "IPinfo (ipinfo.io)",
		endpoint: "https://ipinfo.io/json",
		parse: func(data map[string]interface{}, r *IPDetectResult) {
			r.IP = mapString(data, "ip")
			r.Country = mapString(data, "country")
			r.CountryCode = mapString(data, "country")
			r.Region = mapString(data, "region")
			r.City = mapString(data, "city")
			r.ISP = mapString(data, "org")
			r.Org = mapString(data, "org")
		},
	},
	{
		key:      "ipwho",
		label:    "ipwho.is",
		endpoint: "https://ipwho.is/",
		parse: func(data map[string]interface{}, r *IPDetectResult) {
			r.IP = mapString(data, "ip")
			r.Country = mapString(data, "country")
			r.CountryCode = mapString(data, "country_code")
			r.Region = mapString(data, "region")
			r.City = mapString(data, "city")
			if conn, ok := data["connection"].(map[string]interface{}); ok {
				r.ISP = mapString(conn, "isp")
				r.Org = firstNonEmpty(mapString(conn, "org"), mapString(conn, "isp"))
			}
		},
	},
	{
		key:      "ipsb",
		label:    "IP.SB (api.ip.sb)",
		endpoint: "https://api.ip.sb/geoip",
		parse: func(data map[string]interface{}, r *IPDetectResult) {
			r.IP = mapString(data, "ip")
			r.Country = mapString(data, "country")
			r.CountryCode = mapString(data, "country_code")
			r.Region = mapString(data, "region")
			r.City = mapString(data, "city")
			r.ISP = mapString(data, "isp")
			r.Org = firstNonEmpty(mapString(data, "asn_organization"), mapString(data, "isp"))
		},
	},
	{
		key:      "ipapico",
		label:    "ipapi.co",
		endpoint: "https://ipapi.co/json/",
		parse: func(data map[string]interface{}, r *IPDetectResult) {
			r.IP = mapString(data, "ip")
			r.Country = firstNonEmpty(mapString(data, "country_name"), mapString(data, "country"))
			r.CountryCode = mapString(data, "country_code")
			r.Region = mapString(data, "region")
			r.City = mapString(data, "city")
			r.ISP = mapString(data, "org")
			r.Org = mapString(data, "org")
		},
	},
}

// ListIPDetectSources 返回可用的检测源列表（供前端下拉）。
func ListIPDetectSources() []IPDetectSource {
	out := make([]IPDetectSource, 0, len(ipDetectServices))
	for _, svc := range ipDetectServices {
		out = append(out, IPDetectSource{Key: svc.key, Label: svc.label})
	}
	return out
}

func resolveIPDetectService(source string) ipDetectService {
	key := strings.TrimSpace(strings.ToLower(source))
	for _, svc := range ipDetectServices {
		if svc.key == key {
			return svc
		}
	}
	// 默认回退到第一个源（ip-api）
	return ipDetectServices[0]
}

// DetectIPByConfig 通过一段临时代理配置，使用指定检测源查询出口 IP 信息。
// 用于尚未保存到代理池、没有 proxyId 的手动代理配置（如添加代理抽屉）。
func DetectIPByConfig(
	source string,
	proxyConfig string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
) IPDetectResult {
	svc := resolveIPDetectService(source)
	result := IPDetectResult{
		Source:    svc.key,
		RawData:   map[string]interface{}{},
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	src := strings.TrimSpace(proxyConfig)
	if src == "" {
		result.Error = "代理配置为空"
		return result
	}

	client, err := buildProxyHTTPClient(src, "", proxies, xrayMgr, singboxMgr, 15*time.Second)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	runIPDetectRequest(client, svc, &result)
	return result
}

// DetectLocalIP 不经过任何代理，直接查询本机出口公网 IP 信息。
// 用于创建页“本机网络环境”展示真实出口 IP。
func DetectLocalIP(source string) IPDetectResult {
	svc := resolveIPDetectService(source)
	result := IPDetectResult{
		Source:    svc.key,
		RawData:   map[string]interface{}{},
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	client := &http.Client{Timeout: 15 * time.Second}
	runIPDetectRequest(client, svc, &result)
	return result
}

// runIPDetectRequest 用给定 HTTP 客户端请求检测源并把结果写入 result。
func runIPDetectRequest(client *http.Client, svc ipDetectService, result *IPDetectResult) {
	req, _ := http.NewRequest(http.MethodGet, svc.endpoint, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AntChrome/1.0")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("请求检测源失败: %v", err)
		return
	}
	defer resp.Body.Close()
	result.LatencyMs = time.Since(start).Milliseconds()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("读取检测源响应失败: %v", err)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = fmt.Sprintf("检测源 HTTP %d: %s", resp.StatusCode, bodySnippet(body, 180))
		return
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		result.Error = fmt.Sprintf("检测源 JSON 解析失败: %v", err)
		return
	}

	// ip-api 失败时 status=fail
	if status := mapString(data, "status"); strings.EqualFold(status, "fail") {
		result.Error = firstNonEmpty(mapString(data, "message"), "检测源返回失败")
		result.RawData = data
		return
	}

	svc.parse(data, result)
	result.RawData = data
	if strings.TrimSpace(result.IP) == "" {
		result.Error = "未能解析出口 IP"
		return
	}
	result.Ok = true
}

// mapString 从 map 中取字符串字段，缺失或非字符串时返回空串。
func mapString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
