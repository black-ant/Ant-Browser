package backend

import (
	"strings"
)

// browserProcessLanguageEnv 根据 --lang 推导浏览器进程的 LANGUAGE 环境变量。
//
// Chromium 在 Linux 上的 Intl 默认 locale（Intl.DateTimeFormat().resolvedOptions().locale、
// Date#toLocaleString 等）跟随进程环境而不是 --lang；只传 --lang=en-US 时，
// navigator.language 是 en-US，Intl 仍然是宿主桌面的 zh-CN，形成可检测的语言不一致。
// LANGUAGE 不依赖系统已安装的 glibc locale，因此比 LC_ALL 更安全。
// Windows 和 macOS 的 locale 来自系统 API，不受该变量影响，返回 nil。
func browserProcessLanguageEnv(hostOS string, args []string) []string {
	if strings.ToLower(strings.TrimSpace(hostOS)) != "linux" {
		return nil
	}
	lang := browserArgValue(args, browserLangArg)
	if lang == "" {
		return nil
	}
	return []string{"LANGUAGE=" + posixLanguageList(lang)}
}

// posixLanguageList 把 BCP 47 语言标签转成 gettext 风格的 LANGUAGE 列表：
// en-US -> en_US:en，zh-CN -> zh_CN:zh，de -> de。
func posixLanguageList(lang string) string {
	posix := strings.ReplaceAll(strings.TrimSpace(lang), "-", "_")
	base := strings.SplitN(posix, "_", 2)[0]
	if base == "" || base == posix {
		return posix
	}
	return posix + ":" + base
}

// browserProcessEnv 在 base 环境上覆盖 overrides 中的同名变量。
// overrides 为空时返回 nil，让 exec.Cmd 沿用父进程环境。
func browserProcessEnv(base []string, overrides []string) []string {
	if len(overrides) == 0 {
		return nil
	}
	overridden := make(map[string]struct{}, len(overrides))
	for _, entry := range overrides {
		overridden[envKey(entry)] = struct{}{}
	}
	env := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		if _, replaced := overridden[envKey(entry)]; replaced {
			continue
		}
		env = append(env, entry)
	}
	return append(env, overrides...)
}

func envKey(entry string) string {
	if index := strings.Index(entry, "="); index >= 0 {
		return entry[:index]
	}
	return entry
}
