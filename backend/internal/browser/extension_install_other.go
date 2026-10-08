//go:build !windows
// +build !windows

package browser

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// externalExtensionRegistration 是 Chromium 在 Linux/macOS 上识别外部安装插件的清单内容，
// 对应 Windows 上的注册表项 Software\Google\Chrome\Extensions\<插件 ID>。
type externalExtensionRegistration struct {
	ExternalCRX     string `json:"external_crx"`
	ExternalVersion string `json:"external_version"`
}

func installCRXIntoProfile(userDataDir string, chromeBinaryPath string, packagePath string, extension Extension, installArgs []string) (string, error) {
	runtimeID, err := runtimeExtensionIDFromPackage(packagePath, extension)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(extension.Version) == "" {
		return "", fmt.Errorf("插件版本为空，无法注册外部安装")
	}
	// 清单必须在浏览器启动前写入，浏览器才会在本次启动时按外部安装清单落地插件。
	restoreRegistration, err := registerExternalExtension(userDataDir, runtimeID, packagePath, extension.Version)
	if err != nil {
		return "", err
	}
	keepRegistration := false
	defer func() {
		if !keepRegistration {
			_ = restoreRegistration()
		}
	}()

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
			keepRegistration = true
			return runtimeExtensionID, nil
		}
		select {
		case processErr := <-waitResult:
			if processErr != nil {
				return "", fmt.Errorf("浏览器安装插件进程失败: %w", processErr)
			}
			if runtimeExtensionID, err := findInstalledRuntimeExtensionID(userDataDir, extension); err == nil && runtimeExtensionID != "" {
				keepRegistration = true
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

// ensurePersistentExternalExtensionRegistry 在非 Windows 平台上把插件注册为 profile 级外部安装，
// 使浏览器每次启动都能从插件包重新落地插件代码。
func ensurePersistentExternalExtensionRegistry(userDataDir string, runtimeID string, packagePath string, version string) error {
	runtimeID = NormalizeExtensionID(runtimeID)
	packagePath = strings.TrimSpace(packagePath)
	version = strings.TrimSpace(version)
	if runtimeID == "" || packagePath == "" || version == "" {
		return fmt.Errorf("external extension registration arguments are incomplete")
	}
	registrationPath, err := externalExtensionRegistrationPath(userDataDir, runtimeID)
	if err != nil {
		return err
	}
	absolutePackagePath, err := filepath.Abs(packagePath)
	if err != nil {
		return fmt.Errorf("resolve external extension package path: %w", err)
	}
	if existing, readErr := readExternalExtensionRegistration(registrationPath); readErr == nil && existing != nil {
		if !sameExternalExtensionPackagePath(existing.ExternalCRX, absolutePackagePath) {
			return fmt.Errorf("external extension id is already registered to another path: %s", runtimeID)
		}
	}
	payload, err := json.Marshal(externalExtensionRegistration{ExternalCRX: absolutePackagePath, ExternalVersion: version})
	if err != nil {
		return fmt.Errorf("encode external extension registration: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(registrationPath), 0o755); err != nil {
		return fmt.Errorf("create external extension registration directory: %w", err)
	}
	if err := os.WriteFile(registrationPath, payload, 0o644); err != nil {
		return fmt.Errorf("write external extension registration: %w", err)
	}
	return nil
}

func externalExtensionRegistrationPath(userDataDir string, runtimeID string) (string, error) {
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		return "", fmt.Errorf("external extension registration requires a user data directory")
	}
	return filepath.Join(userDataDir, "External Extensions", NormalizeExtensionID(runtimeID)+".json"), nil
}

func readExternalExtensionRegistration(registrationPath string) (*externalExtensionRegistration, error) {
	data, err := os.ReadFile(registrationPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var registration externalExtensionRegistration
	if err := json.Unmarshal(data, &registration); err != nil {
		return nil, err
	}
	return &registration, nil
}

func sameExternalExtensionPackagePath(left string, right string) bool {
	left = filepath.Clean(filepath.FromSlash(strings.TrimSpace(left)))
	right = filepath.Clean(filepath.FromSlash(strings.TrimSpace(right)))
	if left == "." || right == "." {
		return false
	}
	return strings.EqualFold(left, right)
}

// registerExternalExtension 写入外部安装清单，并在安装失败时恢复原有清单。
func registerExternalExtension(userDataDir string, runtimeID string, packagePath string, version string) (func() error, error) {
	registrationPath, err := externalExtensionRegistrationPath(userDataDir, runtimeID)
	if err != nil {
		return nil, err
	}
	previousData, readErr := os.ReadFile(registrationPath)
	previousExists := readErr == nil
	if readErr != nil && !os.IsNotExist(readErr) {
		return nil, fmt.Errorf("read external extension registration: %w", readErr)
	}
	if err := ensurePersistentExternalExtensionRegistry(userDataDir, runtimeID, packagePath, version); err != nil {
		return nil, err
	}
	return func() error {
		if previousExists {
			return os.WriteFile(registrationPath, previousData, 0o644)
		}
		if err := os.Remove(registrationPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}, nil
}
