package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

// NormalizePathInput standardizes separators from user/config input before
// path resolution so Windows-style relative paths still work on Linux/macOS.
func NormalizePathInput(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}

	p = strings.ReplaceAll(p, `\`, string(filepath.Separator))
	p = strings.ReplaceAll(p, `/`, string(filepath.Separator))
	cleaned := filepath.Clean(p)
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func ResolveUserDataDir(appPathResolver func(string) string, userDataRoot string, userDataDir string) (string, error) {
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		return "", fmt.Errorf("用户数据目录不能为空")
	}
	if filepath.IsAbs(userDataDir) {
		return userDataDir, nil
	}

	root := strings.TrimSpace(userDataRoot)
	if root == "" {
		root = "data"
	}
	if appPathResolver != nil {
		root = appPathResolver(root)
	}
	return filepath.Join(root, userDataDir), nil
}

func ResolveExistingPath(appPathResolver func(string) string, inputPath string, emptyMessage string) (string, error) {
	inputPath = strings.TrimSpace(inputPath)
	if inputPath == "" {
		return "", errors.New(emptyMessage)
	}
	if filepath.IsAbs(inputPath) {
		return inputPath, nil
	}
	if appPathResolver != nil {
		return appPathResolver(inputPath), nil
	}
	return inputPath, nil
}

// ValidateExecutable checks whether a file is runnable on the current platform.
func ValidateExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("目标是目录，不是可执行文件")
	}
	if goruntime.GOOS == "windows" {
		return nil
	}
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("文件缺少执行权限")
	}
	return nil
}

// EnsureExecutable attempts to repair missing execute bits on non-Windows
// systems so repository-pinned runtime binaries can run from source checkout.
func EnsureExecutable(path string) error {
	if goruntime.GOOS == "windows" {
		return ValidateExecutable(path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("目标是目录，不是可执行文件")
	}
	if info.Mode()&0o111 != 0 {
		return nil
	}

	nextMode := info.Mode() | 0o111
	if err := os.Chmod(path, nextMode); err != nil {
		return fmt.Errorf("补充执行权限失败: %w", err)
	}
	return nil
}
