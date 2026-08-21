package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"os"
	"strings"
)

// 内核后端可能需要通过环境变量传递配置（Cloak 的 license key、缓存目录等）。
// 这些值来自内核配置的 core_env，不是全局环境，所以要在启动进程时按内核合并。

// browserCoreEnvKeyPrefixes 允许通过内核配置注入的环境变量前缀白名单。
// 收紧白名单是为了避免内核配置变成任意改写子进程环境的通道
// （例如覆盖 PATH 或代理相关变量，会影响实例的实际出网行为）。
var browserCoreEnvKeyPrefixes = []string{
	"CLOAKBROWSER_",
}

// buildBrowserProcessEnv 返回启动浏览器进程时使用的环境变量。
// 返回 nil 表示无需定制，调用方应保持 cmd.Env 为 nil 以继承父进程环境。
func buildBrowserProcessEnv(core browser.Core, profileID string) []string {
	overrides, rejected := sanitizeCoreEnvEntries(core.CoreEnv)
	if len(rejected) > 0 {
		logger.New("Browser").Warn("忽略内核环境变量（不在允许的前缀白名单内）",
			logger.F("profile_id", profileID),
			logger.F("core_id", core.CoreId),
			logger.F("core_backend", config.NormalizeCoreBackend(core.CoreBackend)),
			logger.F("rejected", rejected),
			logger.F("allowed_prefixes", browserCoreEnvKeyPrefixes),
		)
	}
	if len(overrides) == 0 {
		return nil
	}
	return mergeProcessEnv(os.Environ(), overrides)
}

// sanitizeCoreEnvEntries 过滤内核环境变量，返回通过白名单的条目和被拒绝的键名。
func sanitizeCoreEnvEntries(entries []string) ([]string, []string) {
	if len(entries) == 0 {
		return nil, nil
	}
	accepted := make([]string, 0, len(entries))
	rejected := make([]string, 0)
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		key, _, ok := splitEnvEntry(entry)
		if !ok {
			rejected = append(rejected, entry)
			continue
		}
		if !coreEnvKeyAllowed(key) {
			rejected = append(rejected, key)
			continue
		}
		accepted = append(accepted, entry)
	}
	if len(accepted) == 0 {
		accepted = nil
	}
	if len(rejected) == 0 {
		rejected = nil
	}
	return accepted, rejected
}

func coreEnvKeyAllowed(key string) bool {
	upperKey := strings.ToUpper(key)
	for _, prefix := range browserCoreEnvKeyPrefixes {
		if strings.HasPrefix(upperKey, prefix) && len(upperKey) > len(prefix) {
			return true
		}
	}
	return false
}

// mergeProcessEnv 将 overrides 合并进 base，同名键以 overrides 为准。
// Windows 环境变量名不区分大小写，这里统一按大写键比较，避免出现同名双份条目。
func mergeProcessEnv(base []string, overrides []string) []string {
	overrideIndex := make(map[string]string, len(overrides))
	order := make([]string, 0, len(overrides))
	for _, entry := range overrides {
		key, _, ok := splitEnvEntry(entry)
		if !ok {
			continue
		}
		upperKey := strings.ToUpper(key)
		if _, exists := overrideIndex[upperKey]; !exists {
			order = append(order, upperKey)
		}
		overrideIndex[upperKey] = entry
	}

	merged := make([]string, 0, len(base)+len(order))
	consumed := make(map[string]struct{}, len(order))
	for _, entry := range base {
		key, _, ok := splitEnvEntry(entry)
		if !ok {
			merged = append(merged, entry)
			continue
		}
		upperKey := strings.ToUpper(key)
		if replacement, exists := overrideIndex[upperKey]; exists {
			if _, done := consumed[upperKey]; done {
				continue
			}
			consumed[upperKey] = struct{}{}
			merged = append(merged, replacement)
			continue
		}
		merged = append(merged, entry)
	}
	for _, upperKey := range order {
		if _, done := consumed[upperKey]; done {
			continue
		}
		merged = append(merged, overrideIndex[upperKey])
	}
	return merged
}

// splitEnvEntry 拆分 KEY=VALUE，key 为空或缺少 "=" 时返回 ok=false。
func splitEnvEntry(entry string) (string, string, bool) {
	index := strings.Index(entry, "=")
	if index <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(entry[:index])
	if key == "" {
		return "", "", false
	}
	return key, entry[index+1:], true
}
