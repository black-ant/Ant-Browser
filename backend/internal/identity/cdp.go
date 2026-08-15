package identity

// CDPOverride 是一条启动后经 CDP 下发的覆盖指令。
// 用于 Chrome 144+ 已无法通过启动 flag 设置的维度(主要是地理定位),
// 时区/locale 也同时经 CDP 覆盖作为双保险。
type CDPOverride struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

// CDPOverrides 返回该身份启动后应下发的 CDP 覆盖指令。
// 地理坐标为零时不下发 geolocation(避免注入 0,0 这一异常信号)。
func (i Identity) CDPOverrides() []CDPOverride {
	var ovs []CDPOverride

	if (i.Geo != Geo{}) {
		ovs = append(ovs, CDPOverride{
			Method: "Emulation.setGeolocationOverride",
			Params: map[string]any{
				"latitude":  i.Geo.Latitude,
				"longitude": i.Geo.Longitude,
				"accuracy":  i.Geo.Accuracy,
			},
		})
	}

	if i.Timezone != "" {
		ovs = append(ovs, CDPOverride{
			Method: "Emulation.setTimezoneOverride",
			Params: map[string]any{"timezoneId": i.Timezone},
		})
	}

	if i.Locale != "" {
		ovs = append(ovs, CDPOverride{
			Method: "Emulation.setLocaleOverride",
			Params: map[string]any{"locale": i.Locale},
		})
	}

	return ovs
}
