package backend

import (
	"ant-chrome/backend/internal/backup/channels/s3"
	"ant-chrome/backend/internal/config"
	"context"
	"fmt"
	"time"
)

func (a *App) BackupS3Test(input map[string]string) (map[string]interface{}, error) {
	client, err := a.backupS3Client(input)
	if err != nil {
		return nil, err
	}
	ctx, cancel := a.backupS3Context(s3.ControlTimeout)
	defer cancel()
	if err := client.Test(ctx); err != nil {
		return nil, fmt.Errorf("S3 连接测试失败：%w", err)
	}
	return map[string]interface{}{
		"ok":      true,
		"message": "S3 连接测试成功",
	}, nil
}

func (a *App) backupS3Client(input map[string]string) (*s3.Client, error) {
	settings, err := a.backupResolvedS3Config(input)
	if err != nil {
		return nil, err
	}
	return newS3Client(settings)
}

func newS3Client(settings config.S3ChannelConfig) (*s3.Client, error) {
	return s3.NewClient(s3.Config{
		Endpoint:        settings.Endpoint,
		Region:          settings.Region,
		Bucket:          settings.Bucket,
		AccessKeyID:     settings.AccessKeyID,
		SecretAccessKey: settings.SecretAccessKey,
		SessionToken:    settings.SessionToken,
		ForcePathStyle:  settings.ForcePathStyle,
	})
}

func (a *App) backupResolvedS3Config(input map[string]string) (config.S3ChannelConfig, error) {
	settings := a.backupStoredS3Config()
	if err := applyS3InputToConfig(&settings, input); err != nil {
		return config.S3ChannelConfig{}, err
	}
	return settings, nil
}

func applyS3InputToConfig(settings *config.S3ChannelConfig, input map[string]string) error {
	if settings == nil {
		return nil
	}
	backupConfig := config.BackupConfig{}
	backupConfig.Channels.S3 = *settings
	if err := applyS3Input(&backupConfig, input); err != nil {
		return err
	}
	*settings = normalizeBackupSettings(backupConfig).Channels.S3
	return nil
}

func (a *App) backupStoredS3Config() config.S3ChannelConfig {
	if a == nil {
		return config.DefaultConfig().Backup.Channels.S3
	}
	if a.backupScheduler != nil {
		a.backupScheduler.mu.RLock()
		defer a.backupScheduler.mu.RUnlock()
		return a.backupScheduler.settings.Channels.S3
	}
	if a.config != nil {
		return a.config.Backup.Channels.S3
	}
	return config.DefaultConfig().Backup.Channels.S3
}

func (a *App) backupS3Context(timeout time.Duration) (context.Context, context.CancelFunc) {
	parent := context.Background()
	if a != nil && a.ctx != nil {
		parent = a.ctx
	}
	return context.WithTimeout(parent, timeout)
}
