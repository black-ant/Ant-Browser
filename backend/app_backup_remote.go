package backend

import (
	"ant-chrome/backend/internal/backup/channels"
	"ant-chrome/backend/internal/backup/channels/openlist"
	"ant-chrome/backend/internal/config"
	"context"
	"encoding/json"
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

type backupRemoteMetadataDownloader interface {
	DownloadMetadata(context.Context, string, string) error
}

const (
	backupRemoteHistoryMetadataTimeout     = 5 * time.Second
	backupRemoteHistoryMetadataConcurrency = 4
)

func (a *App) backupRemoteHistoryEntries(client backupRemoteMetadataDownloader, items []channels.File, timeout time.Duration) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		entry := map[string]interface{}{
			`name`:       item.Name,
			`size`:       item.Size,
			`modifiedAt`: item.ModifiedAt,
		}
		for key, value := range backupPackageInfoFields(backupPackageInfoFromFileName(item.Name)) {
			entry[key] = value
		}
		result = append(result, entry)
	}
	if client == nil || len(items) == 0 {
		return result
	}

	metadataRoot, err := os.MkdirTemp(``, `ant-chrome-backup-remote-history-`)
	if err != nil {
		return result
	}
	defer os.RemoveAll(metadataRoot)

	metadataTimeout := backupRemoteHistoryMetadataTimeout
	if timeout > 0 && timeout < metadataTimeout {
		metadataTimeout = timeout
	}
	metadataContext, metadataCancel := a.backupRemoteContext(metadataTimeout)
	defer metadataCancel()
	metadata := make([]backupMetadata, len(items))
	metadataAvailable := make([]bool, len(items))
	jobs := make(chan int)
	workerCount := backupRemoteHistoryMetadataConcurrency
	if workerCount > len(items) {
		workerCount = len(items)
	}
	done := make(chan struct{}, workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for index := range jobs {
				item := items[index]
				metadataName := filepath.Base(backupMetadataPath(item.Name))
				metadataPath := filepath.Join(metadataRoot, fmt.Sprintf(`%d-%s`, index, metadataName))
				downloadErr := client.DownloadMetadata(metadataContext, metadataName, metadataPath)
				if downloadErr != nil {
					continue
				}
				loaded, metadataErr := readBackupMetadataForFile(metadataPath, filepath.Base(item.Name))
				if metadataErr != nil {
					continue
				}
				metadata[index] = loaded
				metadataAvailable[index] = true
			}
		}()
	}
	for index := range items {
		jobs <- index
	}
	close(jobs)
	for worker := 0; worker < workerCount; worker++ {
		<-done
	}

	for index, itemMetadata := range metadata {
		if !metadataAvailable[index] {
			continue
		}
		if itemMetadata.PackageType != `` {
			result[index][`packageType`] = itemMetadata.PackageType
		}
		if itemMetadata.ProfileCount > 0 {
			result[index][`profileCount`] = itemMetadata.ProfileCount
		}
		if len(itemMetadata.ProfileNames) > 0 {
			result[index][`profileNames`] = itemMetadata.ProfileNames
		}
	}
	return result
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
	return a.backupRemoteHistoryEntries(client, items, openlist.ControlTimeout), nil
}

func (a *App) BackupOpenListUpload(input map[string]string) (map[string]interface{}, error) {
	if err := a.lockBackupMaintenance(); err != nil {
		a.backupEmitExportProgress(`error`, 100, fmt.Sprintf(`OpenList 备份失败: %v`, err))
		return nil, err
	}
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
	outcome, err := a.backupUploadRemoteArtifacts(backupRemoteUploadTarget{
		label:               `OpenList`,
		client:              client,
		timeout:             openlist.TransferTimeout,
		uploadRateLimitMBps: openListConfig.UploadRateLimitMBps,
	}, localPath, fileName)
	if err != nil {
		a.backupEmitExportProgress(`error`, 100, err.Error())
		return nil, err
	}
	result[`remoteName`] = outcome.File.Name
	result[`remoteSize`] = outcome.File.Size
	if outcome.Warning != `` {
		result[`remoteWarning`] = outcome.Warning
	}
	a.backupEmitExportProgress(`done`, 100, `backup uploaded to OpenList`)
	result[`message`] = `backup uploaded to OpenList`
	return result, nil
}

func (a *App) BackupOpenListRestore(input map[string]string, fileName string) (map[string]interface{}, error) {
	if err := a.lockBackupImportMaintenance(); err != nil {
		a.backupEmitImportProgress(`error`, 100, fmt.Sprintf(`OpenList 备份恢复失败: %v`, err))
		return nil, err
	}
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

	configuredDirectory := ``
	if a.config != nil {
		configuredDirectory = strings.TrimSpace(a.config.Backup.LocalDirectory)
	}
	if err := a.lockMaintenanceWithNotice(nil); err != nil {
		return nil, err
	}
	defer a.maintenanceMu.Unlock()
	var savePath string
	var err error
	if configuredDirectory != `` {
		savePath, _, err = a.backupResolveLocalPackagePath(defaultName, fmt.Sprintf(`download %s backup`, label))
		if err != nil {
			return nil, err
		}
	} else {
		savePath, err = wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
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
	}
	if strings.TrimSpace(savePath) == `` {
		return map[string]interface{}{
			`cancelled`: true,
			`message`:   `cancelled`,
		}, nil
	}
	savePath = backupEnsureZipSuffix(savePath)
	if configuredDirectory == `` {
		if err := a.backupSetLocalDirectoryLocked(filepath.Dir(savePath)); err != nil {
			return nil, err
		}
	}

	ctx, cancel := a.backupRemoteContext(timeout)
	defer cancel()
	if err := client.Download(ctx, trimmedName, savePath); err != nil {
		return nil, fmt.Errorf(`下载%s备份失败: %w`, label, err)
	}
	metadataPath := backupMetadataPath(savePath)
	remoteMetadataName := filepath.Base(backupMetadataPath(defaultName))
	metadataErr := a.backupDownloadRemoteMetadata(client, backupRemoteControlTimeout(timeout), remoteMetadataName, metadataPath, filepath.Base(savePath))
	return map[string]interface{}{
		`cancelled`:         false,
		`zipPath`:           savePath,
		`metadataPath`:      metadataPath,
		`metadataAvailable`: metadataErr == nil,
		`localDirectory`:    filepath.Dir(savePath),
		`remoteName`:        trimmedName,
		`message`:           fmt.Sprintf(`已下载%s备份`, label),
	}, nil
}

func (a *App) backupDownloadRemoteMetadata(client backupRemoteMetadataDownloader, timeout time.Duration, remoteMetadataName, metadataPath, backupFileName string) error {
	metadataDir := filepath.Dir(metadataPath)
	temporaryRoot, err := os.MkdirTemp(metadataDir, `.`+filepath.Base(metadataPath)+`.download-*`)
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporaryRoot)
	temporaryPath := filepath.Join(temporaryRoot, filepath.Base(metadataPath))

	metadataContext, metadataCancel := a.backupRemoteContext(timeout)
	metadataErr := client.DownloadMetadata(metadataContext, remoteMetadataName, temporaryPath)
	metadataCancel()
	if metadataErr != nil {
		return metadataErr
	}
	if err := normalizeDownloadedBackupMetadata(temporaryPath, backupFileName); err != nil {
		return err
	}
	if err := publishDownloadedBackupMetadata(temporaryPath, metadataPath); err != nil {
		return err
	}
	return nil
}

func publishDownloadedBackupMetadata(sourcePath, targetPath string) error {
	if err := os.Rename(sourcePath, targetPath); err == nil {
		return nil
	} else {
		renameErr := err
		backupPath, backupErr := moveExistingBackupMetadataAside(targetPath)
		if backupErr != nil {
			return fmt.Errorf(`replace downloaded backup metadata failed: %w`, renameErr)
		}
		if err := os.Rename(sourcePath, targetPath); err != nil {
			if backupPath != `` {
				if restoreErr := os.Rename(backupPath, targetPath); restoreErr != nil {
					return fmt.Errorf(`replace downloaded backup metadata failed: %w; restore existing metadata failed: %v`, err, restoreErr)
				}
			}
			return fmt.Errorf(`replace downloaded backup metadata failed: %w`, err)
		}
		if backupPath != `` {
			_ = os.Remove(backupPath)
		}
		return nil
	}
}

func moveExistingBackupMetadataAside(targetPath string) (string, error) {
	info, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		return ``, nil
	}
	if err != nil {
		return ``, err
	}
	if info.IsDir() {
		return ``, fmt.Errorf(`metadata target is a directory`)
	}
	temporaryFile, err := os.CreateTemp(filepath.Dir(targetPath), `.`+filepath.Base(targetPath)+`.backup-*`)
	if err != nil {
		return ``, err
	}
	backupPath := temporaryFile.Name()
	if err := temporaryFile.Close(); err != nil {
		_ = os.Remove(backupPath)
		return ``, err
	}
	if err := os.Remove(backupPath); err != nil {
		return ``, err
	}
	if err := os.Rename(targetPath, backupPath); err != nil {
		_ = os.Remove(backupPath)
		return ``, err
	}
	return backupPath, nil
}

func normalizeDownloadedBackupMetadata(metadataPath, backupFileName string) error {
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return err
	}
	var metadata backupMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return err
	}
	if strings.TrimSpace(metadata.Format) != `ant-chrome-backup-metadata` {
		return fmt.Errorf(`unsupported backup metadata format`)
	}
	expectedBackupFile := filepath.Base(backupFileName)
	if declaredBackupFile := strings.TrimSpace(metadata.BackupFile); declaredBackupFile != `` && !strings.EqualFold(filepath.Base(filepath.FromSlash(declaredBackupFile)), expectedBackupFile) {
		return fmt.Errorf(`备份元数据与 ZIP 文件不匹配`)
	}
	metadata.BackupFile = expectedBackupFile
	updated, err := json.MarshalIndent(metadata, ``, `  `)
	if err != nil {
		return err
	}
	return writeBackupMetadataFile(metadataPath, append(updated, '\n'))
}

func (a *App) backupUploadRemoteArtifacts(target backupRemoteUploadTarget, localPath, fileName string) (channels.UploadOutcome, error) {
	uploadMessage, uploadSize, err := backupRemoteUploadProgressMessage(localPath, `备份文件`, target.label, target.uploadRateLimitMBps)
	if err != nil {
		return channels.UploadOutcome{}, err
	}
	a.backupEmitExportProgressTransfer(`uploading`, 96, uploadMessage, channels.UploadProgress{TotalBytes: uploadSize})
	ctx, cancel := a.backupRemoteContext(target.timeout)
	outcome, err := backupUploadWithProgress(ctx, target.client, localPath, fileName, a.backupRemoteUploadProgressCallback(target.label, `备份文件`, 96, 98))
	cancel()
	if err != nil {
		return channels.UploadOutcome{}, fmt.Errorf(`上传%s备份文件失败: %w`, target.label, err)
	}
	if strings.TrimSpace(outcome.Warning) != `` {
		a.backupEmitExportProgress(`warning`, 99, fmt.Sprintf(`%s备份文件已写入，但远程目标同步存在警告：%s`, target.label, strings.TrimSpace(outcome.Warning)))
	}
	if target.skipMetadata {
		return outcome, nil
	}

	metadataPath := backupMetadataPath(localPath)
	metadataName := filepath.Base(backupMetadataPath(fileName))
	if _, err := os.Stat(metadataPath); err != nil {
		if os.IsNotExist(err) {
			a.backupEmitExportProgress(`warning`, 99, fmt.Sprintf(`%s backup metadata is missing; skipped metadata upload`, target.label))
			return outcome, nil
		}
		return channels.UploadOutcome{}, fmt.Errorf(`read %s backup metadata failed: %w`, target.label, err)
	}
	metadataUploadPath, cleanupMetadata, metadataPrepareErr := backupPrepareRemoteMetadata(metadataPath, filepath.Base(localPath), fileName)
	if metadataPrepareErr != nil {
		a.backupEmitExportProgress(`warning`, 99, fmt.Sprintf(`%s backup metadata is unavailable; skipped metadata upload: %v`, target.label, metadataPrepareErr))
		return outcome, nil
	}
	defer cleanupMetadata()
	metadataMessage, metadataSize, err := backupRemoteUploadProgressMessage(metadataUploadPath, `备份元数据`, target.label, target.uploadRateLimitMBps)
	if err != nil {
		a.backupEmitExportProgress(`warning`, 99, fmt.Sprintf(`%s backup metadata is unavailable; skipped metadata upload: %v`, target.label, err))
		return outcome, nil
	}
	a.backupEmitExportProgressTransfer(`uploading`, 98, metadataMessage, channels.UploadProgress{TotalBytes: metadataSize})
	metadataContext, metadataCancel := a.backupRemoteContext(backupRemoteControlTimeout(target.timeout))
	_, metadataErr := backupUploadMetadataWithProgress(metadataContext, target.client, metadataUploadPath, metadataName, a.backupRemoteUploadProgressCallback(target.label, `备份元数据`, 98, 99))
	metadataCancel()
	if metadataErr != nil {
		a.backupEmitExportProgress(`warning`, 99, fmt.Sprintf(`%s backup metadata upload failed; ZIP kept: %v`, target.label, metadataErr))
		return outcome, nil
	}
	return outcome, nil
}

func normalizeRemoteBackupRestoreName(fileName string) (string, error) {
	normalized := strings.ReplaceAll(fileName, `\`, `/`)
	if strings.IndexFunc(normalized, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return ``, fmt.Errorf(`remote backup file name contains a control character`)
	}
	normalized = strings.TrimSpace(normalized)
	if normalized == `` || normalized == `.` || normalized == `..` || strings.Contains(normalized, `/`) {
		return ``, fmt.Errorf(`invalid remote backup file name`)
	}
	if !strings.EqualFold(filepath.Ext(normalized), `.zip`) {
		return ``, fmt.Errorf(`remote backup file must use .zip suffix`)
	}
	return normalized, nil
}

func (a *App) backupRestoreRemoteLocked(client channels.Client, label string, timeout time.Duration, fileName, temporaryPrefix string) (map[string]interface{}, error) {
	normalizedName, err := normalizeRemoteBackupRestoreName(fileName)
	if err != nil {
		return nil, err
	}
	temporaryRoot, err := os.MkdirTemp(``, temporaryPrefix)
	if err != nil {
		return nil, fmt.Errorf(`create temporary restore directory failed: %w`, err)
	}
	defer os.RemoveAll(temporaryRoot)
	localPath := filepath.Join(temporaryRoot, `remote-backup.zip`)
	metadataPath := filepath.Join(temporaryRoot, `remote-backup.json`)
	metadataName := filepath.Base(backupMetadataPath(normalizedName))
	metadataContext, metadataCancel := a.backupRemoteContext(backupRemoteControlTimeout(timeout))
	metadataErr := client.DownloadMetadata(metadataContext, metadataName, metadataPath)
	metadataCancel()
	if metadataErr == nil {
		if err := normalizeDownloadedBackupMetadata(metadataPath, normalizedName); err != nil {
			return nil, fmt.Errorf(`远程备份元数据校验失败: %w`, err)
		}
	} else {
		_ = os.Remove(metadataPath)
	}
	a.backupEmitImportProgress(`preparing`, 5, fmt.Sprintf(`正在从%s下载备份`, label))
	ctx, cancel := a.backupRemoteContext(timeout)
	defer cancel()
	if err := client.Download(ctx, normalizedName, localPath); err != nil {
		a.backupEmitImportProgress(`error`, 100, err.Error())
		return nil, err
	}
	if _, err := inspectBackupPackageInfo(localPath); err != nil {
		err = fmt.Errorf(`远程备份包校验失败: %w`, err)
		a.backupEmitImportProgress(`error`, 100, err.Error())
		return nil, err
	}
	result, err := a.backupRestorePackageFromPathLocked(localPath)
	if err != nil {
		a.backupEmitImportProgress(`error`, 100, fmt.Sprintf(`restore remote backup failed: %v`, err))
		return nil, err
	}
	result[`remoteName`] = normalizedName
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
		AuthMode:            settings.AuthMode,
		Username:            settings.Username,
		Password:            settings.Password,
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
	settings.BaseURL = baseURL
	settings.RemotePath = remotePath
	if err := applyOpenListAuthInput(&settings, input); err != nil {
		return config.OpenListChannelConfig{}, err
	}
	if value := backupOpenListInputValue(input, `uploadRateLimitMBps`, `uploadRateLimitMbps`, `upload_rate_limit_mbps`); value != `` {
		rateLimit, err := strconv.Atoi(value)
		if err != nil || rateLimit < 0 {
			return config.OpenListChannelConfig{}, fmt.Errorf(`OpenList 上传限速必须是非负整数 MB/s`)
		}
		settings.UploadRateLimitMBps = rateLimit
	}
	return settings, nil
}

func applyOpenListAuthInput(settings *config.OpenListChannelConfig, input map[string]string) error {
	if settings == nil {
		return nil
	}
	authMode, authModeProvided := backupOpenListInputValueWithPresence(input, `authMode`, `auth_mode`)
	username, usernameProvided := backupOpenListInputValueWithPresence(input, `username`, `user`)
	password, passwordProvided := backupOpenListInputValueWithPresence(input, `password`, `pass`)
	token, tokenProvided := backupOpenListInputValueWithPresence(input, `token`)

	if authModeProvided {
		switch strings.ToLower(strings.TrimSpace(authMode)) {
		case `basic`:
			settings.AuthMode = `basic`
			if usernameProvided {
				settings.Username = username
			}
			if passwordProvided && password != `` {
				settings.Password = password
			}
			settings.Token = ``
		case `token`:
			settings.AuthMode = `token`
			if tokenProvided && token != `` {
				settings.Token = token
			}
			settings.Username = ``
			settings.Password = ``
		default:
			return fmt.Errorf(`OpenList 认证方式无效`)
		}
		return nil
	}

	if usernameProvided && username != `` {
		settings.AuthMode = `basic`
		settings.Username = username
		if passwordProvided && password != `` {
			settings.Password = password
		}
		settings.Token = ``
		return nil
	}
	if tokenProvided && token != `` {
		settings.AuthMode = `token`
		settings.Token = token
		settings.Username = ``
		settings.Password = ``
	}
	return nil
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
	unitIndex := -1
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

func backupUploadWithProgress(ctx context.Context, client channels.Client, localPath, fileName string, progress channels.UploadProgressFunc) (channels.UploadOutcome, error) {
	if outcomeClient, ok := client.(channels.UploadOutcomeClient); ok {
		return outcomeClient.UploadWithProgressOutcome(ctx, localPath, fileName, progress)
	}
	if progressClient, ok := client.(channels.ProgressClient); ok {
		file, err := progressClient.UploadWithProgress(ctx, localPath, fileName, progress)
		return channels.UploadOutcome{File: file}, err
	}
	file, err := client.Upload(ctx, localPath, fileName)
	return channels.UploadOutcome{File: file}, err
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

func backupRemoteControlTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 || timeout > time.Minute {
		return time.Minute
	}
	return timeout
}
