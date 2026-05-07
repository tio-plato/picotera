package server

import (
	"testing"

	"picotera/pkg/contract"
)

func TestComputeCostIncludesCacheWrite1H(t *testing.T) {
	inputTokens := int32(100)
	outputTokens := int32(200)
	cacheReadTokens := int32(300)
	cacheWriteTokens := int32(400)
	cacheWrite1hTokens := int32(500)
	pricing := &contract.Pricing{
		Currency: "USD",
		Tiers: []contract.PricingTier{
			{
				Input:        1,
				Output:       2,
				CacheRead:    3,
				CacheWrite:   4,
				CacheWrite1H: 5,
			},
		},
	}

	got, currency, ok := computeCost(pricing, &inputTokens, &outputTokens, &cacheReadTokens, &cacheWriteTokens, &cacheWrite1hTokens)
	if !ok {
		t.Fatal("computeCost ok=false")
	}
	if !currency.Valid || currency.String != "USD" {
		t.Fatalf("currency = %#v, want USD", currency)
	}
	if got.Int == nil || got.Int.String() != "5500" || got.Exp != -6 || !got.Valid {
		t.Fatalf("cost = %#v, want 0.005500", got)
	}
}
