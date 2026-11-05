package backend

import (
	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"
	"context"
	"strings"
	"sync"
	"time"
)

type quitMode uint8

const (
	quitModeFull quitMode = iota
	quitModeAppOnly
)

// App 应用结构体
type App struct {
	ctx                         context.Context
	config                      *config.Config
	db                          *database.DB
	interceptor                 *logger.MethodInterceptor
	browserMgr                  *browser.Manager
	xrayMgr                     *proxy.XrayManager
	clashMgr                    *proxy.ClashManager
	singboxMgr                  *proxy.SingBoxManager
	mieruMgr                    *proxy.MieruManager
	launchCodeSvc               *launchcode.LaunchCodeService
	launchServer                *launchcode.LaunchServer
	automationMgr               *automation.Manager
	speedScheduler              *browser.ProxySpeedScheduler
	backupScheduler             *backupScheduler
	proxySourceRefreshScheduler *proxySourceRefreshScheduler
	appRoot                     string
	version                     string

	forceQuit                     bool
	quitMode                      quitMode
	quitMu                        sync.RWMutex
	runtimeMu                     sync.RWMutex
	runtimeStopped                bool
	backgroundTaskCtx             context.Context
	backgroundTaskCancel          context.CancelFunc
	backgroundTasks               sync.WaitGroup
	backgroundTasksBlocked        bool
	maintenanceMu                 sync.Mutex
	bridgeMu                      sync.Mutex
	proxyVirtualGroupMu           sync.Mutex
	proxyVirtualGroupCursor       map[string]int
	proxyVirtualGroupLocks        map[string]*sync.Mutex
	virtualGroupSnapshot          virtualGroupSnapshotCache
	proxyFailoverMu               sync.Mutex
	proxySourceIgnoredMu          sync.Mutex
	// proxySourceRefreshMu 串行化订阅刷新的"读池 → 合并 → 全量保存"过程。
	// 前端手动刷新、前端自动刷新与后端调度器可能同时触发，非原子会导致后保存者覆盖先保存者的结果。
	proxySourceRefreshMu sync.Mutex
	proxyFailoverLastAt  map[string]time.Time
	proxyFailoverCount   map[string]int
	// instanceGate 实例并发启动闸门（安全控制），见 app_instance_launch_gate.go。
	instanceGate     *instanceLaunchGate
	instanceGateOnce sync.Once
	profileBridgeRefs             map[string]profileProxyBridgeRef
	deferredStartTargetsMu        sync.Mutex
	deferredStartTargets          map[string]deferredStartTargetsPlan
	automationTargetMu            sync.Mutex
	automationTargetCursor        map[string]string
	profileWindowMarkersMu        sync.Mutex
	profileWindowMarkers          map[string]*profileWindowMarker
	browserProcessMonitors        map[string]*browserProcessMonitor
	backupLocalConfigPathOverride string
	stopServicesOnce              sync.Once
	finalizeOnce                  sync.Once
}

// NewApp 创建新的应用实例
func NewApp(appRoot string, appVersion ...string) *App {
	version := ""
	if len(appVersion) > 0 {
		version = strings.TrimSpace(appVersion[0])
	}
	return &App{
		appRoot:                 strings.TrimSpace(appRoot),
		version:                 version,
		profileBridgeRefs:       make(map[string]profileProxyBridgeRef),
		proxyVirtualGroupCursor: make(map[string]int),
		proxyVirtualGroupLocks:  make(map[string]*sync.Mutex),
		proxyFailoverLastAt:     make(map[string]time.Time),
		proxyFailoverCount:      make(map[string]int),
		deferredStartTargets:    make(map[string]deferredStartTargetsPlan),
		automationTargetCursor:  make(map[string]string),
		profileWindowMarkers:    make(map[string]*profileWindowMarker),
		browserProcessMonitors:  make(map[string]*browserProcessMonitor),
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
