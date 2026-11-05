package backend

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func extractProxyCoreArchive(archivePath string, targetDir string, binaryBase string, targetOS string) error {
	lower := strings.ToLower(archivePath)
	if strings.HasSuffix(lower, ".zip") {
		return extractZipArchive(archivePath, targetDir)
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return extractTarGzArchive(archivePath, targetDir)
	}
	if strings.HasSuffix(lower, ".gz") {
		return extractGzipBinary(archivePath, filepath.Join(targetDir, proxyCoreBinaryName(binaryBase, targetOS)))
	}
	return fmt.Errorf("不支持的压缩格式: %s", filepath.Base(archivePath))
}

func extractGzipBinary(archivePath string, targetPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, gz)
	return err
}

func extractZipArchive(archivePath string, targetDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if err := writeArchiveFile(targetDir, file.Name, file.FileInfo().Mode(), file.FileInfo().IsDir(), func() (io.ReadCloser, error) { return file.Open() }); err != nil {
			return err
		}
	}
	return nil
}

func extractTarGzArchive(archivePath string, targetDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		mode := os.FileMode(header.Mode)
		isDir := header.FileInfo().IsDir()
		if header.Typeflag == tar.TypeDir {
			isDir = true
		}
		if err := writeArchiveFile(targetDir, header.Name, mode, isDir, func() (io.ReadCloser, error) { return io.NopCloser(tr), nil }); err != nil {
			return err
		}
	}
}

func writeArchiveFile(targetDir string, name string, mode os.FileMode, isDir bool, open func() (io.ReadCloser, error)) error {
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) || filepath.IsAbs(cleanName) {
		return fmt.Errorf("压缩包包含非法路径: %s", name)
	}
	dest := filepath.Join(targetDir, cleanName)
	if isDir {
		return os.MkdirAll(dest, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	src, err := open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode|0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}

func findProxyCoreBinary(root string, binaryBase string, targetOS string) (string, error) {
	names := []string{proxyCoreBinaryName(binaryBase, targetOS), binaryBase}
	var matches []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		base := strings.ToLower(filepath.Base(path))
		for _, name := range names {
			if proxyCoreBinaryNameMatches(base, strings.ToLower(name), binaryBase, targetOS) {
				matches = append(matches, path)
				break
			}
		}
		return nil
	})
	if len(matches) == 0 {
		return "", fmt.Errorf("解压后未找到 %s 可执行文件", binaryBase)
	}
	sort.Strings(matches)
	return matches[0], nil
}

func proxyCoreBinaryNameMatches(base string, expected string, binaryBase string, targetOS string) bool {
	if base == expected {
		return true
	}
	baseNoExt := strings.TrimSuffix(base, ".exe")
	expectedNoExt := strings.TrimSuffix(expected, ".exe")
	if baseNoExt == expectedNoExt {
		return true
	}
	if binaryBase == "mihomo" && strings.HasPrefix(baseNoExt, "mihomo-") {
		return targetOS != "windows" || strings.HasSuffix(base, ".exe")
	}
	return false
}

func normalizeInstalledProxyCoreBinary(binaryPath string, installDir string, binaryBase string, targetOS string) (string, error) {
	standardPath := filepath.Join(installDir, proxyCoreBinaryName(binaryBase, targetOS))
	if sameCleanPath(binaryPath, standardPath) {
		return binaryPath, nil
	}
	if err := os.MkdirAll(filepath.Dir(standardPath), 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(standardPath); err == nil {
		if err := os.Remove(standardPath); err != nil {
			return "", err
		}
	}
	if err := os.Rename(binaryPath, standardPath); err != nil {
		return "", err
	}
	return standardPath, nil
}

func replaceDirContents(srcDir string, dstDir string) error {
	entries, err := os.ReadDir(dstDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "proxy-core-") || strings.HasPrefix(entry.Name(), "extract-") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dstDir, entry.Name())); err != nil {
			return err
		}
	}
	return filepath.WalkDir(srcDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil || rel == "." {
			return err
		}
		dest := filepath.Join(dstDir, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return copyFile(path, dest, info.Mode())
	})
}

func copyFile(src string, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode|0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
