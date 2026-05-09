package server

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

func TestDecodedInternalResponseReaderSplitsCompressedResponse(t *testing.T) {
	plain := []byte(`data: {"ok":true}` + "\n\n")
	compressed := gzipBytes(t, plain)
	resp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"gzip"}},
		Body:   io.NopCloser(bytes.NewReader(compressed)),
	}
	var client bytes.Buffer

	internal, err := decodedInternalResponseReader(resp, &client)
	if err != nil {
		t.Fatalf("decodedInternalResponseReader: %v", err)
	}
	if err := internal.StartClientWrite(); err != nil {
		t.Fatalf("StartClientWrite: %v", err)
	}
	got, err := io.ReadAll(internal.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	closeDecodedInternalResponseReader(internal.Body, resp)

	if !bytes.Equal(got, plain) {
		t.Fatalf("internal body = %q, want %q", got, plain)
	}
	if !bytes.Equal(client.Bytes(), compressed) {
		t.Fatalf("client body was not raw compressed bytes")
	}
}

func TestDecodedInternalResponseReaderLeavesRawResponseSinglePath(t *testing.T) {
	plain := []byte("raw upstream")
	resp := &http.Response{
		Header: http.Header{},
		Body:   io.NopCloser(bytes.NewReader(plain)),
	}
	var client bytes.Buffer

	internal, err := decodedInternalResponseReader(resp, &client)
	if err != nil {
		t.Fatalf("decodedInternalResponseReader: %v", err)
	}
	if internal.Body != resp.Body {
		t.Fatal("expected unencoded response to use original body")
	}
	got, err := io.ReadAll(internal.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	closeDecodedInternalResponseReader(internal.Body, resp)

	if !bytes.Equal(got, plain) {
		t.Fatalf("internal body = %q, want %q", got, plain)
	}
	if client.Len() != 0 {
		t.Fatalf("unencoded helper should not write client copy, got %q", client.Bytes())
	}
}
