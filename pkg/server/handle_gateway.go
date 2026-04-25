package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"picotera/pkg/db"
	"picotera/pkg/errorx"

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

	// 1. Read request body
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		writeGatewayError(w, http.StatusInternalServerError, "failed to read request body", errorx.InternalError.Error())
		return
	}

	// 2. Insert meta request immediately on arrival
	metaID := xid.New().String()
	h.insertRequest(bgCtx, db.InsertRequestParams{
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

	failMeta := func(statusCode int32, errMsg string) {
		h.updateRequestOnComplete(bgCtx, db.UpdateRequestOnCompleteParams{
			ID:           metaID,
			StatusCode:   pgtype.Int4{Int32: statusCode, Valid: true},
			ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
			TimeSpentMs:  pgtype.Int4{Int32: int32(time.Since(gatewayStart).Milliseconds()), Valid: true},
			Status:       db.RequestStatusFailed,
		})
	}

	// 3. Match endpoint by path
	endpoint, err := h.resolveEndpoint(r.Context(), r.URL.Path)
	if err != nil {
		var gwErr *gatewayError
		if errors.As(err, &gwErr) {
			failMeta(int32(gwErr.status), gwErr.message)
		} else {
			failMeta(http.StatusInternalServerError, "failed to query endpoint")
		}
		handleGatewayErr(w, err)
		return
	}

	// 4. Check credentials_resolver (only generalApiKey = 1 is supported in v1)
	if endpoint.CredentialsResolver != credentialsResolverGeneralAPIKey {
		errMsg := fmt.Sprintf("unsupported credentials resolver: %d", endpoint.CredentialsResolver)
		failMeta(http.StatusInternalServerError, errMsg)
		writeGatewayError(w, http.StatusInternalServerError, errMsg, errorx.InternalError.Error())
		return
	}

	// 5. Resolve auth type from client headers
	authTyp, err := resolveAuthType(r)
	if err != nil {
		var gwErr *gatewayError
		if errors.As(err, &gwErr) {
			failMeta(int32(gwErr.status), gwErr.message)
		} else {
			failMeta(http.StatusInternalServerError, "auth resolution failed")
		}
		handleGatewayErr(w, err)
		return
	}

	// 6. Extract model name from request body
	model, err := extractModel(body, endpoint.ModelPath)
	if err != nil {
		var gwErr *gatewayError
		if errors.As(err, &gwErr) {
			failMeta(int32(gwErr.status), gwErr.message)
		} else {
			failMeta(http.StatusBadRequest, "model extraction failed")
		}
		handleGatewayErr(w, err)
		return
	}

	// 7. Resolve providers
	providers, err := h.resolveProviders(r.Context(), endpoint.Path, model)
	if err != nil {
		var gwErr *gatewayError
		if errors.As(err, &gwErr) {
			failMeta(int32(gwErr.status), gwErr.message)
		} else {
			failMeta(http.StatusInternalServerError, "failed to query providers")
		}
		handleGatewayErr(w, err)
		return
	}

	// 8. Try each provider with failover
	var lastErr error
	for _, provider := range providers {
		attemptStart := time.Now()
		ctx, cancel := context.WithCancel(r.Context())

		// Insert upstream request
		upstreamID := xid.New().String()
		h.insertRequest(bgCtx, db.InsertRequestParams{
			ID:           upstreamID,
			SpanID:       pgtype.Text{String: metaID, Valid: true},
			ParentSpanID: pgtype.Text{Valid: false},
			Type:         db.RequestTypeUpstream,
			Status:       db.RequestStatusPending,
			ProviderID:   pgtype.Int4{Int32: provider.ProviderID, Valid: true},
			EndpointPath: pgtype.Text{String: endpoint.Path, Valid: true},
			ApiKeyID:     pgtype.Int4{Valid: false},
			Model:        pgtype.Text{String: model, Valid: true},
			StatusCode:   pgtype.Int4{Valid: false},
			ErrorMessage: pgtype.Text{Valid: false},
			TimeSpentMs:  pgtype.Int4{Valid: false},
		})

		// Determine upstream model name
		upstreamModel := ""
		if provider.UpstreamModelName.Valid {
			upstreamModel = provider.UpstreamModelName.String
		}

		// Determine provider credentials
		creds := ""
		if provider.ProviderCredentials.Valid {
			creds = provider.ProviderCredentials.String
		}

		// Build upstream request
		req, err := buildUpstreamRequest(ctx, r, body, provider.UpstreamUrl.String, upstreamModel, creds, authTyp)
		if err != nil {
			cancel()
			timeSpent := int32(time.Since(attemptStart).Milliseconds())
			h.updateRequestOnComplete(bgCtx, db.UpdateRequestOnCompleteParams{
				ID:           upstreamID,
				StatusCode:   pgtype.Int4{Int32: 0, Valid: true},
				ErrorMessage: pgtype.Text{String: err.Error(), Valid: true},
				TimeSpentMs:  pgtype.Int4{Int32: timeSpent, Valid: true},
				Status:       db.RequestStatusFailed,
			})
			lastErr = err
			continue
		}

		// Forward request
		resp, err := h.forwardRequest(req)
		if err != nil {
			cancel()
			timeSpent := int32(time.Since(attemptStart).Milliseconds())
			h.updateRequestOnComplete(bgCtx, db.UpdateRequestOnCompleteParams{
				ID:           upstreamID,
				StatusCode:   pgtype.Int4{Int32: 0, Valid: true},
				ErrorMessage: pgtype.Text{String: err.Error(), Valid: true},
				TimeSpentMs:  pgtype.Int4{Int32: timeSpent, Valid: true},
				Status:       db.RequestStatusFailed,
			})
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusOK {
			// Backfill meta and upstream on header success
			h.updateRequestOnHeader(bgCtx, db.UpdateRequestOnHeaderParams{
				ID:           metaID,
				ProviderID:   pgtype.Int4{Int32: provider.ProviderID, Valid: true},
				Model:        pgtype.Text{String: model, Valid: true},
				EndpointPath: pgtype.Text{String: endpoint.Path, Valid: true},
				ApiKeyID:     pgtype.Int4{Valid: false},
				Status:       db.RequestStatusHeaderReceived,
			})
			h.updateRequestOnHeader(bgCtx, db.UpdateRequestOnHeaderParams{
				ID:           upstreamID,
				ProviderID:   pgtype.Int4{Int32: provider.ProviderID, Valid: true},
				Model:        pgtype.Text{String: model, Valid: true},
				EndpointPath: pgtype.Text{String: endpoint.Path, Valid: true},
				ApiKeyID:     pgtype.Int4{Valid: false},
				Status:       db.RequestStatusHeaderReceived,
			})

			// Stream response to client, stripping Content-Length since we're chunk-copying
			for key, values := range resp.Header {
				if strings.ToLower(key) == "content-length" {
					continue
				}
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			w.WriteHeader(http.StatusOK)

			reader := newIdleTimeoutReader(resp.Body, h.config.GatewayReadTimeout, cancel)
			flusher, canFlush := w.(http.Flusher)
			buf := make([]byte, 32*1024)
			for {
				n, readErr := reader.Read(buf)
				if n > 0 {
					w.Write(buf[:n])
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

			// Complete upstream request
			upstreamTimeSpent := int32(time.Since(attemptStart).Milliseconds())
			h.updateRequestOnComplete(bgCtx, db.UpdateRequestOnCompleteParams{
				ID:           upstreamID,
				StatusCode:   pgtype.Int4{Int32: int32(resp.StatusCode), Valid: true},
				ErrorMessage: pgtype.Text{Valid: false},
				TimeSpentMs:  pgtype.Int4{Int32: upstreamTimeSpent, Valid: true},
				Status:       db.RequestStatusCompleted,
			})

			// Complete meta request
			metaTimeSpent := int32(time.Since(gatewayStart).Milliseconds())
			h.updateRequestOnComplete(bgCtx, db.UpdateRequestOnCompleteParams{
				ID:           metaID,
				StatusCode:   pgtype.Int4{Int32: int32(resp.StatusCode), Valid: true},
				ErrorMessage: pgtype.Text{Valid: false},
				TimeSpentMs:  pgtype.Int4{Int32: metaTimeSpent, Valid: true},
				Status:       db.RequestStatusCompleted,
			})
			return
		}

		// Non-200 response: read body, complete upstream, try next provider
		cancel()
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		timeSpent := int32(time.Since(attemptStart).Milliseconds())
		errMsg := string(respBody)
		h.updateRequestOnComplete(bgCtx, db.UpdateRequestOnCompleteParams{
			ID:           upstreamID,
			StatusCode:   pgtype.Int4{Int32: int32(resp.StatusCode), Valid: true},
			ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
			TimeSpentMs:  pgtype.Int4{Int32: timeSpent, Valid: true},
			Status:       db.RequestStatusFailed,
		})
		lastErr = fmt.Errorf("upstream returned %d: %s", resp.StatusCode, errMsg)
	}

	// 9. All providers failed — complete meta request
	errMsg := "all providers failed"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	metaTimeSpent := int32(time.Since(gatewayStart).Milliseconds())
	h.updateRequestOnComplete(bgCtx, db.UpdateRequestOnCompleteParams{
		ID:           metaID,
		StatusCode:   pgtype.Int4{Int32: http.StatusBadGateway, Valid: true},
		ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
		TimeSpentMs:  pgtype.Int4{Int32: metaTimeSpent, Valid: true},
		Status:       db.RequestStatusFailed,
	})
	writeGatewayError(w, http.StatusBadGateway, errMsg, errorx.UpstreamError.Error())
}
