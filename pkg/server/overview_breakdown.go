package server

import (
	"sort"

	"picotera/pkg/contract"
	"picotera/pkg/db"
)

type breakdownKey struct {
	APIKeyID      int32
	Model         string
	UpstreamModel string
	ProviderID    int32
	ProjectID     int32
}

func mergeBreakdown(
	tokens []db.ListOverviewBreakdownTokensRow,
	costs []db.ListOverviewBreakdownCostsRow,
) []contract.OverviewBreakdownRowView {
	rows := make(map[breakdownKey]*contract.OverviewBreakdownRowView, len(tokens)+len(costs))

	for _, t := range tokens {
		k := breakdownKey{APIKeyID: t.ApiKeyID, Model: t.Model, UpstreamModel: t.UpstreamModel, ProviderID: t.ProviderID, ProjectID: t.ProjectID}
		rows[k] = &contract.OverviewBreakdownRowView{
			ApiKeyID:      t.ApiKeyID,
			Model:         t.Model,
			UpstreamModel: t.UpstreamModel,
			ProviderID:    t.ProviderID,
			ProjectID:     t.ProjectID,
			TotalTokens:   t.TotalTokens,
			Costs:         []contract.OverviewCostView{},
		}
	}

	for _, c := range costs {
		k := breakdownKey{APIKeyID: c.ApiKeyID, Model: c.Model, UpstreamModel: c.UpstreamModel, ProviderID: c.ProviderID, ProjectID: c.ProjectID}
		row, ok := rows[k]
		if !ok {
			row = &contract.OverviewBreakdownRowView{
				ApiKeyID:      c.ApiKeyID,
				Model:         c.Model,
				UpstreamModel: c.UpstreamModel,
				ProviderID:    c.ProviderID,
				ProjectID:     c.ProjectID,
				TotalTokens:   0,
				Costs:         []contract.OverviewCostView{},
			}
			rows[k] = row
		}
		row.Costs = append(row.Costs, contract.OverviewCostView{Currency: c.Currency, Amount: c.Amount})
	}

	out := make([]contract.OverviewBreakdownRowView, 0, len(rows))
	for _, r := range rows {
		sort.Slice(r.Costs, func(i, j int) bool { return r.Costs[i].Currency < r.Costs[j].Currency })
		out = append(out, *r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TotalTokens != out[j].TotalTokens {
			return out[i].TotalTokens > out[j].TotalTokens
		}
		return out[i].ApiKeyID < out[j].ApiKeyID
	})
	return out
}
