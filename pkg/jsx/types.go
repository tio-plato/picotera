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
// and beforeRequest/rewriteRequest hooks. The Go side constructs typed
// literals; values round-trip through JSON when JS hooks return modified
// candidate arrays.
type Candidate struct {
	Provider ProviderSummary `json:"provider"`
	MPE      CandidateMPE    `json:"mpe"`
}

// CandidateMPE is the JS-visible projection of a model_provider_endpoint row,
// extended with the resolved endpoint path so hooks can disambiguate
// candidates that share a provider but differ by endpoint.
type CandidateMPE struct {
	ModelName         string          `json:"modelName"`
	ProviderID        int32           `json:"providerId"`
	EndpointPath      string          `json:"endpointPath"`
	UpstreamModelName string          `json:"upstreamModelName"`
	Priority          int32           `json:"priority"`
	Annotations       json.RawMessage `json:"annotations,omitempty"`
}

// RequestShape is the JS-visible shape of the incoming client request.
// Body is included as a parsed JSON value (json.RawMessage) only when the
// content-type is application/json and the body parses; otherwise omitted
// so JS scripts cannot read it.
type RequestShape struct {
	Path     string              `json:"path"`
	Method   string              `json:"method"`
	Headers  map[string][]string `json:"headers"`
	Model    string              `json:"model"`
	// PathVars holds path variables extracted from the matched endpoint pattern
	// (e.g. {model} in /v1beta/models/{model}:generateContent). Omitted when
	// the endpoint has no variables.
	PathVars map[string]string   `json:"pathVars,omitempty"`
	Body     json.RawMessage     `json:"body,omitempty"`
}

// ApiKeySummary is the JS-visible shape of the API key that authorized the
// inbound request. The raw key string is intentionally omitted so scripts
// cannot leak it; only metadata is exposed.
type ApiKeySummary struct {
	ID          int32             `json:"id"`
	Name        string            `json:"name"`
	Annotations map[string]string `json:"annotations"`
	Disabled    bool              `json:"disabled"`
}

// SortInput is the ctx passed to the sortProviders waterfall.
type SortInput struct {
	Endpoint  any            `json:"endpoint"`
	Model     any            `json:"model"`
	Request   RequestShape   `json:"request"`
	Providers []Candidate    `json:"providers"`
	ApiKey    *ApiKeySummary `json:"apiKey"`
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
	Endpoint          any             `json:"endpoint"`
	Model             any             `json:"model"`
	Request           RequestShape    `json:"request"`
	Provider          ProviderSummary `json:"provider"`
	MPE               CandidateMPE    `json:"mpe"`
	CurrentRetryCount int             `json:"currentRetryCount"`
	TotalAttemptCount int             `json:"totalAttemptCount"`
	LastError         *LastError      `json:"lastError"`
	ApiKey            *ApiKeySummary  `json:"apiKey"`
}

// BeforeRequestDecision is the JS-returned shape from the beforeRequest hook.
// UpstreamModel, when non-empty, replaces the model name written into the
// upstream request body for this attempt.
type BeforeRequestDecision struct {
	Next          bool   `json:"next"`
	Delay         int    `json:"delay"`
	UpstreamModel string `json:"upstreamModel"`
}

// RewriteModelInput is the ctx passed to the rewriteModel waterfall. The
// hook fires once between extractModel and resolveProviders, so only the
// raw client request snapshot is in scope.
type RewriteModelInput struct {
	Request RequestShape   `json:"request"`
	Model   string         `json:"model"`
	ApiKey  *ApiKeySummary `json:"apiKey"`
}

// PendingRequestShape mirrors the upstream request that is about to be sent.
// Body is a parsed JSON value (json.RawMessage) only when content-type is
// application/json; otherwise omitted, and the Go side keeps using the
// pre-hook bytes verbatim.
type PendingRequestShape struct {
	URL     string              `json:"url"`
	Method  string              `json:"method"`
	Headers map[string][]string `json:"headers"`
	Body    json.RawMessage     `json:"body,omitempty"`
}

// RewriteInput is the ctx passed to the rewriteRequest waterfall.
type RewriteInput struct {
	Endpoint          any                 `json:"endpoint"`
	Model             any                 `json:"model"`
	Provider          ProviderSummary     `json:"provider"`
	MPE               CandidateMPE        `json:"mpe"`
	CurrentRetryCount int                 `json:"currentRetryCount"`
	TotalAttemptCount int                 `json:"totalAttemptCount"`
	ClientRequest     RequestShape        `json:"clientRequest"`
	PendingRequest    PendingRequestShape `json:"pendingRequest"`
	ApiKey            *ApiKeySummary      `json:"apiKey"`
}

// ProviderModelEntry mirrors the JSON shape of contract.ProviderModelEntry.
// Declared here to avoid a reverse dependency from jsx → contract.
type ProviderModelEntry struct {
	Model             string            `json:"model"`
	UpstreamModelName string            `json:"upstreamModelName,omitempty"`
	Endpoints         []string          `json:"endpoints,omitempty"`
	Priority          int32             `json:"priority,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Disabled          bool              `json:"disabled,omitempty"`
}

// ProviderSummary is the shape of ctx.provider in rewriteProviderModels and
// the Provider field on Candidate (and the *Input shapes that copy it).
// Credentials are intentionally omitted for security.
//
// Annotations is the raw JSONB blob from the provider row, exposed to JS as
// the parsed object. ProviderModels is only populated for the
// rewriteProviderModels hook; dispatch hooks see it omitted.
type ProviderSummary struct {
	ID             int32                `json:"id"`
	Name           string               `json:"name"`
	Priority       int32                `json:"priority"`
	ProviderModels []ProviderModelEntry `json:"providerModels,omitempty"`
	Annotations    json.RawMessage      `json:"annotations,omitempty"`
	Disabled       bool                 `json:"disabled"`
}

// RewriteProviderModelsInput is the ctx passed to the rewriteProviderModels waterfall.
type RewriteProviderModelsInput struct {
	Provider         ProviderSummary `json:"provider"`
	EndpointPath     string          `json:"endpointPath"`
	UpstreamResponse json.RawMessage `json:"upstreamResponse"`
}
