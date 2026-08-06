package browser

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	extensionInstallerTimeout = 20 * time.Second
	extensionBackupRoot       = "extension-backups"
)

type extensionLegacySetting struct {
	Location int    `json:"location"`
	Path     string `json:"path"`
}

type extensionLegacyPreferences struct {
	Extensions struct {
		Settings map[string]extensionLegacySetting `json:"settings"`
	} `json:"extensions"`
}

func (m *Manager) PrepareProfileExtensions(profile *Profile, chromeBinaryPath string, userDataDir string) ([]string, error) {
	if m == nil || m.ExtensionDAO == nil || profile == nil {
		return nil, nil
	}
	if strings.TrimSpace(chromeBinaryPath) == "" {
		return nil, fmt.Errorf("插件持久安装失败：浏览器内核路径为空")
	}
	if strings.TrimSpace(userDataDir) == "" {
		return nil, fmt.Errorf("插件持久安装失败：实例数据目录为空")
	}

	settings, err := m.ExtensionDAO.GetProfileSettings(profile.ProfileId)
	if err != nil {
		return nil, err
	}
	var extensions []Extension
	if settings.Configured {
		extensions, err = m.ExtensionDAO.ListByIDs(settings.ExtensionIDs)
	} else {
		extensions, err = m.ExtensionDAO.ListEnabled()
	}
	if err != nil {
		return nil, err
	}

	desired := make(map[string]Extension, len(extensions))
	commandlineDirs := make([]string, 0, len(extensions))
	for _, extension := range extensions {
		extension.InstallMode = normalizeExtensionInstallMode(extension.InstallMode)
		desired[extension.ExtensionID] = extension
		if extension.InstallMode == ExtensionInstallModeCommandline {
			dir := strings.TrimSpace(extension.InstallDir)
			if dir == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err == nil {
				commandlineDirs = append(commandlineDirs, dir)
			}
			continue
		}
		if err := m.ensurePersistentExtensionInstalled(profile, userDataDir, chromeBinaryPath, extension); err != nil {
			return nil, err
		}
	}

	runtimeStates, err := m.ExtensionDAO.ListProfileExtensionRuntime(profile.ProfileId)
	if err != nil {
		return nil, err
	}
	for _, runtimeState := range runtimeStates {
		if runtimeState.InstallMode != ExtensionInstallModePersistent {
			continue
		}
		if _, ok := desired[runtimeState.ExtensionID]; ok {
			continue
		}
		if strings.TrimSpace(runtimeState.RuntimeExtensionID) != "" {
			if err := removePersistentExtensionCode(userDataDir, runtimeState.RuntimeExtensionID); err != nil {
				return nil, err
			}
		}
		runtimeState.Status = ExtensionRuntimeStatusDisabled
		runtimeState.LastVerifiedAt = time.Now().Format(time.RFC3339)
		runtimeState.LastError = ""
		if err := m.ExtensionDAO.UpsertProfileExtensionRuntime(runtimeState); err != nil {
			return nil, err
		}
	}

	return normalizeNonEmptyExtensionDirs(commandlineDirs), nil
}

func (m *Manager) RemoveExtensionFromStoppedProfiles(extensionID string) error {
	if m == nil || m.ExtensionDAO == nil {
		return nil
	}
	extensionID = strings.TrimSpace(extensionID)
	if extensionID == "" {
		return nil
	}
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	extension, extensionErr := m.ExtensionDAO.Get(extensionID)
	if extensionErr != nil && extensionErr != sql.ErrNoRows {
		return extensionErr
	}
	for _, profile := range m.Profiles {
		if profile == nil {
			continue
		}
		runtimeState, runtimeErr := m.ExtensionDAO.GetProfileExtensionRuntime(profile.ProfileId, extensionID)
		if runtimeErr != nil && runtimeErr != sql.ErrNoRows {
			return runtimeErr
		}
		legacyRuntimeIDs := []string{}
		if extensionErr == nil {
			var legacyErr error
			legacyRuntimeIDs, legacyErr = findLegacyRuntimeExtensionIDs(m.ResolveUserDataDir(profile), extension.InstallDir)
			if legacyErr != nil {
				return legacyErr
			}
		}
		runtimeIDs := uniqueExtensionIDs(append(legacyRuntimeIDs, runtimeState.RuntimeExtensionID))
		if profile.Running && extensionErr == nil && normalizeExtensionInstallMode(extension.InstallMode) == ExtensionInstallModePersistent {
			if persistentRuntimeID := persistentExtensionCodeID(m.ResolveUserDataDir(profile), extension.ExtensionID); persistentRuntimeID != "" {
				return fmt.Errorf("插件仍在运行中的实例中：%s。请先停止实例后再禁用或删除", profile.ProfileName)
			}
		}
		if profile.Running && extensionErr == nil && normalizeExtensionInstallMode(extension.InstallMode) == ExtensionInstallModeCommandline {
			return fmt.Errorf("插件仍在运行中的实例中：%s。请先停止实例后再禁用或删除", profile.ProfileName)
		}
		if profile.Running && extensionErr == nil && normalizeExtensionInstallMode(extension.InstallMode) == ExtensionInstallModePersistent && runtimeErr == sql.ErrNoRows && len(legacyRuntimeIDs) == 0 {
			installedRuntimeID, findErr := findInstalledRuntimeExtensionID(m.ResolveUserDataDir(profile), extension)
			if findErr != nil {
				return findErr
			}
			if installedRuntimeID != "" {
				return fmt.Errorf("插件仍在运行中的实例中：%s。请先停止实例后再禁用或删除", profile.ProfileName)
			}
		}
		if profile.Running && (len(legacyRuntimeIDs) > 0 || (runtimeErr == nil && runtimeState.Status != ExtensionRuntimeStatusDisabled && len(runtimeIDs) > 0)) {
			return fmt.Errorf("插件仍在运行中的实例中：%s。请先停止实例后再禁用或删除", profile.ProfileName)
		}
	}
	for _, profile := range m.Profiles {
		if profile == nil || profile.Running {
			continue
		}
		runtimeState, runtimeErr := m.ExtensionDAO.GetProfileExtensionRuntime(profile.ProfileId, extensionID)
		if runtimeErr != nil && runtimeErr != sql.ErrNoRows {
			return runtimeErr
		}
		legacyRuntimeIDs := []string{}
		if extensionErr == nil {
			var legacyErr error
			legacyRuntimeIDs, legacyErr = findLegacyRuntimeExtensionIDs(m.ResolveUserDataDir(profile), extension.InstallDir)
			if legacyErr != nil {
				return legacyErr
			}
		}
		runtimeIDs := uniqueExtensionIDs(append(legacyRuntimeIDs, runtimeState.RuntimeExtensionID))
		if extensionErr == nil && normalizeExtensionInstallMode(extension.InstallMode) == ExtensionInstallModePersistent && len(runtimeIDs) == 0 {
			if persistentRuntimeID := persistentExtensionCodeID(m.ResolveUserDataDir(profile), extension.ExtensionID); persistentRuntimeID != "" {
				runtimeIDs = []string{persistentRuntimeID}
			}
		}
		for _, runtimeID := range runtimeIDs {
			if err := removePersistentExtensionCode(m.ResolveUserDataDir(profile), runtimeID); err != nil {
				return err
			}
		}
		if runtimeErr == sql.ErrNoRows && len(runtimeIDs) == 0 {
			continue
		}
		if runtimeErr == sql.ErrNoRows {
			runtimeState = ProfileExtensionRuntime{
				ProfileID:        profile.ProfileId,
				ExtensionID:      extensionID,
				InstallMode:      ExtensionInstallModePersistent,
				InstalledVersion: extension.Version,
				PackageHash:      extension.PackageHash,
			}
		}
		runtimeState.Status = ExtensionRuntimeStatusDisabled
		runtimeState.LastVerifiedAt = time.Now().Format(time.RFC3339)
		runtimeState.LastError = ""
		if len(runtimeIDs) > 0 {
			runtimeState.RuntimeExtensionID = runtimeIDs[0]
		}
		if err := m.ExtensionDAO.UpsertProfileExtensionRuntime(runtimeState); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) RemoveExtensionPackageFiles(extension Extension) error {
	paths, err := m.ExtensionPackagePaths(extension)
	if err != nil {
		return err
	}
	for _, targetPath := range paths {
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除插件包失败: %w", err)
		}
	}
	return nil
}

func (m *Manager) ExtensionPackagePaths(extension Extension) ([]string, error) {
	paths := make([]string, 0, 2)
	packagePath := strings.TrimSpace(extension.PackagePath)
	if packagePath != "" {
		if !filepath.IsAbs(packagePath) {
			packagePath = m.ResolveRelativePath(packagePath)
		}
		paths = append(paths, packagePath)
		paths = append(paths, strings.TrimSuffix(packagePath, filepath.Ext(packagePath))+".pem")
	}
	root, err := filepath.Abs(m.ResolveRelativePath(filepath.Join("data", extensionsRootDir, "packages")))
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	validated := make([]string, 0, len(paths))
	for _, targetPath := range paths {
		if strings.TrimSpace(targetPath) == "" {
			continue
		}
		absolutePath, err := filepath.Abs(targetPath)
		if err != nil {
			return nil, err
		}
		relativePath, err := filepath.Rel(root, filepath.Clean(absolutePath))
		if err != nil || relativePath == "." || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("拒绝删除插件包根目录外的路径: %s", targetPath)
		}
		duplicate := false
		for _, existingPath := range validated {
			if strings.EqualFold(existingPath, filepath.Clean(absolutePath)) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			validated = append(validated, filepath.Clean(absolutePath))
		}
	}
	return validated, nil
}

func (m *Manager) ensurePersistentExtensionInstalled(profile *Profile, userDataDir string, chromeBinaryPath string, extension Extension) error {
	packagePath, packageHash, err := m.resolveExtensionPackage(extension, chromeBinaryPath)
	if err != nil {
		return err
	}

	runtimeState, runtimeErr := m.ExtensionDAO.GetProfileExtensionRuntime(profile.ProfileId, extension.ExtensionID)
	if runtimeErr != nil && runtimeErr != sql.ErrNoRows {
		return runtimeErr
	}
	if runtimeErr == nil && runtimeState.Status == ExtensionRuntimeStatusInstalled &&
		runtimeState.InstalledVersion == extension.Version && runtimeState.PackageHash == packageHash &&
		persistentExtensionArtifactMatches(userDataDir, runtimeState.RuntimeExtensionID, extension.Version) {
		runtimeState.LastVerifiedAt = time.Now().Format(time.RFC3339)
		runtimeState.LastError = ""
		return m.ExtensionDAO.UpsertProfileExtensionRuntime(runtimeState)
	}

	legacyRuntimeIDs, err := findLegacyRuntimeExtensionIDs(userDataDir, extension.InstallDir)
	if err != nil {
		return err
	}
	if runtimeErr == nil && strings.TrimSpace(runtimeState.RuntimeExtensionID) != "" {
		legacyRuntimeIDs = append(legacyRuntimeIDs, runtimeState.RuntimeExtensionID)
	}
	legacyRuntimeIDs = uniqueExtensionIDs(legacyRuntimeIDs)

	backupPath, err := m.backupProfileExtensionState(profile.ProfileId, extension.ExtensionID, userDataDir, legacyRuntimeIDs)
	if err != nil {
		return err
	}
	runtimeExtensionID, err := installCRXIntoProfile(userDataDir, chromeBinaryPath, packagePath, extension)
	if err != nil {
		_ = restoreProfileExtensionState(userDataDir, backupPath, legacyRuntimeIDs, "")
		m.recordProfileExtensionRuntimeError(profile.ProfileId, extension, runtimeState, packageHash, backupPath, err)
		return fmt.Errorf("插件持久安装失败（%s）：%w；安装前备份已保留在 %s", extension.Name, err, backupPath)
	}

	for _, legacyRuntimeID := range legacyRuntimeIDs {
		if legacyRuntimeID == runtimeExtensionID {
			continue
		}
		if err := migrateExtensionStorage(userDataDir, legacyRuntimeID, runtimeExtensionID); err != nil {
			_ = restoreProfileExtensionState(userDataDir, backupPath, legacyRuntimeIDs, runtimeExtensionID)
			m.recordProfileExtensionRuntimeError(profile.ProfileId, extension, runtimeState, packageHash, backupPath, err)
			return fmt.Errorf("迁移插件数据失败（%s -> %s）：%w；安装前备份在 %s", legacyRuntimeID, runtimeExtensionID, err, backupPath)
		}
	}

	now := time.Now().Format(time.RFC3339)
	if runtimeErr == nil && strings.TrimSpace(runtimeState.CreatedAt) != "" {
		if backupPath == "" {
			backupPath = runtimeState.BackupPath
		}
	} else {
		runtimeState.CreatedAt = now
	}
	runtimeState.ProfileID = profile.ProfileId
	runtimeState.ExtensionID = extension.ExtensionID
	runtimeState.RuntimeExtensionID = runtimeExtensionID
	runtimeState.InstallMode = ExtensionInstallModePersistent
	runtimeState.InstalledVersion = extension.Version
	runtimeState.PackageHash = packageHash
	runtimeState.Status = ExtensionRuntimeStatusInstalled
	runtimeState.BackupPath = backupPath
	runtimeState.LastVerifiedAt = now
	runtimeState.LastError = ""
	if err := m.ExtensionDAO.UpsertProfileExtensionRuntime(runtimeState); err != nil {
		_ = restoreProfileExtensionState(userDataDir, backupPath, legacyRuntimeIDs, runtimeExtensionID)
		return err
	}
	return nil
}

func (m *Manager) recordProfileExtensionRuntimeError(profileID string, extension Extension, runtimeState ProfileExtensionRuntime, packageHash string, backupPath string, installErr error) {
	if m == nil || m.ExtensionDAO == nil {
		return
	}
	runtimeState.ProfileID = profileID
	runtimeState.ExtensionID = extension.ExtensionID
	runtimeState.InstallMode = ExtensionInstallModePersistent
	runtimeState.InstalledVersion = extension.Version
	runtimeState.PackageHash = packageHash
	runtimeState.Status = ExtensionRuntimeStatusError
	runtimeState.BackupPath = backupPath
	runtimeState.LastVerifiedAt = ""
	runtimeState.LastError = installErr.Error()
	_ = m.ExtensionDAO.UpsertProfileExtensionRuntime(runtimeState)
}

func (m *Manager) resolveExtensionPackage(extension Extension, chromeBinaryPath string) (string, string, error) {
	packagePath := strings.TrimSpace(extension.PackagePath)
	if packagePath != "" && !filepath.IsAbs(packagePath) {
		packagePath = m.ResolveRelativePath(packagePath)
	}
	if packagePath != "" {
		if packageData, err := os.ReadFile(packagePath); err == nil && isCRXExtensionPackage(packageData) {
			packageHash := extensionPackageHash(packageData)
			return packagePath, packageHash, nil
		}
	}

	installDir := strings.TrimSpace(extension.InstallDir)
	if installDir == "" {
		return "", "", fmt.Errorf("插件持久安装失败：插件目录为空（%s）", extension.ExtensionID)
	}
	if _, err := os.Stat(filepath.Join(installDir, "manifest.json")); err != nil {
		return "", "", fmt.Errorf("插件持久安装失败：插件目录不存在（%s）：%w", installDir, err)
	}
	generatedPackagePath, generatedHash, err := m.packExtensionDirectory(extension, chromeBinaryPath)
	if err != nil {
		return "", "", err
	}
	return generatedPackagePath, generatedHash, nil
}

func (m *Manager) packExtensionDirectory(extension Extension, chromeBinaryPath string) (string, string, error) {
	packageRoot := m.ResolveRelativePath(filepath.Join("data", extensionsRootDir, "packages"))
	if err := os.MkdirAll(packageRoot, 0o755); err != nil {
		return "", "", fmt.Errorf("创建插件包目录失败: %w", err)
	}
	keyPath := filepath.Join(packageRoot, extension.ExtensionID+".pem")
	workParent := filepath.Join(m.ResolveRelativePath(filepath.Join("data", extensionsRootDir)), ".pack")
	if err := os.MkdirAll(workParent, 0o755); err != nil {
		return "", "", fmt.Errorf("创建插件打包目录失败: %w", err)
	}
	workRoot, err := os.MkdirTemp(workParent, extension.ExtensionID+"-")
	if err != nil {
		return "", "", fmt.Errorf("创建插件打包目录失败: %w", err)
	}
	defer os.RemoveAll(workRoot)
	workingExtensionDir := filepath.Join(workRoot, "extension")
	if err := copyExtensionDirectory(extension.InstallDir, workingExtensionDir); err != nil {
		return "", "", err
	}

	arguments := []string{"--pack-extension=" + workingExtensionDir, "--no-message-box"}
	if _, err := os.Stat(keyPath); err == nil {
		arguments = append(arguments, "--pack-extension-key="+keyPath)
	}
	packCommand := exec.Command(chromeBinaryPath, arguments...)
	packCommand.Dir = filepath.Dir(chromeBinaryPath)
	packCommand.Stdout = io.Discard
	packCommand.Stderr = io.Discard
	hideExtensionInstallerWindow(packCommand)
	if err := packCommand.Run(); err != nil {
		return "", "", fmt.Errorf("生成插件持久安装包失败: %w", err)
	}

	generatedCRXPath := workingExtensionDir + ".crx"
	generatedKeyPath := workingExtensionDir + ".pem"
	packageData, err := os.ReadFile(generatedCRXPath)
	if err != nil || !isCRXExtensionPackage(packageData) {
		if err == nil {
			err = fmt.Errorf("生成文件不是有效 CRX")
		}
		return "", "", fmt.Errorf("读取生成的插件包失败: %w", err)
	}
	storedPath, packageHash, err := m.storeExtensionPackage(extension.ExtensionID, packageData)
	if err != nil {
		return "", "", err
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		if keyData, readErr := os.ReadFile(generatedKeyPath); readErr == nil {
			if writeErr := os.WriteFile(keyPath, keyData, 0o600); writeErr != nil {
				return "", "", fmt.Errorf("保存插件签名密钥失败: %w", writeErr)
			}
		}
	}

	extension.InstallMode = ExtensionInstallModePersistent
	extension.PackagePath = storedPath
	extension.PackageHash = packageHash
	if m.ExtensionDAO != nil {
		if err := m.ExtensionDAO.Upsert(extension); err != nil {
			return "", "", err
		}
	}
	return storedPath, packageHash, nil
}

func terminateExtensionInstallerProcess(command *exec.Cmd) {
	if command == nil || command.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		killTree := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", command.Process.Pid))
		_ = killTree.Run()
		return
	}
	_ = command.Process.Kill()
}

func findInstalledRuntimeExtensionID(userDataDir string, extension Extension) (string, error) {
	extensionRoot := filepath.Join(userDataDir, "Default", "Extensions")
	entries, err := os.ReadDir(extensionRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	expectedID := NormalizeExtensionID(extension.ExtensionID)
	if expectedID != "" {
		if persistentExtensionArtifactMatches(userDataDir, expectedID, extension.Version) {
			return expectedID, nil
		}
	}

	expectedName := strings.TrimSpace(extension.Name)
	fallbackRuntimeID := ""
	for _, entry := range entries {
		if !entry.IsDir() || !extensionIDPattern.MatchString(entry.Name()) {
			continue
		}
		versionEntries, err := os.ReadDir(filepath.Join(extensionRoot, entry.Name()))
		if err != nil {
			continue
		}
		for _, versionEntry := range versionEntries {
			if !versionEntry.IsDir() {
				continue
			}
			manifestData, err := os.ReadFile(filepath.Join(extensionRoot, entry.Name(), versionEntry.Name(), "manifest.json"))
			if err != nil {
				continue
			}
			var manifest extensionManifest
			if json.Unmarshal(manifestData, &manifest) != nil || strings.TrimSpace(manifest.Version) != strings.TrimSpace(extension.Version) {
				continue
			}
			if fallbackRuntimeID == "" {
				fallbackRuntimeID = entry.Name()
			}
			if expectedName != "" && !strings.EqualFold(expectedName, strings.TrimSpace(manifest.Name)) {
				continue
			}
			return entry.Name(), nil
		}
	}
	return fallbackRuntimeID, nil
}

func persistentExtensionArtifactMatches(userDataDir string, runtimeExtensionID string, version string) bool {
	runtimeExtensionID = strings.TrimSpace(runtimeExtensionID)
	if !extensionIDPattern.MatchString(runtimeExtensionID) {
		return false
	}
	versionRoot := filepath.Join(userDataDir, "Default", "Extensions", runtimeExtensionID)
	versionEntries, err := os.ReadDir(versionRoot)
	if err != nil {
		return false
	}
	for _, versionEntry := range versionEntries {
		if !versionEntry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(versionRoot, versionEntry.Name(), "manifest.json")
		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var manifest extensionManifest
		if json.Unmarshal(manifestData, &manifest) == nil && strings.TrimSpace(manifest.Version) == strings.TrimSpace(version) {
			return true
		}
	}
	return false
}

func findLegacyRuntimeExtensionIDs(userDataDir string, installDir string) ([]string, error) {
	preferencesPath := filepath.Join(userDataDir, "Default", "Secure Preferences")
	data, err := os.ReadFile(preferencesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var preferences extensionLegacyPreferences
	if err := json.Unmarshal(data, &preferences); err != nil {
		return nil, fmt.Errorf("读取插件旧安装记录失败: %w", err)
	}
	installDir = filepath.Clean(strings.TrimSpace(installDir))
	ids := make([]string, 0)
	for extensionID, setting := range preferences.Extensions.Settings {
		if !extensionIDPattern.MatchString(extensionID) || setting.Location != 8 {
			continue
		}
		settingPath := filepath.Clean(strings.TrimSpace(setting.Path))
		if settingPath != "" && strings.EqualFold(settingPath, installDir) {
			ids = append(ids, extensionID)
		}
	}
	return uniqueExtensionIDs(ids), nil
}

func (m *Manager) backupProfileExtensionState(profileID string, extensionID string, userDataDir string, runtimeIDs []string) (string, error) {
	if len(runtimeIDs) == 0 {
		return "", nil
	}
	backupRoot := filepath.Join(
		m.ResolveRelativePath(filepath.Join("data", extensionBackupRoot)),
		safeExtensionPathSegment(profileID),
		safeExtensionPathSegment(extensionID),
		time.Now().Format("20060102-150405.000000000"),
	)
	if err := os.MkdirAll(backupRoot, 0o755); err != nil {
		return "", fmt.Errorf("创建插件备份目录失败: %w", err)
	}
	defaultDir := filepath.Join(userDataDir, "Default")
	for _, runtimeID := range runtimeIDs {
		for _, relativePath := range extensionRuntimeStoragePaths(runtimeID) {
			sourcePath := filepath.Join(defaultDir, relativePath)
			if _, err := os.Stat(sourcePath); err != nil {
				continue
			}
			targetPath := filepath.Join(backupRoot, relativePath)
			if err := copyPath(sourcePath, targetPath); err != nil {
				return "", fmt.Errorf("备份插件数据失败: %w", err)
			}
		}
	}
	for _, rootName := range []string{"IndexedDB", "Service Worker"} {
		for _, runtimeID := range runtimeIDs {
			if err := backupExtensionRuntimeEntries(defaultDir, backupRoot, rootName, runtimeID); err != nil {
				return "", fmt.Errorf("备份插件运行数据失败: %w", err)
			}
		}
	}
	for _, fileName := range []string{"Preferences", "Secure Preferences"} {
		sourcePath := filepath.Join(defaultDir, fileName)
		if _, err := os.Stat(sourcePath); err != nil {
			continue
		}
		if err := copyPath(sourcePath, filepath.Join(backupRoot, fileName)); err != nil {
			return "", fmt.Errorf("备份插件配置失败: %w", err)
		}
	}
	return backupRoot, nil
}

func backupExtensionRuntimeEntries(defaultDir string, backupRoot string, rootName string, runtimeID string) error {
	rootPath := filepath.Join(defaultDir, rootName)
	entries, err := collectExtensionRuntimeEntries(rootPath, runtimeID)
	if err != nil {
		return err
	}
	for _, sourcePath := range entries {
		relativePath, err := filepath.Rel(defaultDir, sourcePath)
		if err != nil || relativePath == "." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) || relativePath == ".." {
			return fmt.Errorf("插件备份路径越界: %s", sourcePath)
		}
		if err := copyPath(sourcePath, filepath.Join(backupRoot, relativePath)); err != nil {
			return err
		}
	}
	return nil
}

func migrateExtensionStorage(userDataDir string, oldRuntimeID string, newRuntimeID string) error {
	oldRuntimeID = strings.TrimSpace(oldRuntimeID)
	newRuntimeID = strings.TrimSpace(newRuntimeID)
	if oldRuntimeID == "" || newRuntimeID == "" || oldRuntimeID == newRuntimeID {
		return nil
	}
	defaultDir := filepath.Join(userDataDir, "Default")
	for _, rootName := range []string{"Local Extension Settings", "Sync Extension Settings", "Managed Extension Settings", "Extension State", "Extension Rules", "Extension Scripts"} {
		oldPath := filepath.Join(defaultDir, rootName, oldRuntimeID)
		newPath := filepath.Join(defaultDir, rootName, newRuntimeID)
		if _, err := os.Stat(oldPath); err != nil {
			continue
		}
		if _, err := os.Stat(newPath); err == nil {
			if err := os.RemoveAll(newPath); err != nil {
				return err
			}
		}
		if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
			return err
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return err
		}
	}
	for _, rootName := range []string{"IndexedDB", "Service Worker"} {
		rootPath := filepath.Join(defaultDir, rootName)
		if err := renameExtensionRuntimeEntries(rootPath, oldRuntimeID, newRuntimeID); err != nil {
			return err
		}
	}
	return nil
}

func restoreProfileExtensionState(userDataDir string, backupPath string, legacyRuntimeIDs []string, runtimeExtensionID string) error {
	if strings.TrimSpace(backupPath) == "" {
		return nil
	}
	_ = removePersistentExtensionCode(userDataDir, runtimeExtensionID)
	for _, runtimeID := range uniqueExtensionIDs(append(append([]string{}, legacyRuntimeIDs...), runtimeExtensionID)) {
		_ = removePersistentExtensionCode(userDataDir, runtimeID)
		for _, rootName := range []string{"Local Extension Settings", "Sync Extension Settings", "Managed Extension Settings", "Extension State", "Extension Rules", "Extension Scripts"} {
			_ = os.RemoveAll(filepath.Join(userDataDir, "Default", rootName, runtimeID))
		}
		for _, rootName := range []string{"IndexedDB", "Service Worker"} {
			_ = removeExtensionRuntimeEntries(filepath.Join(userDataDir, "Default", rootName), runtimeID)
		}
	}
	entries, err := os.ReadDir(backupPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(backupPath, entry.Name())
		targetPath := filepath.Join(userDataDir, "Default", entry.Name())
		if entry.Name() == "Preferences" || entry.Name() == "Secure Preferences" {
			targetPath = filepath.Join(userDataDir, "Default", entry.Name())
		}
		_ = os.RemoveAll(targetPath)
		if err := copyPath(sourcePath, targetPath); err != nil {
			return err
		}
	}
	return nil
}

func removeExtensionRuntimeEntries(rootPath string, runtimeID string) error {
	entries, err := collectExtensionRuntimeEntries(rootPath, runtimeID)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(entry); err != nil {
			return err
		}
	}
	return nil
}

func renameExtensionRuntimeEntries(rootPath string, oldRuntimeID string, newRuntimeID string) error {
	entries, err := collectExtensionRuntimeEntries(rootPath, oldRuntimeID)
	if err != nil {
		return err
	}
	for _, oldPath := range entries {
		newPath := filepath.Join(filepath.Dir(oldPath), strings.ReplaceAll(filepath.Base(oldPath), oldRuntimeID, newRuntimeID))
		if _, err := os.Stat(newPath); err == nil {
			if err := os.RemoveAll(newPath); err != nil {
				return err
			}
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return err
		}
	}
	return nil
}

func collectExtensionRuntimeEntries(rootPath string, runtimeID string) ([]string, error) {
	rootPath = filepath.Clean(rootPath)
	runtimeID = strings.TrimSpace(runtimeID)
	if runtimeID == "" {
		return nil, nil
	}
	entries := make([]string, 0)
	err := filepath.WalkDir(rootPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != rootPath && strings.Contains(strings.ToLower(filepath.Base(path)), strings.ToLower(runtimeID)) {
			entries = append(entries, path)
			if entry.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	sort.Slice(entries, func(left int, right int) bool { return len(entries[left]) > len(entries[right]) })
	return entries, nil
}

func persistentExtensionCodeID(userDataDir string, extensionID string) string {
	runtimeID := NormalizeExtensionID(extensionID)
	if runtimeID == "" {
		return ""
	}
	path := filepath.Join(userDataDir, "Default", "Extensions", runtimeID)
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return runtimeID
	}
	return ""
}

func removePersistentExtensionCode(userDataDir string, runtimeExtensionID string) error {
	runtimeExtensionID = strings.TrimSpace(runtimeExtensionID)
	if !extensionIDPattern.MatchString(runtimeExtensionID) {
		return nil
	}
	extensionsRoot := filepath.Clean(filepath.Join(userDataDir, "Default", "Extensions"))
	targetPath := filepath.Clean(filepath.Join(extensionsRoot, runtimeExtensionID))
	relativePath, err := filepath.Rel(extensionsRoot, targetPath)
	if err != nil || relativePath == "." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) || relativePath == ".." {
		return fmt.Errorf("拒绝删除实例外插件目录: %s", runtimeExtensionID)
	}
	if err := os.RemoveAll(targetPath); err != nil {
		return fmt.Errorf("删除实例持久插件代码失败: %w", err)
	}
	return nil
}

func extensionRuntimeStoragePaths(runtimeID string) []string {
	return []string{
		filepath.Join("Extensions", runtimeID),
		filepath.Join("Local Extension Settings", runtimeID),
		filepath.Join("Sync Extension Settings", runtimeID),
		filepath.Join("Managed Extension Settings", runtimeID),
		filepath.Join("Extension State", runtimeID),
		filepath.Join("Extension Rules", runtimeID),
		filepath.Join("Extension Scripts", runtimeID),
	}
}

func copyPath(sourcePath string, targetPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDirectory(sourcePath, targetPath)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	return os.WriteFile(targetPath, data, 0o600)
}

func copyDirectory(sourceDir string, targetDir string) error {
	return filepath.WalkDir(sourceDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetDir, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0o600)
	})
}

func uniqueExtensionIDs(extensionIDs []string) []string {
	seen := make(map[string]struct{}, len(extensionIDs))
	result := make([]string, 0, len(extensionIDs))
	for _, extensionID := range extensionIDs {
		extensionID = strings.TrimSpace(extensionID)
		if !extensionIDPattern.MatchString(extensionID) {
			continue
		}
		if _, exists := seen[extensionID]; exists {
			continue
		}
		seen[extensionID] = struct{}{}
		result = append(result, extensionID)
	}
	return result
}

func normalizeNonEmptyExtensionDirs(dirs []string) []string {
	seen := make(map[string]struct{}, len(dirs))
	result := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(dir))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, dir)
	}
	return result
}

func safeExtensionPathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var builder strings.Builder
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '-' || character == '_' {
			builder.WriteRune(character)
			continue
		}
		builder.WriteByte('_')
	}
	if builder.Len() == 0 {
		return "unknown"
	}
	return builder.String()
}
