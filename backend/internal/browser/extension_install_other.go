//go:build !windows
// +build !windows

package browser

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"
)

func installCRXIntoProfile(userDataDir string, chromeBinaryPath string, packagePath string, extension Extension, installArgs []string) (string, error) {
	commandArgs := []string{
		"--user-data-dir=" + userDataDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--install-extension=" + packagePath,
	}
	commandArgs = append(commandArgs, installArgs...)
	commandArgs = append(commandArgs, "about:blank")
	command := exec.Command(chromeBinaryPath, commandArgs...)
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
		if runtimeExtensionID, err := findInstalledRuntimeExtensionID(userDataDir, extension); err == nil && runtimeExtensionID != "" {
			terminateExtensionInstallerProcess(command)
			<-waitResult
			return runtimeExtensionID, nil
		}
		select {
		case processErr := <-waitResult:
			if processErr != nil {
				return "", fmt.Errorf("浏览器安装插件进程失败: %w", processErr)
			}
			if runtimeExtensionID, err := findInstalledRuntimeExtensionID(userDataDir, extension); err == nil && runtimeExtensionID != "" {
				return runtimeExtensionID, nil
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

func browserExtensionInstallerExitHint(_ string) string {
	return ""
}

func (m *Manager) cleanupManagedExternalExtensionRegistry() error {
	return nil
}

func ensurePersistentExternalExtensionRegistry(runtimeID string, packagePath string, version string) error {
	return nil
}
