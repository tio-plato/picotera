package llmbridgeimpl

import (
	"context"
	"fmt"
	"net/http"

	"picotera/pkg/llmbridge"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// BridgeRequest converts a source-format request body into the upstream
// format. The pendingURL is just used to feed Gemini Inbound, which extracts
// the model name and stream marker from the URL path; the URL itself is
// otherwise irrelevant — the caller already built the outgoing request and
// only swaps in the returned bytes.
//
// When src and dst are equal the call is identity (returns body unchanged).
//
// Returns the upstream-format body bytes and the Content-Type that should be
// set on the outgoing request.
func BridgeRequest(ctx context.Context, src, dst llmbridge.Format, body []byte, headers http.Header, pendingURL string, profile llmbridge.OutboundProfile) ([]byte, string, error) {
	if src == llmbridge.FormatUnknown || dst == llmbridge.FormatUnknown {
		return nil, "", fmt.Errorf("llmbridge: bridge with unknown format (src=%s dst=%s)", src, dst)
	}
	if src == dst {
		return body, contentTypeOrDefault(headers), nil
	}

	llmReq, err := parseSourceRequest(ctx, src, body, headers, pendingURL)
	if err != nil {
		return nil, "", fmt.Errorf("llmbridge: parse source %s request: %w", src, err)
	}

	out, err := outboundFor(dst, profile)
	if err != nil {
		return nil, "", err
	}
	upReq, err := out.TransformRequest(ctx, llmReq)
	if err != nil {
		return nil, "", fmt.Errorf("llmbridge: build %s request: %w", dst, err)
	}
	ct := upReq.Headers.Get("Content-Type")
	if ct == "" {
		ct = "application/json"
	}
	return upReq.Body, ct, nil
}

// BridgeNonStream converts a non-streaming upstream JSON response body into
// source-format JSON. Identity when src == upstream.
func BridgeNonStream(ctx context.Context, src, upstream llmbridge.Format, upstreamBody []byte, upstreamHeaders http.Header, profile llmbridge.OutboundProfile) ([]byte, string, error) {
	if src == llmbridge.FormatUnknown || upstream == llmbridge.FormatUnknown {
		return nil, "", fmt.Errorf("llmbridge: bridge non-stream with unknown format (src=%s upstream=%s)", src, upstream)
	}
	if src == upstream {
		return upstreamBody, contentTypeOrDefault(upstreamHeaders), nil
	}

	out, err := outboundFor(upstream, profile)
	if err != nil {
		return nil, "", err
	}
	llmResp, err := out.TransformResponse(ctx, &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers:    upstreamHeaders,
		Body:       upstreamBody,
	})
	if err != nil {
		return nil, "", fmt.Errorf("llmbridge: parse %s response: %w", upstream, err)
	}

	in, err := inboundFor(src)
	if err != nil {
		return nil, "", err
	}
	cliResp, err := in.TransformResponse(ctx, llmResp)
	if err != nil {
		return nil, "", fmt.Errorf("llmbridge: write %s response: %w", src, err)
	}
	ct := cliResp.Headers.Get("Content-Type")
	if ct == "" {
		ct = "application/json"
	}
	return cliResp.Body, ct, nil
}

// parseSourceRequest builds an *llm.Request from a source-format body. We
// supply the synthesized URL/path to Gemini Inbound (which derives model and
// stream from the path) and a Content-Type so the strict transformers don't
// reject the body.
func parseSourceRequest(ctx context.Context, src llmbridge.Format, body []byte, headers http.Header, pendingURL string) (*llm.Request, error) {
	in, err := inboundFor(src)
	if err != nil {
		return nil, err
	}
	hdr := headers.Clone()
	if hdr == nil {
		hdr = http.Header{}
	}
	if hdr.Get("Content-Type") == "" {
		hdr.Set("Content-Type", "application/json")
	}

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     pendingURL,
		Path:    pendingURL,
		Headers: hdr,
		Body:    body,
	}
	return in.TransformRequest(ctx, httpReq)
}

// contentTypeOrDefault returns the Content-Type from headers, or a JSON
// default when the header is absent.
func contentTypeOrDefault(h http.Header) string {
	if h == nil {
		return "application/json"
	}
	if ct := h.Get("Content-Type"); ct != "" {
		return ct
	}
	return "application/json"
}
