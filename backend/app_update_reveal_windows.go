//go:build windows

package backend

import (
	"fmt"
	"os/exec"
)

// revealFileInFileManager 在资源管理器中定位文件（打开所在目录并选中）。
// explorer /select, 即使执行成功也常返回非 0 退出码，因此只校验是否成功启动。
func revealFileInFileManager(path string) error {
	if err := exec.Command("explorer.exe", "/select,", path).Start(); err != nil {
		return fmt.Errorf("打开文件位置失败: %w", err)
	}
	return nil
}
