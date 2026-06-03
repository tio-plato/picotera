package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"picotera/pkg/annotations"
	"picotera/pkg/db"
	"picotera/pkg/errorx"
	"picotera/pkg/jsx"
	"picotera/pkg/llmbridge"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tidwall/gjson"
)

type gatewayRouteKind int

const (
	gatewayRoutePath gatewayRouteKind = iota
	gatewayRouteUnified
)

type gatewayFlow struct {
	h              *gatewayHandler
	w              http.ResponseWriter
	r              *http.Request
	startedAt      time.Time
	ctxs           gatewayContexts
	config         gatewayFlowConfig
	body           []byte
	preRewriteBody []byte
	meta           gatewayMetaState
	auth           gatewayAuthState
	model          gatewayModelState
	session        *jsx.Session
}

type gatewayFlowConfig struct {
	Kind              gatewayRouteKind
	Endpoint          db.Endpoint
	PathVars          map[string]string
	SourceFormat      llmbridge.Format
	Credentials       int32
	ExtractModel      func(*http.Request, []byte, map[string]string) (gatewayModelMode, error)
	SetBodyModel      func([]byte, string) ([]byte, error)
	ResolveCandidates func(context.Context, gatewayModelMode, gatewayAuthState) (candidateSet, error)
	PrepareAttempt    func(context.Context, *gatewayFlow, attemptInput) (attemptPrepared, error)
	HandleSuccess     func(successInput)
}

type gatewayMetaState struct {
	ID             string
	CreatedAt      time.Time
	ParentSpanID   string
	ParentSpanIDPg pgtype.Text
	ProjectID      pgtype.Int4
	RequestHeader  http.Header
	RequestMethod  string
	RequestURL     string
}

type gatewayAuthState struct {
	APIKey     *db.ApiKey
	APIKeyID   pgtype.Int4
	APIKeyJS   *jsx.ApiKeySummary
	APIKeyAnno map[string]string
}

type gatewayModelState struct {
	Mode        gatewayModelMode
	Original    string
	Routed      string
	Annotations map[string]string
}

type gatewayModelMode struct {
	OriginalModel string
	RoutedModel   string
	// Streaming is the five-rule detection of whether the client expects a
	// streaming response (see detectStreaming), filled in resolveAndRewriteModel.
	// It drives candidateEndpointTypes' upstream-variant selection, the
	// upstream header-timeout decision, and the beforeTransform hook's Stream
	// input — all from this single source.
	Streaming bool
	HasModel  bool
}

func newGatewayFlow(h *gatewayHandler, w http.ResponseWriter, r *http.Request, startedAt time.Time, cfg gatewayFlowConfig) *gatewayFlow {
	if cfg.Credentials == 0 {
		cfg.Credentials = cfg.Endpoint.CredentialsResolver
	}
	return &gatewayFlow{h: h, w: w, r: r, startedAt: startedAt, config: cfg}
}

// detectStreaming reports whether the client expects a streaming response,
// applying five rules in order (any match => true):
//  1. the source format is the Gemini stream endpoint;
//  2. the request body has `stream: true`;
//  3. the Accept header contains text/event-stream;
//  4. the Accept header contains application/x-ndjson;
//  5. otherwise false.
//
// Accept matching is case-insensitive across all Accept header values.
func detectStreaming(srcFormat llmbridge.Format, r *http.Request, body []byte) bool {
	if srcFormat == llmbridge.FormatGeminiStreamGenerateContent {
		return true
	}
	if gjson.GetBytes(body, "stream").Bool() {
		return true
	}
	for _, accept := range r.Header.Values("Accept") {
		lower := strings.ToLower(accept)
		if strings.Contains(lower, "text/event-stream") || strings.Contains(lower, "application/x-ndjson") {
			return true
		}
	}
	return false
}

func (f *gatewayFlow) run() {
	f.ctxs = newGatewayContexts(f.r)
	defer f.ctxs.cancelBase()
	defer f.ctxs.cancelRequest()
	if !f.readBody() {
		return
	}
	if !f.insertMetaRequest() {
		return
	}
	defer f.h.liveRequests.Remove(f.meta.ID)
	if !f.authenticateAndBackfill() {
		return
	}
	if !f.resolveAndRewriteModel() {
		return
	}
	defer f.session.Close()
	sorted, sidecars, js, ok := f.resolveAndSortCandidates()
	if !ok {
		return
	}
	result := f.runAttempts(sorted, sidecars, js)
	if result.Handled {
		return
	}
	f.failAllProviders(result.LastErr)
}

func (f *gatewayFlow) readBody() bool {
	body, err := io.ReadAll(f.r.Body)
	_ = f.r.Body.Close()
	if err != nil {
		writeGatewayError(f.w, http.StatusInternalServerError, "failed to read request body", errorx.InternalError.Error())
		return false
	}
	f.body = body
	return true
}

func (f *gatewayFlow) insertMetaRequest() bool {
	metaID, metaIDCreatedAt := newRequestID()
	header := f.r.Header.Clone()
	parentSpanID := extractParentSpanID(header)
	parentSpanIDPg := pgtype.Text{String: parentSpanID, Valid: parentSpanID != ""}
	projectIDPg := f.h.extractProjectID(f.ctxs.Request, f.body)
	pctx, pcancel := f.ctxs.Persist()
	defer pcancel()
	createdAt := f.h.insertRequest(pctx, db.InsertRequestParams{
		ID:                 metaID,
		SpanID:             pgtype.Text{String: metaID, Valid: true},
		ParentSpanID:       parentSpanIDPg,
		Type:               db.RequestTypeMeta,
		Status:             db.RequestStatusPending,
		ProviderID:         pgtype.Int4{Valid: false},
		EndpointPath:       pgtype.Text{String: f.config.Endpoint.Path, Valid: true},
		ApiKeyID:           pgtype.Int4{Valid: false},
		Model:              pgtype.Text{Valid: false},
		UpstreamModel:      pgtype.Text{Valid: false},
		StatusCode:         pgtype.Int4{Valid: false},
		ErrorMessage:       pgtype.Text{Valid: false},
		TimeSpentMs:        pgtype.Int4{Valid: false},
		UserMessagePreview: extractUserMessagePreview(f.body, f.config.Endpoint.EndpointType),
		ProjectID:          projectIDPg,
		CreatedAt:          pgtype.Timestamp{Time: metaIDCreatedAt, Valid: true},
	})
	f.meta = gatewayMetaState{
		ID:             metaID,
		CreatedAt:      createdAt,
		ParentSpanID:   parentSpanID,
		ParentSpanIDPg: parentSpanIDPg,
		ProjectID:      projectIDPg,
		RequestHeader:  header,
		RequestMethod:  f.r.Method,
		RequestURL:     f.r.URL.String(),
	}
	f.h.liveRequests.RegisterMeta(metaID, f.ctxs.cancelRequest)
	f.h.uploadRequestArtifact(pctx, metaID, createdAt, f.r.Method, f.r.URL.String(), header, f.body)
	if projectIDPg.Valid {
		seenCtx, seenCancel := f.ctxs.Persist()
		go func() {
			defer seenCancel()
			f.h.upsertProjectSeen(seenCtx, projectIDPg.Int32, createdAt)
		}()
	}
	return true
}

func (f *gatewayFlow) authenticateAndBackfill() bool {
	apiKey, err := f.h.authenticateClient(f.ctxs.Request, f.r, f.config.Credentials)
	if err != nil {
		var gwErr *gatewayError
		if errors.As(err, &gwErr) {
			f.failMeta(int32(gwErr.status), gwErr.message, db.FinishReasonInternal)
		} else {
			f.failMeta(http.StatusInternalServerError, "auth validation failed", db.FinishReasonInternal)
		}
		f.failGatewayError(err)
		return false
	}
	apiKeyJS := apiKeySummaryFromRow(apiKey)
	f.auth = gatewayAuthState{
		APIKey:     apiKey,
		APIKeyID:   pgtype.Int4{Int32: apiKey.ID, Valid: true},
		APIKeyJS:   apiKeyJS,
		APIKeyAnno: apiKeyJS.Annotations,
	}
	pctx, pcancel := f.ctxs.Persist()
	defer pcancel()
	f.h.updateRequestOnHeader(pctx, db.UpdateRequestOnHeaderParams{
		ID:           f.meta.ID,
		EndpointPath: pgtype.Text{String: f.config.Endpoint.Path, Valid: true},
		ApiKeyID:     f.auth.APIKeyID,
		Status:       db.RequestStatusPending,
		CreatedAt:    pgtype.Timestamp{Time: f.meta.CreatedAt, Valid: true},
	})
	return true
}

func (f *gatewayFlow) resolveAndRewriteModel() bool {
	mode, err := f.config.ExtractModel(f.r, f.body, f.config.PathVars)
	if err != nil {
		f.failGatewayErrorWithFallback(err, http.StatusBadRequest, "model extraction failed")
		return false
	}
	mode.RoutedModel = mode.OriginalModel
	mode.Streaming = detectStreaming(f.config.SourceFormat, f.r, f.body)
	f.model = gatewayModelState{Mode: mode, Original: mode.OriginalModel, Routed: mode.RoutedModel}
	f.updateMetaModel(mode.RoutedModel)
	session, err := f.h.jsxEngine.NewSession(f.ctxs.Request, f.meta.ID)
	if err != nil {
		f.failInternal(http.StatusBadGateway, "failed to load js hooks: "+err.Error(), errorx.UpstreamError.Error())
		return false
	}
	f.session = session
	f.model.Annotations = f.h.fetchModelAnnotations(f.ctxs.Request, mode.RoutedModel)
	f.preRewriteBody = append([]byte(nil), f.body...)
	newModel, err := f.session.RunRewriteModelHook(jsx.RewriteModelInput{
		Request:     serializeClientRequest(f.r, f.body, mode.RoutedModel, f.config.PathVars),
		Model:       mode.OriginalModel,
		ApiKey:      f.auth.APIKeyJS,
		Annotations: annotations.Merge(f.model.Annotations, f.auth.APIKeyAnno),
	}, mode.RoutedModel)
	if err != nil {
		f.failHook(err)
		return false
	}
	if !mode.HasModel && newModel != "" {
		f.failHook(errors.New("rewriteModel returned non-empty model on no-model endpoint"))
		return false
	}
	if newModel != mode.RoutedModel {
		updated, serr := f.config.SetBodyModel(f.body, newModel)
		if serr != nil {
			f.failInternal(http.StatusInternalServerError, "failed to set model in body: "+serr.Error(), errorx.InternalError.Error())
			return false
		}
		f.body = updated
		mode.RoutedModel = newModel
		f.model.Routed = newModel
		f.model.Mode = mode
		f.model.Annotations = f.h.fetchModelAnnotations(f.ctxs.Request, newModel)
		f.updateMetaModel(newModel)
	}
	return true
}

func (f *gatewayFlow) resolveAndSortCandidates() ([]jsx.Candidate, map[string]gatewayCandidateSidecar, gatewayJSContext, bool) {
	set, err := f.config.ResolveCandidates(f.ctxs.Request, f.model.Mode, f.auth)
	if err != nil {
		f.failGatewayErrorWithFallback(err, http.StatusInternalServerError, "failed to query providers")
		return nil, nil, gatewayJSContext{}, false
	}
	if set.ModelAnno != nil {
		f.model.Annotations = set.ModelAnno
	}
	candidates := make([]jsx.Candidate, 0, len(set.Items))
	for _, item := range set.Items {
		candidates = append(candidates, item.Candidate)
	}
	js := gatewayJSContext{
		Endpoint:      endpointSummaryFromRow(f.config.Endpoint),
		Model:         &jsx.ModelSummary{Name: f.model.Routed, Annotations: f.model.Annotations},
		ClientRequest: serializeClientRequest(f.r, f.body, f.model.Routed, f.config.PathVars),
		APIKey:        f.auth.APIKeyJS,
		Annotations:   annotations.Merge(f.model.Annotations, f.auth.APIKeyAnno),
	}
	sorted, err := f.session.RunSortHook(jsx.SortInput{
		Endpoint:    js.Endpoint,
		Model:       js.Model,
		Request:     js.ClientRequest,
		Providers:   candidates,
		ApiKey:      js.APIKey,
		Annotations: js.Annotations,
	})
	if err != nil {
		f.failHook(err)
		return nil, nil, gatewayJSContext{}, false
	}
	return sorted, candidateSidecarMap(set), js, true
}

func (f *gatewayFlow) updateMetaModel(model string) {
	pctx, pcancel := f.ctxs.Persist()
	defer pcancel()
	f.h.updateRequestModel(pctx, db.UpdateRequestModelParams{
		ID:        f.meta.ID,
		Model:     pgtype.Text{String: model, Valid: model != ""},
		CreatedAt: pgtype.Timestamp{Time: f.meta.CreatedAt, Valid: true},
	})
}
