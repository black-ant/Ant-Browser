package backend

import (
	"ant-chrome/backend/internal/backup"
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

const backupPackageInfoEntryLimit = 16 * 1024 * 1024

type backupPackageInfo struct {
	PackageType  string
	ProfileCount int
	ProfileNames []string
}

func inspectBackupPackageInfo(zipPath string) (backupPackageInfo, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return backupPackageInfo{}, fmt.Errorf(`open backup package failed: %w`, err)
	}
	defer reader.Close()

	manifestFile := findBackupPackageEntry(reader.File, `manifest.json`)
	if manifestFile == nil {
		return backupPackageInfo{}, fmt.Errorf(`backup package has no manifest.json`)
	}
	var header map[string]json.RawMessage
	if err := decodeBackupPackageEntry(manifestFile, &header); err != nil {
		return backupPackageInfo{}, fmt.Errorf(`parse backup manifest failed: %w`, err)
	}
	format := backupPackageJSONFieldString(header, `format`)
	switch format {
	case backup.PackageFormat:
		return backupPackageInfo{PackageType: `full`}, nil
	case profilePackageFormat:
		info := backupPackageInfo{
			PackageType:  `profile`,
			ProfileCount: backupPackageJSONFieldInt(header, `profileCount`),
			ProfileNames: backupPackageJSONFieldStringList(header, `profileNames`),
		}
		if len(info.ProfileNames) == 0 {
			info.ProfileNames = inspectProfileNamesFromPackage(reader.File, backupPackageJSONFieldInt(header, `version`))
		}
		if info.ProfileCount < len(info.ProfileNames) {
			info.ProfileCount = len(info.ProfileNames)
		}
		return info, nil
	default:
		return backupPackageInfo{}, fmt.Errorf(`unsupported backup format: %s`, format)
	}
}

func backupPackageInfoFromFileName(fileName string) backupPackageInfo {
	baseName := filepath.Base(strings.TrimSpace(fileName))
	lowerName := strings.ToLower(baseName)
	const singlePrefix = `ant-chrome-profile-backup-single--`
	const multiPrefix = `ant-chrome-profile-backup-multi-`
	const profilePrefix = `ant-chrome-profile-backup-`
	const fullPrefix = `ant-chrome-backup-`

	if strings.HasPrefix(lowerName, singlePrefix) {
		name := strings.TrimSuffix(baseName[len(singlePrefix):], filepath.Ext(baseName))
		if separator := strings.LastIndex(name, `--`); separator > 0 {
			name = strings.TrimSpace(name[:separator])
		}
		info := backupPackageInfo{PackageType: `profile`, ProfileCount: 1}
		if name != `` && !strings.EqualFold(name, `未命名实例`) {
			info.ProfileNames = []string{name}
		}
		return info
	}
	if strings.HasPrefix(lowerName, multiPrefix) {
		countPart := baseName[len(multiPrefix):]
		if separator := strings.Index(countPart, `--`); separator >= 0 {
			countPart = countPart[:separator]
		}
		count, _ := strconv.Atoi(countPart)
		return backupPackageInfo{PackageType: `profile`, ProfileCount: count}
	}
	if strings.HasPrefix(lowerName, profilePrefix) {
		return backupPackageInfo{PackageType: `profile`}
	}
	if strings.HasPrefix(lowerName, fullPrefix) {
		return backupPackageInfo{PackageType: `full`}
	}
	return backupPackageInfo{}
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

func backupPackageJSONFieldStringList(fields map[string]json.RawMessage, key string) []string {
	var values []string
	if err := json.Unmarshal(fields[key], &values); err != nil {
		return nil
	}
	return normalizeBackupPackageProfileNames(values)
}

func backupPackageJSONFieldInt(fields map[string]json.RawMessage, key string) int {
	var value int
	if err := json.Unmarshal(fields[key], &value); err != nil || value < 0 {
		return 0
	}
	return value
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
