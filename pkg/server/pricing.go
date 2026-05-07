package server

import (
	"encoding/json"
	"math/big"

	"picotera/pkg/contract"

	"github.com/jackc/pgx/v5/pgtype"
)

// pickTier returns the highest-MinInputTokens tier whose threshold is
// <= inputTokens. Returns nil if pricing is missing or no tier qualifies.
func pickTier(p *contract.Pricing, inputTokens int64) *contract.PricingTier {
	if p == nil || len(p.Tiers) == 0 {
		return nil
	}
	var picked *contract.PricingTier
	for i := range p.Tiers {
		t := &p.Tiers[i]
		if t.MinInputTokens > inputTokens {
			break
		}
		picked = t
	}
	return picked
}

// computeCost calculates the per-request cost in p.Currency given token counts.
// inputTokens must be non-nil to drive tier selection. implicitCacheRead is
// not yet tracked in the request table, so it contributes 0. ok=false signals
// "not enough information to bill".
//
// The returned Numeric carries the amount with 6-decimal precision. The Text
// is the currency code. Both are zero-valued when ok is false.
func computeCost(p *contract.Pricing, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens, cacheWrite1hTokens *int32) (pgtype.Numeric, pgtype.Text, bool) {
	if p == nil || len(p.Tiers) == 0 || inputTokens == nil {
		return pgtype.Numeric{}, pgtype.Text{}, false
	}
	tier := pickTier(p, int64(*inputTokens))
	if tier == nil {
		return pgtype.Numeric{}, pgtype.Text{}, false
	}

	million := big.NewRat(1_000_000, 1)
	total := new(big.Rat)
	add := func(perMillion float64, tokens int32) {
		if tokens == 0 || perMillion == 0 {
			return
		}
		price := new(big.Rat).SetFloat64(perMillion)
		if price == nil {
			return
		}
		t := new(big.Rat).SetInt64(int64(tokens))
		term := new(big.Rat).Mul(price, t)
		term.Quo(term, million)
		total.Add(total, term)
	}

	add(tier.Input, *inputTokens)
	if outputTokens != nil {
		add(tier.Output, *outputTokens)
	}
	if cacheReadTokens != nil {
		add(tier.CacheRead, *cacheReadTokens)
	}
	if cacheWriteTokens != nil {
		add(tier.CacheWrite, *cacheWriteTokens)
	}
	if cacheWrite1hTokens != nil {
		add(tier.CacheWrite1H, *cacheWrite1hTokens)
	}
	// implicitCacheRead — no token column yet; reserved.

	num, err := ratToNumeric6(total)
	if err != nil {
		return pgtype.Numeric{}, pgtype.Text{}, false
	}
	return num, pgtype.Text{String: p.Currency, Valid: true}, true
}

// ratToNumeric6 converts a big.Rat to a pgtype.Numeric with exactly 6 decimal
// places (matching the column definition NUMERIC(20, 6)).
func ratToNumeric6(r *big.Rat) (pgtype.Numeric, error) {
	scale := big.NewInt(1_000_000)
	scaled := new(big.Rat).Mul(r, new(big.Rat).SetInt(scale))
	num := scaled.Num()
	den := scaled.Denom()
	q, m := new(big.Int).QuoRem(num, den, new(big.Int))
	// Round half-up.
	if m.Sign() != 0 {
		twice := new(big.Int).Mul(m, big.NewInt(2))
		twice.Abs(twice)
		absDen := new(big.Int).Abs(den)
		if twice.Cmp(absDen) >= 0 {
			if num.Sign() >= 0 {
				q.Add(q, big.NewInt(1))
			} else {
				q.Sub(q, big.NewInt(1))
			}
		}
	}
	return pgtype.Numeric{Int: q, Exp: -6, Valid: true}, nil
}

// providerEntryPricing returns the Pricing for the given model in provider's
// providerModels[]. Nil if not found / no pricing configured.
func providerEntryPricing(providerModels []byte, model string) (*contract.Pricing, error) {
	if len(providerModels) == 0 {
		return nil, nil
	}
	var entries []contract.ProviderModelEntry
	if err := json.Unmarshal(providerModels, &entries); err != nil {
		return nil, err
	}
	for i := range entries {
		if entries[i].Model == model {
			return entries[i].Pricing, nil
		}
	}
	return nil, nil
}
