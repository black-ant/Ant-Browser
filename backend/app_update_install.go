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
	updateProgressEvent = "app:update:progress"
	updatePendingEvent  = "app:update:pending"
	updatePendingFile   = "update-pending.json"
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

func (a *App) updateAsset(version string, assetKey string, proxyConfig string) (AppUpdateAsset, string, error) {
	if a == nil || a.config == nil || !a.config.Update.IsEnabled() {
		return AppUpdateAsset{}, "", fmt.Errorf("更新检查已关闭")
	}
	version = strings.TrimSpace(version)
	assetKey = strings.TrimSpace(assetKey)
	if assetKey == "" {
		assetKey = updateAssetKey()
	}
	info := fetchAppUpdateInfo(context.Background(), a.appVersion(), a.updateReleaseAPIURL(), a.updateHTTPClientFor(proxyConfig))
	if !info.OK {
		return AppUpdateAsset{}, "", fmt.Errorf("获取更新信息失败: %s", info.Error)
	}
	if version == "" {
		version = info.LatestVersion
	}
	if !info.UpdateAvailable && version == info.CurrentVersion {
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
	return a.downloadAppUpdate(version, assetKey, a.updateProxyConfig())
}

// DownloadAppUpdateWithProxy 允许前端临时指定代理下载更新包，不写入配置。
func (a *App) DownloadAppUpdateWithProxy(version string, assetKey string, proxyConfig string) error {
	return a.downloadAppUpdate(version, assetKey, proxyConfig)
}

func (a *App) downloadAppUpdate(version string, assetKey string, proxyConfig string) error {
	asset, version, err := a.updateAsset(version, assetKey, proxyConfig)
	if err != nil {
		return err
	}
	return a.startBackgroundTask(func(ctx context.Context) {
		log := logger.New("AppUpdate")
		emit := func(phase string, progress int, message string) {
			a.emitUpdateProgress(ctx, phase, progress, message)
		}
		emit("resolving", 5, "准备下载更新包")
		log.Info("开始下载应用更新", logger.F("version", version), logger.F("asset", asset.Name))

		targetDir := a.updateDataPath("updates", version)
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			emit("error", 0, "创建更新目录失败: "+err.Error())
			return
		}
		targetPath := filepath.Join(targetDir, asset.Name)
		tempPath := targetPath + ".part"
		out, err := os.Create(tempPath)
		if err != nil {
			emit("error", 0, "创建临时文件失败: "+err.Error())
			return
		}
		defer out.Close()

		client := a.updateHTTPClientFor(proxyConfig)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
		if err != nil {
			emit("error", 0, "创建下载请求失败: "+err.Error())
			return
		}
		request.Header.Set("User-Agent", "Ant-Browser-Updater/"+a.appVersion())
		response, err := client.Do(request)
		if err != nil {
			emit("error", 0, "下载更新包失败: "+err.Error())
			return
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			emit("error", 0, fmt.Sprintf("下载更新包失败: HTTP %d", response.StatusCode))
			return
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
					emit("error", 0, "写入更新包失败: "+writeErr.Error())
					return
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
					emit("error", 0, "下载更新包失败: "+readErr.Error())
					return
				}
				break
			}
		}
		if err := out.Sync(); err != nil {
			emit("error", 0, "写入更新包失败: "+err.Error())
			return
		}
		if err := out.Close(); err != nil {
			emit("error", 0, "关闭更新包失败: "+err.Error())
			return
		}

		emit("verifying", 80, "校验更新包")
		if a.config.Update.VerifySHA256() && strings.TrimSpace(asset.SHA256) != "" {
			sum, hashErr := sha256File(tempPath)
			if hashErr != nil {
				_ = os.Remove(tempPath)
				emit("error", 0, "校验更新包失败: "+hashErr.Error())
				return
			}
			if !strings.EqualFold(sum, strings.TrimSpace(asset.SHA256)) {
				_ = os.Remove(tempPath)
				emit("error", 0, "校验更新包失败：SHA256 不匹配")
				return
			}
		}
		// Windows 上目标文件已存在时 Rename 会失败，先清掉旧包
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			emit("error", 0, "清理旧更新包失败: "+err.Error())
			return
		}
		if err := os.Rename(tempPath, targetPath); err != nil {
			emit("error", 0, "保存更新包失败: "+err.Error())
			return
		}
		log.Info("应用更新包已下载", logger.F("version", version), logger.F("path", targetPath))
		emit("done", 100, "更新包已就绪，可以退出并安装")
	})
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
	if _, err := os.Stat(pending.InstallerPath); err != nil {
		return nil
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
			info := fetchAppUpdateInfo(ctx, a.appVersion(), a.updateReleaseAPIURL(), a.updateHTTPClient())
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
	asset, version, err := a.updateAsset(version, assetKey, a.updateProxyConfig())
	if err != nil {
		return err
	}
	installerPath := a.updateDataPath("updates", version, asset.Name)
	if _, err := os.Stat(installerPath); err != nil {
		return fmt.Errorf("更新包不存在，请先下载: %s", installerPath)
	}

	pending := UpdatePendingInfo{
		Version:       version,
		InstallerPath: installerPath,
		PID:           os.Getpid(),
		CreatedAt:     time.Now().Format(time.RFC3339),
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
