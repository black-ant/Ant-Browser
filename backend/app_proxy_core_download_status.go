package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/fsutil"
)

func (a *App) proxyCoreStatus(spec proxyCoreSpec, target proxyCoreTarget) ProxyCoreStatusResult {
	result := ProxyCoreStatusResult{Core: spec.Core, GOOS: target.GOOS, GOARCH: target.GOARCH}
	if a == nil || a.config == nil {
		result.Message = "配置未初始化"
		return result
	}
	result.Active = proxyCoreIsActive(a, spec)
	configuredPath := strings.TrimSpace(proxyCoreConfiguredPath(a, spec))
	if configuredPath != "" && target.GOOS == goruntime.GOOS && target.GOARCH == goruntime.GOARCH {
		if path, ok := existingProxyCoreFile(configuredPath, a.appRoot); ok {
			result.Installed = true
			result.Configured = true
			result.BinaryPath = path
			result.Source = "config"
			result.Message = proxyCoreInstalledMessage(result.Active, true)
			return result
		}
	}
	if path, source, ok := findInstalledProxyCoreBinary(a.appRoot, spec, target); ok {
		result.Installed = true
		result.BinaryPath = path
		result.Source = source
		if target.GOOS == goruntime.GOOS && target.GOARCH == goruntime.GOARCH && configuredPath != "" {
			if configured, ok := existingProxyCoreFile(configuredPath, a.appRoot); ok && sameCleanPath(configured, path) {
				result.Configured = true
				result.Source = "config"
			}
		}
		result.Message = proxyCoreInstalledMessage(result.Active, result.Configured)
		return result
	}
	if result.Active {
		result.Message = "当前内核未找到"
		return result
	}
	result.Message = "未下载"
	return result
}

func proxyCoreIsActive(a *App, spec proxyCoreSpec) bool {
	if a == nil || a.config == nil {
		return false
	}
	current := strings.ToLower(strings.TrimSpace(a.config.Browser.DefaultConnectorType))
	if current == "" {
		current = "xray"
	}
	switch spec.Core {
	case "xray":
		return current == "xray"
	case "mihomo":
		return current == "mihomo" || current == "clash"
	case "sing-box":
		return current == "sing-box" || current == "singbox"
	case "mieru":
		// mieru 是独立的桥接内核，不隶属任何连接栈，按需启用。
		return true
	default:
		return false
	}
}

func proxyCoreInstalledMessage(active bool, configured bool) string {
	if active {
		return "已启用"
	}
	if configured {
		return "已配置"
	}
	return "已下载"
}

func findInstalledProxyCoreBinary(appRoot string, spec proxyCoreSpec, target proxyCoreTarget) (string, string, bool) {
	platformDir := fmt.Sprintf("%s-%s", target.GOOS, target.GOARCH)
	searchDirs := []struct {
		path   string
		source string
	}{
		{apppath.Resolve(appRoot, filepath.Join("bin", platformDir, spec.Core)), "downloaded"},
		{apppath.Resolve(appRoot, filepath.Join("bin", platformDir)), "runtime"},
		{apppath.Resolve(appRoot, "bin"), "runtime"},
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		searchDirs = append(searchDirs,
			struct {
				path   string
				source string
			}{filepath.Join(exeDir, "bin", platformDir, spec.Core), "downloaded"},
			struct {
				path   string
				source string
			}{filepath.Join(exeDir, "bin", platformDir), "runtime"},
			struct {
				path   string
				source string
			}{filepath.Join(exeDir, "bin"), "runtime"},
		)
	}
	for _, dir := range searchDirs {
		if strings.TrimSpace(dir.path) == "" {
			continue
		}
		if path, err := findProxyCoreBinary(dir.path, spec.BinaryBase, target.GOOS); err == nil {
			return path, dir.source, true
		}
	}
	if target.GOOS == goruntime.GOOS && target.GOARCH == goruntime.GOARCH {
		if path, err := exec.LookPath(proxyCoreBinaryName(spec.BinaryBase, target.GOOS)); err == nil {
			return path, "path", true
		}
	}
	return "", "", false
}

func proxyCoreConfiguredPath(a *App, spec proxyCoreSpec) string {
	if a == nil || a.config == nil {
		return ""
	}
	switch spec.ConfigKey {
	case "xray":
		return a.config.Browser.XrayBinaryPath
	case "clash":
		return a.config.Browser.ClashBinaryPath
	case "sing-box":
		return a.config.Browser.SingBoxBinaryPath
	case "mieru":
		return a.config.Browser.MieruBinaryPath
	default:
		return ""
	}
}

func existingProxyCoreFile(path string, appRoot string) (string, bool) {
	path = fsutil.NormalizePathInput(path)
	if path == "" {
		return "", false
	}
	if !filepath.IsAbs(path) && strings.TrimSpace(appRoot) != "" {
		path = apppath.Resolve(appRoot, path)
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path, true
	}
	return "", false
}

func sameCleanPath(a string, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func (a *App) saveProxyCoreBinaryPath(spec proxyCoreSpec, binaryPath string) error {
	if a.config == nil {
		return fmt.Errorf("config is nil")
	}
	clean := fsutil.NormalizePathInput(binaryPath)
	switch spec.ConfigKey {
	case "xray":
		a.config.Browser.XrayBinaryPath = clean
	case "clash":
		a.config.Browser.ClashBinaryPath = clean
	case "sing-box":
		a.config.Browser.SingBoxBinaryPath = clean
	case "mieru":
		a.config.Browser.MieruBinaryPath = clean
	default:
		return fmt.Errorf("未知配置键: %s", spec.ConfigKey)
	}
	if a.xrayMgr != nil {
		a.xrayMgr.Config = a.config
	}
	if a.clashMgr != nil {
		a.clashMgr.Config = a.config
	}
	if a.singboxMgr != nil {
		a.singboxMgr.Config = a.config
	}
	if a.mieruMgr != nil {
		a.mieruMgr.Config = a.config
	}
	return a.config.Save(a.resolveAppPath("config.yaml"))
}
