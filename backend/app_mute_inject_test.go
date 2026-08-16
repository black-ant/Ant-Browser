package backend

import (
	"reflect"
	"testing"
)

// muteAudioLaunchArgs 决定是否需要在启动参数里注入 --mute-audio(硬静音,取消需重启该实例)。
// 直接测试该纯函数,覆盖新增的静音决策逻辑本身。
func TestMuteAudioLaunchArgs(t *testing.T) {
	cases := []struct {
		name    string
		profile *BrowserProfile
		want    []string
	}{
		{"静音开启时注入--mute-audio", &BrowserProfile{MuteAudio: true}, []string{"--mute-audio"}},
		{"静音关闭时不注入", &BrowserProfile{MuteAudio: false}, nil},
		{"空实例不注入", nil, nil},
	}
	for _, c := range cases {
		got := muteAudioLaunchArgs(c.profile)
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
