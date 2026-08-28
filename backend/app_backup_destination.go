package backend

import (
	"ant-chrome/backend/internal/backup/channels"
	"ant-chrome/backend/internal/backup/channels/openlist"
	"ant-chrome/backend/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// BackupCreatePackage 创建一份备份包，并按目的地选择保存到本地和/或上传到 OpenList。
func (a *App) BackupCreatePackage(input map[string]string) (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	localEnabled := backupDestinationFlag(input, "local", "localEnabled")
	openListEnabled := backupDestinationFlag(input, "openList", "openlist", "openListEnabled")
	if !localEnabled && !openListEnabled {
		err := fmt.Errorf("至少选择一个备份位置")
		a.backupEmitExportProgress("error", 100, fmt.Sprintf("备份失败: %v", err))
		return nil, err
	}
	if localEnabled && a.ctx == nil {
		err := fmt.Errorf("应用上下文未初始化")
		a.backupEmitExportProgress("error", 100, fmt.Sprintf("备份失败: %v", err))
		return nil, err
	}

	var openListClient channels.Client
	var openListConfig config.OpenListChannelConfig
	var err error
	if openListEnabled {
		openListConfig, openListClient, err = a.backupOpenListClientWithConfig(input)
		if err != nil {
			a.backupEmitExportProgress("error", 100, fmt.Sprintf("备份失败: %v", err))
			return nil, err
		}
	}

	var packagePath string
	var temporaryRoot string
	if localEnabled {
		a.backupEmitExportProgress("starting", 0, "等待选择本地备份路径...")
		defaultName := fmt.Sprintf("ant-chrome-backup-%s.zip", time.Now().Format("20060102-150405"))
		packagePath, err = wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
			Title:           "保存本地备份",
			DefaultFilename: defaultName,
			Filters: []wailsruntime.FileFilter{
				{DisplayName: "ZIP 文件 (*.zip)", Pattern: "*.zip"},
			},
		})
		if err != nil {
			a.backupEmitExportProgress("error", 100, fmt.Sprintf("打开保存对话框失败: %v", err))
			return nil, fmt.Errorf("打开保存对话框失败: %w", err)
		}
		if strings.TrimSpace(packagePath) == "" {
			a.backupEmitExportProgress("cancelled", 0, "已取消备份")
			return map[string]interface{}{
				"cancelled": true,
				"message":   "已取消备份",
			}, nil
		}
		packagePath = backupEnsureZipSuffix(packagePath)
	} else {
		temporaryRoot, err = os.MkdirTemp("", "ant-chrome-backup-")
		if err != nil {
			a.backupEmitExportProgress("error", 100, fmt.Sprintf("创建临时备份目录失败: %v", err))
			return nil, fmt.Errorf("创建临时备份目录失败: %w", err)
		}
		defer os.RemoveAll(temporaryRoot)
		packagePath = filepath.Join(temporaryRoot, fmt.Sprintf("ant-chrome-backup-%s.zip", time.Now().Format("20060102-150405.000000000")))
	}

	result, err := a.backupExportPackageToPath(packagePath)
	if err != nil {
		return nil, err
	}
	result["localSaved"] = localEnabled
	result["remoteUploaded"] = false

	if openListEnabled {
		fileName := filepath.Base(packagePath)
		uploadMessage, uploadErr := backupOpenListUploadProgressMessage(packagePath, "备份文件", openListConfig.UploadRateLimitMBps)
		var remoteFile channels.File
		if uploadErr == nil {
			a.backupEmitExportProgress("uploading", 96, uploadMessage)
			ctx, cancel := a.backupOpenListContext(openlist.TransferTimeout)
			remoteFile, uploadErr = openListClient.Upload(ctx, packagePath, fileName)
			cancel()
		}
		if uploadErr != nil {
			if localEnabled {
				result["partial"] = true
				result["remoteError"] = uploadErr.Error()
				result["message"] = fmt.Sprintf("本地备份已保存，但上传 OpenList 失败: %v", uploadErr)
				a.backupEmitExportProgress("error", 100, result["message"].(string))
				return result, nil
			}
			a.backupEmitExportProgress("error", 100, fmt.Sprintf("上传 OpenList 失败: %v", uploadErr))
			return nil, uploadErr
		}
		result["remoteUploaded"] = true
		result["remoteName"] = remoteFile.Name
		result["remoteSize"] = remoteFile.Size

		metadataPath := backupMetadataPath(packagePath)
		metadataName := filepath.Base(metadataPath)
		metadataMessage, metadataErr := backupOpenListUploadProgressMessage(metadataPath, "备份元数据", openListConfig.UploadRateLimitMBps)
		if metadataErr == nil {
			a.backupEmitExportProgress("uploading", 98, metadataMessage)
			metadataContext, metadataCancel := a.backupOpenListContext(openlist.TransferTimeout)
			_, metadataErr = openListClient.UploadMetadata(metadataContext, metadataPath, metadataName)
			metadataCancel()
		}
		if metadataErr != nil {
			result["partial"] = true
			result["remoteError"] = metadataErr.Error()
			result["message"] = fmt.Sprintf("备份已上传，但同名 JSON 上传失败: %v", metadataErr)
			a.backupEmitExportProgress("error", 100, result["message"].(string))
			return result, nil
		}
	}

	if !localEnabled {
		delete(result, "zipPath")
	}
	switch {
	case localEnabled && openListEnabled:
		result["message"] = "本地和 OpenList 备份完成"
	case localEnabled:
		result["message"] = "本地备份完成"
	default:
		result["message"] = "OpenList 备份完成"
	}
	a.backupEmitExportProgress("done", 100, result["message"].(string))
	return result, nil
}

func backupDestinationFlag(input map[string]string, keys ...string) bool {
	for _, key := range keys {
		switch strings.ToLower(strings.TrimSpace(input[key])) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}
