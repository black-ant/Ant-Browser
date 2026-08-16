package identity

import (
	"fmt"
	"strings"
)

// LaunchArgs 将身份序列化为 fingerprint-chromium 可识别的启动参数。
//
// 只产出内核在 Chrome 144+ 仍接受的 flag;WebGL vendor/renderer、screen、
// device-memory、geolocation 等已废弃项不在此产出——它们由 seed 内部派生,
// 地理定位则在启动后经 CDP Emulation.setGeolocationOverride 注入。
func (i Identity) LaunchArgs() []string {
	var args []string
	if i.Seed != 0 {
		args = append(args, fmt.Sprintf("--fingerprint=%d", i.Seed))
	}
	if i.Platform != "" {
		args = append(args, "--fingerprint-platform="+i.Platform)
	}
	if i.PlatformVersion != "" {
		args = append(args, "--fingerprint-platform-version="+i.PlatformVersion)
	}
	if i.BrowserBrand != "" {
		args = append(args, "--fingerprint-brand="+i.BrowserBrand)
	}
	if i.BrandVersion != "" {
		args = append(args, "--fingerprint-brand-version="+i.BrandVersion)
	}
	// UA 字符串必须与 brand-version(Client Hints)同版本,否则 navigator.userAgent
	// 用的是内核真实版本、而 Sec-CH-UA 是身份版本,两者矛盾易被检测。下发完整 UA 覆盖
	// 内核默认 UA,使所有服务端可采集的版本面(UA / Sec-CH-UA / fullVersionList /
	// uaFullVersion)统一到身份版本 —— 实现"版本多样性且自洽"。
	if i.UAFull != "" {
		args = append(args, "--user-agent="+i.UAFull)
	}
	if i.HardwareConcurrency > 0 {
		args = append(args, fmt.Sprintf("--fingerprint-hardware-concurrency=%d", i.HardwareConcurrency))
	}
	if i.WindowSize != "" {
		args = append(args, "--window-size="+i.WindowSize)
	}
	if i.Locale != "" {
		args = append(args, "--lang="+i.Locale)
	}
	if len(i.Languages) > 0 {
		args = append(args, "--accept-lang="+strings.Join(i.Languages, ","))
	}
	if i.Timezone != "" {
		args = append(args, "--timezone="+i.Timezone)
	}
	if i.CanvasNoise {
		args = append(args, "--fingerprinting-canvas-image-data-noise")
	}
	if i.ClientRectsNoise {
		args = append(args, "--fingerprinting-client-rects-noise")
	}
	if i.WebRTCPolicy == "disable_non_proxied_udp" {
		args = append(args, "--disable-non-proxied-udp")
	}
	return args
}
