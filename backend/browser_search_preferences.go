package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type searchEnginePreference struct {
	id         string
	name       string
	keyword    string
	searchURL  string
	suggestURL string
	faviconURL string
}

var searchEnginePreferences = map[string]searchEnginePreference{
	"google": {
		id:         "google",
		name:       "Google",
		keyword:    "google.com",
		searchURL:  "https://www.google.com/search?q={searchTerms}",
		suggestURL: "https://www.google.com/complete/search?client=chrome&q={searchTerms}",
		faviconURL: "https://www.google.com/favicon.ico",
	},
	"bing": {
		id:         "bing",
		name:       "Bing",
		keyword:    "bing.com",
		searchURL:  "https://www.bing.com/search?q={searchTerms}",
		suggestURL: "https://www.bing.com/osjson.aspx?query={searchTerms}",
		faviconURL: "https://www.bing.com/favicon.ico",
	},
	"duckduckgo": {
		id:         "duckduckgo",
		name:       "DuckDuckGo",
		keyword:    "duckduckgo.com",
		searchURL:  "https://duckduckgo.com/?q={searchTerms}",
		suggestURL: "https://duckduckgo.com/ac/?q={searchTerms}&type=list",
		faviconURL: "https://duckduckgo.com/favicon.ico",
	},
	"baidu": {
		id:         "baidu",
		name:       "百度",
		keyword:    "baidu.com",
		searchURL:  "https://www.baidu.com/s?wd={searchTerms}",
		suggestURL: "https://www.baidu.com/sugrec?prod=pc&wd={searchTerms}",
		faviconURL: "https://www.baidu.com/favicon.ico",
	},
}

func isSupportedSearchEngine(value string) bool {
	_, ok := searchEnginePreferences[strings.ToLower(strings.TrimSpace(value))]
	return ok
}

func ensureDefaultSearchEngine(userDataDir string, searchEngine string) error {
	engine, ok := searchEnginePreferences[strings.ToLower(strings.TrimSpace(searchEngine))]
	if !ok {
		return nil
	}

	profileDir := filepath.Join(userDataDir, "Default")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return fmt.Errorf("创建 profile 目录失败: %w", err)
	}

	preferencesPath := filepath.Join(profileDir, "Preferences")
	root := map[string]interface{}{}
	if data, err := os.ReadFile(preferencesPath); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("读取 Preferences 失败: %w", err)
		}
	}

	root["default_search_provider"] = map[string]interface{}{
		"enabled":             true,
		"name":                engine.name,
		"keyword":             engine.keyword,
		"search_url":          engine.searchURL,
		"suggest_url":         engine.suggestURL,
		"favicon_url":         engine.faviconURL,
		"new_tab_url":         "",
		"encoding":            "UTF-8",
		"alternate_urls":      []interface{}{},
		"image_url":           "",
		"image_translate_url": "",
	}
	root["default_search_provider_data"] = map[string]interface{}{
		"template_url_data": chromeTemplateURLData(engine),
	}

	out, err := json.MarshalIndent(root, "", "   ")
	if err != nil {
		return fmt.Errorf("序列化 Preferences 失败: %w", err)
	}
	return os.WriteFile(preferencesPath, out, 0644)
}

func chromeTemplateURLData(engine searchEnginePreference) map[string]interface{} {
	return map[string]interface{}{
		"alternate_urls":       []interface{}{},
		"created_by_policy":    false,
		"date_created":         "0",
		"encoding":             "UTF-8",
		"favicon_url":          engine.faviconURL,
		"id":                   1,
		"input_encodings":      []interface{}{"UTF-8"},
		"keyword":              engine.keyword,
		"last_modified":        "0",
		"last_visited":         "0",
		"name":                 engine.name,
		"new_tab_url":          "",
		"safe_for_autoreplace": false,
		"search_url":           engine.searchURL,
		"short_name":           engine.name,
		"suggest_url":          engine.suggestURL,
		"synced_guid":          "ant-browser-" + engine.id,
		"url":                  engine.searchURL,
	}
}
