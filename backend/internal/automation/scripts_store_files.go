package automation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (s *ScriptStore) ensureRoot() error {
	if s.rootDir == "" || s.rootDir == "." {
		return fmt.Errorf("automation script root dir is empty")
	}
	return os.MkdirAll(s.rootDir, 0o755)
}

func (s *ScriptStore) readScriptDir(dir string) (ScriptRecord, error) {
	data, err := readScriptStoreConfigFile(dir)
	if err != nil {
		return ScriptRecord{}, err
	}

	var config scriptStoreConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return ScriptRecord{}, fmt.Errorf("unmarshal automation script config failed: %w", err)
	}

	record, err := normalizeScriptRecord(ScriptRecord{
		PackageFormat:   config.PackageFormat,
		ManifestVersion: config.ManifestVersion,
		ID:              config.ID,
		Name:            config.Name,
		Description:     config.Description,
		Type:            config.Type,
		Status:          config.Status,
		EntryFile:       config.EntryFile,
		Tags:            config.Tags,
		SelectorText:    config.SelectorText,
		ParamsText:      config.ParamsText,
		Notes:           config.Notes,
		TargetConfig:    config.TargetConfig,
		PublicAPI:       config.PublicAPI,
		Source:          config.Source,
		CreatedAt:       config.CreatedAt,
		UpdatedAt:       config.UpdatedAt,
	}, ScriptRecord{})
	if err != nil {
		return ScriptRecord{}, err
	}

	scriptData, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(record.EntryFile)))
	if err != nil {
		if !os.IsNotExist(err) {
			return ScriptRecord{}, fmt.Errorf("read automation script file failed: %w", err)
		}
		record.ScriptText = ""
		return record, nil
	}
	record.ScriptText = string(scriptData)
	return record, nil
}

func (s *ScriptStore) scriptDir(scriptID string) (string, error) {
	normalizedID := strings.TrimSpace(scriptID)
	if normalizedID == "" {
		return "", fmt.Errorf("script id is required")
	}
	if !isSafeScriptID(normalizedID) {
		return "", fmt.Errorf("script id is invalid")
	}
	return filepath.Join(s.rootDir, normalizedID), nil
}

func (s *ScriptStore) writeRecord(dir string, record ScriptRecord, existing ScriptRecord, files []ImportedBundleFile) (ScriptRecord, error) {
	hadStoreConfig := scriptStoreFileExists(filepath.Join(dir, scriptStoreConfigFileName))
	hadLegacyConfigOnly := !hadStoreConfig && scriptStoreFileExists(filepath.Join(dir, scriptStoreLegacyConfigName))

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ScriptRecord{}, fmt.Errorf("create automation script dir failed: %w", err)
	}

	if len(files) > 0 {
		if err := os.RemoveAll(dir); err != nil {
			return ScriptRecord{}, fmt.Errorf("reset automation script dir failed: %w", err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return ScriptRecord{}, fmt.Errorf("create automation script dir failed: %w", err)
		}
	}

	for _, file := range files {
		relativePath, err := normalizeBundleFilePath(file.Path)
		if err != nil {
			return ScriptRecord{}, err
		}
		if relativePath == scriptStoreConfigFileName {
			continue
		}
		targetPath := filepath.Join(dir, filepath.FromSlash(relativePath))
		if err := writeFileAtomic(targetPath, file.Content, 0o644); err != nil {
			return ScriptRecord{}, fmt.Errorf("write automation script bundle file failed: %w", err)
		}
	}

	scriptPath := filepath.Join(dir, filepath.FromSlash(record.EntryFile))
	if err := writeFileAtomic(scriptPath, []byte(record.ScriptText), 0o644); err != nil {
		return ScriptRecord{}, fmt.Errorf("write automation script file failed: %w", err)
	}

	config := scriptStoreConfig{
		PackageFormat:   record.PackageFormat,
		ManifestVersion: record.ManifestVersion,
		ID:              record.ID,
		Name:            record.Name,
		Description:     record.Description,
		Type:            record.Type,
		Status:          record.Status,
		EntryFile:       record.EntryFile,
		Tags:            append([]string{}, record.Tags...),
		SelectorText:    record.SelectorText,
		ParamsText:      record.ParamsText,
		Notes:           record.Notes,
		TargetConfig:    record.TargetConfig,
		PublicAPI:       record.PublicAPI,
		Source:          record.Source,
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return ScriptRecord{}, fmt.Errorf("marshal automation script config failed: %w", err)
	}
	if err := writeFileAtomic(filepath.Join(dir, scriptStoreConfigFileName), configData, 0o644); err != nil {
		return ScriptRecord{}, fmt.Errorf("write automation script config failed: %w", err)
	}
	if len(files) == 0 && hadLegacyConfigOnly {
		_ = os.Remove(filepath.Join(dir, scriptStoreLegacyConfigName))
	}

	if len(files) == 0 && existing.EntryFile != "" && existing.EntryFile != record.EntryFile {
		_ = os.Remove(filepath.Join(dir, filepath.FromSlash(existing.EntryFile)))
	}

	return record, nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Chmod(mode); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err == nil {
		return nil
	}
	if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
		return removeErr
	}
	return os.Rename(tmpPath, path)
}

func collectScriptStoreBundleFiles(root string) ([]ImportedBundleFile, error) {
	files := make([]ImportedBundleFile, 0, 8)

	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)
		if relativePath == "." || relativePath == scriptStoreConfigFileName {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, ImportedBundleFile{
			Path:    relativePath,
			Content: content,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect automation script export files failed: %w", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func readScriptStoreConfigFile(dir string) ([]byte, error) {
	for _, candidate := range []string{scriptStoreConfigFileName, scriptStoreLegacyConfigName} {
		data, err := os.ReadFile(filepath.Join(dir, candidate))
		if err == nil {
			return data, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return nil, os.ErrNotExist
}

func scriptStoreFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
