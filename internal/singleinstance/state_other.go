//go:build !windows

package singleinstance

import (
	"ant-chrome/backend"
	"path/filepath"
	"strings"
)

func singleInstanceStateRoot(appRoot string) string {
	if root := strings.TrimSpace(backend.RuntimeStateRoot(appRoot)); root != "" {
		return root
	}
	return filepath.Join(appRoot, "data")
}
