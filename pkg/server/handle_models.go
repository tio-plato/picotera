package server

import (
	"context"
	"picotera/pkg/contract"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Server) handleGetModel(ctx context.Context, input *contract.GetModelRequest) (*contract.GetModelResponse, error) {
	model, err := s.queries.GetModelByName(ctx, input.Name)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get model", err)
	}

	modelView, err := contract.ToModelView(&model)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to convert model to view", err)
	}

	return &contract.GetModelResponse{
		Body: *modelView,
	}, nil
}

func (s *Server) handlePutModel(ctx context.Context, input *contract.PutModelRequest) (*contract.GetModelResponse, error) {
	model, err := s.queries.UpsertModel(ctx, db.UpsertModelParams{
		Name:      input.Body.Name,
		Title:     pgtype.Text{String: input.Body.Title, Valid: true},
		Developer: pgtype.Text{String: input.Body.Developer, Valid: true},
		Series:    pgtype.Text{String: input.Body.Series, Valid: true},
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to upsert model", err)
	}

	modelView, err := contract.ToModelView(&model)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to convert model to view", err)
	}

	return &contract.GetModelResponse{
		Body: *modelView,
	}, nil
}

func (s *Server) handleListModels(ctx context.Context, input *struct{}) (*contract.ListModelsResponse, error) {
	models, err := s.queries.GetModels(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get models", err)
	}

	modelViews := make([]contract.ModelView, len(models))
	for i, model := range models {
		modelView, err := contract.ToModelView(&model)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to convert model to view", err)
		}
		modelViews[i] = *modelView
	}
	return &contract.ListModelsResponse{
		Body: modelViews,
	}, nil
}
