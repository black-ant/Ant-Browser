package browser

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

type archiveEntryMeta struct {
	Name string
}

type archiveProgress struct {
	index int
	total int
}

func SupportedCoreArchivePattern() string {
	return "*.zip;*.tar;*.tar.gz;*.tgz;*.tar.xz;*.txz;*.tar.bz2;*.tbz2"
}

func SupportedCoreArchiveDescription() string {
	return "支持 ZIP、TAR、TAR.GZ、TAR.XZ、TAR.BZ2"
}

func coreArchiveTempPattern(rawURL string) string {
	lowerName := strings.ToLower(strings.TrimSpace(rawURL))
	if parsed, err := filepathFromURLPath(lowerName); err == nil && parsed != "" {
		lowerName = parsed
	}
	for _, suffix := range coreArchiveSuffixes() {
		if strings.HasSuffix(lowerName, suffix) {
			return "download_*" + suffix
		}
	}
	return "download_*"
}

func filepathFromURLPath(raw string) (string, error) {
	parts := strings.SplitN(raw, "?", 2)
	parts = strings.SplitN(parts[0], "#", 2)
	return filepath.Base(parts[0]), nil
}

func extractCoreArchiveAndStripRoot(archivePath, dest string, progressCb func(int, string)) error {
	lower := strings.ToLower(archivePath)
	if strings.HasSuffix(lower, ".zip") {
		return extractZipArchiveAndStripRoot(archivePath, dest, progressCb)
	}
	if isTarArchivePath(lower) {
		return extractTarArchiveAndStripRoot(archivePath, dest, progressCb)
	}
	if err := extractZipArchiveAndStripRoot(archivePath, dest, progressCb); err == nil {
		return nil
	}
	return extractTarArchiveAndStripRoot(archivePath, dest, progressCb)
}

func ExtractCoreArchiveAndStripRootForImport(archivePath, dest string, progressCb func(int, string)) error {
	return extractCoreArchiveAndStripRoot(archivePath, dest, progressCb)
}

func extractZipArchiveAndStripRoot(archivePath, dest string, progressCb func(int, string)) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	if len(reader.File) == 0 {
		return fmt.Errorf("空的压缩包")
	}
	metas := make([]archiveEntryMeta, 0, len(reader.File))
	for _, file := range reader.File {
		metas = append(metas, archiveEntryMeta{Name: file.Name})
	}
	rootPrefix, hasCommonRoot := detectCommonArchiveRoot(metas)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	progress := archiveProgress{total: len(reader.File)}
	for _, file := range reader.File {
		progress.report(progressCb)
		cleanName := strippedArchiveName(file.Name, rootPrefix, hasCommonRoot)
		if cleanName == "" {
			continue
		}
		targetPath, err := safeArchiveTargetPath(dest, cleanName)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, file.Mode().Perm()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("读取压缩包文件失败 %s: %w", file.Name, err)
		}
		if err := writeReaderToFile(targetPath, rc, file.Mode().Perm()); err != nil {
			return err
		}
	}
	progressCb(100, "解压完成！")
	return nil
}

func extractTarArchiveAndStripRoot(archivePath, dest string, progressCb func(int, string)) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	stream, closeStream, err := tarStreamReader(archivePath, file)
	if err != nil {
		return err
	}
	defer closeStream()

	reader := tar.NewReader(stream)
	entryCount := 0
	topLevels := make(map[string]struct{})
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		entryCount++
		if entryCount == 1 || entryCount%50 == 0 {
			progressCb(0, fmt.Sprintf("正在解压文件 %d...", entryCount))
		}
		cleanName := strippedArchiveName(header.Name, "", false)
		if cleanName == "" {
			continue
		}
		if top := topLevelArchiveName(cleanName); top != "" {
			topLevels[top] = struct{}{}
		}
		targetPath, err := safeArchiveTargetPath(dest, cleanName)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, header.FileInfo().Mode().Perm()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			_ = os.Remove(targetPath)
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("创建符号链接失败 %s: %w", cleanName, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			if err := writeReaderToFile(targetPath, reader, header.FileInfo().Mode().Perm()); err != nil {
				return err
			}
		}
	}
	if entryCount == 0 {
		return fmt.Errorf("空的压缩包")
	}
	if err := stripSingleExtractedRoot(dest, topLevels); err != nil {
		return err
	}
	progressCb(100, "解压完成！")
	return nil
}

func tarStreamReader(archivePath string, file *os.File) (io.Reader, func(), error) {
	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		reader, err := gzip.NewReader(file)
		if err != nil {
			return nil, func() {}, err
		}
		return reader, func() { _ = reader.Close() }, nil
	case strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".txz"):
		reader, err := xz.NewReader(file)
		return reader, func() {}, err
	case strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tbz2"):
		return bzip2.NewReader(file), func() {}, nil
	case strings.HasSuffix(lower, ".tar"):
		return file, func() {}, nil
	default:
		return file, func() {}, nil
	}
}

func isTarArchivePath(path string) bool {
	for _, suffix := range coreArchiveSuffixes() {
		if suffix == ".zip" {
			continue
		}
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func coreArchiveSuffixes() []string {
	return []string{".tar.gz", ".tar.xz", ".tar.bz2", ".tgz", ".txz", ".tbz2", ".zip", ".tar"}
}

func detectCommonArchiveRoot(entries []archiveEntryMeta) (string, bool) {
	var rootPrefix string
	for _, entry := range entries {
		cleanName := normalizeArchiveEntryName(entry.Name)
		parts := strings.SplitN(cleanName, "/", 2)
		if len(parts) == 0 || parts[0] == "" {
			continue
		}
		if rootPrefix == "" {
			rootPrefix = parts[0] + "/"
			continue
		}
		if !strings.HasPrefix(cleanName, rootPrefix) && cleanName != strings.TrimSuffix(rootPrefix, "/") {
			return "", false
		}
	}
	return rootPrefix, rootPrefix != ""
}

func strippedArchiveName(name string, rootPrefix string, hasCommonRoot bool) string {
	cleanName := normalizeArchiveEntryName(name)
	if hasCommonRoot {
		if cleanName == rootPrefix || cleanName == strings.TrimSuffix(rootPrefix, "/") {
			return ""
		}
		cleanName = strings.TrimPrefix(cleanName, rootPrefix)
	}
	if cleanName == "" || cleanName == "." || cleanName == "/" {
		return ""
	}
	return cleanName
}

func topLevelArchiveName(name string) string {
	cleanName := normalizeArchiveEntryName(name)
	if cleanName == "" || cleanName == "." {
		return ""
	}
	parts := strings.SplitN(cleanName, "/", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func stripSingleExtractedRoot(dest string, topLevels map[string]struct{}) error {
	if len(topLevels) != 1 {
		return nil
	}
	var rootName string
	for name := range topLevels {
		rootName = name
	}
	rootPath, err := safeArchiveTargetPath(dest, rootName)
	if err != nil {
		return err
	}
	info, err := os.Stat(rootPath)
	if err != nil || !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		source := filepath.Join(rootPath, entry.Name())
		target := filepath.Join(dest, entry.Name())
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("剥离顶层目录失败，目标已存在: %s", target)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(source, target); err != nil {
			return err
		}
	}
	return os.Remove(rootPath)
}

func normalizeArchiveEntryName(name string) string {
	cleanName := filepath.ToSlash(strings.TrimSpace(name))
	cleanName = strings.TrimPrefix(cleanName, "/")
	return filepath.ToSlash(filepath.Clean(cleanName))
}

func safeArchiveTargetPath(dest, cleanName string) (string, error) {
	if cleanName == "." || strings.HasPrefix(cleanName, "../") || cleanName == ".." || filepath.IsAbs(cleanName) {
		return "", fmt.Errorf("非法文件路径: %s", cleanName)
	}
	targetPath := filepath.Join(dest, filepath.FromSlash(cleanName))
	destClean := filepath.Clean(dest)
	targetClean := filepath.Clean(targetPath)
	if targetClean != destClean && !strings.HasPrefix(targetClean, destClean+string(os.PathSeparator)) {
		return "", fmt.Errorf("非法文件路径: %s", cleanName)
	}
	return targetPath, nil
}

func writeReaderToFile(targetPath string, reader io.Reader, mode os.FileMode) error {
	outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("打开解压文件写入失败 %s: %w", targetPath, err)
	}
	_, copyErr := io.Copy(outFile, reader)
	closeErr := outFile.Close()
	if copyErr != nil {
		return fmt.Errorf("写入文件流失败 %s: %w", targetPath, copyErr)
	}
	return closeErr
}

func (p *archiveProgress) report(progressCb func(int, string)) {
	p.index++
	if p.total <= 0 {
		progressCb(0, "正在解压...")
		return
	}
	percent := int((float64(p.index-1) / float64(p.total)) * 100)
	if p.index == 1 || p.index%50 == 0 {
		progressCb(percent, fmt.Sprintf("正在解压文件 %d / %d...", p.index, p.total))
	}
}
