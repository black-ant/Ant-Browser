package identity

import (
	"fmt"
	"net"

	"github.com/oschwald/maxminddb-golang"
)

// MMDBResolver 用 MaxMind GeoLite2 / DB-IP 格式的离线 .mmdb 实现 GeoResolver。
// 数据文件为构建期产物(见 specs 的 research.md R1),运行期只读离线。
type MMDBResolver struct {
	db *maxminddb.Reader
}

// OpenMMDBResolver 打开 .mmdb 文件;文件不存在或损坏时返回错误(调用方据此优雅降级为不对齐)。
func OpenMMDBResolver(path string) (*MMDBResolver, error) {
	r, err := maxminddb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("identity: 打开 GeoIP 库失败 %q: %w", path, err)
	}
	return &MMDBResolver{db: r}, nil
}

// Close 关闭底层文件。
func (m *MMDBResolver) Close() error {
	if m == nil || m.db == nil {
		return nil
	}
	return m.db.Close()
}

// mmdbRecord 兼容 GeoLite2-City 与 DB-IP City Lite 的字段布局。
type mmdbRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Latitude       float64 `maxminddb:"latitude"`
		Longitude      float64 `maxminddb:"longitude"`
		TimeZone       string  `maxminddb:"time_zone"`
		AccuracyRadius uint16  `maxminddb:"accuracy_radius"`
	} `maxminddb:"location"`
}

// Resolve 把 IP 解析为地理信息。
func (m *MMDBResolver) Resolve(ip string) (GeoInfo, error) {
	if m == nil || m.db == nil {
		return GeoInfo{}, fmt.Errorf("identity: GeoIP 解析器未初始化")
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return GeoInfo{}, fmt.Errorf("identity: 无效 IP %q", ip)
	}
	var rec mmdbRecord
	if err := m.db.Lookup(parsed, &rec); err != nil {
		return GeoInfo{}, err
	}
	acc := float64(rec.Location.AccuracyRadius)
	if acc <= 0 {
		acc = 50 // 缺省定位精度(米)
	}
	return GeoInfo{
		CountryCode: rec.Country.ISOCode,
		City:        rec.City.Names["en"],
		Latitude:    rec.Location.Latitude,
		Longitude:   rec.Location.Longitude,
		Timezone:    rec.Location.TimeZone,
		Accuracy:    acc,
	}, nil
}
