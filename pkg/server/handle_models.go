package server

import (
	"context"
	"picotera/pkg/contract"

	"github.com/danielgtaylor/huma/v2"
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
	if err := input.Body.Pricing.Validate(); err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	params, err := contract.FromModelView(&input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to encode model", err)
	}

	model, err := s.queries.UpsertModel(ctx, *params)
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

func (s *Server) handleDeleteModel(ctx context.Context, input *contract.DeleteModelRequest) (*struct{}, error) {
	if err := s.queries.DeleteModel(ctx, input.Body.Name); err != nil {
		return nil, huma.Error500InternalServerError("failed to delete model", err)
	}
	return &struct{}{}, nil
}
