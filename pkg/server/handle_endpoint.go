package server

import (
	"context"
	"picotera/pkg/contract"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
)

func (s *Server) handleListEndpoints(ctx context.Context, input *struct{}) (*contract.ListEndpointsResponse, error) {
	endpoints, err := s.queries.GetEndpoints(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get endpoints", err)
	}

	endpointViews := make([]contract.EndpointView, len(endpoints))
	for i, endpoint := range endpoints {
		endpointView, err := contract.ToEndpointView(&endpoint)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to convert endpoint to view", err)
		}
		endpointViews[i] = *endpointView
	}
	return &contract.ListEndpointsResponse{
		Body: endpointViews,
	}, nil
}

func (s *Server) handleUpsertEndpoint(ctx context.Context, input *contract.UpsertEndpointRequest) (*contract.UpsertEndpointResponse, error) {
	endpoint, err := s.queries.UpsertEndpoint(ctx, db.UpsertEndpointParams{
		Name:                input.Body.Name,
		Path:                input.Body.Path,
		ModelPath:           input.Body.ModelPath,
		CredentialsResolver: contract.ToCredentialsResolver(input.Body.CredentialsResolver),
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to upsert endpoint", err)
	}

	endpointView, err := contract.ToEndpointView(&endpoint)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to convert endpoint to view", err)
	}
	return &contract.UpsertEndpointResponse{
		Body: *endpointView,
	}, nil
}

func (s *Server) handleDeleteEndpoint(ctx context.Context, input *contract.DeleteEndpointRequest) (*struct{}, error) {
	err := s.queries.DeleteEndpoint(ctx, input.Body.Path)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete endpoint", err)
	}
	return &struct{}{}, nil
}
