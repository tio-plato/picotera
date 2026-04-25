package artifacts

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"unicode/utf8"

	"github.com/klauspost/compress/zstd"
)

type Payload struct {
	Method       string      `json:"method,omitempty"`
	URL          string      `json:"url,omitempty"`
	StatusCode   int         `json:"statusCode,omitempty"`
	Headers      http.Header `json:"headers"`
	Body         string      `json:"body"`
	BodyEncoding string      `json:"bodyEncoding"`
}

func BuildRequest(method, url string, header http.Header, body []byte) ([]byte, error) {
	p := Payload{
		Method:  method,
		URL:     url,
		Headers: normalizeHeader(header),
	}
	encodeBody(&p, body)
	return marshalAndCompress(&p)
}

func BuildResponse(statusCode int, header http.Header, body []byte) ([]byte, error) {
	p := Payload{
		StatusCode: statusCode,
		Headers:    normalizeHeader(header),
	}
	encodeBody(&p, body)
	return marshalAndCompress(&p)
}

func normalizeHeader(h http.Header) http.Header {
	if h == nil {
		return http.Header{}
	}
	return h
}

func encodeBody(p *Payload, body []byte) {
	if utf8.Valid(body) {
		p.Body = string(body)
		p.BodyEncoding = "utf8"
		return
	}
	p.Body = base64.StdEncoding.EncodeToString(body)
	p.BodyEncoding = "base64"
}

func marshalAndCompress(p *Payload) ([]byte, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf)
	if err != nil {
		return nil, fmt.Errorf("zstd writer: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("zstd write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("zstd close: %w", err)
	}
	return buf.Bytes(), nil
}
