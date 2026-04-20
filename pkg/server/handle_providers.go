package server

import (
	"context"
	"errors"
	"picotera/pkg/contract"
	"picotera/pkg/errorx"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
)

func (s *Server) handleGetProvider(ctx context.Context, input *contract.GetProviderRequest) (*contract.GetProviderResponse, error) {
	provider, err := s.queries.GetProviderByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("provider not found", errorx.ProviderNotFound)
		}
		return nil, huma.Error500InternalServerError("failed to get provider", err)
	}

	providerView, err := contract.ToProviderView(provider)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to convert provider to view", err)
	}

	return &contract.GetProviderResponse{
		Body: *providerView,
	}, nil
}
