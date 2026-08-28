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
	if err := os.WriteFile(metadataPath, append(data, '\n'), 0644); err != nil {
		return "", fmt.Errorf("写入备份元数据失败: %w", err)
	}
	return metadataPath, nil
}
