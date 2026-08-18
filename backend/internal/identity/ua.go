package identity

import (
	"fmt"
	"strconv"
	"strings"
)

// UA / Client-Hints 版本必须与实际运行的内核引擎版本一致。
//
// 背景:所有实例共享同一个 fingerprint-chromium 引擎二进制,若 UA 声称的版本与引擎不符
// (如"145 UA + 148 引擎"),普通网站看 UA 无从察觉,但高级风控会交叉验证 UA 声称的版本与
// 引擎实际暴露的 JS/CSS/V8 能力,判出版本撒谎并静默降低账号信任分。因此"版本多样性"只能来自
// **真实内置的多个内核版本**(如 148/144),UA 由实例实际运行的内核版本驱动,而非伪造字符串。
//
// Chrome UA Reduction:自 Chrome 110+,navigator.userAgent 的版本恒为 `<major>.0.0.0`
// (次版本被抹平),同平台同大版本的真实用户 UA 完全相同——所以统一到内核大版本不但不异常,
// 反而最贴近真实人群;唯一性由硬件/屏幕/时区/Canvas 等其它维度提供。

// BuildReducedUA 返回指定平台 + 大版本的 Chrome UA Reduction 形式 UA 字符串。
// platform 取 "macos" 时用 Mac 形态,其余(含 "windows"、空、未知)用 Windows 形态。
func BuildReducedUA(platform string, major int) string {
	if major <= 0 {
		return ""
	}
	chrome := fmt.Sprintf("Chrome/%d.0.0.0", major)
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "macos", "mac", "darwin":
		return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) " + chrome + " Safari/537.36"
	default:
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) " + chrome + " Safari/537.36"
	}
}

// BrandVersionForMajor 返回大版本对应的 Client-Hints brandVersion(Reduction 形式 <major>.0.0.0)。
func BrandVersionForMajor(major int) string {
	if major <= 0 {
		return ""
	}
	return fmt.Sprintf("%d.0.0.0", major)
}

// MajorFromVersion 从版本串(如 "148" 或 "148.0.7778.215")解析大版本;无法解析返回 0。
func MajorFromVersion(version string) int {
	version = strings.TrimSpace(version)
	if version == "" {
		return 0
	}
	head := version
	if i := strings.IndexByte(version, '.'); i >= 0 {
		head = version[:i]
	}
	n, err := strconv.Atoi(strings.TrimSpace(head))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// ApplyKernelVersion 把身份的 UAFull / BrandVersion 按内核大版本重写(平台取身份自身 Platform)。
// major<=0 时不改动。用于新建实例分配身份时,让 DB/UI 的版本面预先与内核一致
// (启动时还有一层权威覆盖,见 backend.overrideUAToKernelVersion)。
func ApplyKernelVersion(id Identity, major int) Identity {
	if major <= 0 {
		return id
	}
	if ua := BuildReducedUA(id.Platform, major); ua != "" {
		id.UAFull = ua
	}
	id.BrandVersion = BrandVersionForMajor(major)
	return id
}
