package identity

import (
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
