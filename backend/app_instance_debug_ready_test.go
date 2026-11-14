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
	}, nil)

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
	}, nil)

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

func TestWaitBrowserDebugPortReadyAbortsPromptly(t *testing.T) {
	startedAt := time.Now()
	_, err := waitBrowserDebugPortReady(0, t.TempDir(), time.Second, nil, func() bool {
		return true
	})
	if !errors.Is(err, errBrowserStartCancelled) {
		t.Fatalf("wait error = %v, want %v", err, errBrowserStartCancelled)
	}
	if elapsed := time.Since(startedAt); elapsed > 100*time.Millisecond {
		t.Fatalf("abort took %s, want under 100ms", elapsed)
	}
}
