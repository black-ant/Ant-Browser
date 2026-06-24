// +build !windows

package fileutil

import (
	"os"
)

// SecureFileWrite 在Unix系统上写入文件并设置0600权限（仅所有者可读写）。
func SecureFileWrite(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}
