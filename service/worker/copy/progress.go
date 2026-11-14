package copy

import (
	"context"
	"sync/atomic"
)

type progressKey struct{}

// WithProgress makes CopyObject report the cumulative number of bytes sent to the destination while an object
// is transferred. fn is called from the upload path (possibly from several goroutines) on every chunk, so it
// must be cheap and must not block.
func WithProgress(ctx context.Context, fn func(done int64)) context.Context {
	return context.WithValue(ctx, progressKey{}, fn)
}

func progressFrom(ctx context.Context) func(int64) {
	fn, _ := ctx.Value(progressKey{}).(func(int64))
	return fn
}

// progressReader is handed to minio-go as PutObjectOptions.Progress: it is read with exactly the bytes of
// every chunk sent to the destination. A part retried by the SDK is sent twice, so the count is capped to size.
type progressReader struct {
	fn    func(int64)
	size  int64
	total atomic.Int64
}

func (p *progressReader) Read(b []byte) (int, error) {
	n := int64(len(b))
	if n == 0 {
		return 0, nil
	}
	done := p.total.Add(n)
	if p.size > 0 && done > p.size {
		done = p.size
	}
	p.fn(done)
	return len(b), nil
}
