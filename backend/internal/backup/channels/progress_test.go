package channels

import (
	"bytes"
	"io"
	"testing"
)

func TestUploadProgressReaderReportsFinalTransfer(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 256*1024)
	updates := make([]UploadProgress, 0, 1)
	reader := NewUploadProgressReader(bytes.NewReader(payload), int64(len(payload)), func(progress UploadProgress) {
		updates = append(updates, progress)
	})

	written, err := io.Copy(io.Discard, reader)
	if err != nil {
		t.Fatalf("copy through progress reader: %v", err)
	}
	if written != int64(len(payload)) {
		t.Fatalf("written bytes = %d, want %d", written, len(payload))
	}
	if len(updates) == 0 {
		t.Fatal("expected at least one upload progress update")
	}
	last := updates[len(updates)-1]
	if last.BytesTransferred != int64(len(payload)) || last.TotalBytes != int64(len(payload)) {
		t.Fatalf("last progress = %+v, want completed transfer", last)
	}
	if last.BytesPerSecond < 0 {
		t.Fatalf("last speed = %f, want non-negative", last.BytesPerSecond)
	}
}

func TestUploadProgressReaderReturnsOriginalReaderWhenDisabled(t *testing.T) {
	reader := bytes.NewReader([]byte("content"))
	if got := NewUploadProgressReader(reader, int64(reader.Len()), nil); got != reader {
		t.Fatal("disabled progress should return the original reader")
	}
}
