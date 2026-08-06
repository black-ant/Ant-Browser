package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type BrowserExtension = browser.Extension
type BrowserExtensionLookupResult = browser.ExtensionLookupResult
type BrowserProfileExtensionSettings = browser.ProfileExtensionSettings

type BrowserExtensionWebStoreRequest struct {
	Query       string `json:"query"`
	UseProxy    bool   `json:"useProxy"`
	ProxyConfig string `json:"proxyConfig"`
}

type BrowserExtensionManualInstallGuide struct {
	ExtensionID string `json:"extensionId"`
	StoreURL    string `json:"storeUrl"`
	DownloadURL string `json:"downloadUrl"`
	DownloadDir string `json:"downloadDir"`
	FileName    string `json:"fileName"`
}

type BrowserExtensionManualDownloadFile struct {
	FileName  string `json:"fileName"`
	FilePath  string `json:"filePath"`
	SizeBytes int64  `json:"sizeBytes"`
	UpdatedAt string `json:"updatedAt"`
}

func (a *App) BrowserExtensionList() ([]BrowserExtension, error) {
	if a.browserMgr == nil || a.browserMgr.ExtensionDAO == nil {
		return []BrowserExtension{}, nil
	}
	return a.browserMgr.ExtensionDAO.List()
}

func (a *App) BrowserExtensionLookup(query string) (BrowserExtensionLookupResult, error) {
	return a.BrowserExtensionLookupWithProxy(BrowserExtensionWebStoreRequest{Query: query})
}

func (a *App) BrowserExtensionLookupWithProxy(input BrowserExtensionWebStoreRequest) (BrowserExtensionLookupResult, error) {
	if a.browserMgr == nil {
		return BrowserExtensionLookupResult{}, fmt.Errorf("浏览器管理器未初始化")
	}
	client, err := a.extensionDownloadHTTPClient(input.UseProxy, input.ProxyConfig)
	if err != nil {
		return BrowserExtensionLookupResult{}, fmt.Errorf("下载代理配置错误: %w", err)
	}
	return a.browserMgr.LookupExtensionWithHTTPClient(input.Query, client)
}

func (a *App) BrowserExtensionInstall(query string) (BrowserExtension, error) {
	return a.BrowserExtensionInstallWithProxy(BrowserExtensionWebStoreRequest{Query: query})
}

func (a *App) BrowserExtensionInstallWithProxy(input BrowserExtensionWebStoreRequest) (BrowserExtension, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.ctx == nil {
		return BrowserExtension{}, fmt.Errorf("应用上下文未初始化")
	}
	if a.browserMgr == nil {
		return BrowserExtension{}, fmt.Errorf("浏览器管理器未初始化")
	}
	client, err := a.extensionDownloadHTTPClient(input.UseProxy, input.ProxyConfig)
	if err != nil {
		return BrowserExtension{}, fmt.Errorf("下载代理配置错误: %w", err)
	}
	return a.browserMgr.InstallExtensionFromWebStoreWithHTTPClient(a.ctx, input.Query, client)
}

func (a *App) BrowserExtensionManualInstallGuide(query string) (BrowserExtensionManualInstallGuide, error) {
	extensionID := browser.NormalizeExtensionID(query)
	if extensionID == "" {
		return BrowserExtensionManualInstallGuide{}, fmt.Errorf("请输入 Chrome 插件 ID 或 Chrome Web Store 链接")
	}
	downloadDir := a.extensionManualDownloadDir()
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return BrowserExtensionManualInstallGuide{}, fmt.Errorf("创建手动下载目录失败: %w", err)
	}
	return BrowserExtensionManualInstallGuide{
		ExtensionID: extensionID,
		StoreURL:    browser.BuildChromeWebStoreURL(extensionID),
		DownloadURL: browser.BuildChromeExtensionDownloadURL(extensionID),
		DownloadDir: downloadDir,
		FileName:    extensionID + ".crx",
	}, nil
}

func (a *App) BrowserExtensionOpenManualDownloadDir() error {
	downloadDir := a.extensionManualDownloadDir()
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return fmt.Errorf("创建手动下载目录失败: %w", err)
	}
	absPath, err := filepath.Abs(downloadDir)
	if err != nil {
		return err
	}
	return openPathInFileManager(absPath)
}

func (a *App) BrowserExtensionListManualDownloadFiles() ([]BrowserExtensionManualDownloadFile, error) {
	downloadDir := a.extensionManualDownloadDir()
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建手动下载目录失败: %w", err)
	}
	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		return nil, fmt.Errorf("读取手动下载目录失败: %w", err)
	}
	files := make([]BrowserExtensionManualDownloadFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		lowerName := strings.ToLower(name)
		if !strings.HasSuffix(lowerName, ".crx") && !strings.HasSuffix(lowerName, ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, BrowserExtensionManualDownloadFile{
			FileName:  name,
			FilePath:  filepath.Join(downloadDir, name),
			SizeBytes: info.Size(),
			UpdatedAt: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}
	return files, nil
}

func (a *App) BrowserExtensionInstallManualDownloadFile(fileName string) (BrowserExtension, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.browserMgr == nil {
		return BrowserExtension{}, fmt.Errorf("浏览器管理器未初始化")
	}
	path, err := a.resolveManualDownloadFile(fileName)
	if err != nil {
		return BrowserExtension{}, err
	}
	return a.browserMgr.InstallExtensionPackageFile(path)
}

func (a *App) extensionManualDownloadDir() string {
	return a.resolveAppPath(filepath.ToSlash(filepath.Join("data", "extensions", "manual-downloads")))
}

func (a *App) resolveManualDownloadFile(fileName string) (string, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return "", fmt.Errorf("请选择要导入的插件包")
	}
	baseDir, err := filepath.Abs(a.extensionManualDownloadDir())
	if err != nil {
		return "", err
	}
	cleanName := filepath.Clean(filepath.FromSlash(fileName))
	if cleanName == "." || cleanName == "" || filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) || cleanName == ".." {
		return "", fmt.Errorf("插件包路径无效")
	}
	lowerName := strings.ToLower(cleanName)
	if !strings.HasSuffix(lowerName, ".crx") && !strings.HasSuffix(lowerName, ".zip") {
		return "", fmt.Errorf("只支持导入 .crx 或 .zip 插件包")
	}
	fullPath, err := filepath.Abs(filepath.Join(baseDir, cleanName))
	if err != nil {
		return "", err
	}
	if fullPath != baseDir && !strings.HasPrefix(fullPath, baseDir+string(os.PathSeparator)) {
		return "", fmt.Errorf("插件包路径越界")
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("插件包不存在: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("请选择插件包文件，不是目录")
	}
	return fullPath, nil
}

func (a *App) extensionDownloadHTTPClient(useProxy bool, proxyConfig string) (*http.Client, error) {
	proxyConfig = strings.TrimSpace(proxyConfig)
	log := logger.New("Extension")
	if useProxy && proxyConfig == "" {
		return nil, fmt.Errorf("已启用下载代理，但代理配置为空，请重新选择代理节点")
	}
	if proxyConfig == "" || strings.EqualFold(proxyConfig, "direct://") {
		log.Info("Chrome 插件下载使用直连")
		client, _, err := proxyCoreHTTPClient(browser.ExtensionDownloadTimeout(), "")
		return client, err
	}
	proxies := a.getLatestProxies()
	connectorType := config.BrowserConnectorXray
	if a != nil && a.config != nil {
		connectorType = a.config.Browser.DefaultConnectorType
	}
	log.Info("Chrome 插件下载使用代理", logger.F("connector", connectorType), logger.F("proxy_prefix", proxyConfigLogPrefix(proxyConfig)))
	return proxy.BuildProxyHTTPClient(proxyConfig, "", proxies, a.xrayMgr, a.singboxMgr, a.clashMgr, connectorType, browser.ExtensionDownloadTimeout())
}

func proxyConfigLogPrefix(proxyConfig string) string {
	proxyConfig = strings.TrimSpace(proxyConfig)
	if len(proxyConfig) <= 24 {
		return proxyConfig
	}
	return proxyConfig[:24]
}

func (a *App) BrowserExtensionInstallLocalFile() (BrowserExtension, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.ctx == nil {
		return BrowserExtension{}, fmt.Errorf("应用上下文未初始化")
	}
	if a.browserMgr == nil {
		return BrowserExtension{}, fmt.Errorf("浏览器管理器未初始化")
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择 Chrome 插件包",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Chrome 插件包 (*.crx;*.zip)", Pattern: "*.crx;*.zip"},
		},
	})
	if err != nil {
		return BrowserExtension{}, fmt.Errorf("打开文件选择框失败: %w", err)
	}
	if strings.TrimSpace(path) == "" {
		return BrowserExtension{}, fmt.Errorf("已取消选择")
	}
	return a.browserMgr.InstallExtensionPackageFile(path)
}

func (a *App) BrowserExtensionInstallLocalDirectory() (BrowserExtension, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.ctx == nil {
		return BrowserExtension{}, fmt.Errorf("应用上下文未初始化")
	}
	if a.browserMgr == nil {
		return BrowserExtension{}, fmt.Errorf("浏览器管理器未初始化")
	}
	path, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "选择已解压插件目录"})
	if err != nil {
		return BrowserExtension{}, fmt.Errorf("打开目录选择框失败: %w", err)
	}
	if strings.TrimSpace(path) == "" {
		return BrowserExtension{}, fmt.Errorf("已取消选择")
	}
	return a.browserMgr.InstallExtensionDirectory(path)
}

func (a *App) BrowserExtensionSetEnabled(extensionID string, enabled bool) (BrowserExtension, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.browserMgr == nil || a.browserMgr.ExtensionDAO == nil {
		return BrowserExtension{}, fmt.Errorf("插件管理器未初始化")
	}
	extensionID = strings.TrimSpace(extensionID)
	if extensionID == "" {
		return BrowserExtension{}, fmt.Errorf("插件 ID 不能为空")
	}
	if err := a.browserMgr.ExtensionDAO.SetEnabled(extensionID, enabled); err != nil {
		return BrowserExtension{}, err
	}
	if !enabled {
		if err := a.browserMgr.RemoveExtensionFromStoppedProfiles(extensionID); err != nil {
			_ = a.browserMgr.ExtensionDAO.SetEnabled(extensionID, true)
			return BrowserExtension{}, err
		}
	}
	return a.browserMgr.ExtensionDAO.Get(extensionID)
}

func (a *App) BrowserExtensionDelete(extensionID string) error {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.browserMgr == nil || a.browserMgr.ExtensionDAO == nil {
		return fmt.Errorf("插件管理器未初始化")
	}
	extensionID = strings.TrimSpace(extensionID)
	if extensionID == "" {
		return fmt.Errorf("插件 ID 不能为空")
	}
	extension, err := a.browserMgr.ExtensionDAO.Get(extensionID)
	if err != nil {
		return err
	}
	if _, err := a.resolveBrowserExtensionInstallDir(extension.InstallDir); err != nil {
		return err
	}
	if err := a.browserMgr.RemoveExtensionFromStoppedProfiles(extensionID); err != nil {
		return err
	}
	deletionStage, err := a.stageBrowserExtensionDeletion(extension)
	if err != nil {
		return err
	}
	if err := a.browserMgr.ExtensionDAO.Delete(extensionID); err != nil {
		if restoreErr := deletionStage.restore(); restoreErr != nil {
			return fmt.Errorf("删除插件记录失败：%w；物理文件恢复失败：%v", err, restoreErr)
		}
		return err
	}
	if err := deletionStage.cleanup(); err != nil {
		return fmt.Errorf("插件记录已删除，但清理删除暂存失败：%w", err)
	}
	return nil
}

type browserExtensionDeletionEntry struct {
	originalPath string
	stagedPath   string
}

type browserExtensionDeletionStage struct {
	root    string
	entries []browserExtensionDeletionEntry
}

func (a *App) stageBrowserExtensionDeletion(extension BrowserExtension) (*browserExtensionDeletionStage, error) {
	installDir, err := a.resolveBrowserExtensionInstallDir(extension.InstallDir)
	if err != nil {
		return nil, err
	}
	packagePaths, err := a.browserMgr.ExtensionPackagePaths(extension)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, 1+len(packagePaths))
	if installDir != "" {
		paths = append(paths, installDir)
	}
	paths = append(paths, packagePaths...)

	uniquePaths := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" {
			continue
		}
		duplicate := false
		for _, existingPath := range uniquePaths {
			if strings.EqualFold(existingPath, path) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			uniquePaths = append(uniquePaths, path)
		}
	}

	stage := &browserExtensionDeletionStage{}
	for _, path := range uniquePaths {
		if _, err := os.Lstat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("读取插件删除路径失败：%w", err)
		}
	}
	if len(uniquePaths) == 0 {
		return stage, nil
	}

	stage.root = a.resolveAppPath(filepath.ToSlash(filepath.Join("data", "extensions", ".delete-staging", fmt.Sprintf("extension-%d", time.Now().UnixNano()))))
	if err := os.MkdirAll(stage.root, 0o755); err != nil {
		return nil, fmt.Errorf("创建插件删除暂存目录失败：%w", err)
	}
	for index, path := range uniquePaths {
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			continue
		}
		stagedPath := filepath.Join(stage.root, fmt.Sprintf("%03d-%s", index, filepath.Base(path)))
		if err := os.Rename(path, stagedPath); err != nil {
			restoreErr := stage.restore()
			if restoreErr != nil {
				return nil, fmt.Errorf("暂存插件删除文件失败：%w；恢复失败：%v", err, restoreErr)
			}
			return nil, fmt.Errorf("暂存插件删除文件失败：%w", err)
		}
		stage.entries = append(stage.entries, browserExtensionDeletionEntry{originalPath: path, stagedPath: stagedPath})
	}
	return stage, nil
}

func (s *browserExtensionDeletionStage) restore() error {
	if s == nil {
		return nil
	}
	var firstErr error
	for index := len(s.entries) - 1; index >= 0; index-- {
		entry := s.entries[index]
		if _, err := os.Lstat(entry.stagedPath); err != nil {
			if !os.IsNotExist(err) && firstErr == nil {
				firstErr = err
			}
			continue
		}
		if _, err := os.Lstat(entry.originalPath); err == nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("恢复目标已存在：%s", entry.originalPath)
			}
			continue
		} else if !os.IsNotExist(err) {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(entry.originalPath), 0o755); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := os.Rename(entry.stagedPath, entry.originalPath); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr == nil && s.root != "" {
		if err := os.RemoveAll(s.root); err != nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *browserExtensionDeletionStage) cleanup() error {
	if s == nil || s.root == "" {
		return nil
	}
	return os.RemoveAll(s.root)
}

func (a *App) resolveBrowserExtensionInstallDir(installDir string) (string, error) {
	installDir = strings.TrimSpace(installDir)
	if installDir == "" {
		return "", nil
	}
	target, err := filepath.Abs(installDir)
	if err != nil {
		return "", fmt.Errorf("解析插件目录失败: %w", err)
	}
	root, err := filepath.Abs(a.resolveAppPath(filepath.ToSlash(filepath.Join("data", "extensions"))))
	if err != nil {
		return "", fmt.Errorf("解析插件根目录失败: %w", err)
	}
	target = filepath.Clean(target)
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || rel == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("拒绝删除插件根目录外的路径: %s", installDir)
	}
	return target, nil
}

func (a *App) BrowserProfileExtensionGet(profileID string) (BrowserProfileExtensionSettings, error) {
	if a.browserMgr == nil || a.browserMgr.ExtensionDAO == nil {
		return BrowserProfileExtensionSettings{}, fmt.Errorf("插件管理器未初始化")
	}
	return a.browserMgr.ExtensionDAO.GetProfileSettings(profileID)
}

func (a *App) BrowserProfileExtensionSave(profileID string, extensionIDs []string, configured bool) (BrowserProfileExtensionSettings, error) {
	if a.browserMgr == nil || a.browserMgr.ExtensionDAO == nil {
		return BrowserProfileExtensionSettings{}, fmt.Errorf("插件管理器未初始化")
	}
	return a.browserMgr.ExtensionDAO.SetProfileSettings(profileID, extensionIDs, configured)
}
