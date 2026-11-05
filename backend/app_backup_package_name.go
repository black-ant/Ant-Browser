package backend

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func backupProfilePackageFileName(profileNames []string, now time.Time, precise bool) string {
	timestampFormat := "20060102-150405"
	if precise {
		timestampFormat += ".000000000"
	}
	timestamp := now.Format(timestampFormat)
	switch len(profileNames) {
	case 1:
		return fmt.Sprintf("ant-chrome-profile-backup-single--%s--%s.zip", sanitizeBackupFileNamePart(profileNames[0]), timestamp)
	case 0:
		return fmt.Sprintf("ant-chrome-profile-backup--%s.zip", timestamp)
	default:
		return fmt.Sprintf("ant-chrome-profile-backup-multi-%d--%s.zip", len(profileNames), timestamp)
	}
}

func sanitizeBackupFileNamePart(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, char := range value {
		if unicode.IsControl(char) || strings.ContainsRune(`<>:"/\\|?*`, char) {
			builder.WriteRune('_')
			continue
		}
		builder.WriteRune(char)
	}
	value = strings.TrimSpace(strings.TrimRight(builder.String(), "."))
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	if value == "" {
		return "未命名实例"
	}
	runes := []rune(value)
	if len(runes) > 80 {
		value = string(runes[:80])
	}
	return value
}

func backupPackageInfoFromFileName(fileName string) backupPackageInfo {
	name := filepath.Base(strings.ReplaceAll(strings.TrimSpace(fileName), `\`, `/`))
	if !strings.EqualFold(filepath.Ext(name), `.zip`) {
		return backupPackageInfo{}
	}

	const zipSuffix = `.zip`
	lowerName := strings.ToLower(name)
	if strings.HasPrefix(lowerName, `ant-chrome-backup-`) {
		return backupPackageInfo{PackageType: `full`}
	}

	const singlePrefix = `ant-chrome-profile-backup-single--`
	if strings.HasPrefix(lowerName, singlePrefix) {
		payload := name[len(singlePrefix) : len(name)-len(zipSuffix)]
		if separator := strings.LastIndex(payload, `--`); separator > 0 {
			profileName := strings.TrimSpace(payload[:separator])
			if profileName != `` {
				return backupPackageInfo{
					PackageType:  `profile`,
					ProfileCount: 1,
					ProfileNames: []string{profileName},
				}
			}
		}
		return backupPackageInfo{PackageType: `profile`, ProfileCount: 1}
	}

	const multiPrefix = `ant-chrome-profile-backup-multi-`
	if strings.HasPrefix(lowerName, multiPrefix) {
		payload := name[len(multiPrefix) : len(name)-len(zipSuffix)]
		if separator := strings.Index(payload, `--`); separator > 0 {
			profileCount, err := strconv.Atoi(payload[:separator])
			if err == nil && profileCount > 0 {
				return backupPackageInfo{PackageType: `profile`, ProfileCount: profileCount}
			}
		}
		return backupPackageInfo{PackageType: `profile`}
	}

	if strings.HasPrefix(lowerName, `ant-chrome-profile-backup--`) {
		return backupPackageInfo{PackageType: `profile`}
	}
	return backupPackageInfo{}
}
