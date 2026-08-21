package browser

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"ant-chrome/backend/internal/config"
)

// 本文件集中存放"内核后端差异"知识，避免后端判断散落在各处。
//
// fingerprint_chromium：历史唯一后端，目录里有 chrome.exe 和 manifest.json / *.manifest。
// cloak：CloakBrowser，源码级 patch 的 stealth Chromium，二进制单独分发。
//
// 以下事实取自 CloakBrowser wrapper 源码 cloakbrowser/config.py
// （get_binary_dir / get_binary_path / SUPPORTED_PLATFORMS）：
//
//	缓存根目录  ~/.cloakbrowser（可用 CLOAKBROWSER_CACHE_DIR 覆盖）
//	版本目录    chromium-<version>[-pro]      例：chromium-146.0.7680.177.5-pro
//	Windows     <版本目录>/chrome.exe
//	Linux       <版本目录>/chrome             （扁平二进制，无扩展名）
//	macOS       <版本目录>/Chromium.app/Contents/MacOS/Chromium
//
// 注意版本目录名用的是 chromium-<version>，而 Release tag 用的是 chromium-v<version>，
// 两者差一个 "v"；这里解析的是解压后的目录名，所以不带 v，但同时兼容带 v 的写法。

// CoreBackendLabel 返回内核后端的中文展示名。
func CoreBackendLabel(backend string) string {
	if config.NormalizeCoreBackend(backend) == config.CoreBackendCloak {
		return "Cloak"
	}
	return "fingerprint-chromium"
}

// cloakExecutableCandidates 返回 Cloak 后端在当前平台的可执行文件候选名。
// 名单按 wrapper 的 get_binary_path 收窄到实际会产出的文件，避免误匹配同目录其他可执行文件。
func cloakExecutableCandidates() []string {
	switch goruntime.GOOS {
	case "windows":
		return []string{"chrome.exe"}
	case "linux":
		return []string{"chrome"}
	case "darwin":
		return []string{"Chromium.app/Contents/MacOS/Chromium"}
	default:
		return []string{"chrome"}
	}
}

// CoreExecutableCandidatesForBackend 返回指定后端在当前平台可接受的可执行文件候选名。
func CoreExecutableCandidatesForBackend(backend string) []string {
	if config.NormalizeCoreBackend(backend) == config.CoreBackendCloak {
		return cloakExecutableCandidates()
	}
	return CoreExecutableCandidates()
}

// CoreExecutableCandidatesAnyBackend 返回所有后端候选名的并集，用于"后端未知"的场景
// （例如自动扫描内核目录时还没有确定这是哪种内核）。
func CoreExecutableCandidatesAnyBackend() []string {
	merged := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)
	for _, backend := range config.KnownCoreBackends() {
		for _, candidate := range CoreExecutableCandidatesForBackend(backend) {
			key := strings.ToLower(candidate)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, candidate)
		}
	}
	return merged
}

// FindCoreExecutableForBackend 按指定后端的候选名在目录中查找可执行文件。
func FindCoreExecutableForBackend(baseDir string, backend string) (string, string, bool) {
	return findCoreExecutableWithCandidates(baseDir, CoreExecutableCandidatesForBackend(backend))
}

// DetectCoreBackend 根据目录特征推断内核后端。
// 仅用于自动扫描和导入时给出初值，用户仍可在内核管理里显式修改。
func DetectCoreBackend(baseDir string) string {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return config.CoreBackendFingerprintChromium
	}
	// Cloak 的缓存布局把版本号放在目录名里：chromium-<version>/
	if cloakVersionFromDirName(filepath.Base(baseDir)) != "" {
		return config.CoreBackendCloak
	}
	if entries, err := os.ReadDir(baseDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if cloakVersionFromDirName(entry.Name()) != "" {
				return config.CoreBackendCloak
			}
		}
	}
	return config.CoreBackendFingerprintChromium
}

// cloakVersionFromDirName 从 Cloak 缓存目录名提取版本号。
//
// 支持的形式（来自 wrapper get_binary_dir，附带兼容 release tag 的 v 前缀）：
//
//	chromium-146.0.7680.177.5        普通版本目录
//	chromium-148.0.7778.215.3-pro    Pro 版本目录（-pro 后缀）
//	chromium-v146.0.7680.177.5       release tag 写法，容错处理
//
// 不匹配时返回空串。
func cloakVersionFromDirName(name string) string {
	name = strings.TrimSpace(name)
	const prefix = "chromium-"
	if len(name) <= len(prefix) || !strings.EqualFold(name[:len(prefix)], prefix) {
		return ""
	}
	version := strings.TrimSpace(name[len(prefix):])
	// Pro 构建的目录名带 -pro 后缀，版本号本身不含该后缀
	version = strings.TrimSuffix(strings.TrimSuffix(version, "-pro"), "-PRO")
	// 容错 release tag 的 v 前缀（chromium-v146...）
	if len(version) > 1 && (version[0] == 'v' || version[0] == 'V') {
		version = version[1:]
	}
	if version == "" {
		return ""
	}
	// 只接受数字和点组成的版本号，避免把 chromium-foo 之类的目录误判成版本。
	for _, char := range version {
		if (char < '0' || char > '9') && char != '.' {
			return ""
		}
	}
	if !strings.ContainsRune(version, '.') {
		return ""
	}
	return version
}

// cloakCoreVersion 解析 Cloak 内核目录的 Chromium 版本号。
// 先看目录本身，再看下一层子目录（Cloak 缓存把二进制放在 chromium-<version>/ 里）。
func cloakCoreVersion(baseDir string) string {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return ""
	}
	if version := cloakVersionFromDirName(filepath.Base(baseDir)); version != "" {
		return version
	}
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return ""
	}
	best := ""
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		version := cloakVersionFromDirName(entry.Name())
		if version == "" {
			continue
		}
		// 同时存在多个版本目录时取版本号最大的一个，与 Cloak 自身"用最新缓存"的行为一致。
		if best == "" || compareDottedVersion(version, best) > 0 {
			best = version
		}
	}
	return best
}

// compareDottedVersion 按数值逐段比较点分版本号，返回 -1 / 0 / 1。
func compareDottedVersion(left string, right string) int {
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	maxLen := len(leftParts)
	if len(rightParts) > maxLen {
		maxLen = len(rightParts)
	}
	for i := 0; i < maxLen; i++ {
		leftValue := dottedVersionSegment(leftParts, i)
		rightValue := dottedVersionSegment(rightParts, i)
		if leftValue != rightValue {
			if leftValue > rightValue {
				return 1
			}
			return -1
		}
	}
	return 0
}

func dottedVersionSegment(parts []string, index int) int {
	if index >= len(parts) {
		return 0
	}
	value := 0
	for _, char := range parts[index] {
		if char < '0' || char > '9' {
			return value
		}
		value = value*10 + int(char-'0')
		// 防御异常长数字段导致溢出
		if value > 1<<30 {
			return 1 << 30
		}
	}
	return value
}
