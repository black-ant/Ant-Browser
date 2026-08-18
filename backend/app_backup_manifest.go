package backend

import (
	"ant-chrome/backend/internal/backup"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type backupArchiveStats struct {
	fileCount int
	byteSize  int64
	digest    hash.Hash
}

func newBackupArchiveStats() backupArchiveStats {
	return backupArchiveStats{digest: sha256.New()}
}

func (s *backupArchiveStats) addFile(archivePath, sourcePath string, destination io.Writer) error {
	in, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer in.Close()

	return s.addReader(archivePath, in, destination)
}

func (s *backupArchiveStats) addReader(archivePath string, source io.Reader, destination io.Writer) error {
	if s.digest == nil {
		s.digest = sha256.New()
	}
	_, _ = s.digest.Write([]byte(archivePath))
	_, _ = s.digest.Write([]byte{0})

	target := io.Writer(s.digest)
	if destination != nil {
		target = io.MultiWriter(destination, s.digest)
	}
	written, err := io.Copy(target, source)
	if err != nil {
		return err
	}
	s.fileCount++
	s.byteSize += written
	return nil
}

func (s backupArchiveStats) sha256() string {
	if s.digest == nil {
		return ""
	}
	return hex.EncodeToString(s.digest.Sum(nil))
}

func backupArchiveStatsFromPath(path, archivePath string, entryType backup.EntryType) (backupArchiveStats, error) {
	stats := newBackupArchiveStats()
	if entryType == backup.EntryTypeFile {
		if err := stats.addFile(strings.TrimSuffix(archivePath, "/"), path, nil); err != nil {
			return backupArchiveStats{}, err
		}
		return stats, nil
	}

	base := strings.TrimSuffix(filepath.ToSlash(archivePath), "/")
	err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == path || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(path, current)
		if err != nil {
			return err
		}
		memberPath := base + "/" + filepath.ToSlash(rel)
		return stats.addFile(memberPath, current, nil)
	})
	if err != nil {
		return backupArchiveStats{}, err
	}
	return stats, nil
}

func backupValidateManifest(extractRoot string, manifest backup.Manifest) error {
	payloadRoot := filepath.Join(extractRoot, "payload")
	payloadInfo, err := os.Stat(payloadRoot)
	if err != nil || !payloadInfo.IsDir() {
		return fmt.Errorf("备份包缺少 payload 目录")
	}
	if len(manifest.Entries) == 0 {
		return fmt.Errorf("manifest.json 缺少备份条目")
	}

	seenIDs := make(map[string]struct{}, len(manifest.Entries))
	seenPaths := make(map[string]struct{}, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		id := strings.TrimSpace(entry.ID)
		archivePath, err := backupNormalizeManifestPath(entry.ArchivePath)
		if err != nil {
			return err
		}
		if id == "" {
			return fmt.Errorf("manifest.json 包含空条目 ID")
		}
		if _, exists := seenIDs[id]; exists {
			return fmt.Errorf("manifest.json 包含重复条目 ID: %s", id)
		}
		pathKey := archivePath
		if runtime.GOOS == "windows" {
			pathKey = strings.ToLower(pathKey)
		}
		if _, exists := seenPaths[pathKey]; exists {
			return fmt.Errorf("manifest.json 包含重复归档路径: %s", archivePath)
		}
		seenIDs[id] = struct{}{}
		seenPaths[pathKey] = struct{}{}

		if entry.FileCount < 0 || entry.ByteSize < 0 {
			return fmt.Errorf("manifest.json 条目统计值非法: %s", id)
		}
		if entry.SHA256 != "" {
			if len(entry.SHA256) != sha256.Size*2 {
				return fmt.Errorf("manifest.json 条目哈希长度非法: %s", id)
			}
			if _, err := hex.DecodeString(entry.SHA256); err != nil {
				return fmt.Errorf("manifest.json 条目哈希格式非法: %s", id)
			}
		}
		if entry.EntryType != backup.EntryTypeFile && entry.EntryType != backup.EntryTypeDir {
			return fmt.Errorf("备份条目类型不支持(%s): %s", id, entry.EntryType)
		}

		absPath := filepath.Join(extractRoot, filepath.FromSlash(archivePath))
		info, statErr := os.Stat(absPath)
		if statErr != nil {
			if os.IsNotExist(statErr) && !entry.Required {
				continue
			}
			if os.IsNotExist(statErr) {
				return fmt.Errorf("备份包缺少必需条目: %s", id)
			}
			return fmt.Errorf("读取备份条目失败(%s): %w", id, statErr)
		}
		if entry.EntryType == backup.EntryTypeFile && !info.Mode().IsRegular() {
			return fmt.Errorf("备份条目不是普通文件(%s)", id)
		}
		if entry.EntryType == backup.EntryTypeDir && !info.IsDir() {
			return fmt.Errorf("备份条目不是目录(%s)", id)
		}
		if entry.SHA256 == "" {
			continue
		}
		stats, err := backupArchiveStatsFromPath(absPath, archivePath, entry.EntryType)
		if err != nil {
			return fmt.Errorf("校验备份条目失败(%s): %w", id, err)
		}
		if stats.fileCount != entry.FileCount || stats.byteSize != entry.ByteSize || !strings.EqualFold(stats.sha256(), entry.SHA256) {
			return fmt.Errorf("备份条目校验失败(%s): 文件数量、大小或 SHA-256 不匹配", id)
		}
	}
	return nil
}

func backupNormalizeManifestPath(path string) (string, error) {
	path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	path = strings.TrimRight(path, "/")
	if path == "" || strings.HasPrefix(path, "/") || filepath.IsAbs(filepath.FromSlash(path)) {
		return "", fmt.Errorf("manifest.json 包含非法归档路径: %s", path)
	}
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("manifest.json 包含非法归档路径: %s", path)
		}
	}
	if !strings.HasPrefix(path, "payload/") {
		return "", fmt.Errorf("manifest.json 条目必须位于 payload 目录: %s", path)
	}
	return strings.Join(parts, "/"), nil
}
