package backend

import (
	"ant-chrome/backend/internal/backup/channels"
	"ant-chrome/backend/internal/backup/channels/openlist"
	"ant-chrome/backend/internal/backup/channels/s3"
	"ant-chrome/backend/internal/config"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// BackupCreatePackage 创建一份备份包，并按目的地选择保存到本地和/或上传到远程渠道。
func (a *App) BackupCreatePackage(input map[string]string) (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	localEnabled := backupDestinationFlag(input, "local", "localEnabled")
	openListEnabled := backupDestinationFlag(input, "openList", "openlist", "openListEnabled")
	s3Enabled := backupDestinationFlag(input, "s3", "s3Enabled")
	if !localEnabled && !openListEnabled && !s3Enabled {
		err := fmt.Errorf("至少选择一个备份位置")
		a.backupEmitExportProgress("error", 100, fmt.Sprintf("备份失败: %v", err))
		return nil, err
	}
	profileIDs, err := backupProfileIDsFromInput(input)
	if err != nil {
		a.backupEmitExportProgress("error", 100, fmt.Sprintf("备份失败: %v", err))
		return nil, err
	}
	profileNames := []string(nil)
	if len(profileIDs) > 0 {
		profiles, collectErr := a.collectProfilesForPackage(profileIDs)
		if collectErr != nil {
			return nil, collectErr
		}
		profileNames = profilePackageProfileNames(profiles)
	}
	if localEnabled && a.ctx == nil {
		err := fmt.Errorf("应用上下文未初始化")
		a.backupEmitExportProgress("error", 100, fmt.Sprintf("备份失败: %v", err))
		return nil, err
	}

	var openListClient channels.Client
	var openListConfig config.OpenListChannelConfig
	var s3Client channels.Client
	if openListEnabled {
		openListConfig, openListClient, err = a.backupOpenListClientWithConfig(input)
		if err != nil {
			a.backupEmitExportProgress("error", 100, fmt.Sprintf("备份失败: %v", err))
			return nil, err
		}
	}
	if s3Enabled {
		s3Client, err = a.backupS3Client(input)
		if err != nil {
			a.backupEmitExportProgress("error", 100, fmt.Sprintf("备份失败: %v", err))
			return nil, err
		}
	}

	var packagePath string
	var temporaryRoot string
	if localEnabled {
		a.backupEmitExportProgress("starting", 0, "等待选择本地备份路径...")
		defaultName := backupPackageDefaultName(len(profileIDs) > 0, profileNames, time.Now())
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
		packagePath = filepath.Join(temporaryRoot, backupTemporaryPackageName(len(profileIDs) > 0, profileNames, time.Now()))
	}

	var result map[string]interface{}
	if len(profileIDs) > 0 {
		result, err = a.backupExportProfilePackageToPath(packagePath, profileIDs)
	} else {
		result, err = a.backupExportPackageToPath(packagePath)
	}
	if err != nil {
		return nil, err
	}
	result["localSaved"] = localEnabled
	result["remoteUploaded"] = false

	remoteTargets := make([]backupRemoteUploadTarget, 0, 2)
	if openListEnabled {
		remoteTargets = append(remoteTargets, backupRemoteUploadTarget{
			label:               "OpenList",
			client:              openListClient,
			timeout:             openlist.TransferTimeout,
			uploadRateLimitMBps: openListConfig.UploadRateLimitMBps,
			skipMetadata:        len(profileIDs) > 0,
		})
	}
	if s3Enabled {
		remoteTargets = append(remoteTargets, backupRemoteUploadTarget{
			label:        "S3",
			client:       s3Client,
			timeout:      s3.TransferTimeout,
			skipMetadata: len(profileIDs) > 0,
		})
	}
	remoteErrors := make([]string, 0, len(remoteTargets))
	remoteNames := make([]string, 0, len(remoteTargets))
	remoteFileName := filepath.Base(packagePath)
	if len(profileIDs) > 0 {
		remoteFileName = backupProfilePackageFileName(profileNames, time.Now(), true)
	}
	for _, target := range remoteTargets {
		remoteFile, uploadErr := a.backupUploadRemoteArtifacts(target, packagePath, remoteFileName)
		if uploadErr != nil {
			remoteErrors = append(remoteErrors, fmt.Sprintf("%s: %v", target.label, uploadErr))
			continue
		}
		result["remoteUploaded"] = true
		remoteNames = append(remoteNames, fmt.Sprintf("%s:%s", target.label, remoteFile.Name))
		if _, exists := result["remoteName"]; !exists {
			result["remoteName"] = remoteFile.Name
			result["remoteSize"] = remoteFile.Size
		}
	}
	if len(remoteErrors) > 0 {
		result["partial"] = true
		result["remoteError"] = strings.Join(remoteErrors, "; ")
		if len(remoteNames) > 0 {
			result["remoteNames"] = remoteNames
		}
		if len(remoteNames) == 0 && !localEnabled {
			a.backupEmitExportProgress("error", 100, result["remoteError"].(string))
			return nil, fmt.Errorf("远程备份失败: %s", result["remoteError"].(string))
		}
		if !localEnabled {
			delete(result, "zipPath")
		}
		result["message"] = fmt.Sprintf("备份已完成，但部分远程渠道失败: %s", result["remoteError"].(string))
		a.backupEmitExportProgress("error", 100, result["message"].(string))
		return result, nil
	}
	if !localEnabled {
		delete(result, "zipPath")
	}
	switch {
	case localEnabled && openListEnabled && s3Enabled:
		result["message"] = "\u672c\u5730\u3001OpenList \u548c S3 \u5907\u4efd\u5b8c\u6210"
	case openListEnabled && s3Enabled:
		result["message"] = "OpenList \u548c S3 \u5907\u4efd\u5b8c\u6210"
	case localEnabled && s3Enabled:
		result["message"] = "\u672c\u5730\u548c S3 \u5907\u4efd\u5b8c\u6210"
	case s3Enabled:
		result["message"] = "S3 \u5907\u4efd\u5b8c\u6210"
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

func backupProfileIDsFromInput(input map[string]string) ([]string, error) {
	raw := strings.TrimSpace(input["profileIds"])
	if raw == "" {
		return nil, nil
	}

	var profileIDs []string
	if err := json.Unmarshal([]byte(raw), &profileIDs); err != nil {
		return nil, fmt.Errorf("实例 ID 参数无效: %w", err)
	}
	profileIDs = normalizeProfilePackageIDs(profileIDs)
	if len(profileIDs) == 0 {
		return nil, fmt.Errorf("请选择要备份的实例")
	}
	return profileIDs, nil
}

func backupPackageDefaultName(profileOnly bool, profileNames []string, now time.Time) string {
	if profileOnly {
		return backupProfilePackageFileName(profileNames, now, false)
	}
	prefix := "ant-chrome-backup"
	return fmt.Sprintf("%s-%s.zip", prefix, now.Format("20060102-150405"))
}

func backupTemporaryPackageName(profileOnly bool, profileNames []string, now time.Time) string {
	if profileOnly {
		return backupProfilePackageFileName(profileNames, now, true)
	}
	prefix := "ant-chrome-backup"
	return fmt.Sprintf("%s-%s.zip", prefix, now.Format("20060102-150405.000000000"))
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
