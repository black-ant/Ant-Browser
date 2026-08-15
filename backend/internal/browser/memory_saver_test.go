package browser

import (
	"strings"
	"testing"
)

// 省内存模式必须包含关键内存杠杆,且绝不包含会破坏电商/视频站的开关。
func TestMemorySaverArgsAreEcomVideoSafe(t *testing.T) {
	args := MemorySaverArgs()

	must := []string{"--process-per-site", "--disable-back-forward-cache", "--disable-background-networking"}
	for _, w := range must {
		found := false
		for _, a := range args {
			if a == w {
				found = true
			}
		}
		if !found {
			t.Errorf("missing memory-saver flag %q", w)
		}
	}

	for _, a := range args {
		if a == "--disable-gpu" || a == "--disable-javascript" || a == "--single-process" || strings.Contains(a, "imagesEnabled=false") {
			t.Errorf("memory-saver must not break ecom/video, found %q", a)
		}
	}
}
