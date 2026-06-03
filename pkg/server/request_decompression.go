package server

import (
	"net/http"

	"picotera/pkg/errorx"
)

// decompressRequest is an HTTP middleware that decompresses request bodies
// encoded with gzip, br, or zstd. It runs before the body is read by any
// downstream handler (gateway flow, unified generation, artifact storage,
// project/model extraction, JS hooks, upstream forwarding).
//
// On successful decompression it strips Content-Encoding and Content-Length
// headers so downstream code — including identity passthrough — sees plain
// text with consistent headers.
func decompressRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoding, err := contentEncoding(r.Header)
		if err != nil {
			writeGatewayError(w, http.StatusUnsupportedMediaType, err.Error(), errorx.InvalidRequest.Error())
			return
		}
		if encoding == "" {
			next.ServeHTTP(w, r)
			return
		}

		decoded, err := decodedReadCloser(r.Body, encoding)
		if err != nil {
			writeGatewayError(w, http.StatusBadRequest, "failed to decode request body: "+err.Error(), errorx.InvalidRequest.Error())
			return
		}
		r.Body = decoded
		r.Header.Del("Content-Encoding")
		r.Header.Del("Content-Length")
		r.ContentLength = -1

		next.ServeHTTP(w, r)
	})
}
