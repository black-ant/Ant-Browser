package backend

import (
	"ant-chrome/backend/internal/snapshot"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// getProfileForSnapshot 获取实例信息（加锁）
func (a *App) getProfileForSnapshot(profileId string) (*BrowserProfile, error) {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		return nil, fmt.Errorf("实例不存在: %s", profileId)
	}
	return profile, nil
}

// BrowserSnapshotCreate 创建快照
func (a *App) BrowserSnapshotCreate(profileId, name string) (SnapshotInfo, error) {
	profile, err := a.getProfileForSnapshot(profileId)
	if err != nil {
		return SnapshotInfo{}, err
	}
	if profile.Running {
		return SnapshotInfo{}, fmt.Errorf("请先停止实例再创建快照")
	}

	userDataDir := a.browserMgr.ResolveUserDataDir(profile)
	if _, err := os.Stat(userDataDir); os.IsNotExist(err) {
		return SnapshotInfo{}, fmt.Errorf("用户数据目录不存在，无法创建快照")
	}

	snapDir, err := a.snapshotDir(profileId)
	if err != nil {
		return SnapshotInfo{}, err
	}

	snapshotID := uuid.NewString()
	safeName := strings.ReplaceAll(name, string(os.PathSeparator), "_")
	zipPath := filepath.Join(snapDir, snapshotID+"_"+safeName+".zip")
	metaPath := filepath.Join(snapDir, snapshotID+"_"+safeName+".meta.json")

	if err := snapshot.ZipDir(userDataDir, zipPath); err != nil {
		return SnapshotInfo{}, fmt.Errorf("压缩失败: %w", err)
	}

	fi, err := os.Stat(zipPath)
	if err != nil {
		return SnapshotInfo{}, err
	}
	sizeMB := float64(fi.Size()) / 1024 / 1024

	info := SnapshotInfo{
		SnapshotId: snapshotID,
		ProfileId:  profileId,
		Name:       name,
		SizeMB:     sizeMB,
		CreatedAt:  time.Now().Format(time.RFC3339),
		FilePath:   zipPath,
	}

	metaData, _ := json.Marshal(info)
	if err := os.WriteFile(metaPath, metaData, 0o644); err != nil {
		return SnapshotInfo{}, err
	}

	info.FilePath = ""
	return info, nil
}

// BrowserSnapshotList 列出实例的所有快照
func (a *App) BrowserSnapshotList(profileId string) ([]SnapshotInfo, error) {
	snapDir, err := a.snapshotDir(profileId)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(snapDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SnapshotInfo{}, nil
		}
		return nil, err
	}

	var list []SnapshotInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(snapDir, entry.Name()))
		if err != nil {
			continue
		}
		var info SnapshotInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}
		info.FilePath = ""
		list = append(list, info)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt > list[j].CreatedAt
	})
	return list, nil
}

// BrowserSnapshotRestore 恢复快照
func (a *App) BrowserSnapshotRestore(profileId, snapshotId string) error {
	profile, err := a.getProfileForSnapshot(profileId)
	if err != nil {
		return err
	}
	if profile.Running {
		return fmt.Errorf("请先停止实例再恢复快照")
	}

	snapDir, err := a.snapshotDir(profileId)
	if err != nil {
		return err
	}

	metaPath, zipPath, err := snapshot.FindFiles(snapDir, snapshotId)
	if err != nil {
		return err
	}
	_ = metaPath

	userDataDir := a.browserMgr.ResolveUserDataDir(profile)
	if err := os.RemoveAll(userDataDir); err != nil {
		return fmt.Errorf("清空用户数据目录失败: %w", err)
	}
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		return err
	}
	return snapshot.UnzipTo(zipPath, userDataDir)
}

// BrowserSnapshotDelete 删除快照
func (a *App) BrowserSnapshotDelete(profileId, snapshotId string) error {
	snapDir, err := a.snapshotDir(profileId)
	if err != nil {
		return err
	}
	metaPath, zipPath, err := snapshot.FindFiles(snapDir, snapshotId)
	if err != nil {
		return err
	}
	_ = os.Remove(zipPath)
	_ = os.Remove(metaPath)
	return nil
}
