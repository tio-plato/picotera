package contract

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type OverviewRange string

const (
	OverviewRange24H OverviewRange = "24h"
	OverviewRange1D  OverviewRange = "1d"
	OverviewRange7D  OverviewRange = "7d"
	OverviewRange1M  OverviewRange = "1m"
)

type OverviewDimension string

const (
	OverviewDimensionApiKey        OverviewDimension = "apiKey"
	OverviewDimensionModel         OverviewDimension = "model"
	OverviewDimensionUpstreamModel OverviewDimension = "upstreamModel"
	OverviewDimensionProvider      OverviewDimension = "provider"
)

type OverviewSeriesDimension string

const (
	OverviewSeriesDimensionNone          OverviewSeriesDimension = "none"
	OverviewSeriesDimensionApiKey        OverviewSeriesDimension = "apiKey"
	OverviewSeriesDimensionModel         OverviewSeriesDimension = "model"
	OverviewSeriesDimensionUpstreamModel OverviewSeriesDimension = "upstreamModel"
	OverviewSeriesDimensionProvider      OverviewSeriesDimension = "provider"
)

type OverviewCostView struct {
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
}

type OverviewSummaryView struct {
	TotalTokens     int64              `json:"totalTokens"`
	TotalRequests   int64              `json:"totalRequests"`
	TotalTraceCount int64              `json:"totalTraceCount"`
	Costs           []OverviewCostView `json:"costs"`
}

type OverviewDistributionRowView struct {
	Dimension    OverviewDimension  `json:"dimension"`
	Key          string             `json:"key"`
	Label        string             `json:"label"`
	TotalTokens  int64              `json:"totalTokens"`
	RequestCount int64              `json:"requestCount"`
	TraceCount   int64              `json:"traceCount"`
	Costs        []OverviewCostView `json:"costs"`
}

type OverviewSeriesMetric string

const (
	OverviewSeriesMetricTokens   OverviewSeriesMetric = "tokens"
	OverviewSeriesMetricCost     OverviewSeriesMetric = "cost"
	OverviewSeriesMetricRequests OverviewSeriesMetric = "requests"
	OverviewSeriesMetricTraces   OverviewSeriesMetric = "traces"
)

type OverviewSeriesRowView struct {
	Metric     OverviewSeriesMetric `json:"metric"`
	BucketAt   string               `json:"bucketAt"`
	GroupKey   string               `json:"groupKey"`
	GroupLabel string               `json:"groupLabel"`
	Value      float64              `json:"value"`
	Currency   string               `json:"currency"`
}

type GetOverviewRequest struct {
	Range                 string `query:"range" required:"true" enum:"24h,1d,7d,1m"`
	ApiKeyID              int32  `query:"apiKeyId,omitempty" minimum:"1"`
	Model                 string `query:"model,omitempty"`
	ModelPresent          bool   `json:"-"`
	UpstreamModel         string `query:"upstreamModel,omitempty"`
	UpstreamModelPresent  bool   `json:"-"`
	ProviderID            int32  `query:"providerId,omitempty" minimum:"1"`
	DistributionDimension string `query:"distributionDimension,omitempty" enum:"apiKey,model,upstreamModel,provider"`
	SeriesDimension       string `query:"seriesDimension,omitempty" enum:"none,apiKey,model,upstreamModel,provider"`
}

func (r *GetOverviewRequest) Resolve(ctx huma.Context) []error {
	u := ctx.URL()
	query := u.Query()
	_, r.ModelPresent = query["model"]
	_, r.UpstreamModelPresent = query["upstreamModel"]
	return nil
}

type GetOverviewResponse struct {
	Body struct {
		Range      OverviewRange       `json:"range"`
		StartAt    string              `json:"startAt"`
		EndAt      string              `json:"endAt"`
		Bucket     string              `json:"bucket"`
		Summary    OverviewSummaryView `json:"summary"`
		Dimensions struct {
			Distribution OverviewDimension       `json:"distribution"`
			Series       OverviewSeriesDimension `json:"series"`
		} `json:"dimensions"`
		Distributions []OverviewDistributionRowView `json:"distributions"`
		Series        []OverviewSeriesRowView       `json:"series"`
	}
}

var OperationGetOverview = huma.Operation{
	OperationID: "getOverview",
	Method:      http.MethodGet,
	Path:        "/overview",
	Summary:     "Get dashboard overview",
}

func ValidateOverviewRange(value string) (OverviewRange, bool) {
	switch OverviewRange(value) {
	case OverviewRange24H, OverviewRange1D, OverviewRange7D, OverviewRange1M:
		return OverviewRange(value), true
	default:
		return "", false
	}
}

func ValidateOverviewDimension(value string) (OverviewDimension, bool) {
	switch OverviewDimension(value) {
	case OverviewDimensionApiKey, OverviewDimensionModel, OverviewDimensionUpstreamModel, OverviewDimensionProvider:
		return OverviewDimension(value), true
	default:
		return "", false
	}
}

func ValidateOverviewSeriesDimension(value string) (OverviewSeriesDimension, bool) {
	switch OverviewSeriesDimension(value) {
	case OverviewSeriesDimensionNone, OverviewSeriesDimensionApiKey, OverviewSeriesDimensionModel, OverviewSeriesDimensionUpstreamModel, OverviewSeriesDimensionProvider:
		return OverviewSeriesDimension(value), true
	default:
		return "", false
	}
}
