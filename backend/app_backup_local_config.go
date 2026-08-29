package backend

import (
	"ant-chrome/backend/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const backupLocalConfigFileName = "backup.local.yaml"

type backupLocalConfigFile struct {
	Backup backupLocalSecrets `yaml:"backup"`
}

type backupLocalSecrets struct {
	Channels backupLocalChannelsSecrets `yaml:"channels"`
}

type backupLocalChannelsSecrets struct {
	OpenList backupLocalOpenListSecrets `yaml:"openlist"`
	S3       backupLocalS3Config        `yaml:"s3,omitempty"`
}

type backupLocalOpenListSecrets struct {
	Token string `yaml:"token,omitempty"`
}

type backupLocalS3Config struct {
	Endpoint        string `yaml:"endpoint,omitempty"`
	Region          string `yaml:"region,omitempty"`
	Bucket          string `yaml:"bucket,omitempty"`
	Prefix          string `yaml:"prefix,omitempty"`
	AccessKeyID     string `yaml:"access_key_id,omitempty"`
	SecretAccessKey string `yaml:"secret_access_key,omitempty"`
	SessionToken    string `yaml:"session_token,omitempty"`
	ForcePathStyle  bool   `yaml:"force_path_style,omitempty"`
}

type backupLocalS3ConfigRead struct {
	Endpoint        string `yaml:"endpoint,omitempty"`
	Region          string `yaml:"region,omitempty"`
	Bucket          string `yaml:"bucket,omitempty"`
	Prefix          string `yaml:"prefix,omitempty"`
	AccessKeyID     string `yaml:"access_key_id,omitempty"`
	SecretAccessKey string `yaml:"secret_access_key,omitempty"`
	SessionToken    string `yaml:"session_token,omitempty"`
	ForcePathStyle  *bool  `yaml:"force_path_style,omitempty"`
}

type backupLocalConfigReadFile struct {
	Backup backupLocalConfigReadBackup `yaml:"backup"`
}

type backupLocalConfigReadBackup struct {
	Channels backupLocalConfigReadChannels `yaml:"channels"`
	OpenList config.OpenListChannelConfig  `yaml:"openlist"`
	Schedule config.BackupScheduleConfig   `yaml:"schedule"`
}

type backupLocalConfigReadChannels struct {
	OpenList config.OpenListChannelConfig `yaml:"openlist"`
	S3       backupLocalS3ConfigRead      `yaml:"s3"`
}

func defaultBackupConfig() config.BackupConfig {
	return config.DefaultConfig().Backup
}

func normalizeBackupSettings(value config.BackupConfig) config.BackupConfig {
	defaults := defaultBackupConfig()
	value.Channels.OpenList.BaseURL = strings.TrimSpace(value.Channels.OpenList.BaseURL)
	value.Channels.OpenList.RemotePath = strings.TrimSpace(value.Channels.OpenList.RemotePath)
	if value.Channels.OpenList.RemotePath == "" {
		value.Channels.OpenList.RemotePath = defaults.Channels.OpenList.RemotePath
	}
	value.Channels.OpenList.Token = strings.TrimSpace(value.Channels.OpenList.Token)
	value.Channels.S3.Endpoint = strings.TrimSpace(value.Channels.S3.Endpoint)
	value.Channels.S3.Region = strings.TrimSpace(value.Channels.S3.Region)
	if value.Channels.S3.Region == "" {
		value.Channels.S3.Region = defaults.Channels.S3.Region
	}
	value.Channels.S3.Bucket = strings.TrimSpace(value.Channels.S3.Bucket)
	value.Channels.S3.Prefix = strings.TrimSpace(value.Channels.S3.Prefix)
	value.Channels.S3.AccessKeyID = strings.TrimSpace(value.Channels.S3.AccessKeyID)
	value.Channels.S3.SecretAccessKey = strings.TrimSpace(value.Channels.S3.SecretAccessKey)
	value.Channels.S3.SessionToken = strings.TrimSpace(value.Channels.S3.SessionToken)
	value.Schedule.DailyTime = strings.TrimSpace(value.Schedule.DailyTime)
	if value.Schedule.DailyTime == "" {
		value.Schedule.DailyTime = defaults.Schedule.DailyTime
	}
	return value
}

func backupConfigsEqual(left, right config.BackupConfig) bool {
	left = normalizeBackupSettings(left)
	right = normalizeBackupSettings(right)
	return left.Channels.OpenList.BaseURL == right.Channels.OpenList.BaseURL &&
		left.Channels.OpenList.RemotePath == right.Channels.OpenList.RemotePath &&
		left.Channels.OpenList.Token == right.Channels.OpenList.Token &&
		left.Channels.OpenList.UploadRateLimitMBps == right.Channels.OpenList.UploadRateLimitMBps &&
		left.Channels.S3.Endpoint == right.Channels.S3.Endpoint &&
		left.Channels.S3.Region == right.Channels.S3.Region &&
		left.Channels.S3.Bucket == right.Channels.S3.Bucket &&
		left.Channels.S3.Prefix == right.Channels.S3.Prefix &&
		left.Channels.S3.AccessKeyID == right.Channels.S3.AccessKeyID &&
		left.Channels.S3.SecretAccessKey == right.Channels.S3.SecretAccessKey &&
		left.Channels.S3.SessionToken == right.Channels.S3.SessionToken &&
		left.Channels.S3.ForcePathStyle == right.Channels.S3.ForcePathStyle &&
		left.Schedule.Enabled == right.Schedule.Enabled &&
		left.Schedule.DailyTime == right.Schedule.DailyTime
}

func backupHasLocalSecrets(value config.BackupConfig) bool {
	return strings.TrimSpace(value.Channels.OpenList.Token) != "" ||
		strings.TrimSpace(value.Channels.S3.AccessKeyID) != "" ||
		strings.TrimSpace(value.Channels.S3.SecretAccessKey) != "" ||
		strings.TrimSpace(value.Channels.S3.SessionToken) != ""
}

func backupHasS3Config(value config.BackupConfig) bool {
	s3 := value.Channels.S3
	defaultRegion := defaultBackupConfig().Channels.S3.Region
	return strings.TrimSpace(s3.Endpoint) != "" ||
		(strings.TrimSpace(s3.Region) != "" && strings.TrimSpace(s3.Region) != strings.TrimSpace(defaultRegion)) ||
		strings.TrimSpace(s3.Bucket) != "" ||
		strings.TrimSpace(s3.Prefix) != "" ||
		strings.TrimSpace(s3.AccessKeyID) != "" ||
		strings.TrimSpace(s3.SecretAccessKey) != "" ||
		strings.TrimSpace(s3.SessionToken) != "" ||
		s3.ForcePathStyle
}

func backupHasLocalConfig(value config.BackupConfig) bool {
	return backupHasLocalSecrets(value) || backupHasS3Config(value)
}

func loadBackupLocalConfig(path string, base config.BackupConfig) (config.BackupConfig, bool, error) {
	settings := normalizeBackupSettings(base)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return settings, false, nil
		}
		return settings, true, fmt.Errorf("读取本地备份配置失败: %w", err)
	}

	var stored backupLocalConfigReadFile
	if err := yaml.Unmarshal(data, &stored); err != nil {
		return settings, true, fmt.Errorf("解析本地备份配置失败: %w", err)
	}

	localOpenList := stored.Backup.Channels.OpenList
	legacyOpenList := stored.Backup.OpenList
	if strings.TrimSpace(localOpenList.BaseURL) == "" {
		localOpenList.BaseURL = legacyOpenList.BaseURL
	}
	if strings.TrimSpace(localOpenList.RemotePath) == "" {
		localOpenList.RemotePath = legacyOpenList.RemotePath
	}
	if strings.TrimSpace(localOpenList.Token) == "" {
		localOpenList.Token = legacyOpenList.Token
	}
	localS3 := stored.Backup.Channels.S3

	if strings.TrimSpace(base.Channels.OpenList.BaseURL) == "" {
		settings.Channels.OpenList.BaseURL = strings.TrimSpace(localOpenList.BaseURL)
	}
	defaults := defaultBackupConfig()
	if strings.TrimSpace(base.Channels.OpenList.RemotePath) == "" ||
		(strings.TrimSpace(base.Channels.OpenList.RemotePath) == defaults.Channels.OpenList.RemotePath && strings.TrimSpace(localOpenList.RemotePath) != "" && strings.TrimSpace(localOpenList.RemotePath) != defaults.Channels.OpenList.RemotePath) {
		settings.Channels.OpenList.RemotePath = strings.TrimSpace(localOpenList.RemotePath)
	}
	if strings.TrimSpace(localOpenList.Token) != "" {
		settings.Channels.OpenList.Token = strings.TrimSpace(localOpenList.Token)
	}
	if value := strings.TrimSpace(localS3.Endpoint); value != "" {
		settings.Channels.S3.Endpoint = value
	}
	if value := strings.TrimSpace(localS3.Region); value != "" {
		settings.Channels.S3.Region = value
	}
	if value := strings.TrimSpace(localS3.Bucket); value != "" {
		settings.Channels.S3.Bucket = value
	}
	if value := strings.TrimSpace(localS3.Prefix); value != "" {
		settings.Channels.S3.Prefix = value
	}
	if value := strings.TrimSpace(localS3.AccessKeyID); value != "" {
		settings.Channels.S3.AccessKeyID = value
	}
	if value := strings.TrimSpace(localS3.SecretAccessKey); value != "" {
		settings.Channels.S3.SecretAccessKey = value
	}
	if value := strings.TrimSpace(localS3.SessionToken); value != "" {
		settings.Channels.S3.SessionToken = value
	}
	if localS3.ForcePathStyle != nil {
		settings.Channels.S3.ForcePathStyle = *localS3.ForcePathStyle
	}
	if (strings.TrimSpace(base.Schedule.DailyTime) == "" && !base.Schedule.Enabled) ||
		(!base.Schedule.Enabled && strings.TrimSpace(base.Schedule.DailyTime) == defaults.Schedule.DailyTime && strings.TrimSpace(stored.Backup.Schedule.DailyTime) != "" && strings.TrimSpace(stored.Backup.Schedule.DailyTime) != defaults.Schedule.DailyTime) {
		if strings.TrimSpace(stored.Backup.Schedule.DailyTime) != "" || stored.Backup.Schedule.Enabled {
			settings.Schedule = stored.Backup.Schedule
		}
	}

	return normalizeBackupSettings(settings), true, nil
}

func saveBackupLocalConfig(path string, value config.BackupConfig) error {
	token := strings.TrimSpace(value.Channels.OpenList.Token)
	s3Config := backupLocalS3Config{}
	if backupHasS3Config(value) {
		s3Config = backupLocalS3Config{
			Endpoint:        strings.TrimSpace(value.Channels.S3.Endpoint),
			Region:          strings.TrimSpace(value.Channels.S3.Region),
			Bucket:          strings.TrimSpace(value.Channels.S3.Bucket),
			Prefix:          strings.TrimSpace(value.Channels.S3.Prefix),
			AccessKeyID:     strings.TrimSpace(value.Channels.S3.AccessKeyID),
			SecretAccessKey: strings.TrimSpace(value.Channels.S3.SecretAccessKey),
			SessionToken:    strings.TrimSpace(value.Channels.S3.SessionToken),
			ForcePathStyle:  value.Channels.S3.ForcePathStyle,
		}
	}
	if token == "" && !backupHasS3Config(value) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除本地备份配置失败: %w", err)
		}
		return nil
	}

	stored := backupLocalConfigFile{
		Backup: backupLocalSecrets{
			Channels: backupLocalChannelsSecrets{
				OpenList: backupLocalOpenListSecrets{Token: token},
				S3:       s3Config,
			},
		},
	}
	data, err := yaml.Marshal(stored)
	if err != nil {
		return fmt.Errorf("序列化本地备份配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建本地备份配置目录失败: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("写入本地备份配置失败: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("设置本地备份配置权限失败: %w", err)
	}
	return nil
}

func (a *App) prepareBackupLocalConfig() error {
	if a == nil || a.config == nil {
		return fmt.Errorf("应用配置未初始化")
	}

	path := a.resolveAppPath(backupLocalConfigFileName)
	base := normalizeBackupSettings(a.config.Backup)
	settings, _, err := loadBackupLocalConfig(path, a.config.Backup)
	if err != nil {
		if !backupHasLocalSecrets(base) {
			return err
		}
		settings = base
	}
	if backupHasLocalConfig(settings) {
		if err := saveBackupLocalConfig(path, settings); err != nil {
			return fmt.Errorf("迁移旧版本地备份配置失败: %w", err)
		}
	}

	settings = normalizeBackupSettings(settings)
	if backupConfigsEqual(a.config.Backup, settings) && !backupHasLocalConfig(base) {
		return nil
	}
	previous := a.config.Backup
	a.config.Backup = settings
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		a.config.Backup = previous
		return fmt.Errorf("保存备份配置失败: %w", err)
	}
	return nil
}
