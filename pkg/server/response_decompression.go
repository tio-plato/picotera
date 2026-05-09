package server

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

type decodedResponseBody struct {
	Body       io.ReadCloser
	Encoding   string
	Compressed bool
}

func decodedBody(resp *http.Response) (*decodedResponseBody, error) {
	encoding, err := contentEncoding(resp.Header)
	if err != nil {
		return nil, err
	}
	if encoding == "" {
		return &decodedResponseBody{Body: resp.Body}, nil
	}
	body, err := decodedReadCloser(resp.Body, encoding)
	if err != nil {
		return nil, err
	}
	return &decodedResponseBody{
		Body:       body,
		Encoding:   encoding,
		Compressed: true,
	}, nil
}

func contentEncoding(header http.Header) (string, error) {
	values, ok := header["Content-Encoding"]
	if !ok || len(values) == 0 {
		return "", nil
	}
	if len(values) != 1 {
		return "", fmt.Errorf("unsupported content encoding: %q", values)
	}
	encoding := values[0]
	switch encoding {
	case "", "gzip", "br", "zstd":
		return encoding, nil
	default:
		return "", fmt.Errorf("unsupported content encoding: %q", encoding)
	}
}

func decodedReadCloser(src io.ReadCloser, encoding string) (io.ReadCloser, error) {
	switch encoding {
	case "":
		return src, nil
	case "gzip":
		return gzip.NewReader(src)
	case "br":
		return &readerWithCloser{r: brotli.NewReader(src), c: src}, nil
	case "zstd":
		decoder, err := zstd.NewReader(src)
		if err != nil {
			return nil, err
		}
		return &zstdReadCloser{decoder: decoder, src: src}, nil
	default:
		return nil, fmt.Errorf("unsupported content encoding: %q", encoding)
	}
}

type zstdReadCloser struct {
	decoder *zstd.Decoder
	src     io.Closer
}

func (rc *zstdReadCloser) Read(p []byte) (int, error) {
	return rc.decoder.Read(p)
}

func (rc *zstdReadCloser) Close() error {
	err := rc.src.Close()
	rc.decoder.Close()
	return err
}
