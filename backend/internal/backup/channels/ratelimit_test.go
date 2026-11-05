package channels

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestNewRateLimitedReaderReturnsOriginalReaderWhenDisabled(t *testing.T) {
	reader := strings.NewReader("content")
	if got := NewRateLimitedReader(context.Background(), reader, 0); got != reader {
		t.Fatal("disabled rate limit should return the original reader")
	}
}

func TestRateLimitedReaderStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := NewRateLimitedReader(ctx, strings.NewReader("content"), 1)

	_, err := io.ReadAll(reader)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("read error = %v, want context.Canceled", err)
	}
}
