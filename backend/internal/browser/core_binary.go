package browser

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

// CoreExecutableCandidates 返回当前平台可接受的浏览器可执行文件候选名。
func CoreExecutableCandidates() []string {
	switch goruntime.GOOS {
	case "windows":
		return []string{"chrome.exe"}
	case "linux":
		return []string{"chrome", "chrome-bin", "chromium", "chromium-browser", "ungoogled-chromium", "chrome.exe"}
	case "darwin":
		return []string{
			"Google Chrome.app/Contents/MacOS/Google Chrome",
			"Chromium.app/Contents/MacOS/Chromium",
			"chrome",
		}
	default:
		return []string{"chrome"}
	}
}

func CoreExecutablePlatform() string {
	return goruntime.GOOS + "/" + goruntime.GOARCH
}

// FindCoreExecutable 在指定目录查找可执行文件，返回绝对路径和命中的候选名。
func FindCoreExecutable(baseDir string) (string, string, bool) {
	if directPath, directCandidate, ok := FindCoreExecutableShallow(baseDir); ok {
		return directPath, directCandidate, true
	}
	if recursivePath, recursiveCandidate, ok := findNestedCoreExecutable(baseDir); ok {
		return recursivePath, recursiveCandidate, true
	}
	return "", "", false
}

func FindCoreExecutableShallow(baseDir string) (string, string, bool) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return "", "", false
	}
	if directPath, directCandidate, ok := findDirectCoreExecutable(baseDir); ok {
		return directPath, directCandidate, true
	}
	if bundlePath, bundleCandidate, ok := findAppBundleExecutable(baseDir); ok {
		return bundlePath, bundleCandidate, true
	}
	for _, candidate := range CoreExecutableCandidates() {
		p := filepath.Join(baseDir, filepath.FromSlash(candidate))
		if _, err := os.Stat(p); err == nil {
			return p, candidate, true
		}
	}
	return "", "", false
}

func findNestedCoreExecutable(baseDir string) (string, string, bool) {
	info, err := os.Stat(baseDir)
	if err != nil || !info.IsDir() {
		return "", "", false
	}
	baseDepth := strings.Count(filepath.ToSlash(filepath.Clean(baseDir)), "/")
	candidateNames := make(map[string]string)
	for _, candidate := range CoreExecutableCandidates() {
		candidateNames[strings.ToLower(filepath.Base(candidate))] = candidate
	}

	var matchedPath string
	var matchedCandidate string
	_ = filepath.WalkDir(baseDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || path == baseDir || matchedPath != "" {
			return nil
		}
		if entry.IsDir() {
			depth := strings.Count(filepath.ToSlash(filepath.Clean(path)), "/") - baseDepth
			if depth > 5 {
				return filepath.SkipDir
			}
			return nil
		}
		candidate, ok := candidateNames[strings.ToLower(entry.Name())]
		if !ok {
			return nil
		}
		matchedPath = path
		matchedCandidate = candidate
		return nil
	})
	if matchedPath == "" {
		return "", "", false
	}
	return matchedPath, matchedCandidate, true
}

func findDirectCoreExecutable(path string) (string, string, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", "", false
	}

	normalized := filepath.ToSlash(filepath.Clean(path))
	for _, candidate := range CoreExecutableCandidates() {
		candidatePath := filepath.ToSlash(candidate)
		if strings.HasSuffix(normalized, candidatePath) || filepath.Base(normalized) == filepath.Base(candidatePath) {
			return path, candidate, true
		}
	}

	return "", "", false
}

func findAppBundleExecutable(path string) (string, string, bool) {
	if goruntime.GOOS != "darwin" {
		return "", "", false
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", "", false
	}

	normalized := filepath.ToSlash(filepath.Clean(path))
	if !strings.HasSuffix(strings.ToLower(normalized), ".app") {
		return "", "", false
	}

	for _, candidate := range CoreExecutableCandidates() {
		candidatePath := filepath.ToSlash(candidate)
		appMarker := ".app/"
		index := strings.Index(strings.ToLower(candidatePath), appMarker)
		if index < 0 {
			continue
		}
		if !strings.EqualFold(filepath.Base(normalized), filepath.Base(candidatePath[:index+len(".app")])) {
			continue
		}

		relativeExecutable := candidatePath[index+len(appMarker):]
		if relativeExecutable == "" {
			continue
		}

		p := filepath.Join(path, filepath.FromSlash(relativeExecutable))
		if _, err := os.Stat(p); err == nil {
			return p, candidate, true
		}
	}

	return "", "", false
}
