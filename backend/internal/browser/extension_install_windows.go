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
	defer func() { _ = restoreRegistry() }()

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
			return installedRuntimeID, nil
		}
		select {
		case processErr := <-waitResult:
			if processErr != nil {
				return "", fmt.Errorf("浏览器安装插件进程失败: %w", processErr)
			}
			if installedRuntimeID, findErr := findInstalledRuntimeExtensionID(userDataDir, extension); findErr == nil && installedRuntimeID != "" {
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
