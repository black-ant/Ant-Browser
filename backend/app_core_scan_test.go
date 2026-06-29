package backend

import (
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
)

func TestBrowserCoreScanRegistersDetectedCoreIntoSQLite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0755); err != nil {
		t.Fatalf("创建 data 目录失败: %v", err)
	}

	coreExecutable := filepath.Join(root, "chrome", "fingerprint-macos", "Chromium.app", "Contents", "MacOS", "Chromium")
	if err := os.MkdirAll(filepath.Dir(coreExecutable), 0755); err != nil {
		t.Fatalf("创建内核目录失败: %v", err)
	}
	if err := os.WriteFile(coreExecutable, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("写入内核可执行文件失败: %v", err)
	}

	cfg := config.DefaultConfig()
	app := NewApp(root)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, root)

	db, err := database.NewDB(filepath.Join(root, "data", "app.db"))
	if err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("迁移数据库失败: %v", err)
	}

	app.browserMgr.CoreDAO = browser.NewSQLiteCoreDAO(db.GetConn())

	cores := app.BrowserCoreScan()
	if len(cores) != 1 {
		t.Fatalf("期望扫描后注册 1 个内核，实际=%d (%+v)", len(cores), cores)
	}
	if cores[0].CorePath != "chrome/fingerprint-macos" {
		t.Fatalf("期望内核路径为 chrome/fingerprint-macos，实际=%s", cores[0].CorePath)
	}
	if !cores[0].IsDefault {
		t.Fatalf("期望扫描出的内核被设为默认内核: %+v", cores[0])
	}

	listed := app.browserMgr.ListCores()
	if len(listed) != 1 {
		t.Fatalf("期望 DAO 中已有 1 个内核，实际=%d (%+v)", len(listed), listed)
	}
	if listed[0].CorePath != "chrome/fingerprint-macos" {
		t.Fatalf("期望 DAO 内核路径为 chrome/fingerprint-macos，实际=%s", listed[0].CorePath)
	}
}

func TestEnsureDefaultCoresPromotesBundledCoreOverInvalidDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	coreDir := filepath.Join(root, "chrome", "fingerprint")
	coreExecutable := filepath.Join(coreDir, filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(coreExecutable), 0755); err != nil {
		t.Fatalf("创建内核目录失败: %v", err)
	}
	if err := os.WriteFile(coreExecutable, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("写入内核可执行文件失败: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Browser.Cores = []browser.Core{{
		CoreId:    "broken-default",
		CoreName:  "Broken",
		CorePath:  "missing-core",
		IsDefault: true,
	}}

	app := NewApp(root)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, root)

	app.ensureDefaultCores()

	var fingerprintCore browser.Core
	for _, core := range app.config.Browser.Cores {
		if core.CorePath == filepath.Join("chrome", "fingerprint") {
			fingerprintCore = core
		}
	}
	if fingerprintCore.CoreId == "" {
		t.Fatalf("期望注册随包指纹内核: %+v", app.config.Browser.Cores)
	}
	if !fingerprintCore.IsDefault {
		t.Fatalf("期望随包指纹内核成为默认内核: %+v", app.config.Browser.Cores)
	}
}

func TestSystemChromeExecutableDetectionForWindows(t *testing.T) {
	t.Parallel()

	if !isLikelySystemChromeExecutablePathForOS(`C:\Program Files\Google\Chrome\Application\chrome.exe`, "windows") {
		t.Fatal("期望识别 Windows 系统 Chrome")
	}
	if isLikelySystemChromeExecutablePathForOS(`C:\Users\me\AppData\Local\Maka Browser\chrome\fingerprint\chrome.exe`, "windows") {
		t.Fatal("随包指纹内核不应被识别为系统 Chrome")
	}
}
