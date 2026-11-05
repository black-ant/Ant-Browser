package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"strings"
)

type BrowserBookmark = config.BrowserBookmark

type BookmarkSyncResult struct {
	Total       int      `json:"total"`
	Synced      int      `json:"synced"`
	Skipped     int      `json:"skipped"`
	Failed      int      `json:"failed"`
	SkippedList []string `json:"skippedList"`
	FailedList  []string `json:"failedList"`
}

var defaultBookmarkList = []BrowserBookmark{
	{Name: "指纹检测", URL: fingerprintCheckBookmarkURL},
	{Name: "Google", URL: "https://www.google.com/"},
	{Name: "Gmail", URL: "https://mail.google.com/"},
	{Name: "Claude", URL: "https://claude.ai/"},
	{Name: "ChatGPT", URL: "https://chatgpt.com/"},
	{Name: "YouTube", URL: "https://www.youtube.com/"},
	{Name: "IPPure", URL: "https://ippure.com/"},
	{Name: "IPLark", URL: "https://iplark.com/"},
	{Name: "Ping0", URL: "https://ping0.cc/"},
}

var protectedBookmarkList = []BrowserBookmark{
	{Name: "指纹检测", URL: fingerprintCheckBookmarkURL},
}

// BookmarkList 获取默认书签列表（优先 SQLite，降级 config.yaml）
func (a *App) BookmarkList() []BrowserBookmark {
	if a.browserMgr.BookmarkDAO != nil {
		list, err := a.browserMgr.BookmarkDAO.List()
		if err == nil && len(list) > 0 {
			return normalizeBookmarkList(list)
		}
	}
	if len(a.config.Browser.DefaultBookmarks) > 0 {
		return normalizeBookmarkList(a.config.Browser.DefaultBookmarks)
	}
	return normalizeBookmarkList(defaultBookmarkList)
}

// BookmarkSave 保存默认书签列表（优先 SQLite，降级 config.yaml）
func (a *App) BookmarkSave(items []BrowserBookmark) error {
	log := logger.New("Bookmark")
	valid := make([]BrowserBookmark, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		url := strings.TrimSpace(item.URL)
		if name != "" && url != "" {
			valid = append(valid, BrowserBookmark{Name: name, URL: url, OpenOnStart: item.OpenOnStart})
		}
	}
	valid = normalizeBookmarkList(valid)

	if a.browserMgr.BookmarkDAO != nil {
		if err := a.browserMgr.BookmarkDAO.ReplaceAll(valid); err != nil {
			log.Error("书签保存到数据库失败", logger.F("error", err.Error()))
			return err
		}
		if deletionDAO, ok := a.browserMgr.BookmarkDAO.(browser.BookmarkDeletionDAO); ok {
			deletedURLs, err := deletionDAO.DeletedURLs()
			if err != nil {
				log.Error("读取书签删除标记失败", logger.F("error", err.Error()))
				return err
			}
			a.config.Browser.DeletedBookmarkURLs = deletedURLs
		}
		a.config.Browser.DefaultBookmarks = valid
		log.Info("书签已保存到数据库", logger.F("count", len(valid)))
		return nil
	}

	// 降级：写入 config.yaml
	previous := a.BookmarkList()
	a.config.Browser.DeletedBookmarkURLs = updateDeletedBookmarkURLs(a.config.Browser.DeletedBookmarkURLs, previous, valid)
	a.config.Browser.DefaultBookmarks = valid
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		log.Error("书签保存失败", logger.F("error", err.Error()))
		return err
	}
	log.Info("书签已保存到 config.yaml", logger.F("count", len(valid)))
	return nil
}

func updateDeletedBookmarkURLs(existing []string, previous, current []BrowserBookmark) []string {
	return filterBookmarkDeletionsByActiveURLs(
		mergeBookmarkDeletionURLs(existing, bookmarkURLs(previous)),
		activeBookmarkURLSet(current),
	)
}

func bookmarkURLs(bookmarks []BrowserBookmark) []string {
	urls := make([]string, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		urls = append(urls, bookmark.URL)
	}
	return urls
}

// BookmarkReset 恢复默认书签
func (a *App) BookmarkReset() error {
	return a.BookmarkSave(append([]BrowserBookmark{}, defaultBookmarkList...))
}

func mergeBookmarksByURL(items []BrowserBookmark, required []BrowserBookmark) []BrowserBookmark {
	merged := make([]BrowserBookmark, 0, len(items)+len(required))
	seen := make(map[string]struct{}, len(items)+len(required))
	appendOne := func(item BrowserBookmark) {
		name := strings.TrimSpace(item.Name)
		url := strings.TrimSpace(item.URL)
		if name == "" || url == "" {
			return
		}
		key := bookmarkURLKey(url)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		merged = append(merged, BrowserBookmark{Name: name, URL: url, OpenOnStart: item.OpenOnStart})
	}
	for _, item := range items {
		appendOne(item)
	}
	for _, item := range required {
		appendOne(item)
	}
	return merged
}

func normalizeBookmarkList(items []BrowserBookmark) []BrowserBookmark {
	protectedOpenOnStart := map[string]bool{}
	for _, item := range items {
		if isProtectedBookmarkURL(item.URL) {
			protectedOpenOnStart[fingerprintCheckBookmarkURL] = item.OpenOnStart
		}
	}
	protected := make([]BrowserBookmark, 0, len(protectedBookmarkList))
	for _, item := range protectedBookmarkList {
		item.OpenOnStart = protectedOpenOnStart[item.URL]
		protected = append(protected, item)
	}
	mergedRegular := mergeBookmarksByURL(items, nil)
	out := append([]BrowserBookmark{}, protected...)
	seen := map[string]struct{}{}
	for _, item := range out {
		seen[bookmarkURLKey(item.URL)] = struct{}{}
	}
	for _, item := range mergedRegular {
		key := bookmarkURLKey(item.URL)
		if key == "" || isProtectedBookmarkURL(key) {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

// BookmarkSyncToProfiles 将当前默认书签增量同步到已有未运行实例。
func (a *App) BookmarkSyncToProfiles() BookmarkSyncResult {
	result := BookmarkSyncResult{}
	log := logger.New("Bookmark")
	bookmarks := a.BookmarkList()
	if len(bookmarks) == 0 || a.browserMgr == nil {
		return result
	}

	a.browserMgr.InitData()
	type bookmarkSyncTarget struct {
		profile *BrowserProfile
		live    bool
	}
	targets := make([]bookmarkSyncTarget, 0, len(a.browserMgr.Profiles))
	a.browserMgr.Mutex.Lock()
	for _, profile := range a.browserMgr.Profiles {
		if profile == nil {
			continue
		}
		targets = append(targets, bookmarkSyncTarget{
			profile: profile,
			live:    isBrowserProfileLive(profile, a.browserMgr.BrowserProcesses[profile.ProfileId]),
		})
	}
	a.browserMgr.Mutex.Unlock()

	result.Total = len(targets)
	for _, target := range targets {
		profile := target.profile
		if target.live {
			result.Skipped++
			result.SkippedList = append(result.SkippedList, profile.ProfileName)
			continue
		}

		userDataDir := a.browserMgr.ResolveUserDataDir(profile)
		expectedArgs := a.fingerprintCheckExpectedArgsFromProfile(profile)
		runtimeBookmarks, fingerprintBookmarkURL, err := a.runtimeBookmarksForProfileExpectedArgsAndProfile(profile.ProfileId, expectedArgs, profile, bookmarks)
		if err != nil {
			result.Failed++
			name := profile.ProfileName
			if name == "" {
				name = profile.ProfileId
			}
			result.FailedList = append(result.FailedList, name)
			log.Error("生成实例默认书签失败", logger.F("profile_id", profile.ProfileId), logger.F("error", err.Error()))
			continue
		}
		if fingerprintBookmarkURL != "" {
			if _, err := browser.ReplaceBookmarkURL(userDataDir, fingerprintCheckBookmarkURL, fingerprintBookmarkURL); err != nil {
				result.Failed++
				name := profile.ProfileName
				if name == "" {
					name = profile.ProfileId
				}
				result.FailedList = append(result.FailedList, name)
				log.Error("更新旧指纹检测书签失败", logger.F("profile_id", profile.ProfileId), logger.F("error", err.Error()))
				continue
			}
		}
		if err := browser.EnsureDefaultBookmarks(userDataDir, runtimeBookmarks); err != nil {
			result.Failed++
			name := profile.ProfileName
			if name == "" {
				name = profile.ProfileId
			}
			result.FailedList = append(result.FailedList, name)
			log.Error("同步默认书签到实例失败", logger.F("profile_id", profile.ProfileId), logger.F("error", err.Error()))
			continue
		}
		result.Synced++
	}
	return result
}
