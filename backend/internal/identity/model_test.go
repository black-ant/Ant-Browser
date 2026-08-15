package identity

import "testing"

// sampleIdentity 返回一个内部自洽的样例身份,用于测试。
func sampleIdentity() Identity {
	return Identity{
		Platform:            "windows",
		PlatformVersion:     "10.0.0",
		BrowserBrand:        "Chrome",
		BrandVersion:        "142.0.0.0",
		UAFull:              "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
		HardwareConcurrency: 8,
		DeviceMemory:        8,
		Screen:              Screen{Width: 1920, Height: 1080, DevicePixelRatio: 1, ColorDepth: 24},
		WindowSize:          "1280,720",
		Languages:           []string{"en-US", "en"},
		Locale:              "en-US",
		Timezone:            "America/New_York",
		Geo:                 Geo{Latitude: 40.7128, Longitude: -74.0060, Accuracy: 50},
		Seed:                123456789,
		CanvasNoise:         true,
		ClientRectsNoise:    true,
		WebRTCPolicy:        "disable_non_proxied_udp",
	}
}

// FingerprintHash 必须是确定性的:同一身份多次计算得到相同且非空的 hash。
func TestFingerprintHashIsDeterministic(t *testing.T) {
	id := sampleIdentity()
	h1 := id.FingerprintHash()
	h2 := id.FingerprintHash()
	if h1 == "" {
		t.Fatal("expected non-empty fingerprint hash")
	}
	if h1 != h2 {
		t.Fatalf("expected deterministic hash, got %q then %q", h1, h2)
	}
}

// 关键指纹字段(如 seed)不同,则 hash 必须不同——保证跨环境不重复。
func TestFingerprintHashChangesWithSeed(t *testing.T) {
	a := sampleIdentity()
	b := sampleIdentity()
	b.Seed = a.Seed + 1
	if a.FingerprintHash() == b.FingerprintHash() {
		t.Fatal("expected different hash when seed differs")
	}
}

// 仅溯源/可变状态字段不同(登记来源、代理地理快照、一致性状态),hash 必须相同——
// 这些不属于"身份定义"字段,不应影响唯一性判定。
func TestFingerprintHashIgnoresCosmeticFields(t *testing.T) {
	a := sampleIdentity()
	b := sampleIdentity()
	b.CoherenceStatus = "warning"
	b.ProxyGeoSnapshot = `{"country":"US"}`
	b.SourcePoolRecordID = "pool-42"
	if a.FingerprintHash() != b.FingerprintHash() {
		t.Fatal("expected identical hash when only provenance/status fields differ")
	}
}
