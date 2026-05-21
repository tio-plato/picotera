package server

import (
	"io"
	"net/http"
	"sync"
)

// streamingResponseRecorder implements http.ResponseWriter + http.Flusher
// backed by an io.Pipe so that SSE frames written by a handler are available
// to a reader immediately (unlike httptest.Recorder which buffers everything).
type streamingResponseRecorder struct {
	header      http.Header
	code        int
	pr          *io.PipeReader
	pw          *io.PipeWriter
	once        sync.Once
	statusReady chan struct{}
}

func newStreamingResponseRecorder() *streamingResponseRecorder {
	pr, pw := io.Pipe()
	return &streamingResponseRecorder{
		header:      make(http.Header),
		pr:          pr,
		pw:          pw,
		statusReady: make(chan struct{}),
	}
}

func (r *streamingResponseRecorder) Header() http.Header { return r.header }

func (r *streamingResponseRecorder) WriteHeader(code int) {
	r.once.Do(func() {
		r.code = code
		close(r.statusReady)
	})
}

func (r *streamingResponseRecorder) Write(p []byte) (int, error) {
	r.once.Do(func() {
		r.code = http.StatusOK
		close(r.statusReady)
	})
	return r.pw.Write(p)
}

func (r *streamingResponseRecorder) Flush() {}

func (r *streamingResponseRecorder) Reader() *io.PipeReader { return r.pr }
func (r *streamingResponseRecorder) StatusCode() int         { return r.code }
func (r *streamingResponseRecorder) StatusReady() <-chan struct{} {
	return r.statusReady
}

func (r *streamingResponseRecorder) Close() error {
	return r.pw.Close()
}
