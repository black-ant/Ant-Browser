package identity

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand"
)

// poolJSON 是内嵌的真机分布指纹池(引导版)。
// 完整版由构建期工具 tools/identity/ 用 fingerprint-suite/BrowserForge 生成后同格式替换。
//
//go:embed data/pool.json
var poolJSON []byte

// PoolRecord 是一条真机指纹模板,采样时整条取出以保证内部自洽。
// locale/timezone 为默认值(直连时使用),绑定代理后会被地理对齐覆盖。
type PoolRecord struct {
	ID                  string   `json:"id,omitempty"` // 运行时可编辑池的稳定标识;内嵌默认池不含,首次物化时赋值。采样不使用。
	Platform            string   `json:"platform"`
	PlatformVersion     string   `json:"platformVersion"`
	BrandVersion        string   `json:"brandVersion"`
	UAFull              string   `json:"uaFull"`
	HardwareConcurrency int      `json:"hardwareConcurrency"`
	DeviceMemory        int      `json:"deviceMemory"`
	Screen              Screen   `json:"screen"`
	WindowSize          string   `json:"windowSize"`
	Languages           []string `json:"languages"`
	Locale              string   `json:"locale"`
	Timezone            string   `json:"timezone"`
	Weight              int      `json:"weight"`
}

// Pool 是一组加权真机指纹记录。
type Pool struct {
	records []PoolRecord
	total   int
}

// EmbeddedPoolRecords 解析并返回内嵌默认指纹池的记录副本(用于"恢复默认"与首次物化)。
func EmbeddedPoolRecords() ([]PoolRecord, error) {
	var recs []PoolRecord
	if err := json.Unmarshal(poolJSON, &recs); err != nil {
		return nil, fmt.Errorf("identity: 解析内嵌指纹池失败: %w", err)
	}
	if len(recs) == 0 {
		return nil, fmt.Errorf("identity: 内嵌指纹池为空")
	}
	return recs, nil
}

// NewPool 用给定记录构建采样池,重算加权采样所需的 total(Weight<=0 的记录不计入)。
func NewPool(records []PoolRecord) *Pool {
	total := 0
	for _, r := range records {
		if r.Weight > 0 {
			total += r.Weight
		}
	}
	return &Pool{records: records, total: total}
}

// LoadEmbeddedPool 载入内嵌指纹池。
func LoadEmbeddedPool() (*Pool, error) {
	recs, err := EmbeddedPoolRecords()
	if err != nil {
		return nil, err
	}
	return NewPool(recs), nil
}

// Len 返回池中记录数。
func (p *Pool) Len() int { return len(p.records) }

// Records 返回池中所有记录(只读用途)。
func (p *Pool) Records() []PoolRecord { return p.records }

// Sample 按权重从池中采样一条记录。
func (p *Pool) Sample(r *rand.Rand) PoolRecord {
	if p.total <= 0 {
		return p.records[r.Intn(len(p.records))]
	}
	pick := r.Intn(p.total)
	acc := 0
	for _, rec := range p.records {
		if rec.Weight <= 0 {
			continue
		}
		acc += rec.Weight
		if pick < acc {
			return rec
		}
	}
	return p.records[len(p.records)-1]
}

// randomSeed 返回 1..2^31-1 的 seed,与 fingerprint-chromium 的 --fingerprint 取值范围一致。
func randomSeed(r *rand.Rand) int64 {
	return r.Int63n(2147483646) + 1
}

// NewIdentity 采样一条记录并配一个随机 seed,组装成基础身份(尚未对齐代理地理)。
func (p *Pool) NewIdentity(r *rand.Rand) Identity {
	return BuildIdentity(p.Sample(r), randomSeed(r))
}

// BuildIdentity 用池记录 + seed 组装一套身份,并填入安全的指纹默认项。
// timezone/locale/languages 取记录默认值,绑定代理后由地理对齐覆盖。
func BuildIdentity(rec PoolRecord, seed int64) Identity {
	return Identity{
		Platform:            rec.Platform,
		PlatformVersion:     rec.PlatformVersion,
		BrowserBrand:        "Chrome",
		BrandVersion:        rec.BrandVersion,
		UAFull:              rec.UAFull,
		HardwareConcurrency: rec.HardwareConcurrency,
		DeviceMemory:        rec.DeviceMemory,
		Screen:              rec.Screen,
		WindowSize:          rec.WindowSize,
		Languages:           rec.Languages,
		Locale:              rec.Locale,
		Timezone:            rec.Timezone,
		Seed:                seed,
		CanvasNoise:         true,
		ClientRectsNoise:    true,
		WebRTCPolicy:        "disable_non_proxied_udp",
		SourcePoolRecordID:  rec.UAFull,
		CoherenceStatus:     "pending",
	}
}
