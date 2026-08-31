package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/logger"
)

type ProxyCoreDownloadRequest struct {
	Core        string `json:"core"`
	GOOS        string `json:"goos"`
	GOARCH      string `json:"goarch"`
	ProxyConfig string `json:"proxyConfig"`
	Version     string `json:"version"`
}

type ProxyCoreDownloadProgress struct {
	Core     string `json:"core"`
	GOOS     string `json:"goos"`
	GOARCH   string `json:"goarch"`
	Phase    string `json:"phase"`
	Progress int    `json:"progress"`
	Message  string `json:"message"`
}

type ProxyCoreStatusResult struct {
	Core       string `json:"core"`
	GOOS       string `json:"goos"`
	GOARCH     string `json:"goarch"`
	Installed  bool   `json:"installed"`
	Configured bool   `json:"configured"`
	Active     bool   `json:"active"`
	BinaryPath string `json:"binaryPath"`
	Source     string `json:"source"`
	Message    string `json:"message"`
}

type ProxyCoreDownloadInfoResult struct {
	Core        string `json:"core"`
	GOOS        string `json:"goos"`
	GOARCH      string `json:"goarch"`
	Version     string `json:"version"`
	Repo        string `json:"repo"`
	ReleaseURL  string `json:"releaseUrl"`
	DownloadURL string `json:"downloadUrl"`
	AssetName   string `json:"assetName"`
	InstallDir  string `json:"installDir"`
	BinaryName  string `json:"binaryName"`
	Message     string `json:"message"`
}

type proxyCoreSpec struct {
	Core        string
	Repo        string
	DisplayName string
	BinaryBase  string
	ConfigKey   string
	Version     string
}

func (a *App) BrowserProxyCoreDownload(input ProxyCoreDownloadRequest) error {
	if a.ctx == nil {
		return fmt.Errorf("app context is nil")
	}
	spec, err := normalizeProxyCoreSpec(input.Core)
	if err != nil {
		return err
	}
	target, err := normalizeProxyCoreTarget(input.GOOS, input.GOARCH)
	if err != nil {
		return err
	}
	version := normalizeProxyCoreVersion(input.Version, spec.Version)
	go a.downloadProxyCore(a.ctx, spec, target, input.ProxyConfig, version)
	return nil
}

func (a *App) BrowserProxyCoreStatus(input ProxyCoreDownloadRequest) ProxyCoreStatusResult {
	spec, err := normalizeProxyCoreSpec(input.Core)
	if err != nil {
		return ProxyCoreStatusResult{Core: strings.TrimSpace(input.Core), Message: err.Error()}
	}
	target, err := normalizeProxyCoreTarget(input.GOOS, input.GOARCH)
	if err != nil {
		return ProxyCoreStatusResult{Core: spec.Core, Message: err.Error()}
	}
	return a.proxyCoreStatus(spec, target)
}

func (a *App) BrowserProxyCoreDownloadInfo(input ProxyCoreDownloadRequest) ProxyCoreDownloadInfoResult {
	spec, err := normalizeProxyCoreSpec(input.Core)
	if err != nil {
		return ProxyCoreDownloadInfoResult{Core: strings.TrimSpace(input.Core), Message: err.Error()}
	}
	target, err := normalizeProxyCoreTarget(input.GOOS, input.GOARCH)
	if err != nil {
		return ProxyCoreDownloadInfoResult{Core: spec.Core, Message: err.Error()}
	}
	version := normalizeProxyCoreVersion(input.Version, spec.Version)
	info := proxyCoreDownloadInfoBase(a, spec, target)
	info.Version = version
	info.ReleaseURL = proxyCoreReleaseURL(spec.Repo, version)
	client, _, err := a.unifiedProxyCoreHTTPClient(30*time.Second, input.ProxyConfig)
	if err != nil {
		info.Message = manualProxyCoreDownloadMessage(spec, target, "下载代理配置错误: "+err.Error())
		return info
	}
	release, err := fetchGitHubRelease(context.Background(), client, spec.Repo, version)
	if err != nil {
		info.Message = manualProxyCoreDownloadMessage(spec, target, "自动查询 Release 失败: "+err.Error())
		return info
	}
	asset, err := selectProxyCoreAsset(spec, release.Assets, target.GOOS, target.GOARCH)
	if err != nil {
		info.Message = manualProxyCoreDownloadMessage(spec, target, err.Error())
		return info
	}
	info.AssetName = asset.Name
	info.DownloadURL = asset.BrowserDownloadURL
	info.Message = "可打开远程地址手动下载，下载后解压到本地目录"
	return info
}

func (a *App) BrowserProxyCoreOpenLocal(input ProxyCoreDownloadRequest) error {
	spec, err := normalizeProxyCoreSpec(input.Core)
	if err != nil {
		return err
	}
	target, err := normalizeProxyCoreTarget(input.GOOS, input.GOARCH)
	if err != nil {
		return err
	}
	status := a.proxyCoreStatus(spec, target)
	path := strings.TrimSpace(status.BinaryPath)
	if path == "" {
		path = proxyCoreInstallDir(a, spec, target)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("创建本地目录失败: %w", err)
		}
	}
	if err := openPathInFileManager(path); err != nil {
		return fmt.Errorf("打开本地路径失败: %w", err)
	}
	return nil
}

type proxyCoreTarget struct {
	GOOS   string
	GOARCH string
}

func normalizeProxyCoreTarget(goos string, goarch string) (proxyCoreTarget, error) {
	goos = strings.ToLower(strings.TrimSpace(goos))
	goarch = strings.ToLower(strings.TrimSpace(goarch))
	if goos == "" {
		goos = goruntime.GOOS
	}
	if goarch == "" {
		goarch = goruntime.GOARCH
	}
	switch goos {
	case "win", "windows":
		goos = "windows"
	case "linux":
		goos = "linux"
	case "mac", "macos", "darwin":
		goos = "darwin"
	default:
		return proxyCoreTarget{}, fmt.Errorf("不支持的目标系统: %s", goos)
	}
	switch goarch {
	case "x64", "x86_64", "amd64":
		goarch = "amd64"
	case "aarch64", "arm64":
		goarch = "arm64"
	case "x86", "i386", "386":
		goarch = "386"
	default:
		return proxyCoreTarget{}, fmt.Errorf("不支持的目标架构: %s", goarch)
	}
	return proxyCoreTarget{GOOS: goos, GOARCH: goarch}, nil
}

func normalizeProxyCoreSpec(core string) (proxyCoreSpec, error) {
	switch strings.ToLower(strings.TrimSpace(core)) {
	case "", "xray":
		return proxyCoreSpec{Core: "xray", Repo: "XTLS/Xray-core", DisplayName: "Xray", BinaryBase: "xray", ConfigKey: "xray", Version: "v26.3.27"}, nil
	case "mihomo", "clash", "clash-meta":
		return proxyCoreSpec{Core: "mihomo", Repo: "MetaCubeX/mihomo", DisplayName: "Mihomo", BinaryBase: "mihomo", ConfigKey: "clash", Version: "v1.19.27"}, nil
	case "sing-box", "singbox":
		return proxyCoreSpec{Core: "sing-box", Repo: "SagerNet/sing-box", DisplayName: "sing-box", BinaryBase: "sing-box", ConfigKey: "sing-box", Version: "v1.13.13"}, nil
	default:
		return proxyCoreSpec{}, fmt.Errorf("不支持的代理内核: %s", core)
	}
}

func (a *App) downloadProxyCore(ctx context.Context, spec proxyCoreSpec, target proxyCoreTarget, proxyConfig string, version string) {
	log := logger.New("ProxyCore")
	send := func(phase string, progress int, message string) {
		a.emitRuntimeEvent("proxy-core:download:progress", ProxyCoreDownloadProgress{Core: spec.Core, GOOS: target.GOOS, GOARCH: target.GOARCH, Phase: phase, Progress: progress, Message: message})
	}
	client, proxyLabel, err := a.unifiedProxyCoreHTTPClient(90*time.Second, proxyConfig)
	if err != nil {
		send("error", 0, "下载代理配置错误: "+err.Error())
		return
	}
	send("resolving", 0, fmt.Sprintf("正在查询官方 Release %s（%s）", version, proxyLabel))

	release, err := fetchGitHubRelease(ctx, client, spec.Repo, version)
	if err != nil {
		send("error", 0, "查询 Release 失败: "+err.Error())
		return
	}
	asset, err := selectProxyCoreAsset(spec, release.Assets, target.GOOS, target.GOARCH)
	if err != nil {
		send("error", 0, err.Error())
		return
	}

	platformDir := fmt.Sprintf("%s-%s", target.GOOS, target.GOARCH)
	installDir := proxyCoreInstallDir(a, spec, target)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		send("error", 0, "创建安装目录失败: "+err.Error())
		return
	}

	tmp, err := os.CreateTemp(installDir, "proxy-core-*"+archiveExt(asset.Name))
	if err != nil {
		send("error", 0, "创建临时文件失败: "+err.Error())
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	send("downloading", 5, fmt.Sprintf("开始下载 %s %s（%s）", spec.DisplayName, release.TagName, proxyLabel))
	if err := downloadProxyCoreAsset(ctx, client, asset.BrowserDownloadURL, tmp, asset.Size, send); err != nil {
		_ = tmp.Close()
		send("error", 0, "下载失败: "+err.Error())
		return
	}
	_ = tmp.Close()

	extractDir, err := os.MkdirTemp(installDir, "extract-*")
	if err != nil {
		send("error", 0, "创建解压目录失败: "+err.Error())
		return
	}
	defer os.RemoveAll(extractDir)

	send("extracting", 80, "下载完成，正在解压")
	if err := extractProxyCoreArchive(tmpPath, extractDir, spec.BinaryBase, target.GOOS); err != nil {
		send("error", 0, "解压失败: "+err.Error())
		return
	}
	binaryPath, err := findProxyCoreBinary(extractDir, spec.BinaryBase, target.GOOS)
	if err != nil {
		send("error", 0, err.Error())
		return
	}

	if err := replaceDirContents(extractDir, installDir); err != nil {
		send("error", 0, "安装失败: "+err.Error())
		return
	}
	installedBinary := filepath.Join(installDir, mustRelPath(extractDir, binaryPath))
	installedBinary, err = normalizeInstalledProxyCoreBinary(installedBinary, installDir, spec.BinaryBase, target.GOOS)
	if err != nil {
		send("error", 0, "规范内核文件名失败: "+err.Error())
		return
	}
	if target.GOOS == goruntime.GOOS && target.GOARCH == goruntime.GOARCH {
		if err := fsutil.EnsureExecutable(installedBinary); err != nil {
			send("error", 0, "设置可执行权限失败: "+err.Error())
			return
		}
		if err := a.saveProxyCoreBinaryPath(spec, installedBinary); err != nil {
			send("error", 0, "保存配置失败: "+err.Error())
			return
		}
		send("done", 100, fmt.Sprintf("%s 已安装并启用: %s", spec.DisplayName, installedBinary))
	} else {
		send("done", 100, fmt.Sprintf("%s 已下载到 %s/%s: %s", spec.DisplayName, target.GOOS, target.GOARCH, installedBinary))
	}

	log.Info("代理内核安装完成", logger.F("core", spec.Core), logger.F("target", platformDir), logger.F("version", release.TagName), logger.F("binary", installedBinary))
}

func proxyCoreDownloadInfoBase(a *App, spec proxyCoreSpec, target proxyCoreTarget) ProxyCoreDownloadInfoResult {
	return ProxyCoreDownloadInfoResult{
		Core:       spec.Core,
		GOOS:       target.GOOS,
		GOARCH:     target.GOARCH,
		Version:    spec.Version,
		Repo:       spec.Repo,
		ReleaseURL: proxyCoreReleaseURL(spec.Repo, spec.Version),
		InstallDir: proxyCoreInstallDir(a, spec, target),
		BinaryName: proxyCoreBinaryName(spec.BinaryBase, target.GOOS),
	}
}

func normalizeProxyCoreVersion(version string, fallback string) string {
	version = strings.TrimSpace(version)
	if version == "" || strings.EqualFold(version, "stable") {
		version = strings.TrimSpace(fallback)
	}
	if version == "" || strings.EqualFold(version, "latest") {
		return "latest"
	}
	if !strings.HasPrefix(strings.ToLower(version), "v") {
		version = "v" + version
	}
	return version
}

func proxyCoreReleaseURL(repo string, version string) string {
	if strings.EqualFold(strings.TrimSpace(version), "latest") {
		return "https://github.com/" + repo + "/releases/latest"
	}
	return "https://github.com/" + repo + "/releases/tag/" + strings.TrimSpace(version)
}

func proxyCoreInstallDir(a *App, spec proxyCoreSpec, target proxyCoreTarget) string {
	appRoot := ""
	if a != nil {
		appRoot = a.appRoot
	}
	return apppath.Resolve(appRoot, filepath.Join("bin", fmt.Sprintf("%s-%s", target.GOOS, target.GOARCH), spec.Core))
}

func manualProxyCoreDownloadMessage(spec proxyCoreSpec, target proxyCoreTarget, reason string) string {
	return fmt.Sprintf("%s。请打开 Release 页面，下载 %s/%s 的 %s，解压后把 %s 放到本地目录。需要代理时，请在下载代理里填写 http://、https:// 或 socks5:// 地址。", reason, target.GOOS, target.GOARCH, spec.DisplayName, proxyCoreBinaryName(spec.BinaryBase, target.GOOS))
}
