package server

import (
	"io"
	"time"
)

type LineTimingRecorder struct {
	inner     io.Reader
	startTime time.Time
	Timings   []float64
}

func NewLineTimingRecorder(inner io.Reader, startTime time.Time) *LineTimingRecorder {
	return &LineTimingRecorder{
		inner:     inner,
		startTime: startTime,
	}
}

func (r *LineTimingRecorder) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	if n > 0 {
		ms := float64(time.Since(r.startTime).Microseconds()) / 1000.0
		for _, b := range p[:n] {
			if b == '\n' {
				r.Timings = append(r.Timings, ms)
			}
		}
	}
	return n, err
}
