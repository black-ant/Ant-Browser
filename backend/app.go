package backend

import (
	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"ant-chrome/backend/internal/identity"
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"
	"context"
	"strings"
	"sync"
)

type quitMode uint8

const (
	quitModeFull quitMode = iota
	quitModeAppOnly
)

// App 应用结构体
type App struct {
	ctx            context.Context
	config         *config.Config
	db             *database.DB
	interceptor    *logger.MethodInterceptor
	browserMgr     *browser.Manager
	xrayMgr        *proxy.XrayManager
	clashMgr       *proxy.ClashManager
	singboxMgr     *proxy.SingBoxManager
	launchCodeSvc  *launchcode.LaunchCodeService
	launchServer   *launchcode.LaunchServer
	automationMgr  *automation.Manager
	speedScheduler *browser.ProxySpeedScheduler
	geoResolver    *identity.MMDBResolver
	appRoot        string
	version        string

	forceQuit              bool
	quitMode               quitMode
	maintenanceMu          sync.Mutex
	bridgeMu               sync.Mutex
	profileBridgeRefs      map[string]profileProxyBridgeRef
	deferredStartTargetsMu sync.Mutex
	deferredStartTargets   map[string]deferredStartTargetsPlan
	automationTargetMu     sync.Mutex
	automationTargetCursor map[string]string
	stopServicesOnce       sync.Once
	finalizeOnce           sync.Once
}

// NewApp 创建新的应用实例
func NewApp(appRoot string, appVersion ...string) *App {
	version := ""
	if len(appVersion) > 0 {
		version = strings.TrimSpace(appVersion[0])
	}
	return &App{
		appRoot:                strings.TrimSpace(appRoot),
		version:                version,
		profileBridgeRefs:      make(map[string]profileProxyBridgeRef),
		deferredStartTargets:   make(map[string]deferredStartTargetsPlan),
		automationTargetCursor: make(map[string]string),
	}
}

func (a *App) appName() string {
	if a.config != nil {
		if name := strings.TrimSpace(a.config.App.Name); name != "" {
			return name
		}
	}
	return "ZwBrowser"
}

func (a *App) appVersion() string {
	version := strings.TrimSpace(a.version)
	if version == "" {
		return "unknown"
	}
	return version
}
