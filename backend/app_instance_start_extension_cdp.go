package backend

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
)

// Chrome 137 起移除了 --load-extension 命令行开关（Chrome for Developers 2025-06 公告：
// "移除 --load-extension 标志"），稳定版 Chrome 会静默忽略该参数。
// 实测 Chrome 155 上：命令行参数确实带上了两个插件目录，但 chrome://extensions 里
// 一个都没有，CDP Extensions.getExtensions 返回空列表。
//
// 替代路径是浏览器的 CDP 扩展接口：Extensions.getExtensions 查已加载，
// Extensions.loadUnpacked 逐个加载未打包目录。本文件在调试端口稳定后补这一步。
// 旧版 Chrome（没有 Extensions 域）会在这里拿到方法不存在的错误，此时
// --load-extension 本身有效，直接跳过，行为与改动前一致。
const (
	cdpMethodExtensionsGet  = "Extensions.getExtensions"
	cdpMethodExtensionsLoad = "Extensions.loadUnpacked"

	// extensionLoadBudget 是补齐插件的总时间预算。单个 CDP 调用在本机通常几十毫秒，
	// 这里留足余量但避免个别插件卡住时把启动流程拖到几十秒。
	extensionLoadBudget = 6 * time.Second
)

// ensureProfileExtensionsLoaded 在浏览器就绪后通过 CDP 补齐插件加载。
// 返回给用户看的警告文案（加载失败时非空），不影响实例启动结果。
func (a *App) ensureProfileExtensionsLoaded(profileId string, debugPort int, paths []string) string {
	if debugPort <= 0 || len(paths) == 0 {
		return ""
	}
	log := logger.New("Extension")

	loaded, err := cdpBrowserCallResult(debugPort, cdpMethodExtensionsGet, nil)
	if err != nil {
		log.Info("当前内核不支持 CDP 扩展接口，沿用启动参数加载插件",
			logger.F("profile_id", profileId),
			logger.F("debug_port", debugPort),
			logger.F("error", err.Error()),
		)
		return ""
	}

	loadedPaths := make([]string, 0)
	if items, ok := loaded["extensions"].([]any); ok {
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if path, ok := entry["path"].(string); ok && path != "" {
				loadedPaths = append(loadedPaths, path)
			}
		}
	}

	pending := missingExtensionPaths(loadedPaths, paths)

	failures := make([]string, 0)
	loadedCount := 0
	deadline := time.Now().Add(extensionLoadBudget)
	for index, clean := range pending {
		if time.Now().After(deadline) {
			remaining := len(pending) - index
			log.Warn("插件加载超出时间预算，剩余插件跳过",
				logger.F("profile_id", profileId),
				logger.F("remaining", remaining),
				logger.F("budget", extensionLoadBudget.String()),
			)
			failures = append(failures, fmt.Sprintf("剩余 %d 个插件因超时未加载", remaining))
			break
		}
		result, err := cdpBrowserCallResult(debugPort, cdpMethodExtensionsLoad, map[string]any{"path": clean})
		if err != nil {
			log.Warn("插件通过 CDP 加载失败",
				logger.F("profile_id", profileId),
				logger.F("path", clean),
				logger.F("error", err.Error()),
			)
			failures = append(failures, fmt.Sprintf("%s（%v）", filepath.Base(clean), err))
			continue
		}
		loadedCount++
		runtimeID, _ := result["id"].(string)
		log.Info("插件已通过 CDP 加载",
			logger.F("profile_id", profileId),
			logger.F("path", clean),
			logger.F("runtime_extension_id", runtimeID),
		)
	}

	if loadedCount > 0 {
		log.Info("插件加载完成",
			logger.F("profile_id", profileId),
			logger.F("loaded", loadedCount),
			logger.F("total", len(paths)),
		)
	}
	if len(failures) > 0 {
		return "部分插件加载失败：" + strings.Join(failures, "；")
	}
	return ""
}

// missingExtensionPaths 返回「需要补加载」的插件目录：去掉空值、按路径归一去重，
// 并剔除浏览器里已经加载过的目录（旧版 Chrome 走 --load-extension 已经把插件装上了）。
// 抽成纯函数是为了能脱离浏览器进程做单测。
func missingExtensionPaths(alreadyLoaded []string, wanted []string) []string {
	loaded := make(map[string]bool, len(alreadyLoaded))
	for _, path := range alreadyLoaded {
		if clean := normalizeExtensionPath(path); clean != "" {
			loaded[clean] = true
		}
	}

	pending := make([]string, 0, len(wanted))
	seen := make(map[string]bool, len(wanted))
	for _, path := range wanted {
		clean := normalizeExtensionPath(path)
		if clean == "" {
			continue
		}
		if loaded[clean] || seen[clean] {
			continue
		}
		seen[clean] = true
		pending = append(pending, clean)
	}
	return pending
}

// normalizeExtensionPath 把插件目录归一成可比较的绝对路径形式，空值返回空串。
func normalizeExtensionPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	clean := filepath.Clean(trimmed)
	if clean == "." || clean == string(filepath.Separator) {
		return ""
	}
	return clean
}
