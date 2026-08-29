package config

const (
	DefaultLaunchServerPort         = 19876
	DefaultLaunchServerAPIKeyHeader = "X-Ant-Api-Key"
	DefaultAutomationInstallPolicy  = "on_demand"
	DefaultAutomationNodeSource     = "auto"
	DefaultAutomationNodeVersion    = "22.15.1"
	DefaultAutomationPWVersion      = "1.59.0"
	DefaultMCPPath                  = "/mcp"
)

const (
	AutomationNodeSourceAuto    = "auto"
	AutomationNodeSourceSystem  = "system"
	AutomationNodeSourceBundled = "bundled"
)

// LaunchServerConfig Launch HTTP 服务配置
type LaunchServerConfig struct {
	Port int                    `yaml:"port"`
	Auth LaunchServerAuthConfig `yaml:"auth"`
}

type LaunchServerAuthConfig struct {
	Enabled bool   `yaml:"enabled"`
	APIKey  string `yaml:"api_key"`
	Header  string `yaml:"header"`
}

// MCPConfig MCP（Model Context Protocol）服务配置。
//
// 有意不设独立端口和独立 token：MCP 端点挂在 LaunchServer 上，
// 复用其端口、localhost 限制和 API Key 鉴权，减少配置面和认知负担。
type MCPConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Path    string `yaml:"path,omitempty" json:"path"`
	// Stateless 为 true 时不维护会话状态，每个请求独立处理。
	// 适合无状态代理场景；此时 GET / DELETE 会返回 405。
	Stateless bool `yaml:"stateless,omitempty" json:"stateless"`
}

type AutomationConfig struct {
	Enabled               bool   `yaml:"enabled"`
	InstallPolicy         string `yaml:"install_policy,omitempty"`
	RuntimeVersion        string `yaml:"runtime_version,omitempty"`
	HeadlessDefault       bool   `yaml:"headless_default,omitempty"`
	KeepRuntimeOnDisable  bool   `yaml:"keep_runtime_on_disable,omitempty"`
	AllowTypeScriptBuild  bool   `yaml:"allow_typescript_build,omitempty"`
	ArtifactsDir          string `yaml:"artifacts_dir,omitempty"`
	NodeSource            string `yaml:"node_source,omitempty"`
	SystemNodePath        string `yaml:"system_node_path,omitempty"`
	NodeVersion           string `yaml:"node_version,omitempty"`
	PlaywrightCoreVersion string `yaml:"playwright_core_version,omitempty"`
}

// Config 应用配置
type Config struct {
	Database     DatabaseConfig     `yaml:"database"`
	App          AppConfig          `yaml:"app"`
	Runtime      RuntimeConfig      `yaml:"runtime"`
	Logging      LoggingConfig      `yaml:"logging"`
	Browser      BrowserConfig      `yaml:"browser"`
	ProxyCheck   ProxyCheckConfig   `yaml:"proxy_check"`
	LaunchServer LaunchServerConfig `yaml:"launch_server"`
	MCP          MCPConfig          `yaml:"mcp"`
	Automation   AutomationConfig   `yaml:"automation"`
}

type ProxyCheckConfig struct {
	BridgeStartTimeoutMs int                `yaml:"bridge_start_timeout_ms" json:"bridgeStartTimeoutMs"`
	SpeedTargetID        string             `yaml:"speed_target_id" json:"speedTargetId"`
	IPHealthTargetID     string             `yaml:"ip_health_target_id" json:"ipHealthTargetId"`
	Targets              []ProxyCheckTarget `yaml:"targets" json:"targets"`
}

type ProxyCheckTarget struct {
	ID             string `yaml:"id" json:"id"`
	Name           string `yaml:"name" json:"name"`
	Type           string `yaml:"type" json:"type"`
	URL            string `yaml:"url" json:"url"`
	Parser         string `yaml:"parser,omitempty" json:"parser,omitempty"`
	TimeoutMs      int    `yaml:"timeout_ms,omitempty" json:"timeoutMs,omitempty"`
	ExpectedStatus []int  `yaml:"expected_status,omitempty" json:"expectedStatus,omitempty"`
}

type DatabaseConfig struct {
	Type   string       `yaml:"type"`
	SQLite SQLiteConfig `yaml:"sqlite"`
}

type SQLiteConfig struct {
	Path string `yaml:"path"`
}

type AppConfig struct {
	Name   string       `yaml:"name"`
	Window WindowConfig `yaml:"window"`
}

type WindowConfig struct {
	Width     int `yaml:"width"`
	Height    int `yaml:"height"`
	MinWidth  int `yaml:"min_width"`
	MinHeight int `yaml:"min_height"`
}

type RuntimeConfig struct {
	MaxMemoryMB int `yaml:"max_memory_mb"`
	GCPercent   int `yaml:"gc_percent"`
}

type BrowserBookmark struct {
	Name        string `yaml:"name" json:"name"`
	URL         string `yaml:"url" json:"url"`
	OpenOnStart bool   `yaml:"open_on_start,omitempty" json:"openOnStart"`
}

type BrowserConfig struct {
	UserDataRoot           string                 `yaml:"user_data_root"`
	DefaultFingerprintArgs []string               `yaml:"default_fingerprint_args"`
	DefaultLaunchArgs      []string               `yaml:"default_launch_args"`
	DefaultStartURLs       []string               `yaml:"default_start_urls"`
	LightStartEnabled      *bool                  `yaml:"light_start_enabled,omitempty"`
	RestoreLastSession     bool                   `yaml:"restore_last_session"`
	StartReadyTimeoutMs    int                    `yaml:"start_ready_timeout_ms,omitempty"`
	StartStableWindowMs    int                    `yaml:"start_stable_window_ms,omitempty"`
	DefaultBookmarks       []BrowserBookmark      `yaml:"default_bookmarks,omitempty"`
	Cores                  []BrowserCore          `yaml:"cores,omitempty"`
	Proxies                []BrowserProxy         `yaml:"proxies,omitempty"`
	Profiles               []BrowserProfileConfig `yaml:"profiles,omitempty"`
	ChromeBinaryPath       string                 `yaml:"chrome_binary_path,omitempty"`
	ClashBinaryPath        string                 `yaml:"clash_binary_path,omitempty"`
	XrayBinaryPath         string                 `yaml:"xray_binary_path,omitempty"`
	SingBoxBinaryPath      string                 `yaml:"singbox_binary_path,omitempty"`
	CoreRoot               string                 `yaml:"core_root,omitempty"`
	DefaultCoreId          string                 `yaml:"default_core_id,omitempty"`
	DefaultConnectorType   string                 `yaml:"default_connector_type,omitempty"`
	Environments           []BrowserEnvironment   `yaml:"environments,omitempty"`
}

type BrowserCore struct {
	CoreId    string `yaml:"core_id" json:"coreId"`
	CoreName  string `yaml:"core_name" json:"coreName"`
	CorePath  string `yaml:"core_path" json:"corePath"`
	IsDefault bool   `yaml:"is_default" json:"isDefault"`
}

type BrowserProxy struct {
	ProxyId                string `yaml:"proxy_id" json:"proxyId"`
	ProxyName              string `yaml:"proxy_name" json:"proxyName"`
	ProxyConfig            string `yaml:"proxy_config" json:"proxyConfig"`
	PreferredKernel        string `yaml:"preferred_kernel,omitempty" json:"preferredKernel,omitempty"`
	DnsServers             string `yaml:"dns_servers,omitempty" json:"dnsServers,omitempty"`
	GroupName              string `yaml:"group_name,omitempty" json:"groupName,omitempty"`
	SortOrder              int    `yaml:"sort_order,omitempty" json:"sortOrder,omitempty"`
	SourceID               string `yaml:"source_id,omitempty" json:"sourceId,omitempty"`
	SourceURL              string `yaml:"source_url,omitempty" json:"sourceUrl,omitempty"`
	SourceNamePrefix       string `yaml:"source_name_prefix,omitempty" json:"sourceNamePrefix,omitempty"`
	SourceAutoRefresh      bool   `yaml:"source_auto_refresh,omitempty" json:"sourceAutoRefresh,omitempty"`
	SourceRefreshIntervalM int    `yaml:"source_refresh_interval_m,omitempty" json:"sourceRefreshIntervalM,omitempty"`
	SourceLastRefreshAt    string `yaml:"source_last_refresh_at,omitempty" json:"sourceLastRefreshAt,omitempty"`
	LastLatencyMs          int64  `yaml:"-" json:"lastLatencyMs"`
	LastTestOk             bool   `yaml:"-" json:"lastTestOk"`
	LastTestedAt           string `yaml:"-" json:"lastTestedAt"`
	LastIPHealthJSON       string `yaml:"-" json:"lastIPHealthJson,omitempty"`
}

type BrowserEnvironment struct {
	CoreId        string `yaml:"core_id" json:"coreId"`
	CoreName      string `yaml:"core_name" json:"coreName"`
	CorePath      string `yaml:"core_path" json:"corePath"`
	ProxyConfig   string `yaml:"proxy_config" json:"proxyConfig"`
	ConnectorType string `yaml:"connector_type" json:"connectorType"`
	IsDefault     bool   `yaml:"is_default" json:"isDefault"`
}

type BrowserProfileConfig struct {
	ProfileId          string   `yaml:"profile_id" json:"profileId"`
	ProfileName        string   `yaml:"profile_name" json:"profileName"`
	UserDataDir        string   `yaml:"user_data_dir" json:"userDataDir"`
	CoreId             string   `yaml:"core_id" json:"coreId"`
	RestoreLastSession string   `yaml:"restore_last_session,omitempty" json:"restoreLastSession,omitempty"`
	FingerprintArgs    []string `yaml:"fingerprint_args" json:"fingerprintArgs"`
	ProxyId            string   `yaml:"proxy_id" json:"proxyId"`
	ProxyConfig        string   `yaml:"proxy_config" json:"proxyConfig"`
	ProxyBindSourceID  string   `yaml:"proxy_bind_source_id,omitempty" json:"proxyBindSourceId,omitempty"`
	ProxyBindSourceURL string   `yaml:"proxy_bind_source_url,omitempty" json:"proxyBindSourceUrl,omitempty"`
	ProxyBindName      string   `yaml:"proxy_bind_name,omitempty" json:"proxyBindName,omitempty"`
	ProxyBindUpdatedAt string   `yaml:"proxy_bind_updated_at,omitempty" json:"proxyBindUpdatedAt,omitempty"`
	MemoryLimitMB      int      `yaml:"memory_limit_mb,omitempty" json:"memoryLimitMb,omitempty"`
	LaunchArgs         []string `yaml:"launch_args" json:"launchArgs"`
	Tags               []string `yaml:"tags" json:"tags"`
	Keywords           []string `yaml:"keywords,omitempty" json:"keywords,omitempty"`
	CreatedAt          string   `yaml:"created_at" json:"createdAt"`
	UpdatedAt          string   `yaml:"updated_at" json:"updatedAt"`
}

type LoggingConfig struct {
	Level           string            `yaml:"level"`
	FileEnabled     bool              `yaml:"file_enabled"`
	FilePath        string            `yaml:"file_path"`
	Format          string            `yaml:"format"`
	BufferSize      int               `yaml:"buffer_size"`
	AsyncQueueSize  int               `yaml:"async_queue_size"`
	FlushIntervalMs int               `yaml:"flush_interval_ms"`
	Rotation        RotationConfig    `yaml:"rotation"`
	Interceptor     InterceptorConfig `yaml:"interceptor"`
}

type RotationConfig struct {
	Enabled      bool   `yaml:"enabled"`
	MaxSizeMB    int    `yaml:"max_size_mb"`
	MaxAge       int    `yaml:"max_age"`
	MaxBackups   int    `yaml:"max_backups"`
	TimeInterval string `yaml:"time_interval"`
}

type InterceptorConfig struct {
	Enabled         bool     `yaml:"enabled"`
	LogParameters   bool     `yaml:"log_parameters"`
	LogResults      bool     `yaml:"log_results"`
	SensitiveFields []string `yaml:"sensitive_fields"`
}
