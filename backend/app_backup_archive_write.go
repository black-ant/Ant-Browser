package backend

import (
	"ant-chrome/backend/internal/backup"
	"archive/zip"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func backupWritePackageZip(zipPath string, scope backup.Scope, manifest backup.Manifest, emitProgress func(phase string, progress int, message string, meta *backupProgressMeta)) (int, int, int, error) {
	emit := func(phase string, progress int, message string, meta *backupProgressMeta) {
		if emitProgress != nil {
			emitProgress(phase, progress, message, meta)
		}
	}
	if err := os.MkdirAll(filepath.Dir(zipPath), 0755); err != nil {
		return 0, 0, 0, fmt.Errorf("创建导出目录失败: %w", err)
	}
	emit("writing", 18, "正在创建导出文件...", nil)

	tmpPath := zipPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("创建导出文件失败: %w", err)
	}
	w := zip.NewWriter(f)

	includedEntries := 0
	skippedEntries := 0
	fileCount := 0
	metadataPath := backupMetadataPath(zipPath)

	writeErr := func() error {
		totalEntries := len(scope.Entries)
		if totalEntries == 0 {
			emit("writing", 90, "没有可导出的目录条目", nil)
		}
		for i, entry := range scope.Entries {
			meta := &backupProgressMeta{
				ComponentID:   entry.ID,
				ComponentName: backupResolveEntryComponentName(entry),
				EntryIndex:    i + 1,
				EntryTotal:    totalEntries,
			}
			startProgress := 20 + int(float64(i)/float64(totalEntries)*70)
			emit("writing", startProgress, fmt.Sprintf("开始处理组件 %d/%d：%s", i+1, totalEntries, meta.ComponentName), meta)

			info, err := os.Stat(entry.SourcePath)
			if err != nil {
				if os.IsNotExist(err) && !entry.Required {
					skippedEntries++
					progress := 20 + int(float64(i+1)/float64(totalEntries)*70)
					emit("writing", progress, fmt.Sprintf("组件跳过：%s（源路径不存在）", meta.ComponentName), meta)
					continue
				}
				return fmt.Errorf("读取导出源失败(%s): %w", entry.ID, err)
			}
			entryAddedFiles := 0
			entryStats := newBackupArchiveStats()
			if info.IsDir() {
				var err error
				entryStats, err = backupZipAddDir(w, entry.SourcePath, entry.ArchivePath, zipPath, metadataPath)
				if err != nil {
					return fmt.Errorf("写入目录失败(%s): %w", entry.ID, err)
				}
				fileCount += entryStats.fileCount
				entryAddedFiles = entryStats.fileCount
			} else {
				if backupSamePath(entry.SourcePath, zipPath) || backupSamePath(entry.SourcePath, zipPath+".tmp") || backupSamePath(entry.SourcePath, metadataPath) || backupSamePath(entry.SourcePath, metadataPath+".tmp") {
					skippedEntries++
					progress := 20 + int(float64(i+1)/float64(totalEntries)*70)
					emit("writing", progress, fmt.Sprintf("组件跳过：%s（导出文件本身）", meta.ComponentName), meta)
					continue
				}
				if err := backupZipAddFile(w, entry.SourcePath, strings.TrimSuffix(entry.ArchivePath, "/"), &entryStats); err != nil {
					return fmt.Errorf("写入文件失败(%s): %w", entry.ID, err)
				}
				fileCount++
				entryAddedFiles = 1
			}
			for i := range manifest.Entries {
				if manifest.Entries[i].ID != entry.ID {
					continue
				}
				manifest.Entries[i].FileCount = entryStats.fileCount
				manifest.Entries[i].ByteSize = entryStats.byteSize
				manifest.Entries[i].SHA256 = entryStats.sha256()
				break
			}
			includedEntries++
			progress := 20 + int(float64(i+1)/float64(totalEntries)*70)
			emit("writing", progress, fmt.Sprintf("组件完成：%s（新增 %d 个文件）", meta.ComponentName, entryAddedFiles), meta)
		}
		emit("writing", 94, "正在写入备份清单...", nil)
		manifestData, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return err
		}
		mw, err := w.Create("manifest.json")
		if err != nil {
			return err
		}
		if _, err := mw.Write(manifestData); err != nil {
			return err
		}
		fileCount++
		return nil
	}()

	closeErr := w.Close()
	fileCloseErr := f.Close()
	if writeErr != nil {
		emit("error", 100, writeErr.Error(), nil)
		_ = os.Remove(tmpPath)
		return 0, 0, 0, writeErr
	}
	if closeErr != nil {
		emit("error", 100, closeErr.Error(), nil)
		_ = os.Remove(tmpPath)
		return 0, 0, 0, closeErr
	}
	if fileCloseErr != nil {
		emit("error", 100, fileCloseErr.Error(), nil)
		_ = os.Remove(tmpPath)
		return 0, 0, 0, fileCloseErr
	}
	if err := os.Rename(tmpPath, zipPath); err != nil {
		emit("error", 100, err.Error(), nil)
		_ = os.Remove(tmpPath)
		return 0, 0, 0, fmt.Errorf("写入导出文件失败: %w", err)
	}
	if _, err := backupWriteMetadata(zipPath, manifest, includedEntries, skippedEntries); err != nil {
		emit("warning", 100, fmt.Sprintf("备份文件已生成，但元数据文件写入失败：%v", err), nil)
	}
	return includedEntries, skippedEntries, fileCount, nil
}

func backupZipAddDir(w *zip.Writer, srcDir, archiveBase, outputZipPath, outputMetadataPath string) (backupArchiveStats, error) {
	base := strings.TrimSuffix(filepath.ToSlash(strings.TrimSpace(archiveBase)), "/")
	if base == "" {
		return backupArchiveStats{}, fmt.Errorf("archive base 不能为空")
	}
	stats := newBackupArchiveStats()
	if _, err := w.Create(base + "/"); err != nil {
		return backupArchiveStats{}, err
	}
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if backupSamePath(path, outputZipPath) || backupSamePath(path, outputZipPath+".tmp") || backupSamePath(path, outputMetadataPath) || backupSamePath(path, outputMetadataPath+".tmp") {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		targetName := base + "/" + rel
		if d.IsDir() {
			_, err := w.Create(strings.TrimSuffix(targetName, "/") + "/")
			return err
		}
		if err := backupZipAddFile(w, path, targetName, &stats); err != nil {
			return err
		}
		return nil
	})
	return stats, err
}

func backupZipAddFile(w *zip.Writer, srcFile, archivePath string, stats *backupArchiveStats) error {
	info, err := os.Stat(srcFile)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("不支持将目录按文件写入: %s", srcFile)
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(archivePath)), "/")
	header.Method = zip.Deflate
	if header.Name == "" {
		return fmt.Errorf("archivePath 不能为空")
	}
	writer, err := w.CreateHeader(header)
	if err != nil {
		return err
	}
	if stats == nil {
		return fmt.Errorf("备份条目统计器不能为空")
	}
	return stats.addFile(header.Name, srcFile, writer)
}
