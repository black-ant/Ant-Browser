//go:build windows
// +build windows

package browser

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows/registry"
)

const externalExtensionRegistryRoot = `Software\Google\Chrome\Extensions`

var externalExtensionInstallMutex sync.Mutex

type externalExtensionRegistryState struct {
	keyPath       string
	existed       bool
	previousPath  string
	pathExists    bool
	previousVer   string
	versionExists bool
}

func installCRXIntoProfile(userDataDir string, chromeBinaryPath string, packagePath string, extension Extension) (string, error) {
	externalExtensionInstallMutex.Lock()
	defer externalExtensionInstallMutex.Unlock()

	runtimeID, err := runtimeExtensionIDFromPackage(packagePath, extension)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(extension.Version) == "" {
		return "", fmt.Errorf("插件版本为空，无法注册外部安装")
	}
	restoreRegistry, err := registerExternalExtension(runtimeID, packagePath, extension.Version)
	if err != nil {
		return "", err
	}
	keepRegistry := false
	defer func() {
		if !keepRegistry {
			_ = restoreRegistry()
		}
	}()

	command := exec.Command(
		chromeBinaryPath,
		"--user-data-dir="+userDataDir,
		"--no-first-run",
		"--no-default-browser-check",
		"about:blank",
	)
	command.Dir = filepath.Dir(chromeBinaryPath)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	hideExtensionInstallerWindow(command)
	if err := command.Start(); err != nil {
		return "", fmt.Errorf("启动插件安装进程失败: %w", err)
	}
	waitResult := make(chan error, 1)
	go func() { waitResult <- command.Wait() }()

	deadline := time.Now().Add(extensionInstallerTimeout)
	for time.Now().Before(deadline) {
		if installedRuntimeID, findErr := findInstalledRuntimeExtensionID(userDataDir, extension); findErr == nil && installedRuntimeID != "" {
			terminateExtensionInstallerProcess(command)
			<-waitResult
			keepRegistry = true
			return installedRuntimeID, nil
		}
		select {
		case processErr := <-waitResult:
			if processErr != nil {
				return "", fmt.Errorf("浏览器安装插件进程失败: %w", processErr)
			}
			if installedRuntimeID, findErr := findInstalledRuntimeExtensionID(userDataDir, extension); findErr == nil && installedRuntimeID != "" {
				keepRegistry = true
				return installedRuntimeID, nil
			}
			return "", fmt.Errorf("浏览器安装进程结束，但未发现持久插件目录")
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}

	terminateExtensionInstallerProcess(command)
	<-waitResult
	return "", fmt.Errorf("等待浏览器完成插件安装超时")
}

func ensurePersistentExternalExtensionRegistry(runtimeID string, packagePath string, version string) error {
	externalExtensionInstallMutex.Lock()
	defer externalExtensionInstallMutex.Unlock()
	return ensureExternalExtensionRegistry(runtimeID, packagePath, version)
}

func ensureExternalExtensionRegistry(runtimeID string, packagePath string, version string) error {
	runtimeID = NormalizeExtensionID(runtimeID)
	packagePath = strings.TrimSpace(packagePath)
	version = strings.TrimSpace(version)
	if runtimeID == "" || packagePath == "" || version == "" {
		return fmt.Errorf("external extension registration arguments are incomplete")
	}

	keyPath := filepath.Join(externalExtensionRegistryRoot, runtimeID)
	key, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err == registry.ErrNotExist {
		key, _, err = registry.CreateKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	}
	if err != nil {
		return fmt.Errorf("open persistent external extension registration: %w", err)
	}
	defer key.Close()

	existingPath, pathExists, err := readExternalExtensionValue(key, "path")
	if err != nil {
		return fmt.Errorf("read persistent external extension registration: %w", err)
	}
	if pathExists && !sameExternalExtensionPath(existingPath, packagePath) {
		return fmt.Errorf("external extension id is already registered to another path: %s", runtimeID)
	}
	if err := key.SetStringValue("path", packagePath); err != nil {
		return fmt.Errorf("write external extension path: %w", err)
	}
	if err := key.SetStringValue("version", version); err != nil {
		return fmt.Errorf("write external extension version: %w", err)
	}
	return nil
}

func sameExternalExtensionPath(left string, right string) bool {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	if left == "" || right == "" {
		return false
	}
	return strings.EqualFold(left, right)
}

func registerExternalExtension(runtimeID string, packagePath string, version string) (func() error, error) {
	keyPath := externalExtensionRegistryRoot + `\` + runtimeID
	key, existed, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return nil, fmt.Errorf("创建外部插件注册表项失败: %w", err)
	}
	state := externalExtensionRegistryState{keyPath: keyPath, existed: existed}
	state.previousPath, state.pathExists, err = readExternalExtensionValue(key, "path")
	if err == nil {
		state.previousVer, state.versionExists, err = readExternalExtensionValue(key, "version")
	}
	_ = key.Close()
	if err != nil {
		_ = restoreExternalExtensionRegistry(state)
		return nil, fmt.Errorf("读取外部插件注册表项失败: %w", err)
	}

	if state.pathExists && !sameExternalExtensionPath(state.previousPath, packagePath) {
		_ = restoreExternalExtensionRegistry(state)
		return nil, fmt.Errorf("external extension id is already registered to another path: %s", runtimeID)
	}
	key, _, err = registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
	if err != nil {
		_ = restoreExternalExtensionRegistry(state)
		return nil, fmt.Errorf("打开外部插件注册表项失败: %w", err)
	}
	if err := key.SetStringValue("path", packagePath); err == nil {
		err = key.SetStringValue("version", version)
	}
	_ = key.Close()
	if err != nil {
		_ = restoreExternalExtensionRegistry(state)
		return nil, fmt.Errorf("写入外部插件注册表项失败: %w", err)
	}
	return func() error { return restoreExternalExtensionRegistry(state) }, nil
}

func readExternalExtensionValue(key registry.Key, name string) (string, bool, error) {
	value, _, err := key.GetStringValue(name)
	if err == registry.ErrNotExist {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func restoreExternalExtensionRegistry(state externalExtensionRegistryState) error {
	if !state.existed {
		if err := registry.DeleteKey(registry.CURRENT_USER, state.keyPath); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, state.keyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	var firstErr error
	if state.pathExists {
		if err := key.SetStringValue("path", state.previousPath); err != nil && firstErr == nil {
			firstErr = err
		}
	} else if err := key.DeleteValue("path"); err != nil && err != registry.ErrNotExist && firstErr == nil {
		firstErr = err
	}
	if state.versionExists {
		if err := key.SetStringValue("version", state.previousVer); err != nil && firstErr == nil {
			firstErr = err
		}
	} else if err := key.DeleteValue("version"); err != nil && err != registry.ErrNotExist && firstErr == nil {
		firstErr = err
	}
	if err := key.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

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
