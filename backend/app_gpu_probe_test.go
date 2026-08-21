package backend

import "testing"

const sampleGPUText = `Graphics Feature Status
Canvas: Hardware accelerated
Video Decode: Hardware accelerated
Video Encode: Software only. Hardware acceleration disabled
WebGL: Hardware accelerated
`

func TestParseGPUFeatureStatus(t *testing.T) {
	status := parseGPUFeatureStatus(sampleGPUText)
	if status["Video Decode"] != "Hardware accelerated" {
		t.Fatalf("Video Decode got %q", status["Video Decode"])
	}
	if status["Video Encode"] != "Software only. Hardware acceleration disabled" {
		t.Fatalf("Video Encode got %q", status["Video Encode"])
	}
	if status["Canvas"] != "Hardware accelerated" {
		t.Fatalf("Canvas got %q", status["Canvas"])
	}
}

func TestParseGPUFeatureStatusIgnoresNoise(t *testing.T) {
	status := parseGPUFeatureStatus("Graphics Feature Status\n\n   \nno-colon-line\nTrailing:\n")
	if len(status) != 0 {
		t.Fatalf("expected no entries, got %v", status)
	}
}

func TestGPUHardwareDecodeOK(t *testing.T) {
	if !gpuHardwareDecodeOK(parseGPUFeatureStatus(sampleGPUText)) {
		t.Error("hardware accelerated video decode should pass")
	}
	soft := parseGPUFeatureStatus("Video Decode: Software only. Hardware acceleration disabled\n")
	if gpuHardwareDecodeOK(soft) {
		t.Error("software-only decode must fail the check")
	}
}

// 读不到条目与没有硬解,对运维的处置一样(必须人工核查),故 fail-closed。
func TestGPUHardwareDecodeFailsClosed(t *testing.T) {
	if gpuHardwareDecodeOK(map[string]string{}) {
		t.Error("missing entry must fail (fail-closed)")
	}
	if gpuHardwareDecodeOK(nil) {
		t.Error("nil status must fail (fail-closed)")
	}
}
