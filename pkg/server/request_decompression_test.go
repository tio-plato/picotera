package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// capturingNext is a test helper that records the request it receives.
type capturingNext struct {
	called bool
	body   []byte
	r      *http.Request
}

func (c *capturingNext) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.called = true
	c.r = r
	c.body, _ = io.ReadAll(r.Body)
}

func TestDecompressRequestNoEncoding(t *testing.T) {
	payload := []byte("plain body")
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	w := httptest.NewRecorder()

	next := &capturingNext{}
	decompressRequest(next).ServeHTTP(w, r)

	if !next.called {
		t.Fatal("expected next handler to be called")
	}
	if string(next.body) != string(payload) {
		t.Fatalf("body = %q, want %q", next.body, payload)
	}
	if next.r.Header.Get("Content-Encoding") != "" {
		t.Fatal("expected no Content-Encoding header")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestDecompressRequestGzip(t *testing.T) {
	payload := "hello gzip"
	body := gzipBytes(t, []byte(payload))
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Encoding", "gzip")
	r.Header.Set("Content-Length", "123")
	w := httptest.NewRecorder()

	next := &capturingNext{}
	decompressRequest(next).ServeHTTP(w, r)

	if !next.called {
		t.Fatal("expected next handler to be called")
	}
	if string(next.body) != payload {
		t.Fatalf("body = %q, want %q", next.body, payload)
	}
	if next.r.Header.Get("Content-Encoding") != "" {
		t.Fatal("expected Content-Encoding to be stripped")
	}
	if next.r.ContentLength != -1 {
		t.Fatalf("ContentLength = %d, want -1", next.r.ContentLength)
	}
}

func TestDecompressRequestBrotli(t *testing.T) {
	payload := "hello brotli"
	body := brotliBytes(t, []byte(payload))
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Encoding", "br")
	w := httptest.NewRecorder()

	next := &capturingNext{}
	decompressRequest(next).ServeHTTP(w, r)

	if !next.called {
		t.Fatal("expected next handler to be called")
	}
	if string(next.body) != payload {
		t.Fatalf("body = %q, want %q", next.body, payload)
	}
	if next.r.Header.Get("Content-Encoding") != "" {
		t.Fatal("expected Content-Encoding to be stripped")
	}
	if next.r.ContentLength != -1 {
		t.Fatalf("ContentLength = %d, want -1", next.r.ContentLength)
	}
}

func TestDecompressRequestZstd(t *testing.T) {
	payload := "hello zstd"
	body := zstdBytes(t, []byte(payload))
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Encoding", "zstd")
	w := httptest.NewRecorder()

	next := &capturingNext{}
	decompressRequest(next).ServeHTTP(w, r)

	if !next.called {
		t.Fatal("expected next handler to be called")
	}
	if string(next.body) != payload {
		t.Fatalf("body = %q, want %q", next.body, payload)
	}
	if next.r.Header.Get("Content-Encoding") != "" {
		t.Fatal("expected Content-Encoding to be stripped")
	}
	if next.r.ContentLength != -1 {
		t.Fatalf("ContentLength = %d, want -1", next.r.ContentLength)
	}
}

func TestDecompressRequestMultipleContentEncoding(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header["Content-Encoding"] = []string{"gzip", "br"}
	w := httptest.NewRecorder()

	next := &capturingNext{}
	decompressRequest(next).ServeHTTP(w, r)

	if next.called {
		t.Fatal("expected next handler NOT to be called")
	}
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", w.Code)
	}
}

func TestDecompressRequestUnknownEncoding(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Content-Encoding", "deflate")
	w := httptest.NewRecorder()

	next := &capturingNext{}
	decompressRequest(next).ServeHTTP(w, r)

	if next.called {
		t.Fatal("expected next handler NOT to be called")
	}
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", w.Code)
	}
}

func TestDecompressRequestCorruptGzip(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not gzip at all")))
	r.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()

	next := &capturingNext{}
	decompressRequest(next).ServeHTTP(w, r)

	if next.called {
		t.Fatal("expected next handler NOT to be called")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
