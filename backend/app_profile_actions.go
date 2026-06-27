package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type profileStartupActions struct {
	HasExplicitConfig            bool
	ImportCookies                string
	ApplyDefaultBookmarks        bool
	ClearCacheBeforeStart        bool
	ClearCookiesBeforeStart      bool
	ClearLocalStorageBeforeStart bool
	RandomFingerprintOnStart     bool
	// 启动门控开关（启动前依据出口 IP 判定是否放行）
	KeepNetworkOn        bool
	StopOnIpChange       bool
	StopOnIpRegionChange bool
	// Baseline 上次成功放行时记录的出口 IP / 国家，用于 IP / 地区变化比对
	Baseline profileRuntimeState
	// 网址访问黑 / 白名单（每行一个 URL 或域名片段）
	WebsiteAccessBlacklist string
	WebsiteAccessWhitelist string
	// IPChangeReminder 开启后：检测到出口 IP / 国家变化但未被门控拦截时，发事件提醒（不阻断启动）
	IPChangeReminder bool
}

type profileFormStateActionsConfig struct {
	RandomFingerprintOnStart bool   `json:"randomFingerprintOnStart"`
	KeepNetworkOn            bool   `json:"keepNetworkOn"`
	StopOnIpChange           bool   `json:"stopOnIpChange"`
	StopOnIpRegionChange     bool   `json:"stopOnIpRegionChange"`
	WebsiteAccessBlacklist   string `json:"websiteAccessBlacklist"`
	WebsiteAccessWhitelist   string `json:"websiteAccessWhitelist"`
	IPChangeReminder         string `json:"ipChangeReminder"`
}

// profileRuntimeState 保存窗口运行相关的可变状态（与用户配置区分），
// 持久化在 profileConfig 顶层的 "runtimeState" 键下。
type profileRuntimeState struct {
	LastIP      string `json:"lastIp"`
	LastCountry string `json:"lastCountry"`
}

type profilePostCreateActionsConfig struct {
	ImportCookies                string `json:"importCookies"`
	ApplyDefaultBookmarks        bool   `json:"applyDefaultBookmarks"`
	ClearCacheBeforeStart        bool   `json:"clearCacheBeforeStart"`
	ClearCookiesBeforeStart      bool   `json:"clearCookiesBeforeStart"`
	ClearLocalStorageBeforeStart bool   `json:"clearLocalStorageBeforeStart"`
}

func parseProfileStartupActions(raw string) profileStartupActions {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" {
		return profileStartupActions{}
	}

	var cfg struct {
		Version           int                             `json:"version"`
		FormState         json.RawMessage                 `json:"formState"`
		PostCreateActions *profilePostCreateActionsConfig `json:"postCreateActions"`
		RuntimeState      *profileRuntimeState            `json:"runtimeState"`
	}
	if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
		return profileStartupActions{}
	}

	actions := profileStartupActions{
		HasExplicitConfig: cfg.Version != 0 || len(cfg.FormState) > 0 || cfg.PostCreateActions != nil,
	}
	if cfg.RuntimeState != nil {
		actions.Baseline = *cfg.RuntimeState
	}
	if len(cfg.FormState) > 0 && strings.TrimSpace(string(cfg.FormState)) != "null" {
		var formState profileFormStateActionsConfig
		if err := json.Unmarshal(cfg.FormState, &formState); err == nil {
			actions.RandomFingerprintOnStart = formState.RandomFingerprintOnStart
			actions.KeepNetworkOn = formState.KeepNetworkOn
			actions.StopOnIpChange = formState.StopOnIpChange
			actions.StopOnIpRegionChange = formState.StopOnIpRegionChange
			actions.WebsiteAccessBlacklist = strings.TrimSpace(formState.WebsiteAccessBlacklist)
			actions.WebsiteAccessWhitelist = strings.TrimSpace(formState.WebsiteAccessWhitelist)
			actions.IPChangeReminder = strings.EqualFold(strings.TrimSpace(formState.IPChangeReminder), "on")
		}
	}
	if cfg.PostCreateActions == nil {
		return actions
	}
	actions.ApplyDefaultBookmarks = cfg.PostCreateActions.ApplyDefaultBookmarks
	actions.ImportCookies = strings.TrimSpace(cfg.PostCreateActions.ImportCookies)
	actions.ClearCacheBeforeStart = cfg.PostCreateActions.ClearCacheBeforeStart
	actions.ClearCookiesBeforeStart = cfg.PostCreateActions.ClearCookiesBeforeStart
	actions.ClearLocalStorageBeforeStart = cfg.PostCreateActions.ClearLocalStorageBeforeStart
	return actions
}

func shouldApplyDefaultBookmarks(actions profileStartupActions) bool {
	return !actions.HasExplicitConfig || actions.ApplyDefaultBookmarks
}

func newRandomFingerprintSeed() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

func withFingerprintSeed(args []string, seed string) []string {
	seed = strings.TrimSpace(seed)
	if seed == "" {
		return append([]string{}, args...)
	}
	nextArg := "--fingerprint=" + seed
	out := make([]string, 0, len(args)+1)
	replaced := false
	for _, raw := range args {
		arg := strings.TrimSpace(raw)
		if arg == "" {
			continue
		}
		if strings.HasPrefix(arg, "--fingerprint=") {
			if !replaced {
				out = append(out, nextArg)
				replaced = true
			}
			continue
		}
		out = append(out, arg)
	}
	if !replaced {
		out = append(out, nextArg)
	}
	return out
}

func profilePreStartCleanupRelativePaths(actions profileStartupActions) []string {
	paths := make([]string, 0, 12)
	if actions.ClearCacheBeforeStart {
		paths = append(paths,
			"Default/Cache",
			"Default/Code Cache",
			"Default/GPUCache",
			"Default/Network/Cache",
			"Default/Service Worker/CacheStorage",
			"Default/Service Worker/ScriptCache",
		)
	}
	if actions.ClearCookiesBeforeStart {
		paths = append(paths,
			"Default/Network/Cookies",
			"Default/Network/Cookies-journal",
			"Default/Cookies",
			"Default/Cookies-journal",
		)
	}
	if actions.ClearLocalStorageBeforeStart {
		paths = append(paths, "Default/Local Storage")
	}
	return paths
}

func applyProfilePreStartActions(userDataDir string, actions profileStartupActions) error {
	var errs []error
	for _, rel := range profilePreStartCleanupRelativePaths(actions) {
		if err := removeChromeProfilePath(userDataDir, rel); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", rel, err))
		}
	}
	return errors.Join(errs...)
}

func removeChromeProfilePath(userDataDir string, rel string) error {
	target, err := safeChromeProfilePath(userDataDir, rel)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(target); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.RemoveAll(target)
}

func safeChromeProfilePath(userDataDir string, rel string) (string, error) {
	root, err := filepath.Abs(userDataDir)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	inside, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if inside == "." || inside == ".." || filepath.IsAbs(inside) || strings.HasPrefix(inside, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("refusing to clean path outside profile data dir: %s", rel)
	}
	return target, nil
}

func (a *App) applyProfilePostStartActions(profileId string, actions profileStartupActions, launchArgs []string) error {
	if strings.TrimSpace(actions.ImportCookies) == "" {
		return nil
	}

	cookies, err := parseCookieImport(actions.ImportCookies, firstLaunchURL(launchArgs))
	if err != nil {
		return err
	}
	if len(cookies) == 0 {
		return nil
	}
	if err := a.BrowserSetCookies(profileId, cookies); err != nil {
		return fmt.Errorf("import cookies through CDP: %w", err)
	}
	if err := a.clearConsumedProfileImportCookies(profileId); err != nil {
		return fmt.Errorf("clear imported cookie action: %w", err)
	}
	return nil
}

func parseCookieImport(raw string, fallbackURL string) ([]CookieInfo, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
		if cookies, ok, err := parseJSONCookieImport(trimmed, fallbackURL); ok || err != nil {
			return cookies, err
		}
	}

	var cookies []CookieInfo
	for lineNo, rawLine := range strings.Split(trimmed, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if fields := strings.Fields(line); len(fields) >= 7 {
			expires, _ := strconv.ParseFloat(fields[4], 64)
			cookies = append(cookies, normalizeImportCookie(CookieInfo{
				Domain:  fields[0],
				Path:    fields[2],
				Secure:  strings.EqualFold(fields[3], "TRUE"),
				Expires: expires,
				Name:    fields[5],
				Value:   strings.Join(fields[6:], " "),
			}, fallbackURL))
			continue
		}
		if name, value, ok := strings.Cut(line, "="); ok && strings.TrimSpace(name) != "" {
			if strings.TrimSpace(fallbackURL) == "" {
				return nil, fmt.Errorf("cookie line %d has no domain; add a startup URL or use JSON/Netscape cookie format", lineNo+1)
			}
			cookies = append(cookies, normalizeImportCookie(CookieInfo{
				Name:  strings.TrimSpace(name),
				Value: strings.TrimSpace(value),
				URL:   fallbackURL,
				Path:  "/",
			}, fallbackURL))
			continue
		}
		return nil, fmt.Errorf("unsupported cookie line %d", lineNo+1)
	}
	return filterValidImportCookies(cookies), nil
}

func parseJSONCookieImport(raw string, fallbackURL string) ([]CookieInfo, bool, error) {
	var cookies []CookieInfo
	if err := json.Unmarshal([]byte(raw), &cookies); err == nil {
		return normalizeImportCookies(cookies, fallbackURL), true, nil
	}

	var wrapper struct {
		Cookies []CookieInfo `json:"cookies"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err == nil && wrapper.Cookies != nil {
		return normalizeImportCookies(wrapper.Cookies, fallbackURL), true, nil
	}

	var one CookieInfo
	if err := json.Unmarshal([]byte(raw), &one); err == nil && strings.TrimSpace(one.Name) != "" {
		return normalizeImportCookies([]CookieInfo{one}, fallbackURL), true, nil
	}
	return nil, true, fmt.Errorf("cookie JSON must be an array, a single cookie object, or {\"cookies\": [...]}")
}

func normalizeImportCookies(cookies []CookieInfo, fallbackURL string) []CookieInfo {
	out := make([]CookieInfo, 0, len(cookies))
	for _, cookie := range cookies {
		out = append(out, normalizeImportCookie(cookie, fallbackURL))
	}
	return filterValidImportCookies(out)
}

func normalizeImportCookie(cookie CookieInfo, fallbackURL string) CookieInfo {
	cookie.Name = strings.TrimSpace(cookie.Name)
	cookie.Domain = strings.TrimSpace(cookie.Domain)
	cookie.URL = strings.TrimSpace(cookie.URL)
	if cookie.Path == "" {
		cookie.Path = "/"
	}
	if cookie.Domain == "" && cookie.URL == "" {
		cookie.URL = fallbackURL
	}
	return cookie
}

func filterValidImportCookies(cookies []CookieInfo) []CookieInfo {
	out := make([]CookieInfo, 0, len(cookies))
	for _, cookie := range cookies {
		if strings.TrimSpace(cookie.Name) == "" {
			continue
		}
		if strings.TrimSpace(cookie.Domain) == "" && strings.TrimSpace(cookie.URL) == "" {
			continue
		}
		out = append(out, cookie)
	}
	return out
}

func firstLaunchURL(args []string) string {
	for _, arg := range args {
		value := strings.TrimSpace(arg)
		if value == "" || strings.HasPrefix(value, "--") {
			continue
		}
		parsed, err := url.Parse(value)
		if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
			return parsed.String()
		}
	}
	return ""
}

func clearProfileConfigImportCookies(raw string) (string, bool, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "{}" {
		return "{}", false, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return raw, false, err
	}
	post, ok := cfg["postCreateActions"].(map[string]any)
	if !ok {
		return raw, false, nil
	}
	value, ok := post["importCookies"]
	if !ok || strings.TrimSpace(fmt.Sprint(value)) == "" {
		return raw, false, nil
	}
	delete(post, "importCookies")
	out, err := json.Marshal(cfg)
	if err != nil {
		return raw, false, err
	}
	return string(out), true, nil
}

func (a *App) clearConsumedProfileImportCookies(profileId string) error {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		return fmt.Errorf("profile not found")
	}
	nextConfig, changed, err := clearProfileConfigImportCookies(profile.ProfileConfig)
	if err != nil || !changed {
		return err
	}
	profile.ProfileConfig = nextConfig
	profile.UpdatedAt = time.Now().Format(time.RFC3339)
	if a.browserMgr.ProfileDAO != nil {
		return a.browserMgr.ProfileDAO.Upsert(profile)
	}
	return a.browserMgr.SaveProfiles()
}
