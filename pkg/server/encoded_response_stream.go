package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
)

type internalResponseReader struct {
	Body             io.ReadCloser
	StartClientWrite func() error
}

func decodedInternalResponseReader(resp *http.Response, clientWriter io.Writer) (*internalResponseReader, error) {
	encoding, err := contentEncoding(resp.Header)
	if err != nil {
		return nil, err
	}
	if encoding == "" {
		return &internalResponseReader{Body: resp.Body, StartClientWrite: func() error { return nil }}, nil
	}

	pr, pw := io.Pipe()
	delayedClientWriter := newDelayedWriter(clientWriter)
	rawReader := io.TeeReader(resp.Body, delayedClientWriter)
	go func() {
		_, err := io.Copy(pw, rawReader)
		_ = resp.Body.Close()
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()

	decoded, err := decodedReadCloser(pr, encoding)
	if err != nil {
		_ = pr.Close()
		delayedClientWriter.Discard()
		return nil, err
	}
	return &internalResponseReader{
		Body:             &pipeDecodedReadCloser{decoded: decoded, pipe: pr},
		StartClientWrite: delayedClientWriter.Start,
	}, nil
}

func closeDecodedInternalResponseReader(reader io.ReadCloser, resp *http.Response) {
	if reader == nil {
		return
	}
	if reader == resp.Body {
		_ = resp.Body.Close()
		return
	}
	_ = reader.Close()
}

type pipeDecodedReadCloser struct {
	decoded io.ReadCloser
	pipe    *io.PipeReader
}

func (rc *pipeDecodedReadCloser) Read(p []byte) (int, error) {
	return rc.decoded.Read(p)
}

func (rc *pipeDecodedReadCloser) Close() error {
	err := rc.decoded.Close()
	pipeErr := rc.pipe.Close()
	if err != nil {
		return err
	}
	if pipeErr != nil {
		return fmt.Errorf("close response decode pipe: %w", pipeErr)
	}
	return nil
}

type lockedResponseWriter struct {
	mu sync.Mutex
	w  http.ResponseWriter
}

func newLockedResponseWriter(w http.ResponseWriter) *lockedResponseWriter {
	return &lockedResponseWriter{w: w}
}

func (w *lockedResponseWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

func (w *lockedResponseWriter) Flush() {
	flusher, ok := w.w.(http.Flusher)
	if !ok {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	flusher.Flush()
}

type delayedWriter struct {
	mu      sync.Mutex
	target  io.Writer
	pending bytes.Buffer
	started bool
	discard bool
}

func newDelayedWriter(target io.Writer) *delayedWriter {
	return &delayedWriter{target: target}
}

func (w *delayedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.discard {
		return len(p), nil
	}
	if !w.started {
		return w.pending.Write(p)
	}
	return w.target.Write(p)
}

func (w *delayedWriter) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.discard || w.started {
		return nil
	}
	w.started = true
	if w.pending.Len() == 0 {
		return nil
	}
	_, err := w.target.Write(w.pending.Bytes())
	w.pending.Reset()
	return err
}

func (w *delayedWriter) Discard() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.discard = true
	w.pending.Reset()
}
