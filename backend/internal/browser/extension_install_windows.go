//go:build windows
// +build windows

package browser

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const externalExtensionRegistryRoot = `Software\Google\Chrome\Extensions`

func (m *Manager) cleanupManagedExternalExtensionRegistry() error {
	if m == nil {
		return nil
	}
	managedRoot, err := filepath.Abs(m.ResolveRelativePath(filepath.Join("data", extensionsRootDir)))
	if err != nil {
		return fmt.Errorf("解析插件管理目录失败: %w", err)
	}
	managedRoot = filepath.Clean(managedRoot)
	rootKey, err := registry.OpenKey(registry.CURRENT_USER, externalExtensionRegistryRoot, registry.READ)
	if err == registry.ErrNotExist {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取旧插件注册表失败: %w", err)
	}
	defer rootKey.Close()
	keyNames, err := rootKey.ReadSubKeyNames(-1)
	if err != nil {
		return fmt.Errorf("枚举旧插件注册表失败: %w", err)
	}
	for _, keyName := range keyNames {
		key, openErr := registry.OpenKey(rootKey, keyName, registry.QUERY_VALUE)
		if openErr != nil {
			if openErr == registry.ErrNotExist {
				continue
			}
			return fmt.Errorf("读取旧插件注册表项失败（%s）: %w", keyName, openErr)
		}
		pathValue, _, valueErr := key.GetStringValue("path")
		_ = key.Close()
		if valueErr == registry.ErrNotExist {
			continue
		}
		if valueErr != nil {
			return fmt.Errorf("读取旧插件路径失败（%s）: %w", keyName, valueErr)
		}
		if !isPathWithinRoot(managedRoot, pathValue) {
			continue
		}
		keyPath := externalExtensionRegistryRoot + `\` + keyName
		if deleteErr := registry.DeleteKey(registry.CURRENT_USER, keyPath); deleteErr != nil && deleteErr != registry.ErrNotExist {
			return fmt.Errorf("清理旧插件注册表项失败（%s）: %w", keyName, deleteErr)
		}
	}
	return nil
}

func isPathWithinRoot(rootPath string, targetPath string) bool {
	rootPath = filepath.Clean(strings.TrimSpace(rootPath))
	targetPath = strings.TrimSpace(targetPath)
	if rootPath == "" || targetPath == "" {
		return false
	}
	absoluteTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}
	relativePath, err := filepath.Rel(rootPath, filepath.Clean(absoluteTarget))
	if err != nil || relativePath == "." || relativePath == ".." {
		return false
	}
	return !strings.HasPrefix(relativePath, ".."+string(filepath.Separator))
}
