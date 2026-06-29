package server

import (
	"context"

	"picotera/pkg/contract"
)

func (s *Server) handleGetConfig(ctx context.Context, _ *struct{}) (*contract.GetConfigResponse, error) {
	return &contract.GetConfigResponse{Body: contract.ConfigView{Title: s.config.AppTitle}}, nil
}
