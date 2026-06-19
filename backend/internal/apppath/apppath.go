package apppath

import (
	"ant-chrome/backend/internal/fsutil"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
)

const appStateDirName = "ant-browser"

type roots struct {
	installRoot string
	stateRoot   string
	detached    bool
}

var rootsCache sync.Map

// InstallRoot 返回应用安装根目录的绝对路径。
func InstallRoot(appRoot string) string {
	return detect(appRoot).installRoot
}

// StateRoot 返回应用可写状态目录的绝对路径。
func StateRoot(appRoot string) string {
	return detect(appRoot).stateRoot
}

// IsDetached 返回当前是否启用了“安装目录只读、状态目录独立”的模式。
func IsDetached(appRoot string) bool {
	return detect(appRoot).detached
}

// Resolve 将相对路径解析到安装目录或用户状态目录。
// 已安装的 Linux / macOS 应用在启用 detached 模式后，除 bin/ 外的相对路径都会落到用户可写目录。
func Resolve(appRoot, p string) string {
	return resolveForOS(appRoot, p, goruntime.GOOS)
}

func resolveForOS(appRoot, p, goos string) string {
	p = fsutil.NormalizePathInput(p)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}

	root := detectForOS(appRoot, goos)
	base := root.installRoot
	if root.detached && useStateRoot(p) {
		base = root.stateRoot
	}
	return filepath.Join(base, p)
}

// EnsureWritableLayout 为需要 detached 状态目录的已安装应用准备首启所需的可写目录，
// 并把随包默认配置迁移到用户目录。
func EnsureWritableLayout(appRoot string) error {
	return ensureWritableLayoutForOS(appRoot, goruntime.GOOS)
}

func ensureWritableLayoutForOS(appRoot, goos string) error {
	root := detectForOS(appRoot, goos)
	if !root.detached {
		return nil
	}

	if err := os.MkdirAll(root.stateRoot, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root.stateRoot, "data"), 0755); err != nil {
		return err
	}

	if err := copyFileIfMissing(
		filepath.Join(root.installRoot, "config.yaml"),
		filepath.Join(root.stateRoot, "config.yaml"),
	); err != nil {
		return err
	}
	if err := copyFileIfMissing(
		filepath.Join(root.installRoot, "proxies.yaml"),
		filepath.Join(root.stateRoot, "proxies.yaml"),
	); err != nil {
		return err
	}
	if err := copyDirIfMissing(
		filepath.Join(root.installRoot, "chrome"),
		filepath.Join(root.stateRoot, "chrome"),
	); err != nil {
		return err
	}

	return nil
}

func detect(appRoot string) roots {
	return detectForOS(appRoot, goruntime.GOOS)
}

func detectForOS(appRoot, goos string) roots {
	normalized := normalizeRoot(appRoot)
	cacheKey := buildCacheKey(goos, normalized)
	if cached, ok := rootsCache.Load(cacheKey); ok {
		return cached.(roots)
	}

	root := roots{
		installRoot: normalized,
		stateRoot:   normalized,
	}
	if shouldDetachStateRoot(goos, normalized) {
		root.stateRoot = userStateRootForOS(goos, normalized)
		root.detached = root.stateRoot != "" && root.stateRoot != normalized
	}

	actual, _ := rootsCache.LoadOrStore(cacheKey, root)
	return actual.(roots)
}

func buildCacheKey(goos, root string) string {
	return normalizeGOOS(goos) + "\x00" + root
}

func normalizeGOOS(goos string) string {
	return strings.ToLower(strings.TrimSpace(goos))
}

func shouldDetachStateRoot(goos, installRoot string) bool {
	switch normalizeGOOS(goos) {
	case "linux":
		return !dirWritable(installRoot)
	case "darwin":
		return isMacAppBundleRoot(installRoot) || !dirWritable(installRoot)
	default:
		return false
	}
}

func normalizeRoot(appRoot string) string {
	root := strings.TrimSpace(appRoot)
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}
	if root == "" {
		root = "."
	}
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return filepath.Clean(root)
}

func userStateRootForOS(goos, fallback string) string {
	switch normalizeGOOS(goos) {
	case "linux":
		if base := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); base != "" {
			return filepath.Join(base, appStateDirName)
		}
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			return filepath.Join(home, ".local", "share", appStateDirName)
		}
	case "darwin":
		if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
			return filepath.Join(home, "Library", "Application Support", appStateDirName)
		}
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			return filepath.Join(home, "Library", "Application Support", appStateDirName)
		}
	}
	if tmp := strings.TrimSpace(os.TempDir()); tmp != "" {
		return filepath.Join(tmp, appStateDirName)
	}
	return fallback
}

func isMacAppBundleRoot(dir string) bool {
	clean := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(dir)), "/")
	lower := strings.ToLower(clean)
	return strings.HasSuffix(lower, ".app/contents/macos") || strings.HasSuffix(lower, ".app/contents/resources")
}

func dirWritable(dir string) bool {
	file, err := os.CreateTemp(dir, ".ant-browser-write-test-*")
	if err != nil {
		return false
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return true
}

func useStateRoot(p string) bool {
	clean := filepath.ToSlash(fsutil.NormalizePathInput(p))
	if clean == "" || clean == "." {
		return false
	}
	return clean != "bin" && !strings.HasPrefix(clean, "bin/")
}

func copyFileIfMissing(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func copyDirIfMissing(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := dst
		if rel != "." {
			target = filepath.Join(dst, rel)
		}
		if info.IsDir() {
			dirMode := info.Mode().Perm() | 0700
			return os.MkdirAll(target, dirMode)
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
