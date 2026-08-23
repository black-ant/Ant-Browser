package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// DownloadAndExtractCore 执行异步下载解压并在过程中发送事件
func (m *Manager) DownloadAndExtractCore(ctx context.Context, coreName string, targetUrl string, proxyConfig string) {
	coreName = strings.TrimSpace(coreName)
	for _, core := range m.ListCores() {
		if strings.EqualFold(core.CoreName, coreName) || filepath.Base(core.CorePath) == coreName {
			m.downloadAndExtractCore(ctx, CoreInput{
				CoreId:    core.CoreId,
				CoreName:  core.CoreName,
				CorePath:  core.CorePath,
				IsDefault: core.IsDefault,
			}, targetUrl, proxyConfig, false)
			return
		}
	}
	m.downloadAndExtractCore(ctx, CoreInput{CoreName: coreName}, targetUrl, proxyConfig, false)
}

// RedownloadCore 重新下载指定内核，验证成功后替换原目录并保留原配置。
func (m *Manager) RedownloadCore(ctx context.Context, coreId string, targetUrl string, proxyConfig string) {
	core, ok := m.GetCore(coreId)
	if !ok {
		m.emitRuntimeEvent(ctx, "download:progress", DownloadProgress{Phase: "error", Progress: 0, Message: "内核不存在"})
		return
	}
	m.downloadAndExtractCore(ctx, CoreInput{
		CoreId:    core.CoreId,
		CoreName:  core.CoreName,
		CorePath:  core.CorePath,
		IsDefault: core.IsDefault,
	}, targetUrl, proxyConfig, true)
}

func (m *Manager) downloadAndExtractCore(ctx context.Context, coreInput CoreInput, targetUrl string, proxyConfig string, replaceExisting bool) {
	log := logger.New("Browser")
	t := time.Now()

	sendEvent := func(phase string, progress int, msg string) {
		m.emitRuntimeEvent(ctx, "download:progress", DownloadProgress{
			Phase:    phase,
			Progress: progress,
			Message:  msg,
		})
	}

	sendEvent("downloading", 0, "开始解析地址并创建下载请求: "+targetUrl)

	coreName := strings.TrimSpace(coreInput.CoreName)
	if coreName == "" {
		sendEvent("error", 0, "内核名称不能为空")
		return
	}

	for _, c := range m.ListCores() {
		if replaceExisting && strings.EqualFold(c.CoreId, strings.TrimSpace(coreInput.CoreId)) {
			continue
		}
		if strings.EqualFold(c.CoreName, coreName) || filepath.Base(c.CorePath) == coreName {
			sendEvent("error", 0, "名称已存在，请换一个名称")
			return
		}
	}

	targetCorePath := strings.TrimSpace(coreInput.CorePath)
	if targetCorePath == "" {
		targetCorePath = filepath.Join("chrome", coreName)
	}
	targetDir := m.ResolveRelativePath(targetCorePath)
	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		sendEvent("error", 0, "创建内核目录失败")
		return
	}

	if _, err := os.Stat(targetDir); err == nil && !replaceExisting {
		sendEvent("error", 0, "同名内核目录已存在，请改名下载；如需覆盖，请在内核列表使用重新下载")
		return
	} else if err != nil && !os.IsNotExist(err) {
		sendEvent("error", 0, "检查内核目录失败: "+err.Error())
		return
	}

	// 2. 准备 HttpClient（优先从 Windows 注册表读取真实系统代理，而非仅靠环境变量）
	transport := &http.Transport{}
	if proxyConfig == "__system__" {
		// http.ProxyFromEnvironment 只读环境变量，而 Clash 的全局代理写在 Windows 注册表里
		// 必须直接读取注册表才能拿到正确的代理地址
		if sysProxy, rErr := readSystemProxy(); rErr == nil && sysProxy != "" {
			if proxyURL, pErr := url.Parse(sysProxy); pErr == nil {
				transport.Proxy = http.ProxyURL(proxyURL)
				sendEvent("downloading", 0, "已从系统注册表读取代理: "+sysProxy)
			} else {
				// 解析失败则回退到环境变量
				transport.Proxy = http.ProxyFromEnvironment
			}
		} else {
			// 没有系统代理配置或读取失败，尝试环境变量兜底
			transport.Proxy = http.ProxyFromEnvironment
			sendEvent("downloading", 0, "系统注册表无代理配置，使用环境变量兜底")
		}
	} else if proxyConfig != "" && proxyConfig != "direct://" && proxyConfig != "__direct__" {
		if proxyURL, pErr := url.Parse(proxyConfig); pErr == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		} else {
			sendEvent("error", 0, "代理地址解析失败: "+pErr.Error())
			return
		}
	}

	client := &http.Client{
		Timeout:   0, // 取消全局超时，依靠 context 和分片连接维持
		Transport: transport,
	}

	tempFile, err := os.CreateTemp(parentDir, coreArchiveTempPattern(targetUrl))
	if err != nil {
		sendEvent("error", 0, "创建临时文件失败: "+err.Error())
		return
	}
	tempFilePath := tempFile.Name()
	defer func() {
		tempFile.Close()
		os.Remove(tempFilePath) // 清理临时文件
	}()

	sendEvent("downloading", 0, "开始分析下载链接(检测多线程支持)...")

	err = doConcurrentDownload(ctx, client, targetUrl, tempFile, sendEvent)
	if err != nil {
		sendEvent("error", 0, "下载失败: "+err.Error())
		return
	}

	tempFile.Close() // 解压前先关闭写句柄
	sendEvent("extracting", 0, "下载完成，正在准备解压文件...")
	log.Info("内核下载完成", logger.F("url", targetUrl), logger.F("temp", tempFilePath), logger.F("cost", time.Since(t).String()))

	tempExtractDir, err := os.MkdirTemp(parentDir, coreName+"_extract_*")
	if err != nil {
		sendEvent("error", 0, "创建临时解压目录失败: "+err.Error())
		return
	}
	cleanupTempExtract := true
	defer func() {
		if cleanupTempExtract {
			os.RemoveAll(tempExtractDir)
		}
	}()

	// 3. 执行解压，并剥离顶层文件夹
	if err := extractCoreArchiveAndStripRoot(tempFilePath, tempExtractDir, func(p int, msg string) {
		sendEvent("extracting", p, msg)
	}); err != nil {
		sendEvent("error", 0, "解压失败: "+err.Error())
		return
	}

	if !m.ValidateCorePath(tempExtractDir).Valid {
		sendEvent("error", 0, fmt.Sprintf("解压后未找到当前平台可用的浏览器可执行文件（当前平台 %s，候选：%s），请检查压缩包内容！", CoreExecutablePlatform(), strings.Join(CoreExecutableCandidates(), ", ")))
		return
	}

	if err := replaceCoreDirectory(targetDir, tempExtractDir, replaceExisting); err != nil {
		sendEvent("error", 0, "替换内核目录失败: "+err.Error())
		return
	}
	cleanupTempExtract = false

	coreToSave := CoreInput{
		CoreId:    strings.TrimSpace(coreInput.CoreId),
		CoreName:  coreName,
		CorePath:  targetCorePath,
		IsDefault: coreInput.IsDefault,
	}
	if coreToSave.CoreId == "" {
		coreToSave.CoreId = uuid.NewString()
		coreToSave.IsDefault = len(m.ListCores()) == 0
	}
	if err := m.SaveCore(coreToSave); err != nil {
		sendEvent("error", 0, "保存配置入库失败: "+err.Error())
		return
	}

	if replaceExisting && strings.TrimSpace(coreInput.CoreId) != "" {
		sendEvent("done", 100, "内核重新下载成功！")
		log.Info("内核重新下载成功", logger.F("core_id", coreToSave.CoreId), logger.F("core_name", coreName))
	} else {
		sendEvent("done", 100, "内核下载与配置成功！")
		log.Info("内核下载配置入库成功", logger.F("core_name", coreName))
	}
}

func (m *Manager) emitRuntimeEvent(ctx context.Context, eventName string, optionalData ...interface{}) {
	if m.EventEmitter != nil {
		m.EventEmitter(eventName, optionalData...)
		return
	}
	if ctx != nil {
		runtime.EventsEmit(ctx, eventName, optionalData...)
	}
}

func replaceCoreDirectory(targetDir string, tempExtractDir string, replaceExisting bool) error {
	if !replaceExisting {
		return os.Rename(tempExtractDir, targetDir)
	}

	backupDir := targetDir + ".backup_" + time.Now().Format("20060102150405")
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.Rename(targetDir, backupDir); err != nil {
			return err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(tempExtractDir, targetDir); err != nil {
		if backupDir != "" {
			_ = os.Rename(backupDir, targetDir)
		}
		return err
	}

	if backupDir != "" {
		_ = os.RemoveAll(backupDir)
	}
	return nil
}
