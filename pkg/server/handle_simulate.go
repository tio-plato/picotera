package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"

	"picotera/pkg/annotations"
	"picotera/pkg/contract"
	"picotera/pkg/db"
	"picotera/pkg/errorx"
	"picotera/pkg/jsx"
	"picotera/pkg/llmbridge"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/rs/xid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// handleSimulateDispatch runs the first half of the gateway pipeline (endpoint
// resolve → rewriteModel → candidate resolution → sortProviders) and returns
// the ranked candidate list without sending any upstream request or recording
// a request row.
func (s *Server) handleSimulateDispatch(ctx context.Context, req *contract.SimulateDispatchRequest) (*contract.SimulateDispatchResponse, error) {
	in := req.Body

	// 1. Parse body bytes.
	var bodyBytes []byte
	if in.Body != "" {
		if !json.Valid([]byte(in.Body)) {
			return nil, huma.Error400BadRequest("body is not valid JSON", errorx.InvalidRequest)
		}
		bodyBytes = []byte(in.Body)
	}

	if in.Model == "" {
		return nil, huma.Error400BadRequest("model is required", errorx.ModelNotFound)
	}

	// 2. Load API key.
	apiKeyRow, err := s.queries.GetApiKey(ctx, in.ApiKeyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("api key not found", errorx.RequestNotFound)
		}
		return nil, huma.Error500InternalServerError("failed to query api key", errorx.InternalError)
	}
	if apiKeyRow.Disabled {
		return nil, huma.Error403Forbidden("api key disabled", errorx.Forbidden)
	}
	apiKeyJS := apiKeySummaryFromRow(&apiKeyRow)
	apiKeyAnno := apiKeyJS.Annotations

	// 3. Resolve endpoint (path table or virtual unified).
	var (
		endpoint    db.Endpoint
		srcFormat   llmbridge.Format
		isUnified   bool
		pathVars    = map[string]string{}
	)
	switch in.Endpoint.Kind {
	case "path":
		if in.Endpoint.Path == "" {
			return nil, huma.Error400BadRequest("endpoint.path is required when kind=path", errorx.InvalidRequest)
		}
		if in.Endpoint.Format != "" {
			return nil, huma.Error400BadRequest("endpoint.format must be empty when kind=path", errorx.InvalidRequest)
		}
		ep, vars, ok, lerr := s.endpointRouter.Match(ctx, in.Endpoint.Path)
		if lerr != nil {
			return nil, huma.Error500InternalServerError("failed to query endpoint", errorx.InternalError)
		}
		if !ok {
			return nil, huma.Error404NotFound("route not found", errorx.RouteNotFound)
		}
		endpoint = ep
		// Path variables come from the simulator's input, overriding any
		// vars the router extracted from matching the literal path (which
		// for a literal lookup are usually empty).
		maps.Copy(pathVars, vars)
		maps.Copy(pathVars, in.PathVars)
		srcFormat = formatForEndpointType(endpoint.EndpointType)
	case "unified":
		if in.Endpoint.Format == "" {
			return nil, huma.Error400BadRequest("endpoint.format is required when kind=unified", errorx.InvalidRequest)
		}
		if in.Endpoint.Path != "" {
			return nil, huma.Error400BadRequest("endpoint.path must be empty when kind=unified", errorx.InvalidRequest)
		}
		f, ferr := simulateFormatFromString(in.Endpoint.Format)
		if ferr != nil {
			return nil, huma.Error400BadRequest(ferr.Error(), errorx.InvalidRequest)
		}
		srcFormat = f
		isUnified = true
		endpoint = db.Endpoint{
			Name:                "(unified)",
			Path:                unifiedRoutePath(f),
			ModelPath:           "",
			CredentialsResolver: contract.CredentialsResolver_Unknown,
			EndpointType:        sourceEndpointType(f),
		}
	default:
		return nil, huma.Error400BadRequest("endpoint.kind must be \"path\" or \"unified\"", errorx.InvalidRequest)
	}

	// 4. Determine streaming flag.
	var streaming bool
	switch {
	case isUnified && srcFormat == llmbridge.FormatGeminiGenerateContent:
		streaming = false
	case isUnified && srcFormat == llmbridge.FormatGeminiStreamGenerateContent:
		streaming = true
	default:
		streaming = gjson.GetBytes(bodyBytes, "stream").Bool()
	}

	// 5. Build JSX session.
	session, err := s.jsxEngine.NewSession(ctx, "sim-"+xid.New().String())
	if err != nil {
		return nil, huma.Error502BadGateway("failed to load js hooks: "+err.Error(), errorx.UpstreamError)
	}
	defer session.Close()

	// 6. Run rewriteModel once.
	originalModel := in.Model
	modelName := in.Model
	modelAnno := s.fetchModelAnnotations(ctx, modelName)

	// Synthesize the request shape JS will see. We use a fake header map
	// containing only Content-Type so jsonBodyOrNil treats bodyBytes as the
	// JS-visible body.
	jsHeaders := http.Header{}
	if len(bodyBytes) > 0 {
		jsHeaders.Set("Content-Type", "application/json")
	}
	clientReq := jsx.RequestShape{
		Path:     endpoint.Path,
		Method:   http.MethodPost,
		Headers:  mapLowerKeys(jsHeaders),
		Model:    modelName,
		PathVars: pathVars,
		Body:     jsonBodyOrNil(jsHeaders, bodyBytes),
	}

	newModel, err := session.RunRewriteModelHook(jsx.RewriteModelInput{
		Request:     clientReq,
		Model:       originalModel,
		ApiKey:      apiKeyJS,
		Annotations: annotations.Merge(modelAnno, apiKeyAnno),
	}, modelName)
	if err != nil {
		return nil, hookHumaError(err)
	}
	if newModel != "" && newModel != modelName {
		// For unified Gemini routes the body has no model field; setUnifiedModel
		// handles the no-op. For path endpoints we use sjson directly because the
		// body's model location is not known (could be anywhere via ModelPath).
		// To stay consistent with production, we mirror the gateway behavior:
		// path endpoints rewrite body.model unconditionally (sjson is a no-op
		// on absent paths), unified routes use setUnifiedModel.
		if len(bodyBytes) > 0 {
			if isUnified {
				updated, serr := setUnifiedModel(srcFormat, bodyBytes, newModel)
				if serr != nil {
					return nil, huma.Error500InternalServerError("failed to set model in body: "+serr.Error(), errorx.InternalError)
				}
				bodyBytes = updated
			} else {
				updated, serr := sjson.SetBytes(bodyBytes, "model", newModel)
				if serr != nil {
					return nil, huma.Error500InternalServerError("failed to set model in body: "+serr.Error(), errorx.InternalError)
				}
				bodyBytes = updated
			}
		}
		modelName = newModel
		modelAnno = s.fetchModelAnnotations(ctx, modelName)
		clientReq.Model = modelName
		clientReq.Body = jsonBodyOrNil(jsHeaders, bodyBytes)
	}

	// 7. Resolve candidates.
	type rowView struct {
		ProviderID          int32
		ProviderName        string
		ProviderPriority    int32
		ProviderAnnotations []byte
		ModelAnnotations    []byte
		ModelName           string
		EndpointPath        string
		UpstreamModelName   string
		Priority            int32
		Annotations         []byte
		EndpointType        int32 // path endpoints don't carry it; we fill from endpoint.EndpointType
		UpstreamURL         string
	}
	var rows []rowView
	if isUnified {
		typeSet := candidateEndpointTypes(srcFormat, streaming)
		providers, perr := s.resolveProvidersByTypes(ctx, modelName, typeSet, sourceEndpointType(srcFormat))
		if perr != nil {
			var gwErr *gatewayError
			if errors.As(perr, &gwErr) {
				return nil, mapGatewayError(gwErr)
			}
			return nil, huma.Error500InternalServerError("failed to query providers", errorx.InternalError)
		}
		rows = make([]rowView, 0, len(providers))
		for _, r := range providers {
			rows = append(rows, rowView{
				ProviderID:          r.ProviderID,
				ProviderName:        r.ProviderName,
				ProviderPriority:    r.ProviderPriority,
				ProviderAnnotations: r.ProviderAnnotations,
				ModelAnnotations:    r.ModelAnnotations,
				ModelName:           r.ModelName,
				EndpointPath:        r.EndpointPath,
				UpstreamModelName:   r.UpstreamModelName,
				Priority:            r.Priority,
				Annotations:         r.Annotations,
				EndpointType:        r.EndpointType,
				UpstreamURL:         r.UpstreamUrl,
			})
		}
	} else {
		providers, perr := s.resolveProviders(ctx, endpoint.Path, modelName)
		if perr != nil {
			var gwErr *gatewayError
			if errors.As(perr, &gwErr) {
				return nil, mapGatewayError(gwErr)
			}
			return nil, huma.Error500InternalServerError("failed to query providers", errorx.InternalError)
		}
		rows = make([]rowView, 0, len(providers))
		for _, r := range providers {
			rows = append(rows, rowView{
				ProviderID:          r.ProviderID,
				ProviderName:        r.ProviderName,
				ProviderPriority:    r.ProviderPriority,
				ProviderAnnotations: r.ProviderAnnotations,
				ModelAnnotations:    r.ModelAnnotations,
				ModelName:           r.ModelName,
				EndpointPath:        r.EndpointPath,
				UpstreamModelName:   r.UpstreamModelName,
				Priority:            r.Priority,
				Annotations:         r.Annotations,
				EndpointType:        endpoint.EndpointType,
				UpstreamURL:         r.UpstreamUrl,
			})
		}
	}

	// 8. Build candidate list + sidecar.
	if len(rows) > 0 {
		if m, derr := annotations.Decode(rows[0].ModelAnnotations); derr == nil {
			modelAnno = m
		}
	}
	annoBuilder, err := newCandidateAnnotationsBuilder(nil, apiKeyAnno)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to build annotations: "+err.Error(), errorx.InternalError)
	}
	annoBuilder.modelAnno = modelAnno

	type sidecarEntry struct {
		merged      map[string]string
		upFormat    llmbridge.Format
		disabled    bool
		upstreamURL string
	}
	sidecar := make(map[string]sidecarEntry, len(rows))
	candidates := make([]jsx.Candidate, 0, len(rows))
	for _, r := range rows {
		entryAnno, _ := annotations.Decode(r.Annotations)
		merged, providerAnno := annoBuilder.merge(r.ProviderAnnotations, entryAnno)
		upFormat := srcFormat
		if isUnified {
			upFormat = upstreamFormatFor(r.EndpointType)
		}
		key := fmt.Sprintf("%d|%s", r.ProviderID, r.EndpointPath)
		sidecar[key] = sidecarEntry{
			merged:      merged,
			upFormat:    upFormat,
			upstreamURL: r.UpstreamURL,
		}
		candidates = append(candidates, jsx.Candidate{
			Provider: jsx.ProviderSummary{
				ID:          r.ProviderID,
				Name:        r.ProviderName,
				Priority:    r.ProviderPriority,
				Annotations: providerAnno,
			},
			MPE: jsx.CandidateMPE{
				ModelName:         r.ModelName,
				ProviderID:        r.ProviderID,
				EndpointPath:      r.EndpointPath,
				UpstreamModelName: r.UpstreamModelName,
				Priority:          r.Priority,
				Annotations:       entryAnno,
			},
			Annotations: merged,
		})
	}

	modelJS := &jsx.ModelSummary{Name: modelName, Annotations: modelAnno}
	endpointJS := endpointSummaryFromRow(endpoint)

	// 9. Run sortProviders.
	sorted, err := session.RunSortHook(jsx.SortInput{
		Endpoint:    endpointJS,
		Model:       modelJS,
		Request:     clientReq,
		Providers:   candidates,
		ApiKey:      apiKeyJS,
		Annotations: annotations.Merge(modelAnno, apiKeyAnno),
	})
	if err != nil {
		return nil, hookHumaError(err)
	}

	// 10. Build response, dropping unknown candidates.
	resp := &contract.SimulateDispatchResponse{}
	resp.Body.OriginalModel = originalModel
	resp.Body.ResolvedModel = modelName
	resp.Body.SourceFormat = srcFormat.String()
	resp.Body.Stream = streaming
	resp.Body.Candidates = make([]contract.SimulateCandidate, 0, len(sorted))

	for _, c := range sorted {
		key := fmt.Sprintf("%d|%s", c.Provider.ID, c.MPE.EndpointPath)
		side, ok := sidecar[key]
		if !ok {
			continue
		}
		merged := c.Annotations
		if merged == nil {
			merged = side.merged
		}
		bridged := side.upFormat != srcFormat

		var profile *contract.SimulateOutboundProfile
		if isUnified && bridged {
			p, perr := simulateBeforeTransform(session, beforeTransformArgs{
				endpoint:    endpointJS,
				model:       modelJS,
				candidate:   c,
				annotations: merged,
				apiKey:      apiKeyJS,
				clientReq:   clientReq,
				bodyBytes:   bodyBytes,
				upstreamURL: side.upstreamURL,
				srcFormat:   srcFormat,
				upFormat:    side.upFormat,
				stream:      streaming,
			})
			if perr != nil {
				return nil, hookHumaError(perr)
			}
			profile = p
		}

		resp.Body.Candidates = append(resp.Body.Candidates, contract.SimulateCandidate{
			Provider: contract.SimulateProviderSummary{
				ID:          c.Provider.ID,
				Name:        c.Provider.Name,
				Priority:    c.Provider.Priority,
				Annotations: c.Provider.Annotations,
				Disabled:    c.Provider.Disabled,
			},
			MPE: contract.SimulateMPE{
				ModelName:         c.MPE.ModelName,
				ProviderID:        c.MPE.ProviderID,
				EndpointPath:      c.MPE.EndpointPath,
				UpstreamModelName: c.MPE.UpstreamModelName,
				Priority:          c.MPE.Priority,
				Annotations:       c.MPE.Annotations,
			},
			MergedAnnotations: merged,
			UpstreamFormat:    side.upFormat.String(),
			Bridged:           bridged,
			OutboundProfile:   profile,
		})
	}

	rawLogs := session.Logs()
	if len(rawLogs) > 0 {
		resp.Body.Logs = make([]contract.SimulateLogEntry, len(rawLogs))
		for i, l := range rawLogs {
			resp.Body.Logs[i] = contract.SimulateLogEntry{
				Level:   l.Level,
				Message: l.Message,
				Ts:      l.Ts.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
			}
		}
	}

	return resp, nil
}

// beforeTransformArgs bundles inputs for simulateBeforeTransform so the call
// site stays readable.
type beforeTransformArgs struct {
	endpoint    jsx.EndpointSummary
	model       *jsx.ModelSummary
	candidate   jsx.Candidate
	annotations map[string]string
	apiKey      *jsx.ApiKeySummary
	clientReq   jsx.RequestShape
	bodyBytes   []byte
	upstreamURL string
	srcFormat   llmbridge.Format
	upFormat    llmbridge.Format
	stream      bool
}

// simulateBeforeTransform runs the beforeTransform waterfall for a bridged
// candidate and returns the resolved outbound profile. The PendingRequest
// shape is synthesized from the simulator inputs: source-format body, the
// configured upstream URL, and a minimal header set (Content-Type only).
// Credentials are deliberately omitted — the simulator never touches them.
func simulateBeforeTransform(session *jsx.Session, a beforeTransformArgs) (*contract.SimulateOutboundProfile, error) {
	baseProfile, err := llmbridge.DefaultOutboundProfileForFormat(a.upFormat)
	if err != nil {
		return nil, err
	}
	initial := jsx.OutboundProfile{Type: baseProfile.Type, Config: map[string]any{}}

	pendHeaders := http.Header{}
	if len(a.bodyBytes) > 0 {
		pendHeaders.Set("Content-Type", "application/json")
	}
	pending := jsx.PendingRequestShape{
		URL:     a.upstreamURL,
		Method:  http.MethodPost,
		Headers: mapLowerKeys(pendHeaders),
		Body:    jsonBodyOrNil(pendHeaders, a.bodyBytes),
	}

	hookProfile, err := session.RunBeforeTransformHook(jsx.BeforeTransformInput{
		Endpoint:       a.endpoint,
		Model:          a.model,
		Provider:       a.candidate.Provider,
		MPE:            a.candidate.MPE,
		ClientRequest:  a.clientReq,
		PendingRequest: pending,
		ApiKey:         a.apiKey,
		Annotations:    a.annotations,
		SourceFormat:   a.srcFormat.String(),
		UpstreamFormat: a.upFormat.String(),
		Stream:         a.stream,
	}, initial)
	if err != nil {
		return nil, err
	}
	cfg := hookProfile.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	return &contract.SimulateOutboundProfile{Type: hookProfile.Type, Config: cfg}, nil
}

// hookHumaError maps a JSX hook error to the appropriate huma error. Hook
// timeouts → 503, everything else → 502.
func hookHumaError(err error) error {
	if errors.Is(err, jsx.ErrHookTimeout) {
		return huma.Error503ServiceUnavailable(err.Error(), errorx.UpstreamError)
	}
	return huma.Error502BadGateway(err.Error(), errorx.UpstreamError)
}

// mapGatewayError converts a *gatewayError to the matching huma error so the
// dashboard sees a typed response.
func mapGatewayError(g *gatewayError) error {
	codeErr := errorx.ErrorCode(g.code)
	switch g.status {
	case http.StatusBadRequest:
		return huma.Error400BadRequest(g.message, codeErr)
	case http.StatusUnauthorized:
		return huma.Error401Unauthorized(g.message, codeErr)
	case http.StatusForbidden:
		return huma.Error403Forbidden(g.message, codeErr)
	case http.StatusNotFound:
		return huma.Error404NotFound(g.message, codeErr)
	case http.StatusBadGateway:
		return huma.Error502BadGateway(g.message, codeErr)
	case http.StatusServiceUnavailable:
		return huma.Error503ServiceUnavailable(g.message, codeErr)
	default:
		return huma.Error500InternalServerError(g.message, codeErr)
	}
}

// simulateFormatFromString maps the wire string used by the simulator API to
// the corresponding llmbridge.Format. Unknown strings yield an error.
func simulateFormatFromString(s string) (llmbridge.Format, error) {
	switch s {
	case "anthropicMessages":
		return llmbridge.FormatAnthropicMessages, nil
	case "openaiChatCompletions":
		return llmbridge.FormatOpenAIChatCompletions, nil
	case "openaiResponses":
		return llmbridge.FormatOpenAIResponses, nil
	case "geminiGenerateContent":
		return llmbridge.FormatGeminiGenerateContent, nil
	case "geminiStreamGenerateContent":
		return llmbridge.FormatGeminiStreamGenerateContent, nil
	default:
		return llmbridge.FormatUnknown, fmt.Errorf("unsupported format %q", s)
	}
}

// unifiedRoutePath returns the canonical path the matching unified route is
// mounted on. Mirrors the literal strings in server.go.
func unifiedRoutePath(f llmbridge.Format) string {
	switch f {
	case llmbridge.FormatAnthropicMessages:
		return "/api/picotera/v1/messages"
	case llmbridge.FormatOpenAIResponses:
		return "/api/picotera/v1/responses"
	case llmbridge.FormatOpenAIChatCompletions:
		return "/api/picotera/v1/chat/completions"
	case llmbridge.FormatGeminiGenerateContent:
		return "/api/picotera/v1beta/models/{model}:generateContent"
	case llmbridge.FormatGeminiStreamGenerateContent:
		return "/api/picotera/v1beta/models/{model}:streamGenerateContent"
	default:
		return ""
	}
}

// formatForEndpointType maps a path endpoint's EndpointType to the llmbridge
// format used to compute sourceFormat in the simulator response. Path
// endpoints never bridge — their upstreamFormat is always the same as the
// sourceFormat — so non-generation endpoint types fall back to Unknown.
func formatForEndpointType(t int32) llmbridge.Format {
	return upstreamFormatFor(t)
}
