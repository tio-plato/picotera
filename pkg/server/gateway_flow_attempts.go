package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"picotera/pkg/db"
	"picotera/pkg/jsx"
	"picotera/pkg/llmbridge"

	"github.com/jackc/pgx/v5/pgtype"
)

type gatewayJSContext struct {
	Endpoint      jsx.EndpointSummary
	Model         *jsx.ModelSummary
	ClientRequest jsx.RequestShape
	APIKey        *jsx.ApiKeySummary
	Annotations   map[string]string
}

type attemptInput struct {
	Candidate         jsx.Candidate
	Sidecar           gatewayCandidateSidecar
	Annotations       map[string]string
	Decision          jsx.BeforeRequestDecision
	CurrentRetryCount int
	TotalAttemptCount int
	AttemptCtx        context.Context
	UpstreamID        string
	UpstreamCreatedAt time.Time
	AttemptStart      time.Time
	UpstreamModel     string
	Request           *http.Request
	RequestBody       []byte
	PendingRequest    jsx.PendingRequestShape
	JS                gatewayJSContext
}

type attemptPrepared struct {
	Request         *http.Request
	RequestBody     []byte
	OutboundProfile llmbridge.OutboundProfile
	WebSearch       *webSearchContext
}

type successInput struct {
	Flow              *gatewayFlow
	Candidate         jsx.Candidate
	Sidecar           gatewayCandidateSidecar
	Response          *http.Response
	AttemptCtx        context.Context
	Cancel            context.CancelFunc
	UpstreamID        string
	UpstreamCreatedAt time.Time
	AttemptStart      time.Time
	UpstreamStartTime time.Time
	ProviderID        int32
	RoutedModel       string
	UpstreamModel     string
	Prepared          attemptPrepared
}

type attemptResult struct {
	Handled bool
	LastErr error
}

func (f *gatewayFlow) runAttempts(sorted []jsx.Candidate, sidecars map[string]gatewayCandidateSidecar, js gatewayJSContext) attemptResult {
	state := attemptState{}
	for state.Index < len(sorted) && state.TotalAttemptCount < f.h.config.JSMaxTotalAttempts {
		cand := sorted[state.Index]
		side, ok := lookupCandidateSidecar(f.config.Kind, sidecars, cand)
		if !ok {
			state.Index++
			state.CurrentRetryCount = 0
			continue
		}
		handled, stop := f.runSingleAttempt(cand, side, js, &state)
		if handled || stop {
			return attemptResult{Handled: handled, LastErr: state.LastErr}
		}
	}
	return attemptResult{LastErr: state.LastErr}
}

type attemptState struct {
	Index             int
	CurrentRetryCount int
	TotalAttemptCount int
	LastErr           error
	LastJSErr         *jsx.LastError
}

func (f *gatewayFlow) runSingleAttempt(cand jsx.Candidate, side gatewayCandidateSidecar, js gatewayJSContext, state *attemptState) (handled bool, stop bool) {
	candAnno := cand.Annotations
	if candAnno == nil {
		candAnno = side.Annotations
	}
	dec, err := f.runBeforeRequest(cand, candAnno, js, state)
	if err != nil {
		f.failHook(err)
		return false, true
	}
	if !f.waitHookDelay(dec.Delay) {
		state.LastErr = f.ctxs.Request.Err()
		return false, true
	}
	if dec.Next {
		state.Index++
		state.CurrentRetryCount = 0
		return false, false
	}
	input, cancel, err := f.insertUpstreamAttempt(cand, side, candAnno, dec, js, state)
	if err != nil {
		f.recordAttemptFailure(state, input, side.ProviderID, 0, err, db.FinishReasonInternal)
		cancel()
		return false, false
	}
	prepared, err := f.buildRewrittenUpstreamRequest(input)
	if err != nil {
		var hookErr gatewayHookError
		if errors.As(err, &hookErr) {
			f.failHook(hookErr.err)
			cancel()
			return false, true
		}
		f.recordAttemptFailure(state, input, side.ProviderID, 0, err, db.FinishReasonInternal)
		cancel()
		return false, false
	}
	reqArtifactCtx, reqArtifactCancel := f.ctxs.Persist()
	f.h.uploadRequestArtifact(reqArtifactCtx, input.UpstreamID, input.UpstreamCreatedAt, prepared.Request.Method, prepared.Request.URL.String(), prepared.Request.Header.Clone(), prepared.RequestBody)
	reqArtifactCancel()
	upstreamStart := time.Now()
	resp, err := f.h.forwardRequest(prepared.Request, side.ProxyURL)
	if err != nil {
		f.recordAttemptFailure(state, input, side.ProviderID, 0, err, classifyForwardError(err, f.ctxs.Request))
		cancel()
		return false, false
	}
	if resp.StatusCode == http.StatusOK {
		f.config.HandleSuccess(successInput{Flow: f, Candidate: cand, Sidecar: side, Response: resp, AttemptCtx: input.AttemptCtx, Cancel: cancel, UpstreamID: input.UpstreamID, UpstreamCreatedAt: input.UpstreamCreatedAt, AttemptStart: input.AttemptStart, UpstreamStartTime: upstreamStart, ProviderID: side.ProviderID, RoutedModel: f.model.Routed, UpstreamModel: input.UpstreamModel, Prepared: prepared})
		return true, false
	}
	f.handleUpstreamNonOK(state, input, resp, side.ProviderID)
	cancel()
	return false, false
}

func (f *gatewayFlow) runBeforeRequest(cand jsx.Candidate, candAnno map[string]string, js gatewayJSContext, state *attemptState) (jsx.BeforeRequestDecision, error) {
	return f.session.RunBeforeRequestHook(jsx.BeforeRequestInput{
		Endpoint:          js.Endpoint,
		Model:             js.Model,
		Request:           js.ClientRequest,
		Provider:          cand.Provider,
		MPE:               cand.MPE,
		CurrentRetryCount: state.CurrentRetryCount,
		TotalAttemptCount: state.TotalAttemptCount,
		LastError:         state.LastJSErr,
		ApiKey:            js.APIKey,
		Annotations:       candAnno,
	})
}

func (f *gatewayFlow) waitHookDelay(delayMs int) bool {
	if delayMs <= 0 {
		return true
	}
	delay := time.Duration(delayMs) * time.Millisecond
	if f.h.config.JSMaxDelay > 0 && delay > f.h.config.JSMaxDelay {
		delay = f.h.config.JSMaxDelay
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-f.ctxs.Request.Done():
		return false
	}
}

func (f *gatewayFlow) insertUpstreamAttempt(cand jsx.Candidate, side gatewayCandidateSidecar, candAnno map[string]string, dec jsx.BeforeRequestDecision, js gatewayJSContext, state *attemptState) (attemptInput, context.CancelFunc, error) {
	attemptCtx, cancel := context.WithCancel(f.ctxs.Request)
	upstreamModel := dec.UpstreamModel
	if upstreamModel == "" {
		upstreamModel = candidateUpstreamModel(cand)
	}
	if upstreamModel == "" {
		upstreamModel = f.model.Routed
	}
	upstreamID, upstreamIDCreatedAt := newRequestID()
	pctx, pcancel := f.ctxs.Persist()
	defer pcancel()
	upstreamCreatedAt := f.h.insertRequest(pctx, db.InsertRequestParams{
		ID:                 upstreamID,
		SpanID:             pgtype.Text{String: f.meta.ID, Valid: true},
		ParentSpanID:       f.meta.ParentSpanIDPg,
		Type:               db.RequestTypeUpstream,
		Status:             db.RequestStatusPending,
		ProviderID:         pgtype.Int4{Int32: side.ProviderID, Valid: true},
		EndpointPath:       pgtype.Text{String: side.EndpointPath, Valid: side.EndpointPath != ""},
		ApiKeyID:           f.auth.APIKeyID,
		Model:              pgtype.Text{String: f.model.Routed, Valid: f.model.Routed != ""},
		UpstreamModel:      pgtype.Text{String: upstreamModel, Valid: upstreamModel != ""},
		StatusCode:         pgtype.Int4{Valid: false},
		ErrorMessage:       pgtype.Text{Valid: false},
		TimeSpentMs:        pgtype.Int4{Valid: false},
		UserMessagePreview: pgtype.Text{Valid: false},
		ProjectID:          f.meta.ProjectID,
		CreatedAt:          pgtype.Timestamp{Time: upstreamIDCreatedAt, Valid: true},
	})
	return attemptInput{Candidate: cand, Sidecar: side, Annotations: candAnno, Decision: dec, CurrentRetryCount: state.CurrentRetryCount, TotalAttemptCount: state.TotalAttemptCount, AttemptCtx: attemptCtx, UpstreamID: upstreamID, UpstreamCreatedAt: upstreamCreatedAt, AttemptStart: time.Now(), UpstreamModel: upstreamModel, JS: js}, cancel, nil
}

func (f *gatewayFlow) buildRewrittenUpstreamRequest(input attemptInput) (attemptPrepared, error) {
	body := f.body
	upstreamModel := input.UpstreamModel
	if f.config.Kind == gatewayRouteUnified {
		upstreamModel = ""
		var err error
		body, err = setUnifiedModel(f.config.SourceFormat, f.body, input.UpstreamModel)
		if err != nil {
			return attemptPrepared{}, err
		}
	}
	req, reqBody, err := buildUpstreamRequest(input.AttemptCtx, f.r, body, input.Sidecar.UpstreamURL, upstreamModel, input.Sidecar.Credentials, input.Sidecar.SendResolver, f.config.PathVars)
	if err != nil {
		return attemptPrepared{}, err
	}
	pending := serializePendingRequest(req, reqBody)
	newPending, err := f.session.RunRewriteHook(jsx.RewriteInput{
		Endpoint:          input.JS.Endpoint,
		Model:             input.JS.Model,
		Provider:          input.Candidate.Provider,
		MPE:               input.Candidate.MPE,
		CurrentRetryCount: input.CurrentRetryCount,
		TotalAttemptCount: input.TotalAttemptCount,
		ClientRequest:     input.JS.ClientRequest,
		PendingRequest:    pending,
		ApiKey:            input.JS.APIKey,
		Annotations:       input.Annotations,
	})
	if err != nil {
		return attemptPrepared{}, gatewayHookError{err: err}
	}
	req, reqBody, err = buildRequestFromPending(input.AttemptCtx, newPending, reqBody)
	if err != nil {
		return attemptPrepared{}, gatewayHookError{err: err}
	}
	input.Request = req
	input.RequestBody = reqBody
	input.PendingRequest = newPending
	return f.config.PrepareAttempt(input.AttemptCtx, f, input)
}

func (f *gatewayFlow) handleUpstreamNonOK(state *attemptState, input attemptInput, resp *http.Response, providerID int32) {
	decoded, err := decodedBody(resp)
	if err != nil {
		_ = resp.Body.Close()
		f.recordAttemptFailure(state, input, providerID, int32(resp.StatusCode), fmt.Errorf("decode upstream response: %w", err), db.FinishReasonInternal)
		return
	}
	respBody, err := io.ReadAll(decoded.Body)
	_ = decoded.Body.Close()
	if err != nil {
		f.recordAttemptFailure(state, input, providerID, int32(resp.StatusCode), fmt.Errorf("decode upstream response: %w", err), db.FinishReasonInternal)
		return
	}
	pctx, pcancel := f.ctxs.Persist()
	defer pcancel()
	f.h.uploadResponseArtifact(pctx, input.UpstreamID, input.UpstreamCreatedAt, resp.StatusCode, resp.Header.Clone(), respBody, nil)
	errMsg := string(respBody)
	f.h.updateRequestOnComplete(pctx, db.UpdateRequestOnCompleteParams{
		ID:           input.UpstreamID,
		StatusCode:   pgtype.Int4{Int32: int32(resp.StatusCode), Valid: true},
		ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
		TimeSpentMs:  pgtype.Int4{Int32: int32(time.Since(input.AttemptStart).Milliseconds()), Valid: true},
		Status:       db.RequestStatusFailed,
		FinishReason: pgtype.Int4{Int32: db.FinishReasonInternal, Valid: true},
		CreatedAt:    pgtype.Timestamp{Time: input.UpstreamCreatedAt, Valid: true},
	})
	updateAttemptState(state, providerID, resp.StatusCode, errMsg, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, errMsg))
}

func (f *gatewayFlow) recordAttemptFailure(state *attemptState, input attemptInput, providerID int32, statusCode int32, err error, finishReason int32) {
	pctx, pcancel := f.ctxs.Persist()
	defer pcancel()
	f.h.completeFailedAttemptWithReason(pctx, input.UpstreamID, input.UpstreamCreatedAt, input.AttemptStart, statusCode, err.Error(), finishReason)
	updateAttemptState(state, providerID, int(statusCode), err.Error(), err)
}

func updateAttemptState(state *attemptState, providerID int32, statusCode int, message string, err error) {
	state.LastErr = err
	state.LastJSErr = &jsx.LastError{ProviderID: int(providerID), StatusCode: statusCode, Message: message}
	state.CurrentRetryCount++
	state.TotalAttemptCount++
}

func identityPrepareAttempt(_ context.Context, _ *gatewayFlow, input attemptInput) (attemptPrepared, error) {
	return attemptPrepared{Request: input.Request, RequestBody: input.RequestBody}, nil
}

func prepareUnifiedAttempt(ctx context.Context, f *gatewayFlow, input attemptInput) (attemptPrepared, error) {
	req, reqBody := input.Request, input.RequestBody
	wsCtx, err := prepareUnifiedWebSearch(f, input, reqBody)
	if err != nil {
		return attemptPrepared{}, err
	}
	if wsCtx != nil && wsCtx.active {
		reqBody = wsCtx.rewrittenBody
		resetRequestBody(req, reqBody)
	}
	profile, err := prepareUnifiedOutboundProfile(f, input)
	if err != nil {
		return attemptPrepared{}, err
	}
	req, reqBody, err = bridgeUnifiedRequest(ctx, f, input, req, reqBody, profile)
	if err != nil {
		return attemptPrepared{}, err
	}
	return attemptPrepared{Request: req, RequestBody: reqBody, OutboundProfile: profile, WebSearch: wsCtx}, nil
}

func prepareUnifiedWebSearch(f *gatewayFlow, input attemptInput, reqBody []byte) (*webSearchContext, error) {
	if f.config.SourceFormat != llmbridge.FormatAnthropicMessages || input.Sidecar.SupportsNativeWebSearch || !hasWebSearchTool(reqBody) {
		return nil, nil
	}
	rewrote, err := rewriteWebSearchTools(reqBody)
	if err != nil {
		return nil, err
	}
	rewrote, err = rewriteWebSearchHistory(rewrote)
	if err != nil {
		return nil, err
	}
	return &webSearchContext{active: true, apiKeyToken: f.auth.APIKey.Key, metaID: f.meta.ID, metaCreatedAt: f.meta.CreatedAt, parentSpanID: f.meta.ParentSpanID, originalRequestBody: append([]byte(nil), f.preRewriteBody...), rewrittenBody: rewrote}, nil
}

func prepareUnifiedOutboundProfile(f *gatewayFlow, input attemptInput) (llmbridge.OutboundProfile, error) {
	base, err := llmbridge.DefaultOutboundProfileForFormat(input.Sidecar.UpstreamFormat)
	if err != nil {
		return llmbridge.OutboundProfile{}, gatewayHookError{err: err}
	}
	hookProfile, err := f.session.RunBeforeTransformHook(jsx.BeforeTransformInput{
		Endpoint:          input.JS.Endpoint,
		Model:             input.JS.Model,
		Provider:          input.Candidate.Provider,
		MPE:               input.Candidate.MPE,
		CurrentRetryCount: input.CurrentRetryCount,
		TotalAttemptCount: input.TotalAttemptCount,
		ClientRequest:     input.JS.ClientRequest,
		PendingRequest:    input.PendingRequest,
		ApiKey:            input.JS.APIKey,
		Annotations:       input.Annotations,
		SourceFormat:      f.config.SourceFormat.String(),
		UpstreamFormat:    input.Sidecar.UpstreamFormat.String(),
		Stream:            f.model.Streaming,
	}, jsx.OutboundProfile{Type: base.Type, Config: map[string]any{}})
	if err != nil {
		return llmbridge.OutboundProfile{}, err
	}
	out := llmbridge.OutboundProfile{Type: hookProfile.Type, Config: hookProfile.Config}
	if out.Config == nil {
		out.Config = map[string]any{}
	}
	return out, nil
}

func bridgeUnifiedRequest(ctx context.Context, f *gatewayFlow, input attemptInput, req *http.Request, reqBody []byte, profile llmbridge.OutboundProfile) (*http.Request, []byte, error) {
	if input.Sidecar.UpstreamFormat == f.config.SourceFormat {
		return req, reqBody, nil
	}
	if !f.h.llmBridge.Enabled() {
		return nil, nil, fmt.Errorf("llmbridge: wasm module is not configured")
	}
	bridgeURL := req.URL.String()
	if f.config.SourceFormat == llmbridge.FormatGeminiGenerateContent || f.config.SourceFormat == llmbridge.FormatGeminiStreamGenerateContent {
		bridgeURL = llmbridge.SyntheticGeminiPath(f.config.SourceFormat, f.model.Original)
	}
	upBody, upCT, err := f.h.llmBridge.BridgeRequest(ctx, f.config.SourceFormat, input.Sidecar.UpstreamFormat, reqBody, req.Header, bridgeURL, profile)
	if err != nil {
		return nil, nil, err
	}
	resetRequestBody(req, upBody)
	req.Header.Set("Content-Type", upCT)
	return req, upBody, nil
}

func resetRequestBody(req *http.Request, body []byte) {
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
}
