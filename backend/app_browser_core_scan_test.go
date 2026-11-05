package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"testing"
)

type browserCoreDAOStub struct {
	list []browser.Core
}

func (s *browserCoreDAOStub) List() ([]browser.Core, error) {
	return append([]browser.Core{}, s.list...), nil
}

func (s *browserCoreDAOStub) Upsert(core browser.Core) error {
	for i := range s.list {
		if s.list[i].CoreId == core.CoreId {
			s.list[i] = core
			return nil
		}
	}
	s.list = append(s.list, core)
	return nil
}

func (s *browserCoreDAOStub) Delete(coreId string) error {
	next := s.list[:0]
	for _, core := range s.list {
		if core.CoreId != coreId {
			next = append(next, core)
		}
	}
	s.list = next
	return nil
}

func (s *browserCoreDAOStub) SetDefault(coreId string) error {
	for i := range s.list {
		s.list[i].IsDefault = s.list[i].CoreId == coreId
	}
	return nil
}

func TestBrowserCoreScanRegistersDetectedCoreInDAO(t *testing.T) {
	root := t.TempDir()
	coreDir := filepath.Join(root, "chrome", "chromium-148")
	exePath := filepath.Join(coreDir, filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("创建测试内核目录失败: %v", err)
	}
	if err := os.WriteFile(exePath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("写入测试内核可执行文件失败: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Browser.Cores = nil

	app := NewApp(root)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, root)
	dao := &browserCoreDAOStub{}
	app.browserMgr.CoreDAO = dao

	cores := app.BrowserCoreScan()
	if len(dao.list) != 1 {
		t.Fatalf("扫描后应写入 DAO 1 个内核，got=%d", len(dao.list))
	}
	if dao.list[0].CoreId != "core-chromium-148" {
		t.Fatalf("内核 ID 不符合预期: got=%q", dao.list[0].CoreId)
	}
	if dao.list[0].CorePath != filepath.Join("chrome", "chromium-148") {
		t.Fatalf("内核路径不符合预期: got=%q", dao.list[0].CorePath)
	}
	if !dao.list[0].IsDefault {
		t.Fatal("首个扫描到的内核应设为默认")
	}
	if len(cores) != 1 || cores[0].CoreId != dao.list[0].CoreId {
		t.Fatalf("扫描返回列表未包含新内核: %+v", cores)
	}
}
