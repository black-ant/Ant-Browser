package browser

import (
	"testing"
	"time"
)

func TestNewStartupQueueDefaultsConcurrency(t *testing.T) {
	t.Parallel()

	q := NewStartupQueue(0)
	if got := q.GetQueueInfo()["maxConcurrent"].(int); got != 3 {
		t.Fatalf("expected default maxConcurrent=3, got %d", got)
	}
}

func TestStartupQueueLimitsConcurrency(t *testing.T) {
	t.Parallel()

	q := NewStartupQueue(2)

	// 占满两个并发名额
	if err := q.Acquire("a"); err != nil {
		t.Fatalf("acquire a: %v", err)
	}
	if err := q.Acquire("b"); err != nil {
		t.Fatalf("acquire b: %v", err)
	}

	info := q.GetQueueInfo()
	if info["current"].(int) != 2 || info["available"].(int) != 0 {
		t.Fatalf("expected current=2 available=0, got %+v", info)
	}

	// 第三个获取应当阻塞，直到有名额释放
	acquired := make(chan struct{})
	go func() {
		_ = q.Acquire("c")
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("third Acquire should block while queue is full")
	case <-time.After(150 * time.Millisecond):
		// 符合预期：仍在阻塞
	}

	// 释放一个名额后，第三个应当迅速完成
	q.Release()
	select {
	case <-acquired:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("third Acquire did not proceed after Release")
	}

	// 收尾：释放剩余名额
	q.Release()
	q.Release()
}
