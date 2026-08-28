package backend

import (
	"ant-chrome/backend/internal/backup/channels"
	"ant-chrome/backend/internal/backup/channels/openlist"
	"ant-chrome/backend/internal/config"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (a *App) BackupOpenListTest(input map[string]string) (map[string]interface{}, error) {
	client, err := a.backupOpenListClient(input)
	if err != nil {
		return nil, err
	}
	ctx, cancel := a.backupOpenListContext(openlist.ControlTimeout)
	defer cancel()
	if err := client.Test(ctx); err != nil {
		return nil, fmt.Errorf(`OpenList connection test failed: %w`, err)
	}
	return map[string]interface{}{
		`ok`:      true,
		`message`: `OpenList connection test passed`,
	}, nil
}

func (a *App) BackupOpenListList(input map[string]string) ([]map[string]interface{}, error) {
	client, err := a.backupOpenListClient(input)
	if err != nil {
		return nil, err
	}
	ctx, cancel := a.backupOpenListContext(openlist.ControlTimeout)
	defer cancel()
	items, err := client.List(ctx)
	if err != nil {
		return nil, fmt.Errorf(`list OpenList backups failed: %w`, err)
	}
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]interface{}{
			`name`:       item.Name,
			`size`:       item.Size,
			`modifiedAt`: item.ModifiedAt,
		})
	}
	return result, nil
}

func (a *App) BackupOpenListUpload(input map[string]string) (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	return a.backupOpenListUploadLocked(input)
}

func (a *App) backupOpenListUploadLocked(input map[string]string) (map[string]interface{}, error) {
	openListConfig, client, err := a.backupOpenListClientWithConfig(input)
	if err != nil {
		return nil, err
	}
	temporaryRoot, err := os.MkdirTemp(``, `ant-chrome-openlist-upload-`)
	if err != nil {
		return nil, fmt.Errorf(`create temporary backup directory failed: %w`, err)
	}
	defer os.RemoveAll(temporaryRoot)

	fileName := fmt.Sprintf(`ant-chrome-backup-%s.zip`, time.Now().Format(`20060102-150405.000000000`))
	localPath := filepath.Join(temporaryRoot, fileName)
	result, err := a.backupExportPackageToPath(localPath)
	if err != nil {
		return nil, err
	}
	uploadMessage, err := backupOpenListUploadProgressMessage(localPath, `备份文件`, openListConfig.UploadRateLimitMBps)
	if err != nil {
		a.backupEmitExportProgress(`error`, 100, err.Error())
		return nil, err
	}
	a.backupEmitExportProgress(`uploading`, 96, uploadMessage)
	ctx, cancel := a.backupOpenListContext(openlist.TransferTimeout)
	remoteFile, err := client.Upload(ctx, localPath, fileName)
	cancel()
	if err != nil {
		a.backupEmitExportProgress(`error`, 100, err.Error())
		return nil, err
	}
	result[`remoteName`] = remoteFile.Name
	result[`remoteSize`] = remoteFile.Size
	metadataPath := backupMetadataPath(localPath)
	metadataName := filepath.Base(metadataPath)
	metadataMessage, err := backupOpenListUploadProgressMessage(metadataPath, `备份元数据`, openListConfig.UploadRateLimitMBps)
	if err != nil {
		a.backupEmitExportProgress(`error`, 100, err.Error())
		return nil, err
	}
	a.backupEmitExportProgress(`uploading`, 98, metadataMessage)
	metadataContext, metadataCancel := a.backupOpenListContext(openlist.TransferTimeout)
	if _, err := client.UploadMetadata(metadataContext, metadataPath, metadataName); err != nil {
		metadataCancel()
		a.backupEmitExportProgress(`error`, 100, err.Error())
		return nil, err
	}
	metadataCancel()
	a.backupEmitExportProgress(`done`, 100, `backup uploaded to OpenList`)
	result[`message`] = `backup uploaded to OpenList`
	return result, nil
}

func (a *App) BackupOpenListRestore(input map[string]string, fileName string) (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	client, err := a.backupOpenListClient(input)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(fileName) == `` {
		return nil, fmt.Errorf(`remote backup file name is empty`)
	}
	temporaryRoot, err := os.MkdirTemp(``, `ant-chrome-openlist-restore-`)
	if err != nil {
		return nil, fmt.Errorf(`create temporary restore directory failed: %w`, err)
	}
	defer os.RemoveAll(temporaryRoot)
	localPath := filepath.Join(temporaryRoot, `remote-backup.zip`)
	a.backupEmitImportProgress(`preparing`, 5, `downloading backup from OpenList`)
	ctx, cancel := a.backupOpenListContext(openlist.TransferTimeout)
	defer cancel()
	if err := client.Download(ctx, fileName, localPath); err != nil {
		a.backupEmitImportProgress(`error`, 100, err.Error())
		return nil, err
	}
	result, err := a.backupImportFromPathLocked(localPath)
	if err != nil {
		a.backupEmitImportProgress(`error`, 100, fmt.Sprintf(`restore remote backup failed: %v`, err))
		return nil, err
	}
	result[`remoteName`] = fileName
	return result, nil
}

func (a *App) backupOpenListClient(input map[string]string) (channels.Client, error) {
	_, client, err := a.backupOpenListClientWithConfig(input)
	return client, err
}

func (a *App) backupOpenListClientWithConfig(input map[string]string) (config.OpenListChannelConfig, channels.Client, error) {
	settings, err := a.backupResolvedOpenListConfig(input)
	if err != nil {
		return config.OpenListChannelConfig{}, nil, err
	}
	client, err := openlist.NewClient(openlist.Config{
		BaseURL:             settings.BaseURL,
		RemotePath:          settings.RemotePath,
		Token:               settings.Token,
		UploadRateLimitMBps: settings.UploadRateLimitMBps,
	})
	if err != nil {
		return settings, nil, err
	}
	return settings, client, nil
}

func (a *App) backupResolvedOpenListConfig(input map[string]string) (config.OpenListChannelConfig, error) {
	stored := a.backupStoredOpenListConfig()
	settings := stored
	baseURL := backupOpenListInputValue(input, `baseURL`, `baseUrl`)
	if baseURL == `` {
		baseURL = settings.BaseURL
	}
	remotePath, remotePathProvided := backupOpenListInputValueWithPresence(input, `remotePath`, `path`)
	if !remotePathProvided {
		remotePath = settings.RemotePath
	}
	token := backupOpenListInputValue(input, `token`)
	if token == `` {
		token = settings.Token
	}
	settings.BaseURL = baseURL
	settings.RemotePath = remotePath
	settings.Token = token
	if value := backupOpenListInputValue(input, `uploadRateLimitMBps`, `uploadRateLimitMbps`, `upload_rate_limit_mbps`); value != `` {
		rateLimit, err := strconv.Atoi(value)
		if err != nil || rateLimit < 0 {
			return config.OpenListChannelConfig{}, fmt.Errorf(`OpenList 上传限速必须是非负整数 MB/s`)
		}
		settings.UploadRateLimitMBps = rateLimit
	}
	return settings, nil
}

func backupOpenListUploadProgressMessage(localPath, artifactName string, uploadRateLimitMBps int) (string, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return ``, fmt.Errorf(`读取%s大小失败: %w`, artifactName, err)
	}
	if info.IsDir() {
		return ``, fmt.Errorf(`%s路径是目录`, artifactName)
	}
	rateDescription := `不限速`
	if uploadRateLimitMBps > 0 {
		rateDescription = fmt.Sprintf(`%d MB/s`, uploadRateLimitMBps)
	}
	return fmt.Sprintf(`准备上传%s到 OpenList：文件大小 %s（%d bytes），上传限速 %s`, artifactName, formatBackupFileSize(info.Size()), info.Size(), rateDescription), nil
}

func formatBackupFileSize(size int64) string {
	if size < 0 {
		return `未知`
	}
	if size < 1024 {
		return fmt.Sprintf(`%d B`, size)
	}
	value := float64(size)
	units := []string{`KB`, `MB`, `GB`, `TB`}
	unitIndex := 0
	for value >= 1024 && unitIndex < len(units)-1 {
		value /= 1024
		unitIndex++
	}
	return fmt.Sprintf(`%.2f %s`, value, units[unitIndex])
}

func (a *App) backupStoredOpenListConfig() config.OpenListChannelConfig {
	if a == nil {
		return config.DefaultConfig().Backup.Channels.OpenList
	}
	if a.backupScheduler != nil {
		a.backupScheduler.mu.RLock()
		defer a.backupScheduler.mu.RUnlock()
		return a.backupScheduler.settings.Channels.OpenList
	}
	if a.config != nil {
		return a.config.Backup.Channels.OpenList
	}
	return config.DefaultConfig().Backup.Channels.OpenList
}

func backupOpenListInputValue(input map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(input[key]); value != `` {
			return value
		}
	}
	return ``
}

func backupOpenListInputValueWithPresence(input map[string]string, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := input[key]; ok {
			return strings.TrimSpace(value), true
		}
	}
	return ``, false
}

func (a *App) backupOpenListContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	parent := context.Background()
	if a != nil && a.ctx != nil {
		parent = a.ctx
	}
	return context.WithTimeout(parent, timeout)
}
