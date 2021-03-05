package ratelimit

import (
	"context"
	"io"
	"time"

	"golang.org/x/time/rate"
)

const burstLimit = 1000 * 1000 * 1000

type Reader struct {
	r       io.Reader
	limiter *rate.Limiter
	ctx     context.Context
}

// NewReader returns a reader that is rate limited l.
// Each token in the bucket is one byte.
// If you want 8kb/sec, you would set l = rate.NewLimiter(rate.Every(1*time.Second), 8192).
func NewReader(c context.Context, r io.Reader) *Reader {
	return &Reader{
		r:   r,
		ctx: c,
	}
}

// SetRateLimit sets rate limit (bytes/sec) to the reader.
func (s *Reader) SetRateLimit(bytesPerSec float64) {
	s.limiter = rate.NewLimiter(rate.Limit(bytesPerSec), burstLimit)
	s.limiter.AllowN(time.Now(), burstLimit) // spend initial burst
}

// Read reads bytes into p.
func (s *Reader) Read(p []byte) (int, error) {
	// If it's not limited, read as much as you want.
	if s.limiter == nil {
		return s.r.Read(p)
	}
	// Read from the parent reader.
	n, err := s.r.Read(p)
	if err != nil {
		return n, err
	}

	// Set up delay for the next round.
	if err := s.limiter.WaitN(s.ctx, n); err != nil {
		return n, err
	}

	return n, nil
}
