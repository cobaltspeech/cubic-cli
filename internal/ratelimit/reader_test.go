package ratelimit

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
)

// Tests were borrowed from https://github.com/fujiwara/shapeio

var rates = []float64{
	500 * 1024,       // 500KB/sec
	1024 * 1024,      // 1MB/sec
	10 * 1024 * 1024, // 10MB/sec
	50 * 1024 * 1024, // 50MB/sec
}

var srcs = []*bytes.Reader{
	bytes.NewReader(bytes.Repeat([]byte{0}, 64*1024)),   // 64KB
	bytes.NewReader(bytes.Repeat([]byte{1}, 256*1024)),  // 256KB
	bytes.NewReader(bytes.Repeat([]byte{2}, 1024*1024)), // 1MB
}

func TestRead(t *testing.T) {
	for _, src := range srcs {
		for _, limit := range rates {
			_, _ = src.Seek(0, 0)
			lr := NewReader(context.Background(), src)
			lr.SetRateLimit(limit)
			start := time.Now()
			n, err := io.Copy(ioutil.Discard, lr)
			elapsed := time.Since(start)
			if err != nil {
				t.Error("io.Copy failed", err)
			}
			realRate := float64(n) / elapsed.Seconds()
			if realRate > limit {
				t.Errorf("Limit %f but real rate %f", limit, realRate)
			}
			t.Logf(
				"read %s / %s: Real %s/sec Limit %s/sec. (%f %%)",
				humanize.IBytes(uint64(n)),
				elapsed,
				humanize.IBytes(uint64(realRate)),
				humanize.IBytes(uint64(limit)),
				realRate/limit*100,
			)
		}
	}
}

func TestReadNilLimiter(t *testing.T) {
	src := srcs[2]
	limit := rates[3]

	_, _ = src.Seek(0, 0)
	lr := NewReader(context.Background(), src)
	// lr.SetRateLimit(limit) // Don't set the limit here.
	start := time.Now()
	n, err := io.Copy(ioutil.Discard, lr)
	elapsed := time.Since(start)
	if err != nil {
		t.Error("io.Copy failed", err)
	}
	realRate := float64(n) / elapsed.Seconds()
	if realRate < limit { // We want to make sure this goes fast.
		t.Errorf("Limit %f but real rate %f", limit, realRate)
	}
	t.Logf(
		"read %s / %s: Real %s/sec Limit %s/sec. (%f %%)",
		humanize.IBytes(uint64(n)),
		elapsed,
		humanize.IBytes(uint64(realRate)),
		humanize.IBytes(uint64(limit)),
		realRate/limit*100,
	)
}
