package automation

import "testing"

func TestDefaultScriptBundles(t *testing.T) {
	bundles, err := DefaultScriptBundles()
	if err != nil {
		t.Fatalf("DefaultScriptBundles returned error: %v", err)
	}
	if len(bundles) != 7 {
		t.Fatalf("expected seven default script bundles, got %d", len(bundles))
	}

	expected := []struct {
		id        string
		name      string
		uri       string
		publicAPI string
	}{
		{
			id:        DualInstanceRuntimeScriptID,
			name:      "双实例启动与 Runtime 切换",
			uri:       "repo://backend/internal/automation/demo-library/dual-instance-runtime-switch",
			publicAPI: "",
		},
		{
			id:        NewsQueryTXTScriptID,
			name:      "查询新闻并写 TXT",
			uri:       "repo://backend/internal/automation/demo-library/news-query-txt",
			publicAPI: "",
		},
		{
			id:        ProtonMailFirstMessageID,
			name:      "Proton 邮件搜索并读取最新邮件",
			uri:       "repo://backend/internal/automation/demo-library/proton-mail-first-message",
			publicAPI: "mail/proton-first-message",
		},
		{
			id:        WebImageGenerateScriptID,
			name:      "网页图片生成并下载",
			uri:       "repo://backend/internal/automation/demo-library/web-image-generate-download",
			publicAPI: "image/chatgpt-generate-download",
		},
		{
			id:        LianjiaWHHomeStep1ScriptID,
			name:      "链家武汉首页 S1 打开页面",
			uri:       "repo://backend/internal/automation/demo-library/lianjia-wh-home-step1",
			publicAPI: "",
		},
		{
			id:        LianjiaWHCookiePrepareID,
			name:      "链家武汉 Cookie 准备",
			uri:       "repo://backend/internal/automation/demo-library/lianjia-wh-cookie-prepare",
			publicAPI: "",
		},
		{
			id:        BeikeHousePriceExtractID,
			name:      "贝壳二手房价格提取",
			uri:       "repo://backend/internal/automation/demo-library/beike-house-price-extract",
			publicAPI: "",
		},
	}

	for index, item := range expected {
		bundle := bundles[index]
		if bundle.Record.ID != item.id {
			t.Fatalf("unexpected bundle id at %d: want %q got %q", index, item.id, bundle.Record.ID)
		}
		if bundle.Record.Name != item.name {
			t.Fatalf("unexpected bundle name at %d: want %q got %q", index, item.name, bundle.Record.Name)
		}
		if bundle.Record.EntryFile != "index.cjs" {
			t.Fatalf("unexpected entry file for %q: %q", bundle.Record.ID, bundle.Record.EntryFile)
		}
		if bundle.Record.Source.Type != "builtin" || bundle.Record.Source.URI != item.uri {
			t.Fatalf("unexpected source for %q: %+v", bundle.Record.ID, bundle.Record.Source)
		}
		if bundle.Record.PublicAPI.Path != item.publicAPI {
			t.Fatalf("unexpected public api path for %q: %+v", bundle.Record.ID, bundle.Record.PublicAPI)
		}
		if item.publicAPI != "" && !bundle.Record.PublicAPI.Enabled {
			t.Fatalf("expected public api to be enabled for %q", bundle.Record.ID)
		}
		if bundle.Record.ID == WebImageGenerateScriptID {
			variables := bundle.Record.PublicAPI.Variables
			if len(variables) != 2 || variables[0].Name != "code" || variables[1].Name != "prompt" {
				t.Fatalf("expected web image script to expose code and prompt variables, got %+v", variables)
			}
		}
		if len(bundle.Files) == 0 {
			t.Fatalf("expected bundled files for %q", bundle.Record.ID)
		}
	}
}

func TestImportBuiltinBundleFromSourcePreservesManifestID(t *testing.T) {
	bundle, err := ImportBuiltinBundleFromSource(ScriptSource{
		Type: "builtin",
		Path: NewsQueryTXTScriptID,
	})
	if err != nil {
		t.Fatalf("ImportBuiltinBundleFromSource returned error: %v", err)
	}
	if bundle.Record.ID != NewsQueryTXTScriptID {
		t.Fatalf("expected manifest id %q, got %q", NewsQueryTXTScriptID, bundle.Record.ID)
	}
	if bundle.Record.Source.ImportedAt == "" {
		t.Fatalf("expected importedAt to be populated")
	}
}
