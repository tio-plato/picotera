//go:build !wasip1

package llmbridge

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

type Config struct {
	PoolSize    int
	WASMPath    string
	CacheDir    string
	RuntimeMode string
}

type Bridge interface {
	Enabled() bool
	Close(ctx context.Context) error
	BridgeRequest(ctx context.Context, src, dst Format, body []byte, headers http.Header, pendingURL string, profile OutboundProfile) ([]byte, string, error)
	BridgeNonStream(ctx context.Context, src, upstream Format, upstreamBody []byte, upstreamHeaders http.Header, profile OutboundProfile) ([]byte, string, error)
	BridgeStream(ctx context.Context, src, upstream Format, upstreamBody io.ReadCloser, upstreamCT string, profile OutboundProfile) (io.ReadCloser, error)
	AggregateStream(ctx context.Context, format Format, contentType string, body []byte, profile OutboundProfile) ([]byte, error)
}

var errDisabled = fmt.Errorf("llmbridge: wasm module is not configured")

type disabledBridge struct{}

func New(ctx context.Context, cfg Config) (Bridge, error) {
	return newWASMBridge(ctx, cfg)
}

func (disabledBridge) Enabled() bool {
	return false
}

func (disabledBridge) Close(ctx context.Context) error {
	return nil
}

func (disabledBridge) BridgeRequest(ctx context.Context, src, dst Format, body []byte, headers http.Header, pendingURL string, profile OutboundProfile) ([]byte, string, error) {
	if src == dst {
		return body, contentTypeOrDefault(headers), nil
	}
	return nil, "", errDisabled
}

func (disabledBridge) BridgeNonStream(ctx context.Context, src, upstream Format, upstreamBody []byte, upstreamHeaders http.Header, profile OutboundProfile) ([]byte, string, error) {
	if src == upstream {
		return upstreamBody, contentTypeOrDefault(upstreamHeaders), nil
	}
	return nil, "", errDisabled
}

func (disabledBridge) BridgeStream(ctx context.Context, src, upstream Format, upstreamBody io.ReadCloser, upstreamCT string, profile OutboundProfile) (io.ReadCloser, error) {
	if src == upstream {
		return upstreamBody, nil
	}
	_ = upstreamBody.Close()
	return nil, errDisabled
}

func (disabledBridge) AggregateStream(ctx context.Context, format Format, contentType string, body []byte, profile OutboundProfile) ([]byte, error) {
	return nil, errDisabled
}

func contentTypeOrDefault(h http.Header) string {
	if h == nil {
		return "application/json"
	}
	if ct := h.Get("Content-Type"); ct != "" {
		return ct
	}
	return "application/json"
}
