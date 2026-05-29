package server

import (
	"context"
	"errors"

	"picotera/pkg/contract"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
)

func (s *Server) handleListGlobalSettings(ctx context.Context, _ *struct{}) (*contract.ListGlobalSettingsResponse, error) {
	rows, err := s.queries.ListGlobalSettings(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list settings", err)
	}
	out := make([]contract.GlobalSettingView, len(rows))
	for i := range rows {
		out[i] = contract.ToGlobalSettingView(&rows[i])
	}
	return &contract.ListGlobalSettingsResponse{Body: out}, nil
}

func (s *Server) handleGetGlobalSetting(ctx context.Context, in *contract.GetGlobalSettingRequest) (*contract.GetGlobalSettingResponse, error) {
	row, err := s.queries.GetGlobalSetting(ctx, in.Key)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("setting not found")
		}
		return nil, huma.Error500InternalServerError("failed to get setting", err)
	}
	v := contract.ToGlobalSettingView(&row)
	return &contract.GetGlobalSettingResponse{Body: v}, nil
}

func (s *Server) handleUpsertGlobalSetting(ctx context.Context, in *contract.UpsertGlobalSettingRequest) (*contract.UpsertGlobalSettingResponse, error) {
	if in.Body.Key == "" {
		return nil, huma.Error400BadRequest("key is required")
	}
	if len(in.Body.Value) == 0 {
		return nil, huma.Error400BadRequest("value is required")
	}
	row, err := s.queries.UpsertGlobalSetting(ctx, db.UpsertGlobalSettingParams{
		Key:   in.Body.Key,
		Value: in.Body.Value,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to upsert setting", err)
	}
	v := contract.ToGlobalSettingView(&row)
	return &contract.UpsertGlobalSettingResponse{Body: v}, nil
}

func (s *Server) handleDeleteGlobalSetting(ctx context.Context, in *contract.DeleteGlobalSettingRequest) (*struct{}, error) {
	affected, err := s.queries.DeleteGlobalSetting(ctx, in.Key)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete setting", err)
	}
	if affected == 0 {
		return nil, huma.Error404NotFound("setting not found")
	}
	return &struct{}{}, nil
}
