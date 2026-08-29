package channels

import (
	"io"
	"time"
)

const uploadProgressReportInterval = 250 * time.Millisecond

type uploadProgressReader struct {
	reader            io.Reader
	totalBytes        int64
	bytesTransferred  int64
	startedAt         time.Time
	lastReportedAt    time.Time
	lastReportedBytes int64
	callback          UploadProgressFunc
}

func NewUploadProgressReader(reader io.Reader, totalBytes int64, callback UploadProgressFunc) io.Reader {
	if reader == nil || callback == nil {
		return reader
	}
	if totalBytes < 0 {
		totalBytes = 0
	}
	now := time.Now()
	return &uploadProgressReader{
		reader:         reader,
		totalBytes:     totalBytes,
		startedAt:      now,
		lastReportedAt: now,
		callback:       callback,
	}
}

func (reader *uploadProgressReader) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	n, err := reader.reader.Read(buffer)
	if n <= 0 {
		return n, err
	}

	reader.bytesTransferred += int64(n)
	now := time.Now()
	completed := reader.totalBytes > 0 && reader.bytesTransferred >= reader.totalBytes
	if !completed && now.Sub(reader.lastReportedAt) < uploadProgressReportInterval {
		return n, err
	}

	elapsed := now.Sub(reader.lastReportedAt).Seconds()
	if elapsed <= 0 {
		elapsed = now.Sub(reader.startedAt).Seconds()
	}
	bytesPerSecond := 0.0
	if elapsed > 0 {
		bytesPerSecond = float64(reader.bytesTransferred-reader.lastReportedBytes) / elapsed
	}
	reader.callback(UploadProgress{
		BytesTransferred: reader.bytesTransferred,
		TotalBytes:       reader.totalBytes,
		BytesPerSecond:   bytesPerSecond,
	})
	reader.lastReportedAt = now
	reader.lastReportedBytes = reader.bytesTransferred
	return n, err
}
