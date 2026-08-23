package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) GetBrowserSettings() BrowserSettings {
	return BrowserSettings{
		UserDataRoot:           a.config.Browser.UserDataRoot,
		DefaultFingerprintArgs: append([]string{}, a.config.Browser.DefaultFingerprintArgs...),
		DefaultLaunchArgs:      append([]string{}, a.config.Browser.DefaultLaunchArgs...),
		DefaultStartURLs:       append([]string{}, a.config.Browser.DefaultStartURLs...),
		LightStartEnabled:      browserLightStartEnabled(a.config),
		RestoreLastSession:     a.config.Browser.RestoreLastSession,
		StartReadyTimeoutMs:    browserStartReadyTimeoutMillis(a.config),
		StartStableWindowMs:    browserStartStableWindowMillis(a.config),
		DefaultConnectorType:   config.NormalizeBrowserConnectorType(a.config.Browser.DefaultConnectorType),
	}
}

func (a *App) SaveBrowserSettings(settings BrowserSettings) error {
	log := logger.New("Browser")
	a.config.Browser.UserDataRoot = strings.TrimSpace(settings.UserDataRoot)
	a.config.Browser.DefaultFingerprintArgs = append([]string{}, settings.DefaultFingerprintArgs...)
	a.config.Browser.DefaultLaunchArgs = append([]string{}, settings.DefaultLaunchArgs...)
	if settings.DefaultStartURLs != nil {
		a.config.Browser.DefaultStartURLs = normalizeNonEmptyStrings(settings.DefaultStartURLs)
	} else if a.config.Browser.DefaultStartURLs == nil {
		a.config.Browser.DefaultStartURLs = config.DefaultBrowserStartURLs()
	}
	lightStartEnabled := settings.LightStartEnabled
	a.config.Browser.LightStartEnabled = &lightStartEnabled
	a.config.Browser.RestoreLastSession = settings.RestoreLastSession
	a.config.Browser.DefaultConnectorType = config.NormalizeBrowserConnectorType(settings.DefaultConnectorType)
	if settings.StartReadyTimeoutMs > 0 {
		a.config.Browser.StartReadyTimeoutMs = settings.StartReadyTimeoutMs
	} else if a.config.Browser.StartReadyTimeoutMs <= 0 {
		a.config.Browser.StartReadyTimeoutMs = browserStartReadyTimeoutMillis(nil)
	}
	if settings.StartStableWindowMs > 0 {
		a.config.Browser.StartStableWindowMs = settings.StartStableWindowMs
	} else if a.config.Browser.StartStableWindowMs <= 0 {
		a.config.Browser.StartStableWindowMs = browserStartStableWindowMillis(nil)
	}
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		log.Error("浏览器配置保存失败", logger.F("error", err))
		return err
	}
	return nil
}

func (a *App) BrowserCoreList() []BrowserCore {
	return a.browserMgr.ListCores()
}

func (a *App) BrowserCoreSave(input BrowserCoreInput) error {
	return a.browserMgr.SaveCore(input)
}

func (a *App) BrowserCoreDelete(coreId string) error {
	return a.browserMgr.DeleteCore(coreId)
}

func (a *App) BrowserCoreSetDefault(coreId string) error {
	return a.browserMgr.SetDefaultCore(coreId)
}

func (a *App) BrowserCoreValidate(corePath string) BrowserCoreValidateResult {
	return a.browserMgr.ValidateCorePath(corePath)
}

func (a *App) BrowserCoreExtendedInfo() []BrowserCoreExtendedInfo {
	return a.browserMgr.GetCoresExtendedInfo()
}

// BrowserCoreScan 重新扫描 chrome 目录，自动注册新内核
func (a *App) BrowserCoreScan() []BrowserCore {
	a.autoDetectCores()
	return a.browserMgr.ListCores()
}

// BrowserCoreImportLocal 选择一个已解压内核目录或归档文件并注册。
func (a *App) BrowserCoreImportLocal() (*BrowserCore, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("app context is nil")
	}
	if a.browserMgr == nil {
		return nil, fmt.Errorf("browser manager is nil")
	}

	selectedPath, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择 Chrome 内核归档文件",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Chrome 内核归档 (" + browser.SupportedCoreArchiveDescription() + ")", Pattern: browser.SupportedCoreArchivePattern()},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, err
	}
	selectedPath = strings.TrimSpace(selectedPath)
	if selectedPath == "" {
		return nil, nil
	}

	absPath, err := filepath.Abs(selectedPath)
	if err != nil {
		return nil, err
	}
	return a.importLocalBrowserCoreArchive(absPath)
}

func (a *App) importLocalBrowserCoreArchive(archivePath string) (*BrowserCore, error) {
	archiveName := strings.TrimSpace(filepath.Base(archivePath))
	coreName := strings.TrimSpace(coreNameFromArchiveName(archiveName))
	if coreName == "" {
		coreName = "本地内核"
	}

	targetCorePath := filepath.Join("chrome", coreName)
	targetDir := a.browserMgr.ResolveRelativePath(targetCorePath)
	if _, err := os.Stat(targetDir); err == nil {
		return nil, fmt.Errorf("同名内核目录已存在：%s", targetCorePath)
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return nil, err
	}
	tempExtractDir, err := os.MkdirTemp(parentDir, coreName+"_import_*")
	if err != nil {
		return nil, err
	}
	cleanupTempExtract := true
	defer func() {
		if cleanupTempExtract {
			_ = os.RemoveAll(tempExtractDir)
		}
	}()

	a.emitBrowserCoreImportProgress("extracting", 0, "开始解压本地内核包...")
	if err := browser.ExtractCoreArchiveAndStripRootForImport(archivePath, tempExtractDir, func(progress int, message string) {
		a.emitBrowserCoreImportProgress("extracting", progress, message)
	}); err != nil {
		a.emitBrowserCoreImportProgress("error", 0, "解压失败: "+err.Error())
		return nil, fmt.Errorf("解压失败: %w", err)
	}
	a.emitBrowserCoreImportProgress("validating", 90, "正在校验内核可执行文件...")
	if _, _, ok := browser.FindCoreExecutable(tempExtractDir); !ok {
		err := fmt.Errorf("所选归档不是当前平台可用的内核包：当前平台 %s，未找到浏览器可执行文件（候选：%s）", browser.CoreExecutablePlatform(), strings.Join(browser.CoreExecutableCandidates(), ", "))
		a.emitBrowserCoreImportProgress("error", 0, err.Error())
		return nil, err
	}
	a.emitBrowserCoreImportProgress("saving", 95, "正在保存内核配置...")
	if err := os.Rename(tempExtractDir, targetDir); err != nil {
		a.emitBrowserCoreImportProgress("error", 0, "保存内核目录失败: "+err.Error())
		return nil, err
	}
	cleanupTempExtract = false

	input := browser.CoreInput{
		CoreName:  coreName,
		CorePath:  targetCorePath,
		IsDefault: len(a.browserMgr.ListCores()) == 0,
	}
	if err := a.browserMgr.SaveCore(input); err != nil {
		a.emitBrowserCoreImportProgress("error", 0, "保存配置失败: "+err.Error())
		return nil, err
	}
	for _, saved := range a.browserMgr.ListCores() {
		if normalizeCorePathForCompare(saved.CorePath) == normalizeCorePathForCompare(targetCorePath) {
			a.emitBrowserCoreImportProgress("done", 100, "导入完成")
			return &saved, nil
		}
	}
	err = fmt.Errorf("本地内核已保存但未能读取结果")
	a.emitBrowserCoreImportProgress("error", 0, err.Error())
	return nil, err
}

func (a *App) emitBrowserCoreImportProgress(phase string, progress int, message string) {
	if a == nil || a.ctx == nil {
		return
	}
	a.emitRuntimeEvent("core-import:progress", map[string]interface{}{
		"phase":    phase,
		"progress": progress,
		"message":  message,
	})
}

func coreNameFromArchiveName(name string) string {
	name = strings.TrimSpace(name)
	for _, suffix := range []string{".tar.gz", ".tar.xz", ".tar.bz2", ".tgz", ".txz", ".tbz2", ".zip", ".tar"} {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			return strings.TrimSpace(name[:len(name)-len(suffix)])
		}
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// BrowserCoreImportLocalDirectory 选择一个已解压内核目录并直接注册，不下载、不复制文件。
func (a *App) BrowserCoreImportLocalDirectory() (*BrowserCore, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("app context is nil")
	}
	if a.browserMgr == nil {
		return nil, fmt.Errorf("browser manager is nil")
	}

	selectedDir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择已解压的 Chrome 内核目录",
	})
	if err != nil {
		return nil, err
	}
	selectedDir = strings.TrimSpace(selectedDir)
	if selectedDir == "" {
		return nil, nil
	}

	absDir, err := filepath.Abs(selectedDir)
	if err != nil {
		return nil, err
	}
	if _, _, ok := browser.FindCoreExecutable(absDir); !ok {
		return nil, fmt.Errorf("所选目录不是当前平台可用的内核目录：当前平台 %s，未找到浏览器可执行文件（候选：%s）", browser.CoreExecutablePlatform(), strings.Join(browser.CoreExecutableCandidates(), ", "))
	}

	corePath := a.relativeCorePathIfPossible(absDir)
	coreName := strings.TrimSpace(filepath.Base(absDir))
	if coreName == "" || coreName == "." || coreName == string(filepath.Separator) {
		coreName = "本地内核"
	}

	for _, existing := range a.browserMgr.ListCores() {
		if normalizeCorePathForCompare(existing.CorePath) == normalizeCorePathForCompare(corePath) {
			return &existing, nil
		}
	}

	input := browser.CoreInput{
		CoreName:  coreName,
		CorePath:  corePath,
		IsDefault: len(a.browserMgr.ListCores()) == 0,
	}
	if err := a.browserMgr.SaveCore(input); err != nil {
		return nil, err
	}

	for _, saved := range a.browserMgr.ListCores() {
		if normalizeCorePathForCompare(saved.CorePath) == normalizeCorePathForCompare(corePath) {
			return &saved, nil
		}
	}
	return nil, fmt.Errorf("本地内核已保存但未能读取结果")
}

func (a *App) relativeCorePathIfPossible(absDir string) string {
	for _, root := range []string{a.appRootAbs(), a.appStateRootAbs()} {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(rootAbs, absDir)
		if err == nil && rel != "." && rel != ".." && !strings.HasPrefix(filepath.ToSlash(rel), "../") && !filepath.IsAbs(rel) {
			return filepath.ToSlash(rel)
		}
	}
	return absDir
}

// BrowserCoreDownload 在线下载并自动解压配置内核
func (a *App) BrowserCoreDownload(coreName, url, proxyConfig string) error {
	if a.ctx == nil {
		return fmt.Errorf("app context is nil")
	}
	go a.browserMgr.DownloadAndExtractCore(a.ctx, coreName, url, proxyConfig)
	return nil
}

// BrowserCoreRedownload 重新下载并替换指定内核目录
func (a *App) BrowserCoreRedownload(coreId, url, proxyConfig string) error {
	if a.ctx == nil {
		return fmt.Errorf("app context is nil")
	}
	go a.browserMgr.RedownloadCore(a.ctx, coreId, url, proxyConfig)
	return nil
}
