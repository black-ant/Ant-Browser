package mcpserver

import (
	"context"

	"ant-chrome/backend/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProxyNode 是代理节点的精简视图。
//
// 有意不返回 proxyConfig：它包含服务器地址、端口和密码等凭据，
// 没有理由交给模型。需要用某个代理时按 proxyId 引用即可。
type ProxyNode struct {
	ProxyID      string `json:"proxyId"`
	ProxyName    string `json:"proxyName"`
	GroupName    string `json:"groupName,omitempty"`
	Protocol     string `json:"protocol,omitempty" jsonschema:"代理协议，从配置中解析得出"`
	LatencyMs    int64  `json:"latencyMs,omitempty" jsonschema:"最近一次测速延迟，0 表示尚未测速"`
	LastTestOk   bool   `json:"lastTestOk" jsonschema:"最近一次测速是否成功"`
	LastTestedAt string `json:"lastTestedAt,omitempty"`
}

// detectProtocol 从代理配置里提取协议名。
// 只取 scheme 部分，不解析凭据，避免把敏感信息带出去。
func detectProtocol(proxyConfig string) string {
	for i := 0; i < len(proxyConfig); i++ {
		if proxyConfig[i] == ':' {
			return proxyConfig[:i]
		}
		// scheme 只允许字母、数字和少量符号；遇到其他字符说明不是 URI 形式。
		c := proxyConfig[i]
		isSchemeChar := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.'
		if !isSchemeChar {
			return ""
		}
	}
	return ""
}

func toProxyNode(p config.BrowserProxy) ProxyNode {
	return ProxyNode{
		ProxyID:      p.ProxyId,
		ProxyName:    p.ProxyName,
		GroupName:    p.GroupName,
		Protocol:     detectProtocol(p.ProxyConfig),
		LatencyMs:    p.LastLatencyMs,
		LastTestOk:   p.LastTestOk,
		LastTestedAt: p.LastTestedAt,
	}
}

// BrowserCoreInfo 是浏览器内核的精简视图。
type BrowserCoreInfo struct {
	CoreID    string `json:"coreId"`
	CoreName  string `json:"coreName"`
	IsDefault bool   `json:"isDefault"`
}

type listProxiesInput struct {
	GroupName     string `json:"groupName,omitempty" jsonschema:"只返回该分组下的代理"`
	AvailableOnly bool   `json:"availableOnly,omitempty" jsonschema:"只返回最近一次测速成功的代理"`
}

type listProxiesOutput struct {
	Count int         `json:"count"`
	Items []ProxyNode `json:"items"`
}

type proxyIDInput struct {
	ProxyID string `json:"proxyId" jsonschema:"代理节点 ID"`
}

type testProxySpeedOutput struct {
	ProxyID   string `json:"proxyId"`
	Ok        bool   `json:"ok"`
	LatencyMs int64  `json:"latencyMs"`
	Engine    string `json:"engine,omitempty" jsonschema:"实际使用的连接栈"`
	Error     string `json:"error,omitempty"`
}

type checkProxyHealthOutput struct {
	ProxyID        string `json:"proxyId"`
	Ok             bool   `json:"ok"`
	IP             string `json:"ip,omitempty" jsonschema:"代理出口 IP"`
	Country        string `json:"country,omitempty"`
	Region         string `json:"region,omitempty"`
	City           string `json:"city,omitempty"`
	AsOrganization string `json:"asOrganization,omitempty"`
	FraudScore     int64  `json:"fraudScore,omitempty" jsonschema:"风险评分，越高越可疑"`
	IsResidential  bool   `json:"isResidential"`
	Source         string `json:"source,omitempty" jsonschema:"检测数据来源"`
	Error          string `json:"error,omitempty"`
}

type listCoresOutput struct {
	Count int               `json:"count"`
	Items []BrowserCoreInfo `json:"items"`
}

func registerProxyTools(srv *mcp.Server, p ProxyProvider) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_proxy_list",
		Description: "列出代理池中的节点。出于安全考虑不返回代理配置本身，绑定代理时按 proxyId 引用。",
		Annotations: readOnly("列出代理节点"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in listProxiesInput) (*mcp.CallToolResult, listProxiesOutput, error) {
		items, err := p.ListProxies()
		if err != nil {
			return toolError(err), listProxiesOutput{}, nil
		}

		out := make([]ProxyNode, 0, len(items))
		for _, item := range items {
			if in.GroupName != "" && item.GroupName != in.GroupName {
				continue
			}
			if in.AvailableOnly && !item.LastTestOk {
				continue
			}
			out = append(out, toProxyNode(item))
		}
		return nil, listProxiesOutput{Count: len(out), Items: out}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_proxy_test_speed",
		Description: "对指定代理测速并保存结果。会按当前连接栈启动桥接进程，属于耗时操作。",
		Annotations: mutating("代理测速"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in proxyIDInput) (*mcp.CallToolResult, testProxySpeedOutput, error) {
		result, err := p.TestProxySpeed(in.ProxyID)
		if err != nil {
			return toolError(err), testProxySpeedOutput{}, nil
		}
		return nil, testProxySpeedOutput{
			ProxyID:   result.ProxyID,
			Ok:        result.Ok,
			LatencyMs: result.LatencyMs,
			Engine:    result.Engine,
			Error:     result.Error,
		}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_proxy_check_health",
		Description: "检测代理出口 IP 的归属地、ASN 和风险评分，用于确认代理是否符合预期。",
		Annotations: mutating("代理 IP 健康检测"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in proxyIDInput) (*mcp.CallToolResult, checkProxyHealthOutput, error) {
		result, err := p.CheckProxyHealth(in.ProxyID)
		if err != nil {
			return toolError(err), checkProxyHealthOutput{}, nil
		}
		return nil, checkProxyHealthOutput{
			ProxyID:        result.ProxyID,
			Ok:             result.Ok,
			IP:             result.IP,
			Country:        result.Country,
			Region:         result.Region,
			City:           result.City,
			AsOrganization: result.AsOrganization,
			FraudScore:     result.FraudScore,
			IsResidential:  result.IsResidential,
			Source:         result.Source,
			Error:          result.Error,
		}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ant_core_list",
		Description: "列出已登记的浏览器内核。创建实例时可用 coreId 指定内核。",
		Annotations: readOnly("列出浏览器内核"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listCoresOutput, error) {
		items, err := p.ListCores()
		if err != nil {
			return toolError(err), listCoresOutput{}, nil
		}

		out := make([]BrowserCoreInfo, 0, len(items))
		for _, item := range items {
			out = append(out, BrowserCoreInfo{
				CoreID:    item.CoreId,
				CoreName:  item.CoreName,
				IsDefault: item.IsDefault,
			})
		}
		return nil, listCoresOutput{Count: len(out), Items: out}, nil
	})
}
