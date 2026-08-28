package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func backupSyncDir(src, dst string, stats *backupMergeStats, shouldSkip func(rel string) bool) error {
	if !backupPathExists(src) {
		return nil
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			stats.Skipped++
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if shouldSkip != nil && shouldSkip(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			stats.Skipped++
			return nil
		}

		target := filepath.Join(dst, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		if _, err := os.Stat(target); os.IsNotExist(err) {
			if err := backupCopyFile(path, target); err != nil {
				return err
			}
			stats.Imported++
			return nil
		} else if err != nil {
			return err
		}

		same, err := backupFilesSame(path, target)
		if err != nil {
			return err
		}
		if same {
			stats.Skipped++
		} else {
			stats.Conflicts++
		}
		return nil
	})
}

func backupCopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	tmpPath := dst + ".tmp"
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func backupFilesSame(a, b string) (bool, error) {
	ainfo, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	binfo, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	if ainfo.Size() != binfo.Size() {
		return false, nil
	}
	ah, err := backupSHA256File(a)
	if err != nil {
		return false, err
	}
	bh, err := backupSHA256File(b)
	if err != nil {
		return false, err
	}
	return ah == bh, nil
}

func backupSHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func backupShouldSkipAppDBFile(rel string) bool {
	r := strings.TrimSpace(filepath.ToSlash(rel))
	return r == "app.db" || r == "app.db-wal" || r == "app.db-shm"
}
