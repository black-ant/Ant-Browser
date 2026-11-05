package automation

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxDiscoveredScriptLibraryPackages = 256

var automationLibraryEntryMarkers = [][]byte{
	[]byte("module.exports.run"),
	[]byte("exports.run"),
	[]byte("export async function run"),
	[]byte("export function run"),
	[]byte("export const run"),
	[]byte("export default"),
}

func DiscoverImportableScriptDirectoriesWithOptions(rootDir string) ([]string, error) {
	baseDir := filepath.Clean(strings.TrimSpace(rootDir))
	if baseDir == "" || baseDir == "." {
		return nil, fmt.Errorf("script library directory is required")
	}

	info, err := os.Stat(baseDir)
	if err != nil {
		return nil, fmt.Errorf("stat script library directory failed: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("script library path is not a directory")
	}

	discovered := make([]string, 0, 8)
	seen := make(map[string]struct{})

	err = filepath.WalkDir(baseDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}

		if path != baseDir {
			switch strings.ToLower(strings.TrimSpace(entry.Name())) {
			case ".git", "node_modules":
				return filepath.SkipDir
			}
		}

		importable, err := isDiscoverableScriptDirectory(path)
		if err != nil {
			return err
		}
		if !importable {
			return nil
		}

		normalizedPath := filepath.Clean(path)
		if _, exists := seen[normalizedPath]; !exists {
			if len(discovered) >= maxDiscoveredScriptLibraryPackages {
				return fmt.Errorf("script library contains too many importable packages")
			}
			discovered = append(discovered, normalizedPath)
			seen[normalizedPath] = struct{}{}
		}
		return filepath.SkipDir
	})
	if err != nil {
		return nil, fmt.Errorf("scan script library failed: %w", err)
	}

	if len(discovered) == 0 {
		return nil, fmt.Errorf("未在所选目录下找到可导入脚本包")
	}

	sort.Strings(discovered)
	return discovered, nil
}

func isDiscoverableScriptDirectory(dir string) (bool, error) {
	manifestPath, err := resolveImportManifest(dir)
	if err != nil {
		return false, err
	}
	if manifestPath != "" {
		return true, nil
	}

	entryPath, found, err := findDiscoverableScriptEntryFile(dir)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	return looksLikeAutomationEntryFile(entryPath)
}

func findDiscoverableScriptEntryFile(dir string) (string, bool, error) {
	for _, candidate := range []string{
		"index.cjs",
		"index.js",
		"index.mjs",
		"index.ts",
		"index.cts",
		"index.mts",
	} {
		entryPath := filepath.Join(dir, candidate)
		info, err := os.Stat(entryPath)
		if err == nil && !info.IsDir() {
			return entryPath, true, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", false, fmt.Errorf("stat script entry failed: %w", err)
		}
	}
	return "", false, nil
}

func looksLikeAutomationEntryFile(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read script entry failed: %w", err)
	}

	compact := bytes.ToLower(bytes.TrimSpace(data))
	if len(compact) == 0 {
		return false, nil
	}

	for _, marker := range automationLibraryEntryMarkers {
		if bytes.Contains(compact, bytes.ToLower(marker)) {
			return true, nil
		}
	}
	return false, nil
}
