package server

import (
	"fmt"

	"picotera/pkg/annotations"
	"picotera/pkg/contract"
	"picotera/pkg/db"
	"picotera/pkg/jsx"
	"picotera/pkg/llmbridge"
)

type gatewayCandidate struct {
	Candidate jsx.Candidate
	Sidecar   gatewayCandidateSidecar
}

type gatewayCandidateSidecar struct {
	Key                     string
	ProviderID              int32
	UpstreamURL             string
	Credentials             string
	SendResolver            int32
	ProxyURL                string
	EndpointPath            string
	EndpointType            int32
	UpstreamFormat          llmbridge.Format
	Annotations             map[string]string
	SupportsNativeWebSearch bool
}

type candidateSet struct {
	Items     []gatewayCandidate
	ModelAnno map[string]string
}

func buildPathCandidateSet(providers []providerCandidateRow, apiKeyAnno map[string]string, modelAnno map[string]string, endpoint db.Endpoint) (candidateSet, error) {
	if len(providers) > 0 {
		if m, err := annotations.Decode(providers[0].ModelAnnotations); err == nil {
			modelAnno = m
		}
	}
	annoBuilder, err := newCandidateAnnotationsBuilder(nil, apiKeyAnno)
	if err != nil {
		return candidateSet{}, err
	}
	annoBuilder.modelAnno = modelAnno
	out := candidateSet{Items: make([]gatewayCandidate, 0, len(providers)), ModelAnno: modelAnno}
	for _, row := range providers {
		entryAnno, _ := annotations.Decode(row.EntryAnnotations)
		merged, providerAnno := annoBuilder.merge(row.ProviderAnnotations, entryAnno)
		proxyURL := ""
		if row.ProxyURL.Valid {
			proxyURL = row.ProxyURL.String
		}
		cand := jsx.Candidate{
			Provider:    buildJSProviderSummary(row.ProviderID, row.ProviderName, row.ProviderPriority, providerAnno),
			MPE:         buildJSMPE(row.ModelName, row.ProviderID, row.EndpointPath, row.UpstreamModelName, row.EntryPriority, entryAnno),
			Annotations: merged,
		}
		key := fmt.Sprintf("%d", row.ProviderID)
		out.Items = append(out.Items, gatewayCandidate{
			Candidate: cand,
			Sidecar: gatewayCandidateSidecar{
				Key:          key,
				ProviderID:   row.ProviderID,
				UpstreamURL:  row.UpstreamURL,
				Credentials:  row.ProviderCredentials,
				SendResolver: effectiveSendResolver(endpoint.CredentialsResolver, row.SendCredentialsResolver),
				ProxyURL:     proxyURL,
				EndpointPath: endpoint.Path,
				EndpointType: endpoint.EndpointType,
				Annotations:  merged,
			},
		})
	}
	return out, nil
}

func buildUnifiedCandidateSet(providers []db.GetProvidersByEndpointTypesAndModelRow, apiKeyAnno map[string]string, modelAnno map[string]string, virtualEndpoint db.Endpoint) (candidateSet, error) {
	if len(providers) > 0 {
		if m, err := annotations.Decode(providers[0].ModelAnnotations); err == nil {
			modelAnno = m
		}
	}
	annoBuilder, err := newCandidateAnnotationsBuilder(nil, apiKeyAnno)
	if err != nil {
		return candidateSet{}, err
	}
	annoBuilder.modelAnno = modelAnno
	out := candidateSet{Items: make([]gatewayCandidate, 0, len(providers)), ModelAnno: modelAnno}
	for _, row := range providers {
		entryAnno, _ := annotations.Decode(row.Annotations)
		merged, providerAnno := annoBuilder.merge(row.ProviderAnnotations, entryAnno)
		proxyURL := ""
		if row.ProxyUrl.Valid {
			proxyURL = row.ProxyUrl.String
		}
		cand := jsx.Candidate{
			Provider:    buildJSProviderSummary(row.ProviderID, row.ProviderName, row.ProviderPriority, providerAnno),
			MPE:         buildJSMPE(row.ModelName, row.ProviderID, row.EndpointPath, row.UpstreamModelName, row.Priority, entryAnno),
			Annotations: merged,
		}
		key := fmt.Sprintf("%d|%s", row.ProviderID, row.EndpointPath)
		out.Items = append(out.Items, gatewayCandidate{
			Candidate: cand,
			Sidecar: gatewayCandidateSidecar{
				Key:                     key,
				ProviderID:              row.ProviderID,
				UpstreamURL:             row.UpstreamUrl,
				Credentials:             row.ProviderCredentials,
				SendResolver:            effectiveSendResolver(virtualEndpoint.CredentialsResolver, row.SendCredentialsResolver),
				ProxyURL:                proxyURL,
				EndpointPath:            row.EndpointPath,
				EndpointType:            row.EndpointType,
				UpstreamFormat:          upstreamFormatFor(row.EndpointType),
				Annotations:             merged,
				SupportsNativeWebSearch: row.SupportsNativeWebSearch,
			},
		})
	}
	return out, nil
}

func buildJSProviderSummary(id int32, name string, priority int32, anno map[string]string) jsx.ProviderSummary {
	return jsx.ProviderSummary{ID: id, Name: name, Priority: priority, Annotations: anno}
}

func buildJSMPE(modelName string, providerID int32, endpointPath string, upstreamModelName string, priority int32, anno map[string]string) jsx.CandidateMPE {
	return jsx.CandidateMPE{
		ModelName:         modelName,
		ProviderID:        providerID,
		EndpointPath:      endpointPath,
		UpstreamModelName: upstreamModelName,
		Priority:          priority,
		Annotations:       anno,
	}
}

func candidateSidecarMap(set candidateSet) map[string]gatewayCandidateSidecar {
	out := make(map[string]gatewayCandidateSidecar, len(set.Items))
	for _, item := range set.Items {
		out[item.Sidecar.Key] = item.Sidecar
	}
	return out
}

func candidateKey(kind gatewayRouteKind, cand jsx.Candidate) string {
	if kind == gatewayRouteUnified {
		return fmt.Sprintf("%d|%s", candidateProviderID(cand), candidateEndpointPath(cand))
	}
	return fmt.Sprintf("%d", candidateProviderID(cand))
}

func candidateEndpointPath(c jsx.Candidate) string {
	return c.MPE.EndpointPath
}

func lookupCandidateSidecar(kind gatewayRouteKind, sidecars map[string]gatewayCandidateSidecar, cand jsx.Candidate) (gatewayCandidateSidecar, bool) {
	side, ok := sidecars[candidateKey(kind, cand)]
	return side, ok
}

func sourceEndpointTypeForPath(endpointType int32) llmbridge.Format {
	switch endpointType {
	case contract.EndpointType_AnthropicMessages:
		return llmbridge.FormatAnthropicMessages
	case contract.EndpointType_OpenAIChatCompletions:
		return llmbridge.FormatOpenAIChatCompletions
	case contract.EndpointType_OpenAIResponses:
		return llmbridge.FormatOpenAIResponses
	case contract.EndpointType_GeminiGenerateContent:
		return llmbridge.FormatGeminiGenerateContent
	case contract.EndpointType_GeminiStreamGenerateContent:
		return llmbridge.FormatGeminiStreamGenerateContent
	default:
		return llmbridge.FormatUnknown
	}
}
