package backend

// 使用用户真实数据库副本 + 真实虚拟分组 + 真实内核做端到端验证。
// 若本机没有真实库或内核则自动跳过（CI 安全）。

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"ant-chrome/backend/internal/proxy"
	xproxy "golang.org/x/net/proxy"
)

const realUserDBPath = `E:\software\Ant Browser\data\app.db`

func newRealDataApp(t *testing.T, dbPath string) *App {
	t.Helper()
	dir := t.TempDir()
	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// 副本库可能来自旧版本，先对齐到当前 schema 再测。
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate copy failed: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Browser.XrayBinaryPath = mustAbsIfExists(t, filepath.Join("..", "bin", "xray.exe"))
	cfg.Browser.SingBoxBinaryPath = mustAbsIfExists(t, filepath.Join("..", "bin", "sing-box.exe"))
	if mieru := os.Getenv("MIERU_BINARY_PATH"); mieru != "" {
		cfg.Browser.MieruBinaryPath = mieru
	} else if _, err := os.Stat(`D:\tmp-mieru\mieru.exe`); err == nil {
		cfg.Browser.MieruBinaryPath = `D:\tmp-mieru\mieru.exe`
	}

	app := NewApp(dir)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, app.appRoot)
	app.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(db.GetConn())
	app.browserMgr.ProxyVirtualGroupDAO = browser.NewSQLiteProxyVirtualGroupDAO(db.GetConn())
	app.browserMgr.ProxyVirtualGroupMemberStateDAO = browser.NewSQLiteProxyVirtualGroupMemberStateDAO(db.GetConn())
	app.xrayMgr = proxy.NewXrayManager(cfg, app.appRoot)
	app.singboxMgr = proxy.NewSingBoxManager(cfg, app.appRoot)
	app.mieruMgr = proxy.NewMieruManager(cfg, app.appRoot)
	t.Cleanup(func() {
		app.xrayMgr.StopAll()
		app.singboxMgr.StopAll()
		app.mieruMgr.Stop()
	})
	return app
}

func mustAbsIfExists(t *testing.T, rel string) string {
	t.Helper()
	abs, err := filepath.Abs(rel)
	if err != nil {
		return ""
	}
	if _, err := os.Stat(abs); err != nil {
		return ""
	}
	return abs
}

func buildClientForProxyURL(t *testing.T, proxyURL string, timeout time.Duration) (*http.Client, error) {
	t.Helper()
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "socks5" {
		dialer, err := xproxy.SOCKS5("tcp", parsed.Host, nil, xproxy.Direct)
		if err != nil {
			return nil, err
		}
		return &http.Client{Timeout: timeout, Transport: &http.Transport{DialContext: dialer.(xproxy.ContextDialer).DialContext}}, nil
	}
	return &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: http.ProxyURL(parsed)}}, nil
}

func requestThroughProxyURL(t *testing.T, proxyURL string) error {
	t.Helper()
	client, err := buildClientForProxyURL(t, proxyURL, 15*time.Second)
	if err != nil {
		return err
	}
	var lastErr error
	for _, target := range []string{"https://www.baidu.com/", "http://www.gstatic.com/generate_204"} {
		resp, reqErr := client.Get(target)
		if reqErr != nil {
			lastErr = reqErr
			continue
		}
		status := resp.Status
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			t.Logf("真实请求 %s -> %s", target, status)
			return nil
		}
		lastErr = fmtStatusError(status)
	}
	return lastErr
}

type statusError string

func (e statusError) Error() string      { return string(e) }
func fmtStatusError(status string) error { return statusError("status " + status) }

// 用真实库副本验证：创建实例绑定分组 → 连续两次启动 → 真实 HTTP 请求 → 目标解析 → 保存校验。
func TestVirtualGroupRealUserDataEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	if _, err := os.Stat(realUserDBPath); err != nil {
		t.Skipf("本机无真实用户库（%s），跳过", realUserDBPath)
	}
	copyPath := filepath.Join(t.TempDir(), "app.db")
	copyDBFile(t, realUserDBPath, copyPath)

	app := newRealDataApp(t, copyPath)
	groups, err := app.browserMgr.ProxyVirtualGroupDAO.List()
	if err != nil || len(groups) == 0 {
		t.Skipf("用户库无虚拟分组: %v", err)
	}
	target := groups[0]
	t.Logf("测试分组: %s strategy=%s members=%v", target.GroupName, target.Strategy, target.MemberProxyIds)
	if len(target.MemberProxyIds) == 0 {
		t.Skip("分组无成员")
	}

	// 1) 创建实例并绑定分组（与 UI 相同的 Manager 入口）
	profile, err := app.BrowserProfileCreate(BrowserProfileInput{
		ProfileName:     "real-data-vg-profile",
		UserDataDir:     "",
		ProxyId:         strings.TrimSpace(target.GroupId),
		ProxyConfig:     "",
		LaunchArgs:      []string{},
		FingerprintArgs: []string{},
	})
	if err != nil {
		t.Fatalf("创建实例绑定虚拟分组失败: %v", err)
	}
	if profile.ProxyId != target.GroupId || profile.ProxyMode != "fixed" {
		t.Fatalf("绑定结果异常: id=%s mode=%s", profile.ProxyId, profile.ProxyMode)
	}
	t.Logf("[1] 绑定 OK: proxy_id=%s mode=%s", profile.ProxyId, profile.ProxyMode)

	// 2) 连续启动两次：验证策略选择 + 真实代理可用
	seen := make([]string, 0, 2)
	for round := 1; round <= 2; round++ {
		proxyURL, ref, usingBridge, err := app.resolveBrowserStartProxy(browserStartInput{ProfileID: profile.ProfileId}, profile)
		if err != nil {
			t.Fatalf("第 %d 次启动失败: %v", round, err)
		}
		bridgeKey := ref.Key
		if len(bridgeKey) > 8 {
			bridgeKey = bridgeKey[:8]
		}
		t.Logf("[2.%d] 启动 OK: proxy=%s bridge=%v engine=%s bridgeKey=%s", round, proxyURL, usingBridge, ref.Engine, bridgeKey)
		if err := requestThroughProxyURL(t, proxyURL); err != nil {
			t.Errorf("第 %d 次真实代理请求失败: %v", round, err)
		}
		seen = append(seen, proxyURL)
		app.releaseProxyBridgeRef(ref)
		time.Sleep(300 * time.Millisecond)
	}
	if len(seen) == 2 && seen[0] != seen[1] {
		t.Logf("[3] 轮询生效: 成员1=%s 成员2=%s", seen[0], seen[1])
	} else {
		t.Logf("[3] 两次均使用 %s（round_robin 下可能只剩一个健康成员）", seen[0])
	}

	// 3) 测速/IP健康/预热目标解析
	memberID, memberConfig, ok := app.resolveVirtualGroupProxyTarget(profile.ProxyId, app.getLatestProxies())
	if !ok || memberID == "" || memberConfig == "" {
		t.Fatalf("目标解析失败: id=%q config=%q ok=%v", memberID, memberConfig, ok)
	}
	t.Logf("[4] 目标解析 OK: member=%s", memberID)

	// 4) 保存校验
	validation := app.ValidateProxyConfig("", profile.ProxyId)
	if !validation.Supported {
		t.Fatalf("[5] 保存校验未通过: %s", validation.ErrorMsg)
	}
	t.Logf("[5] 保存校验 OK")

	// 5) 再次编辑识别（模拟重新打开编辑页）
	reResolved := resolveVirtualGroupProfileBindingForTest(app, profile)
	if reResolved != target.GroupId {
		t.Fatalf("[6] 重新编辑识别异常: %s, want %s", reResolved, target.GroupId)
	}
	t.Logf("[6] 重新编辑保持绑定 OK")
}

// copyDBFile 用 SQLite backup 复制库（保证 WAL 内容一致）。
func copyDBFile(t *testing.T, src, dst string) {
	t.Helper()
	srcDB, err := database.NewDB(src)
	if err != nil {
		t.Fatalf("open src db failed: %v", err)
	}
	defer srcDB.Close()
	if _, err := srcDB.GetConn().Exec(`VACUUM INTO ?`, dst); err != nil {
		t.Fatalf("copy db failed: %v", err)
	}
}

// resolveVirtualGroupProfileBindingForTest 模拟编辑器打开时对已存 proxyId 的识别。
func resolveVirtualGroupProfileBindingForTest(app *App, profile *BrowserProfile) string {
	groups, _ := app.browserMgr.ProxyVirtualGroupDAO.List()
	for _, group := range groups {
		if strings.EqualFold(strings.TrimSpace(group.GroupId), strings.TrimSpace(profile.ProxyId)) {
			return strings.TrimSpace(group.GroupId)
		}
	}
	return ""
}
