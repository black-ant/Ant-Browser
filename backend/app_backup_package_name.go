package backend

import (
	"fmt"
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
