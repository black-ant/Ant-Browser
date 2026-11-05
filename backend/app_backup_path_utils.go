package backend

import (
	"os"
	"path/filepath"
	"strings"
)

func backupNormalizePath(path string) string {
	return strings.ToLower(filepath.Clean(strings.TrimSpace(path)))
}

func backupPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func backupSamePath(a, b string) bool {
	return backupNormalizePath(a) == backupNormalizePath(b)
}

func backupPathWithin(path, root string) bool {
	p := backupNormalizePath(path)
	r := backupNormalizePath(root)
	if p == r {
		return true
	}
	if !strings.HasSuffix(r, string(filepath.Separator)) {
		r += string(filepath.Separator)
	}
	return strings.HasPrefix(p, r)
}

func backupIsNoSuchTableError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "no such table")
}

func backupUniqueNonEmpty(list []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(list))
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := backupNormalizePath(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}
