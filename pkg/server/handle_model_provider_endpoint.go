package server

import (
	"context"
	"picotera/pkg/contract"
	"picotera/pkg/db"
	"picotera/pkg/errorx"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Server) handleListModelProviderEndpoints(ctx context.Context, input *contract.ListModelProviderEndpointsRequest) (*contract.ListModelProviderEndpointsResponse, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	fetchLimit := limit + 1

	var cursorModelName pgtype.Text
	var cursorProviderID pgtype.Int4
	var cursorEndpointPath pgtype.Text

	if input.Cursor != "" {
		var modelName, endpointPath string
		var providerID int32
		if err := contract.DecodeCursor(input.Cursor, "modelName", &modelName, "providerId", &providerID, "endpointPath", &endpointPath); err != nil {
			return nil, huma.Error400BadRequest("invalid cursor", err)
		}
		cursorModelName = pgtype.Text{String: modelName, Valid: true}
		cursorProviderID = pgtype.Int4{Int32: providerID, Valid: true}
		cursorEndpointPath = pgtype.Text{String: endpointPath, Valid: true}
	}

	var filterModelName pgtype.Text
	if input.ModelName != "" {
		filterModelName = pgtype.Text{String: input.ModelName, Valid: true}
	}

	var filterProviderID pgtype.Int4
	if input.ProviderID != 0 {
		filterProviderID = pgtype.Int4{Int32: input.ProviderID, Valid: true}
	}

	var filterEndpointPath pgtype.Text
	if input.EndpointPath != "" {
		filterEndpointPath = pgtype.Text{String: input.EndpointPath, Valid: true}
	}

	rows, err := s.queries.ListModelProviderEndpoints(ctx, db.ListModelProviderEndpointsParams{
		ModelName:          filterModelName,
		ProviderID:         filterProviderID,
		EndpointPath:       filterEndpointPath,
		CursorModelName:    cursorModelName,
		CursorProviderID:   cursorProviderID,
		CursorEndpointPath: cursorEndpointPath,
		Limit:              pgtype.Int4{Int32: fetchLimit, Valid: true},
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list model provider endpoints", err)
	}

	hasMore := int32(len(rows)) > limit
	if hasMore {
		rows = rows[:limit]
	}

	items := make([]contract.ModelProviderEndpointView, len(rows))
	for i, row := range rows {
		view, err := contract.ToModelProviderEndpointView(&row)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to convert model provider endpoint", err)
		}
		items[i] = *view
	}

	pagination := contract.PaginationInfo{HasMore: hasMore}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		cursor, err := contract.EncodeCursor("modelName", last.ModelName, "providerId", last.ProviderID, "endpointPath", last.EndpointPath)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to encode cursor", err)
		}
		pagination.NextCursor = cursor
	}

	return &contract.ListModelProviderEndpointsResponse{
		Body: contract.PaginatedBody[contract.ModelProviderEndpointView]{
			Items:      items,
			Pagination: pagination,
		},
	}, nil
}

func (s *Server) handleGetModelProviderEndpoint(ctx context.Context, input *contract.GetModelProviderEndpointRequest) (*contract.GetModelProviderEndpointResponse, error) {
	mpe, err := s.queries.GetModelProviderEndpoint(ctx, db.GetModelProviderEndpointParams{
		ModelName:    input.ModelName,
		ProviderID:   input.ProviderID,
		EndpointPath: input.EndpointPath,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, huma.Error404NotFound("model provider endpoint not found", errorx.ModelProviderEndpointNotFound)
		}
		return nil, huma.Error500InternalServerError("failed to get model provider endpoint", err)
	}

	view, err := contract.ToModelProviderEndpointView(&mpe)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to convert model provider endpoint", err)
	}

	return &contract.GetModelProviderEndpointResponse{Body: *view}, nil
}

func (s *Server) handleUpsertModelProviderEndpoint(ctx context.Context, input *contract.UpsertModelProviderEndpointRequest) (*contract.UpsertModelProviderEndpointResponse, error) {
	params, err := contract.FromModelProviderEndpointView(&input.Body)
	if err != nil {
		return nil, huma.Error400BadRequest("invalid request body", err)
	}

	mpe, err := s.queries.UpsertModelProviderEndpoint(ctx, *params)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to upsert model provider endpoint", err)
	}

	view, err := contract.ToModelProviderEndpointView(&mpe)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to convert model provider endpoint", err)
	}

	return &contract.UpsertModelProviderEndpointResponse{Body: *view}, nil
}

func (s *Server) handleDeleteModelProviderEndpoint(ctx context.Context, input *contract.DeleteModelProviderEndpointRequest) (*struct{}, error) {
	err := s.queries.DeleteModelProviderEndpoint(ctx, db.DeleteModelProviderEndpointParams{
		ModelName:    input.Body.ModelName,
		ProviderID:   input.Body.ProviderID,
		EndpointPath: input.Body.EndpointPath,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete model provider endpoint", err)
	}
	return &struct{}{}, nil
}
