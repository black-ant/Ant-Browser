package backend

import (
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

// OpenUserDataDir 在资源管理器中打开用户数据目录
func (a *App) OpenUserDataDir(userDataDir string) error {
	log := logger.New("Browser")

	userDataRoot := ""
	if a.config != nil {
		userDataRoot = a.config.Browser.UserDataRoot
	}
	fullPath, err := fsutil.ResolveUserDataDir(a.resolveAppPath, userDataRoot, userDataDir)
	if err != nil {
		return err
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			log.Error("创建用户数据目录失败", logger.F("path", fullPath), logger.F("error", err))
			return fmt.Errorf("创建目录失败: %v", err)
		}
	}

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		log.Error("获取绝对路径失败", logger.F("path", fullPath), logger.F("error", err))
		return err
	}

	if err := openPathInFileManager(absPath); err != nil {
		log.Error("打开资源管理器失败", logger.F("path", absPath), logger.F("error", err))
		return err
	}

	log.Info("已打开用户数据目录", logger.F("path", absPath))
	return nil
}

// OpenUserDataRoot 在资源管理器中打开浏览器用户数据根目录。
func (a *App) OpenUserDataRoot() error {
	log := logger.New("Browser")

	userDataRoot := "data"
	if a.config != nil && strings.TrimSpace(a.config.Browser.UserDataRoot) != "" {
		userDataRoot = strings.TrimSpace(a.config.Browser.UserDataRoot)
	}
	rootPath := a.resolveAppPath(userDataRoot)
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		if err := os.MkdirAll(rootPath, 0755); err != nil {
			log.Error("创建用户数据根目录失败", logger.F("path", rootPath), logger.F("error", err))
			return fmt.Errorf("创建目录失败: %v", err)
		}
	}

	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		log.Error("获取用户数据根目录绝对路径失败", logger.F("path", rootPath), logger.F("error", err))
		return err
	}

	if err := openPathInFileManager(absPath); err != nil {
		log.Error("打开用户数据根目录失败", logger.F("path", absPath), logger.F("error", err))
		return err
	}

	log.Info("已打开用户数据根目录", logger.F("path", absPath))
	return nil
}

// OpenCorePath 在资源管理器中打开内核路径
func (a *App) OpenCorePath(corePath string) error {
	log := logger.New("Browser")

	fullPath, err := fsutil.ResolveExistingPath(a.resolveAppPath, corePath, "内核路径不能为空")
	if err != nil {
		return err
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("路径不存在: %s", fullPath)
	}

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		log.Error("获取绝对路径失败", logger.F("path", fullPath), logger.F("error", err))
		return err
	}

	if err := openPathInFileManager(absPath); err != nil {
		log.Error("打开资源管理器失败", logger.F("path", absPath), logger.F("error", err))
		return err
	}

	log.Info("已打开内核路径", logger.F("path", absPath))
	return nil
}

// OpenProjectRoot 在资源管理器中打开项目根目录
func (a *App) OpenProjectRoot() error {
	log := logger.New("Browser")

	rootPath := strings.TrimSpace(a.appRootAbs())
	if rootPath == "" {
		return fmt.Errorf("项目根目录不能为空")
	}

	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		return fmt.Errorf("项目根目录不存在: %s", rootPath)
	}

	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		log.Error("获取项目根目录绝对路径失败", logger.F("path", rootPath), logger.F("error", err))
		return err
	}

	if err := openPathInFileManager(absPath); err != nil {
		log.Error("打开项目根目录失败", logger.F("path", absPath), logger.F("error", err))
		return err
	}

	log.Info("已打开项目根目录", logger.F("path", absPath))
	return nil
}

// openPathInFileManager 调用系统文件管理器打开路径。
// Windows 下不能复用 hideWindow，否则可能导致资源管理器窗口被隐藏。
func openPathInFileManager(absPath string) error {
	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}

	switch goruntime.GOOS {
	case "windows":
		if info.IsDir() {
			return exec.Command("explorer.exe", absPath).Start()
		}
		return exec.Command("explorer.exe", "/select,", absPath).Start()
	case "darwin":
		if info.IsDir() {
			return exec.Command("open", absPath).Start()
		}
		return exec.Command("open", "-R", absPath).Start()
	default:
		target := absPath
		if !info.IsDir() {
			target = filepath.Dir(absPath)
		}
		return exec.Command("xdg-open", target).Start()
	}
}
