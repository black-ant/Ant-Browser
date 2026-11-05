package snapshot

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type UnzipLimits struct {
	MaxEntries           int
	MaxUncompressedBytes uint64
	MaxSingleFileBytes   uint64
	MaxCompressedBytes   uint64
}

func ZipDir(src, dest string) error {
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

func UnzipTo(src, dest string) error {
	return UnzipToWithLimits(src, dest, UnzipLimits{})
}

func UnzipToWithLimits(src, dest string, limits UnzipLimits) error {
	if limits.MaxCompressedBytes > 0 {
		info, err := os.Stat(src)
		if err != nil {
			return err
		}
		if uint64(info.Size()) > limits.MaxCompressedBytes {
			return fmt.Errorf("压缩包大小超过限制: %d > %d", info.Size(), limits.MaxCompressedBytes)
		}
	}

	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	seen := make(map[string]struct{}, len(r.File))
	entryCount := 0
	var totalBytes uint64
	for _, f := range r.File {
		entryCount++
		if limits.MaxEntries > 0 && entryCount > limits.MaxEntries {
			return fmt.Errorf("压缩包条目数量超过限制: %d > %d", entryCount, limits.MaxEntries)
		}

		memberName, err := normalizeArchiveMemberName(f.Name)
		if err != nil {
			return err
		}
		if _, exists := seen[memberName]; exists {
			return fmt.Errorf("压缩包包含重复路径: %s", memberName)
		}
		seen[memberName] = struct{}{}

		target := filepath.Join(dest, filepath.FromSlash(memberName))
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) &&
			filepath.Clean(target) != filepath.Clean(dest) {
			return fmt.Errorf("非法路径: %s", memberName)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if f.FileInfo().Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("压缩包不允许包含符号链接: %s", memberName)
		}
		if limits.MaxSingleFileBytes > 0 && f.UncompressedSize64 > limits.MaxSingleFileBytes {
			return fmt.Errorf("压缩包文件过大(%s): %d > %d", memberName, f.UncompressedSize64, limits.MaxSingleFileBytes)
		}
		if limits.MaxUncompressedBytes > 0 && f.UncompressedSize64 > limits.MaxUncompressedBytes-totalBytes {
			return fmt.Errorf("解压后大小超过限制: %d > %d", totalBytes+f.UncompressedSize64, limits.MaxUncompressedBytes)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
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
		copyReader := io.Reader(rc)
		copyLimit := uint64(0)
		if limits.MaxSingleFileBytes > 0 {
			copyLimit = limits.MaxSingleFileBytes
		}
		if limits.MaxUncompressedBytes > 0 {
			remaining := limits.MaxUncompressedBytes - totalBytes
			if copyLimit == 0 || remaining < copyLimit {
				copyLimit = remaining
			}
		}
		if copyLimit > 0 {
			copyReader = io.LimitReader(rc, int64(copyLimit)+1)
		}
		written, copyErr := io.Copy(out, copyReader)
		rc.Close()
		out.Close()
		if copyErr != nil {
			return copyErr
		}
		if limits.MaxSingleFileBytes > 0 && uint64(written) > limits.MaxSingleFileBytes {
			return fmt.Errorf("解压后文件过大(%s): %d > %d", memberName, written, limits.MaxSingleFileBytes)
		}
		totalBytes += uint64(written)
		if limits.MaxUncompressedBytes > 0 && totalBytes > limits.MaxUncompressedBytes {
			return fmt.Errorf("解压后大小超过限制: %d > %d", totalBytes, limits.MaxUncompressedBytes)
		}
	}
	return nil
}

func normalizeArchiveMemberName(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("压缩包包含空路径或非法字符")
	}
	name = strings.TrimRight(name, "/")
	if name == "" {
		return "", fmt.Errorf("压缩包包含空路径")
	}
	if strings.HasPrefix(name, "/") || filepath.IsAbs(filepath.FromSlash(name)) {
		return "", fmt.Errorf("非法路径: %s", name)
	}
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("非法路径: %s", name)
		}
	}
	return strings.Join(parts, "/"), nil
}
