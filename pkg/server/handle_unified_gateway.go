package server

import (
	"context"
	"net/http"
	"time"

	"picotera/pkg/contract"
	"picotera/pkg/db"
	"picotera/pkg/llmbridge"
)

func (s *Server) handleUnifiedGenerate(srcFormat llmbridge.Format) http.HandlerFunc {
	h := &gatewayHandler{s}
	return func(w http.ResponseWriter, r *http.Request) {
		newGatewayFlow(h, w, r, time.Now(), h.newUnifiedGatewayFlowConfig(srcFormat, r)).run()
	}
}

func (h *gatewayHandler) newUnifiedGatewayFlowConfig(srcFormat llmbridge.Format, r *http.Request) gatewayFlowConfig {
	virtualEndpoint := db.Endpoint{
		Name: "(unified)",
		// Record the registered route pattern (e.g. .../{model}:generateContent)
		// rather than r.URL.Path, so the meta row's endpoint_path keeps the
		// {model} placeholder instead of baking in a concrete model name. The
		// concrete model still reaches the upstream URL via PathVars below.
		Path:                unifiedRoutePath(srcFormat),
		ModelPath:           "",
		CredentialsResolver: contract.CredentialsResolver_Unknown,
		EndpointType:        sourceEndpointType(srcFormat),
	}
	return gatewayFlowConfig{
		Kind:         gatewayRouteUnified,
		Endpoint:     virtualEndpoint,
		PathVars:     chiURLParams(r),
		SourceFormat: srcFormat,
		ExtractModel: func(req *http.Request, body []byte, _ map[string]string) (gatewayModelMode, error) {
			model, err := extractUnifiedModel(srcFormat, req, body)
			return gatewayModelMode{OriginalModel: model, HasModel: true}, err
		},
		SetBodyModel: func(body []byte, model string) ([]byte, error) {
			return setUnifiedModel(srcFormat, body, model)
		},
		ResolveCandidates: func(ctx context.Context, mode gatewayModelMode, auth gatewayAuthState) (candidateSet, error) {
			typeSet := candidateEndpointTypes(srcFormat, mode.Streaming)
			providers, err := h.resolveProvidersByTypes(ctx, mode.RoutedModel, typeSet, sourceEndpointType(srcFormat))
			if err != nil {
				return candidateSet{}, err
			}
			return buildUnifiedCandidateSet(providers, auth.UserAnno, auth.APIKeyAnno, nil, virtualEndpoint)
		},
		PrepareAttempt: prepareUnifiedAttempt,
		HandleSuccess: func(input successInput) {
			input.Flow.h.unifiedStreamSuccess(input)
		},
	}
}
