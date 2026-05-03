package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"picotera/pkg/contract"
	"picotera/pkg/db"
	"picotera/pkg/errorx"
	"picotera/pkg/jsx"
	"picotera/pkg/logx"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// gatewayError represents an error that should be returned to the client
// with a specific HTTP status code and error code.
type gatewayError struct {
	status  int
	message string
	code    string
}

func (e *gatewayError) Error() string { return e.message }

// isRouteNotFound reports whether err is a gatewayError signalling that no
// configured LLM endpoint matches the requested path.
func isRouteNotFound(err error) bool {
	var gw *gatewayError
	return errors.As(err, &gw) && gw.code == errorx.RouteNotFound.Error()
}

// looksLikeBrowserNav reports whether the request is a safe navigation that
// can fall through to the dashboard SPA when no LLM endpoint matches.
// API clients (POST, Accept: application/json) are excluded so they receive
// the structured gateway 404 they expect.
func looksLikeBrowserNav(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	accept := r.Header.Get("Accept")
	if accept == "" {
		return true
	}
	lower := strings.ToLower(accept)
	return strings.Contains(lower, "text/html") || strings.Contains(lower, "*/*")
}

// writeGatewayError writes a structured error response in the format:
// {"message":"...","code":"...","details":[]}.
// Returns the bytes written to the body (for artifact capture).
func writeGatewayError(w http.ResponseWriter, status int, message, code string) []byte {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body, _ := json.Marshal(map[string]any{
		"message": message,
		"code":    code,
		"details": []string{},
	})
	body = append(body, '\n')
	w.Write(body)
	return body
}

// handleGatewayErr writes a gateway error response. If err is a *gatewayError,
// its status, message, and code are used; otherwise a 500 INTERNAL_ERROR is returned.
// Returns (status, body) for artifact capture.
func handleGatewayErr(w http.ResponseWriter, err error) (int, []byte) {
	var gwErr *gatewayError
	if err != nil && errors.As(err, &gwErr) {
		return gwErr.status, writeGatewayError(w, gwErr.status, gwErr.message, gwErr.code)
	}
	return http.StatusInternalServerError, writeGatewayError(w, http.StatusInternalServerError, "internal error", errorx.InternalError.Error())
}

// resolveEndpoint matches the request path to an endpoint in the database.
func (s *Server) resolveEndpoint(ctx context.Context, path string) (db.Endpoint, error) {
	endpoint, err := s.queries.GetEndpointByPath(ctx, path)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Endpoint{}, &gatewayError{
				status:  http.StatusNotFound,
				message: "route not found",
				code:    errorx.RouteNotFound.Error(),
			}
		}
		// Real DB error (not "no row matched"). The early-fail path skips the
		// request log, so make sure these stay visible.
		logx.WithContext(ctx).WithError(err).WithField("path", path).Error("endpoint lookup failed")
		return db.Endpoint{}, &gatewayError{
			status:  http.StatusInternalServerError,
			message: "failed to query endpoint",
			code:    errorx.InternalError.Error(),
		}
	}
	return endpoint, nil
}

// validateClientAuth checks that the client request includes credentials.
// Returns a gatewayError if credentials are missing.
func validateClientAuth(r *http.Request) error {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return nil
	}
	apiKey := r.Header.Get("X-Api-Key")
	if apiKey != "" {
		return nil
	}
	return &gatewayError{
		status:  http.StatusUnauthorized,
		message: "missing credentials",
		code:    errorx.Unauthorized.Error(),
	}
}

// setCredentialsHeaders sets the appropriate authentication headers based on the
// credentials resolver type. For generalApiKey with a source request, it infers
// the auth style from the source request (preserving existing gateway behavior).
// For generalApiKey without a source request, both headers are sent as fallback.
// For bearerToken and xApiKey, the source request is ignored.
func setCredentialsHeaders(headers http.Header, credentials string, resolver int32, sourceRequest *http.Request) {
	if credentials == "" {
		return
	}
	switch resolver {
	case contract.CredentialsResolver_GeneralApiKey:
		if sourceRequest != nil {
			auth := sourceRequest.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				headers.Set("Authorization", "Bearer "+credentials)
			} else {
				apiKey := sourceRequest.Header.Get("X-Api-Key")
				if apiKey != "" {
					headers.Set("X-Api-Key", credentials)
				} else {
					headers.Set("Authorization", "Bearer "+credentials)
					headers.Set("X-Api-Key", credentials)
				}
			}
		} else {
			headers.Set("Authorization", "Bearer "+credentials)
			headers.Set("X-Api-Key", credentials)
		}
	case contract.CredentialsResolver_BearerToken:
		headers.Set("Authorization", "Bearer "+credentials)
	case contract.CredentialsResolver_XApiKey:
		headers.Set("X-Api-Key", credentials)
	}
}

// extractModel extracts the model name from the request body using the given JSON path.
func extractModel(body []byte, modelPath string) (string, error) {
	if modelPath == "" {
		return "", &gatewayError{
			status:  http.StatusBadRequest,
			message: "endpoint has no model path configured",
			code:    errorx.ModelNotFound.Error(),
		}
	}
	result := gjson.GetBytes(body, modelPath)
	if !result.Exists() || result.Str == "" {
		return "", &gatewayError{
			status:  http.StatusBadRequest,
			message: "model not found in request body",
			code:    errorx.ModelNotFound.Error(),
		}
	}
	return result.Str, nil
}

// resolveProviders gets providers for the given endpoint and model, filters out
// those without upstream URLs, and sorts by combined priority (descending).
func (s *Server) resolveProviders(ctx context.Context, endpointPath, model string) ([]db.GetProvidersByEndpointAndModelRow, error) {
	rows, err := s.queries.GetProvidersByEndpointAndModel(ctx, db.GetProvidersByEndpointAndModelParams{
		EndpointPath: endpointPath,
		ModelName:    model,
	})
	if err != nil {
		return nil, &gatewayError{
			status:  http.StatusInternalServerError,
			message: "failed to query providers",
			code:    errorx.InternalError.Error(),
		}
	}
	if len(rows) == 0 {
		return nil, &gatewayError{
			status:  http.StatusNotFound,
			message: "no provider available for model",
			code:    errorx.NoProviderAvailable.Error(),
		}
	}

	// Filter out providers without upstream URLs or credentials
	valid := make([]db.GetProvidersByEndpointAndModelRow, 0, len(rows))
	for _, row := range rows {
		if row.UpstreamUrl != "" && row.ProviderCredentials != "" {
			valid = append(valid, row)
		}
	}

	if len(valid) == 0 {
		return nil, &gatewayError{
			status:  http.StatusNotFound,
			message: "no provider available for model",
			code:    errorx.NoProviderAvailable.Error(),
		}
	}

	// Sort by combined priority (provider_priority + per-model priority) descending.
	sort.Slice(valid, func(i, j int) bool {
		pi := int(valid[i].Priority) + int(valid[i].ProviderPriority)
		pj := int(valid[j].Priority) + int(valid[j].ProviderPriority)
		return pi > pj
	})

	return valid, nil
}

// buildUpstreamRequest constructs the upstream HTTP request.
// It copies headers from the original request, replaces the model name in the body
// if upstreamModel differs, and sets credentials based on the auth type.
// The provided ctx is used for the request context, enabling cancellation of
// upstream reads (e.g., by the idle timeout reader).
func buildUpstreamRequest(ctx context.Context, original *http.Request, body []byte, upstreamURL, upstreamModel, creds string, resolver int32) (*http.Request, []byte, error) {
	// Replace model name if upstream_model_name is set
	reqBody := body
	if upstreamModel != "" {
		var err error
		reqBody, err = sjson.SetBytes(body, "model", upstreamModel)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to set model in request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, original.Method, upstreamURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create upstream request: %w", err)
	}

	// Copy headers from original request, excluding auth headers, Host, and Content-Length
	for key, values := range original.Header {
		lower := strings.ToLower(key)
		if lower == "authorization" || lower == "x-api-key" || lower == "host" || lower == "content-length" {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Set credentials based on resolver type
	setCredentialsHeaders(req.Header, creds, resolver, original)

	req.ContentLength = int64(len(reqBody))

	return req, reqBody, nil
}

// forwardRequest sends the request to the upstream provider using the shared HTTP client.
func (s *Server) forwardRequest(req *http.Request) (*http.Response, error) {
	return s.httpClient.Do(req)
}

// insertRequest inserts a request record and returns the DB-assigned created_at.
// On error, returns time.Now().UTC() so artifact keys remain computable.
func (s *Server) insertRequest(ctx context.Context, arg db.InsertRequestParams) time.Time {
	createdAt, err := s.queries.InsertRequest(ctx, arg)
	if err != nil {
		logx.WithContext(ctx).WithError(err).Error("failed to insert request")
		return time.Now().UTC()
	}
	if !createdAt.Valid {
		return time.Now().UTC()
	}
	return createdAt.Time
}

// updateRequestOnHeader backfills provider and request metadata. Errors are logged but do not affect the response.
func (s *Server) updateRequestOnHeader(ctx context.Context, arg db.UpdateRequestOnHeaderParams) {
	if err := s.queries.UpdateRequestOnHeader(ctx, arg); err != nil {
		logx.WithContext(ctx).WithError(err).Error("failed to update request on header")
	}
}

// updateRequestModel backfills the model field early. Errors are logged but do not affect the response.
func (s *Server) updateRequestModel(ctx context.Context, arg db.UpdateRequestModelParams) {
	if err := s.queries.UpdateRequestModel(ctx, arg); err != nil {
		logx.WithContext(ctx).WithError(err).Error("failed to update request model")
	}
}

// updateRequestOnComplete backfills result fields. Errors are logged but do not affect the response.
func (s *Server) updateRequestOnComplete(ctx context.Context, arg db.UpdateRequestOnCompleteParams) {
	if err := s.queries.UpdateRequestOnComplete(ctx, arg); err != nil {
		logx.WithContext(ctx).WithError(err).Error("failed to update request on complete")
	}
}

// metricsToPG converts ResponseMetrics to pgtype fields for DB queries.
func metricsToPG(m ResponseMetrics) (ttftMs pgtype.Int4, inputTokens pgtype.Int4, outputTokens pgtype.Int4, cacheReadTokens pgtype.Int4, cacheWriteTokens pgtype.Int4) {
	if m.TTFTMs != nil {
		ttftMs = pgtype.Int4{Int32: int32(*m.TTFTMs), Valid: true}
	}
	if m.InputTokens != nil {
		inputTokens = pgtype.Int4{Int32: int32(*m.InputTokens), Valid: true}
	}
	if m.OutputTokens != nil {
		outputTokens = pgtype.Int4{Int32: int32(*m.OutputTokens), Valid: true}
	}
	if m.CacheReadTokens != nil {
		cacheReadTokens = pgtype.Int4{Int32: int32(*m.CacheReadTokens), Valid: true}
	}
	if m.CacheWriteTokens != nil {
		cacheWriteTokens = pgtype.Int4{Int32: int32(*m.CacheWriteTokens), Valid: true}
	}
	return
}

// candidateProviderID extracts the provider ID from a hook-returned candidate.
// JS round-trips numbers as float64, so we accept both int32 (Go-side construction)
// and float64 (post-JSON) shapes. Returns false if the field is missing or malformed.
func candidateProviderID(c jsx.Candidate) (int32, bool) {
	pm, ok := c.Provider.(map[string]any)
	if !ok {
		return 0, false
	}
	switch v := pm["id"].(type) {
	case float64:
		return int32(v), true
	case int32:
		return v, true
	case int:
		return int32(v), true
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return int32(n), true
		}
	}
	return 0, false
}

// candidateUpstreamModel pulls upstreamModelName from a candidate's mpe field.
// Empty string means "use the model name from the request body verbatim",
// matching the existing buildUpstreamRequest contract.
func candidateUpstreamModel(c jsx.Candidate) string {
	mm, ok := c.MPE.(map[string]any)
	if !ok {
		return ""
	}
	if v, ok := mm["upstreamModelName"].(string); ok {
		return v
	}
	return ""
}

// isJSONContentType reports whether the given Content-Type header value
// (possibly with parameters like "; charset=utf-8") names application/json.
func isJSONContentType(ct string) bool {
	ct = strings.TrimSpace(ct)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.EqualFold(strings.TrimSpace(ct), "application/json")
}

// jsonBodyOrNil returns body wrapped as json.RawMessage when headers signal
// application/json and body is itself valid JSON; otherwise nil so the field
// is omitted from the JS-visible shape.
func jsonBodyOrNil(headers http.Header, body []byte) json.RawMessage {
	if !isJSONContentType(headers.Get("Content-Type")) {
		return nil
	}
	if !json.Valid(body) {
		return nil
	}
	return json.RawMessage(body)
}

// serializePendingRequest captures the upstream request as a PendingRequestShape
// for the rewriteRequest hook. Headers are lower-cased; body follows the
// content-type rule (only application/json bodies are exposed to JS).
func serializePendingRequest(req *http.Request, body []byte) jsx.PendingRequestShape {
	return jsx.PendingRequestShape{
		URL:     req.URL.String(),
		Method:  req.Method,
		Headers: mapLowerKeys(req.Header.Clone()),
		Body:    jsonBodyOrNil(req.Header, body),
	}
}

// serializeClientRequest captures the inbound client request as a RequestShape
// for the JS hooks. Same body rule as serializePendingRequest.
func serializeClientRequest(r *http.Request, body []byte, model string) jsx.RequestShape {
	return jsx.RequestShape{
		Path:    r.URL.Path,
		Method:  r.Method,
		Headers: mapLowerKeys(r.Header.Clone()),
		Model:   model,
		Body:    jsonBodyOrNil(r.Header, body),
	}
}

// buildRequestFromPending constructs a fresh *http.Request from the rewrite
// hook's returned PendingRequestShape. fallbackBody is used when p.Body is
// absent (non-JSON content-type / hidden from JS) — those bytes are sent as
// the request body verbatim. When p.Body is present it carries a JSON string
// token (the SDK stringifies object bodies before they reach Go); the inner
// string contents become the outgoing body bytes.
func buildRequestFromPending(ctx context.Context, p jsx.PendingRequestShape, fallbackBody []byte) (*http.Request, []byte, error) {
	outBody := fallbackBody
	if p.Body != nil {
		var s string
		if err := json.Unmarshal(p.Body, &s); err != nil {
			return nil, nil, fmt.Errorf("rewriteRequest: decode body: %w", err)
		}
		outBody = []byte(s)
	}
	req, err := http.NewRequestWithContext(ctx, p.Method, p.URL, bytes.NewReader(outBody))
	if err != nil {
		return nil, nil, fmt.Errorf("rewriteRequest: build request: %w", err)
	}
	req.Header = http.Header{}
	for k, vv := range p.Headers {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	req.ContentLength = int64(len(outBody))
	return req, outBody, nil
}

// completeFailedAttempt is a small wrapper around updateRequestOnComplete for the
// retry loop's error path.
func (s *Server) completeFailedAttempt(ctx context.Context, upstreamID string, attemptStart time.Time, statusCode int32, errMsg string) {
	s.updateRequestOnComplete(ctx, db.UpdateRequestOnCompleteParams{
		ID:           upstreamID,
		StatusCode:   pgtype.Int4{Int32: statusCode, Valid: true},
		ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
		TimeSpentMs:  pgtype.Int4{Int32: int32(time.Since(attemptStart).Milliseconds()), Valid: true},
		Status:       db.RequestStatusFailed,
	})
}

// idleTimeoutReader wraps an io.Reader and enforces a per-read idle timeout.
// If no data is received within the timeout period, the read is cancelled
// via the provided cancel function.
type idleTimeoutReader struct {
	reader  io.Reader
	timeout time.Duration
	cancel  context.CancelFunc
}

func newIdleTimeoutReader(reader io.Reader, timeout time.Duration, cancel context.CancelFunc) *idleTimeoutReader {
	return &idleTimeoutReader{reader: reader, timeout: timeout, cancel: cancel}
}

func (r *idleTimeoutReader) Read(p []byte) (int, error) {
	type result struct {
		n   int
		err error
	}
	ch := make(chan result, 1)
	go func() {
		n, err := r.reader.Read(p)
		ch <- result{n, err}
	}()

	timer := time.NewTimer(r.timeout)
	defer timer.Stop()

	select {
	case res := <-ch:
		return res.n, res.err
	case <-timer.C:
		r.cancel()
		res := <-ch
		return res.n, fmt.Errorf("gateway: read idle timeout after %v: %w", r.timeout, res.err)
	}
}