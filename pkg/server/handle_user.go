package server

import (
	"context"

	"picotera/pkg/auth"
	"picotera/pkg/contract"

	"github.com/danielgtaylor/huma/v2"
)

func (s *Server) handleGetMe(ctx context.Context, _ *struct{}) (*contract.GetMeResponse, error) {
	u := auth.UserFromContext(ctx)
	if u == nil {
		// The auth middleware guards /api/picotera, so a missing user here
		// would be a wiring bug rather than an unauthenticated request.
		return nil, huma.Error500InternalServerError("no authenticated user")
	}
	return &contract.GetMeResponse{Body: contract.ToMeView(u)}, nil
}
