package server

import (
	"context"
	"testing"
	"time"

	"picotera/pkg/contract"

	"github.com/stretchr/testify/require"
)

func TestOverviewRangeBounds(t *testing.T) {
	now := time.Date(2026, 5, 9, 7, 12, 34, 0, time.UTC)

	tests := []struct {
		name          string
		overviewRange contract.OverviewRange
		wantStart     time.Time
		wantEnd       time.Time
	}{
		{
			name:          "24h",
			overviewRange: contract.OverviewRange24H,
			wantStart:     time.Date(2026, 5, 8, 8, 0, 0, 0, time.UTC),
			wantEnd:       time.Date(2026, 5, 9, 8, 0, 0, 0, time.UTC),
		},
		{
			name:          "1d",
			overviewRange: contract.OverviewRange1D,
			wantStart:     time.Date(2026, 5, 8, 8, 0, 0, 0, time.UTC),
			wantEnd:       time.Date(2026, 5, 9, 8, 0, 0, 0, time.UTC),
		},
		{
			name:          "7d",
			overviewRange: contract.OverviewRange7D,
			wantStart:     time.Date(2026, 5, 2, 8, 0, 0, 0, time.UTC),
			wantEnd:       time.Date(2026, 5, 9, 8, 0, 0, 0, time.UTC),
		},
		{
			name:          "1m",
			overviewRange: contract.OverviewRange1M,
			wantStart:     time.Date(2026, 4, 9, 8, 0, 0, 0, time.UTC),
			wantEnd:       time.Date(2026, 5, 9, 8, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startAt, endAt := overviewRangeBounds(now, tt.overviewRange)
			require.Equal(t, tt.wantStart, startAt)
			require.Equal(t, tt.wantEnd, endAt)
		})
	}
}

func TestOverviewValidationHelpers(t *testing.T) {
	overviewRange, ok := contract.ValidateOverviewRange("7d")
	require.True(t, ok)
	require.Equal(t, contract.OverviewRange7D, overviewRange)
	_, ok = contract.ValidateOverviewRange("7D")
	require.False(t, ok)

	dimension, ok := contract.ValidateOverviewDimension("upstreamModel")
	require.True(t, ok)
	require.Equal(t, contract.OverviewDimensionUpstreamModel, dimension)
	_, ok = contract.ValidateOverviewDimension("upstream_model")
	require.False(t, ok)

	seriesDimension, ok := contract.ValidateOverviewSeriesDimension("none")
	require.True(t, ok)
	require.Equal(t, contract.OverviewSeriesDimensionNone, seriesDimension)
	_, ok = contract.ValidateOverviewSeriesDimension("")
	require.False(t, ok)
}

func TestHandleGetOverviewRejectsInvalidInputBeforeQueries(t *testing.T) {
	s := &Server{}

	_, err := s.handleGetOverview(context.Background(), &contract.GetOverviewRequest{Range: "bad"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid range")

	_, err = s.handleGetOverview(context.Background(), &contract.GetOverviewRequest{
		Range:                 "24h",
		DistributionDimension: "providerId",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid distributionDimension")

	_, err = s.handleGetOverview(context.Background(), &contract.GetOverviewRequest{
		Range:           "24h",
		SeriesDimension: "api_key",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid seriesDimension")

	_, err = s.handleGetOverview(context.Background(), &contract.GetOverviewRequest{
		Range:        "24h",
		ModelPresent: true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "model must be non-empty")

	_, err = s.handleGetOverview(context.Background(), &contract.GetOverviewRequest{
		Range:                "24h",
		UpstreamModelPresent: true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "upstreamModel must be non-empty")
}
