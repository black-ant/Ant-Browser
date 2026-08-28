package channels

import (
	"context"
	"io"

	"golang.org/x/time/rate"
)

const rateLimitReadBurstBytes = 64 * 1024

type rateLimitedReader struct {
	ctx     context.Context
	reader  io.Reader
	limiter *rate.Limiter
	maxRead int
}

func NewRateLimitedReader(ctx context.Context, reader io.Reader, bytesPerSecond int64) io.Reader {
	if reader == nil || bytesPerSecond <= 0 {
		return reader
	}
	if ctx == nil {
		ctx = context.Background()
	}

	burst := rateLimitReadBurstBytes
	if bytesPerSecond < int64(burst) {
		burst = int(bytesPerSecond)
	}
	if burst < 1 {
		burst = 1
	}

	return &rateLimitedReader{
		ctx:     ctx,
		reader:  reader,
		limiter: rate.NewLimiter(rate.Limit(bytesPerSecond), burst),
		maxRead: burst,
	}
}

func (r *rateLimitedReader) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	if len(buffer) > r.maxRead {
		buffer = buffer[:r.maxRead]
	}
	if err := r.limiter.WaitN(r.ctx, len(buffer)); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}
