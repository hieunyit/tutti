package copy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	minio "github.com/minio/minio-go/v7"
	"golang.org/x/time/rate"
)

// objectOpener opens a ranged read [start, end] (inclusive) of the source object.
type objectOpener func(ctx context.Context, start, end int64) (io.ReadCloser, error)

// parallelSource implements io.ReaderAt (and io.Reader/io.Seeker for minio-go's fallback paths) on top of
// independent ranged GETs, so minio-go can read several parts concurrently when PutOptions.NumThreads > 1.
// minio.Object.ReadAt would serialize the parts behind its mutex.
//
// minio-go reads each part through an io.SectionReader in small chunks, i.e. as consecutive ReadAt calls.
// A streaming range GET per part is therefore kept open and continued while the next offset matches, and
// closed once the part is read. A retried part restarts at its own offset and opens a new stream.
type parallelSource struct {
	ctx      context.Context
	size     int64
	partSize int64
	open     objectOpener
	limiter  *rate.Limiter

	mu      sync.Mutex
	streams map[int64]*rangeStream // keyed by next expected offset
	pos     int64                  // sequential Read/Seek position
	err     error
	closed  bool
}

type rangeStream struct {
	rc  io.ReadCloser
	end int64 // exclusive
}

func newParallelSource(ctx context.Context, size, partSize int64, limiter *rate.Limiter, open objectOpener) *parallelSource {
	return &parallelSource{ctx: ctx, size: size, partSize: partSize, open: open, limiter: limiter, streams: map[int64]*rangeStream{}}
}

func minioObjectOpener(obj func(ctx context.Context, opts minio.GetObjectOptions) (*minio.Object, error), version, etag string) objectOpener {
	return func(ctx context.Context, start, end int64) (io.ReadCloser, error) {
		opts := minio.GetObjectOptions{VersionID: version}
		if err := opts.SetRange(start, end); err != nil {
			return nil, err
		}
		if etag != "" {
			// every part must come from the same object revision the stat described
			if err := opts.SetMatchETag(etag); err != nil {
				return nil, err
			}
		}
		o, err := obj(ctx, opts)
		if err != nil {
			return nil, err
		}
		return o, nil
	}
}

func (p *parallelSource) setErr(err error) {
	if err != nil && !errors.Is(err, io.EOF) {
		p.mu.Lock()
		if p.err == nil {
			p.err = err
		}
		p.mu.Unlock()
	}
}

// Err returns the first non-EOF source read error.
func (p *parallelSource) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

func (p *parallelSource) ReadAt(b []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("negative offset")
	}
	if off >= p.size {
		return 0, io.EOF
	}
	want := int64(len(b))
	eof := false
	if off+want > p.size {
		want = p.size - off
		eof = true
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return 0, errors.New("source is closed")
	}
	st := p.streams[off]
	delete(p.streams, off)
	p.mu.Unlock()

	if st == nil {
		// a new part (or a retried one): stream up to the end of its partSize-aligned part
		end := off + p.partSize
		if rem := p.partSize - off%p.partSize; off%p.partSize != 0 {
			end = off + rem
		}
		if end > p.size {
			end = p.size
		}
		rc, err := p.open(p.ctx, off, end-1)
		if err != nil {
			p.setErr(err)
			return 0, err
		}
		st = &rangeStream{rc: rc, end: end}
	}

	n, err := io.ReadFull(st.rc, b[:want])
	if p.limiter != nil && n > 0 {
		if werr := p.waitN(n); werr != nil {
			st.rc.Close()
			return n, werr
		}
	}
	if err != nil {
		st.rc.Close()
		if errors.Is(err, io.ErrUnexpectedEOF) {
			err = io.ErrUnexpectedEOF
		}
		p.setErr(err)
		return n, fmt.Errorf("read source range: %w", err)
	}
	next := off + int64(n)
	if next >= st.end {
		st.rc.Close()
	} else {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			st.rc.Close()
		} else {
			if old := p.streams[next]; old != nil {
				old.rc.Close()
			}
			p.streams[next] = st
			p.mu.Unlock()
		}
	}
	if eof {
		return n, io.EOF
	}
	return n, nil
}

func (p *parallelSource) waitN(n int) error {
	burst := p.limiter.Burst()
	for n > 0 {
		c := n
		if burst > 0 && c > burst {
			c = burst
		}
		if err := p.limiter.WaitN(p.ctx, c); err != nil {
			return err
		}
		n -= c
	}
	return nil
}

func (p *parallelSource) Read(b []byte) (int, error) {
	p.mu.Lock()
	pos := p.pos
	p.mu.Unlock()
	n, err := p.ReadAt(b, pos)
	p.mu.Lock()
	p.pos += int64(n)
	p.mu.Unlock()
	return n, err
}

func (p *parallelSource) Seek(offset int64, whence int) (int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += p.pos
	case io.SeekEnd:
		offset += p.size
	default:
		return 0, errors.New("invalid whence")
	}
	if offset < 0 {
		return 0, errors.New("negative position")
	}
	p.pos = offset
	return offset, nil
}

// Close releases all open range streams.
func (p *parallelSource) Close() error {
	p.mu.Lock()
	p.closed = true
	streams := p.streams
	p.streams = map[int64]*rangeStream{}
	p.mu.Unlock()
	for _, st := range streams {
		st.rc.Close()
	}
	return nil
}
