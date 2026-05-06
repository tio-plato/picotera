package server

import (
	"context"

	"picotera/pkg/contract"
	"picotera/pkg/pricing"

	"github.com/danielgtaylor/huma/v2"
)

func (s *Server) handleMatchPricing(ctx context.Context, input *contract.MatchPricingRequest) (*contract.MatchPricingResponse, error) {
	if input.Body.TargetModel == "" {
		return nil, huma.Error400BadRequest("targetModel is required")
	}

	candidates, err := pricing.Match(input.Body.TargetModel, 8)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to match pricing", err)
	}

	resp := &contract.MatchPricingResponse{}
	resp.Body.Candidates = candidates
	return resp, nil
}
