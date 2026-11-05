package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
)

func bookmarkURLKey(rawURL string) string {
	return browser.NormalizeBookmarkURL(rawURL)
}

func isProtectedBookmarkURL(rawURL string) bool {
	return bookmarkURLKey(rawURL) == bookmarkURLKey(fingerprintCheckBookmarkURL)
}

func mergeBookmarkDeletionURLs(urlLists ...[]string) []string {
	seen := make(map[string]struct{})
	merged := make([]string, 0)
	for _, urls := range urlLists {
		for _, rawURL := range urls {
			bookmarkURL := bookmarkURLKey(rawURL)
			if bookmarkURL == "" || isProtectedBookmarkURL(bookmarkURL) {
				continue
			}
			if _, ok := seen[bookmarkURL]; ok {
				continue
			}
			seen[bookmarkURL] = struct{}{}
			merged = append(merged, bookmarkURL)
		}
	}
	return merged
}

func activeBookmarkURLSet(bookmarks []config.BrowserBookmark) map[string]struct{} {
	active := make(map[string]struct{}, len(bookmarks))
	for _, bookmark := range bookmarks {
		if bookmarkURL := bookmarkURLKey(bookmark.URL); bookmarkURL != "" {
			active[bookmarkURL] = struct{}{}
		}
	}
	return active
}

func filterBookmarkDeletionsByActiveURLs(deletedURLs []string, activeURLs map[string]struct{}) []string {
	filtered := make([]string, 0, len(deletedURLs))
	for _, bookmarkURL := range mergeBookmarkDeletionURLs(deletedURLs) {
		if _, active := activeURLs[bookmarkURL]; active {
			continue
		}
		filtered = append(filtered, bookmarkURL)
	}
	return filtered
}

func filterBookmarksByDeletedURLs(bookmarks []config.BrowserBookmark, deletedURLs []string) []config.BrowserBookmark {
	deleted := make(map[string]struct{}, len(deletedURLs))
	for _, bookmarkURL := range mergeBookmarkDeletionURLs(deletedURLs) {
		deleted[bookmarkURL] = struct{}{}
	}
	filtered := make([]config.BrowserBookmark, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		bookmarkURL := bookmarkURLKey(bookmark.URL)
		if !isProtectedBookmarkURL(bookmarkURL) {
			if _, deleted := deleted[bookmarkURL]; deleted {
				continue
			}
		}
		filtered = append(filtered, bookmark)
	}
	return filtered
}
