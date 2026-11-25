package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
)

const (
	updateProgressEvent  = "app:update:progress"
	updatePendingEvent   = "app:update:pending"
	updatePendingFile    = "update-pending.json"
	downloadedUpdateFile = "downloaded-update.json"
)

// UpdateProgress 是更新下载/安装进度事件的数据。
type UpdateProgress struct {
	Phase    string `json:"phase"`
	Progress int    `json:"progress"`
	Message  string `json:"message"`
}

// UpdatePendingInfo 记录“已下载、等待退出后安装”的状态。
type UpdatePendingInfo struct {
	Version       string `json:"version"`
	InstallerPath string `json:"installerPath"`
	PID           int    `json:"pid"`
	CreatedAt     string `json:"createdAt"`
	// Missing 表示安装包已不存在（被删除或移动），前端据此显示明确失败提示，而不是静默隐藏。
	Missing bool `json:"missing"`
}

type downloadedUpdateInfo struct {
	Version   string `json:"version"`
	AssetName string `json:"assetName"`
}

// AppDownloadedUpdate 描述已下载、尚未安装的更新包，供重启后继续安装。
type AppDownloadedUpdate struct {
	Version       string `json:"version"`
	AssetName     string `json:"assetName"`
	InstallerPath string `json:"installerPath"`
}

func (a *App) downloadedUpdatePath() string {
	return a.updateDataPath("updates", downloadedUpdateFile)
}

func (a *App) saveDownloadedUpdate(info downloadedUpdateInfo) error {
	path := a.downloadedUpdatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (a *App) loadDownloadedUpdate() *downloadedUpdateInfo {
	data, err := os.ReadFile(a.downloadedUpdatePath())
	if err != nil {
		return nil
	}
	var info downloadedUpdateInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil
	}
	if strings.TrimSpace(info.Version) == "" || strings.TrimSpace(info.AssetName) == "" {
		return nil
	}
	return &info
}

// GetDownloadedAppUpdate 返回本地已下载、待安装的更新包，供重启后继续安装。
// 更新功能已关闭、版本已安装或安装包已丢失时返回 nil：这几种情况都无需再提示安装。
func (a *App) GetDownloadedAppUpdate() *AppDownloadedUpdate {
	if a == nil || a.config == nil || !a.config.Update.IsEnabled() {
		return nil
	}
	downloaded := a.loadDownloadedUpdate()
	if downloaded == nil {
		return nil
	}
	if comparison, err := compareUpdateVersions(a.appVersion(), downloaded.Version); err != nil || comparison >= 0 {
		return nil
	}
	installerPath := a.updateDataPath("updates", downloaded.Version, downloaded.AssetName)
	if _, err := os.Stat(installerPath); err != nil {
		return nil
	}
	return &AppDownloadedUpdate{
		Version:       downloaded.Version,
		AssetName:     downloaded.AssetName,
		InstallerPath: installerPath,
	}
}

// OpenUpdatePackageLocation 在系统文件管理器中定位已下载的更新包，方便用户直接运行。
func (a *App) OpenUpdatePackageLocation() error {
	downloaded := a.loadDownloadedUpdate()
	if downloaded == nil {
		return fmt.Errorf("没有已下载的更新包")
	}
	installerPath := a.updateDataPath("updates", downloaded.Version, downloaded.AssetName)
	if _, err := os.Stat(installerPath); err != nil {
		return fmt.Errorf("更新包不存在，请重新下载")
	}
	return revealFileInFileManager(installerPath)
}

// ClearDownloadedAppUpdate 丢弃已下载待安装的更新包，同时清掉安装文件避免占用磁盘。
func (a *App) ClearDownloadedAppUpdate() error {
	downloaded := a.loadDownloadedUpdate()
	if err := os.Remove(a.downloadedUpdatePath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	if downloaded == nil {
		return nil
	}
	installerPath := a.updateDataPath("updates", downloaded.Version, downloaded.AssetName)
	_ = os.Remove(installerPath)
	_ = os.Remove(filepath.Dir(installerPath))
	return nil
}

func updateAssetKey() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

func (a *App) updateDataPath(parts ...string) string {
	root := "data"
	if a != nil && a.config != nil && strings.TrimSpace(a.config.Browser.UserDataRoot) != "" {
		root = strings.TrimSpace(a.config.Browser.UserDataRoot)
	}
	segments := append([]string{root}, parts...)
	return a.resolveAppPath(filepath.Join(segments...))
}

func (a *App) emitUpdateProgress(ctx context.Context, phase string, progress int, message string) {
	payload := UpdateProgress{Phase: phase, Progress: progress, Message: message}
	a.emitRuntimeEventWithContext(ctx, updateProgressEvent, payload)
}

func (a *App) updateAsset(version string, assetKey string, proxyConfig string, proxyID string, channel string) (AppUpdateAsset, string, error) {
	if a == nil || a.config == nil || !a.config.Update.IsEnabled() {
		return AppUpdateAsset{}, "", fmt.Errorf("更新检查已关闭")
	}
	version = strings.TrimSpace(version)
	assetKey = strings.TrimSpace(assetKey)
	channel = normalizeUpdateChannel(channel)
	if assetKey == "" {
		assetKey = updateAssetKey()
	}
	client, err := a.updateHTTPClientForSelection(updateResolveTimeout, proxyConfig, proxyID)
	if err != nil {
		logger.New("AppUpdate").Warn("下载更新时构建代理客户端失败",
			logger.F("proxy_id", proxyID),
			logger.F("error", err.Error()),
		)
		return AppUpdateAsset{}, "", fmt.Errorf("更新代理配置错误: %w", err)
	}
	info := fetchAppUpdateInfoForChannel(
		context.Background(),
		a.appVersion(),
		channel,
		version,
		a.updateReleasesListURL(),
		client,
	)
	if !info.OK {
		logger.New("AppUpdate").Warn("下载更新时获取更新信息失败",
			logger.F("proxy_id", proxyID),
			logger.F("channel", channel),
			logger.F("error", info.Error),
		)
		return AppUpdateAsset{}, "", fmt.Errorf("获取更新信息失败: %s", info.Error)
	}
	if version == "" {
		if !info.UpdateAvailable {
			return AppUpdateAsset{}, "", fmt.Errorf("当前已是最新版本")
		}
		version = info.LatestVersion
	} else if comparison, compareErr := compareUpdateVersions(info.CurrentVersion, version); compareErr != nil {
		return AppUpdateAsset{}, "", fmt.Errorf("版本号无效: %s", version)
	} else if comparison >= 0 {
		// 预览通道的最新预发布版本可能低于当前正式版，这里必须拦住，避免“更新”成更低版本。
		return AppUpdateAsset{}, "", fmt.Errorf("当前已是最新版本")
	}
	asset, ok := info.Assets[assetKey]
	if !ok || strings.TrimSpace(asset.URL) == "" {
		return AppUpdateAsset{}, "", fmt.Errorf("没有适用于 %s 的更新包，请前往发布页手动下载", assetKey)
	}
	return asset, version, nil
}

// DownloadAppUpdate 使用配置里的代理后台下载更新包。
func (a *App) DownloadAppUpdate(version string, assetKey string) error {
	return a.downloadAppUpdate(version, assetKey, a.updateProxyConfig(), "", UpdateChannelStable)
}

// DownloadAppUpdateWithProxy 允许前端临时指定代理下载更新包，不写入配置。
func (a *App) DownloadAppUpdateWithProxy(version string, assetKey string, proxyConfig string) error {
	return a.downloadAppUpdate(version, assetKey, proxyConfig, "", UpdateChannelStable)
}

// DownloadAppUpdateWithProxySelection 允许前端指定代理地址或代理池节点 ID 下载更新包。
func (a *App) DownloadAppUpdateWithProxySelection(version string, assetKey string, proxyConfig string, proxyID string) error {
	return a.downloadAppUpdate(version, assetKey, proxyConfig, proxyID, UpdateChannelStable)
}

// DownloadAppUpdateWithChannel 允许前端同时指定更新通道（正式 / 预览）。
func (a *App) DownloadAppUpdateWithChannel(version string, assetKey string, proxyConfig string, proxyID string, channel string) error {
	return a.downloadAppUpdate(version, assetKey, proxyConfig, proxyID, channel)
}

func (a *App) downloadAppUpdate(version string, assetKey string, proxyConfig string, proxyID string, channel string) error {
	asset, version, err := a.updateAsset(version, assetKey, proxyConfig, proxyID, channel)
	if err != nil {
		return err
	}
	client, err := a.updateDownloadHTTPClientFor(proxyConfig, proxyID)
	if err != nil {
		return fmt.Errorf("更新代理配置错误: %w", err)
	}
	return a.startBackgroundTask(func(ctx context.Context) {
		log := logger.New("AppUpdate")
		emit := func(phase string, progress int, message string) {
			a.emitUpdateProgress(ctx, phase, progress, message)
		}
		log.Info("开始下载应用更新", logger.F("version", version), logger.F("asset", asset.Name))
		if err := a.downloadUpdatePackage(ctx, version, asset, client, emit); err != nil {
			emit("error", 0, err.Error())
			return
		}
		log.Info("应用更新包已下载", logger.F("version", version), logger.F("path", a.updateDataPath("updates", version, asset.Name)))
	})
}

// downloadUpdatePackage 下载安装包并做 SHA256 校验与落盘；返回的错误文本直接展示给用户。
func (a *App) downloadUpdatePackage(ctx context.Context, version string, asset AppUpdateAsset, client *http.Client, emit func(phase string, progress int, message string)) error {
	emit("resolving", 5, "准备下载更新包")

	targetDir := a.updateDataPath("updates", version)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("创建更新目录失败: %s", err.Error())
	}
	targetPath := filepath.Join(targetDir, asset.Name)
	tempPath := targetPath + ".part"
	// 下载或校验中途失败时不留下半成品；成功 rename 后该路径已不存在，删除是空操作。
	defer func() { _ = os.Remove(tempPath) }()
	out, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %s", err.Error())
	}
	defer out.Close()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return fmt.Errorf("创建下载请求失败: %s", err.Error())
	}
	request.Header.Set("User-Agent", "Ant-Browser-Updater/"+a.appVersion())
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("下载更新包失败: %s", err.Error())
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("下载更新包失败: HTTP %d", response.StatusCode)
	}

	total := response.ContentLength
	var written int64
	buffer := make([]byte, 128*1024)
	lastPercent := -1
	for {
		readCount, readErr := response.Body.Read(buffer)
		if readCount > 0 {
			count, writeErr := out.Write(buffer[:readCount])
			written += int64(count)
			if writeErr != nil {
				return fmt.Errorf("写入更新包失败: %s", writeErr.Error())
			}
			if total > 0 {
				percent := 5 + int(written*70/total)
				if percent != lastPercent {
					lastPercent = percent
					emit("downloading", percent, fmt.Sprintf("下载中 %d%%", percent))
				}
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				return fmt.Errorf("下载更新包失败: %s", readErr.Error())
			}
			break
		}
	}
	if err := out.Sync(); err != nil {
		return fmt.Errorf("写入更新包失败: %s", err.Error())
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("关闭更新包失败: %s", err.Error())
	}

	emit("verifying", 80, "校验更新包")
	if a.config.Update.VerifySHA256() && strings.TrimSpace(asset.SHA256) != "" {
		sum, hashErr := sha256File(tempPath)
		if hashErr != nil {
			return fmt.Errorf("校验更新包失败: %s", hashErr.Error())
		}
		if !strings.EqualFold(sum, strings.TrimSpace(asset.SHA256)) {
			return fmt.Errorf("校验更新包失败：SHA256 不匹配")
		}
	}
	// Windows 上目标文件已存在时 Rename 会失败，先清掉旧包
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("清理旧更新包失败: %s", err.Error())
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		return fmt.Errorf("保存更新包失败: %s", err.Error())
	}
	if err := a.saveDownloadedUpdate(downloadedUpdateInfo{Version: version, AssetName: asset.Name}); err != nil {
		return fmt.Errorf("记录更新包信息失败: %s", err.Error())
	}
	emit("done", 100, "更新包已就绪，可以退出并安装")
	return nil
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// GetPendingAppUpdate 返回上次未完成的安装任务，供启动时提示。
// 该标记对应的版本已经装上时视为过期记录直接忽略；安装包丢失时仍返回状态并标记 Missing，
// 让前端给出“安装包已丢失，请重新下载”的明确提示，而不是静默隐藏。
func (a *App) GetPendingAppUpdate() *UpdatePendingInfo {
	path := a.updateDataPath(updatePendingFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pending UpdatePendingInfo
	if err := json.Unmarshal(data, &pending); err != nil || strings.TrimSpace(pending.InstallerPath) == "" {
		return nil
	}
	if comparison, compareErr := compareUpdateVersions(a.appVersion(), pending.Version); compareErr == nil && comparison >= 0 {
		return nil
	}
	if _, err := os.Stat(pending.InstallerPath); err != nil {
		pending.Missing = true
	}
	return &pending
}

// ClearPendingAppUpdate 清除未完成的安装标记。
func (a *App) ClearPendingAppUpdate() error {
	return os.Remove(a.updateDataPath(updatePendingFile))
}

// startupInitAutoUpdateCheck 启动后延迟做首次检查，然后按配置间隔检查并通知前端。
func (a *App) startupInitAutoUpdateCheck() {
	if a == nil || a.config == nil || !a.config.Update.IsEnabled() || !a.config.Update.ShouldAutoCheck() {
		return
	}
	_ = a.startBackgroundTask(func(ctx context.Context) {
		initial := time.NewTimer(2 * time.Minute)
		defer initial.Stop()
		select {
		case <-ctx.Done():
			return
		case <-initial.C:
		}

		interval := time.Duration(a.config.Update.IntervalHours()) * time.Hour
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			var info AppUpdateInfo
			client, err := a.updateHTTPClient()
			if err != nil {
				logger.New("AppUpdate").Warn("自动检查更新代理配置失败", logger.F("error", err.Error()))
			} else {
				info = fetchAppUpdateInfo(ctx, a.appVersion(), a.updateReleaseAPIURL(), client)
			}
			if ctx.Err() != nil {
				return
			}
			if info.OK && info.UpdateAvailable {
				a.emitRuntimeEventWithContext(ctx, "app:update:available", info)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
}

// NotifyPendingAppUpdate 供启动流程调用，把未完成的安装提示发给前端。
func (a *App) NotifyPendingAppUpdate() {
	pending := a.GetPendingAppUpdate()
	if pending == nil {
		return
	}
	a.emitRuntimeEvent(updatePendingEvent, pending)
}

// ApplyAppUpdate 写入安装标记并拉起脱离主进程的等待脚本；调用方随后退出应用。
func (a *App) ApplyAppUpdate(version string, assetKey string) error {
	if a == nil || a.config == nil || !a.config.Update.IsEnabled() {
		return fmt.Errorf("更新检查已关闭")
	}
	downloaded := a.loadDownloadedUpdate()
	if downloaded == nil {
		return fmt.Errorf("没有已下载的更新包，请先下载")
	}
	if requested := strings.TrimSpace(version); requested != "" &&
		normalizeUpdateVersion(requested) != normalizeUpdateVersion(downloaded.Version) {
		return fmt.Errorf("更新包版本与待安装版本不一致")
	}
	installerPath := a.updateDataPath("updates", downloaded.Version, downloaded.AssetName)
	if _, err := os.Stat(installerPath); err != nil {
		return fmt.Errorf("更新包不存在，请先下载: %s", installerPath)
	}

	pending := UpdatePendingInfo{
		Version:       downloaded.Version,
		InstallerPath: installerPath,
		PID:           os.Getpid(),
		CreatedAt:     time.Now().Format(time.RFC3339),
	}
	// 同一进程、同一个安装包已经安排过等待脚本时直接复用，重复拉起会让安装包被执行两次。
	if previous := a.GetPendingAppUpdate(); previous != nil && !previous.Missing &&
		previous.PID == pending.PID && previous.InstallerPath == pending.InstallerPath {
		logger.New("AppUpdate").Info("退出后安装已安排，跳过重复拉起", logger.F("version", pending.Version), logger.F("path", installerPath))
		return nil
	}
	if err := a.savePendingUpdate(pending); err != nil {
		return err
	}
	if err := a.launchUpdateWaiter(pending); err != nil {
		_ = os.Remove(a.updateDataPath(updatePendingFile))
		return err
	}
	logger.New("AppUpdate").Info("已安排退出后安装", logger.F("version", version), logger.F("path", installerPath))
	return nil
}

// ApplyPendingAppUpdate 复用上次已下载的安装包，只读本地标记、不联网。
// 用于应用重启后“重新打开安装”，把等待 PID 更新为当前进程，避免旧 PID 已消失导致提前启动。
func (a *App) ApplyPendingAppUpdate() error {
	pending := a.GetPendingAppUpdate()
	if pending == nil {
		return fmt.Errorf("没有待安装的更新包")
	}
	if _, err := os.Stat(pending.InstallerPath); err != nil {
		return fmt.Errorf("待安装更新包不存在: %w", err)
	}
	// 当前进程已经安排过等待脚本：重复拉起会让同一个安装包被执行两次。
	if pending.PID == os.Getpid() {
		logger.New("AppUpdate").Info("退出后安装已安排，跳过重复拉起", logger.F("version", pending.Version), logger.F("path", pending.InstallerPath))
		return nil
	}

	pending.PID = os.Getpid()
	pending.CreatedAt = time.Now().Format(time.RFC3339)
	if err := a.savePendingUpdate(*pending); err != nil {
		return err
	}
	if err := a.launchUpdateWaiter(*pending); err != nil {
		return err
	}
	logger.New("AppUpdate").Info("已重新安排退出后安装", logger.F("version", pending.Version), logger.F("path", pending.InstallerPath))
	return nil
}

func (a *App) savePendingUpdate(pending UpdatePendingInfo) error {
	path := a.updateDataPath(updatePendingFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(pending, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
