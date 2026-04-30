package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"picotera/pkg/artifacts"
	"picotera/pkg/db"
	"picotera/pkg/errorx"
	"picotera/pkg/jsx"
	"picotera/pkg/logx"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/xid"
)

type gatewayHandler struct {
	*Server
}

var _ http.Handler = (*gatewayHandler)(nil)

func (h *gatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	gatewayStart := time.Now()
	bgCtx := context.Background()

	// 1. Match endpoint by path. If no endpoint matches, we don't log the
	// request at all — the request table tracks LLM gateway traffic, not
	// every miss. Browser navigations get the dashboard SPA; everything else
	// gets the structured JSON 404.
	endpoint, err := h.resolveEndpoint(r.Context(), r.URL.Path)
	if err != nil {
		if isRouteNotFound(err) && looksLikeBrowserNav(r) {
			h.staticHandler.ServeHTTP(w, r)
			return
		}
		handleGatewayErr(w, err)
		return
	}

	// 2. Read request body
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		writeGatewayError(w, http.StatusInternalServerError, "failed to read request body", errorx.InternalError.Error())
		return
	}

	// 3. Insert meta request now that we know the endpoint matched.
	metaID := xid.New().String()
	metaReqHeader := r.Header.Clone()
	metaReqMethod := r.Method
	metaReqURL := r.URL.String()
	metaCreatedAt := h.insertRequest(bgCtx, db.InsertRequestParams{
		ID:           metaID,
		SpanID:       pgtype.Text{String: metaID, Valid: true},
		ParentSpanID: pgtype.Text{Valid: false},
		Type:         db.RequestTypeMeta,
		Status:       db.RequestStatusPending,
		ProviderID:   pgtype.Int4{Valid: false},
		EndpointPath: pgtype.Text{Valid: false},
		ApiKeyID:     pgtype.Int4{Valid: false},
		Model:        pgtype.Text{Valid: false},
		StatusCode:   pgtype.Int4{Valid: false},
		ErrorMessage: pgtype.Text{Valid: false},
		TimeSpentMs:  pgtype.Int4{Valid: false},
	})

	h.uploadRequestArtifact(bgCtx, metaID, metaCreatedAt, metaReqMethod, metaReqURL, metaReqHeader, body)

	failMeta := func(statusCode int32, errMsg string) {
		h.updateRequestOnComplete(bgCtx, db.UpdateRequestOnCompleteParams{
			ID:           metaID,
			StatusCode:   pgtype.Int4{Int32: statusCode, Valid: true},
			ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
			TimeSpentMs:  pgtype.Int4{Int32: int32(time.Since(gatewayStart).Milliseconds()), Valid: true},
			Status:       db.RequestStatusFailed,
		})
	}

	failMetaResponse := func(err error) {
		statusCode, respBody := handleGatewayErr(w, err)
		h.uploadResponseArtifact(bgCtx, metaID, metaCreatedAt, statusCode, w.Header().Clone(), respBody)
	}

	failHook := func(err error) {
		status := http.StatusBadGateway
		if errors.Is(err, jsx.ErrHookTimeout) {
			status = http.StatusServiceUnavailable
		}
		errMsg := err.Error()
		failMeta(int32(status), errMsg)
		respBody := writeGatewayError(w, status, errMsg, errorx.UpstreamError.Error())
		h.uploadResponseArtifact(bgCtx, metaID, metaCreatedAt, status, w.Header().Clone(), respBody)
	}

	// 4. Validate client auth
	err = validateClientAuth(r)
	if err != nil {
		var gwErr *gatewayError
		if errors.As(err, &gwErr) {
			failMeta(int32(gwErr.status), gwErr.message)
		} else {
			failMeta(http.StatusInternalServerError, "auth validation failed")
		}
		failMetaResponse(err)
		return
	}

	// 5. Extract model name
	modelName, err := extractModel(body, endpoint.ModelPath)
	if err != nil {
		var gwErr *gatewayError
		if errors.As(err, &gwErr) {
			failMeta(int32(gwErr.status), gwErr.message)
		} else {
			failMeta(http.StatusBadRequest, "model extraction failed")
		}
		failMetaResponse(err)
		return
	}

	// 7. Resolve providers
	providers, err := h.resolveProviders(r.Context(), endpoint.Path, modelName)
	if err != nil {
		var gwErr *gatewayError
		if errors.As(err, &gwErr) {
			failMeta(int32(gwErr.status), gwErr.message)
		} else {
			failMeta(http.StatusInternalServerError, "failed to query providers")
		}
		failMetaResponse(err)
		return
	}

	// 8. Build jsx session and run hooks. The session loads enabled scripts from
	// the DB; if no scripts are enabled this is essentially a no-op pass-through.
	session, err := h.jsxEngine.NewSession(r.Context(), metaID)
	if err != nil {
		errMsg := "failed to load js hooks: " + err.Error()
		failMeta(http.StatusBadGateway, errMsg)
		respBody := writeGatewayError(w, http.StatusBadGateway, errMsg, errorx.UpstreamError.Error())
		h.uploadResponseArtifact(bgCtx, metaID, metaCreatedAt, http.StatusBadGateway, w.Header().Clone(), respBody)
		return
	}
	defer session.Close()

	// 8a. Build candidate list and a sidecar map for fields not exposed to JS
	// (upstream URL, credentials). The hooks see {provider, mpe}; we look up
	// the rest by providerID after the hook returns.
	type providerSidecar struct {
		upstreamURL string
		credentials string
	}
	sidecar := make(map[int32]providerSidecar, len(providers))
	candidates := make([]jsx.Candidate, 0, len(providers))
	for _, row := range providers {
		sidecar[row.ProviderID] = providerSidecar{
			upstreamURL: row.UpstreamUrl.String,
			credentials: row.ProviderCredentials.String,
		}
		candidates = append(candidates, jsx.Candidate{
			Provider: map[string]any{
				"id":       row.ProviderID,
				"name":     row.ProviderName.String,
				"priority": row.ProviderPriority.Int32,
				"annotations": json.RawMessage(row.ProviderAnnotations),
			},
			MPE: map[string]any{
				"modelName":         row.ModelName,
				"providerId":        row.ProviderID,
				"endpointPath":      row.EndpointPath,
				"upstreamModelName": row.UpstreamModelName.String,
				"priority":          row.Priority,
				"annotations":       json.RawMessage(row.Annotations),
			},
		})
	}

	// 8b. The JS-visible client request shape (read-only).
	jsClientRequest := serializeClientRequest(r, body, modelName)

	// 8c. sortProviders — once before the loop.
	sortedCandidates, err := session.RunSortHook(jsx.SortInput{
		Endpoint:  endpoint,
		Model:     nil, // model row lookup is out of scope for v1
		Request:   jsClientRequest,
		Providers: candidates,
	})
	if err != nil {
		failHook(err)
		return
	}

	// 8d. Retry loop
	var lastErr error
	var lastJSErr *jsx.LastError
	i := 0
	currentRetryCount := 0
	totalAttemptCount := 0

	for {
		if i >= len(sortedCandidates) {
			break
		}
		if totalAttemptCount >= h.config.JSMaxTotalAttempts {
			break
		}
		cand := sortedCandidates[i]

		// Pull providerID back from the JSON-roundtripped Provider field.
		providerID, ok := candidateProviderID(cand)
		if !ok {
			// Skip malformed candidate.
			i++
			currentRetryCount = 0
			continue
		}
		side, hasSide := sidecar[providerID]
		if !hasSide {
			// JS introduced a provider we never saw — fail safely by skipping it.
			i++
			currentRetryCount = 0
			continue
		}

		dec, err := session.RunBeforeRequestHook(jsx.BeforeRequestInput{
			Endpoint:          endpoint,
			Model:             nil,
			Request:           jsClientRequest,
			Provider:          cand.Provider,
			MPE:               cand.MPE,
			CurrentRetryCount: currentRetryCount,
			TotalAttemptCount: totalAttemptCount,
			LastError:         lastJSErr,
		})
		if err != nil {
			failHook(err)
			return
		}
		if dec.Delay > 0 {
			d := time.Duration(dec.Delay) * time.Millisecond
			if h.config.JSMaxDelay > 0 && d > h.config.JSMaxDelay {
				d = h.config.JSMaxDelay
			}
			time.Sleep(d)
		}
		if dec.Next {
			i++
			currentRetryCount = 0
			continue
		}

		// Build upstream request.
		attemptStart := time.Now()
		ctx, cancel := context.WithCancel(r.Context())

		upstreamID := xid.New().String()
		upstreamCreatedAt := h.insertRequest(bgCtx, db.InsertRequestParams{
			ID:           upstreamID,
			SpanID:       pgtype.Text{String: metaID, Valid: true},
			ParentSpanID: pgtype.Text{Valid: false},
			Type:         db.RequestTypeUpstream,
			Status:       db.RequestStatusPending,
			ProviderID:   pgtype.Int4{Int32: providerID, Valid: true},
			EndpointPath: pgtype.Text{String: endpoint.Path, Valid: true},
			ApiKeyID:     pgtype.Int4{Valid: false},
			Model:        pgtype.Text{String: modelName, Valid: true},
			StatusCode:   pgtype.Int4{Valid: false},
			ErrorMessage: pgtype.Text{Valid: false},
			TimeSpentMs:  pgtype.Int4{Valid: false},
		})

		upstreamModel := candidateUpstreamModel(cand)

		req, reqBody, berr := buildUpstreamRequest(ctx, r, body, side.upstreamURL, upstreamModel, side.credentials, endpoint.CredentialsResolver)
		if berr != nil {
			cancel()
			h.completeFailedAttempt(bgCtx, upstreamID, attemptStart, 0, berr.Error())
			lastErr = berr
			lastJSErr = &jsx.LastError{ProviderID: int(providerID), StatusCode: 0, Message: berr.Error()}
			currentRetryCount++
			totalAttemptCount++
			continue
		}

		// rewriteRequest hook. Serialize the upstream request, hand it to JS,
		// then rebuild a fresh *http.Request from whatever the hook returns —
		// no mutate-in-place, so the outgoing request is exactly the JSON
		// shape JS produced.
		newPending, rerr := session.RunRewriteHook(jsx.RewriteInput{
			Endpoint:          endpoint,
			Model:             nil,
			Provider:          cand.Provider,
			MPE:               cand.MPE,
			CurrentRetryCount: currentRetryCount,
			TotalAttemptCount: totalAttemptCount,
			ClientRequest:     jsClientRequest,
			PendingRequest:    serializePendingRequest(req, reqBody),
		})
		if rerr != nil {
			cancel()
			failHook(rerr)
			return
		}
		req, reqBody, rerr = buildRequestFromPending(ctx, newPending, reqBody)
		if rerr != nil {
			cancel()
			failHook(rerr)
			return
		}

		// Upload upstream request artifact AFTER rewrite so it reflects what was sent.
		h.uploadRequestArtifact(bgCtx, upstreamID, upstreamCreatedAt, req.Method, req.URL.String(), req.Header.Clone(), reqBody)

		upstreamStartTime := time.Now()
		resp, err := h.forwardRequest(req)
		if err != nil {
			cancel()
			h.completeFailedAttempt(bgCtx, upstreamID, attemptStart, 0, err.Error())
			lastErr = err
			lastJSErr = &jsx.LastError{ProviderID: int(providerID), StatusCode: 0, Message: err.Error()}
			currentRetryCount++
			totalAttemptCount++
			continue
		}

		if resp.StatusCode == http.StatusOK {
			h.streamSuccess(w, r, ctx, cancel, resp, upstreamID, upstreamCreatedAt, attemptStart, metaID, metaCreatedAt, gatewayStart, providerID, modelName, endpoint.Path, upstreamStartTime, bgCtx)
			return
		}

		// Non-200: record + try again.
		cancel()
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		h.uploadResponseArtifact(bgCtx, upstreamID, upstreamCreatedAt, resp.StatusCode, resp.Header.Clone(), respBody)
		errMsg := string(respBody)
		h.updateRequestOnComplete(bgCtx, db.UpdateRequestOnCompleteParams{
			ID:           upstreamID,
			StatusCode:   pgtype.Int4{Int32: int32(resp.StatusCode), Valid: true},
			ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
			TimeSpentMs:  pgtype.Int4{Int32: int32(time.Since(attemptStart).Milliseconds()), Valid: true},
			Status:       db.RequestStatusFailed,
		})
		lastErr = fmt.Errorf("upstream returned %d: %s", resp.StatusCode, errMsg)
		lastJSErr = &jsx.LastError{ProviderID: int(providerID), StatusCode: resp.StatusCode, Message: errMsg}
		currentRetryCount++
		totalAttemptCount++
	}

	// 9. All providers failed (or attempts cap reached) — fail meta with 502.
	errMsg := "all providers failed"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	failMeta(http.StatusBadGateway, errMsg)
	respBody := writeGatewayError(w, http.StatusBadGateway, errMsg, errorx.UpstreamError.Error())
	h.uploadResponseArtifact(bgCtx, metaID, metaCreatedAt, http.StatusBadGateway, w.Header().Clone(), respBody)
}

func mapLowerKeys(header http.Header) http.Header {
	lower := make(http.Header, len(header))
	for k, v := range header {
		lower[strings.ToLower(k)] = v
	}
	return lower
}

// uploadRequestArtifact builds and asynchronously uploads a request artifact for the given id+ts.
func (h *gatewayHandler) uploadRequestArtifact(ctx context.Context, id string, ts time.Time, method, url string, header http.Header, body []byte) {
	if !h.artifacts.Enabled() {
		return
	}
	payload, err := artifacts.BuildRequest(method, url, header, body)
	if err != nil {
		logx.WithContext(ctx).WithError(err).WithField("id", id).Warn("artifact: build request failed")
		return
	}
	h.artifacts.Put(ctx, artifacts.RequestKey(id, ts), payload)
}

// uploadResponseArtifact builds and asynchronously uploads a response artifact for the given id+ts.
func (h *gatewayHandler) uploadResponseArtifact(ctx context.Context, id string, ts time.Time, statusCode int, header http.Header, body []byte) {
	if !h.artifacts.Enabled() {
		return
	}
	payload, err := artifacts.BuildResponse(statusCode, header, body)
	if err != nil {
		logx.WithContext(ctx).WithError(err).WithField("id", id).Warn("artifact: build response failed")
		return
	}
	h.artifacts.Put(ctx, artifacts.ResponseKey(id, ts), payload)
}

// streamSuccess writes the upstream 200 response back to the client and
// completes both the upstream and meta request rows. Pulled out of the main
// handler so the retry loop body stays scannable.
func (h *gatewayHandler) streamSuccess(
	w http.ResponseWriter, r *http.Request,
	ctx context.Context, cancel context.CancelFunc, resp *http.Response,
	upstreamID string, upstreamCreatedAt time.Time, attemptStart time.Time,
	metaID string, metaCreatedAt time.Time, gatewayStart time.Time,
	providerID int32, modelName, endpointPath string,
	upstreamStartTime time.Time,
	bgCtx context.Context,
) {
	h.updateRequestOnHeader(bgCtx, db.UpdateRequestOnHeaderParams{
		ID:           metaID,
		ProviderID:   pgtype.Int4{Int32: providerID, Valid: true},
		Model:        pgtype.Text{String: modelName, Valid: true},
		EndpointPath: pgtype.Text{String: endpointPath, Valid: true},
		ApiKeyID:     pgtype.Int4{Valid: false},
		Status:       db.RequestStatusHeaderReceived,
	})
	h.updateRequestOnHeader(bgCtx, db.UpdateRequestOnHeaderParams{
		ID:           upstreamID,
		ProviderID:   pgtype.Int4{Int32: providerID, Valid: true},
		Model:        pgtype.Text{String: modelName, Valid: true},
		EndpointPath: pgtype.Text{String: endpointPath, Valid: true},
		ApiKeyID:     pgtype.Int4{Valid: false},
		Status:       db.RequestStatusHeaderReceived,
	})

	for key, values := range resp.Header {
		if strings.ToLower(key) == "content-length" {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	metaRespHeader := w.Header().Clone()
	w.WriteHeader(http.StatusOK)

	extractor := NewResponseExtractor(resp.Body, resp.Header.Get("Content-Type"), upstreamStartTime)
	reader := newIdleTimeoutReader(extractor, h.config.GatewayReadTimeout, cancel)
	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 32*1024)
	var captureBuf bytes.Buffer
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			captureBuf.Write(buf[:n])
			if canFlush {
				flusher.Flush()
			}
		}
		if readErr != nil {
			break
		}
	}
	cancel()
	resp.Body.Close()

	respBytes := captureBuf.Bytes()
	h.uploadResponseArtifact(bgCtx, upstreamID, upstreamCreatedAt, resp.StatusCode, resp.Header.Clone(), respBytes)
	h.uploadResponseArtifact(bgCtx, metaID, metaCreatedAt, http.StatusOK, metaRespHeader, respBytes)

	m := extractor.Metrics()
	ttftMs, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens := metricsToPG(m)

	upstreamTimeSpent := int32(time.Since(attemptStart).Milliseconds())
	h.updateRequestOnComplete(bgCtx, db.UpdateRequestOnCompleteParams{
		ID:               upstreamID,
		StatusCode:       pgtype.Int4{Int32: int32(resp.StatusCode), Valid: true},
		ErrorMessage:     pgtype.Text{Valid: false},
		TimeSpentMs:      pgtype.Int4{Int32: upstreamTimeSpent, Valid: true},
		Status:           db.RequestStatusCompleted,
		TtftMs:           ttftMs,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		CacheReadTokens:  cacheReadTokens,
		CacheWriteTokens: cacheWriteTokens,
	})

	metaTimeSpent := int32(time.Since(gatewayStart).Milliseconds())
	h.updateRequestOnComplete(bgCtx, db.UpdateRequestOnCompleteParams{
		ID:               metaID,
		StatusCode:       pgtype.Int4{Int32: int32(resp.StatusCode), Valid: true},
		ErrorMessage:     pgtype.Text{Valid: false},
		TimeSpentMs:      pgtype.Int4{Int32: metaTimeSpent, Valid: true},
		Status:           db.RequestStatusCompleted,
		TtftMs:           ttftMs,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		CacheReadTokens:  cacheReadTokens,
		CacheWriteTokens: cacheWriteTokens,
	})
	_ = r // kept for interface symmetry; r.Context() may be useful for future hooks
}
