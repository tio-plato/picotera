package server

import (
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
	"picotera/pkg/jsx"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
)

func (s *Server) handleListProviderEndpoints(ctx context.Context, input *contract.ListProviderEndpointsRequest) (*contract.ListProviderEndpointsResponse, error) {
	rows, err := s.queries.ListProviderEndpoints(ctx, input.ProviderID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list provider endpoints", err)
	}

	items := make([]contract.ProviderEndpointView, len(rows))
	for i, row := range rows {
		items[i] = *contract.ToProviderEndpointView(&row)
	}
	return &contract.ListProviderEndpointsResponse{Body: items}, nil
}

func (s *Server) handleUpsertProviderEndpoint(ctx context.Context, input *contract.UpsertProviderEndpointRequest) (*contract.UpsertProviderEndpointResponse, error) {
	params := contract.FromProviderEndpointView(&input.Body)
	pe, err := s.queries.UpsertProviderEndpoint(ctx, *params)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to upsert provider endpoint", err)
	}
	return &contract.UpsertProviderEndpointResponse{Body: *contract.ToProviderEndpointView(&pe)}, nil
}

func (s *Server) handleDeleteProviderEndpoint(ctx context.Context, input *contract.DeleteProviderEndpointRequest) (*struct{}, error) {
	err := s.queries.DeleteProviderEndpoint(ctx, db.DeleteProviderEndpointParams{
		ProviderID:   input.Body.ProviderID,
		EndpointPath: input.Body.EndpointPath,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete provider endpoint", err)
	}
	return &struct{}{}, nil
}

func (s *Server) handleFetchModels(ctx context.Context, input *contract.FetchModelsRequest) (*contract.FetchModelsResponse, error) {
	provider, err := s.queries.GetProviderByID(ctx, input.Body.ProviderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("provider not found")
		}
		return nil, huma.Error500InternalServerError("failed to get provider", err)
	}

	pe, err := s.queries.GetProviderEndpoint(ctx, db.GetProviderEndpointParams{
		ProviderID:   input.Body.ProviderID,
		EndpointPath: input.Body.EndpointPath,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("provider-endpoint binding not found")
		}
		return nil, huma.Error500InternalServerError("failed to get provider endpoint", err)
	}

	endpoint, err := s.queries.GetEndpointByPath(ctx, input.Body.EndpointPath)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("endpoint not found")
		}
		return nil, huma.Error500InternalServerError("failed to get endpoint", err)
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, pe.UpstreamUrl, nil)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create upstream request", err)
	}

	applyCredentials(req, provider.Credentials, endpoint.CredentialsResolver, nil)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, huma.Error502BadGateway("upstream request failed: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, huma.Error502BadGateway(fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, huma.Error502BadGateway("failed to read upstream response: " + err.Error())
	}

	upstreamNames, err := parseModelsResponse(body)
	if err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}

	var upstreamRaw any
	if jerr := json.Unmarshal(body, &upstreamRaw); jerr != nil {
		upstreamRaw = nil
	}

	oldList := make([]contract.ProviderModelEntry, 0)
	if len(provider.ProviderModels) > 0 {
		_ = json.Unmarshal(provider.ProviderModels, &oldList)
	}

	var annotations map[string]string
	if len(provider.Annotations) > 0 {
		if jerr := json.Unmarshal(provider.Annotations, &annotations); jerr != nil {
			annotations = nil
		}
	}

	aggregated, removed := aggregateProviderModels(oldList, upstreamNames)

	sess, serr := s.jsxEngine.NewSession(ctx, fmt.Sprintf("fetch-models:%d:%d", input.Body.ProviderID, time.Now().UnixNano()))
	if serr != nil {
		return nil, huma.Error502BadGateway("failed to load js hooks: " + serr.Error())
	}
	defer sess.Close()

	upstreamRawJSON, _ := json.Marshal(upstreamRaw)

	processed, herr := sess.RunRewriteProviderModelsHook(jsx.RewriteProviderModelsInput{
		Provider: jsx.ProviderSummary{
			ID:             provider.ID,
			Name:           provider.Name,
			Priority:       provider.Priority,
			ProviderModels: contractToJsxEntries(oldList),
			Annotations:    annotations,
			Disabled:       provider.Disabled,
		},
		EndpointPath:     input.Body.EndpointPath,
		UpstreamResponse: upstreamRawJSON,
	}, contractToJsxEntries(aggregated))
	if herr != nil {
		status := http.StatusBadGateway
		if errors.Is(herr, jsx.ErrHookTimeout) {
			status = http.StatusServiceUnavailable
		}
		return nil, huma.NewError(status, herr.Error())
	}

	converted := jsxToContractEntries(processed)
	var final []contract.ProviderModelEntry
	for _, e := range converted {
		e.Model = strings.TrimSpace(e.Model)
		if e.Model == "" {
			continue
		}
		final = append(final, e)
	}
	final = dedupProviderModelsByPair(final)

	out := &contract.FetchModelsResponse{}
	out.Body.ProviderID = input.Body.ProviderID
	out.Body.ProviderModels = final
	out.Body.RemovedModels = removed
	return out, nil
}

func parseModelsResponse(body []byte) ([]string, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	if models := extractFieldFromData(raw, "id"); len(models) > 0 {
		return models, nil
	}
	if models := extractFieldFromData(raw, "name"); len(models) > 0 {
		return models, nil
	}
	if models := extractFieldFromTopLevel(raw, "id"); len(models) > 0 {
		return models, nil
	}
	if models := extractFieldFromTopLevel(raw, "name"); len(models) > 0 {
		return models, nil
	}

	return nil, fmt.Errorf("could not parse models from upstream response")
}

func extractFieldFromData(raw any, field string) []string {
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	data, ok := obj["data"]
	if !ok {
		return nil
	}
	arr, ok := data.([]any)
	if !ok {
		return nil
	}
	return extractStrings(arr, field)
}

func extractFieldFromTopLevel(raw any, field string) []string {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	return extractStrings(arr, field)
}

func extractStrings(arr []any, field string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		val, ok := obj[field].(string)
		if !ok || val == "" {
			continue
		}
		if !seen[val] {
			seen[val] = true
			result = append(result, val)
		}
	}
	sort.Strings(result)
	return result
}

// aggregateProviderModels applies the default merge rule: retain old entries
// whose actualUpstream is still in upstreamNames, append new-only names as
// {model: name}, and dedup. Returns the merged list and the list of removed
// actualUpstream names.
func aggregateProviderModels(
	old []contract.ProviderModelEntry,
	upstreamNames []string,
) ([]contract.ProviderModelEntry, []string) {
	upstreamSet := make(map[string]bool, len(upstreamNames))
	for _, n := range upstreamNames {
		upstreamSet[n] = true
	}

	var result []contract.ProviderModelEntry
	seenActual := make(map[string]bool)
	var removed []string

	for _, e := range old {
		actual := e.UpstreamModelName
		if actual == "" {
			actual = e.Model
		}
		if upstreamSet[actual] {
			result = append(result, e)
			seenActual[actual] = true
		} else {
			removed = append(removed, actual)
		}
	}

	for _, name := range upstreamNames {
		if !seenActual[name] {
			result = append(result, contract.ProviderModelEntry{Model: name})
		}
	}

	return dedupProviderModelsByPair(result), removed
}

// dedupProviderModelsByPair deduplicates by (Model, UpstreamModelName), keeping
// the first occurrence. Other fields are not used as keys.
func dedupProviderModelsByPair(in []contract.ProviderModelEntry) []contract.ProviderModelEntry {
	type key struct{ model, upstream string }
	seen := make(map[key]bool, len(in))
	out := make([]contract.ProviderModelEntry, 0, len(in))
	for _, e := range in {
		k := key{e.Model, e.UpstreamModelName}
		if !seen[k] {
			seen[k] = true
			out = append(out, e)
		}
	}
	return out
}

// contractToJsxEntries converts contract.ProviderModelEntry slice to
// jsx.ProviderModelEntry slice via JSON round-trip (identical JSON shapes).
func contractToJsxEntries(in []contract.ProviderModelEntry) []jsx.ProviderModelEntry {
	if in == nil {
		return nil
	}
	b, _ := json.Marshal(in)
	var out []jsx.ProviderModelEntry
	_ = json.Unmarshal(b, &out)
	return out
}

// jsxToContractEntries converts jsx.ProviderModelEntry slice to
// contract.ProviderModelEntry slice via JSON round-trip (identical JSON shapes).
func jsxToContractEntries(in []jsx.ProviderModelEntry) []contract.ProviderModelEntry {
	if in == nil {
		return nil
	}
	b, _ := json.Marshal(in)
	var out []contract.ProviderModelEntry
	_ = json.Unmarshal(b, &out)
	return out
}
