//go:build windows

package cdp

import "errors"

// ErrPipeUnsupported 表示当前平台无法使用 --remote-debugging-pipe。
// Windows 上 Go 的 os/exec 不支持把句柄映射成子进程的 fd 3/4（Chrome 固定从 fd 3/4
// 读写 CDP 管道），需绕过 os/exec 手工拼 MSVCRT lpReserved2 句柄表，代价极高且未文档化。
// 因此 Windows 直接返回该错误，由调用方回退到调试端口（端口仅绑 127.0.0.1，网页无法探测）。
var ErrPipeUnsupported = errors.New("--remote-debugging-pipe 在 Windows 上不受支持，回退调试端口")

func NewDebugPipe() (*DebugPipe, error) {
	return nil, ErrPipeUnsupported
}
