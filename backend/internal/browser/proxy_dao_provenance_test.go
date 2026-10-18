package browser

import (
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/database"
)

func TestProxySpeedResultProvenancePersistsAndInvalidates(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "proxy.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	dao := NewSQLiteProxyDAO(db.GetConn())
	if err := dao.Upsert(Proxy{ProxyId: "proxy-1", ProxyName: "proxy", ProxyConfig: "http://127.0.0.1:8080"}); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	if err := dao.UpdateSpeedResultWithProvenance("proxy-1", true, 35, "2026-10-17T12:00:00Z", "native", "xray", "fingerprint"); err != nil {
		t.Fatalf("UpdateSpeedResultWithProvenance returned error: %v", err)
	}

	proxies, err := dao.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(proxies) != 1 || proxies[0].LastTestFingerprint != "fingerprint" || proxies[0].LastTestConnectorType != "xray" || proxies[0].LastTestEngine != "native" {
		t.Fatalf("persisted speed provenance = %#v", proxies)
	}

	if err := dao.InvalidateSpeedResult("proxy-1"); err != nil {
		t.Fatalf("InvalidateSpeedResult returned error: %v", err)
	}
	proxies, err = dao.List()
	if err != nil {
		t.Fatalf("List after invalidation returned error: %v", err)
	}
	if proxies[0].LastTestOk || proxies[0].LastLatencyMs != -1 || proxies[0].LastTestedAt != "" || proxies[0].LastTestFingerprint != "" {
		t.Fatalf("invalidated speed result was not cleared: %#v", proxies[0])
	}
}
