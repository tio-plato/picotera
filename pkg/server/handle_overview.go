package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"picotera/pkg/contract"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

type overviewQueryParams struct {
	startAt       pgtype.Timestamp
	endAt         pgtype.Timestamp
	apiKeyID      pgtype.Int4
	model         pgtype.Text
	upstreamModel pgtype.Text
	providerID    pgtype.Int4
}

func overviewRangeBounds(now time.Time, overviewRange contract.OverviewRange) (time.Time, time.Time) {
	endAt := now.UTC().Truncate(time.Hour).Add(time.Hour)
	duration := 24 * time.Hour
	switch overviewRange {
	case contract.OverviewRange7D:
		duration = 7 * 24 * time.Hour
	case contract.OverviewRange1M:
		duration = 30 * 24 * time.Hour
	}
	return endAt.Add(-duration), endAt
}

func overviewNumericToFloat(n pgtype.Numeric) (float64, error) {
	if !n.Valid {
		return 0, nil
	}
	f, err := n.Float64Value()
	if err != nil {
		return 0, err
	}
	if !f.Valid {
		return 0, nil
	}
	return f.Float64, nil
}

func overviewCostsFromRows(rows []db.GetOverviewCostSummaryRow) ([]contract.OverviewCostView, error) {
	costs := make([]contract.OverviewCostView, 0, len(rows))
	for _, row := range rows {
		amount, err := overviewNumericToFloat(row.Amount)
		if err != nil {
			return nil, err
		}
		costs = append(costs, contract.OverviewCostView{
			Currency: row.Currency.String,
			Amount:   amount,
		})
	}
	return costs, nil
}

func overviewCostsFromJSON(raw []byte) ([]contract.OverviewCostView, error) {
	var costs []contract.OverviewCostView
	if err := json.Unmarshal(raw, &costs); err != nil {
		return nil, err
	}
	if costs == nil {
		costs = []contract.OverviewCostView{}
	}
	return costs, nil
}

func overviewTimestamp(ts pgtype.Timestamp) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.UTC().Format(time.RFC3339Nano)
}

func (p overviewQueryParams) summaryParams() db.GetOverviewSummaryParams {
	return db.GetOverviewSummaryParams{
		StartAt:       p.startAt,
		EndAt:         p.endAt,
		ApiKeyID:      p.apiKeyID,
		Model:         p.model,
		UpstreamModel: p.upstreamModel,
		ProviderID:    p.providerID,
	}
}

func (p overviewQueryParams) costSummaryParams() db.GetOverviewCostSummaryParams {
	return db.GetOverviewCostSummaryParams{
		StartAt:       p.startAt,
		EndAt:         p.endAt,
		ApiKeyID:      p.apiKeyID,
		Model:         p.model,
		UpstreamModel: p.upstreamModel,
		ProviderID:    p.providerID,
	}
}

func (p overviewQueryParams) traceCountParams() db.GetOverviewTraceCountParams {
	return db.GetOverviewTraceCountParams{
		StartAt:       p.startAt,
		EndAt:         p.endAt,
		ApiKeyID:      p.apiKeyID,
		Model:         p.model,
		UpstreamModel: p.upstreamModel,
		ProviderID:    p.providerID,
	}
}

func (p overviewQueryParams) distributionParams(dimension contract.OverviewDimension) db.GetOverviewDistributionParams {
	return db.GetOverviewDistributionParams{
		Dimension:     string(dimension),
		StartAt:       p.startAt,
		EndAt:         p.endAt,
		ApiKeyID:      p.apiKeyID,
		Model:         p.model,
		UpstreamModel: p.upstreamModel,
		ProviderID:    p.providerID,
	}
}

func (p overviewQueryParams) requestSeriesParams(dimension contract.OverviewSeriesDimension) db.GetOverviewHourlyRequestSeriesParams {
	return db.GetOverviewHourlyRequestSeriesParams{
		Dimension:     string(dimension),
		StartAt:       p.startAt,
		EndAt:         p.endAt,
		ApiKeyID:      p.apiKeyID,
		Model:         p.model,
		UpstreamModel: p.upstreamModel,
		ProviderID:    p.providerID,
	}
}

func (p overviewQueryParams) costSeriesParams(dimension contract.OverviewSeriesDimension) db.GetOverviewHourlyCostSeriesParams {
	return db.GetOverviewHourlyCostSeriesParams{
		Dimension:     string(dimension),
		StartAt:       p.startAt,
		EndAt:         p.endAt,
		ApiKeyID:      p.apiKeyID,
		Model:         p.model,
		UpstreamModel: p.upstreamModel,
		ProviderID:    p.providerID,
	}
}

func (p overviewQueryParams) traceSeriesParams(dimension contract.OverviewSeriesDimension) db.GetOverviewHourlyTraceSeriesParams {
	return db.GetOverviewHourlyTraceSeriesParams{
		Dimension:     string(dimension),
		StartAt:       p.startAt,
		EndAt:         p.endAt,
		ApiKeyID:      p.apiKeyID,
		Model:         p.model,
		UpstreamModel: p.upstreamModel,
		ProviderID:    p.providerID,
	}
}

func (s *Server) handleGetOverview(ctx context.Context, input *contract.GetOverviewRequest) (*contract.GetOverviewResponse, error) {
	overviewRange, ok := contract.ValidateOverviewRange(input.Range)
	if !ok {
		return nil, huma.Error400BadRequest("invalid range")
	}

	distributionDimension := contract.OverviewDimensionProvider
	if input.DistributionDimension != "" {
		var ok bool
		distributionDimension, ok = contract.ValidateOverviewDimension(input.DistributionDimension)
		if !ok {
			return nil, huma.Error400BadRequest("invalid distributionDimension")
		}
	}

	seriesDimension := contract.OverviewSeriesDimensionNone
	if input.SeriesDimension != "" {
		var ok bool
		seriesDimension, ok = contract.ValidateOverviewSeriesDimension(input.SeriesDimension)
		if !ok {
			return nil, huma.Error400BadRequest("invalid seriesDimension")
		}
	}

	startAt, endAt := overviewRangeBounds(time.Now(), overviewRange)
	params := overviewQueryParams{
		startAt: pgtype.Timestamp{Time: startAt, Valid: true},
		endAt:   pgtype.Timestamp{Time: endAt, Valid: true},
	}
	if input.ApiKeyID > 0 {
		params.apiKeyID = pgtype.Int4{Int32: input.ApiKeyID, Valid: true}
	}
	if input.ProviderID > 0 {
		params.providerID = pgtype.Int4{Int32: input.ProviderID, Valid: true}
	}
	if input.ModelPresent {
		if input.Model == "" {
			return nil, huma.Error400BadRequest("model must be non-empty")
		}
		params.model = pgtype.Text{String: input.Model, Valid: true}
	}
	if input.UpstreamModelPresent {
		if input.UpstreamModel == "" {
			return nil, huma.Error400BadRequest("upstreamModel must be non-empty")
		}
		params.upstreamModel = pgtype.Text{String: input.UpstreamModel, Valid: true}
	}

	summary, err := s.queries.GetOverviewSummary(ctx, params.summaryParams())
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to load overview summary", err)
	}
	costRows, err := s.queries.GetOverviewCostSummary(ctx, params.costSummaryParams())
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to load overview costs", err)
	}
	costs, err := overviewCostsFromRows(costRows)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to parse overview costs", err)
	}
	traceCount, err := s.queries.GetOverviewTraceCount(ctx, params.traceCountParams())
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to load overview trace count", err)
	}
	distributionRows, err := s.queries.GetOverviewDistribution(ctx, params.distributionParams(distributionDimension))
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to load overview distribution", err)
	}
	requestSeriesRows, err := s.queries.GetOverviewHourlyRequestSeries(ctx, params.requestSeriesParams(seriesDimension))
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to load overview request series", err)
	}
	costSeriesRows, err := s.queries.GetOverviewHourlyCostSeries(ctx, params.costSeriesParams(seriesDimension))
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to load overview cost series", err)
	}
	traceSeriesRows, err := s.queries.GetOverviewHourlyTraceSeries(ctx, params.traceSeriesParams(seriesDimension))
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to load overview trace series", err)
	}

	distributions := make([]contract.OverviewDistributionRowView, 0, len(distributionRows))
	for _, row := range distributionRows {
		rowCosts, err := overviewCostsFromJSON(row.Costs)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to parse overview distribution costs", err)
		}
		distributions = append(distributions, contract.OverviewDistributionRowView{
			Dimension:    contract.OverviewDimension(row.Dimension),
			Key:          row.GroupKey,
			Label:        row.GroupLabel,
			TotalTokens:  row.TotalTokens,
			RequestCount: row.RequestCount,
			TraceCount:   row.TraceCount,
			Costs:        rowCosts,
		})
	}

	series := make([]contract.OverviewSeriesRowView, 0, len(requestSeriesRows)*2+len(costSeriesRows)+len(traceSeriesRows))
	for _, row := range requestSeriesRows {
		bucketAt := overviewTimestamp(row.BucketAt)
		series = append(series,
			contract.OverviewSeriesRowView{
				Metric:     contract.OverviewSeriesMetricTokens,
				BucketAt:   bucketAt,
				GroupKey:   row.GroupKey,
				GroupLabel: row.GroupLabel,
				Value:      row.TokensValue,
				Currency:   "",
			},
			contract.OverviewSeriesRowView{
				Metric:     contract.OverviewSeriesMetricRequests,
				BucketAt:   bucketAt,
				GroupKey:   row.GroupKey,
				GroupLabel: row.GroupLabel,
				Value:      row.RequestsValue,
				Currency:   "",
			},
		)
	}
	for _, row := range costSeriesRows {
		series = append(series, contract.OverviewSeriesRowView{
			Metric:     contract.OverviewSeriesMetricCost,
			BucketAt:   overviewTimestamp(row.BucketAt),
			GroupKey:   row.GroupKey,
			GroupLabel: fmt.Sprintf("%s %s", row.GroupLabel, row.Currency),
			Value:      row.CostValue,
			Currency:   row.Currency,
		})
	}
	for _, row := range traceSeriesRows {
		series = append(series, contract.OverviewSeriesRowView{
			Metric:     contract.OverviewSeriesMetricTraces,
			BucketAt:   overviewTimestamp(row.BucketAt),
			GroupKey:   row.GroupKey,
			GroupLabel: row.GroupLabel,
			Value:      row.TraceValue,
			Currency:   "",
		})
	}

	response := &contract.GetOverviewResponse{}
	response.Body.Range = overviewRange
	response.Body.StartAt = startAt.Format(time.RFC3339Nano)
	response.Body.EndAt = endAt.Format(time.RFC3339Nano)
	response.Body.Bucket = "hour"
	response.Body.Summary = contract.OverviewSummaryView{
		TotalTokens:     summary.TotalTokens,
		TotalRequests:   summary.TotalRequests,
		TotalTraceCount: traceCount,
		Costs:           costs,
	}
	response.Body.Dimensions.Distribution = distributionDimension
	response.Body.Dimensions.Series = seriesDimension
	response.Body.Distributions = distributions
	response.Body.Series = series
	return response, nil
}
