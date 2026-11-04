package config

import (
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultLaunchServerPort         = 19876
	DefaultLaunchServerAPIKeyHeader = "X-Ant-Api-Key"
	DefaultAutomationInstallPolicy  = "on_demand"
	DefaultAutomationNodeSource     = "auto"
	DefaultAutomationNodeVersion    = "22.15.1"
	DefaultAutomationPWVersion      = "1.59.0"
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

// ProxySourceRefreshConfig 订阅源定时刷新（后端调度器）。
type ProxySourceRefreshConfig struct {
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// IsEnabled 未显式配置时默认开启：URL 导入的订阅按各自间隔定期拉取。
func (c ProxySourceRefreshConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// Config 应用配置
type Config struct {
	Database           DatabaseConfig           `yaml:"database"`
	App                AppConfig                `yaml:"app"`
	Runtime            RuntimeConfig            `yaml:"runtime"`
	Logging            LoggingConfig            `yaml:"logging"`
	Browser            BrowserConfig            `yaml:"browser"`
	ProxyCheck         ProxyCheckConfig         `yaml:"proxy_check"`
	ProxySourceRefresh ProxySourceRefreshConfig `yaml:"proxy_source_refresh,omitempty"`
	LaunchServer       LaunchServerConfig       `yaml:"launch_server"`
	Automation         AutomationConfig         `yaml:"automation"`
	Backup             BackupConfig             `yaml:"backup"`
}

// SourceAlignPair 一次对齐明细：旧节点名 → 新节点名及匹配方式。
type SourceAlignPair struct {
	OldName string `yaml:"old_name" json:"oldName"`
	NewName string `yaml:"new_name" json:"newName"`
	Match   string `yaml:"match" json:"match"` // name / endpoint / country
}

// SourceAlignRecord 订阅源对齐记录（刷新自动对齐或对齐工作台手动对齐）。
type SourceAlignRecord struct {
	At          string            `yaml:"at" json:"at"`
	Mode        string            `yaml:"mode" json:"mode"` // auto / manual
	Pairs       []SourceAlignPair `yaml:"pairs,omitempty" json:"pairs,omitempty"`
	Added       int               `yaml:"added" json:"added"`
	KeptMissing int               `yaml:"kept_missing" json:"keptMissing"`
}

type ProxyCheckConfig struct {
	SpeedIntervalMinutes int                `yaml:"speed_interval_minutes" json:"speedIntervalMinutes"`
	BridgeStartTimeoutMs int                `yaml:"bridge_start_timeout_ms" json:"bridgeStartTimeoutMs"`
	FailureRetryMinutes  int                `yaml:"failure_retry_minutes" json:"failureRetryMinutes"`
	SpeedConcurrency     int                `yaml:"speed_concurrency" json:"speedConcurrency"`
	SpeedTargetID        string             `yaml:"speed_target_id" json:"speedTargetId"`
	IPHealthTargetID     string             `yaml:"ip_health_target_id" json:"ipHealthTargetId"`
	Targets              []ProxyCheckTarget `yaml:"targets" json:"targets"`

	// 虚拟分组健康状态（降级/升级）相关配置。
	VirtualGroupHealthEnabled        *bool `yaml:"virtual_group_health_enabled,omitempty" json:"virtualGroupHealthEnabled,omitempty"`
	VirtualGroupProbeIntervalMinutes int   `yaml:"virtual_group_probe_interval_minutes" json:"virtualGroupProbeIntervalMinutes"`
	DegradeFailureThreshold          int   `yaml:"degrade_failure_threshold" json:"degradeFailureThreshold"`
	RestoreSuccessThreshold          int   `yaml:"restore_success_threshold" json:"restoreSuccessThreshold"`
	RuntimeFailoverEnabled           *bool `yaml:"runtime_failover_enabled,omitempty" json:"runtimeFailoverEnabled,omitempty"`
	RuntimeFailoverCooldownMinutes   int   `yaml:"runtime_failover_cooldown_minutes" json:"runtimeFailoverCooldownMinutes"`
	RuntimeFailoverMaxAttempts       int   `yaml:"runtime_failover_max_attempts" json:"runtimeFailoverMaxAttempts"`
}

// IsVirtualGroupHealthEnabled 未显式配置时默认开启。
func (c ProxyCheckConfig) IsVirtualGroupHealthEnabled() bool {
	return c.VirtualGroupHealthEnabled == nil || *c.VirtualGroupHealthEnabled
}

// IsRuntimeFailoverEnabled 未显式配置时默认开启（仅响应桥接死亡强信号）。
func (c ProxyCheckConfig) IsRuntimeFailoverEnabled() bool {
	return c.RuntimeFailoverEnabled == nil || *c.RuntimeFailoverEnabled
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

type BackupConfig struct {
	LocalDirectory string               `yaml:"local_directory,omitempty"`
	Channels       BackupChannelsConfig `yaml:"channels"`
	Schedule       BackupScheduleConfig `yaml:"schedule"`
}

type BackupChannelsConfig struct {
	OpenList OpenListChannelConfig `yaml:"openlist"`
	S3       S3ChannelConfig       `yaml:"s3,omitempty"`
}

type OpenListChannelConfig struct {
	BaseURL             string `yaml:"base_url,omitempty"`
	RemotePath          string `yaml:"remote_path,omitempty"`
	AuthMode            string `yaml:"auth_mode,omitempty"`
	Username            string `yaml:"username,omitempty"`
	Password            string `yaml:"password,omitempty"`
	Token               string `yaml:"token,omitempty"`
	UploadRateLimitMBps int    `yaml:"upload_rate_limit_mbps,omitempty"`
}

type S3ChannelConfig struct {
	Endpoint        string `yaml:"endpoint,omitempty"`
	Region          string `yaml:"region,omitempty"`
	Bucket          string `yaml:"bucket,omitempty"`
	Prefix          string `yaml:"prefix,omitempty"`
	AccessKeyID     string `yaml:"access_key_id,omitempty"`
	SecretAccessKey string `yaml:"secret_access_key,omitempty"`
	SessionToken    string `yaml:"session_token,omitempty"`
	ForcePathStyle  bool   `yaml:"force_path_style,omitempty"`
}

func (c *BackupConfig) UnmarshalYAML(node *yaml.Node) error {
	var decoded struct {
		LocalDirectory string                `yaml:"local_directory"`
		Channels       BackupChannelsConfig  `yaml:"channels"`
		OpenList       OpenListChannelConfig `yaml:"openlist"`
		Schedule       BackupScheduleConfig  `yaml:"schedule"`
	}
	if err := node.Decode(&decoded); err != nil {
		return err
	}

	channels := decoded.Channels
	openList := channels.OpenList
	if strings.TrimSpace(openList.BaseURL) == "" {
		openList.BaseURL = decoded.OpenList.BaseURL
	}
	if strings.TrimSpace(openList.RemotePath) == "" {
		openList.RemotePath = decoded.OpenList.RemotePath
	}
	if strings.TrimSpace(openList.Token) == "" {
		openList.Token = decoded.OpenList.Token
	}
	if strings.TrimSpace(openList.AuthMode) == "" {
		openList.AuthMode = decoded.OpenList.AuthMode
	}
	if strings.TrimSpace(openList.Username) == "" {
		openList.Username = decoded.OpenList.Username
	}
	if strings.TrimSpace(openList.Password) == "" {
		openList.Password = decoded.OpenList.Password
	}
	if openList.UploadRateLimitMBps == 0 {
		openList.UploadRateLimitMBps = decoded.OpenList.UploadRateLimitMBps
	}
	channels.OpenList = openList
	c.LocalDirectory = strings.TrimSpace(decoded.LocalDirectory)
	c.Channels = channels
	c.Schedule = decoded.Schedule
	return nil
}

type BackupScheduleConfig struct {
	Enabled           bool     `yaml:"enabled"`
	DailyTime         string   `yaml:"daily_time"`
	RecentBackupTimes []string `yaml:"recent_backup_times,omitempty"`
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
	UserDataRoot           string                     `yaml:"user_data_root"`
	DefaultFingerprintArgs []string                   `yaml:"default_fingerprint_args"`
	DefaultLaunchArgs      []string                   `yaml:"default_launch_args"`
	DefaultStartURLs       []string                   `yaml:"default_start_urls"`
	LightStartEnabled      *bool                      `yaml:"light_start_enabled,omitempty"`
	RestoreLastSession     bool                       `yaml:"restore_last_session"`
	StartReadyTimeoutMs    int                        `yaml:"start_ready_timeout_ms,omitempty"`
	StartStableWindowMs    int                        `yaml:"start_stable_window_ms,omitempty"`
	// MaxConcurrentInstances 同时运行实例数上限；0 表示不限制，超出后启动请求排队等待空闲名额。
	MaxConcurrentInstances   int                        `yaml:"max_concurrent_instances,omitempty"`
	DefaultBookmarks       []BrowserBookmark          `yaml:"default_bookmarks,omitempty"`
	DeletedBookmarkURLs    []string                   `yaml:"deleted_bookmark_urls,omitempty"`
	Cores                  []BrowserCore              `yaml:"cores,omitempty"`
	Proxies                []BrowserProxy             `yaml:"proxies,omitempty"`
	ProxyVirtualGroups     []BrowserProxyVirtualGroup `yaml:"proxy_virtual_groups,omitempty"`
	Profiles               []BrowserProfileConfig     `yaml:"profiles,omitempty"`
	// AlignRecords 订阅源对齐记录（sourceId -> 最近 3 条），由刷新自动对齐与对齐工作台写入。
	AlignRecords map[string][]SourceAlignRecord `yaml:"align_records,omitempty" json:"align_records,omitempty"`
	ChromeBinaryPath       string                     `yaml:"chrome_binary_path,omitempty"`
	ClashBinaryPath        string                     `yaml:"clash_binary_path,omitempty"`
	XrayBinaryPath         string                     `yaml:"xray_binary_path,omitempty"`
	SingBoxBinaryPath      string                     `yaml:"singbox_binary_path,omitempty"`
	MieruBinaryPath        string                     `yaml:"mieru_binary_path,omitempty"`
	CoreRoot               string                     `yaml:"core_root,omitempty"`
	DefaultCoreId          string                     `yaml:"default_core_id,omitempty"`
	DefaultConnectorType   string                     `yaml:"default_connector_type,omitempty"`
	Environments           []BrowserEnvironment       `yaml:"environments,omitempty"`
	// ProxySourceIgnoredNames 按 sourceId 记录订阅刷新时要跳过的节点名（导入预览里被用户取消勾选的节点）。
	// 之前只存在前端 localStorage，后端定时刷新无法感知，导致被忽略节点反复“复活”。
	ProxySourceIgnoredNames map[string][]string `yaml:"proxy_source_ignored_names,omitempty" json:"proxySourceIgnoredNames,omitempty"`
}

type BrowserCore struct {
	CoreId    string `yaml:"core_id" json:"coreId"`
	CoreName  string `yaml:"core_name" json:"coreName"`
	CorePath  string `yaml:"core_path" json:"corePath"`
	IsDefault bool   `yaml:"is_default" json:"isDefault"`
	// CoreBackend 标记内核后端类型（fingerprint_chromium / cloak）。
	// 空值按 fingerprint_chromium 兼容历史数据。
	// 注意：这里的 "backend" 指浏览器内核实现，与代理内核（preferred_kernel）无关。
	CoreBackend string `yaml:"core_backend,omitempty" json:"coreBackend,omitempty"`
	// CoreEnv 启动该内核时追加的环境变量，元素格式 KEY=VALUE。
	// Cloak 需要 CLOAKBROWSER_LICENSE_KEY / CLOAKBROWSER_CACHE_DIR 等。
	CoreEnv []string `yaml:"core_env,omitempty" json:"coreEnv,omitempty"`
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
	// SourceFetchProxyID 该订阅源拉取订阅内容时使用的代理（导入时选定并持久化）。
	// 为空表示直连；定时刷新与手动刷新都会复用它，避免被墙订阅导入后无法再刷新。
	SourceFetchProxyID string `yaml:"source_fetch_proxy_id,omitempty" json:"sourceFetchProxyId,omitempty"`
	// SourceLastRefreshError 最近一次刷新失败的错误信息（成功刷新后清空）。
	// 失败必须留下明确记录，不能只靠日志，否则前端无法展示"上次刷新失败"。
	SourceLastRefreshError string `yaml:"source_last_refresh_error,omitempty" json:"sourceLastRefreshError,omitempty"`
	// SourceMissing 订阅刷新后该节点已不在来源列表中（名称/国家都匹配不上）。
	// 节点保留在池内、实例绑定不变，仅标记异常供 UI 与排查使用。
	SourceMissing         bool   `yaml:"source_missing,omitempty" json:"sourceMissing,omitempty"`
	SourceMissingSince    string `yaml:"source_missing_since,omitempty" json:"sourceMissingSince,omitempty"`
	LastLatencyMs         int64  `yaml:"-" json:"lastLatencyMs"`
	LastTestOk            bool   `yaml:"-" json:"lastTestOk"`
	LastTestedAt          string `yaml:"-" json:"lastTestedAt"`
	LastTestEngine        string `yaml:"-" json:"lastTestEngine,omitempty"`
	LastTestConnectorType string `yaml:"-" json:"lastTestConnectorType,omitempty"`
	LastTestFingerprint   string `yaml:"-" json:"-"`
	SpeedTestFingerprint  string `yaml:"-" json:"speedTestFingerprint"`
	LastIPHealthJSON      string `yaml:"-" json:"lastIPHealthJson,omitempty"`
}

type BrowserProxyVirtualGroup struct {
	GroupId   string `yaml:"group_id" json:"groupId"`
	GroupName string `yaml:"group_name" json:"groupName"`
	Strategy  string `yaml:"strategy,omitempty" json:"strategy,omitempty"`
	// SelectedProxyId 当前实际使用的成员代理（降级切换会改写它）。
	SelectedProxyId string `yaml:"selected_proxy_id,omitempty" json:"selectedProxyId,omitempty"`
	// PreferredProxyId 用户偏好节点，降级切换时不改写，节点恢复后用于回切。
	PreferredProxyId string   `yaml:"preferred_proxy_id,omitempty" json:"preferredProxyId,omitempty"`
	MemberProxyIds   []string `yaml:"member_proxy_ids,omitempty" json:"memberProxyIds,omitempty"`
	MemberGroupNames []string `yaml:"member_group_names,omitempty" json:"memberGroupNames,omitempty"`
	SortOrder        int      `yaml:"sort_order,omitempty" json:"sortOrder,omitempty"`
	CreatedAt        string   `yaml:"created_at,omitempty" json:"createdAt,omitempty"`
	UpdatedAt        string   `yaml:"updated_at,omitempty" json:"updatedAt,omitempty"`
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
	ProxyMode          string   `yaml:"proxy_mode,omitempty" json:"proxyMode,omitempty"`
	ProxyId            string   `yaml:"proxy_id" json:"proxyId"`
	ProxyConfig        string   `yaml:"proxy_config" json:"proxyConfig"`
	ProxyGroupName     string   `yaml:"proxy_group_name,omitempty" json:"proxyGroupName,omitempty"`
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
