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
}

type backupLocalOpenListSecrets struct {
	Token string `yaml:"token,omitempty"`
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
		left.Schedule.Enabled == right.Schedule.Enabled &&
		left.Schedule.DailyTime == right.Schedule.DailyTime
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
	if token == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除本地备份配置失败: %w", err)
		}
		return nil
	}

	stored := backupLocalConfigFile{
		Backup: backupLocalSecrets{
			Channels: backupLocalChannelsSecrets{
				OpenList: backupLocalOpenListSecrets{Token: token},
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
		if base.Channels.OpenList.Token == "" {
			return err
		}
		settings = base
	}
	if settings.Channels.OpenList.Token != "" {
		if err := saveBackupLocalConfig(path, settings); err != nil {
			return fmt.Errorf("迁移旧版本地备份配置失败: %w", err)
		}
	}

	settings = normalizeBackupSettings(settings)
	if backupConfigsEqual(a.config.Backup, settings) && base.Channels.OpenList.Token == "" {
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
