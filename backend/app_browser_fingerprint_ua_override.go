package backend

import (
	"strings"

	"ant-chrome/backend/internal/identity"
)

// overrideUAToKernelVersion 把 launchArgs 中的 --user-agent 与 --fingerprint-brand-version
// 覆盖为内核真实大版本,保证 UA / UA-CH 与实际运行引擎一致(权威层)。
//
// 身份池里烘焙的 UA 版本非权威:所有实例共享同一引擎,UA 必须跟随该实例实际内核的版本,
// 否则"UA 版本 ≠ 引擎能力"会被高级风控判为版本撒谎。以内核版本为准还带来:换内核(148→149)
// UA 自动跟随、无需重生成身份;即使身份/池数据陈旧也不会漂移。
//
// chromeVersion 无法识别(major<=0)时不改动——宁可保留身份原值,也不用错误版本覆盖。
// 仅重写已存在的对应参数,不新增(无 UA 参数的实例交由内核默认 UA,其版本本就是内核版本)。
func overrideUAToKernelVersion(args []string, chromeVersion string) []string {
	major := parseChromeMajor(chromeVersion)
	if major <= 0 || len(args) == 0 {
		return args
	}
	platform := fingerprintPlatformFromArgs(args)
	ua := identity.BuildReducedUA(platform, major)
	brand := identity.BrandVersionForMajor(major)

	out := make([]string, len(args))
	copy(out, args)
	for i, a := range out {
		switch {
		case ua != "" && strings.HasPrefix(a, "--user-agent="):
			out[i] = "--user-agent=" + ua
		case brand != "" && strings.HasPrefix(a, "--fingerprint-brand-version="):
			out[i] = "--fingerprint-brand-version=" + brand
		}
	}
	return out
}

// fingerprintPlatformFromArgs 从启动参数解析指纹平台(--fingerprint-platform=),
// 缺失时回退按 --user-agent 里的 OS 标记推断,默认 windows。
func fingerprintPlatformFromArgs(args []string) string {
	for _, a := range args {
		if strings.HasPrefix(a, "--fingerprint-platform=") {
			if v := strings.TrimSpace(strings.TrimPrefix(a, "--fingerprint-platform=")); v != "" {
				return v
			}
		}
	}
	for _, a := range args {
		if strings.HasPrefix(a, "--user-agent=") {
			if strings.Contains(a, "Mac OS X") || strings.Contains(a, "Macintosh") {
				return "macos"
			}
		}
	}
	return "windows"
}
