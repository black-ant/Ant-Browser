package backend

import (
	"ant-chrome/backend/internal/backup"
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const backupPackageInfoEntryLimit = 16 * 1024 * 1024

type backupPackageInfo struct {
	PackageType  string
	ProfileCount int
	ProfileNames []string
}

type backupPackageInspection struct {
	info            backupPackageInfo
	fullManifest    *backup.Manifest
	profileManifest *ProfilePackageManifest
	includedEntries int
	skippedEntries  int
	fileCount       int
}

func inspectBackupPackageInfo(zipPath string) (backupPackageInfo, error) {
	inspection, err := inspectBackupPackage(zipPath)
	if err != nil {
		return backupPackageInfo{}, err
	}
	return inspection.info, nil
}

func inspectBackupPackage(zipPath string) (backupPackageInspection, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return backupPackageInspection{}, fmt.Errorf(`open backup package failed: %w`, err)
	}
	defer reader.Close()

	manifestFile := findBackupPackageEntry(reader.File, `manifest.json`)
	if manifestFile == nil {
		return backupPackageInspection{}, fmt.Errorf(`backup package has no manifest.json`)
	}
	var header map[string]json.RawMessage
	if err := decodeBackupPackageEntry(manifestFile, &header); err != nil {
		return backupPackageInspection{}, fmt.Errorf(`parse backup manifest failed: %w`, err)
	}
	format := backupPackageJSONFieldString(header, `format`)
	switch format {
	case backup.PackageFormat:
		var manifest backup.Manifest
		if err := decodeBackupPackageEntry(manifestFile, &manifest); err != nil {
			return backupPackageInspection{}, fmt.Errorf(`parse full backup manifest failed: %w`, err)
		}
		if manifest.Format != backup.PackageFormat {
			return backupPackageInspection{}, fmt.Errorf(`unsupported backup format: %s`, manifest.Format)
		}
		if manifest.ManifestVersion != backup.ManifestVersion {
			return backupPackageInspection{}, fmt.Errorf(`unsupported backup manifest version: %d`, manifest.ManifestVersion)
		}
		normalizedEntries, err := backupNormalizeAndValidateManifestEntries(manifest)
		if err != nil {
			return backupPackageInspection{}, err
		}
		includedEntries, skippedEntries, err := validateBackupManifestArchive(reader.File, normalizedEntries)
		if err != nil {
			return backupPackageInspection{}, err
		}
		return backupPackageInspection{
			info:            backupPackageInfo{PackageType: `full`},
			fullManifest:    &manifest,
			includedEntries: includedEntries,
			skippedEntries:  skippedEntries,
			fileCount:       countBackupPackageFiles(reader.File),
		}, nil
	case profilePackageFormat:
		var manifest ProfilePackageManifest
		if err := decodeBackupPackageEntry(manifestFile, &manifest); err != nil {
			return backupPackageInspection{}, fmt.Errorf(`parse profile backup manifest failed: %w`, err)
		}
		info, err := inspectProfilePackageManifest(reader.File, manifest)
		if err != nil {
			return backupPackageInspection{}, err
		}
		return backupPackageInspection{
			info:            info,
			profileManifest: &manifest,
			fileCount:       countBackupPackageFiles(reader.File),
		}, nil
	default:
		return backupPackageInspection{}, fmt.Errorf(`unsupported backup format: %s`, format)
	}
}

func validateBackupManifestArchive(files []*zip.File, entries []backupNormalizedManifestEntry) (int, int, error) {
	includedEntries := 0
	skippedEntries := 0
	for _, normalized := range entries {
		entry := normalized.entry
		present, wrongType, matchCount := findBackupArchiveEntryState(files, normalized.archivePath, entry.EntryType)
		if wrongType {
			return 0, 0, fmt.Errorf(`备份条目类型与归档内容不匹配(%s)`, entry.ID)
		}
		if entry.EntryType == backup.EntryTypeFile && matchCount > 1 {
			return 0, 0, fmt.Errorf(`备份包包含重复文件条目: %s`, normalized.archivePath)
		}
		if !present {
			if !entry.Required {
				skippedEntries++
				continue
			}
			return 0, 0, fmt.Errorf(`备份包缺少必需条目: %s`, entry.ID)
		}
		includedEntries++
	}
	return includedEntries, skippedEntries, nil
}

func findBackupArchiveEntryState(files []*zip.File, archivePath string, entryType backup.EntryType) (bool, bool, int) {
	base := strings.TrimSuffix(filepath.ToSlash(archivePath), `/`)
	present := false
	wrongType := false
	matchCount := 0
	for _, file := range files {
		name := filepath.ToSlash(strings.TrimSpace(file.Name))
		if name == `` {
			continue
		}
		isDirectory := strings.HasSuffix(name, `/`) || file.FileInfo().IsDir()
		normalizedName := strings.TrimSuffix(name, `/`)
		if entryType == backup.EntryTypeFile {
			if normalizedName != base {
				continue
			}
			if isDirectory {
				wrongType = true
				continue
			}
			present = true
			matchCount++
			continue
		}
		if normalizedName == base {
			if isDirectory {
				present = true
			} else {
				wrongType = true
			}
			continue
		}
		if strings.HasPrefix(name, base+`/`) {
			present = true
		}
	}
	return present, wrongType, matchCount
}

func countBackupPackageFiles(files []*zip.File) int {
	count := 0
	for _, file := range files {
		name := filepath.ToSlash(strings.TrimSpace(file.Name))
		if name == `` || strings.HasSuffix(name, `/`) || file.FileInfo().IsDir() {
			continue
		}
		count++
	}
	return count
}

func inspectProfilePackageManifest(files []*zip.File, manifest ProfilePackageManifest) (backupPackageInfo, error) {
	if manifest.Format != profilePackageFormat {
		return backupPackageInfo{}, fmt.Errorf(`unsupported profile backup format: %s`, manifest.Format)
	}
	if manifest.Version != 1 && manifest.Version != profilePackageVersion {
		return backupPackageInfo{}, fmt.Errorf(`unsupported profile backup manifest version: %d`, manifest.Version)
	}
	if manifest.ProfileCount < 0 {
		return backupPackageInfo{}, fmt.Errorf(`profile backup manifest has an invalid profile count`)
	}
	info := backupPackageInfo{
		PackageType:  `profile`,
		ProfileCount: manifest.ProfileCount,
		ProfileNames: normalizeBackupPackageProfileNames(manifest.ProfileNames),
	}
	if len(info.ProfileNames) == 0 {
		info.ProfileNames = inspectProfileNamesFromPackage(files, manifest.Version)
	}
	if info.ProfileCount < len(info.ProfileNames) {
		info.ProfileCount = len(info.ProfileNames)
	}
	return info, nil
}

func backupPackageInfoFields(info backupPackageInfo) map[string]interface{} {
	fields := make(map[string]interface{})
	if info.PackageType != `` {
		fields[`packageType`] = info.PackageType
	}
	if info.ProfileCount > 0 {
		fields[`profileCount`] = info.ProfileCount
	}
	if len(info.ProfileNames) > 0 {
		fields[`profileNames`] = info.ProfileNames
	}
	return fields
}

func findBackupPackageEntry(files []*zip.File, name string) *zip.File {
	for _, file := range files {
		if filepath.ToSlash(file.Name) == name {
			return file
		}
	}
	return nil
}

func decodeBackupPackageEntry(file *zip.File, target any) error {
	if file == nil {
		return fmt.Errorf(`backup package entry is missing`)
	}
	if file.UncompressedSize64 > backupPackageInfoEntryLimit {
		return fmt.Errorf(`backup package entry is too large`)
	}
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	return json.NewDecoder(io.LimitReader(reader, backupPackageInfoEntryLimit)).Decode(target)
}

func inspectProfileNamesFromPackage(files []*zip.File, version int) []string {
	if version >= profilePackageVersion {
		var snapshot map[string]json.RawMessage
		if file := findBackupPackageEntry(files, profilePackageDatabasePath); file != nil {
			if err := decodeBackupPackageEntry(file, &snapshot); err == nil {
				if names := profileNamesFromJSONList(snapshot[`profiles`]); len(names) > 0 {
					return names
				}
			}
		}
	}

	if file := findBackupPackageEntry(files, `profiles.json`); file != nil {
		var profiles json.RawMessage
		if err := decodeBackupPackageEntry(file, &profiles); err == nil {
			return profileNamesFromJSONList(profiles)
		}
	}
	return nil
}

func profileNamesFromJSONList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var profiles []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &profiles); err != nil {
		return nil
	}
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		if name := backupPackageJSONFieldString(profile, `profileName`); name != `` {
			names = append(names, name)
		}
	}
	return normalizeBackupPackageProfileNames(names)
}

func backupPackageJSONFieldString(fields map[string]json.RawMessage, key string) string {
	var value string
	if err := json.Unmarshal(fields[key], &value); err != nil {
		return ``
	}
	return strings.TrimSpace(value)
}

func normalizeBackupPackageProfileNames(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	result := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == `` {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}
