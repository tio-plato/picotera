package contract

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type MatchPricingRequest struct {
	Body struct {
		TargetModel string `json:"targetModel" example:"claude-sonnet-4-6"`
	}
}

type PricingMatchCandidate struct {
	ProviderID   string  `json:"providerId" example:"anthropic"`
	ProviderName string  `json:"providerName" example:"Anthropic"`
	ModelID      string  `json:"modelId" example:"claude-sonnet-4-6"`
	ModelName    string  `json:"modelName" example:"Claude Sonnet 4.6"`
	Score        int     `json:"score" example:"0"`
	Pricing      Pricing `json:"pricing"`
}

type MatchPricingResponse struct {
	Body struct {
		Candidates []PricingMatchCandidate `json:"candidates"`
	}
}

var OperationMatchPricing = huma.Operation{
	OperationID: "matchPricing",
	Method:      http.MethodPost,
	Path:        "/pricing/matches",
	Summary:     "Match built-in pricing candidates for a model",
}
