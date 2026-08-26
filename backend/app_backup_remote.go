package backend

import (
	"ant-chrome/backend/internal/backupremote"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *App) BackupOpenListTest(input map[string]string) (map[string]interface{}, error) {
	client, err := a.backupOpenListClient(input)
	if err != nil {
		return nil, err
	}
	ctx, cancel := a.backupOpenListContext(20 * time.Second)
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
	ctx, cancel := a.backupOpenListContext(20 * time.Second)
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
	client, err := a.backupOpenListClient(input)
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
	a.backupEmitExportProgress(`uploading`, 96, `uploading backup to OpenList`)
	ctx, cancel := a.backupOpenListContext(30 * time.Minute)
	defer cancel()
	remoteFile, err := client.Upload(ctx, localPath, fileName)
	if err != nil {
		a.backupEmitExportProgress(`error`, 100, err.Error())
		return nil, err
	}
	a.backupEmitExportProgress(`done`, 100, `backup uploaded to OpenList`)
	result[`remoteName`] = remoteFile.Name
	result[`remoteSize`] = remoteFile.Size
	result[`message`] = `backup uploaded to OpenList`
	return result, nil
}

func (a *App) BackupOpenListRestore(input map[string]string, fileName string, resetFirst bool) (map[string]interface{}, error) {
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
	ctx, cancel := a.backupOpenListContext(30 * time.Minute)
	defer cancel()
	if err := client.Download(ctx, fileName, localPath); err != nil {
		a.backupEmitImportProgress(`error`, 100, err.Error())
		return nil, err
	}
	result, err := a.backupImportFromPathLocked(localPath, resetFirst)
	if err != nil {
		a.backupEmitImportProgress(`error`, 100, fmt.Sprintf(`restore remote backup failed: %v`, err))
		return nil, err
	}
	result[`remoteName`] = fileName
	return result, nil
}

func (a *App) backupOpenListClient(input map[string]string) (*backupremote.Client, error) {
	baseURL := backupOpenListInputValue(input, `baseURL`, `baseUrl`)
	remotePath := backupOpenListInputValue(input, `remotePath`, `path`)
	username := backupOpenListInputValue(input, `username`, `user`)
	password := backupOpenListInputValue(input, `password`, `pass`)
	return backupremote.NewClient(backupremote.Config{
		BaseURL:    baseURL,
		RemotePath: remotePath,
		Username:   username,
		Password:   password,
	})
}

func backupOpenListInputValue(input map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(input[key]); value != `` {
			return value
		}
	}
	return ``
}

func (a *App) backupOpenListContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	parent := context.Background()
	if a != nil && a.ctx != nil {
		parent = a.ctx
	}
	return context.WithTimeout(parent, timeout)
}
