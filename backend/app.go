package backend

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/cdp"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"
	"ant-chrome/backend/internal/usernamescan"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type quitMode uint8

const (
	quitModeFull quitMode = iota
	quitModeAppOnly
)

// App 应用结构体
type App struct {
	ctx                  context.Context
	config               *config.Config
	db                   *database.DB
	interceptor          *logger.MethodInterceptor
	browserMgr           *browser.Manager
	startupQueue         *browser.StartupQueue
	cdpManager           *cdp.Manager
	xrayMgr              *proxy.XrayManager
	clashMgr             *proxy.ClashManager
	singboxMgr           *proxy.SingBoxManager
	launchCodeSvc        *launchcode.LaunchCodeService
	launchServer         *launchcode.LaunchServer
	speedScheduler       *browser.ProxySpeedScheduler
	proxySourceScheduler *browser.ProxySourceScheduler
	usernameScanner      *usernamescan.Service
	appRoot              string
	version              string

	forceQuit            bool       // 强制退出标志，用于跳过 OnBeforeClose 的拦截
	quitMode             quitMode   // 退出模式：全量退出 / 仅退出应用
	maintenanceMu        sync.Mutex // 维护类操作（初始化/导入/导出）互斥锁
	bridgeMu             sync.Mutex
	xrayBridgeRefs       map[string]string
	proxyBatchMu         sync.Mutex         // 保护 proxyBatchCancel / proxyBatchGen
	proxyBatchCancel     context.CancelFunc // 当前批量测速/IP健康检测的取消函数
	proxyBatchGen        int                // 批量操作代数，用于安全清理 cancel
	proxyRefreshInFlight sync.Map           // sourceId -> struct{}，防止同一订阅源并发刷新
	dashboard            *dashboardMonitor  // 仪表盘真实指标采样 + 活动日志
	stopServicesOnce     sync.Once
	finalizeOnce         sync.Once

	// pipeConns 保存 --remote-debugging-pipe 模式下每个实例的共享 CDP 连接
	// （key=profileId）。端口模式下不使用。懒初始化，受 pipeConnsMu 保护。
	pipeConnsMu sync.Mutex
	pipeConns   map[string]*cdp.PipeConn
}

// NewApp 创建新的应用实例
func NewApp(appRoot string, appVersion ...string) *App {
	version := ""
	if len(appVersion) > 0 {
		version = strings.TrimSpace(appVersion[0])
	}
	return &App{
		appRoot:        strings.TrimSpace(appRoot),
		version:        version,
		xrayBridgeRefs: make(map[string]string),
	}
}

func (a *App) appName() string {
	if a.config != nil {
		if name := strings.TrimSpace(a.config.App.Name); name != "" {
			return name
		}
	}
	return "Ant Browser"
}

func (a *App) appVersion() string {
	version := strings.TrimSpace(a.version)
	if version == "" {
		return "unknown"
	}
	return version
}

// startup 应用启动时调用
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := apppath.EnsureWritableLayout(a.appRoot); err != nil {
		runtime.LogFatal(ctx, fmt.Sprintf("初始化 Linux 用户数据目录失败: %v", err))
		return
	}
	cfg, err := LoadConfig(a.resolveAppPath("config.yaml"))
	if err != nil {
		cfg = config.DefaultConfig()
	}
	a.config = cfg
	a.applyRuntimeConfig(cfg.Runtime)

	logConfig := logger.LoggerConfig{
		Level:           cfg.Logging.Level,
		FileEnabled:     cfg.Logging.FileEnabled,
		FilePath:        a.resolveAppPath(cfg.Logging.FilePath),
		Format:          cfg.Logging.Format,
		BufferSize:      cfg.Logging.BufferSize,
		AsyncQueueSize:  cfg.Logging.AsyncQueueSize,
		FlushIntervalMs: cfg.Logging.FlushIntervalMs,
		Rotation: logger.RotationConfig{
			Enabled:      cfg.Logging.Rotation.Enabled,
			MaxSizeMB:    cfg.Logging.Rotation.MaxSizeMB,
			MaxAge:       cfg.Logging.Rotation.MaxAge,
			MaxBackups:   cfg.Logging.Rotation.MaxBackups,
			TimeInterval: cfg.Logging.Rotation.TimeInterval,
		},
	}
	logger.InitWithConfig(ctx, logConfig)

	log := logger.New("App")
	log.Info("应用启动中...",
		logger.F("version", a.appVersion()),
		logger.F("max_memory_mb", cfg.Runtime.MaxMemoryMB),
		logger.F("gc_percent", cfg.Runtime.GCPercent),
	)
	if apppath.IsDetached(a.appRoot) {
		log.Info("检测到安装目录需要只读运行，已切换到用户数据目录",
			logger.F("install_root", apppath.InstallRoot(a.appRoot)),
			logger.F("state_root", apppath.StateRoot(a.appRoot)),
		)
	}

	// 确保 data 目录存在（存放数据库、用户数据、快照等）
	if err := os.MkdirAll(a.resolveAppPath("data"), 0755); err != nil {
		log.Error("创建 data 目录失败", logger.F("error", err))
	}

	a.ensureDefaultCores()

	if cfg.Logging.Interceptor.Enabled {
		interceptorConfig := logger.InterceptorConfig{
			Enabled:         cfg.Logging.Interceptor.Enabled,
			LogParameters:   cfg.Logging.Interceptor.LogParameters,
			LogResults:      cfg.Logging.Interceptor.LogResults,
			SensitiveFields: cfg.Logging.Interceptor.SensitiveFields,
		}
		a.interceptor = logger.NewMethodInterceptor(log, interceptorConfig)
	}

	db, err := database.NewDB(a.resolveAppPath(cfg.Database.SQLite.Path))
	if err != nil {
		log.Error("初始化数据库失败", logger.F("error", err))
		runtime.LogFatal(ctx, fmt.Sprintf("初始化数据库失败: %v", err))
		return
	}
	a.db = db
	if err := db.Migrate(); err != nil {
		log.Error("数据库迁移失败", logger.F("error", err))
	}

	// 初始化本机加密密钥并迁移旧密文（账号密码/Cookie 等敏感字段）
	// 强制密钥加载成功，失败则终止启动（保护敏感数据安全）
	if err := a.initAccountEncryptionKey(); err != nil {
		log.Error("加密密钥初始化失败，启动终止", logger.F("error", err))
		panic(fmt.Sprintf("Failed to initialize encryption key: %v", err))
	}

	a.browserMgr = browser.NewManager(cfg, a.appRoot)
	a.startupQueue = browser.NewStartupQueue(browserStartMaxConcurrent(cfg))
	a.cdpManager = cdp.NewManager()
	a.xrayMgr = proxy.NewXrayManager(cfg, a.appRoot)
	a.clashMgr = proxy.NewClashManager(cfg, a.appRoot)
	a.singboxMgr = proxy.NewSingBoxManager(cfg, a.appRoot)

	// 注入 DAO（必须在 InitData 之前）
	conn := db.GetConn()
	a.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(conn)
	a.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(conn)
	a.browserMgr.ProxySourceDAO = browser.NewSQLiteProxySourceDAO(conn)
	a.browserMgr.CoreDAO = browser.NewSQLiteCoreDAO(conn)
	a.browserMgr.BookmarkDAO = browser.NewSQLiteBookmarkDAO(conn)
	a.browserMgr.GroupDAO = browser.NewSQLiteGroupDAO(conn)

	// 一次性迁移：若 SQLite 表为空则从旧文件导入
	a.migrateToSQLite()

	a.browserMgr.InitData()
	a.autoDetectCores()
	a.loadProxies()
	a.reconcileProfileProxyBindings()

	// 初始化 LaunchCode 服务
	launchCodeDAO := launchcode.NewSQLiteLaunchCodeDAO(a.db.GetConn())
	a.launchCodeSvc = launchcode.NewLaunchCodeService(launchCodeDAO)
	if err := a.launchCodeSvc.LoadAll(); err != nil {
		log.Error("LaunchCode 加载失败", logger.F("error", err))
	}
	a.browserMgr.CodeProvider = a.launchCodeSvc

	// 启动 LaunchServer
	port := a.config.LaunchServer.Port
	a.launchServer = launchcode.NewLaunchServer(a.launchCodeSvc, a, a, a.browserMgr, port)
	a.launchServer.SetAPIAuthConfig(launchcode.APIAuthConfig{
		Enabled: a.config.LaunchServer.Auth.Enabled,
		APIKey:  a.config.LaunchServer.Auth.APIKey,
		Header:  a.config.LaunchServer.Auth.Header,
	})
	if err := a.launchServer.Start(); err != nil {
		log.Error("LaunchServer 启动失败", logger.F("error", err))
	} else {
		log.Info("LaunchServer 监听地址",
			logger.F("url", fmt.Sprintf("http://127.0.0.1:%d", a.launchServer.Port())),
			logger.F("preferred_port", port),
		)
	}

	// 连接池失效通知
	a.xrayMgr.OnBridgeDied = func(key string, err error) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "proxy:bridge:died", map[string]interface{}{
				"engine": "xray",
				"key":    key[:8],
				"error":  err.Error(),
			})
		}
	}
	a.singboxMgr.OnBridgeDied = func(key string, err error) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "proxy:bridge:died", map[string]interface{}{
				"engine": "singbox",
				"key":    key[:8],
				"error":  err.Error(),
			})
		}
	}

	// 启动代理测速定时调度器（每5分钟一轮，并发5）
	a.speedScheduler = browser.NewProxySpeedScheduler(
		a.browserMgr.ProxyDAO,
		func(proxyId string) (bool, int64, string) {
			r := proxy.SpeedTest(proxyId, a.config.Browser.Proxies, a.xrayMgr, a.singboxMgr, nil)
			return r.Ok, r.LatencyMs, r.Error
		},
		5*time.Minute,
		5,
	)
	a.speedScheduler.Start()

	// 从现有代理聚合订阅源（仅首次），并启动订阅源自动刷新调度器
	a.seedProxySourcesFromProxies()
	a.proxySourceScheduler = browser.NewProxySourceScheduler(
		a.browserMgr.ProxySourceDAO,
		a.refreshProxySource,
		60*time.Second,
	)
	a.proxySourceScheduler.Start()

	// 启动仪表盘真实资源采样
	a.startDashboardMonitor()

	log.Info("应用启动成功")
}

// ReloadConfig 开放给前端重新读取配置，用于应对手动修补后的配置重载
func (a *App) ReloadConfig() error {
	log := logger.New("App")
	cfg, err := LoadConfig(a.resolveAppPath("config.yaml"))
	if err != nil {
		log.Error("重载配置文件失败", logger.F("error", err))
		return fmt.Errorf("重载配置文件失败: %w", err)
	}

	a.config = cfg
	a.applyRuntimeConfig(cfg.Runtime)
	// Update browser manager config reference
	if a.browserMgr != nil {
		a.browserMgr.Config = cfg
		a.browserMgr.ListCores()
		a.loadProxies()
		a.reconcileProfileProxyBindings()
	}
	if a.xrayMgr != nil {
		a.xrayMgr.Config = cfg
	}
	if a.clashMgr != nil {
		a.clashMgr.Config = cfg
	}
	if a.singboxMgr != nil {
		a.singboxMgr.Config = cfg
	}
	if a.launchServer != nil {
		a.launchServer.SetAPIAuthConfig(launchcode.APIAuthConfig{
			Enabled: cfg.LaunchServer.Auth.Enabled,
			APIKey:  cfg.LaunchServer.Auth.APIKey,
			Header:  cfg.LaunchServer.Auth.Header,
		})
	}

	log.Info("前端触发配置重载成功")
	return nil
}

func (a *App) applyRuntimeConfig(cfg config.RuntimeConfig) {
	if cfg.GCPercent > 0 {
		debug.SetGCPercent(cfg.GCPercent)
	}
	if cfg.MaxMemoryMB > 0 {
		maxMemoryBytes := int64(cfg.MaxMemoryMB) * 1024 * 1024
		debug.SetMemoryLimit(maxMemoryBytes)
		return
	}
	// 0 表示禁用自定义软限制，避免 ReloadConfig 后残留旧的 GOMEMLIMIT。
	debug.SetMemoryLimit(1 << 60)
}

func (a *App) shutdown(ctx context.Context) {
	log := logger.New("App")
	if a.shouldStopRuntimeServicesOnShutdown() {
		log.Info("应用正在关闭...")
		a.stopRuntimeServices()
	} else {
		log.Info("应用正在关闭（保留当前已打开的浏览器实例）...")
	}
	a.finalizeShutdown()
}

func (a *App) GetInterceptor() *logger.MethodInterceptor {
	return a.interceptor
}

// ForceQuit 设置强制退出标志并调用 runtime.Quit
func (a *App) ForceQuit() {
	a.setQuitMode(quitModeFull)
	a.stopRuntimeServices()
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// QuitAppOnly 仅退出应用本身，保留当前已打开的浏览器实例。
func (a *App) QuitAppOnly() {
	a.setQuitMode(quitModeAppOnly)
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

func Start(a *App, ctx context.Context) {
	a.startup(ctx)
}

func Stop(a *App, ctx context.Context) {
	a.shutdown(ctx)
}

func platformSupportsTrayCloseFlow() bool {
	return platformSupportsTrayCloseFlowForOS(goruntime.GOOS)
}

func platformSupportsTrayCloseFlowForOS(goos string) bool {
	return strings.EqualFold(strings.TrimSpace(goos), "windows")
}

func (a *App) setQuitMode(mode quitMode) {
	a.forceQuit = true
	a.quitMode = mode
}

func (a *App) shouldStopRuntimeServicesOnShutdown() bool {
	return a.quitMode != quitModeAppOnly
}

func ShouldBlockClose(a *App, ctx context.Context) bool {
	if a.forceQuit {
		return false
	}
	if !platformSupportsTrayCloseFlow() {
		return false
	}
	runtime.EventsEmit(ctx, "app:request-close")
	return true
}

func (a *App) bindProfileXrayBridge(profileId string, bridgeKey string) {
	profileId = strings.TrimSpace(profileId)
	bridgeKey = strings.TrimSpace(bridgeKey)
	if profileId == "" || bridgeKey == "" {
		return
	}

	a.bridgeMu.Lock()
	a.xrayBridgeRefs[profileId] = bridgeKey
	a.bridgeMu.Unlock()
}

func (a *App) releaseProfileXrayBridge(profileId string) {
	profileId = strings.TrimSpace(profileId)
	if profileId == "" {
		return
	}

	a.bridgeMu.Lock()
	bridgeKey := a.xrayBridgeRefs[profileId]
	delete(a.xrayBridgeRefs, profileId)
	a.bridgeMu.Unlock()

	if bridgeKey != "" && a.xrayMgr != nil {
		a.xrayMgr.ReleaseBridge(bridgeKey)
	}
}

func (a *App) clearProfileXrayBridges() {
	a.bridgeMu.Lock()
	a.xrayBridgeRefs = make(map[string]string)
	a.bridgeMu.Unlock()
}

// ============================================================================
// 仪表盘 API
// ============================================================================

func (a *App) GetDashboardStats() map[string]interface{} {
	profiles := a.browserMgr.List()
	totalInstances := len(profiles)
	runningInstances := 0
	for _, p := range profiles {
		if p.Running {
			runningInstances++
		}
	}
	proxyCount := len(a.config.Browser.Proxies)
	coreCount := len(a.config.Browser.Cores)

	// 可用代理数：最近一次测速成功的代理（真实来源 last_test_ok）
	proxyAvailable := 0
	if a.browserMgr.ProxyDAO != nil {
		if list, err := a.browserMgr.ProxyDAO.List(); err == nil {
			proxyCount = len(list)
			for _, p := range list {
				if p.LastTestOk {
					proxyAvailable++
				}
			}
		}
	}

	var mem goruntime.MemStats
	goruntime.ReadMemStats(&mem)
	memUsedMB := float64(mem.Alloc) / 1024 / 1024

	return map[string]interface{}{
		"totalInstances":   totalInstances,
		"runningInstances": runningInstances,
		"proxyCount":       proxyCount,
		"proxyAvailable":   proxyAvailable,
		"coreCount":        coreCount,
		"memUsedMB":        int(memUsedMB),
		"appVersion":       a.appVersion(),
	}
}

func (a *App) GetAppConfig() map[string]interface{} {
	return map[string]interface{}{
		"name":    a.appName(),
		"version": a.appVersion(),
	}
}

func (a *App) GetMemoryStats() map[string]interface{} {
	var m goruntime.MemStats
	goruntime.ReadMemStats(&m)
	return map[string]interface{}{
		"alloc_mb":       float64(m.Alloc) / 1024 / 1024,
		"total_alloc_mb": float64(m.TotalAlloc) / 1024 / 1024,
		"sys_mb":         float64(m.Sys) / 1024 / 1024,
		"num_gc":         m.NumGC,
		"limit_mb":       a.config.Runtime.MaxMemoryMB,
		"gc_percent":     a.config.Runtime.GCPercent,
	}
}

func (a *App) TriggerGC()               { goruntime.GC() }
func (a *App) SetLogLevel(level string) { logger.SetGlobalLevelString(level) }
func (a *App) GetLogLevel() string      { return logger.New("App").GetLevel().String() }

// GetAppLogs 获取内存缓冲日志
func (a *App) GetAppLogs() []logger.MemoryLogEntry {
	return logger.GetMemoryWriter().GetEntries()
}

// ClearAppLogs 清空内存缓冲日志
func (a *App) ClearAppLogs() {
	logger.GetMemoryWriter().Clear()
}

// GetRunningInstances 获取运行中实例的详细信息
func (a *App) GetRunningInstances() []BrowserProfile {
	all := a.browserMgr.List()
	result := make([]BrowserProfile, 0)
	for _, p := range all {
		if p.Running {
			result = append(result, p)
		}
	}
	return result
}

// ============================================================================
// 浏览器类型别名 (保持 Wails 绑定兼容)
// ============================================================================

type BrowserProfile = browser.Profile
type BrowserProfileInput = browser.ProfileInput
type BrowserTab = browser.Tab
type BrowserSettings = browser.Settings
type BrowserProxy = browser.Proxy
type BrowserCore = browser.Core
type BrowserCoreInput = browser.CoreInput
type BrowserCoreValidateResult = browser.CoreValidateResult
type BrowserCoreExtendedInfo = browser.CoreExtendedInfo

// ============================================================================
// 浏览器配置 API
// ============================================================================

// BrowserProfileList 获取所有实例列表
func (a *App) BrowserProfileList() []BrowserProfile { return a.browserMgr.List() }

// BrowserProfileListByTag 按标签筛选实例列表
func (a *App) BrowserProfileListByTag(tag string) []BrowserProfile {
	return a.browserMgr.ListByTag(tag)
}

// BrowserGetAllTags 获取所有已使用的标签
func (a *App) BrowserGetAllTags() []string {
	return a.browserMgr.GetAllTags()
}

// BrowserProfileSetKeywords 设置实例关键字
func (a *App) BrowserProfileSetKeywords(profileId string, keywords []string) (*BrowserProfile, error) {
	return a.browserMgr.SetKeywords(profileId, keywords)
}

func (a *App) BrowserProfileCreate(input BrowserProfileInput) (*BrowserProfile, error) {
	return a.createBrowserProfileFromInput(input)
}

func (a *App) BrowserProfileUpdate(profileId string, input BrowserProfileInput) (*BrowserProfile, error) {
	return a.updateBrowserProfileFromInput(profileId, input)
}

func (a *App) BrowserProfileDelete(profileId string) error { return a.browserMgr.Delete(profileId) }

// BrowserProfileCopy 复制实例配置（除指纹参数外全部复制）
func (a *App) BrowserProfileCopy(profileId string, newName string) (*BrowserProfile, error) {
	return a.browserMgr.Copy(profileId, newName)
}

// ============================================================================
// 浏览器设置 API
// ============================================================================

func (a *App) GetBrowserSettings() BrowserSettings {
	return BrowserSettings{
		UserDataRoot:           a.config.Browser.UserDataRoot,
		DefaultFingerprintArgs: append([]string{}, a.config.Browser.DefaultFingerprintArgs...),
		DefaultLaunchArgs:      append([]string{}, a.config.Browser.DefaultLaunchArgs...),
		DefaultProxy:           a.config.Browser.DefaultProxy,
		StartReadyTimeoutMs:    browserStartReadyTimeoutMillis(a.config),
		StartStableWindowMs:    browserStartStableWindowMillis(a.config),
	}
}

func (a *App) SaveBrowserSettings(settings BrowserSettings) error {
	log := logger.New("Browser")
	a.config.Browser.UserDataRoot = strings.TrimSpace(settings.UserDataRoot)
	a.config.Browser.DefaultFingerprintArgs = append([]string{}, settings.DefaultFingerprintArgs...)
	a.config.Browser.DefaultLaunchArgs = append([]string{}, settings.DefaultLaunchArgs...)
	a.config.Browser.DefaultProxy = strings.TrimSpace(settings.DefaultProxy)
	if settings.StartReadyTimeoutMs > 0 {
		a.config.Browser.StartReadyTimeoutMs = settings.StartReadyTimeoutMs
	} else if a.config.Browser.StartReadyTimeoutMs <= 0 {
		a.config.Browser.StartReadyTimeoutMs = browserStartReadyTimeoutMillis(nil)
	}
	if settings.StartStableWindowMs > 0 {
		a.config.Browser.StartStableWindowMs = settings.StartStableWindowMs
	} else if a.config.Browser.StartStableWindowMs <= 0 {
		a.config.Browser.StartStableWindowMs = browserStartStableWindowMillis(nil)
	}
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		log.Error("浏览器配置保存失败", logger.F("error", err))
		return err
	}
	return nil
}

// ============================================================================
// 内核管理 API
// ============================================================================

func (a *App) BrowserCoreList() []BrowserCore {
	return a.browserMgr.ListCores()
}

func (a *App) BrowserCoreSave(input BrowserCoreInput) error {
	return a.browserMgr.SaveCore(input)
}

func (a *App) BrowserCoreDelete(coreId string) error {
	return a.browserMgr.DeleteCore(coreId)
}

func (a *App) BrowserCoreSetDefault(coreId string) error {
	return a.browserMgr.SetDefaultCore(coreId)
}

func (a *App) BrowserCoreValidate(corePath string) BrowserCoreValidateResult {
	return a.browserMgr.ValidateCorePath(corePath)
}

func (a *App) BrowserCoreExtendedInfo() []BrowserCoreExtendedInfo {
	return a.browserMgr.GetCoresExtendedInfo()
}

// BrowserCoreScan 重新扫描 chrome 目录，自动注册新内核
func (a *App) BrowserCoreScan() []BrowserCore {
	a.autoDetectCores()
	return a.browserMgr.ListCores()
}

// BrowserCoreDownload 在线下载并自动解压配置内核
func (a *App) BrowserCoreDownload(coreName, url, proxyConfig string) error {
	if a.ctx == nil {
		return fmt.Errorf("app context is nil")
	}
	// 异步启动下载流程，以防阻塞前端请求，通过 Wails events 发送进度
	go a.browserMgr.DownloadAndExtractCore(a.ctx, coreName, url, proxyConfig)
	return nil
}

// ============================================================================
// 代理池 API
// ============================================================================

// ProxyValidationResult 代理验证结果
type ProxyValidationResult struct {
	Supported bool   `json:"supported"`
	ErrorMsg  string `json:"errorMsg"`
}

func (a *App) BrowserProxyList() []BrowserProxy {
	if a.browserMgr.ProxyDAO != nil {
		if list, err := a.browserMgr.ProxyDAO.List(); err == nil {
			return list
		}
	}
	return append([]BrowserProxy{}, a.config.Browser.Proxies...)
}

// BrowserProxyListGroups 获取所有代理分组名称
func (a *App) BrowserProxyListGroups() []string {
	if a.browserMgr.ProxyDAO != nil {
		if groups, err := a.browserMgr.ProxyDAO.ListGroups(); err == nil {
			return groups
		}
	}
	return nil
}

// BrowserProxyListByGroup 按分组名称查询代理
func (a *App) BrowserProxyListByGroup(groupName string) []BrowserProxy {
	if a.browserMgr.ProxyDAO != nil {
		if list, err := a.browserMgr.ProxyDAO.ListByGroup(groupName); err == nil {
			return list
		}
	}
	// 降级：内存过滤
	var result []BrowserProxy
	for _, p := range a.config.Browser.Proxies {
		if p.GroupName == groupName {
			result = append(result, p)
		}
	}
	return result
}

// ValidateProxyConfig 验证代理配置是否支持
func (a *App) ValidateProxyConfig(proxyConfig string, proxyId string) ProxyValidationResult {
	proxies := a.getLatestProxies()
	supported, errorMsg := proxy.ValidateProxyConfig(proxyConfig, proxies, proxyId)
	return ProxyValidationResult{
		Supported: supported,
		ErrorMsg:  errorMsg,
	}
}

// ProxyTestResult 代理测试结果
type ProxyTestResult struct {
	ProxyId   string `json:"proxyId"`
	Ok        bool   `json:"ok"`
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

// TestProxyConnectivity 测试代理连通性
func (a *App) TestProxyConnectivity(proxyId string, proxyConfig string) ProxyTestResult {
	proxies := a.getLatestProxies()
	r := proxy.TestConnectivity(proxyId, proxyConfig, proxies, nil)
	return ProxyTestResult{ProxyId: r.ProxyId, Ok: r.Ok, LatencyMs: r.LatencyMs, Error: r.Error}
}

// TestProxyRealConnectivity 通过真实 HTTP 请求测试代理连通性（Wails 绑定）
// 参考 Clash URLTest 策略：多 URL fallback + 复用桥接 + TCP ping 降级
func (a *App) TestProxyRealConnectivity(proxyId string) ProxyTestResult {
	proxies := a.getLatestProxies()
	r := proxy.SpeedTest(proxyId, proxies, a.xrayMgr, a.singboxMgr, nil)
	return ProxyTestResult{ProxyId: r.ProxyId, Ok: r.Ok, LatencyMs: r.LatencyMs, Error: r.Error}
}

// BrowserProxyTestSpeed 手动触发单个代理测速并持久化结果
func (a *App) BrowserProxyTestSpeed(proxyId string) ProxyTestResult {
	proxies := a.getLatestProxies()
	r := proxy.SpeedTest(proxyId, proxies, a.xrayMgr, a.singboxMgr, nil)
	if a.browserMgr.ProxyDAO != nil {
		testedAt := time.Now().Format(time.RFC3339)
		_ = a.browserMgr.ProxyDAO.UpdateSpeedResult(proxyId, r.Ok, r.LatencyMs, testedAt)
	}
	return ProxyTestResult{ProxyId: r.ProxyId, Ok: r.Ok, LatencyMs: r.LatencyMs, Error: r.Error}
}

// beginProxyBatch 创建可取消的批量操作上下文，并取消上一个未结束的批次。
func (a *App) beginProxyBatch() (context.Context, int) {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithCancel(base)
	a.proxyBatchMu.Lock()
	if a.proxyBatchCancel != nil {
		a.proxyBatchCancel()
	}
	a.proxyBatchGen++
	gen := a.proxyBatchGen
	a.proxyBatchCancel = cancel
	a.proxyBatchMu.Unlock()
	return ctx, gen
}

// endProxyBatch 结束指定代数的批量操作（仅当它仍是当前批次时清理）。
func (a *App) endProxyBatch(gen int) {
	a.proxyBatchMu.Lock()
	if a.proxyBatchGen == gen && a.proxyBatchCancel != nil {
		a.proxyBatchCancel()
		a.proxyBatchCancel = nil
	}
	a.proxyBatchMu.Unlock()
}

// BrowserProxyCancelBatch 取消正在进行的批量测速 / IP 健康检测（停止派发后续任务）。
func (a *App) BrowserProxyCancelBatch() {
	a.proxyBatchMu.Lock()
	cancel := a.proxyBatchCancel
	a.proxyBatchMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// BrowserProxyBatchTestSpeed 批量并发测速，concurrency 控制并发数（默认 20）
func (a *App) BrowserProxyBatchTestSpeed(proxyIds []string, concurrency int) []ProxyTestResult {
	if len(proxyIds) == 0 {
		return []ProxyTestResult{}
	}
	if concurrency <= 0 {
		concurrency = 20
	}
	if concurrency > len(proxyIds) {
		concurrency = len(proxyIds)
	}
	ctx, gen := a.beginProxyBatch()
	defer a.endProxyBatch(gen)
	proxies := a.getLatestProxies()
	results := make([]ProxyTestResult, len(proxyIds))
	type speedJob struct {
		Idx     int
		ProxyId string
	}
	jobs := make(chan speedJob, len(proxyIds))
	var wg sync.WaitGroup

	// 固定大小 worker 池，避免大量代理时创建过多 goroutine
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				// 被取消则停止派发后续任务（在飞任务自然结束）
				select {
				case <-ctx.Done():
					return
				default:
				}
				r := proxy.SpeedTest(job.ProxyId, proxies, a.xrayMgr, a.singboxMgr, nil)
				if a.browserMgr.ProxyDAO != nil {
					testedAt := time.Now().Format(time.RFC3339)
					_ = a.browserMgr.ProxyDAO.UpdateSpeedResult(job.ProxyId, r.Ok, r.LatencyMs, testedAt)
				}
				result := ProxyTestResult{ProxyId: r.ProxyId, Ok: r.Ok, LatencyMs: r.LatencyMs, Error: r.Error}
				results[job.Idx] = result

				// 实时推送单个结果到前端
				if a.ctx != nil {
					runtime.EventsEmit(a.ctx, "proxy:speed:result", result)
				}
			}
		}()
	}

	for i, pid := range proxyIds {
		jobs <- speedJob{Idx: i, ProxyId: pid}
	}
	close(jobs)

	wg.Wait()
	okCount := 0
	for _, r := range results {
		if r.Ok {
			okCount++
		}
	}
	a.recordActivity("speedtest", "info", fmt.Sprintf("批量测速完成：%d/%d 可用", okCount, len(results)), "")
	return results
}

// BrowserProxyCheckIPHealth 检测单个代理的出口 IP 健康信息（通过 IPPure 接口）
func (a *App) BrowserProxyCheckIPHealth(proxyId string) ProxyIPHealthResult {
	proxies := a.getLatestProxies()
	data, err := proxy.FetchIPPureInfo(proxyId, proxies, a.xrayMgr, a.singboxMgr)
	result := buildProxyIPHealthResult(proxyId, data, err)
	a.persistProxyIPHealthResult(result)
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "proxy:iphealth:result", result)
	}
	return result
}

// BrowserProxyBatchCheckIPHealth 批量并发检测代理出口 IP 健康信息
func (a *App) BrowserProxyBatchCheckIPHealth(proxyIds []string, concurrency int) []ProxyIPHealthResult {
	if len(proxyIds) == 0 {
		return []ProxyIPHealthResult{}
	}
	if concurrency <= 0 {
		concurrency = 10
	}
	if concurrency > len(proxyIds) {
		concurrency = len(proxyIds)
	}

	ctx, gen := a.beginProxyBatch()
	defer a.endProxyBatch(gen)
	proxies := a.getLatestProxies()
	results := make([]ProxyIPHealthResult, len(proxyIds))
	type healthJob struct {
		Idx     int
		ProxyId string
	}
	jobs := make(chan healthJob, len(proxyIds))
	var wg sync.WaitGroup

	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				data, err := proxy.FetchIPPureInfo(job.ProxyId, proxies, a.xrayMgr, a.singboxMgr)
				result := buildProxyIPHealthResult(job.ProxyId, data, err)
				a.persistProxyIPHealthResult(result)
				results[job.Idx] = result
				if a.ctx != nil {
					runtime.EventsEmit(a.ctx, "proxy:iphealth:result", result)
				}
			}
		}()
	}

	for i, pid := range proxyIds {
		jobs <- healthJob{Idx: i, ProxyId: pid}
	}
	close(jobs)

	wg.Wait()
	return results
}

func buildProxyIPHealthResult(proxyId string, data map[string]interface{}, err error) ProxyIPHealthResult {
	if err != nil {
		return ProxyIPHealthResult{
			ProxyId:   proxyId,
			Ok:        false,
			Source:    "ippure",
			Error:     err.Error(),
			RawData:   map[string]interface{}{},
			UpdatedAt: time.Now().Format(time.RFC3339),
		}
	}

	if data == nil {
		data = map[string]interface{}{}
	}

	return ProxyIPHealthResult{
		ProxyId:        proxyId,
		Ok:             true,
		Source:         "ippure",
		Error:          "",
		IP:             mapString(data, "ip"),
		FraudScore:     mapInt64(data, "fraudScore"),
		IsResidential:  mapBool(data, "isResidential"),
		IsBroadcast:    mapBool(data, "isBroadcast"),
		Country:        mapString(data, "country"),
		Region:         mapString(data, "region"),
		City:           mapString(data, "city"),
		AsOrganization: mapString(data, "asOrganization"),
		RawData:        data,
		UpdatedAt:      time.Now().Format(time.RFC3339),
	}
}

func (a *App) persistProxyIPHealthResult(result ProxyIPHealthResult) {
	if a.browserMgr.ProxyDAO == nil {
		return
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return
	}
	_ = a.browserMgr.ProxyDAO.UpdateIPHealthResult(result.ProxyId, string(payload))
}

func mapString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprint(v)
	}
}

func mapInt64(m map[string]interface{}, key string) int64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return int64(n)
	case int8:
		return int64(n)
	case int16:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	case uint:
		return int64(n)
	case uint8:
		return int64(n)
	case uint16:
		return int64(n)
	case uint32:
		return int64(n)
	case uint64:
		return int64(n)
	case float32:
		return int64(n)
	case float64:
		return int64(n)
	case json.Number:
		if iv, err := n.Int64(); err == nil {
			return iv
		}
		if fv, err := n.Float64(); err == nil {
			return int64(fv)
		}
	case string:
		if iv, err := strconv.ParseInt(n, 10, 64); err == nil {
			return iv
		}
		if fv, err := strconv.ParseFloat(n, 64); err == nil {
			return int64(fv)
		}
	}
	return 0
}

func mapBool(m map[string]interface{}, key string) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return strings.EqualFold(b, "true") || b == "1"
	case int:
		return b != 0
	case int64:
		return b != 0
	case float64:
		return b != 0
	}
	return false
}

// getLatestProxies 获取最新的代理列表，优先从数据库读取
func (a *App) getLatestProxies() []BrowserProxy {
	if a.browserMgr.ProxyDAO != nil {
		if list, err := a.browserMgr.ProxyDAO.List(); err == nil && len(list) > 0 {
			return list
		}
	}
	return a.config.Browser.Proxies
}

func (a *App) SaveBrowserProxies(proxies []BrowserProxy) error {
	log := logger.New("Browser")
	normalized := make([]BrowserProxy, 0, len(proxies))
	for i, item := range proxies {
		proxyName := strings.TrimSpace(item.ProxyName)
		proxyConfig := strings.TrimSpace(item.ProxyConfig)
		if proxyName == "" || proxyConfig == "" {
			continue
		}
		proxyId := strings.TrimSpace(item.ProxyId)
		if proxyId == "" {
			proxyId = generateUUID()
		}
		sourceURL := strings.TrimSpace(item.SourceURL)
		sourceID := strings.TrimSpace(item.SourceID)
		sourceNamePrefix := strings.TrimSpace(item.SourceNamePrefix)
		sourceLastRefreshAt := strings.TrimSpace(item.SourceLastRefreshAt)
		sourceRefreshIntervalM := item.SourceRefreshIntervalM
		if sourceRefreshIntervalM < 0 {
			sourceRefreshIntervalM = 0
		}
		if sourceRefreshIntervalM > 24*60 {
			sourceRefreshIntervalM = 24 * 60
		}
		sourceAutoRefresh := item.SourceAutoRefresh && sourceURL != ""
		if sourceAutoRefresh && sourceRefreshIntervalM <= 0 {
			sourceRefreshIntervalM = 60
		}
		if !sourceAutoRefresh {
			sourceRefreshIntervalM = 0
		}
		if sourceURL == "" {
			sourceID = ""
			sourceNamePrefix = ""
			sourceLastRefreshAt = ""
			sourceAutoRefresh = false
			sourceRefreshIntervalM = 0
		}
		normalized = append(normalized, BrowserProxy{
			ProxyId:                proxyId,
			ProxyName:              proxyName,
			ProxyConfig:            proxyConfig,
			DnsServers:             strings.TrimSpace(item.DnsServers),
			GroupName:              strings.TrimSpace(item.GroupName),
			SourceID:               sourceID,
			SourceURL:              sourceURL,
			SourceNamePrefix:       sourceNamePrefix,
			SourceAutoRefresh:      sourceAutoRefresh,
			SourceRefreshIntervalM: sourceRefreshIntervalM,
			SourceLastRefreshAt:    sourceLastRefreshAt,
			SortOrder:              i,
		})
	}

	// 确保内置代理始终存在（直连 + 本地代理）
	builtins := []BrowserProxy{
		{ProxyId: "__direct__", ProxyName: "直连（不走代理）", ProxyConfig: "direct://"},
		{ProxyId: "__local__", ProxyName: "本地代理", ProxyConfig: "http://127.0.0.1:7890"},
	}
	for _, b := range builtins {
		found := false
		for _, p := range normalized {
			if p.ProxyId == b.ProxyId {
				found = true
				break
			}
		}
		if !found {
			normalized = append([]BrowserProxy{b}, normalized...)
		}
	}

	a.config.Browser.Proxies = normalized

	// 优先写入 SQLite
	if a.browserMgr.ProxyDAO != nil {
		// 事务化整表替换并在事务内回填测速/IP 健康结果（避免部分失败丢数据，避免重插清空健康列）
		if err := a.browserMgr.ProxyDAO.ReplaceAllProxies(normalized); err != nil {
			log.Error("代理列表保存失败", logger.F("error", err))
			return err
		}
		log.Info("代理列表已保存到数据库", logger.F("count", len(normalized)))
		a.reconcileProfileProxyBindings()
		a.recordActivity("config", "info", fmt.Sprintf("代理配置已更新（%d 条）", len(normalized)), "")
		return nil
	}

	// 降级：写入 proxies.yaml
	if err := config.SaveProxies(a.resolveAppPath("proxies.yaml"), normalized); err != nil {
		log.Error("代理列表保存失败", logger.F("error", err))
		return err
	}
	a.reconcileProfileProxyBindings()
	return nil
}

// ============================================================================
// 文件系统 API
// ============================================================================

// OpenUserDataDir 在资源管理器中打开用户数据目录
func (a *App) OpenUserDataDir(userDataDir string) error {
	log := logger.New("Browser")

	// 解析完整路径
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		return fmt.Errorf("用户数据目录不能为空")
	}

	var fullPath string
	if filepath.IsAbs(userDataDir) {
		fullPath = userDataDir
	} else {
		root := strings.TrimSpace(a.config.Browser.UserDataRoot)
		if root == "" {
			root = "data"
		}
		root = a.resolveAppPath(root)
		fullPath = filepath.Join(root, userDataDir)
	}

	// 检查目录是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// 目录不存在，尝试创建
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			log.Error("创建用户数据目录失败", logger.F("path", fullPath), logger.F("error", err))
			return fmt.Errorf("创建目录失败: %v", err)
		}
	}

	// 获取绝对路径
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		log.Error("获取绝对路径失败", logger.F("path", fullPath), logger.F("error", err))
		return err
	}

	if err := openPathInFileManager(absPath); err != nil {
		log.Error("打开资源管理器失败", logger.F("path", absPath), logger.F("error", err))
		return err
	}

	log.Info("已打开用户数据目录", logger.F("path", absPath))
	return nil
}

// OpenCorePath 在资源管理器中打开内核路径
func (a *App) OpenCorePath(corePath string) error {
	log := logger.New("Browser")

	corePath = strings.TrimSpace(corePath)
	if corePath == "" {
		return fmt.Errorf("内核路径不能为空")
	}

	var fullPath string
	if filepath.IsAbs(corePath) {
		fullPath = corePath
	} else {
		fullPath = a.resolveAppPath(corePath)
	}

	// 检查目录是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("路径不存在: %s", fullPath)
	}

	// 获取绝对路径
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		log.Error("获取绝对路径失败", logger.F("path", fullPath), logger.F("error", err))
		return err
	}

	if err := openPathInFileManager(absPath); err != nil {
		log.Error("打开资源管理器失败", logger.F("path", absPath), logger.F("error", err))
		return err
	}

	log.Info("已打开内核路径", logger.F("path", absPath))
	return nil
}

// openPathInFileManager 调用系统文件管理器打开路径。
// Windows 下不能复用 hideWindow，否则可能导致资源管理器窗口被隐藏。
func openPathInFileManager(absPath string) error {
	// 防止命令注入：清理路径并验证无shell元字符
	absPath = filepath.Clean(absPath)

	// 验证路径不包含shell元字符
	if strings.ContainsAny(absPath, ";|&$`\\\"'<>") {
		return fmt.Errorf("invalid path: contains shell metacharacters")
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}

	switch goruntime.GOOS {
	case "windows":
		if info.IsDir() {
			return exec.Command("explorer.exe", absPath).Start()
		}
		return exec.Command("explorer.exe", "/select,", absPath).Start()
	case "darwin":
		if info.IsDir() {
			return exec.Command("open", absPath).Start()
		}
		return exec.Command("open", "-R", absPath).Start()
	default:
		target := absPath
		if !info.IsDir() {
			target = filepath.Dir(absPath)
		}
		return exec.Command("xdg-open", target).Start()
	}
}

// ============================================================================
// 数据迁移
// ============================================================================

// migrateToSQLite 一次性迁移：若 SQLite 表为空则从旧文件导入数据，或初始化默认数据
// 迁移顺序：cores → proxies → profiles → bookmarks
func (a *App) migrateToSQLite() {
	log := logger.New("Migration")

	// 迁移/初始化内核
	if cores, err := a.browserMgr.CoreDAO.List(); err == nil && len(cores) == 0 {
		// 优先从 config.yaml 迁移
		if len(a.config.Browser.Cores) > 0 {
			for _, c := range a.config.Browser.Cores {
				if err := a.browserMgr.CoreDAO.Upsert(c); err != nil {
					log.Error("内核迁移失败", logger.F("core_id", c.CoreId), logger.F("error", err))
				}
			}
			log.Info("内核数据已迁移", logger.F("count", len(a.config.Browser.Cores)))
		} else {
			// 初始化默认内核（自动检测会补充）
			log.Info("内核表为空，将通过自动检测初始化")
		}
	}

	// 迁移/初始化代理
	if proxies, err := a.browserMgr.ProxyDAO.List(); err == nil && len(proxies) == 0 {
		var srcProxies []browser.Proxy
		// 优先 proxies.yaml，其次 config.yaml
		if loaded, err := config.LoadProxies(a.resolveAppPath("proxies.yaml")); err == nil && len(loaded) > 0 {
			srcProxies = loaded
		} else if len(a.config.Browser.Proxies) > 0 {
			srcProxies = a.config.Browser.Proxies
		} else {
			// 初始化默认代理
			srcProxies = []browser.Proxy{
				{ProxyId: "__direct__", ProxyName: "直连（不走代理）", ProxyConfig: "direct://"},
				{ProxyId: "__local__", ProxyName: "本地代理", ProxyConfig: "http://127.0.0.1:7890"},
			}
			log.Info("代理表为空，初始化默认代理")
		}
		for _, p := range srcProxies {
			if err := a.browserMgr.ProxyDAO.Upsert(p); err != nil {
				log.Error("代理迁移失败", logger.F("proxy_id", p.ProxyId), logger.F("error", err))
			}
		}
		if len(srcProxies) > 0 {
			log.Info("代理数据已初始化", logger.F("count", len(srcProxies)))
		}
	}

	// 迁移实例配置（如果为空则自动创建一个默认实例）
	if profiles, err := a.browserMgr.ProfileDAO.List(); err == nil && len(profiles) == 0 {
		if len(a.config.Browser.Profiles) > 0 {
			for _, pc := range a.config.Browser.Profiles {
				coreId := strings.TrimSpace(pc.CoreId)
				if strings.EqualFold(coreId, "default") {
					coreId = ""
				}
				p := &browser.Profile{
					ProfileId:          pc.ProfileId,
					ProfileName:        pc.ProfileName,
					UserDataDir:        pc.UserDataDir,
					CoreId:             coreId,
					FingerprintArgs:    pc.FingerprintArgs,
					ProxyId:            pc.ProxyId,
					ProxyConfig:        pc.ProxyConfig,
					ProxyBindSourceID:  pc.ProxyBindSourceID,
					ProxyBindSourceURL: pc.ProxyBindSourceURL,
					ProxyBindName:      pc.ProxyBindName,
					ProxyBindUpdatedAt: pc.ProxyBindUpdatedAt,
					LaunchArgs:         pc.LaunchArgs,
					Tags:               pc.Tags,
					Keywords:           pc.Keywords,
					CreatedAt:          pc.CreatedAt,
					UpdatedAt:          pc.UpdatedAt,
				}
				if err := a.browserMgr.ProfileDAO.Upsert(p); err != nil {
					log.Error("实例迁移失败", logger.F("profile_id", pc.ProfileId), logger.F("error", err))
				}
			}
			log.Info("实例数据已迁移", logger.F("count", len(a.config.Browser.Profiles)))
		} else {
			log.Info("实例表为空，自动创建默认实例")
			defaultProfile := &browser.Profile{
				ProfileId:       generateUUID(),
				ProfileName:     "默认实例",
				UserDataDir:     "default",
				CoreId:          "",
				FingerprintArgs: a.config.Browser.DefaultFingerprintArgs,
				LaunchArgs:      a.config.Browser.DefaultLaunchArgs,
				Tags:            []string{"默认"},
				ProxyId:         a.config.Browser.DefaultProxy,
				CreatedAt:       time.Now().Format(time.RFC3339),
				UpdatedAt:       time.Now().Format(time.RFC3339),
			}
			if err := a.browserMgr.ProfileDAO.Upsert(defaultProfile); err != nil {
				log.Error("自动创建默认实例失败", logger.F("error", err))
			}
		}
	}

	// 迁移/初始化书签
	if bookmarks, err := a.browserMgr.BookmarkDAO.List(); err == nil && len(bookmarks) == 0 {
		src := a.config.Browser.DefaultBookmarks
		if len(src) == 0 {
			// 初始化默认书签
			src = []config.BrowserBookmark{
				{Name: "Google", URL: "https://www.google.com/"},
				{Name: "Gmail", URL: "https://mail.google.com/"},
				{Name: "Claude", URL: "https://claude.ai/"},
				{Name: "ChatGPT", URL: "https://chatgpt.com/"},
				{Name: "YouTube", URL: "https://www.youtube.com/"},
			}
		}
		if err := a.browserMgr.BookmarkDAO.ReplaceAll(src); err != nil {
			log.Error("书签迁移失败", logger.F("error", err))
		} else {
			log.Info("书签数据已迁移", logger.F("count", len(src)))
		}
	}
}
