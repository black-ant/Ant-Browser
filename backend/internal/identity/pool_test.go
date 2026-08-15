package identity

import (
	"fmt"
	"math/rand"
	"testing"
)

// 内嵌指纹池应非空,且每条记录基本字段合法。
func TestLoadEmbeddedPoolNonEmpty(t *testing.T) {
	p, err := LoadEmbeddedPool()
	if err != nil {
		t.Fatalf("LoadEmbeddedPool: %v", err)
	}
	if p.Len() == 0 {
		t.Fatal("expected a non-empty embedded fingerprint pool")
	}
	for i, rec := range p.Records() {
		if rec.Platform == "" || rec.UAFull == "" || rec.Weight <= 0 {
			t.Fatalf("record %d invalid: %+v", i, rec)
		}
	}
}

// 相同随机源应产出相同采样(确定性,便于复现与测试)。
func TestSampleIsDeterministicForSameSeed(t *testing.T) {
	p, _ := LoadEmbeddedPool()
	a := p.Sample(rand.New(rand.NewSource(42)))
	b := p.Sample(rand.New(rand.NewSource(42)))
	if a.UAFull != b.UAFull {
		t.Fatalf("expected deterministic sample, got %q vs %q", a.UAFull, b.UAFull)
	}
	if a.Platform == "" {
		t.Fatal("sampled record must carry a platform")
	}
}

// BuildIdentity 应把池记录的字段与给定 seed 组装成一套可哈希、可序列化的身份。
func TestBuildIdentityCarriesRecordAndSeed(t *testing.T) {
	rec := PoolRecord{
		Platform:            "macos",
		PlatformVersion:     "13.5.0",
		BrandVersion:        "142.0.0.0",
		UAFull:              "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
		HardwareConcurrency: 10,
		DeviceMemory:        16,
		Screen:              Screen{Width: 2560, Height: 1600, DevicePixelRatio: 2, ColorDepth: 30},
		WindowSize:          "1440,900",
		Languages:           []string{"en-US", "en"},
		Locale:              "en-US",
		Timezone:            "America/Los_Angeles",
		Weight:              1,
	}
	id := BuildIdentity(rec, 777)
	if id.Platform != "macos" || id.Seed != 777 || id.UAFull != rec.UAFull {
		t.Fatalf("identity should carry record fields + seed, got %+v", id)
	}
	if id.BrowserBrand != "Chrome" {
		t.Fatalf("expected default brand Chrome, got %q", id.BrowserBrand)
	}
	if !id.CanvasNoise || !id.ClientRectsNoise || id.WebRTCPolicy != "disable_non_proxied_udp" {
		t.Fatal("expected safe fingerprint defaults (canvas/clientrects noise, webrtc leak protection)")
	}
	if id.FingerprintHash() == "" {
		t.Fatal("built identity must hash")
	}
	if !hasArg(id.LaunchArgs(), "--fingerprint-platform=macos") {
		t.Fatal("built identity must serialize its platform")
	}
}

// 采样 + 重采唯一 + 落库,应产出互不重复的身份(SC-001 的小规模验证)。
func TestPoolGeneratesUniqueIdentitiesAtScale(t *testing.T) {
	store := newTestStore(t)
	p, _ := LoadEmbeddedPool()
	r := rand.New(rand.NewSource(1))
	seen := map[string]bool{}
	const n = 50
	for i := 0; i < n; i++ {
		id, err := GenerateUnique(store, func() Identity { return p.NewIdentity(r) }, 100)
		if err != nil {
			t.Fatalf("generate %d: %v", i, err)
		}
		if seen[id.FingerprintHash()] {
			t.Fatalf("duplicate identity produced at index %d", i)
		}
		seen[id.FingerprintHash()] = true
		if err := store.Save(fmt.Sprintf("p%d", i), id); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}
	if len(seen) != n {
		t.Fatalf("expected %d unique identities, got %d", n, len(seen))
	}
}
