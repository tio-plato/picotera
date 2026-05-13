package llmbridge

import (
	"bytes"
	"io"
	"sync"
)

type teeReadCloser struct {
	src  io.ReadCloser
	tee  *bytes.Buffer
	mu   sync.Mutex
	done bool
}

// NewUpstreamTee returns a ReadCloser that mirrors src into tee on every
// successful Read. Closing the returned reader closes src.
func NewUpstreamTee(src io.ReadCloser, tee *bytes.Buffer) io.ReadCloser {
	return &teeReadCloser{src: src, tee: tee}
}

func (t *teeReadCloser) Read(p []byte) (int, error) {
	n, err := t.src.Read(p)
	if n > 0 && t.tee != nil {
		t.mu.Lock()
		t.tee.Write(p[:n])
		t.mu.Unlock()
	}
	return n, err
}

func (t *teeReadCloser) Close() error {
	t.mu.Lock()
	if t.done {
		t.mu.Unlock()
		return nil
	}
	t.done = true
	t.mu.Unlock()
	return t.src.Close()
}
