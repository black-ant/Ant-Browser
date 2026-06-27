package backend

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// 窗口数据快照 API
// ============================================================================

// SnapshotInfo 快照元数据
type SnapshotInfo struct {
	SnapshotId string  `json:"snapshotId"`
	ProfileId  string  `json:"profileId"`
	Name       string  `json:"name"`
	SizeMB     float64 `json:"sizeMB"`
	CreatedAt  string  `json:"createdAt"`
	FilePath   string  `json:"filePath,omitempty"`
}

// snapshotDir 返回指定窗口的快照目录路径（存放在 data/snapshots 下）
func (a *App) snapshotDir(profileId string) (string, error) {
	dir := filepath.Join(a.resolveAppPath("data"), "snapshots", profileId)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// zipDir 递归压缩 src 目录为 dest zip 文件
func zipDir(src, dest string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		// 统一使用正斜杠
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == "." {
				return nil
			}
			_, err = w.Create(rel + "/")
			return err
		}
		fw, err := w.Create(rel)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(fw, file)
		return err
	})
}

// unzipTo 解压 src zip 文件到 dest 目录
func unzipTo(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target := filepath.Join(dest, filepath.FromSlash(f.Name))
		// 防止 zip slip
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) &&
			filepath.Clean(target) != filepath.Clean(dest) {
			return fmt.Errorf("非法路径: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		rc.Close()
		out.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

// getProfileForSnapshot 获取窗口信息（加锁）
func (a *App) getProfileForSnapshot(profileId string) (*BrowserProfile, error) {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		return nil, fmt.Errorf("窗口不存在: %s", profileId)
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
		return SnapshotInfo{}, fmt.Errorf("请先停止窗口再创建快照")
	}

	userDataDir := a.browserMgr.ResolveUserDataDir(profile)
	if _, err := os.Stat(userDataDir); os.IsNotExist(err) {
		return SnapshotInfo{}, fmt.Errorf("用户数据目录不存在，无法创建快照")
	}

	snapDir, err := a.snapshotDir(profileId)
	if err != nil {
		return SnapshotInfo{}, err
	}

	snapshotId := uuid.NewString()
	safeName := strings.ReplaceAll(name, string(os.PathSeparator), "_")
	zipPath := filepath.Join(snapDir, snapshotId+"_"+safeName+".zip")
	metaPath := filepath.Join(snapDir, snapshotId+"_"+safeName+".meta.json")

	if err := zipDir(userDataDir, zipPath); err != nil {
		return SnapshotInfo{}, fmt.Errorf("压缩失败: %w", err)
	}

	fi, err := os.Stat(zipPath)
	if err != nil {
		return SnapshotInfo{}, err
	}
	sizeMB := float64(fi.Size()) / 1024 / 1024

	info := SnapshotInfo{
		SnapshotId: snapshotId,
		ProfileId:  profileId,
		Name:       name,
		SizeMB:     sizeMB,
		CreatedAt:  time.Now().Format(time.RFC3339),
		FilePath:   zipPath,
	}

	metaData, _ := json.Marshal(info)
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return SnapshotInfo{}, err
	}

	// 返回给前端时不暴露 FilePath
	info.FilePath = ""
	return info, nil
}

// BrowserSnapshotList 列出窗口的所有快照
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
		return fmt.Errorf("请先停止窗口再恢复快照")
	}

	snapDir, err := a.snapshotDir(profileId)
	if err != nil {
		return err
	}

	// 找到对应 meta.json
	metaPath, zipPath, err := findSnapshotFiles(snapDir, snapshotId)
	if err != nil {
		return err
	}
	_ = metaPath

	userDataDir := a.browserMgr.ResolveUserDataDir(profile)
	if err := os.RemoveAll(userDataDir); err != nil {
		return fmt.Errorf("清空用户数据目录失败: %w", err)
	}
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		return err
	}
	return unzipTo(zipPath, userDataDir)
}

// BrowserSnapshotDelete 删除快照
func (a *App) BrowserSnapshotDelete(profileId, snapshotId string) error {
	snapDir, err := a.snapshotDir(profileId)
	if err != nil {
		return err
	}
	metaPath, zipPath, err := findSnapshotFiles(snapDir, snapshotId)
	if err != nil {
		return err
	}
	_ = os.Remove(zipPath)
	_ = os.Remove(metaPath)
	return nil
}

// findSnapshotFiles 在快照目录中找到指定 snapshotId 的 meta 和 zip 路径
func findSnapshotFiles(snapDir, snapshotId string) (metaPath, zipPath string, err error) {
	entries, err := os.ReadDir(snapDir)
	if err != nil {
		return "", "", err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), snapshotId) && strings.HasSuffix(entry.Name(), ".meta.json") {
			metaPath = filepath.Join(snapDir, entry.Name())
			zipPath = strings.TrimSuffix(metaPath, ".meta.json") + ".zip"
			if _, err := os.Stat(zipPath); err != nil {
				return "", "", fmt.Errorf("快照文件不存在: %s", zipPath)
			}
			return metaPath, zipPath, nil
		}
	}
	return "", "", fmt.Errorf("快照不存在: %s", snapshotId)
}
