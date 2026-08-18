package backend

import (
	"ant-chrome/backend/internal/backup"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/snapshot"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var backupImportUnzipLimits = snapshot.UnzipLimits{
	MaxEntries:           200000,
	MaxUncompressedBytes: 64 * 1024 * 1024 * 1024,
	MaxSingleFileBytes:   16 * 1024 * 1024 * 1024,
	MaxCompressedBytes:   64 * 1024 * 1024 * 1024,
}

func backupExtractAndValidate(zipPath string) (string, backup.Manifest, error) {
	tmpDir, err := os.MkdirTemp("", "ant-chrome-import-*")
	if err != nil {
		return "", backup.Manifest{}, err
	}
	if err := snapshot.UnzipToWithLimits(zipPath, tmpDir, backupImportUnzipLimits); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", backup.Manifest{}, fmt.Errorf("解压备份包失败: %w", err)
	}

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", backup.Manifest{}, fmt.Errorf("备份包缺少 manifest.json")
	}
	var manifest backup.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", backup.Manifest{}, fmt.Errorf("manifest.json 解析失败: %w", err)
	}
	if manifest.Format != backup.PackageFormat {
		_ = os.RemoveAll(tmpDir)
		return "", backup.Manifest{}, fmt.Errorf("不支持的备份格式: %s", manifest.Format)
	}
	if manifest.ManifestVersion != backup.ManifestVersion {
		_ = os.RemoveAll(tmpDir)
		return "", backup.Manifest{}, fmt.Errorf("不支持的 manifest 版本: %d", manifest.ManifestVersion)
	}
	if err := backupValidateManifest(tmpDir, manifest); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", backup.Manifest{}, err
	}
	return tmpDir, manifest, nil
}

func backupLoadIncomingConfig(payloadRoot string) (*config.Config, bool, error) {
	cfgPath := filepath.Join(payloadRoot, "system", "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, false, err
	}
	return cfg, true, nil
}

func backupDetectPresentManifestEntries(extractRoot string, manifest backup.Manifest) map[string]backup.ManifestEntry {
	result := make(map[string]backup.ManifestEntry, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			continue
		}
		archivePath := strings.TrimSpace(strings.TrimSuffix(entry.ArchivePath, "/"))
		if archivePath == "" {
			continue
		}
		absPath := filepath.Join(extractRoot, filepath.FromSlash(archivePath))
		if _, err := os.Stat(absPath); err == nil {
			result[id] = entry
		}
	}
	return result
}
