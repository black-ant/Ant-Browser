package identity

import "testing"

// 池记录硬件/版本规范校验(新规则)。
func TestValidatePoolRecordHardwareAndVersion(t *testing.T) {
	good := PoolRecord{
		Platform: "windows", UAFull: BuildReducedUA("windows", 148), BrandVersion: "148.0.0.0",
		HardwareConcurrency: 12, DeviceMemory: 8,
		Screen: Screen{Width: 1920, Height: 1080}, Languages: []string{"en-US"}, Locale: "en-US", Weight: 1,
	}
	if r := ValidatePoolRecord(good); !r.OK {
		t.Fatalf("good record should pass, got issues: %+v", r.Issues)
	}

	// 奇数 CPU → error
	odd := good
	odd.HardwareConcurrency = 7
	if r := ValidatePoolRecord(odd); r.OK {
		t.Fatalf("odd hc should fail, got OK")
	}

	// CPU 超范围(64)→ error
	big := good
	big.HardwareConcurrency = 64
	if r := ValidatePoolRecord(big); r.OK {
		t.Fatalf("hc=64 should fail")
	}

	// deviceMemory>8 → error
	dm := good
	dm.DeviceMemory = 32
	if r := ValidatePoolRecord(dm); r.OK {
		t.Fatalf("dm=32 should fail")
	}

	// UA 版本非内置(145)→ error
	ua145 := good
	ua145.UAFull = BuildReducedUA("windows", 145)
	ua145.BrandVersion = "145.0.0.0"
	if r := ValidatePoolRecord(ua145); r.OK {
		t.Fatalf("UA major 145 (non-builtin) should fail, issues: %+v", r.Issues)
	}
}

// 全量内嵌池零违规(规范化回归闸)。
func TestEmbeddedPoolHasZeroViolations(t *testing.T) {
	recs, err := EmbeddedPoolRecords()
	if err != nil {
		t.Fatalf("load embedded pool: %v", err)
	}
	if len(recs) == 0 {
		t.Fatalf("embedded pool empty")
	}
	for i, r := range recs {
		res := ValidatePoolRecord(r)
		for _, is := range res.Issues {
			if is.Severity == SeverityError {
				t.Errorf("record #%d (ua=%s hc=%d dm=%d): %s", i, r.UAFull, r.HardwareConcurrency, r.DeviceMemory, is.Message)
			}
		}
	}
}
