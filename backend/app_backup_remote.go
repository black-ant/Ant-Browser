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

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type backupRemoteUploadTarget struct {
	label               string
	client              channels.Client
	timeout             time.Duration
	uploadRateLimitMBps int
	skipMetadata        bool
}

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
	remoteFile, err := a.backupUploadRemoteArtifacts(backupRemoteUploadTarget{
		label:               `OpenList`,
		client:              client,
		timeout:             openlist.TransferTimeout,
		uploadRateLimitMBps: openListConfig.UploadRateLimitMBps,
	}, localPath, fileName)
	if err != nil {
		a.backupEmitExportProgress(`error`, 100, err.Error())
		return nil, err
	}
	result[`remoteName`] = remoteFile.Name
	result[`remoteSize`] = remoteFile.Size
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
	return a.backupRestoreRemoteLocked(client, `OpenList`, openlist.TransferTimeout, fileName, `ant-chrome-openlist-restore-`)
}

func (a *App) BackupOpenListDownload(input map[string]string, fileName string) (map[string]interface{}, error) {
	client, err := a.backupOpenListClient(input)
	if err != nil {
		return nil, err
	}
	return a.backupDownloadRemoteFile(client, `OpenList`, openlist.TransferTimeout, fileName)
}

func (a *App) backupDownloadRemoteFile(client channels.Client, label string, timeout time.Duration, fileName string) (map[string]interface{}, error) {
	if a == nil || a.ctx == nil {
		return nil, fmt.Errorf(`应用上下文未初始化`)
	}
	trimmedName := strings.TrimSpace(fileName)
	if trimmedName == `` {
		return nil, fmt.Errorf(`remote backup file name is empty`)
	}
	defaultName := filepath.Base(strings.ReplaceAll(trimmedName, `\`, `/`))
	if defaultName == `` || defaultName == `.` || defaultName == string(filepath.Separator) {
		return nil, fmt.Errorf(`remote backup file name is invalid`)
	}
	if !strings.EqualFold(filepath.Ext(defaultName), `.zip`) {
		return nil, fmt.Errorf(`远程备份必须是 ZIP 文件`)
	}

	savePath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           fmt.Sprintf(`下载%s备份`, label),
		DefaultFilename: defaultName,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: `ZIP 文件 (*.zip)`, Pattern: `*.zip`},
		},
	})
	if err != nil {
		return nil, fmt.Errorf(`打开保存对话框失败: %w`, err)
	}
	if strings.TrimSpace(savePath) == `` {
		return map[string]interface{}{
			`cancelled`: true,
			`message`:   `已取消下载`,
		}, nil
	}
	savePath = backupEnsureZipSuffix(savePath)

	ctx, cancel := a.backupRemoteContext(timeout)
	defer cancel()
	if err := client.Download(ctx, trimmedName, savePath); err != nil {
		return nil, fmt.Errorf(`下载%s备份失败: %w`, label, err)
	}
	return map[string]interface{}{
		`cancelled`:  false,
		`zipPath`:    savePath,
		`remoteName`: trimmedName,
		`message`:    fmt.Sprintf(`已下载%s备份`, label),
	}, nil
}

func (a *App) backupUploadRemoteArtifacts(target backupRemoteUploadTarget, localPath, fileName string) (channels.File, error) {
	uploadMessage, uploadSize, err := backupRemoteUploadProgressMessage(localPath, `备份文件`, target.label, target.uploadRateLimitMBps)
	if err != nil {
		return channels.File{}, err
	}
	a.backupEmitExportProgressTransfer(`uploading`, 96, uploadMessage, channels.UploadProgress{TotalBytes: uploadSize})
	ctx, cancel := a.backupRemoteContext(target.timeout)
	remoteFile, err := backupUploadWithProgress(ctx, target.client, localPath, fileName, a.backupRemoteUploadProgressCallback(target.label, `备份文件`, 96, 98))
	cancel()
	if err != nil {
		return channels.File{}, fmt.Errorf(`上传%s备份文件失败: %w`, target.label, err)
	}
	if target.skipMetadata {
		return remoteFile, nil
	}

	metadataPath := backupMetadataPath(localPath)
	metadataName := filepath.Base(metadataPath)
	metadataMessage, metadataSize, err := backupRemoteUploadProgressMessage(metadataPath, `备份元数据`, target.label, target.uploadRateLimitMBps)
	if err != nil {
		return channels.File{}, err
	}
	a.backupEmitExportProgressTransfer(`uploading`, 98, metadataMessage, channels.UploadProgress{TotalBytes: metadataSize})
	metadataContext, metadataCancel := a.backupRemoteContext(target.timeout)
	_, metadataErr := backupUploadMetadataWithProgress(metadataContext, target.client, metadataPath, metadataName, a.backupRemoteUploadProgressCallback(target.label, `备份元数据`, 98, 99))
	metadataCancel()
	if metadataErr != nil {
		return channels.File{}, fmt.Errorf(`上传%s备份元数据失败: %w`, target.label, metadataErr)
	}
	return remoteFile, nil
}

func (a *App) backupRestoreRemoteLocked(client channels.Client, label string, timeout time.Duration, fileName, temporaryPrefix string) (map[string]interface{}, error) {
	if strings.TrimSpace(fileName) == `` {
		return nil, fmt.Errorf(`remote backup file name is empty`)
	}
	temporaryRoot, err := os.MkdirTemp(``, temporaryPrefix)
	if err != nil {
		return nil, fmt.Errorf(`create temporary restore directory failed: %w`, err)
	}
	defer os.RemoveAll(temporaryRoot)
	localPath := filepath.Join(temporaryRoot, `remote-backup.zip`)
	a.backupEmitImportProgress(`preparing`, 5, fmt.Sprintf(`正在从%s下载备份`, label))
	ctx, cancel := a.backupRemoteContext(timeout)
	defer cancel()
	if err := client.Download(ctx, fileName, localPath); err != nil {
		a.backupEmitImportProgress(`error`, 100, err.Error())
		return nil, err
	}
	result, err := a.backupRestorePackageFromPathLocked(localPath)
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

func backupRemoteUploadProgressMessage(localPath, artifactName, channelLabel string, uploadRateLimitMBps int) (string, int64, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return ``, 0, fmt.Errorf(`读取%s大小失败: %w`, artifactName, err)
	}
	if info.IsDir() {
		return ``, 0, fmt.Errorf(`%s路径是目录`, artifactName)
	}
	rateDescription := `不限速`
	if uploadRateLimitMBps > 0 {
		rateDescription = fmt.Sprintf(`%d MB/s`, uploadRateLimitMBps)
	}
	return fmt.Sprintf(`准备上传%s到%s：文件大小 %s（%d bytes），上传限速 %s`, artifactName, channelLabel, formatBackupFileSize(info.Size()), info.Size(), rateDescription), info.Size(), nil
}

func backupOpenListUploadProgressMessage(localPath, artifactName string, uploadRateLimitMBps int) (string, int64, error) {
	return backupRemoteUploadProgressMessage(localPath, artifactName, `OpenList`, uploadRateLimitMBps)
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

func formatBackupTransferRate(bytesPerSecond float64) string {
	if bytesPerSecond <= 0 {
		return `计算中`
	}
	value := bytesPerSecond
	units := []string{`B/s`, `KB/s`, `MB/s`, `GB/s`, `TB/s`}
	unitIndex := 0
	for value >= 1024 && unitIndex < len(units)-1 {
		value /= 1024
		unitIndex++
	}
	if unitIndex == 0 {
		return fmt.Sprintf(`%.0f %s`, value, units[unitIndex])
	}
	return fmt.Sprintf(`%.2f %s`, value, units[unitIndex])
}

func (a *App) backupRemoteUploadProgressCallback(channelLabel, artifactName string, startProgress, endProgress int) channels.UploadProgressFunc {
	return func(progress channels.UploadProgress) {
		a.backupEmitExportUploadProgress(channelLabel, artifactName, startProgress, endProgress, progress)
	}
}

func (a *App) backupOpenListUploadProgressCallback(artifactName string, startProgress, endProgress int) channels.UploadProgressFunc {
	return a.backupRemoteUploadProgressCallback(`OpenList`, artifactName, startProgress, endProgress)
}

func backupUploadWithProgress(ctx context.Context, client channels.Client, localPath, fileName string, progress channels.UploadProgressFunc) (channels.File, error) {
	if progressClient, ok := client.(channels.ProgressClient); ok {
		return progressClient.UploadWithProgress(ctx, localPath, fileName, progress)
	}
	return client.Upload(ctx, localPath, fileName)
}

func backupUploadMetadataWithProgress(ctx context.Context, client channels.Client, localPath, fileName string, progress channels.UploadProgressFunc) (channels.File, error) {
	if progressClient, ok := client.(channels.ProgressClient); ok {
		return progressClient.UploadMetadataWithProgress(ctx, localPath, fileName, progress)
	}
	return client.UploadMetadata(ctx, localPath, fileName)
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
	return a.backupRemoteContext(timeout)
}

func (a *App) backupRemoteContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	parent := context.Background()
	if a != nil && a.ctx != nil {
		parent = a.ctx
	}
	return context.WithTimeout(parent, timeout)
}
