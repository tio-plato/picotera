package contract

import (
	"encoding/json"
	"errors"
	"fmt"
)

// PricingTier is a single tier of pricing. All numeric fields are per 1M tokens
// in the parent Pricing.Currency. Values must be >= 0.
type PricingTier struct {
	MinInputTokens    int64   `json:"minInputTokens"`
	Input             float64 `json:"input"`
	Output            float64 `json:"output"`
	CacheRead         float64 `json:"cacheRead"`
	CacheWrite        float64 `json:"cacheWrite"`
	CacheWrite1H      float64 `json:"cacheWrite1h"`
	ImplicitCacheRead float64 `json:"implicitCacheRead"`
}

// Pricing is the JSON shape stored in model.pricing and ProviderModelEntry.pricing.
type Pricing struct {
	Currency string        `json:"currency"`
	Tiers    []PricingTier `json:"tiers"`
}

// Validate enforces the structural invariants documented in design.md:
//   - Currency must be non-empty.
//   - Tiers must contain at least one entry.
//   - First tier's MinInputTokens must be 0.
//   - Tiers must be sorted strictly ascending by MinInputTokens.
//   - All numeric fields must be >= 0.
func (p *Pricing) Validate() error {
	if p == nil {
		return nil
	}
	if p.Currency == "" {
		return errors.New("pricing.currency is required")
	}
	if len(p.Tiers) == 0 {
		return errors.New("pricing.tiers must contain at least one tier")
	}
	if p.Tiers[0].MinInputTokens != 0 {
		return errors.New("pricing.tiers[0].minInputTokens must be 0")
	}
	for i, t := range p.Tiers {
		if t.MinInputTokens < 0 {
			return fmt.Errorf("pricing.tiers[%d].minInputTokens must be >= 0", i)
		}
		if i > 0 && t.MinInputTokens <= p.Tiers[i-1].MinInputTokens {
			return fmt.Errorf("pricing.tiers must be strictly ascending by minInputTokens (tier %d)", i)
		}
		if t.Input < 0 || t.Output < 0 || t.CacheRead < 0 || t.CacheWrite < 0 || t.CacheWrite1H < 0 || t.ImplicitCacheRead < 0 {
			return fmt.Errorf("pricing.tiers[%d] has a negative price", i)
		}
	}
	return nil
}

// PricingFromJSONB converts a JSONB blob from the DB into a *Pricing. Empty
// objects, empty payloads, or pricing with no tiers are interpreted as "unset"
// and return nil.
func PricingFromJSONB(raw []byte) (*Pricing, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	// Detect "{}" without unmarshaling first.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err == nil && len(probe) == 0 {
		return nil, nil
	}
	var p Pricing
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	if p.Currency == "" && len(p.Tiers) == 0 {
		return nil, nil
	}
	if len(p.Tiers) == 0 {
		return nil, nil
	}
	return &p, nil
}

// PricingToJSONB renders a *Pricing into a JSONB blob suitable for sqlc params.
// nil or empty-tier pricing serializes to "{}" so the DB column reflects "unset".
func PricingToJSONB(p *Pricing) ([]byte, error) {
	if p == nil || len(p.Tiers) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(p)
}
