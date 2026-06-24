package config

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultMaxProfileLimit          = 100
	StandardCDKeyProfileBonus       = 10
	GithubStarRewardKey             = "GITHUB_STAR_REWARD"
	GithubStarProfileBonus          = 50
	GithubStarProfileTotal          = DefaultMaxProfileLimit + GithubStarProfileBonus
	DefaultLaunchServerPort         = 19876
	DefaultLaunchServerAPIKeyHeader = "X-Ant-Api-Key"
)

// RewardForUsedKey 返回指定兑换记录对应的永久额度奖励。
func RewardForUsedKey(key string) int {
	normalized := strings.ToUpper(strings.TrimSpace(key))
	if normalized == "" {
		return 0
	}
	if normalized == GithubStarRewardKey {
		return GithubStarProfileBonus
	}
	return StandardCDKeyProfileBonus
}

// MinimumProfileLimitForUsedKeys 根据兑换记录计算最低应得实例额度。
func MinimumProfileLimitForUsedKeys(keys []string) int {
	limit := DefaultMaxProfileLimit
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		normalized := strings.ToUpper(strings.TrimSpace(key))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		limit += RewardForUsedKey(normalized)
	}
	return limit
}

// LaunchServerConfig Launch HTTP 服务配置
type LaunchServerConfig struct {
	// Port 为对外暴露的固定入口端口。
	// Launch API 与 CDP 代理共用此端口，便于外部工具固定接入。
	Port int `yaml:"port"`
	// Auth 为 Launch API 的可选本地认证配置。
	Auth LaunchServerAuthConfig `yaml:"auth"`
}

type LaunchServerAuthConfig struct {
	Enabled bool   `yaml:"enabled"`
	APIKey  string `yaml:"api_key"`
	Header  string `yaml:"header"`
}

// Config 应用配置
type Config struct {
	Database     DatabaseConfig     `yaml:"database"`
	App          AppConfig          `yaml:"app"`
	Runtime      RuntimeConfig      `yaml:"runtime"`
	Logging      LoggingConfig      `yaml:"logging"`
	Browser      BrowserConfig      `yaml:"browser"`
	LaunchServer LaunchServerConfig `yaml:"launch_server"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type   string       `yaml:"type"`
	SQLite SQLiteConfig `yaml:"sqlite"`
}

// SQLiteConfig SQLite 配置
type SQLiteConfig struct {
	Path string `yaml:"path"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name            string       `yaml:"name"`
	Window          WindowConfig `yaml:"window"`
	MaxProfileLimit int          `yaml:"max_profile_limit"`
	UsedCDKeys      []string     `yaml:"used_cd_keys"`
}

// WindowConfig 窗口配置
type WindowConfig struct {
	Width     int `yaml:"width"`
	Height    int `yaml:"height"`
	MinWidth  int `yaml:"min_width"`
	MinHeight int `yaml:"min_height"`
}

// RuntimeConfig 运行时配置
type RuntimeConfig struct {
	MaxMemoryMB int `yaml:"max_memory_mb"` // 最大内存软限制（MB），0 表示禁用
	GCPercent   int `yaml:"gc_percent"`    // GC 触发百分比
}

type BrowserBookmark struct {
	Name string `yaml:"name" json:"name"`
	URL  string `yaml:"url" json:"url"`
}

type BrowserConfig struct {
	UserDataRoot           string   `yaml:"user_data_root"`
	DefaultFingerprintArgs []string `yaml:"default_fingerprint_args"`
	DefaultLaunchArgs      []string `yaml:"default_launch_args"`
	// DefaultStartURLs 实例启动且未指定网址时打开的默认起始页。默认空（落到浏览器
	// 新标签页），避免每次启动都自动外联第三方 IP 检测站点而暴露行为特征。
	DefaultStartURLs []string `yaml:"default_start_urls,omitempty"`
	// CDPTransport 选择 CDP 与浏览器的通信方式："port"(默认，调试端口，全平台稳定) /
	// "pipe"(经 --remote-debugging-pipe，不开本地端口；仅 Linux/macOS 支持，实验性)。
	// 留空等同 "port"。Windows 始终回退端口（os/exec 无法映射 fd 3/4）。
	CDPTransport        string                 `yaml:"cdp_transport,omitempty"`
	DefaultProxy        string                 `yaml:"default_proxy"`
	StartReadyTimeoutMs int                    `yaml:"start_ready_timeout_ms,omitempty"`
	StartStableWindowMs int                    `yaml:"start_stable_window_ms,omitempty"`
	StartMaxConcurrent  int                    `yaml:"start_max_concurrent,omitempty"` // 批量启动并发上限（默认 3，钳制 1-8）
	DefaultBookmarks    []BrowserBookmark      `yaml:"default_bookmarks,omitempty"`
	Cores               []BrowserCore          `yaml:"cores,omitempty"`
	Proxies             []BrowserProxy         `yaml:"proxies,omitempty"`
	Profiles            []BrowserProfileConfig `yaml:"profiles,omitempty"`
	// 废弃字段，保留用于迁移
	ChromeBinaryPath     string               `yaml:"chrome_binary_path,omitempty"`
	ClashBinaryPath      string               `yaml:"clash_binary_path,omitempty"`
	XrayBinaryPath       string               `yaml:"xray_binary_path,omitempty"`
	SingBoxBinaryPath    string               `yaml:"singbox_binary_path,omitempty"`
	CoreRoot             string               `yaml:"core_root,omitempty"`
	DefaultCoreId        string               `yaml:"default_core_id,omitempty"`
	DefaultConnectorType string               `yaml:"default_connector_type,omitempty"`
	Environments         []BrowserEnvironment `yaml:"environments,omitempty"`
}

type BrowserCore struct {
	CoreId    string `yaml:"core_id" json:"coreId"`
	CoreName  string `yaml:"core_name" json:"coreName"`
	CorePath  string `yaml:"core_path" json:"corePath"`
	IsDefault bool   `yaml:"is_default" json:"isDefault"`
}

type BrowserProxy struct {
	ProxyId     string `yaml:"proxy_id" json:"proxyId"`
	ProxyName   string `yaml:"proxy_name" json:"proxyName"`
	ProxyConfig string `yaml:"proxy_config" json:"proxyConfig"`
	DnsServers  string `yaml:"dns_servers,omitempty" json:"dnsServers,omitempty"`
	GroupName   string `yaml:"group_name,omitempty" json:"groupName,omitempty"`
	SortOrder   int    `yaml:"sort_order,omitempty" json:"sortOrder,omitempty"`
	SourceID    string `yaml:"source_id,omitempty" json:"sourceId,omitempty"`
	SourceURL   string `yaml:"source_url,omitempty" json:"sourceUrl,omitempty"`
	// URL 导入时的名称前缀，用于后续自动刷新时重建同名策略
	SourceNamePrefix string `yaml:"source_name_prefix,omitempty" json:"sourceNamePrefix,omitempty"`
	// URL 导入自动刷新开关与间隔（分钟）
	SourceAutoRefresh      bool   `yaml:"source_auto_refresh,omitempty" json:"sourceAutoRefresh,omitempty"`
	SourceRefreshIntervalM int    `yaml:"source_refresh_interval_m,omitempty" json:"sourceRefreshIntervalM,omitempty"`
	SourceLastRefreshAt    string `yaml:"source_last_refresh_at,omitempty" json:"sourceLastRefreshAt,omitempty"`
	// 测速结果（运行时字段，不写入 yaml）
	LastLatencyMs int64  `yaml:"-" json:"lastLatencyMs"`
	LastTestOk    bool   `yaml:"-" json:"lastTestOk"`
	LastTestedAt  string `yaml:"-" json:"lastTestedAt"`
	// IP 健康检测原始结果（运行时字段，不写入 yaml）
	LastIPHealthJSON string `yaml:"-" json:"lastIPHealthJson,omitempty"`
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
	FingerprintArgs    []string `yaml:"fingerprint_args" json:"fingerprintArgs"`
	ProxyId            string   `yaml:"proxy_id" json:"proxyId"`
	ProxyConfig        string   `yaml:"proxy_config" json:"proxyConfig"`
	ProxyBindSourceID  string   `yaml:"proxy_bind_source_id,omitempty" json:"proxyBindSourceId,omitempty"`
	ProxyBindSourceURL string   `yaml:"proxy_bind_source_url,omitempty" json:"proxyBindSourceUrl,omitempty"`
	ProxyBindName      string   `yaml:"proxy_bind_name,omitempty" json:"proxyBindName,omitempty"`
	ProxyBindUpdatedAt string   `yaml:"proxy_bind_updated_at,omitempty" json:"proxyBindUpdatedAt,omitempty"`
	LaunchArgs         []string `yaml:"launch_args" json:"launchArgs"`
	Tags               []string `yaml:"tags" json:"tags"`
	Keywords           []string `yaml:"keywords,omitempty" json:"keywords,omitempty"`
	CreatedAt          string   `yaml:"created_at" json:"createdAt"`
	UpdatedAt          string   `yaml:"updated_at" json:"updatedAt"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level       string `yaml:"level"`
	FileEnabled bool   `yaml:"file_enabled"`
	FilePath    string `yaml:"file_path"`
	Format      string `yaml:"format"` // "text" or "json"

	// 性能配置
	BufferSize      int `yaml:"buffer_size"`       // 缓冲区大小（KB）
	AsyncQueueSize  int `yaml:"async_queue_size"`  // 异步队列大小
	FlushIntervalMs int `yaml:"flush_interval_ms"` // 刷新间隔（毫秒）

	// 分片配置
	Rotation RotationConfig `yaml:"rotation"`

	// 方法拦截配置
	Interceptor InterceptorConfig `yaml:"interceptor"`
}

// RotationConfig 日志分片配置
type RotationConfig struct {
	Enabled      bool   `yaml:"enabled"`
	MaxSizeMB    int    `yaml:"max_size_mb"`   // 单文件最大大小（MB）
	MaxAge       int    `yaml:"max_age"`       // 保留天数
	MaxBackups   int    `yaml:"max_backups"`   // 保留文件数
	TimeInterval string `yaml:"time_interval"` // 时间间隔: "daily", "hourly"
}

// InterceptorConfig 方法拦截器配置
type InterceptorConfig struct {
	Enabled         bool     `yaml:"enabled"`
	LogParameters   bool     `yaml:"log_parameters"`   // 是否记录参数
	LogResults      bool     `yaml:"log_results"`      // 是否记录返回值
	SensitiveFields []string `yaml:"sensitive_fields"` // 敏感字段（脱敏）
}

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	normalizeConfig(&config)

	return &config, nil
}

// normalizeConfig 对历史配置进行字段补齐，不覆盖用户已配置值。
func normalizeConfig(config *Config) {
	defaultConfig := DefaultConfig()

	if strings.TrimSpace(config.Database.Type) == "" {
		config.Database.Type = defaultConfig.Database.Type
	}
	if strings.TrimSpace(config.Database.SQLite.Path) == "" {
		config.Database.SQLite.Path = defaultConfig.Database.SQLite.Path
	}

	if strings.TrimSpace(config.App.Name) == "" {
		config.App.Name = defaultConfig.App.Name
	}
	if config.App.Window.Width <= 0 {
		config.App.Window.Width = defaultConfig.App.Window.Width
	}
	if config.App.Window.Height <= 0 {
		config.App.Window.Height = defaultConfig.App.Window.Height
	}
	if config.App.Window.MinWidth <= 0 {
		config.App.Window.MinWidth = defaultConfig.App.Window.MinWidth
	}
	if config.App.Window.MinHeight <= 0 {
		config.App.Window.MinHeight = defaultConfig.App.Window.MinHeight
	}
	if config.App.UsedCDKeys == nil {
		config.App.UsedCDKeys = []string{}
	}

	// 兼容老版本/损坏配置：若 max_profile_limit 缺失或被写成过小值，
	// 通过兑换记录重新计算最低应得额度，避免基础额度或奖励额度丢失。
	expectedLimit := MinimumProfileLimitForUsedKeys(config.App.UsedCDKeys)
	if config.App.MaxProfileLimit < expectedLimit {
		config.App.MaxProfileLimit = expectedLimit
	}

	if config.Runtime.MaxMemoryMB <= 0 {
		config.Runtime.MaxMemoryMB = defaultConfig.Runtime.MaxMemoryMB
	}
	if config.Runtime.GCPercent <= 0 {
		config.Runtime.GCPercent = defaultConfig.Runtime.GCPercent
	}

	if strings.TrimSpace(config.Logging.Level) == "" {
		config.Logging.Level = defaultConfig.Logging.Level
	}
	if isLegacyDefaultLogPath(config.Logging.FilePath) || strings.TrimSpace(config.Logging.FilePath) == "" {
		config.Logging.FilePath = defaultConfig.Logging.FilePath
	}
	if strings.TrimSpace(config.Logging.Format) == "" {
		config.Logging.Format = defaultConfig.Logging.Format
	}
	if config.Logging.BufferSize <= 0 {
		config.Logging.BufferSize = defaultConfig.Logging.BufferSize
	}
	if config.Logging.AsyncQueueSize <= 0 {
		config.Logging.AsyncQueueSize = defaultConfig.Logging.AsyncQueueSize
	}
	if config.Logging.FlushIntervalMs <= 0 {
		config.Logging.FlushIntervalMs = defaultConfig.Logging.FlushIntervalMs
	}
	if config.Logging.Rotation.MaxSizeMB <= 0 {
		config.Logging.Rotation.MaxSizeMB = defaultConfig.Logging.Rotation.MaxSizeMB
	}
	if config.Logging.Rotation.MaxAge <= 0 {
		config.Logging.Rotation.MaxAge = defaultConfig.Logging.Rotation.MaxAge
	}
	if config.Logging.Rotation.MaxBackups <= 0 {
		config.Logging.Rotation.MaxBackups = defaultConfig.Logging.Rotation.MaxBackups
	}
	if strings.TrimSpace(config.Logging.Rotation.TimeInterval) == "" {
		config.Logging.Rotation.TimeInterval = defaultConfig.Logging.Rotation.TimeInterval
	}

	interceptorAllZero := !config.Logging.Interceptor.Enabled &&
		!config.Logging.Interceptor.LogParameters &&
		!config.Logging.Interceptor.LogResults &&
		config.Logging.Interceptor.SensitiveFields == nil
	if interceptorAllZero {
		config.Logging.Interceptor = cloneInterceptorConfig(defaultConfig.Logging.Interceptor)
	} else if config.Logging.Interceptor.SensitiveFields == nil {
		config.Logging.Interceptor.SensitiveFields = append([]string{}, defaultConfig.Logging.Interceptor.SensitiveFields...)
	}

	if strings.TrimSpace(config.Browser.UserDataRoot) == "" {
		config.Browser.UserDataRoot = defaultConfig.Browser.UserDataRoot
	}
	if len(config.Browser.DefaultFingerprintArgs) == 0 {
		config.Browser.DefaultFingerprintArgs = append([]string{}, defaultConfig.Browser.DefaultFingerprintArgs...)
	}
	if len(config.Browser.DefaultLaunchArgs) == 0 {
		config.Browser.DefaultLaunchArgs = append([]string{}, defaultConfig.Browser.DefaultLaunchArgs...)
	}
	if config.Browser.StartReadyTimeoutMs <= 0 {
		config.Browser.StartReadyTimeoutMs = defaultConfig.Browser.StartReadyTimeoutMs
	}
	if config.Browser.StartStableWindowMs <= 0 {
		config.Browser.StartStableWindowMs = defaultConfig.Browser.StartStableWindowMs
	}
	if config.Browser.StartMaxConcurrent <= 0 {
		config.Browser.StartMaxConcurrent = defaultConfig.Browser.StartMaxConcurrent
	}
	if config.Browser.DefaultBookmarks == nil {
		config.Browser.DefaultBookmarks = []BrowserBookmark{}
	}
	if config.Browser.Cores == nil {
		config.Browser.Cores = []BrowserCore{}
	}
	if config.Browser.Proxies == nil {
		config.Browser.Proxies = []BrowserProxy{}
	}
	if config.Browser.Profiles == nil {
		config.Browser.Profiles = []BrowserProfileConfig{}
	}

	if config.LaunchServer.Port <= 0 {
		config.LaunchServer.Port = defaultConfig.LaunchServer.Port
	}
	config.LaunchServer.Auth.APIKey = strings.TrimSpace(config.LaunchServer.Auth.APIKey)
	if strings.TrimSpace(config.LaunchServer.Auth.Header) == "" {
		config.LaunchServer.Auth.Header = defaultConfig.LaunchServer.Auth.Header
	}
}

func cloneInterceptorConfig(src InterceptorConfig) InterceptorConfig {
	dst := src
	dst.SensitiveFields = append([]string{}, src.SensitiveFields...)
	return dst
}

func isLegacyDefaultLogPath(path string) bool {
	return strings.EqualFold(filepath.ToSlash(strings.TrimSpace(path)), "logs/app.log")
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Type: "sqlite",
			SQLite: SQLiteConfig{
				Path: "data/app.db",
			},
		},
		App: AppConfig{
			Name: "Ant Browser",
			Window: WindowConfig{
				Width:     1750,
				Height:    1000,
				MinWidth:  1200,
				MinHeight: 700,
			},
			MaxProfileLimit: DefaultMaxProfileLimit,
			UsedCDKeys:      []string{},
		},
		Runtime: RuntimeConfig{
			MaxMemoryMB: 0,   // 默认禁用软限制，避免把运行中的前后端直接顶死
			GCPercent:   100, // 默认 100%
		},
		Browser: BrowserConfig{
			UserDataRoot:           "data",
			DefaultFingerprintArgs: defaultFingerprintArgsForOS(goruntime.GOOS),
			// 移除已过时的 --disable-blink-features=AutomationControlled
			// 现代 Chrome 已经不支持该参数，会导致警告
			// 添加语言支持以修复中文显示乱码问题
			DefaultLaunchArgs: []string{
				"--disable-sync",
				"--no-first-run",
				"--lang=zh-CN",               // 设置界面语言为简体中文
				"--accept-language=zh-CN,zh", // 设置接受语言，修复网页中文显示
			},
			DefaultProxy:        "",
			StartReadyTimeoutMs: 3000,
			StartStableWindowMs: 1200,
			StartMaxConcurrent:  3,
		},
		Logging: LoggingConfig{
			Level:           "info",
			FileEnabled:     false,
			FilePath:        "data/logs/app.log",
			Format:          "text",
			BufferSize:      4, // 4KB
			AsyncQueueSize:  1000,
			FlushIntervalMs: 1000, // 1秒
			Rotation: RotationConfig{
				Enabled:      false,
				MaxSizeMB:    100,
				MaxAge:       7,
				MaxBackups:   5,
				TimeInterval: "daily",
			},
			Interceptor: InterceptorConfig{
				Enabled:         true,
				LogParameters:   true,
				LogResults:      true,
				SensitiveFields: []string{"password", "token", "secret"},
			},
		},
		LaunchServer: LaunchServerConfig{
			Port: DefaultLaunchServerPort,
			Auth: LaunchServerAuthConfig{
				Enabled: false,
				APIKey:  "",
				Header:  DefaultLaunchServerAPIKeyHeader,
			},
		},
	}
}

func defaultFingerprintArgsForOS(goos string) []string {
	platform := "windows"
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "darwin":
		platform = "mac"
	case "linux":
		platform = "linux"
	}
	// --webrtc-ip-handling-policy=disable_non_proxied_udp：默认阻止 WebRTC 经
	// 非代理通道泄露真实公网/内网 IP（挂代理时尤为关键，直连下亦安全）。
	// 时区 / 语言不在此处硬编码，避免与代理地区不符——启动时按代理出口 IP 推导。
	return []string{
		"--fingerprint-brand=Chrome",
		"--fingerprint-platform=" + platform,
		"--webrtc-ip-handling-policy=disable_non_proxied_udp",
	}
}

// Save 保存配置到文件
func (c *Config) Save(configPath string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// ProxyStore 代理数据文件结构
type ProxyStore struct {
	Proxies []BrowserProxy `yaml:"proxies"`
}

// LoadProxies 从独立文件加载代理列表
func LoadProxies(path string) ([]BrowserProxy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取代理文件失败: %w", err)
	}
	var store ProxyStore
	if err := yaml.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("解析代理文件失败: %w", err)
	}
	return store.Proxies, nil
}

// SaveProxies 将代理列表保存到独立文件
func SaveProxies(path string, proxies []BrowserProxy) error {
	store := ProxyStore{Proxies: proxies}
	data, err := yaml.Marshal(store)
	if err != nil {
		return fmt.Errorf("序列化代理数据失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建代理目录失败: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入代理文件失败: %w", err)
	}
	return nil
}
