package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

func TestDecodedReadCloser(t *testing.T) {
	plain := []byte("hello compressed upstream")
	cases := []struct {
		name     string
		encoding string
		body     []byte
	}{
		{name: "none", encoding: "", body: plain},
		{name: "gzip", encoding: "gzip", body: gzipBytes(t, plain)},
		{name: "br", encoding: "br", body: brotliBytes(t, plain)},
		{name: "zstd", encoding: "zstd", body: zstdBytes(t, plain)},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rc, err := decodedReadCloser(io.NopCloser(bytes.NewReader(tt.body)), tt.encoding)
			if err != nil {
				t.Fatalf("decodedReadCloser: %v", err)
			}
			got, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			if string(got) != string(plain) {
				t.Fatalf("decoded body = %q, want %q", got, plain)
			}
			if err := rc.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
		})
	}
}

func TestDecodedReadCloserRejectsUnsupportedEncoding(t *testing.T) {
	cases := []string{"deflate", "gzip, br", "GZIP", " gzip", "gzip "}
	for _, encoding := range cases {
		t.Run(encoding, func(t *testing.T) {
			_, err := decodedReadCloser(io.NopCloser(bytes.NewReader(nil)), encoding)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestDecodedBodyRejectsMultipleContentEncodingHeaders(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"gzip", "br"}},
		Body:   io.NopCloser(bytes.NewReader(nil)),
	}
	_, err := decodedBody(resp)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodedBodyReturnsRawReaderWithoutEncoding(t *testing.T) {
	body := io.NopCloser(bytes.NewReader([]byte("raw")))
	resp := &http.Response{Header: http.Header{}, Body: body}
	decoded, err := decodedBody(resp)
	if err != nil {
		t.Fatalf("decodedBody: %v", err)
	}
	if decoded.Body != body {
		t.Fatal("expected original body")
	}
	if decoded.Encoding != "" || decoded.Compressed {
		t.Fatalf("unexpected decoded metadata: %+v", decoded)
	}
}

func TestDecodedReadCloserCloseClosesUnderlyingReader(t *testing.T) {
	src := &trackingReadCloser{Reader: bytes.NewReader(brotliBytes(t, []byte("close me")))}
	rc, err := decodedReadCloser(src, "br")
	if err != nil {
		t.Fatalf("decodedReadCloser: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !src.closed {
		t.Fatal("expected underlying reader to be closed")
	}
}

func gzipBytes(t *testing.T, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(body); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func brotliBytes(t *testing.T, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := brotli.NewWriter(&buf)
	if _, err := w.Write(body); err != nil {
		t.Fatalf("brotli write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("brotli close: %v", err)
	}
	return buf.Bytes()
}

func zstdBytes(t *testing.T, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("zstd writer: %v", err)
	}
	if _, err := w.Write(body); err != nil {
		t.Fatalf("zstd write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zstd close: %v", err)
	}
	return buf.Bytes()
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestDecodedBodyRejectsCommaEncoding(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"gzip, br"}},
		Body:   io.NopCloser(bytes.NewReader(nil)),
	}
	_, err := decodedBody(resp)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodedBodyMetadataForCompressedResponse(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"gzip"}},
		Body:   io.NopCloser(bytes.NewReader(gzipBytes(t, []byte("ok")))),
	}
	decoded, err := decodedBody(resp)
	if err != nil {
		t.Fatalf("decodedBody: %v", err)
	}
	defer decoded.Body.Close()
	if !reflect.DeepEqual([]any{decoded.Encoding, decoded.Compressed}, []any{"gzip", true}) {
		t.Fatalf("unexpected metadata: %+v", decoded)
	}
}
