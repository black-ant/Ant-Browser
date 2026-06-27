//go:build windows

package browser

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// readExecutableProductVersion 读取 Windows 可执行文件版本资源里的固定版本信息
// （VS_FIXEDFILEINFO），返回形如 "144.0.7559.96" 的版本号；读取失败返回空串
// （由调用方回退到 manifest）。
//
// 反检测要点：内核 UA 必须用「真实二进制版本」拼装。manifest 文件名可能与实际 chrome.exe
// 漂移，用它拼 UA 会让 navigator.userAgent 与 navigator.userAgentData（Client Hints，
// 照实报告二进制版本）不一致，被检测站判为「更改了 UserAgent」。chrome.exe 的版本资源
// 直接来自真实二进制，是最可靠来源。
func readExecutableProductVersion(exePath string) string {
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		return ""
	}

	var zeroHandle windows.Handle
	size, err := windows.GetFileVersionInfoSize(exePath, &zeroHandle)
	if err != nil || size == 0 {
		return ""
	}

	buf := make([]byte, size)
	if err := windows.GetFileVersionInfo(exePath, 0, size, unsafe.Pointer(&buf[0])); err != nil {
		return ""
	}

	var fixed *windows.VS_FIXEDFILEINFO
	var fixedLen uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&buf[0]), `\`, unsafe.Pointer(&fixed), &fixedLen); err != nil || fixed == nil {
		return ""
	}

	// 优先用产品版本，回退文件版本（Chrome 两者一致）。每段为 16 位。
	ms, ls := fixed.ProductVersionMS, fixed.ProductVersionLS
	if ms == 0 && ls == 0 {
		ms, ls = fixed.FileVersionMS, fixed.FileVersionLS
	}
	if ms == 0 && ls == 0 {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d.%d", ms>>16&0xffff, ms&0xffff, ls>>16&0xffff, ls&0xffff)
}
