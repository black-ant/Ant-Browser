//go:build !windows

package backend

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// revealFileInFileManager 在系统文件管理器中定位文件。
func revealFileInFileManager(path string) error {
	if runtime.GOOS == "darwin" {
		if err := exec.Command("open", "-R", path).Start(); err != nil {
			return fmt.Errorf("打开文件位置失败: %w", err)
		}
		return nil
	}
	if err := exec.Command("xdg-open", filepath.Dir(path)).Start(); err != nil {
		return fmt.Errorf("打开文件位置失败: %w", err)
	}
	return nil
}
