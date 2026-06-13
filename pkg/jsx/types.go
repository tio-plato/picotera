package jsx

import "encoding/json"

// EndpointSummary is the JS-visible projection of an endpoint (ctx.endpoint).
// Used for both database-backed gateway endpoints and unified route virtual
// endpoints. EndpointType is the format enum — distinct from ctx.endpointType
// ("gateway" | "unified"), which is the routing shape.
type EndpointSummary struct {
	Name                string `json:"name"`
	Path                string `json:"path"`
	ModelPath           string `json:"modelPath"`
	CredentialsResolver int32  `json:"credentialsResolver"`
	EndpointType        int32  `json:"endpointType"`
}

// ModelSummary is the JS-visible projection of the routed model (ctx.routedModel).
// Only the layer-specific annotation map and the canonical name are exposed;
// the merged convenience map lives at ctx.annotations.
type ModelSummary struct {
	Name        string            `json:"name"`
	Annotations map[string]string `json:"annotations"`
}

// RequestShape is the JS-visible shape of the incoming client request
// (ctx.request). It carries no body field: ctx.request.body is installed
// separately by the session as a lazy Proxy over the Go-side body tree (see
// Session.SetClientBody), so a large body a hook never reads never crosses into
// QuickJS.
type RequestShape struct {
	Path    string              `json:"path"`
	Method  string              `json:"method"`
	Headers map[string][]string `json:"headers"`
	Model   string              `json:"model"`
	// PathVars holds path variables extracted from the matched endpoint pattern
	// (e.g. {model} in /v1beta/models/{model}:generateContent). Omitted when
	// the endpoint has no variables.
	PathVars map[string]string `json:"pathVars,omitempty"`
}

// ApiKeySummary is the JS-visible shape of the API key that authorized the
// inbound request (ctx.apiKey). The raw key string is intentionally omitted.
type ApiKeySummary struct {
	ID          int32             `json:"id"`
	Name        string            `json:"name"`
	Annotations map[string]string `json:"annotations"`
	Disabled    bool              `json:"disabled"`
}

// ProviderSummary is the JS-visible shape of a provider (ctx.provider and
// CandidateView.Provider). Credentials are intentionally omitted for security.
type ProviderSummary struct {
	ID          int32             `json:"id"`
	Name        string            `json:"name"`
	Priority    int32             `json:"priority"`
	Annotations map[string]string `json:"annotations"`
	Disabled    bool              `json:"disabled"`
}

// ProviderModel is the resolved single-endpoint model configuration for the
// current candidate (ctx.providerModel and CandidateView.ProviderModel).
// Endpoint is the resolved single endpoint path (singular) — distinct from
// ProviderModelEntry, the configuration item with an Endpoints (plural) list.
type ProviderModel struct {
	Name              string            `json:"name"`
	UpstreamModelName string            `json:"upstreamModelName"`
	Endpoint          string            `json:"endpoint"`
	Priority          int32             `json:"priority"`
	Annotations       map[string]string `json:"annotations"`
	UpstreamFormat    string            `json:"upstreamFormat"` // only meaningful for unified
}

// LastError describes the outcome of the previous upstream attempt
// (ctx.attempt.lastError). Null on the first attempt.
type LastError struct {
	ProviderID int    `json:"providerId"`
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

// AttemptState is the per-attempt state (ctx.attempt). Re-patched before every
// attempt.
type AttemptState struct {
	CurrentRetryCount int        `json:"currentRetryCount"`
	TotalAttemptCount int        `json:"totalAttemptCount"`
	LastError         *LastError `json:"lastError"`
}

// CandidateView is the waterfall value element for the sortProviders hook: a
// (provider, providerModel) pair plus the per-candidate merged annotation map.
type CandidateView struct {
	Provider      ProviderSummary   `json:"provider"`
	ProviderModel ProviderModel     `json:"providerModel"`
	Annotations   map[string]string `json:"annotations"`
}

// BeforeRequestDecision is the waterfall value for the beforeRequest hook.
// UpstreamModel, when non-empty, replaces the upstream model name for this
// attempt.
type BeforeRequestDecision struct {
	Next          bool   `json:"next"`
	Delay         int    `json:"delay"`
	UpstreamModel string `json:"upstreamModel"`
}

// PendingRequestShape is the waterfall value for the rewriteRequest hook: the
// upstream request about to be sent. Body never crosses the JSON boundary
// (json:"-"): on input it is always nil — the session installs pending.body as
// a lazy Proxy over the Go-side body tree — and on output it carries the final
// upstream body bytes directly (nil means "fall back to the pre-hook bytes").
type PendingRequestShape struct {
	URL     string              `json:"url"`
	Method  string              `json:"method"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"-"`
}

// UpstreamErrorView is the waterfall input for the afterUpstreamError hook. It
// describes an upstream attempt that just failed. StatusCode is the upstream's
// original HTTP status (0 for connection/build failures with no response).
// Streamed is true when the client response already started streaming (an
// in-stream SSE error), in which case Break in the decision is ignored.
type UpstreamErrorView struct {
	Break      bool   `json:"break"`
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Streamed   bool   `json:"streamed"`
}

// AfterUpstreamErrorDecision is the waterfall output for the afterUpstreamError
// hook. Break (only honored when streamed=false) interrupts the gateway request
// and writes a downstream response: StatusCode<=0 follows the upstream's
// original status; Message=="" follows the upstream's original body.
type AfterUpstreamErrorDecision struct {
	Break      bool   `json:"break"`
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

// OutboundProfile is the waterfall value for the beforeTransform hook: the
// axonhub outbound transformer selection for a unified gateway attempt.
type OutboundProfile struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config"`
}

// ProviderModelEntry is the waterfall value element for the
// rewriteProviderModels hook (the configuration item, Endpoints plural). It
// mirrors the JSON shape of contract.ProviderModelEntry. Declared here to
// avoid a reverse dependency from jsx → contract.
type ProviderModelEntry struct {
	Model             string            `json:"model"`
	UpstreamModelName string            `json:"upstreamModelName,omitempty"`
	Endpoints         []string          `json:"endpoints,omitempty"`
	Priority          int32             `json:"priority,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Disabled          bool              `json:"disabled,omitempty"`
}

// ContextPatch is the Go-side patch applied to globalThis.ctx. Only non-nil
// pointer fields are shallow-merged (Object.assign) onto the persistent ctx,
// preserving any custom fields the scripts attached.
type ContextPatch struct {
	EndpointType     *string            `json:"endpointType,omitempty"`
	Endpoint         *EndpointSummary   `json:"endpoint,omitempty"`
	RequestModel     *string            `json:"requestModel,omitempty"`
	RoutedModel      *ModelSummary      `json:"routedModel,omitempty"`
	Request          *RequestShape      `json:"request,omitempty"`
	ApiKey           *ApiKeySummary     `json:"apiKey,omitempty"`
	Provider         *ProviderSummary   `json:"provider,omitempty"`
	ProviderModel    *ProviderModel     `json:"providerModel,omitempty"`
	Attempt          *AttemptState      `json:"attempt,omitempty"`
	Annotations      *map[string]string `json:"annotations,omitempty"`
	Stream           *bool              `json:"stream,omitempty"`
	SourceFormat     *string            `json:"sourceFormat,omitempty"`
	Format           *string            `json:"format,omitempty"`
	UpstreamResponse json.RawMessage    `json:"upstreamResponse,omitempty"` // only rewriteProviderModels
}
