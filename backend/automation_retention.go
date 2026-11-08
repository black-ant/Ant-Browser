package backend

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
)

const (
	runtimeCleanupInitialDelay = 2 * time.Minute
	runtimeCleanupInterval     = 6 * time.Hour

	defaultAutomationRunRetentionDays      = 7
	defaultAutomationArtifactRetentionDays = 7
	defaultProxyWorkdirRetentionDays       = 3
)

// startupInitRuntimeCleanup 启动后台清理循环：按天淘汰历史运行记录/产物，回收未引用的内核工作目录与残留锁文件。
func (a *App) startupInitRuntimeCleanup() {
	if a == nil {
		return
	}
	if err := a.startBackgroundTask(func(ctx context.Context) {
		a.runtimeCleanupLoop(ctx)
	}); err != nil {
		logger.New("Cleanup").Warn("启动运行环境清理任务失败", logger.F("error", err))
	}
}

func (a *App) runtimeCleanupLoop(ctx context.Context) {
	timer := time.NewTimer(runtimeCleanupInitialDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	a.runRuntimeCleanupOnce()

	ticker := time.NewTicker(runtimeCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.runRuntimeCleanupOnce()
		}
	}
}

func (a *App) runRuntimeCleanupOnce() {
	if a == nil || a.config == nil {
		return
	}
	log := logger.New("Cleanup")
	now := time.Now()

	runCutoff := now.AddDate(0, 0, -a.automationRunRetentionDays())
	if dir := a.resolveAppPath(filepath.ToSlash(filepath.Join("data", "automation", "runs"))); dir != "" {
		if count, bytesRemoved, err := pruneAutomationRunRecords(dir, runCutoff); err != nil {
			log.Warn("清理历史运行记录失败", logger.F("dir", dir), logger.F("error", err))
		} else if count > 0 {
			log.Info("已清理历史运行记录",
				logger.F("count", count),
				logger.F("bytes", bytesRemoved),
				logger.F("cutoff", runCutoff.Format(time.RFC3339)),
			)
		}
	}

	artifactCutoff := now.AddDate(0, 0, -a.automationArtifactRetentionDays())
	artifactDir := strings.TrimSpace(a.config.Automation.ArtifactsDir)
	if artifactDir == "" {
		artifactDir = filepath.ToSlash(filepath.Join("data", "automation", "artifacts"))
	}
	if dir := a.resolveAppPath(artifactDir); dir != "" {
		if count, bytesRemoved, err := pruneAutomationArtifacts(dir, artifactCutoff); err != nil {
			log.Warn("清理自动化产物失败", logger.F("dir", dir), logger.F("error", err))
		} else if count > 0 {
			log.Info("已清理自动化产物",
				logger.F("count", count),
				logger.F("bytes", bytesRemoved),
				logger.F("cutoff", artifactCutoff.Format(time.RFC3339)),
			)
		}
	}

	workdirCutoff := now.AddDate(0, 0, -a.proxyWorkdirRetentionDays())
	matcher := newRuntimePathMatcher()
	if a.browserMgr != nil {
		root := a.browserMgr.ResolveRelativePath(a.config.Browser.UserDataRoot)
		for _, name := range []string{"_xray", "_singbox"} {
			dir := filepath.Join(root, name)
			if count, bytesRemoved, err := pruneProxyWorkdirs(dir, workdirCutoff, matcher.InUse); err != nil {
				log.Warn("清理内核工作目录失败", logger.F("dir", dir), logger.F("error", err))
			} else if count > 0 {
				log.Info("已清理内核工作目录",
					logger.F("dir", name),
					logger.F("count", count),
					logger.F("bytes", bytesRemoved),
				)
			}
		}
	}

	a.cleanupStaleProfileLocks(log, matcher)
}

func (a *App) automationRunRetentionDays() int {
	if a != nil && a.config != nil && a.config.Automation.RunRetentionDays > 0 {
		return a.config.Automation.RunRetentionDays
	}
	return defaultAutomationRunRetentionDays
}

func (a *App) automationArtifactRetentionDays() int {
	if a != nil && a.config != nil && a.config.Automation.ArtifactRetentionDays > 0 {
		return a.config.Automation.ArtifactRetentionDays
	}
	return defaultAutomationArtifactRetentionDays
}

func (a *App) proxyWorkdirRetentionDays() int {
	if a != nil && a.config != nil && a.config.Browser.ProxyWorkdirRetentionDays > 0 {
		return a.config.Browser.ProxyWorkdirRetentionDays
	}
	return defaultProxyWorkdirRetentionDays
}

// pruneAutomationRunRecords 删除最后修改时间早于 cutoff 的运行记录 JSON。
func pruneAutomationRunRecords(root string, cutoff time.Time) (int, int64, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return 0, 0, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	removed := 0
	var bytesRemoved int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(root, entry.Name())); err != nil {
			continue
		}
		removed++
		bytesRemoved += info.Size()
	}
	return removed, bytesRemoved, nil
}

// pruneAutomationArtifacts 删除 artifacts/<script>/<runDir> 中早于 cutoff 的运行目录。
func pruneAutomationArtifacts(root string, cutoff time.Time) (int, int64, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return 0, 0, nil
	}
	scriptEntries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	removed := 0
	var bytesRemoved int64
	for _, scriptEntry := range scriptEntries {
		if !scriptEntry.IsDir() {
			continue
		}
		scriptDir := filepath.Join(root, scriptEntry.Name())
		runEntries, err := os.ReadDir(scriptDir)
		if err != nil {
			continue
		}
		for _, runEntry := range runEntries {
			info, err := runEntry.Info()
			if err != nil || !info.ModTime().Before(cutoff) {
				continue
			}
			target := filepath.Join(scriptDir, runEntry.Name())
			size := pathSize(target)
			if err := os.RemoveAll(target); err != nil {
				continue
			}
			removed++
			bytesRemoved += size
		}
		if remaining, err := os.ReadDir(scriptDir); err == nil && len(remaining) == 0 {
			_ = os.Remove(scriptDir)
		}
	}
	return removed, bytesRemoved, nil
}

// pruneProxyWorkdirs 删除早于 cutoff 且当前没有进程引用的内核工作目录。
func pruneProxyWorkdirs(root string, cutoff time.Time, inUse func(string) bool) (int, int64, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return 0, 0, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	removed := 0
	var bytesRemoved int64
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if inUse != nil && inUse(dir) {
			continue
		}
		size := pathSize(dir)
		if err := os.RemoveAll(dir); err != nil {
			continue
		}
		removed++
		bytesRemoved += size
	}
	return removed, bytesRemoved, nil
}

// runtimePathMatcher 一次清理任务内只快照一遍进程命令行，
// 避免按目录逐个扫 /proc（几百个内核目录 × 上百个进程会放大成数万次读文件）。
type runtimePathMatcher struct {
	once     sync.Once
	cmdlines [][]byte
	unknown  bool
}

func newRuntimePathMatcher() *runtimePathMatcher {
	return &runtimePathMatcher{}
}

func (m *runtimePathMatcher) load() {
	if runtime.GOOS != "linux" {
		m.unknown = true
		return
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		m.unknown = true
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil || len(data) == 0 {
			continue
		}
		m.cmdlines = append(m.cmdlines, bytes.ReplaceAll(data, []byte{0}, []byte{' '}))
	}
}

// InUse 判断目录是否被任何进程引用；无法判断时保守返回 true（不清理）。
func (m *runtimePathMatcher) InUse(path string) bool {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "" || cleanPath == "." {
		return true
	}
	if m == nil {
		return true
	}
	m.once.Do(m.load)
	if m.unknown {
		return true
	}
	needle := []byte(cleanPath)
	for _, cmdline := range m.cmdlines {
		if cmdlineContainsPath(cmdline, needle) {
			return true
		}
	}
	return false
}

// cmdlineContainsPath 按路径边界匹配，避免 /data/_xray/abc 命中 /data/_xray/abcd。
func cmdlineContainsPath(cmdline []byte, needle []byte) bool {
	if len(needle) == 0 {
		return false
	}
	for offset := 0; offset+len(needle) <= len(cmdline); {
		index := bytes.Index(cmdline[offset:], needle)
		if index < 0 {
			return false
		}
		end := offset + index + len(needle)
		if end == len(cmdline) {
			return true
		}
		switch cmdline[end] {
		case '/', ' ', '"', '\'', 0, '\t':
			return true
		}
		offset = offset + index + 1
	}
	return false
}

func pathSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() {
			return nil
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// cleanupStaleProfileLocks 清理已停止实例残留的 Chromium 单例锁。
func (a *App) cleanupStaleProfileLocks(log *logger.Logger, matcher *runtimePathMatcher) {
	if a == nil || a.browserMgr == nil {
		return
	}
	a.browserMgr.Mutex.Lock()
	profiles := make([]*browser.Profile, 0, len(a.browserMgr.Profiles))
	for _, profile := range a.browserMgr.Profiles {
		if profile != nil {
			profiles = append(profiles, profile)
		}
	}
	a.browserMgr.Mutex.Unlock()

	removed := 0
	for _, profile := range profiles {
		if profile.Running {
			continue
		}
		userDataDir := strings.TrimSpace(a.browserMgr.ResolveUserDataDir(profile))
		if userDataDir == "" || matcher.InUse(userDataDir) {
			continue
		}
		removed += len(removeChromiumSingletonLocks(userDataDir))
	}
	if removed > 0 && log != nil {
		log.Info("已清理残留浏览器单例锁", logger.F("count", removed))
	}
}
