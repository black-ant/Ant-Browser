//go:build !windows

package backend

import (
	"fmt"
	"os/exec"
)

func applyBrowserProcessMemoryLimit(_ *exec.Cmd, memoryLimitMB int) (func(), error) {
	if memoryLimitMB <= 0 {
		return nil, nil
	}
	return nil, fmt.Errorf("当前系统不支持实例内存硬限制")
}
