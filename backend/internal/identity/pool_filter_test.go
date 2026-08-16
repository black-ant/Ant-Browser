package identity

import (
	"math/rand"
	"testing"
)

func TestPoolFilterByPlatformOnlyYieldsTarget(t *testing.T) {
	recs := []PoolRecord{
		{Platform: "windows", UAFull: "w1", Weight: 1},
		{Platform: "macos", UAFull: "m1", Weight: 1},
		{Platform: "windows", UAFull: "w2", Weight: 1},
	}
	pool := NewPool(recs)
	win := pool.Filter(func(r PoolRecord) bool { return r.Platform == "windows" })
	if win.Len() != 2 {
		t.Fatalf("windows 子池应有 2 条，实际 %d", win.Len())
	}
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 50; i++ {
		if got := win.Sample(r).Platform; got != "windows" {
			t.Fatalf("子池采样应只出 windows，得到 %s", got)
		}
	}
}

func TestPoolFilterEmptyResult(t *testing.T) {
	pool := NewPool([]PoolRecord{{Platform: "windows", Weight: 1}})
	got := pool.Filter(func(r PoolRecord) bool { return r.Platform == "linux" })
	if got.Len() != 0 {
		t.Fatalf("无匹配应得空池，实际 %d", got.Len())
	}
}
