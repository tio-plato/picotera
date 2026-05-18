package contract

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// SimulateEndpointSelector picks which endpoint to simulate against. Exactly
// one of Path / Format must be populated depending on Kind.
type SimulateEndpointSelector struct {
	Kind   string `json:"kind" enum:"path,unified" doc:"\"path\" picks a configured endpoint row; \"unified\" picks one of the five unified routes."`
	Path   string `json:"path,omitempty" doc:"Endpoint path; required when kind==path."`
	Format string `json:"format,omitempty" enum:"anthropicMessages,openaiChatCompletions,openaiResponses,geminiGenerateContent,geminiStreamGenerateContent" doc:"Unified source format; required when kind==unified."`
}

type SimulateDispatchRequest struct {
	Body struct {
		Endpoint SimulateEndpointSelector `json:"endpoint"`
		ApiKeyID int32                    `json:"apiKeyId" minimum:"1"`
		Model    string                   `json:"model" minLength:"1"`
		Body     string                   `json:"body" doc:"Raw JSON request body string. Empty allowed (body omitted from hook context)."`
		PathVars map[string]string        `json:"pathVars,omitempty" doc:"Optional path variable map (used when the endpoint path contains {name} tokens)."`
	}
}

// SimulateProviderSummary mirrors jsx.ProviderSummary minus ProviderModels.
type SimulateProviderSummary struct {
	ID          int32             `json:"id"`
	Name        string            `json:"name"`
	Priority    int32             `json:"priority"`
	Annotations map[string]string `json:"annotations"`
	Disabled    bool              `json:"disabled"`
}

// SimulateMPE mirrors jsx.CandidateMPE.
type SimulateMPE struct {
	ModelName         string            `json:"modelName"`
	ProviderID        int32             `json:"providerId"`
	EndpointPath      string            `json:"endpointPath"`
	UpstreamModelName string            `json:"upstreamModelName"`
	Priority          int32             `json:"priority"`
	Annotations       map[string]string `json:"annotations"`
}

// SimulateOutboundProfile is the resolved bridge adapter + config for a
// bridged candidate, after running beforeTransform. Only populated when
// Bridged is true.
type SimulateOutboundProfile struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config"`
}

// SimulateCandidate is one entry in the simulator's ranked candidate list.
type SimulateCandidate struct {
	Provider          SimulateProviderSummary  `json:"provider"`
	MPE               SimulateMPE              `json:"mpe"`
	MergedAnnotations map[string]string        `json:"mergedAnnotations"`
	UpstreamFormat    string                   `json:"upstreamFormat"`
	Bridged           bool                     `json:"bridged"`
	OutboundProfile   *SimulateOutboundProfile `json:"outboundProfile,omitempty"`
}

// SimulateLogEntry is one captured console.* entry. ts is RFC3339.
type SimulateLogEntry struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Ts      string `json:"ts"`
}

type SimulateDispatchResponse struct {
	Body struct {
		OriginalModel string              `json:"originalModel"`
		ResolvedModel string              `json:"resolvedModel"`
		SourceFormat  string              `json:"sourceFormat"`
		Stream        bool                `json:"stream"`
		Candidates    []SimulateCandidate `json:"candidates"`
		Logs          []SimulateLogEntry  `json:"logs"`
	}
}

var OperationSimulateDispatch = huma.Operation{
	OperationID: "simulateDispatch",
	Method:      http.MethodPost,
	Path:        "/simulate/dispatch",
	Summary:     "Simulate dispatch and return ranked candidates",
}
