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
	"picotera/pkg/logx"

	"github.com/google/uuid"
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
// {"message":"...","code":"...","details":[]}
func writeGatewayError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"message": message,
		"code":    code,
		"details": []string{},
	})
}

// handleGatewayErr writes a gateway error response. If err is a *gatewayError,
// its status, message, and code are used; otherwise a 500 INTERNAL_ERROR is returned.
func handleGatewayErr(w http.ResponseWriter, err error) {
	var gwErr *gatewayError
	if err != nil && errors.As(err, &gwErr) {
		writeGatewayError(w, gwErr.status, gwErr.message, gwErr.code)
	} else {
		writeGatewayError(w, http.StatusInternalServerError, "internal error", errorx.InternalError.Error())
	}
}

// resolveEndpoint matches the request path to an endpoint in the database.
func (s *Server) resolveEndpoint(ctx context.Context, path string) (db.Endpoint, error) {
	endpoint, err := s.queries.GetEndpointByPath(ctx, path)
	if err != nil {
		return db.Endpoint{}, &gatewayError{
			status:  http.StatusNotFound,
			message: "route not found",
			code:    errorx.RouteNotFound.Error(),
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
func buildUpstreamRequest(ctx context.Context, original *http.Request, body []byte, upstreamURL, upstreamModel, creds string, auth authType) (*http.Request, error) {
	// Replace model name if upstream_model_name is set
	reqBody := body
	if upstreamModel != "" {
		var err error
		reqBody, err = sjson.SetBytes(body, "model", upstreamModel)
		if err != nil {
			return nil, fmt.Errorf("failed to set model in request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, original.Method, upstreamURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream request: %w", err)
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

	return req, nil
}

// forwardRequest sends the request to the upstream provider using the shared HTTP client.
func (s *Server) forwardRequest(req *http.Request) (*http.Response, error) {
	return s.httpClient.Do(req)
}

// logRequest records the attempt in the request table.
// Errors during logging are reported but do not affect the response.
func (s *Server) logRequest(providerID int32, endpointPath, model string, statusCode int32, errorMessage string, timeSpentMs int32) {
	err := s.queries.InsertRequest(context.Background(), db.InsertRequestParams{
		ID:           uuid.New().String(),
		ProviderID:   providerID,
		EndpointPath: endpointPath,
		Model:        pgtype.Text{String: model, Valid: model != ""},
		StatusCode:   statusCode,
		ErrorMessage: pgtype.Text{String: errorMessage, Valid: errorMessage != ""},
		TimeSpentMs:  timeSpentMs,
	})
	if err != nil {
		logx.WithContext(context.Background()).WithError(err).Error("failed to log request")
	}
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