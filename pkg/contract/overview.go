package contract

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type OverviewCostView struct {
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
}

type OverviewWindowView struct {
	Range   string `json:"range"`
	StartAt string `json:"startAt"`
	EndAt   string `json:"endAt"`
	Bucket  string `json:"bucket"`
}

type OverviewSummaryView struct {
	Window          OverviewWindowView `json:"window"`
	TotalTokens     int64              `json:"totalTokens"`
	TotalRequests   int64              `json:"totalRequests"`
	TotalTraceCount int64              `json:"totalTraceCount"`
	Costs           []OverviewCostView `json:"costs"`
}

type OverviewDistributionRowView struct {
	Key          string             `json:"key"`
	Label        string             `json:"label"`
	TotalTokens  int64              `json:"totalTokens"`
	RequestCount int64              `json:"requestCount"`
	TraceCount   int64              `json:"traceCount"`
	Costs        []OverviewCostView `json:"costs"`
}

type OverviewDistributionView struct {
	Window    OverviewWindowView            `json:"window"`
	Dimension string                        `json:"dimension"`
	Rows      []OverviewDistributionRowView `json:"rows"`
}

type OverviewSeriesGroupView struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type OverviewSeriesPointView struct {
	Metric   string  `json:"metric"`
	BucketAt string  `json:"bucketAt"`
	GroupKey string  `json:"groupKey"`
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

type OverviewSeriesView struct {
	Window    OverviewWindowView        `json:"window"`
	Dimension string                    `json:"dimension"`
	Groups    []OverviewSeriesGroupView `json:"groups"`
	Buckets   []string                  `json:"buckets"`
	Points    []OverviewSeriesPointView `json:"points"`
}

type OverviewCommonRequest struct {
	Range         string `query:"range" enum:"1d,7d,1m" required:"true"`
	ApiKeyID      int32  `query:"apiKeyId,omitempty" minimum:"1"`
	Model         string `query:"model,omitempty" minLength:"1"`
	UpstreamModel string `query:"upstreamModel,omitempty" minLength:"1"`
	ProviderID    int32  `query:"providerId,omitempty" minimum:"1"`
}

type GetOverviewSummaryRequest struct {
	OverviewCommonRequest
}

type GetOverviewSummaryResponse struct {
	Body OverviewSummaryView
}

type GetOverviewDistributionRequest struct {
	OverviewCommonRequest
	Dimension string `query:"dimension" enum:"apiKey,model,upstreamModel,provider" required:"true"`
}

type GetOverviewDistributionResponse struct {
	Body OverviewDistributionView
}

type GetOverviewSeriesRequest struct {
	OverviewCommonRequest
	Dimension string `query:"dimension" enum:"none,apiKey,model,upstreamModel,provider" required:"true"`
}

type GetOverviewSeriesResponse struct {
	Body OverviewSeriesView
}

var OperationGetOverviewSummary = huma.Operation{
	OperationID: "getOverviewSummary",
	Method:      http.MethodGet,
	Path:        "/overview/summary",
	Summary:     "Get overview summary totals",
}

var OperationGetOverviewDistribution = huma.Operation{
	OperationID: "getOverviewDistribution",
	Method:      http.MethodGet,
	Path:        "/overview/distribution",
	Summary:     "Get overview distribution for a dimension",
}

var OperationGetOverviewSeries = huma.Operation{
	OperationID: "getOverviewSeries",
	Method:      http.MethodGet,
	Path:        "/overview/series",
	Summary:     "Get hourly overview series for a dimension",
}
