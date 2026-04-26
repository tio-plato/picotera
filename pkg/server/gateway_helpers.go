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

	"picotera/pkg/db"
	"picotera/pkg/errorx"
	"picotera/pkg/jsx"
	"picotera/pkg/logx"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// authType represents the authentication method used by the client.
type authType int

const (
	authTypeBearer authType = iota
	authTypeAPIKey
)

const credentialsResolverGeneralAPIKey = 1

// gatewayError represents an error that should be returned to the client
// with a specific HTTP status code and error code.
type gatewayError struct {
	status  int
	message string
	code    string
}

func (e *gatewayError) Error() string { return e.message }

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
		return db.Endpoint{}, &gatewayError{
			status:  http.StatusInternalServerError,
			message: "failed to query endpoint",
			code:    errorx.InternalError.Error(),
		}
	}
	return endpoint, nil
}

// resolveAuthType determines the authentication method from the client request headers.
// Returns the auth type and nil on success, or a gatewayError if credentials are missing.
func resolveAuthType(r *http.Request) (authType, error) {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return authTypeBearer, nil
	}
	apiKey := r.Header.Get("X-Api-Key")
	if apiKey != "" {
		return authTypeAPIKey, nil
	}
	return 0, &gatewayError{
		status:  http.StatusUnauthorized,
		message: "missing credentials",
		code:    errorx.Unauthorized.Error(),
	}
}

// extractModel extracts the model name from the request body using the given JSON path.
func extractModel(body []byte, modelPath string) (string, error) {
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
		if row.UpstreamUrl.Valid && row.UpstreamUrl.String != "" && row.ProviderCredentials.Valid {
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

	// Sort by combined priority (provider_priority + mpe.priority) descending.
	// ProviderPriority is pgtype.Int4 (nullable from LEFT JOIN); treat NULL as 0.
	sort.Slice(valid, func(i, j int) bool {
		pi := int(valid[i].Priority)
		if valid[i].ProviderPriority.Valid {
			pi += int(valid[i].ProviderPriority.Int32)
		}
		pj := int(valid[j].Priority)
		if valid[j].ProviderPriority.Valid {
			pj += int(valid[j].ProviderPriority.Int32)
		}
		return pi > pj
	})

	return valid, nil
}

// buildUpstreamRequest constructs the upstream HTTP request.
// It copies headers from the original request, replaces the model name in the body
// if upstreamModel differs, and sets credentials based on the auth type.
// The provided ctx is used for the request context, enabling cancellation of
// upstream reads (e.g., by the idle timeout reader).
func buildUpstreamRequest(ctx context.Context, original *http.Request, body []byte, upstreamURL, upstreamModel, creds string, auth authType) (*http.Request, []byte, error) {
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

	// Set credentials based on auth type
	switch auth {
	case authTypeBearer:
		req.Header.Set("Authorization", "Bearer "+creds)
	case authTypeAPIKey:
		req.Header.Set("X-Api-Key", creds)
	}

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

// applyRewrite mutates req and reqBody based on the JS rewrite hook output.
// Nil pointer fields and empty Body mean "leave unchanged".
func applyRewrite(req *http.Request, reqBody *[]byte, rw jsx.RewriteOutput) {
	if rw.URL != nil {
		if parsed, perr := http.NewRequestWithContext(req.Context(), req.Method, *rw.URL, nil); perr == nil {
			req.URL = parsed.URL
			req.Host = parsed.Host
		}
	}
	if rw.Method != nil {
		req.Method = *rw.Method
	}
	if rw.Headers != nil {
		req.Header = http.Header{}
		for k, vv := range *rw.Headers {
			for _, v := range vv {
				req.Header.Add(k, v)
			}
		}
	}
	if len(rw.Body) > 0 {
		// rw.Body is JSON-encoded. If it's a JSON string, unwrap to its
		// contents; otherwise pass the raw JSON through (object/array bodies
		// are already JSON.stringify'd by the SDK, so they too land as
		// JSON strings — this branch is for safety).
		var asString string
		if jerr := json.Unmarshal(rw.Body, &asString); jerr == nil {
			*reqBody = []byte(asString)
		} else {
			*reqBody = []byte(rw.Body)
		}
		req.Body = io.NopCloser(bytes.NewReader(*reqBody))
		req.ContentLength = int64(len(*reqBody))
	}
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