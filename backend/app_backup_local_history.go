package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const backupMetadataReadLimit = 64 * 1024 * 1024

type BackupLocalSettings struct {
	LocalDirectory string `json:"localDirectory"`
}

type BackupSelectLocalDirectoryResult struct {
	Cancelled      bool   `json:"cancelled"`
	LocalDirectory string `json:"localDirectory"`
}

type BackupLocalHistoryItem struct {
	Name              string   `json:"name"`
	Path              string   `json:"path"`
	Size              int64    `json:"size"`
	ModifiedAt        string   `json:"modifiedAt"`
	CreatedAt         string   `json:"createdAt,omitempty"`
	MetadataAvailable bool     `json:"metadataAvailable"`
	MetadataError     string   `json:"metadataError,omitempty"`
	MetadataOrphan    bool     `json:"metadataOrphan,omitempty"`
	AppName           string   `json:"appName,omitempty"`
	AppVersion        string   `json:"appVersion,omitempty"`
	PackageType       string   `json:"packageType,omitempty"`
	ProfileCount      int      `json:"profileCount,omitempty"`
	ProfileNames      []string `json:"profileNames,omitempty"`
}

func (a *App) BackupGetLocalSettings() BackupLocalSettings {
	if a == nil || a.config == nil {
		return BackupLocalSettings{}
	}
	return BackupLocalSettings{
		LocalDirectory: strings.TrimSpace(a.config.Backup.LocalDirectory),
	}
}

func (a *App) BackupSelectLocalDirectory() (BackupSelectLocalDirectoryResult, error) {
	if a == nil || a.ctx == nil {
		return BackupSelectLocalDirectoryResult{}, fmt.Errorf("应用上下文未初始化")
	}
	selected, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择本地备份目录",
	})
	if err != nil {
		return BackupSelectLocalDirectoryResult{}, fmt.Errorf("打开本地备份目录选择器失败: %w", err)
	}
	selected = strings.TrimSpace(selected)
	if selected == "" {
		return BackupSelectLocalDirectoryResult{
			Cancelled:      true,
			LocalDirectory: a.BackupGetLocalSettings().LocalDirectory,
		}, nil
	}
	settings, err := a.BackupSaveLocalDirectory(selected)
	if err != nil {
		return BackupSelectLocalDirectoryResult{}, err
	}
	return BackupSelectLocalDirectoryResult{LocalDirectory: settings.LocalDirectory}, nil
}

func (a *App) BackupSaveLocalDirectory(directory string) (BackupLocalSettings, error) {
	if a == nil {
		return BackupLocalSettings{}, fmt.Errorf("应用未初始化")
	}
	if a.config == nil {
		return BackupLocalSettings{}, fmt.Errorf("应用配置未初始化")
	}
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return BackupLocalSettings{}, fmt.Errorf("本地备份目录不能为空")
	}
	resolved := a.resolveAppPath(directory)
	absDirectory, err := filepath.Abs(resolved)
	if err != nil {
		return BackupLocalSettings{}, fmt.Errorf("解析本地备份目录失败: %w", err)
	}
	info, err := os.Stat(absDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return BackupLocalSettings{}, fmt.Errorf("本地备份目录不存在: %s", absDirectory)
		}
		return BackupLocalSettings{}, fmt.Errorf("读取本地备份目录失败: %w", err)
	}
	if !info.IsDir() {
		return BackupLocalSettings{}, fmt.Errorf("本地备份路径不是目录: %s", absDirectory)
	}

	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	if err := a.backupSetLocalDirectoryLocked(absDirectory); err != nil {
		return BackupLocalSettings{}, err
	}
	return BackupLocalSettings{LocalDirectory: absDirectory}, nil
}

func (a *App) backupSetLocalDirectoryLocked(directory string) error {
	if a == nil || a.config == nil {
		return fmt.Errorf("应用配置未初始化")
	}
	scheduler := a.backupScheduler
	if scheduler != nil {
		scheduler.mu.Lock()
		defer scheduler.mu.Unlock()
	}
	previous := a.config.Backup
	next := normalizeBackupSettings(previous)
	next.LocalDirectory = strings.TrimSpace(directory)
	a.config.Backup = next
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		a.config.Backup = previous
		return fmt.Errorf("保存本地备份目录失败: %w", err)
	}
	if scheduler != nil {
		scheduler.settings.LocalDirectory = next.LocalDirectory
	}
	return nil
}

func (a *App) backupResolveLocalPackagePath(defaultName, title string) (string, bool, error) {
	if a == nil || a.ctx == nil {
		return "", false, fmt.Errorf("应用上下文未初始化")
	}
	configuredDirectory := ""
	if a.config != nil {
		configuredDirectory = strings.TrimSpace(a.config.Backup.LocalDirectory)
	}
	if configuredDirectory != "" {
		resolvedDirectory := a.resolveAppPath(configuredDirectory)
		info, err := os.Stat(resolvedDirectory)
		if err != nil {
			if os.IsNotExist(err) {
				return "", false, fmt.Errorf("配置的本地备份目录不存在，请重新配置: %s", resolvedDirectory)
			}
			return "", false, fmt.Errorf("读取配置的本地备份目录失败: %w", err)
		}
		if !info.IsDir() {
			return "", false, fmt.Errorf("配置的本地备份路径不是目录，请重新配置: %s", resolvedDirectory)
		}
		path, err := backupNextAvailablePackagePath(resolvedDirectory, defaultName)
		return path, false, err
	}

	savePath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           title,
		DefaultFilename: defaultName,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "ZIP 文件 (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return "", false, fmt.Errorf("打开保存对话框失败: %w", err)
	}
	if strings.TrimSpace(savePath) == "" {
		return "", true, nil
	}
	savePath = backupEnsureZipSuffix(savePath)
	if err := a.backupSetLocalDirectoryLocked(filepath.Dir(savePath)); err != nil {
		return "", false, fmt.Errorf("保存本地备份目录失败: %w", err)
	}
	return savePath, false, nil
}

func backupNextAvailablePackagePath(directory, defaultName string) (string, error) {
	if strings.TrimSpace(defaultName) == "" {
		return "", fmt.Errorf("备份文件名不能为空")
	}
	for index := 0; index < 10000; index++ {
		name := defaultName
		if index > 0 {
			ext := filepath.Ext(defaultName)
			stem := strings.TrimSuffix(defaultName, ext)
			name = fmt.Sprintf("%s-%d%s", stem, index, ext)
		}
		candidate := filepath.Join(directory, name)
		if _, err := os.Stat(candidate); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("检查备份文件名失败: %w", err)
		}
		if _, err := os.Stat(backupMetadataPath(candidate)); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("检查备份元数据文件名失败: %w", err)
		}
		return candidate, nil
	}
	return "", fmt.Errorf("备份目录中的可用文件名已耗尽")
}

func (a *App) BackupListLocalBackups(directory string) ([]BackupLocalHistoryItem, error) {
	if a == nil {
		return nil, fmt.Errorf("应用未初始化")
	}
	directory = strings.TrimSpace(directory)
	if directory == "" && a.config != nil {
		directory = strings.TrimSpace(a.config.Backup.LocalDirectory)
	}
	if directory == "" {
		return []BackupLocalHistoryItem{}, nil
	}
	root := a.resolveAppPath(directory)
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("解析本地备份目录失败: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("本地备份目录不存在: %s", absRoot)
		}
		return nil, fmt.Errorf("读取本地备份目录失败: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("本地备份路径不是目录: %s", absRoot)
	}

	entries, err := os.ReadDir(absRoot)
	if err != nil {
		return nil, fmt.Errorf("扫描本地备份目录失败: %w", err)
	}
	zipNames := collectLocalBackupZIPNames(entries)
	items := make([]BackupLocalHistoryItem, 0, len(entries))
	seenZIPNames := make(map[string]struct{}, len(zipNames))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".zip") {
			continue
		}
		zipKey := localBackupFileKey(entry.Name())
		if _, exists := seenZIPNames[zipKey]; exists {
			continue
		}
		seenZIPNames[zipKey] = struct{}{}
		zipPath := filepath.Join(absRoot, entry.Name())
		zipInfo, statErr := entry.Info()
		if statErr != nil || !zipInfo.Mode().IsRegular() {
			continue
		}
		item := BackupLocalHistoryItem{
			Name:       entry.Name(),
			Path:       zipPath,
			Size:       zipInfo.Size(),
			ModifiedAt: zipInfo.ModTime().UTC().Format(time.RFC3339Nano),
		}
		metadata, metadataErr := readOrGenerateBackupMetadataForZip(zipPath, entry.Name())
		if metadataErr != nil {
			if !os.IsNotExist(metadataErr) {
				item.MetadataError = metadataErr.Error()
			}
		} else {
			item.MetadataAvailable = true
			item.CreatedAt = strings.TrimSpace(metadata.CreatedAt)
			if item.CreatedAt != "" {
				item.ModifiedAt = item.CreatedAt
			}
			item.AppName = strings.TrimSpace(metadata.App.Name)
			item.AppVersion = strings.TrimSpace(metadata.App.Version)
			item.PackageType = strings.TrimSpace(metadata.PackageType)
			item.ProfileCount = metadata.ProfileCount
			item.ProfileNames = normalizeBackupPackageProfileNames(metadata.ProfileNames)
		}
		items = append(items, item)
	}
	items = append(items, collectOrphanBackupMetadataItems(absRoot, entries, zipNames)...)
	sort.SliceStable(items, func(i, j int) bool {
		left, right := parseBackupHistoryTime(items[i].ModifiedAt), parseBackupHistoryTime(items[j].ModifiedAt)
		if !left.Equal(right) {
			return left.After(right)
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items, nil
}

func localBackupFileKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func collectLocalBackupZIPNames(entries []os.DirEntry) map[string]string {
	result := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		result[localBackupFileKey(name)] = name
	}
	return result
}

func collectOrphanBackupMetadataItems(root string, entries []os.DirEntry, zipNames map[string]string) []BackupLocalHistoryItem {
	items := make([]BackupLocalHistoryItem, 0)
	seen := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		metadataPath := filepath.Join(root, entry.Name())
		metadata, err := readBackupMetadataRecord(metadataPath)
		if err != nil {
			continue
		}
		baseZIPName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())) + ".zip"
		targetZIPName := baseZIPName
		if declared := strings.TrimSpace(metadata.BackupFile); declared != "" {
			declaredName := filepath.Base(filepath.FromSlash(declared))
			if !strings.EqualFold(declaredName, baseZIPName) {
				continue
			}
			targetZIPName = declaredName
		}
		if _, exists := zipNames[localBackupFileKey(targetZIPName)]; exists {
			continue
		}
		key := localBackupFileKey(entry.Name())
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, BackupLocalHistoryItem{
			Name:           targetZIPName,
			Path:           metadataPath,
			ModifiedAt:     info.ModTime().UTC().Format(time.RFC3339Nano),
			MetadataError:  fmt.Sprintf("同名 ZIP 文件不存在: %s", targetZIPName),
			MetadataOrphan: true,
		})
	}
	return items
}

func readBackupMetadataRecord(metadataPath string) (backupMetadata, error) {
	file, err := os.Open(metadataPath)
	if err != nil {
		return backupMetadata{}, err
	}
	defer file.Close()
	var metadata backupMetadata
	if err := json.NewDecoder(io.LimitReader(file, backupMetadataReadLimit)).Decode(&metadata); err != nil {
		return backupMetadata{}, fmt.Errorf("解析备份元数据失败: %w", err)
	}
	if strings.TrimSpace(metadata.Format) != "ant-chrome-backup-metadata" {
		return backupMetadata{}, fmt.Errorf("备份元数据格式不支持")
	}
	if metadata.Version <= 0 {
		return backupMetadata{}, fmt.Errorf("备份元数据版本无效")
	}
	return metadata, nil
}

func readBackupMetadataForFile(metadataPath, backupFileName string) (backupMetadata, error) {
	file, err := os.Open(metadataPath)
	if err != nil {
		return backupMetadata{}, err
	}
	defer file.Close()
	var metadata backupMetadata
	if err := json.NewDecoder(io.LimitReader(file, backupMetadataReadLimit)).Decode(&metadata); err != nil {
		return backupMetadata{}, fmt.Errorf("解析备份元数据失败: %w", err)
	}
	if strings.TrimSpace(metadata.Format) != "ant-chrome-backup-metadata" {
		return backupMetadata{}, fmt.Errorf("备份元数据格式不支持")
	}
	if metadata.Version <= 0 {
		return backupMetadata{}, fmt.Errorf("备份元数据版本无效")
	}
	if declared := strings.TrimSpace(metadata.BackupFile); declared != "" && !strings.EqualFold(filepath.Base(declared), backupFileName) {
		return backupMetadata{}, fmt.Errorf("备份元数据与 ZIP 文件不匹配")
	}
	return metadata, nil
}

func readOrGenerateBackupMetadataForZip(zipPath, backupFileName string) (backupMetadata, error) {
	metadataPath := backupMetadataPath(zipPath)
	metadata, err := readBackupMetadataForFile(metadataPath, backupFileName)
	if err == nil {
		return metadata, nil
	}
	if !os.IsNotExist(err) {
		return backupMetadata{}, err
	}

	inspection, err := inspectBackupPackage(zipPath)
	if err != nil {
		return backupMetadata{}, fmt.Errorf("读取备份包内部 manifest.json 失败: %w", err)
	}
	switch {
	case inspection.fullManifest != nil:
		if _, err := backupWriteMetadata(zipPath, *inspection.fullManifest, inspection.includedEntries, inspection.skippedEntries); err != nil {
			return backupMetadata{}, fmt.Errorf("生成备份元数据失败: %w", err)
		}
	case inspection.profileManifest != nil:
		manifest := *inspection.profileManifest
		if len(manifest.ProfileNames) == 0 {
			manifest.ProfileNames = inspection.info.ProfileNames
		}
		if manifest.ProfileCount < len(manifest.ProfileNames) {
			manifest.ProfileCount = len(manifest.ProfileNames)
		}
		if _, err := backupWriteProfileMetadata(zipPath, manifest, inspection.fileCount, "", ""); err != nil {
			return backupMetadata{}, fmt.Errorf("生成备份元数据失败: %w", err)
		}
	default:
		return backupMetadata{}, fmt.Errorf("备份包内部 manifest.json 类型不可识别")
	}
	return readBackupMetadataForFile(metadataPath, backupFileName)
}

func parseBackupHistoryTime(value string) time.Time {
	timestamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return timestamp
}
