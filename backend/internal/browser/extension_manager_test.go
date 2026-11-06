package browser

import (
	"ant-chrome/backend/internal/config"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildChromeExtensionDownloadURL(t *testing.T) {
	extensionID := "nngceckbapebfimnlniiiahkandclblb"
	defaultURL := BuildChromeExtensionDownloadURL(extensionID)
	if !strings.Contains(defaultURL, "prodversion=148.0.0.0") {
		t.Fatalf("默认下载地址应包含 prodversion=148.0.0.0, 实际为: %s", defaultURL)
	}
	if !strings.Contains(defaultURL, "id%3D"+extensionID) {
		t.Fatalf("默认下载地址应包含 extensionID, 实际为: %s", defaultURL)
	}

	customURL := BuildChromeExtensionDownloadURLWithVersion(extensionID, "150.0.1.2")
	if !strings.Contains(customURL, "prodversion=150.0.1.2") {
		t.Fatalf("指定版本下载地址应包含 prodversion=150.0.1.2, 实际为: %s", customURL)
	}

	invalidURL := BuildChromeExtensionDownloadURL("invalid_id")
	if invalidURL != "" {
		t.Fatalf("非法 ID 应返回空字符串, 实际为: %s", invalidURL)
	}
}

func TestNormalizeChromeVersion(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"148.0.0.0", "148.0.0.0"},
		{"Chrome/142.0.7444.175", "142.0.7444.175"},
		{"148", "148.0.0.0"},
		{"134.0", "134.0.0.0"},
		{"134.0.1", "134.0.1.0"},
		{"", ""},
		{"   ", ""},
		{"not-a-version", ""},
	}

	for _, tc := range cases {
		actual := normalizeChromeVersion(tc.input)
		if actual != tc.expected {
			t.Errorf("normalizeChromeVersion(%q) = %q, 期望: %q", tc.input, actual, tc.expected)
		}
	}
}

func TestResolveDownloadChromeVersion(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{
		AppRoot:  tempDir,
		Profiles: make(map[string]*Profile),
		Config:   &config.Config{},
	}

	// 1. 无内核时回退默认保底版本 148.0.0.0
	ver := mgr.ResolveDownloadChromeVersion()
	if ver != DefaultChromeDownloadVersion() {
		t.Fatalf("无内核时期望 %s, 实际: %s", DefaultChromeDownloadVersion(), ver)
	}

	// 2. 内核版本低于 148（例如 120）时，仍保底返回 148.0.0.0
	lowCoreDir := filepath.Join(tempDir, "core-120")
	_ = os.MkdirAll(lowCoreDir, 0o755)
	_ = os.WriteFile(filepath.Join(lowCoreDir, "manifest.json"), []byte(`{"version":"120.0.0.0"}`), 0o644)

	mgr.Config.Browser.Cores = []Core{
		{CoreId: "core-1", CoreName: "Core 120", CorePath: lowCoreDir, IsDefault: true},
	}
	if lowVer := mgr.ResolveDownloadChromeVersion(); lowVer != DefaultChromeDownloadVersion() {
		t.Fatalf("内核版本低于148时应保底返回 %s, 实际: %s", DefaultChromeDownloadVersion(), lowVer)
	}

	// 3. 内核版本高于 148（例如 152）时，应采用该高版本
	highCoreDir := filepath.Join(tempDir, "core-152")
	_ = os.MkdirAll(highCoreDir, 0o755)
	_ = os.WriteFile(filepath.Join(highCoreDir, "manifest.json"), []byte(`{"version":"152.0.7600.100"}`), 0o644)

	mgr.Config.Browser.Cores = []Core{
		{CoreId: "core-2", CoreName: "Core 152", CorePath: highCoreDir, IsDefault: true},
	}
	if highVer := mgr.ResolveDownloadChromeVersion(); highVer != "152.0.7600.100" {
		t.Fatalf("内核版本为152时应返回 152.0.7600.100, 实际: %s", highVer)
	}
}

func TestDownloadChromeExtensionCRX_MockServer(t *testing.T) {
	// Case A: 模拟 Google AUS 返回 204 No Content
	ts204 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts204.Close()

	_, err204 := downloadChromeExtensionCRXOnce(context.Background(), ts204.Client(), ts204.URL, "148.0.0.0")
	if err204 == nil {
		t.Fatal("204 No Content 应返回错误，实际未报错")
	}
	if !strings.Contains(err204.Error(), "204 No Content") {
		t.Fatalf("错误信息应包含 204 No Content, 实际为: %v", err204)
	}

	// Case B: 模拟返回 200 但内容为空
	tsEmpty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer tsEmpty.Close()

	_, errEmpty := downloadChromeExtensionCRXOnce(context.Background(), tsEmpty.Client(), tsEmpty.URL, "148.0.0.0")
	if errEmpty == nil {
		t.Fatal("空响应体应返回错误，实际未报错")
	}
	if !strings.Contains(errEmpty.Error(), "为空") {
		t.Fatalf("错误信息应提示安装包为空, 实际为: %v", errEmpty)
	}

	// Case C: 模拟返回正常数据并验证 User-Agent Header
	receivedUA := ""
	tsSuccess := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("PKfake-crx-content"))
	}))
	defer tsSuccess.Close()

	data, errSuccess := downloadChromeExtensionCRXOnce(context.Background(), tsSuccess.Client(), tsSuccess.URL, "150.0.0.0")
	if errSuccess != nil {
		t.Fatalf("正常下载不应报错: %v", errSuccess)
	}
	if string(data) != "PKfake-crx-content" {
		t.Fatalf("读取的数据不匹配: %s", string(data))
	}
	if !strings.Contains(receivedUA, "Chrome/150.0.0.0") {
		t.Fatalf("User-Agent 应包含 Chrome/150.0.0.0, 实际为: %s", receivedUA)
	}

	// Case D: 验证重试判断逻辑
	if isRetryableExtensionDownloadError(err204) {
		t.Fatal("204 错误不应作为可重试错误")
	}
	if isRetryableExtensionDownloadError(errEmpty) {
		t.Fatal("空包错误不应作为可重试错误")
	}
	if !isRetryableExtensionDownloadError(fmt.Errorf("connection reset by peer")) {
		t.Fatal("connection reset 应识别为可重试错误")
	}
}
