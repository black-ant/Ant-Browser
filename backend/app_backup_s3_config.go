package backend

import (
	"ant-chrome/backend/internal/config"
	"fmt"
	"strconv"
	"strings"
)

func (a *App) BackupS3GetSettings() (map[string]interface{}, error) {
	scheduler, err := a.ensureBackupScheduler()
	if err != nil {
		return nil, err
	}
	scheduler.mu.RLock()
	defer scheduler.mu.RUnlock()
	return backupS3SettingsResult(scheduler.settings.Channels.S3), nil
}

func (a *App) BackupS3SaveSettings(input map[string]string) (map[string]interface{}, error) {
	scheduler, err := a.ensureBackupScheduler()
	if err != nil {
		return nil, err
	}
	return scheduler.saveS3(input)
}

func backupS3SettingsResult(settings config.S3ChannelConfig) map[string]interface{} {
	accessKeyIDConfigured := strings.TrimSpace(settings.AccessKeyID) != ""
	secretAccessKeyConfigured := strings.TrimSpace(settings.SecretAccessKey) != ""
	return map[string]interface{}{
		"endpoint":                  settings.Endpoint,
		"region":                    settings.Region,
		"bucket":                    settings.Bucket,
		"prefix":                    settings.Prefix,
		"forcePathStyle":            settings.ForcePathStyle,
		"accessKeyIDConfigured":     accessKeyIDConfigured,
		"secretAccessKeyConfigured": secretAccessKeyConfigured,
		"credentialsConfigured":     accessKeyIDConfigured && secretAccessKeyConfigured,
		"sessionTokenConfigured":    strings.TrimSpace(settings.SessionToken) != "",
	}
}

func (s *backupScheduler) saveS3(input map[string]string) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.app == nil || s.app.config == nil {
		return nil, fmt.Errorf("应用配置未初始化")
	}

	next := normalizeBackupSettings(s.settings)
	if err := applyS3Input(&next, input); err != nil {
		return nil, err
	}
	if _, err := newS3Client(next.Channels.S3); err != nil {
		return nil, fmt.Errorf("S3 配置无效：%w", err)
	}

	if _, err := s.commitSettingsLocked(next); err != nil {
		return nil, err
	}
	return backupS3SettingsResult(s.settings.Channels.S3), nil
}

func applyS3Input(next *config.BackupConfig, input map[string]string) error {
	if next == nil {
		return nil
	}
	if value, ok := backupS3InputValueWithPresence(input, "endpoint", "baseURL", "baseUrl"); ok {
		next.Channels.S3.Endpoint = value
	}
	if value := backupS3InputValue(input, "region"); value != "" {
		next.Channels.S3.Region = value
	}
	if value := backupS3InputValue(input, "bucket"); value != "" {
		next.Channels.S3.Bucket = value
	}
	if value, ok := backupS3InputValueWithPresence(input, "prefix", "remotePath", "path"); ok {
		next.Channels.S3.Prefix = value
	}
	if value, ok := backupS3InputValueWithPresence(input, "forcePathStyle", "force_path_style"); ok {
		forcePathStyle, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("S3 path style 设置无效")
		}
		next.Channels.S3.ForcePathStyle = forcePathStyle
	}
	if value := backupS3InputValue(input, "accessKeyID", "accessKeyId", "access_key_id"); value != "" {
		next.Channels.S3.AccessKeyID = value
	}
	if value := backupS3InputValue(input, "secretAccessKey", "secret_access_key"); value != "" {
		next.Channels.S3.SecretAccessKey = value
	}
	if value, ok := backupS3InputValueWithPresence(input, "sessionToken", "session_token"); ok {
		next.Channels.S3.SessionToken = value
	}
	return nil
}

func backupS3InputValue(input map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(input[key]); value != "" {
			return value
		}
	}
	return ""
}

func backupS3InputValueWithPresence(input map[string]string, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := input[key]; ok {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}
