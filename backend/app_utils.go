package backend

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// 工具函数
// ============================================================================

// resolveAppPath 将相对路径解析为绝对路径（基于 appRoot）。
// 如果传入的已经是绝对路径则直接返回。
func (a *App) resolveAppPath(p string) string {
	return apppath.Resolve(a.appRoot, p)
}

func generateUUID() string {
	return uuid.NewString()
}

func nextAvailablePort() (int, error) {
	// 二次验证策略：分配端口后立即再次绑定确认未被抢占，最多重试 10 次
	for i := 0; i < 10; i++ {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			continue
		}
		port := l.Addr().(*net.TCPAddr).Port
		l.Close()
		// 短暂等待 OS 释放端口
		time.Sleep(5 * time.Millisecond)
		// 二次验证端口未被其他进程抢占
		v, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		v.Close()
		return port, nil
	}
	return 0, fmt.Errorf("无法分配可用端口")
}

// ============================================================================
// 内核初始化
// ============================================================================

func (a *App) ensureDefaultCores() {
	log := logger.New("Browser")

	// 扫描 chrome/ 目录，无论配置是否已有内核都执行一次，确保新增子目录被发现
	detected := a.scanChromeDir(a.browserCoreRoot())

	if len(a.config.Browser.Cores) == 0 {
		// 配置为空：直接用扫描结果，或兜底写一个占位
		if len(detected) > 0 {
			a.config.Browser.Cores = detected
		} else {
			a.config.Browser.Cores = []browser.Core{}
		}
		if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
			log.Error("内核配置初始化失败", logger.F("error", err))
			return
		}
		log.Info("内核配置初始化完成", logger.F("count", len(a.config.Browser.Cores)))
		return
	}

	// 配置已有内核：将扫描到的新目录追加进去（不覆盖已有的）
	changed := false
	for _, newCore := range detected {
		exists := false
		for _, existing := range a.config.Browser.Cores {
			if existing.CorePath == newCore.CorePath {
				exists = true
				break
			}
		}
		if !exists {
			a.config.Browser.Cores = append(a.config.Browser.Cores, newCore)
			log.Info("发现新内核，已注册", logger.F("path", newCore.CorePath))
			changed = true
		}
	}
	if changed {
		if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
			log.Error("新内核注册保存失败", logger.F("error", err))
		}
	}
}

func (a *App) autoDetectCores() {
	log := logger.New("Browser")
	cores := a.scanAndRegisterCores()
	// SQLite 模式下以内核表为准，避免与 config.yaml 历史条目不一致。
	if a.browserMgr != nil {
		cores = a.browserMgr.ListCores()
	}
	for _, core := range cores {
		if a.browserMgr == nil {
			break
		}
		result := a.browserMgr.ValidateCorePath(core.CorePath)
		if result.Valid {
			log.Debug("内核路径有效", logger.F("core_id", core.CoreId), logger.F("path", core.CorePath))
		} else {
			log.Warn("内核路径无效", logger.F("core_id", core.CoreId), logger.F("path", core.CorePath), logger.F("message", result.Message))
		}
	}
}

func (a *App) browserCoreRoot() string {
	if a != nil && a.config != nil {
		if root := strings.TrimSpace(a.config.Browser.CoreRoot); root != "" {
			return root
		}
	}
	return "chrome"
}

func (a *App) scanAndRegisterCores() []browser.Core {
	log := logger.New("Browser")
	detected := a.scanChromeDir(a.browserCoreRoot())
	if len(detected) == 0 || a.browserMgr == nil {
		return detected
	}

	existing := a.browserMgr.ListCores()
	knownPaths := make(map[string]struct{}, len(existing))
	hasDefault := false
	for _, core := range existing {
		knownPaths[normalizeCorePathForCompare(core.CorePath)] = struct{}{}
		if core.IsDefault {
			hasDefault = true
		}
	}

	for _, core := range detected {
		if _, ok := knownPaths[normalizeCorePathForCompare(core.CorePath)]; ok {
			continue
		}
		core.IsDefault = !hasDefault
		if err := a.browserMgr.SaveCore(browser.CoreInput{
			CoreId:    core.CoreId,
			CoreName:  core.CoreName,
			CorePath:  core.CorePath,
			IsDefault: core.IsDefault,
		}); err != nil {
			log.Warn("自动注册内核失败", logger.F("core_id", core.CoreId), logger.F("path", core.CorePath), logger.F("error", err.Error()))
			continue
		}
		log.Info("发现新内核，已注册", logger.F("core_id", core.CoreId), logger.F("path", core.CorePath))
		knownPaths[normalizeCorePathForCompare(core.CorePath)] = struct{}{}
		hasDefault = hasDefault || core.IsDefault
	}
	return a.browserMgr.ListCores()
}

func normalizeCorePathForCompare(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(path))
}

// scanChromeDir 扫描指定目录，将包含浏览器可执行文件的子文件夹识别为内核。
// 如果目录本身包含可执行文件（旧版单内核结构），则直接返回该目录作为内核。
func (a *App) scanChromeDir(chromeRoot string) []browser.Core {
	log := logger.New("Browser")

	baseDir := a.resolveAppPath(chromeRoot)

	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return nil
	}

	// 如果根目录本身就有浏览器可执行文件，视为单内核结构
	if _, _, ok := browser.FindCoreExecutableShallow(baseDir); ok {
		return []browser.Core{
			{
				CoreId:    "default",
				CoreName:  "默认内核",
				CorePath:  chromeRoot,
				IsDefault: true,
			},
		}
	}

	// 扫描子文件夹
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		log.Warn("扫描 chrome 目录失败", logger.F("path", baseDir), logger.F("error", err.Error()))
		return nil
	}

	var cores []browser.Core
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subPath := filepath.Join(chromeRoot, entry.Name())
		absCoreDir := filepath.Join(baseDir, entry.Name())
		if _, _, ok := browser.FindCoreExecutable(absCoreDir); !ok {
			continue // 没有浏览器可执行文件，跳过
		}
		cores = append(cores, browser.Core{
			CoreId:   fmt.Sprintf("core-%s", entry.Name()),
			CoreName: fmt.Sprintf("Chrome %s", entry.Name()),
			CorePath: subPath,
		})
		log.Debug("发现内核", logger.F("name", entry.Name()), logger.F("path", subPath))
	}
	// 默认内核取"最高版本"（如 148 优先于 144），而非字典序首个，
	// 避免多内核并存时把较老版本误选为默认。
	sort.SliceStable(cores, func(i, j int) bool {
		return coreDirVersionRank(cores[i].CorePath) > coreDirVersionRank(cores[j].CorePath)
	})
	for i := range cores {
		cores[i].IsDefault = i == 0
	}
	return cores
}

// coreDirVersionRank 从内核目录名解析可比较的版本数值（用于"最高版本为默认"排序）。
// 目录名形如 "fingerprint-chromium-148"、"148" 或 "148.0.7778.215":
// 取末尾数字版本段(fingerprint-chromium-148 → 148),非数字段视为 0。
var coreDirVersionRe = regexp.MustCompile(`(\d+)(\.\d+)*\s*$`)

func coreDirVersionRank(corePath string) int64 {
	name := filepath.Base(strings.TrimSpace(corePath))
	m := coreDirVersionRe.FindString(name)
	if m == "" {
		return 0
	}
	parts := strings.Split(strings.TrimSpace(m), ".")
	var rank int64
	for k := 0; k < 4; k++ {
		rank *= 100000
		if k < len(parts) {
			if n, err := strconv.Atoi(strings.TrimSpace(parts[k])); err == nil && n > 0 {
				rank += int64(n)
			}
		}
	}
	return rank
}

// ============================================================================
// 代理数据加载
// ============================================================================

// loadProxies 启动时加载代理数据。
// 优先从 ProxyDAO（SQLite）读取；若 DAO 未注入则降级到 proxies.yaml，最后降级到 config.yaml。
func (a *App) loadProxies() {
	log := logger.New("Browser")

	builtins := []browser.Proxy{
		{ProxyId: "__direct__", ProxyName: "直连（不走代理）", ProxyConfig: "direct://"},
	}

	ensureBuiltins := func(list []browser.Proxy) []browser.Proxy {
		for _, b := range builtins {
			found := false
			for _, p := range list {
				if p.ProxyId == b.ProxyId {
					found = true
					break
				}
			}
			if !found {
				list = append([]browser.Proxy{b}, list...)
			}
		}
		return list
	}

	// 优先从 SQLite 读取
	if a.browserMgr.ProxyDAO != nil {
		list, err := a.browserMgr.ProxyDAO.List()
		if err != nil {
			log.Error("从数据库读取代理失败", logger.F("error", err.Error()))
		} else if len(list) > 0 {
			a.config.Browser.Proxies = list
			log.Info("代理数据从数据库加载完成", logger.F("count", len(list)))
			return
		}
	}

	// 降级：从 proxies.yaml 加载
	loaded, err := config.LoadProxies(a.resolveAppPath("proxies.yaml"))
	if err != nil {
		log.Warn("读取 proxies.yaml 失败", logger.F("error", err.Error()))
	}
	if loaded != nil {
		proxies := ensureBuiltins(loaded)
		a.config.Browser.Proxies = proxies
		log.Info("代理数据从 proxies.yaml 加载完成", logger.F("count", len(proxies)))
		return
	}

	// 最终降级：使用 config.yaml 中的数据
	proxies := ensureBuiltins(a.config.Browser.Proxies)
	a.config.Browser.Proxies = proxies
	log.Info("代理数据使用 config.yaml 默认值", logger.F("count", len(proxies)))
}
