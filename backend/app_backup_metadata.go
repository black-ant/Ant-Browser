package backend

import (
	"ant-chrome/backend/internal/backup"
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const backupMetadataVersion = 1

type backupMetadata struct {
	Format          string                 `json:"format"`
	Version         int                    `json:"version"`
	BackupFile      string                 `json:"backupFile"`
	BackupSizeBytes int64                  `json:"backupSizeBytes"`
	CreatedAt       string                 `json:"createdAt"`
	App             backup.ManifestAppInfo `json:"app"`
	PackageType     string                 `json:"packageType,omitempty"`
	ProfileCount    int                    `json:"profileCount,omitempty"`
	ProfileNames    []string               `json:"profileNames,omitempty"`
	ManifestVersion int                    `json:"manifestVersion"`
	IncludedEntries int                    `json:"includedEntries"`
	SkippedEntries  int                    `json:"skippedEntries"`
	FileCount       int                    `json:"fileCount"`
	Files           []backupMetadataFile   `json:"files"`
	Components      []backup.ManifestEntry `json:"components"`
}

type backupMetadataFile struct {
	Path                string `json:"path"`
	SizeBytes           uint64 `json:"sizeBytes"`
	CompressedSizeBytes uint64 `json:"compressedSizeBytes,omitempty"`
}

func backupMetadataPath(zipPath string) string {
	ext := filepath.Ext(zipPath)
	if ext == "" {
		return zipPath + ".json"
	}
	return strings.TrimSuffix(zipPath, ext) + ".json"
}

func backupWriteMetadata(zipPath string, manifest backup.Manifest, includedEntries, skippedEntries int) (string, error) {
	zipInfo, err := os.Stat(zipPath)
	if err != nil {
		return "", fmt.Errorf("读取备份文件信息失败: %w", err)
	}
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("读取备份文件清单失败: %w", err)
	}

	files := make([]backupMetadataFile, 0, len(archive.File))
	for _, entry := range archive.File {
		name := filepath.ToSlash(strings.TrimSpace(entry.Name))
		if name == "" || name == "manifest.json" || strings.HasSuffix(name, "/") || entry.FileInfo().IsDir() {
			continue
		}
		files = append(files, backupMetadataFile{
			Path:                name,
			SizeBytes:           entry.UncompressedSize64,
			CompressedSizeBytes: entry.CompressedSize64,
		})
	}
	if err := archive.Close(); err != nil {
		return "", fmt.Errorf("关闭备份文件清单失败: %w", err)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	metadata := backupMetadata{
		Format:          "ant-chrome-backup-metadata",
		Version:         backupMetadataVersion,
		BackupFile:      filepath.Base(zipPath),
		BackupSizeBytes: zipInfo.Size(),
		CreatedAt:       manifest.CreatedAt,
		App:             manifest.App,
		PackageType:     "full",
		ManifestVersion: manifest.ManifestVersion,
		IncludedEntries: includedEntries,
		SkippedEntries:  skippedEntries,
		FileCount:       len(files),
		Files:           files,
		Components:      manifest.Entries,
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("生成备份元数据失败: %w", err)
	}

	metadataPath := backupMetadataPath(zipPath)
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0755); err != nil {
		return "", fmt.Errorf("创建备份元数据目录失败: %w", err)
	}
	if err := writeBackupMetadataFile(metadataPath, append(data, '\n')); err != nil {
		return "", fmt.Errorf("写入备份元数据失败: %w", err)
	}
	return metadataPath, nil
}

func backupWriteProfileMetadata(zipPath string, manifest ProfilePackageManifest, fileCount int, appName, appVersion string) (string, error) {
	zipInfo, err := os.Stat(zipPath)
	if err != nil {
		return "", fmt.Errorf("读取备份文件信息失败: %w", err)
	}
	data, err := json.MarshalIndent(backupMetadata{
		Format:          "ant-chrome-backup-metadata",
		Version:         backupMetadataVersion,
		BackupFile:      filepath.Base(zipPath),
		BackupSizeBytes: zipInfo.Size(),
		CreatedAt:       manifest.ExportedAt,
		App:             backup.ManifestAppInfo{Name: strings.TrimSpace(appName), Version: strings.TrimSpace(appVersion)},
		PackageType:     "profile",
		ProfileCount:    manifest.ProfileCount,
		ProfileNames:    manifest.ProfileNames,
		ManifestVersion: manifest.Version,
		FileCount:       fileCount,
	}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("生成备份元数据失败: %w", err)
	}
	metadataPath := backupMetadataPath(zipPath)
	if err := writeBackupMetadataFile(metadataPath, append(data, '\n')); err != nil {
		return "", err
	}
	return metadataPath, nil
}

func writeBackupMetadataFile(metadataPath string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0755); err != nil {
		return fmt.Errorf("创建备份元数据目录失败: %w", err)
	}
	tmpPath := metadataPath + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("创建备份元数据临时文件失败: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("写入备份元数据失败: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("刷新备份元数据失败: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("关闭备份元数据失败: %w", err)
	}
	if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("替换备份元数据失败: %w", err)
	}
	if err := os.Rename(tmpPath, metadataPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("发布备份元数据失败: %w", err)
	}
	return nil
}

func backupPrepareRemoteMetadata(metadataPath, localBackupFileName, remoteBackupFileName string) (string, func(), error) {
	metadata, err := readBackupMetadataForFile(metadataPath, filepath.Base(localBackupFileName))
	if err != nil {
		return "", func() {}, err
	}
	remoteName := filepath.Base(remoteBackupFileName)
	if remoteName == "" || remoteName == "." || remoteName == string(filepath.Separator) {
		return "", func() {}, fmt.Errorf("invalid remote backup file name")
	}
	if strings.EqualFold(strings.TrimSpace(metadata.BackupFile), remoteName) {
		return metadataPath, func() {}, nil
	}
	metadata.BackupFile = remoteName
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", func() {}, err
	}
	temporaryFile, err := os.CreateTemp(filepath.Dir(metadataPath), ".ant-chrome-backup-metadata-*.json")
	if err != nil {
		return "", func() {}, err
	}
	temporaryPath := temporaryFile.Name()
	cleanup := func() { _ = os.Remove(temporaryPath) }
	if _, err := temporaryFile.Write(data); err != nil {
		_ = temporaryFile.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := temporaryFile.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return temporaryPath, cleanup, nil
}
