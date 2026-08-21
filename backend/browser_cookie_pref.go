package backend

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

// ensureLaunchPreferences 在启动 Chrome 前调整实例 Preferences,去掉打扰性提示并放开 Cookie:
//   - profile.cookie_controls_mode = 0 : 允许所有 Cookie(含第三方),去掉"第三方 Cookie 被拦"提示
//     (0=允许全部,1=无痕下拦第三方,3=拦全部第三方;实测启动 flag 无效,必须落到该 pref)。
//   - profile.exit_type = "Normal" / exited_cleanly = true : 去掉"Chrome 未正常关闭/恢复上次页面"气泡。
//   - browser.check_default_browser = false : 去掉"设为默认浏览器"提示(与 --no-default-browser-check 双保险)。
//   - 删除 browser.window_placement / app_window_placement : 否则 Chrome 会用上次保存的(常为最大化)
//     窗口状态覆盖命令行 --window-size,导致窗口开成真机工作区大小、指纹检测 Window Size 对不上。
//
// livePerf=true 时额外关闭一批纯后台的浏览器服务(见函数末尾)。
// 仅在实例未运行(启动前)调用,读改写合并,用 UseNumber 保留数字类型,避免破坏 Preferences 其他字段。
func ensureLaunchPreferences(userDataDir string, livePerf bool) {
	if userDataDir == "" {
		return
	}
	prefsPath := filepath.Join(userDataDir, "Default", "Preferences")
	if err := os.MkdirAll(filepath.Dir(prefsPath), 0o755); err != nil {
		return
	}
	prefs := map[string]interface{}{}
	if data, err := os.ReadFile(prefsPath); err == nil && len(data) > 0 {
		// 必须用 UseNumber 保留数字原始类型:否则整数会被解成 float64,
		// 重新序列化时可能变成 1e+09 之类,导致 Chromium 解析 Preferences 失败、启动即退出。
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if dec.Decode(&prefs) != nil {
			prefs = map[string]interface{}{} // 解析失败则重建最小集
		}
	}
	profile, ok := prefs["profile"].(map[string]interface{})
	if !ok {
		profile = map[string]interface{}{}
	}
	profile["cookie_controls_mode"] = 0
	profile["exit_type"] = "Normal"
	profile["exited_cleanly"] = true
	prefs["profile"] = profile

	browser, ok := prefs["browser"].(map[string]interface{})
	if !ok {
		browser = map[string]interface{}{}
	}
	browser["check_default_browser"] = false
	// 删除持久化窗口位置/尺寸:让每次启动都回退到命令行 --window-size(否则上次最大化状态会覆盖它)。
	// 非指纹参数,删除安全;修复"指纹检测 Window Size 与身份不一致"。
	delete(browser, "window_placement")
	delete(browser, "app_window_placement")
	prefs["browser"] = browser

	if livePerf {
		// 直播性能模式:关掉纯后台的浏览器服务。这些都在**浏览器面**,页面 JS 观测不到。
		//
		// 红线:绝不写 default_content_setting_values —— 权限状态(Notification.permission 等)
		// JS 可读,属指纹面。这里只碰"服务开关",不碰"权限状态"。
		prefs["safebrowsing"] = mergeSubMap(prefs, "safebrowsing", map[string]interface{}{
			"enabled": false, // 关 SafeBrowsing:去掉持续的名单更新与上报网络
		})
		prefs["net"] = mergeSubMap(prefs, "net", map[string]interface{}{
			"network_prediction_options": 2, // 2 = 永不预取 / 预连接
		})
		prefs["search"] = mergeSubMap(prefs, "search", map[string]interface{}{
			"suggest_enabled": false, // 关搜索建议(每次输入都会发请求)
		})
		prefs["translate"] = mergeSubMap(prefs, "translate", map[string]interface{}{
			"enabled": false,
		})
		prefs["spellcheck"] = mergeSubMap(prefs, "spellcheck", map[string]interface{}{
			"use_spelling_service": false,
		})
		prefs["alternate_error_pages"] = mergeSubMap(prefs, "alternate_error_pages", map[string]interface{}{
			"enabled": false,
		})
		prefs["credentials_enable_service"] = false // 关密码保存气泡
		profile["password_manager_leak_detection"] = false
		prefs["profile"] = profile
	}

	if out, err := json.Marshal(prefs); err == nil {
		_ = os.WriteFile(prefsPath, out, 0o644)
	}
}

// mergeSubMap 取 prefs[key] 的既有子对象(无则新建),覆盖写入 kv 后返回。
// 用于只改指定字段而不丢掉 Preferences 里同段的其他内容。
func mergeSubMap(prefs map[string]interface{}, key string, kv map[string]interface{}) map[string]interface{} {
	sub, ok := prefs[key].(map[string]interface{})
	if !ok {
		sub = map[string]interface{}{}
	}
	for k, v := range kv {
		sub[k] = v
	}
	return sub
}
