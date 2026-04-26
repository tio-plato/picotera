package jsx

import (
	"encoding/json"
	"fmt"
)

// marshalToJSLiteral encodes v to JSON. The result is also a valid JS
// expression literal (because JSON is a subset of JS expression syntax).
func marshalToJSLiteral(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("jsx: marshal: %w", err)
	}
	return string(b), nil
}

// unmarshalJSON decodes a JSON string returned by JS into out. Empty/null
// strings are treated as no-op.
func unmarshalJSON(s string, out any) error {
	if s == "" || s == "null" {
		return nil
	}
	if err := json.Unmarshal([]byte(s), out); err != nil {
		return fmt.Errorf("jsx: unmarshal: %w", err)
	}
	return nil
}

// Candidate is a JS-visible (provider, mpe) pair used by the sortProviders
// and beforeRequest/rewriteRequest hooks. The Go side fills both fields with
// db.* row structs and they round-trip through JSON.
type Candidate struct {
	Provider any `json:"provider"`
	MPE      any `json:"mpe"`
}

// RequestShape is the JS-visible shape of the incoming client request.
type RequestShape struct {
	Path    string              `json:"path"`
	Method  string              `json:"method"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
	Model   string              `json:"model"`
}

// SortInput is the ctx passed to the sortProviders waterfall.
type SortInput struct {
	Endpoint  any          `json:"endpoint"`
	Model     any          `json:"model"`
	Request   RequestShape `json:"request"`
	Providers []Candidate  `json:"providers"`
}

// LastError describes the outcome of the last upstream attempt, exposed to
// the beforeRequest hook.
type LastError struct {
	ProviderID int    `json:"providerId"`
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

// BeforeRequestInput is the ctx passed to the beforeRequest waterfall.
type BeforeRequestInput struct {
	Endpoint          any          `json:"endpoint"`
	Model             any          `json:"model"`
	Request           RequestShape `json:"request"`
	Provider          any          `json:"provider"`
	MPE               any          `json:"mpe"`
	CurrentRetryCount int          `json:"currentRetryCount"`
	TotalAttemptCount int          `json:"totalAttemptCount"`
	LastError         *LastError   `json:"lastError"`
}

// BeforeRequestDecision is the JS-returned shape from the beforeRequest hook.
type BeforeRequestDecision struct {
	Next  bool `json:"next"`
	Delay int  `json:"delay"`
}

// UpstreamRequestShape mirrors the upstream request that will be sent
// after rewriteRequest applies overrides.
type UpstreamRequestShape struct {
	URL     string              `json:"url"`
	Method  string              `json:"method"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

// RewriteInput is the ctx passed to the rewriteRequest waterfall.
type RewriteInput struct {
	Endpoint          any                  `json:"endpoint"`
	Model             any                  `json:"model"`
	Request           RequestShape         `json:"request"`
	Provider          any                  `json:"provider"`
	MPE               any                  `json:"mpe"`
	CurrentRetryCount int                  `json:"currentRetryCount"`
	TotalAttemptCount int                  `json:"totalAttemptCount"`
	UpstreamRequest   UpstreamRequestShape `json:"upstreamRequest"`
	ClientRequest     RequestShape         `json:"clientRequest"`
}

// RewriteOutput holds JS-side overrides. Nil pointers / empty Body mean
// "leave the corresponding upstream request field unchanged."
type RewriteOutput struct {
	URL     *string              `json:"url"`
	Method  *string              `json:"method"`
	Headers *map[string][]string `json:"headers"`
	// Body is left as raw JSON to support both string and object inputs;
	// the SDK wrapper (sdk.js) ensures objects are JSON.stringify'd before
	// reaching Go, so this is always a JSON-encoded string when present.
	Body json.RawMessage `json:"body"`
}
