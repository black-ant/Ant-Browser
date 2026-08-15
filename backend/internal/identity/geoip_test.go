package identity

import (
	"os"
	"path/filepath"
	"testing"
)

// .mmdb 不存在时应返回错误(供调用方优雅降级为不做地理对齐)。
func TestOpenMMDBResolverMissingFile(t *testing.T) {
	_, err := OpenMMDBResolver(filepath.Join(t.TempDir(), "does-not-exist.mmdb"))
	if err == nil {
		t.Fatal("expected an error opening a nonexistent mmdb")
	}
}

// 未初始化的解析器解析 IP 应报错而非 panic。
func TestNilResolverResolveErrors(t *testing.T) {
	var m *MMDBResolver
	if _, err := m.Resolve("8.8.8.8"); err == nil {
		t.Fatal("expected error from nil resolver")
	}
}

// MMDBResolver 满足 GeoResolver 接口(编译期保证)。
var _ GeoResolver = (*MMDBResolver)(nil)

// 集成测试:若本机存在真实 mmdb,则解析已知 IP 验证解析器与数据格式。
// 无 mmdb(如 CI)时跳过。
func TestMMDBResolverResolvesKnownIP(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "geoip", "dbip-city-lite.mmdb")
	if _, err := os.Stat(path); err != nil {
		t.Skip("no mmdb present; skipping integration test")
	}
	r, err := OpenMMDBResolver(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer r.Close()
	geo, err := r.Resolve("8.8.8.8")
	if err != nil {
		t.Fatalf("resolve 8.8.8.8: %v", err)
	}
	if geo.CountryCode != "US" {
		t.Errorf("expected US for 8.8.8.8, got %q", geo.CountryCode)
	}
	if geo.Latitude == 0 && geo.Longitude == 0 {
		t.Error("expected non-zero coordinates")
	}
	t.Logf("8.8.8.8 -> country=%s city=%s tz=%s lat=%.4f lon=%.4f",
		geo.CountryCode, geo.City, geo.Timezone, geo.Latitude, geo.Longitude)
}
