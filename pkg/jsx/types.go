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

// ModelSummary is the JS-visible projection of the matched model row. Only
// the layer-specific annotation map and the canonical name are exposed; the
// merged map (model < provider < entry < apiKey) is exposed at the top level
// of each hook input as ctx.annotations.
type ModelSummary struct {
	Name        string            `json:"name"`
	Annotations map[string]string `json:"annotations"`
}

// Candidate is a JS-visible (provider, mpe) pair used by the sortProviders
// and beforeRequest/rewriteRequest hooks. Annotations carries the
// per-candidate merged map (model + provider + entry + apiKey, later wins).
// JS scripts can read it directly without re-running the merge.
type Candidate struct {
	Provider    ProviderSummary   `json:"provider"`
	MPE         CandidateMPE      `json:"mpe"`
	Annotations map[string]string `json:"annotations"`
}

// CandidateMPE is the JS-visible projection of a model_provider_endpoint row,
// extended with the resolved endpoint path so hooks can disambiguate
// candidates that share a provider but differ by endpoint.
type CandidateMPE struct {
	ModelName         string            `json:"modelName"`
	ProviderID        int32             `json:"providerId"`
	EndpointPath      string            `json:"endpointPath"`
	UpstreamModelName string            `json:"upstreamModelName"`
	Priority          int32             `json:"priority"`
	Annotations       map[string]string `json:"annotations"`
}

// RequestShape is the JS-visible shape of the incoming client request.
// Body is included as a parsed JSON value (json.RawMessage) only when the
// content-type is application/json and the body parses; otherwise omitted
// so JS scripts cannot read it.
type RequestShape struct {
	Path    string              `json:"path"`
	Method  string              `json:"method"`
	Headers map[string][]string `json:"headers"`
	Model   string              `json:"model"`
	// PathVars holds path variables extracted from the matched endpoint pattern
	// (e.g. {model} in /v1beta/models/{model}:generateContent). Omitted when
	// the endpoint has no variables.
	PathVars map[string]string `json:"pathVars,omitempty"`
	Body     json.RawMessage   `json:"body,omitempty"`
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

// EndpointSummary is the JS-visible projection of an endpoint. It is used for
// both database-backed gateway endpoints and unified route virtual endpoints.
type EndpointSummary struct {
	Name                string `json:"name"`
	Path                string `json:"path"`
	ModelPath           string `json:"modelPath"`
	CredentialsResolver int32  `json:"credentialsResolver"`
	EndpointType        int32  `json:"endpointType"`
}

// SortInput is the ctx passed to the sortProviders waterfall. Annotations is
// the request-level merge (model + apiKey); each candidate carries its own
// merged map under candidate.annotations.
type SortInput struct {
	Endpoint    EndpointSummary   `json:"endpoint"`
	Model       *ModelSummary     `json:"model"`
	Request     RequestShape      `json:"request"`
	Providers   []Candidate       `json:"providers"`
	ApiKey      *ApiKeySummary    `json:"apiKey"`
	Annotations map[string]string `json:"annotations"`
}

// LastError describes the outcome of the last upstream attempt, exposed to
// the beforeRequest hook.
type LastError struct {
	ProviderID int    `json:"providerId"`
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

// BeforeRequestInput is the ctx passed to the beforeRequest waterfall.
// Annotations is the chosen candidate's merged map (model + provider + entry
// + apiKey).
type BeforeRequestInput struct {
	Endpoint          EndpointSummary   `json:"endpoint"`
	Model             *ModelSummary     `json:"model"`
	Request           RequestShape      `json:"request"`
	Provider          ProviderSummary   `json:"provider"`
	MPE               CandidateMPE      `json:"mpe"`
	CurrentRetryCount int               `json:"currentRetryCount"`
	TotalAttemptCount int               `json:"totalAttemptCount"`
	LastError         *LastError        `json:"lastError"`
	ApiKey            *ApiKeySummary    `json:"apiKey"`
	Annotations       map[string]string `json:"annotations"`
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
// hook fires once between extractModel and resolveProviders, so MPE / provider
// layers are not yet known. Annotations is the model + apiKey merge.
type RewriteModelInput struct {
	Request     RequestShape      `json:"request"`
	Model       string            `json:"model"`
	ApiKey      *ApiKeySummary    `json:"apiKey"`
	Annotations map[string]string `json:"annotations"`
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

// RewriteInput is the ctx passed to the rewriteRequest waterfall. Annotations
// is the chosen candidate's merged map (same as BeforeRequestInput).
type RewriteInput struct {
	Endpoint          EndpointSummary     `json:"endpoint"`
	Model             *ModelSummary       `json:"model"`
	Provider          ProviderSummary     `json:"provider"`
	MPE               CandidateMPE        `json:"mpe"`
	CurrentRetryCount int                 `json:"currentRetryCount"`
	TotalAttemptCount int                 `json:"totalAttemptCount"`
	ClientRequest     RequestShape        `json:"clientRequest"`
	PendingRequest    PendingRequestShape `json:"pendingRequest"`
	ApiKey            *ApiKeySummary      `json:"apiKey"`
	Annotations       map[string]string   `json:"annotations"`
}

// OutboundProfile is the hook-visible profile used by beforeTransform to
// select the axonhub outbound transformer for a unified gateway attempt.
type OutboundProfile struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config"`
}

// BeforeTransformInput is the ctx passed to the beforeTransform waterfall.
// PendingRequest is the rewriteRequest result, still in source format, before
// the bridge converts it to the chosen upstream format.
type BeforeTransformInput struct {
	Endpoint          EndpointSummary     `json:"endpoint"`
	Model             *ModelSummary       `json:"model"`
	Provider          ProviderSummary     `json:"provider"`
	MPE               CandidateMPE        `json:"mpe"`
	CurrentRetryCount int                 `json:"currentRetryCount"`
	TotalAttemptCount int                 `json:"totalAttemptCount"`
	ClientRequest     RequestShape        `json:"clientRequest"`
	PendingRequest    PendingRequestShape `json:"pendingRequest"`
	ApiKey            *ApiKeySummary      `json:"apiKey"`
	Annotations       map[string]string   `json:"annotations"`
	SourceFormat      string              `json:"sourceFormat"`
	UpstreamFormat    string              `json:"upstreamFormat"`
	Stream            bool                `json:"stream"`
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
// Annotations is the decoded provider-level annotation map. ProviderModels is
// only populated for the rewriteProviderModels hook; dispatch hooks see it
// omitted.
type ProviderSummary struct {
	ID             int32                `json:"id"`
	Name           string               `json:"name"`
	Priority       int32                `json:"priority"`
	ProviderModels []ProviderModelEntry `json:"providerModels,omitempty"`
	Annotations    map[string]string    `json:"annotations"`
	Disabled       bool                 `json:"disabled"`
}

// RewriteProviderModelsInput is the ctx passed to the rewriteProviderModels
// waterfall. Annotations is the model + provider + apiKey merge (no entry
// layer at this point). The fetch-models flow has no model context, in which
// case Model is nil and the model layer contributes an empty map.
type RewriteProviderModelsInput struct {
	Provider         ProviderSummary   `json:"provider"`
	Model            *ModelSummary     `json:"model,omitempty"`
	UpstreamResponse json.RawMessage   `json:"upstreamResponse"`
	Annotations      map[string]string `json:"annotations"`
}
