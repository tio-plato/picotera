package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type gatewayHandler struct {
	*Server
}

var _ http.Handler = (*gatewayHandler)(nil)

func (h *gatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Read request body
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		writeGatewayError(w, http.StatusInternalServerError, "failed to read request body", "INTERNAL_ERROR")
		return
	}

	// 2. Match endpoint by path
	endpoint, err := h.resolveEndpoint(r.Context(), r.URL.Path)
	if err != nil {
		gwErr, ok := err.(*gatewayError)
		if ok {
			writeGatewayError(w, gwErr.status, gwErr.message, gwErr.code)
		} else {
			writeGatewayError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		}
		return
	}

	// 3. Check credentials_resolver (only generalApiKey = 1 is supported in v1)
	if endpoint.CredentialsResolver != 1 {
		writeGatewayError(w, http.StatusInternalServerError,
			fmt.Sprintf("unsupported credentials resolver: %d", endpoint.CredentialsResolver),
			"INTERNAL_ERROR")
		return
	}

	// 4. Resolve auth type from client headers
	authTyp, err := resolveAuthType(r)
	if err != nil {
		gwErr, ok := err.(*gatewayError)
		if ok {
			writeGatewayError(w, gwErr.status, gwErr.message, gwErr.code)
		} else {
			writeGatewayError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		}
		return
	}

	// 5. Extract model name from request body
	model, err := extractModel(body, endpoint.ModelPath)
	if err != nil {
		gwErr, ok := err.(*gatewayError)
		if ok {
			writeGatewayError(w, gwErr.status, gwErr.message, gwErr.code)
		} else {
			writeGatewayError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		}
		return
	}

	// 6. Resolve providers
	providers, err := h.resolveProviders(r.Context(), endpoint.Path, model)
	if err != nil {
		gwErr, ok := err.(*gatewayError)
		if ok {
			writeGatewayError(w, gwErr.status, gwErr.message, gwErr.code)
		} else {
			writeGatewayError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		}
		return
	}

	// 7. Try each provider with failover
	var lastErr error
	for _, provider := range providers {
		start := time.Now()
		// Create a per-attempt context so idle timeouts can cancel the upstream request
		ctx, cancel := context.WithCancel(r.Context())

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
			timeSpent := int32(time.Since(start).Milliseconds())
			h.logRequest(provider.ProviderID, endpoint.Path, model, 0, err.Error(), timeSpent)
			lastErr = err
			continue
		}

		// Forward request
		resp, err := h.forwardRequest(req)
		if err != nil {
			cancel()
			timeSpent := int32(time.Since(start).Milliseconds())
			h.logRequest(provider.ProviderID, endpoint.Path, model, 0, err.Error(), timeSpent)
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusOK {
			// Stream response to client
			for key, values := range resp.Header {
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

			timeSpent := int32(time.Since(start).Milliseconds())
			h.logRequest(provider.ProviderID, endpoint.Path, model, int32(resp.StatusCode), "", timeSpent)
			return
		}

		// Non-200 response: read body, log, try next provider
		cancel()
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		timeSpent := int32(time.Since(start).Milliseconds())
		errMsg := string(respBody)
		h.logRequest(provider.ProviderID, endpoint.Path, model, int32(resp.StatusCode), errMsg, timeSpent)
		lastErr = fmt.Errorf("upstream returned %d: %s", resp.StatusCode, errMsg)
	}

	// 8. All providers failed
	errMsg := "all providers failed"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	writeGatewayError(w, http.StatusBadGateway, errMsg, "UPSTREAM_ERROR")
}