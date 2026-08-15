// Package identity 实现每环境指纹自洽引擎的核心:结构化身份模型、离线指纹池采样、
// 代理地理对齐、一致性校验、唯一性登记与内核参数序列化。
//
// 本文件定义结构化身份模型 Identity 及其唯一性指纹 hash。
package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Screen 屏幕几何参数。
type Screen struct {
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	DevicePixelRatio float64 `json:"dpr"`
	ColorDepth       int     `json:"colorDepth"`
}

// Geo 地理定位坐标(与代理出口 IP 对齐)。
type Geo struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
	Accuracy  float64 `json:"accuracy"`
}

// Identity 一个浏览器环境的完整结构化指纹身份。
//
// 字段分两类:
//   - “身份定义”字段:决定这套指纹是谁,参与 FingerprintHash(用于跨环境唯一性)。
//   - 溯源/可变状态字段:SourcePoolRecordID / ProxyGeoSnapshot / CoherenceStatus,
//     不参与 hash(它们描述来源或运行状态,不改变“身份本身”)。
type Identity struct {
	// —— 身份定义字段 ——
	Platform            string   `json:"platform"`
	PlatformVersion     string   `json:"platformVersion"`
	BrowserBrand        string   `json:"browserBrand"`
	BrandVersion        string   `json:"brandVersion"`
	UAFull              string   `json:"uaFull"`
	HardwareConcurrency int      `json:"hardwareConcurrency"`
	DeviceMemory        int      `json:"deviceMemory"`
	Screen              Screen   `json:"screen"`
	WindowSize          string   `json:"windowSize"`
	Languages           []string `json:"languages"`
	Locale              string   `json:"locale"`
	Timezone            string   `json:"timezone"`
	Geo                 Geo      `json:"geo"`
	Seed                int64    `json:"seed"`
	CanvasNoise         bool     `json:"canvasNoise"`
	ClientRectsNoise    bool     `json:"clientRectsNoise"`
	WebRTCPolicy        string   `json:"webrtcPolicy"`

	// —— 溯源 / 可变状态字段(不参与 FingerprintHash) ——
	SourcePoolRecordID string `json:"sourcePoolRecordId"`
	ProxyGeoSnapshot   string `json:"proxyGeoSnapshot"`
	CoherenceStatus    string `json:"coherenceStatus"`
}

// fingerprintKey 仅含身份定义字段,是 FingerprintHash 的稳定输入。
// 字段顺序即 JSON 序列化顺序,保证 hash 确定性。
type fingerprintKey struct {
	Platform            string   `json:"platform"`
	PlatformVersion     string   `json:"platformVersion"`
	BrowserBrand        string   `json:"browserBrand"`
	BrandVersion        string   `json:"brandVersion"`
	UAFull              string   `json:"uaFull"`
	HardwareConcurrency int      `json:"hardwareConcurrency"`
	DeviceMemory        int      `json:"deviceMemory"`
	Screen              Screen   `json:"screen"`
	WindowSize          string   `json:"windowSize"`
	Languages           []string `json:"languages"`
	Locale              string   `json:"locale"`
	Timezone            string   `json:"timezone"`
	Geo                 Geo      `json:"geo"`
	Seed                int64    `json:"seed"`
	CanvasNoise         bool     `json:"canvasNoise"`
	ClientRectsNoise    bool     `json:"clientRectsNoise"`
	WebRTCPolicy        string   `json:"webrtcPolicy"`
}

// FingerprintHash 返回身份定义字段的稳定十六进制摘要(sha256)。
// 两套身份仅当所有身份定义字段一致时才会碰撞;溯源/可变状态字段不影响结果。
func (i Identity) FingerprintHash() string {
	key := fingerprintKey{
		Platform:            i.Platform,
		PlatformVersion:     i.PlatformVersion,
		BrowserBrand:        i.BrowserBrand,
		BrandVersion:        i.BrandVersion,
		UAFull:              i.UAFull,
		HardwareConcurrency: i.HardwareConcurrency,
		DeviceMemory:        i.DeviceMemory,
		Screen:              i.Screen,
		WindowSize:          i.WindowSize,
		Languages:           i.Languages,
		Locale:              i.Locale,
		Timezone:            i.Timezone,
		Geo:                 i.Geo,
		Seed:                i.Seed,
		CanvasNoise:         i.CanvasNoise,
		ClientRectsNoise:    i.ClientRectsNoise,
		WebRTCPolicy:        i.WebRTCPolicy,
	}
	b, _ := json.Marshal(key)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
