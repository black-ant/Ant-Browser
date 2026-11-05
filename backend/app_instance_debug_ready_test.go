package backend

import (
	"errors"
	"testing"
	"time"
)

func TestStabilizeBrowserDebugPortFailsWhenFinalProbeFails(t *testing.T) {
	finalErr := errors.New("debug port unavailable")
	probeCalls := 0

	_, err := stabilizeBrowserDebugPort(9222, 25*time.Millisecond, false, nil, func(_ int, _ time.Duration) error {
		probeCalls++
		if probeCalls == 1 {
			return nil
		}
		return finalErr
	})

	if !errors.Is(err, finalErr) {
		t.Fatalf("expected final probe error, got %v", err)
	}
	if probeCalls < 2 {
		t.Fatalf("expected final probe, got %d probe calls", probeCalls)
	}
}

func TestStabilizeBrowserDebugPortSucceedsWhenFinalProbeSucceeds(t *testing.T) {
	probeCalls := 0

	debugPort, err := stabilizeBrowserDebugPort(9222, 25*time.Millisecond, false, nil, func(_ int, _ time.Duration) error {
		probeCalls++
		return nil
	})

	if err != nil {
		t.Fatalf("expected stable debug port, got %v", err)
	}
	if debugPort != 9222 {
		t.Fatalf("expected debug port 9222, got %d", debugPort)
	}
	if probeCalls < 2 {
		t.Fatalf("expected final probe, got %d probe calls", probeCalls)
	}
}
