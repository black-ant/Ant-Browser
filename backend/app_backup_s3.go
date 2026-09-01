package backend

import (
	"ant-chrome/backend/internal/backup/channels/s3"
	"ant-chrome/backend/internal/config"
	"context"
	"fmt"
	"os"
	"path/filepath"
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

func (a *App) BackupS3List(input map[string]string) ([]map[string]interface{}, error) {
	client, err := a.backupS3Client(input)
	if err != nil {
		return nil, err
	}
	ctx, cancel := a.backupS3Context(s3.ControlTimeout)
	defer cancel()
	items, err := client.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("S3 \u5907\u4efd\u5217\u8868\u8bfb\u53d6\u5931\u8d25\uff1a%w", err)
	}
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		entry := map[string]interface{}{
			"name":       item.Name,
			"size":       item.Size,
			"modifiedAt": item.ModifiedAt,
		}
		for key, value := range backupPackageInfoFields(backupPackageInfoFromFileName(item.Name)) {
			entry[key] = value
		}
		result = append(result, entry)
	}
	return result, nil
}

func (a *App) backupS3Client(input map[string]string) (*s3.Client, error) {
	settings, err := a.backupResolvedS3Config(input)
	if err != nil {
		return nil, err
	}
	return newS3Client(settings)
}

func (a *App) BackupS3Upload(input map[string]string) (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	return a.backupS3UploadLocked(input)
}

func (a *App) backupS3UploadLocked(input map[string]string) (map[string]interface{}, error) {
	client, err := a.backupS3Client(input)
	if err != nil {
		return nil, err
	}
	temporaryRoot, err := os.MkdirTemp("", "ant-chrome-s3-upload-")
	if err != nil {
		return nil, fmt.Errorf("\u521b\u5efa\u4e34\u65f6\u5907\u4efd\u76ee\u5f55\u5931\u8d25\uff1a%w", err)
	}
	defer os.RemoveAll(temporaryRoot)

	fileName := fmt.Sprintf("ant-chrome-backup-%s.zip", time.Now().Format("20060102-150405.000000000"))
	localPath := filepath.Join(temporaryRoot, fileName)
	result, err := a.backupExportPackageToPath(localPath)
	if err != nil {
		return nil, err
	}
	remoteFile, err := a.backupUploadRemoteArtifacts(backupRemoteUploadTarget{
		label:   "S3",
		client:  client,
		timeout: s3.TransferTimeout,
	}, localPath, fileName)
	if err != nil {
		a.backupEmitExportProgress("error", 100, err.Error())
		return nil, err
	}
	result["remoteName"] = remoteFile.Name
	result["remoteSize"] = remoteFile.Size
	result["remoteUploaded"] = true
	result["message"] = "S3 \u5907\u4efd\u5b8c\u6210"
	a.backupEmitExportProgress("done", 100, result["message"].(string))
	return result, nil
}

func (a *App) BackupS3Restore(input map[string]string, fileName string) (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	client, err := a.backupS3Client(input)
	if err != nil {
		return nil, err
	}
	return a.backupRestoreRemoteLocked(client, "S3", s3.TransferTimeout, fileName, "ant-chrome-s3-restore-")
}

func (a *App) BackupS3Download(input map[string]string, fileName string) (map[string]interface{}, error) {
	client, err := a.backupS3Client(input)
	if err != nil {
		return nil, err
	}
	return a.backupDownloadRemoteFile(client, "S3", s3.TransferTimeout, fileName)
}

func newS3Client(settings config.S3ChannelConfig) (*s3.Client, error) {
	return s3.NewClient(s3.Config{
		Endpoint:        settings.Endpoint,
		Region:          settings.Region,
		Bucket:          settings.Bucket,
		Prefix:          settings.Prefix,
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
