package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDefaultSearchEngineWritesChromePreferences(t *testing.T) {
	t.Parallel()

	userDataDir := t.TempDir()
	preferencesPath := filepath.Join(userDataDir, "Default", "Preferences")
	if err := os.MkdirAll(filepath.Dir(preferencesPath), 0755); err != nil {
		t.Fatalf("创建测试 Preferences 目录失败: %v", err)
	}
	if err := os.WriteFile(preferencesPath, []byte(`{"profile":{"name":"keep-me"}}`), 0644); err != nil {
		t.Fatalf("写入测试 Preferences 失败: %v", err)
	}

	if err := ensureDefaultSearchEngine(userDataDir, "duckduckgo"); err != nil {
		t.Fatalf("ensureDefaultSearchEngine failed: %v", err)
	}

	data, err := os.ReadFile(preferencesPath)
	if err != nil {
		t.Fatalf("读取 Preferences 失败: %v", err)
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("Preferences JSON 无效: %v", err)
	}

	profile, ok := root["profile"].(map[string]interface{})
	if !ok || profile["name"] != "keep-me" {
		t.Fatalf("未保留既有 Preferences 字段: %#v", root["profile"])
	}

	defaultSearch, ok := root["default_search_provider"].(map[string]interface{})
	if !ok {
		t.Fatalf("缺少 default_search_provider: %#v", root)
	}
	if defaultSearch["name"] != "DuckDuckGo" {
		t.Fatalf("搜索引擎名称错误: %#v", defaultSearch["name"])
	}
	if defaultSearch["search_url"] != "https://duckduckgo.com/?q={searchTerms}" {
		t.Fatalf("搜索 URL 错误: %#v", defaultSearch["search_url"])
	}

	searchData, ok := root["default_search_provider_data"].(map[string]interface{})
	if !ok {
		t.Fatalf("缺少 default_search_provider_data: %#v", root)
	}
	templateData, ok := searchData["template_url_data"].(map[string]interface{})
	if !ok {
		t.Fatalf("缺少 template_url_data: %#v", searchData)
	}
	if templateData["keyword"] != "duckduckgo.com" {
		t.Fatalf("template_url_data keyword 错误: %#v", templateData["keyword"])
	}
}
