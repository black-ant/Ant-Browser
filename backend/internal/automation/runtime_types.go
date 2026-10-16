package automation

import (
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"

	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/config"
)

const ProgressEventName = "automation:runtime:progress"

type ProgressEvent struct {
	Phase     string `json:"phase"`
	Progress  int    `json:"progress"`
	Message   string `json:"message"`
	Component string `json:"component,omitempty"`
}

type RuntimeState struct {
	Enabled              bool   `json:"enabled"`
	InstallPolicy        string `json:"installPolicy"`
	RuntimeVersion       string `json:"runtimeVersion"`
	HeadlessDefault      bool   `json:"headlessDefault"`
	KeepRuntimeOnDisable bool   `json:"keepRuntimeOnDisable"`
	NodeSource           string `json:"nodeSource"`
	NodeResolution       string `json:"nodeResolution"`
	SystemNodeDetected   bool   `json:"systemNodeDetected"`
	SystemNodePath       string `json:"systemNodePath"`
	SystemNodeError      string `json:"systemNodeError"`
	Installed            bool   `json:"installed"`
	Ready                bool   `json:"ready"`
	Installing           bool   `json:"installing"`
	LastError            string `json:"lastError"`
	RuntimeDir           string `json:"runtimeDir"`
	NodePath             string `json:"nodePath"`
	RunnerPath           string `json:"runnerPath"`
	NodeVersion          string `json:"nodeVersion"`
	PlaywrightVersion    string `json:"playwrightVersion"`
}

type RuntimeCheckResult struct {
	OK                bool   `json:"ok"`
	NodeSource        string `json:"nodeSource"`
	NodeVersion       string `json:"nodeVersion"`
	PlaywrightVersion string `json:"playwrightVersion"`
}

type Options struct {
	NodeDistBaseURL    string
	NPMRegistryBaseURL string
	TargetOS           string
	TargetArch         string
	HTTPClient         *http.Client
}

type Manager struct {
	appRoot string
	config  *config.Config
	emit    func(string, any)
	options Options

	mu          sync.RWMutex
	installing  bool
	lastError   string
	activeTasks map[string]*activeTask
	profileTask map[string]string

	// 常驻页面会话用独立的锁：pageMu 下会做进程 spawn 与管道读写，
	// 挂在 m.mu 上会把安装流程和状态查询一起阻塞。
	pageMu       sync.Mutex
	pageSessions map[string]*pageSession
	pageReaperOn bool
}

type activeTask struct {
	taskID    string
	profileID string
	cmd       *exec.Cmd
}

type nodeArchiveSpec struct {
	FileName    string
	StripPrefix string
	Format      string
}

type playwrightMetadata struct {
	TarballURL string
	Shasum     string
}

func NewManager(appRoot string, cfg *config.Config, emit func(string, any), opts Options) *Manager {
	if strings.TrimSpace(opts.NodeDistBaseURL) == "" {
		opts.NodeDistBaseURL = "https://nodejs.org/dist"
	}
	if strings.TrimSpace(opts.NPMRegistryBaseURL) == "" {
		opts.NPMRegistryBaseURL = "https://registry.npmjs.org"
	}
	if strings.TrimSpace(opts.TargetOS) == "" {
		opts.TargetOS = goruntime.GOOS
	}
	if strings.TrimSpace(opts.TargetArch) == "" {
		opts.TargetArch = goruntime.GOARCH
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		}
	}

	return &Manager{
		appRoot:      strings.TrimSpace(appRoot),
		config:       cfg,
		emit:         emit,
		options:      opts,
		activeTasks:  make(map[string]*activeTask),
		profileTask:  make(map[string]string),
		pageSessions: make(map[string]*pageSession),
	}
}

func (m *Manager) SetConfig(cfg *config.Config) {
	m.mu.Lock()
	m.config = cfg
	m.mu.Unlock()
}

func (m *Manager) CurrentState() RuntimeState {
	m.mu.RLock()
	cfg := m.config
	installing := m.installing
	lastError := m.lastError
	m.mu.RUnlock()

	auto := config.DefaultConfig().Automation
	if cfg != nil {
		auto = cfg.Automation
	}

	runtimeDir := m.runtimeDir(auto.RuntimeVersion)
	runnerPath := m.runnerScriptPath(runtimeDir)
	playwrightPkgPath := filepath.Join(runtimeDir, "node_modules", "playwright-core", "package.json")
	resolvedNode := m.resolveNodeRuntime(runtimeDir, auto)
	nodePath := strings.TrimSpace(resolvedNode.Path)
	installed := fileExists(nodePath) && fileExists(playwrightPkgPath) && fileExists(runnerPath)

	nodeVersion := strings.TrimSpace(auto.NodeVersion)
	if resolvedNode.Version != "" {
		nodeVersion = resolvedNode.Version
	}
	playwrightVersion := strings.TrimSpace(auto.PlaywrightCoreVersion)
	if installed {
		if detected := readPackageVersion(playwrightPkgPath); detected != "" {
			playwrightVersion = detected
		}
	}

	return RuntimeState{
		Enabled:              auto.Enabled,
		InstallPolicy:        auto.InstallPolicy,
		RuntimeVersion:       auto.RuntimeVersion,
		HeadlessDefault:      auto.HeadlessDefault,
		KeepRuntimeOnDisable: auto.KeepRuntimeOnDisable,
		NodeSource:           resolvedNode.Source,
		NodeResolution:       resolvedNode.Resolution,
		SystemNodeDetected:   resolvedNode.SystemNodeDetected,
		SystemNodePath:       resolvedNode.SystemNodePath,
		SystemNodeError:      resolvedNode.SystemNodeError,
		Installed:            installed,
		Ready:                installed,
		Installing:           installing,
		LastError:            lastError,
		RuntimeDir:           runtimeDir,
		NodePath:             nodePath,
		RunnerPath:           runnerPath,
		NodeVersion:          nodeVersion,
		PlaywrightVersion:    playwrightVersion,
	}
}

func (m *Manager) currentAutomationConfig() config.AutomationConfig {
	m.mu.RLock()
	cfg := m.config
	m.mu.RUnlock()
	if cfg == nil {
		return config.DefaultConfig().Automation
	}
	return cfg.Automation
}

func (m *Manager) runtimeRoot() string {
	return apppath.Resolve(m.appRoot, filepath.ToSlash(filepath.Join("data", "runtime", "automation")))
}

func (m *Manager) runtimeDir(runtimeVersion string) string {
	return filepath.Join(m.runtimeRoot(), strings.TrimSpace(runtimeVersion))
}

func (m *Manager) nodeExecutablePath(runtimeDir string) string {
	if strings.EqualFold(strings.TrimSpace(m.options.TargetOS), "windows") {
		return filepath.Join(runtimeDir, "node", "node.exe")
	}
	return filepath.Join(runtimeDir, "node", "bin", "node")
}

func (m *Manager) runnerScriptPath(runtimeDir string) string {
	return filepath.Join(runtimeDir, runnerScriptFileName)
}

func (m *Manager) nodeArchive(version string) (nodeArchiveSpec, error) {
	goos := strings.ToLower(strings.TrimSpace(m.options.TargetOS))
	goarch := strings.ToLower(strings.TrimSpace(m.options.TargetArch))

	switch goos {
	case "windows":
		switch goarch {
		case "amd64":
			name := fmt.Sprintf("node-v%s-win-x64.zip", version)
			return nodeArchiveSpec{FileName: name, StripPrefix: strings.TrimSuffix(name, ".zip") + "/", Format: "zip"}, nil
		case "arm64":
			name := fmt.Sprintf("node-v%s-win-arm64.zip", version)
			return nodeArchiveSpec{FileName: name, StripPrefix: strings.TrimSuffix(name, ".zip") + "/", Format: "zip"}, nil
		}
	case "linux":
		switch goarch {
		case "amd64":
			name := fmt.Sprintf("node-v%s-linux-x64.tar.xz", version)
			return nodeArchiveSpec{FileName: name, StripPrefix: strings.TrimSuffix(name, ".tar.xz") + "/", Format: "tar.xz"}, nil
		case "arm64":
			name := fmt.Sprintf("node-v%s-linux-arm64.tar.xz", version)
			return nodeArchiveSpec{FileName: name, StripPrefix: strings.TrimSuffix(name, ".tar.xz") + "/", Format: "tar.xz"}, nil
		}
	case "darwin":
		switch goarch {
		case "amd64":
			name := fmt.Sprintf("node-v%s-darwin-x64.tar.gz", version)
			return nodeArchiveSpec{FileName: name, StripPrefix: strings.TrimSuffix(name, ".tar.gz") + "/", Format: "tar.gz"}, nil
		case "arm64":
			name := fmt.Sprintf("node-v%s-darwin-arm64.tar.gz", version)
			return nodeArchiveSpec{FileName: name, StripPrefix: strings.TrimSuffix(name, ".tar.gz") + "/", Format: "tar.gz"}, nil
		}
	}

	return nodeArchiveSpec{}, fmt.Errorf("当前平台暂不支持自动化运行时下载：%s/%s", goos, goarch)
}
