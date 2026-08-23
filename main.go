package main

import (
	"ant-chrome/backend"
	"context"
	"embed"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed wails.json
var wailsConfigJSON []byte

//go:embed build/appicon.png
var linuxAppIcon []byte

// appRoot 应用根目录，所有相对路径基于此目录解析。
// 生产环境 = exe 所在目录；dev 环境 = 项目源码根目录（CWD）。
var appRoot string

// isDevMode 标识当前是否为 wails dev 模式（exe 在临时目录）
var isDevMode bool

type App struct {
	*backend.App
}

type wailsBuildConfig struct {
	Info struct {
		ProductVersion string `json:"productVersion"`
	} `json:"info"`
}

func envFlagEnabled(name string) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func resolveBuildVersion() string {
	var cfg wailsBuildConfig
	if err := json.Unmarshal(wailsConfigJSON, &cfg); err != nil {
		log.Printf("解析 wails.json 版本信息失败: %v", err)
		return "unknown"
	}

	version := strings.TrimSpace(cfg.Info.ProductVersion)
	if version == "" {
		log.Printf("wails.json 未配置 info.productVersion，回退为 unknown")
		return "unknown"
	}

	return version
}

func NewApp(appRoot, version string) *App {
	return &App{App: backend.NewApp(appRoot, version)}
}

func (a *App) startup(ctx context.Context) {
	backend.Start(a.App, ctx)
}

func (a *App) shutdown(ctx context.Context) {
	backend.Stop(a.App, ctx)
}

func (a *App) shouldBlockClose(ctx context.Context) bool {
	return backend.ShouldBlockClose(a.App, ctx)
}

func (a *App) BrowserExtensionManualInstallGuide(query string) (backend.BrowserExtensionManualInstallGuide, error) {
	return a.App.BrowserExtensionManualInstallGuide(query)
}

func (a *App) BrowserExtensionOpenManualDownloadDir() error {
	return a.App.BrowserExtensionOpenManualDownloadDir()
}

func (a *App) BrowserExtensionListManualDownloadFiles() ([]backend.BrowserExtensionManualDownloadFile, error) {
	return a.App.BrowserExtensionListManualDownloadFiles()
}

func (a *App) BrowserExtensionInstallManualDownloadFile(fileName string) (backend.BrowserExtension, error) {
	return a.App.BrowserExtensionInstallManualDownloadFile(fileName)
}

func main() {
	defer func() {
		if recovered := recover(); recovered != nil {
			logLifecyclePanic(appRoot, "process.panic", recovered)
			panic(recovered)
		}
	}()

	// 确定应用根目录：
	// 1. 生产环境：exe 所在目录（快捷方式启动时 CWD 可能不对，需要修正）
	// 2. dev 环境：wails dev 时 exe 可能在 temp 目录或 build/bin 目录，使用当前工作目录
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		tempDir := os.TempDir()
		if resolved, err := filepath.EvalSymlinks(exeDir); err == nil {
			exeDir = resolved
		}
		if resolved, err := filepath.EvalSymlinks(tempDir); err == nil {
			tempDir = resolved
		}

		exeDirLower := strings.ToLower(exeDir)
		inTemp := strings.HasPrefix(exeDirLower, strings.ToLower(tempDir))
		// wails dev 会把 exe 编译到 build/bin/ 目录
		inBuildBin := strings.HasSuffix(filepath.ToSlash(exeDirLower), "/build/bin")

		if inTemp || inBuildBin {
			// dev 模式：exe 在临时目录或 build/bin，使用 CWD 作为根目录
			isDevMode = true
			if cwd, err := os.Getwd(); err == nil {
				appRoot = cwd
			} else {
				appRoot = "."
			}
		} else {
			// 生产模式：使用 exe 所在目录
			isDevMode = false
			appRoot = exeDir
			os.Chdir(exeDir)
		}
	} else {
		// 兜底：使用 CWD
		if cwd, err := os.Getwd(); err == nil {
			appRoot = cwd
		} else {
			appRoot = "."
		}
	}

	startupDebugEnabled := envFlagEnabled("ANT_BROWSER_DEBUG_STARTUP")
	if startupDebugEnabled {
		log.Printf("应用根目录: %s (dev=%v)", appRoot, isDevMode)
	}
	if err := backend.EnsureRuntimeLayout(appRoot); err != nil {
		log.Printf("准备用户数据目录失败: %v", err)
		lifecycleLog(appRoot, "runtime_layout.error", map[string]interface{}{"error": err.Error()})
	}
	lifecycleLog(appRoot, "process.start", map[string]interface{}{"dev": isDevMode})
	singleInstance, primaryInstance, err := acquireSingleInstance(appRoot)
	if err != nil {
		log.Printf("单实例检查失败: %v", err)
		lifecycleLog(appRoot, "single_instance.error", map[string]interface{}{"error": err.Error()})
	}
	lifecycleLog(appRoot, "single_instance.checked", map[string]interface{}{"primary": primaryInstance})
	if !primaryInstance {
		lifecycleLog(appRoot, "process.exit.non_primary", nil)
		if startupDebugEnabled {
			log.Printf("检测到已有应用实例，已请求唤醒并退出当前进程")
		}
		return
	}
	defer singleInstance.Close()
	defer lifecycleLog(appRoot, "process.defer_exit", nil)
	if startupDebugEnabled && backend.RuntimeUsesDetachedState(appRoot) {
		log.Printf("检测到安装目录需要只读运行，状态目录切换到: %s", backend.RuntimeStateRoot(appRoot))
	}
	buildVersion := resolveBuildVersion()
	if startupDebugEnabled {
		log.Printf("应用版本: %s", buildVersion)
		log.Printf(
			"Wails 启动环境: GOOS=%s GOARCH=%s DISPLAY=%q WAYLAND_DISPLAY=%q XDG_SESSION_TYPE=%q XDG_CURRENT_DESKTOP=%q",
			goruntime.GOOS,
			goruntime.GOARCH,
			os.Getenv("DISPLAY"),
			os.Getenv("WAYLAND_DISPLAY"),
			os.Getenv("XDG_SESSION_TYPE"),
			os.Getenv("XDG_CURRENT_DESKTOP"),
		)
	}
	if startupDebugEnabled && goruntime.GOOS == "linux" && strings.TrimSpace(os.Getenv("DISPLAY")) == "" && strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) == "" {
		log.Printf("检测到 Linux 图形环境变量为空：DISPLAY / WAYLAND_DISPLAY 都未设置，GUI 窗口大概率无法创建")
	}

	// 加载配置
	cfg, err := backend.LoadConfig(backend.ResolveRuntimePath(appRoot, "config.yaml"))
	if err != nil {
		log.Printf("加载配置失败，使用默认配置: %v", err)
		lifecycleLog(appRoot, "config.error", map[string]interface{}{"error": err.Error()})
		cfg = backend.DefaultConfig()
	}
	lifecycleLog(appRoot, "config.loaded", map[string]interface{}{"path": backend.ResolveRuntimePath(appRoot, "config.yaml")})

	// 创建应用实例
	app := NewApp(appRoot, buildVersion)

	var wailsCtx context.Context
	startupReached := make(chan struct{})
	go func() {
		for activation := range singleInstance.activation {
			if wailsCtx == nil {
				select {
				case <-startupReached:
				case <-time.After(12 * time.Second):
				}
				if wailsCtx == nil {
					close(activation.done)
					continue
				}
			}
			runtime.WindowShow(wailsCtx)
			runtime.WindowUnminimise(wailsCtx)
			runtime.WindowSetAlwaysOnTop(wailsCtx, true)
			runtime.WindowSetAlwaysOnTop(wailsCtx, false)
			activateExistingSingleInstanceWindow(os.Getpid())
			close(activation.done)
		}
	}()

	if startupDebugEnabled {
		go func() {
			select {
			case <-startupReached:
				return
			case <-time.After(12 * time.Second):
				log.Printf("Wails OnStartup 在 12 秒内未触发。若终端一直转圈但没有窗口，优先检查 Linux 图形环境、libgtk-3、libwebkit2gtk，以及是否运行在 SSH/容器/无桌面会话中")
			}
		}()
	}

	// 启动应用
	if startupDebugEnabled {
		log.Printf("准备调用 wails.Run 创建 GUI 窗口")
	}
	windowBounds := resolveStartupWindowBounds(startupWindowBounds{
		Width:     cfg.App.Window.Width,
		Height:    cfg.App.Window.Height,
		MinWidth:  cfg.App.Window.MinWidth,
		MinHeight: cfg.App.Window.MinHeight,
	})
	if startupDebugEnabled {
		log.Printf(
			"窗口启动尺寸: width=%d height=%d minWidth=%d minHeight=%d",
			windowBounds.Width,
			windowBounds.Height,
			windowBounds.MinWidth,
			windowBounds.MinHeight,
		)
	}
	err = wails.Run(&options.App{
		Title:     cfg.App.Name,
		Width:     windowBounds.Width,
		Height:    windowBounds.Height,
		MinWidth:  windowBounds.MinWidth,
		MinHeight: windowBounds.MinHeight,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 245, G: 247, B: 250, A: 255},
		OnStartup: func(ctx context.Context) {
			lifecycleLog(appRoot, "wails.on_startup.begin", nil)
			close(startupReached)
			if startupDebugEnabled {
				log.Printf("Wails OnStartup 已触发，GUI 宿主已创建")
			}
			wailsCtx = ctx
			runtime.WindowCenter(wailsCtx)
			backend.ApplyMainApplicationWindowIcon(appRoot, cfg.App.Name)
			// 启动系统托盘（非阻塞）
			go runTraySafely(appRoot, backend.TrayCallbacks{
				OnShow: func() {
					lifecycleLog(appRoot, "tray.show", nil)
					runtime.WindowShow(wailsCtx)
					runtime.WindowUnminimise(wailsCtx)
					activateExistingSingleInstanceWindow(os.Getpid())
				},
				OnQuitAppOnly: func() {
					lifecycleLog(appRoot, "tray.quit_app_only", nil)
					app.QuitAppOnly()
				},
				OnQuit: func() {
					lifecycleLog(appRoot, "tray.quit", nil)
					app.ForceQuit()
				},
				OnExit: func() {
					lifecycleLog(appRoot, "tray.exit", nil)
				},
			})
			lifecycleLog(appRoot, "tray.start.requested", nil)
			app.startup(ctx)
			lifecycleLog(appRoot, "wails.on_startup.complete", nil)
			if startupDebugEnabled {
				log.Printf("后端 startup 已完成")
			}
		},
		OnShutdown: func(ctx context.Context) {
			lifecycleLog(appRoot, "wails.on_shutdown.begin", nil)
			if startupDebugEnabled {
				log.Printf("Wails OnShutdown 已触发")
			}
			backend.QuitTray()
			app.shutdown(ctx)
			lifecycleLog(appRoot, "wails.on_shutdown.complete", nil)
		},
		// 拦截关闭按钮事件，由前端处理自定义对话框
		OnBeforeClose: func(ctx context.Context) bool {
			lifecycleLog(appRoot, "wails.on_before_close.begin", nil)
			blocked := app.shouldBlockClose(ctx)
			lifecycleLog(appRoot, "wails.on_before_close.result", map[string]interface{}{"blocked": blocked})
			return blocked
		},
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			// Expose a real window icon to Linux desktop environments.
			Icon:             linuxAppIcon,
			WebviewGpuPolicy: linux.WebviewGpuPolicyNever,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})

	if err != nil {
		lifecycleLog(appRoot, "wails.run.error", map[string]interface{}{"error": err.Error()})
		log.Fatal("启动应用失败:", err)
	}
	lifecycleLog(appRoot, "wails.run.returned", nil)
	if startupDebugEnabled {
		log.Printf("wails.Run 已退出")
	}
}
