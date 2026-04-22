package server

import (
	"context"
	"encoding/json"
	"errors"
	"picotera/pkg/contract"
	"picotera/pkg/db"
	"picotera/pkg/errorx"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
)

func (s *Server) handleListProviders(ctx context.Context, input *struct{}) (*contract.ListProvidersResponse, error) {
	providers, err := s.queries.GetProviders(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list providers", err)
	}

	providerViews := make([]contract.ProviderView, len(providers))
	for i, provider := range providers {
		providerView, err := contract.ToProviderView(&provider)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to convert provider to view", err)
		}
		providerViews[i] = *providerView
	}
	return &contract.ListProvidersResponse{
		Body: providerViews,
	}, nil
}

func (s *Server) handleGetProvider(ctx context.Context, input *contract.GetProviderRequest) (*contract.GetProviderResponse, error) {
	provider, err := s.queries.GetProviderByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("provider not found", errorx.ProviderNotFound)
		}
		return nil, huma.Error500InternalServerError("failed to get provider", err)
	}

	providerView, err := contract.ToProviderView(&provider)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to convert provider to view", err)
	}

	return &contract.GetProviderResponse{
		Body: *providerView,
	}, nil
}

func (s *Server) handleCreateProvider(ctx context.Context, input *contract.CreateProviderRequest) (*contract.CreateProviderResponse, error) {

	providerModelsBytes, err := json.Marshal(input.Body.ProviderModels)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to marshal provider models", err)
	}

	annotationsBytes, err := json.Marshal(input.Body.Annotations)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to marshal annotations", err)
	}

	newProvider, err := s.queries.CreateProvider(ctx, db.CreateProviderParams{
		Name:           input.Body.Name,
		Credentials:    input.Body.Credentials,
		Priority:       input.Body.Priority,
		ProviderModels: providerModelsBytes,
		Annotations:    annotationsBytes,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create provider", err)
	}

	providerView, err := contract.ToProviderView(&newProvider)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to convert provider to view", err)
	}

	return &contract.CreateProviderResponse{
		Body: *providerView,
	}, nil
}
