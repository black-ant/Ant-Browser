package backend

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"

	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// startup 应用启动时调用
func (a *App) startup(ctx context.Context) {
	a.setRuntimeContext(ctx)
	if err := apppath.EnsureWritableLayout(a.appRoot); err != nil {
		runtime.LogFatal(ctx, fmt.Sprintf("初始化 Linux 用户数据目录失败: %v", err))
		return
	}

	cfg := a.startupLoadConfig()
	a.config = cfg
	a.applyRuntimeConfig(cfg.Runtime)

	log := a.startupInitLogger(ctx, cfg)
	if err := killResidualRuntimeProcesses(a.appRoot); err != nil {
		log.Warn("清理上次异常退出遗留的代理进程失败", logger.F("error", err))
	}
	a.startupLogEnvironment(log, cfg)
	a.activateStableBackupLocalConfig()
	if err := a.prepareBackupLocalConfig(); err != nil {
		log.Error("本地备份配置初始化失败", logger.F("error", err))
	}

	if err := os.MkdirAll(a.resolveAppPath("data"), 0o755); err != nil {
		log.Error("创建 data 目录失败", logger.F("error", err))
	}

	a.ensureDefaultCores()
	a.startupInitInterceptor(log, cfg)

	db, err := a.startupInitDatabase(cfg)
	if err != nil {
		log.Error("初始化数据库失败", logger.F("error", err))
		runtime.LogFatal(ctx, fmt.Sprintf("初始化数据库失败: %v", err))
		return
	}
	a.db = db

	// 数据库升级安全序列：备份 → 版本迁移 → 自愈对齐。
	// 备份失败不阻断（迁移语句幂等且各版本事务化），仅记录告警。
	dbBackupDir := a.resolveAppPath("data/db-backups")
	if db.HasPendingMigrations() {
		if backupPath, backupErr := db.BackupTo(dbBackupDir, "app-pre-migrate"); backupErr != nil {
			log.Warn("数据库升级前备份失败（继续执行版本迁移）", logger.F("error", backupErr.Error()))
		} else {
			log.Info("数据库升级前已创建备份", logger.F("path", backupPath))
		}
	}
	if err := db.Migrate(); err != nil {
		log.Error("数据库迁移失败", logger.F("error", err))
		_ = db.Close()
		runtime.LogFatal(ctx, fmt.Sprintf("数据库迁移失败: %v", err))
		return
	}

	// 自愈对齐：修复版本号冲突等导致的"版本达标但列缺失"异构库。
	// 失败仅记录并放行启动（运行时已有降级容错），下次启动自动重试。
	if report, reconcileErr := db.Reconcile(dbBackupDir); reconcileErr != nil {
		log.Warn("数据库结构自愈对齐失败（已跳过，下次启动重试）", logger.F("error", reconcileErr.Error()))
	} else if len(report.AddedTables) > 0 || len(report.AddedColumns) > 0 {
		log.Info("数据库结构自愈对齐完成",
			logger.F("added_tables", strings.Join(report.AddedTables, ",")),
			logger.F("added_columns", strings.Join(report.AddedColumns, ",")),
			logger.F("backup", report.BackupPath),
		)
	}

	if err := a.startupInitManagers(cfg, db); err != nil {
		log.Error("浏览器数据初始化失败", logger.F("error", err))
		_ = db.Close()
		runtime.LogFatal(ctx, fmt.Sprintf("浏览器数据初始化失败: %v", err))
		return
	}
	a.startupInitLaunchCode(log)
	a.startupInitLaunchServer(log)
	a.startupInitAutomation()
	a.startupInitBridgeHooks()
	a.startupInitSpeedScheduler()
	a.startupInitBackupScheduler()
	a.emitRuntimeEvent("app:ready")

	log.Info("应用启动成功")
}

func (a *App) startupLoadConfig() *config.Config {
	cfg, err := LoadConfig(a.resolveAppPath("config.yaml"))
	if err != nil {
		return config.DefaultConfig()
	}
	return cfg
}

func (a *App) startupInitLogger(ctx context.Context, cfg *config.Config) *logger.Logger {
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
	return logger.New("App")
}

func (a *App) startupLogEnvironment(log *logger.Logger, cfg *config.Config) {
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
}

func (a *App) startupInitInterceptor(log *logger.Logger, cfg *config.Config) {
	if !cfg.Logging.Interceptor.Enabled {
		return
	}
	interceptorConfig := logger.InterceptorConfig{
		Enabled:         cfg.Logging.Interceptor.Enabled,
		LogParameters:   cfg.Logging.Interceptor.LogParameters,
		LogResults:      cfg.Logging.Interceptor.LogResults,
		SensitiveFields: cfg.Logging.Interceptor.SensitiveFields,
	}
	a.interceptor = logger.NewMethodInterceptor(log, interceptorConfig)
}

func (a *App) startupInitDatabase(cfg *config.Config) (*database.DB, error) {
	return database.NewDB(a.resolveAppPath(cfg.Database.SQLite.Path))
}

func (a *App) startupInitManagers(cfg *config.Config, db *database.DB) error {
	a.browserMgr = browser.NewManager(cfg, a.appRoot)
	a.browserMgr.EventEmitter = a.emitRuntimeEvent
	a.browserMgr.ProfileRuntimeDetector = func(profile *browser.Profile) bool {
		return browserProfileHasRuntime(a, profile, nil)
	}
	a.xrayMgr = proxy.NewXrayManager(cfg, a.appRoot)
	a.clashMgr = proxy.NewClashManager(cfg, a.appRoot)
	a.singboxMgr = proxy.NewSingBoxManager(cfg, a.appRoot)
	a.browserMgr.CoreDownloadHTTPClientFactory = func(proxyConfig string, timeout time.Duration) (*http.Client, error) {
		client, _, err := a.unifiedProxyCoreHTTPClient(timeout, proxyConfig)
		return client, err
	}

	conn := db.GetConn()
	a.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(conn)
	a.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(conn)
	a.browserMgr.ProxyVirtualGroupDAO = browser.NewSQLiteProxyVirtualGroupDAO(conn)
	a.browserMgr.CoreDAO = browser.NewSQLiteCoreDAO(conn)
	a.browserMgr.BookmarkDAO = browser.NewSQLiteBookmarkDAO(conn)
	a.browserMgr.GroupDAO = browser.NewSQLiteGroupDAO(conn)
	a.browserMgr.ExtensionDAO = browser.NewSQLiteExtensionDAO(conn)

	if err := a.migrateToSQLite(); err != nil {
		return err
	}

	a.browserMgr.InitData()
	if err := a.browserMgr.MigrateLegacyExternalExtensionRegistry(); err != nil {
		logger.New("Browser").Error("启动迁移旧版插件外部注册失败", logger.F("error", err))
	}
	if err := a.browserMgr.CleanupExpiredTrash(); err != nil {
		logger.New("Browser").Error("启动清理回收站失败", logger.F("error", err))
	}
	a.autoDetectCores()
	if err := a.loadProxies(); err != nil {
		return err
	}
	a.reconcileProfileProxyBindings()
	return nil
}

func (a *App) startupInitLaunchCode(log *logger.Logger) {
	launchCodeDAO := launchcode.NewSQLiteLaunchCodeDAO(a.db.GetConn())
	a.launchCodeSvc = launchcode.NewLaunchCodeService(launchCodeDAO)
	if err := a.launchCodeSvc.LoadAll(); err != nil {
		log.Error("LaunchCode 加载失败", logger.F("error", err))
	}
	a.browserMgr.CodeProvider = a.launchCodeSvc
}

func (a *App) startupInitLaunchServer(log *logger.Logger) {
	port := a.config.LaunchServer.Port
	a.launchServer = launchcode.NewLaunchServer(a.launchCodeSvc, a, a.browserMgr, port)
	a.launchServer.SetAPIAuthConfig(launchcode.APIAuthConfig{
		Enabled: a.config.LaunchServer.Auth.Enabled,
		APIKey:  a.config.LaunchServer.Auth.APIKey,
		Header:  a.config.LaunchServer.Auth.Header,
	})
	if err := a.launchServer.Start(); err != nil {
		log.Error("LaunchServer 启动失败", logger.F("error", err))
		return
	}
	log.Debug("LaunchServer 监听地址",
		logger.F("url", fmt.Sprintf("http://127.0.0.1:%d", a.launchServer.Port())),
		logger.F("preferred_port", port),
	)
}

func (a *App) startupInitAutomation() {
	a.automationMgr = automation.NewManager(a.appRoot, a.config, func(event string, payload any) {
		a.emitRuntimeEvent(event, payload)
	}, automation.Options{})
}

func (a *App) startupInitBridgeHooks() {
	if a.xrayMgr != nil {
		a.xrayMgr.OnBridgeDied = func(key string, err error) {
			a.handleProxyBridgeDied(profileProxyBridgeEngineXray, key, err)
		}
	}
	if a.singboxMgr != nil {
		a.singboxMgr.OnBridgeDied = func(key string, err error) {
			a.handleProxyBridgeDied(profileProxyBridgeEngineSingBox, key, err)
		}
	}
	if a.clashMgr != nil {
		a.clashMgr.OnNodeBridgeDied = func(key string, err error) {
			a.handleProxyBridgeDied(profileProxyBridgeEngineMihomo, key, err)
		}
	}
}

func bridgeEventKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:8]
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (a *App) startupInitSpeedScheduler() {
	a.ensureSpeedScheduler()
}

func (a *App) ensureSpeedScheduler() {
	if a == nil || a.config == nil || a.browserMgr == nil || a.browserMgr.ProxyDAO == nil {
		return
	}
	interval, failureRetry, schedulerConcurrency := a.proxySpeedSchedule()
	if a.speedScheduler == nil {
		a.speedScheduler = browser.NewProxySpeedSchedulerWithRetry(
			a.browserMgr.ProxyDAO,
			nil,
			interval,
			failureRetry,
			schedulerConcurrency,
		)
		a.speedScheduler.SetProxySpeedTestFunc(func(item browser.Proxy) (bool, int64, string) {
			result := a.testSharedProxySpeed(item.ProxyId, true)
			a.emitRuntimeEvent("proxy:speed:result", result)
			return result.Ok, result.LatencyMs, result.Error
		})
		a.speedScheduler.SetResultCurrentFunc(func(item browser.Proxy, proxies []browser.Proxy) bool {
			return a.proxySpeedResultIsCurrent(item, proxies)
		})
		a.speedScheduler.SetResultPersistence(false)
	}
	a.speedScheduler.Start()
}

func (a *App) proxySpeedSchedule() (time.Duration, time.Duration, int) {
	settings := config.DefaultConfig().ProxyCheck
	if a != nil && a.config != nil {
		settings = a.config.ProxyCheck
	}
	settings = proxy.NormalizeCheckSettings(settings)
	interval := time.Duration(settings.SpeedIntervalMinutes) * time.Minute
	failureRetry := time.Duration(settings.FailureRetryMinutes) * time.Minute
	concurrency := settings.SpeedConcurrency
	return interval, failureRetry, concurrency
}

func (a *App) restartSpeedScheduler() {
	if a == nil {
		return
	}
	if a.speedScheduler != nil {
		a.speedScheduler.Stop()
		a.speedScheduler = nil
	}
	a.ensureSpeedScheduler()
}
