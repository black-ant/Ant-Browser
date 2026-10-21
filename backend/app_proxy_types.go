package backend

import "ant-chrome/backend/internal/proxy"

type ProxyBuildDiagnostic = proxy.ProxyBuildDiagnostic
type ProxyRuntimeDiagnostic = proxy.ProxyRuntimeDiagnostic

// ProxyValidationResult 代理验证结果
type ProxyValidationResult struct {
	Supported bool   `json:"supported"`
	ErrorMsg  string `json:"errorMsg"`
}

// ProxyTestResult 代理测试结果
type ProxyTestResult struct {
	ProxyId       string `json:"proxyId"`
	Ok            bool   `json:"ok"`
	LatencyMs     int64  `json:"latencyMs"`
	Engine        string `json:"engine"`
	Error         string `json:"error"`
	Fingerprint   string `json:"fingerprint"`
	TestedAt      string `json:"testedAt"`
	fingerprint   string
	connectorType string
}

func buildProxyTestResult(result proxy.TestResult) ProxyTestResult {
	return ProxyTestResult{
		ProxyId:   result.ProxyId,
		Ok:        result.Ok,
		LatencyMs: result.LatencyMs,
		Engine:    result.Engine,
		Error:     result.Error,
	}
}

type ProxyBrowserProbeRequest struct {
	ProxyId     string   `json:"proxyId"`
	URLs        []string `json:"urls"`
	Concurrency int      `json:"concurrency"`
	TimeoutMs   int      `json:"timeoutMs"`
}

type ProxyBrowserProbeResult struct {
	ProxyId     string `json:"proxyId"`
	Ok          bool   `json:"ok"`
	TotalMs     int64  `json:"totalMs"`
	AverageMs   int64  `json:"averageMs"`
	P95Ms       int64  `json:"p95Ms"`
	Bytes       int64  `json:"bytes"`
	Completed   int    `json:"completed"`
	Failed      int    `json:"failed"`
	Concurrency int    `json:"concurrency"`
	Error       string `json:"error"`
}

// ProxyBridgeWarmupResult 代理桥接预热结果。
type ProxyBridgeWarmupResult struct {
	ProxyId   string `json:"proxyId"`
	Ok        bool   `json:"ok"`
	Engine    string `json:"engine"`
	SocksURL  string `json:"socksUrl"`
	LatencyMs int64  `json:"latencyMs"`
	Error     string `json:"error"`
}

// ProxyIPHealthResult 代理出口 IP 健康信息（透传第三方接口结果）
type ProxyIPHealthResult struct {
	ProxyId        string                 `json:"proxyId"`
	Ok             bool                   `json:"ok"`
	Source         string                 `json:"source"`
	Error          string                 `json:"error"`
	IP             string                 `json:"ip"`
	FraudScore     int64                  `json:"fraudScore"`
	IsResidential  bool                   `json:"isResidential"`
	IsBroadcast    bool                   `json:"isBroadcast"`
	Country        string                 `json:"country"`
	Region         string                 `json:"region"`
	City           string                 `json:"city"`
	AsOrganization string                 `json:"asOrganization"`
	RawData        map[string]interface{} `json:"rawData"`
	UpdatedAt      string                 `json:"updatedAt"`
}
