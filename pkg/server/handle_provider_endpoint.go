package server

import (
	"context"
	"picotera/pkg/contract"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
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
		ProviderID: input.Body.ProviderID,
		EndpointID: input.Body.EndpointID,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete provider endpoint", err)
	}
	return &struct{}{}, nil
}
