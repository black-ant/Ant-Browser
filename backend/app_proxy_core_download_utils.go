package backend

import (
	"path/filepath"
	"strings"
)

func hasAnySuffix(value string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

func containsAny(value string, tokens []string) bool {
	for _, token := range tokens {
		if token != "" && strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func archiveExt(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return ".tar.gz"
	case strings.HasSuffix(lower, ".tgz"):
		return ".tgz"
	case strings.HasSuffix(lower, ".zip"):
		return ".zip"
	case strings.HasSuffix(lower, ".gz"):
		return ".gz"
	default:
		return ".tmp"
	}
}

func mustRelPath(base string, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return filepath.Base(target)
	}
	return rel
}
