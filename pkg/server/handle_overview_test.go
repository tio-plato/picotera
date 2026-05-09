package server

import (
	"testing"
	"time"

	"picotera/pkg/db"
)

func TestOverviewWindow(t *testing.T) {
	now := time.Date(2026, 5, 9, 8, 17, 32, 0, time.UTC)
	cases := []struct {
		rangeKey string
		wantStr  string
		wantHrs  int
	}{
		{"1d", "2026-05-08T09:00:00Z", 24},
		{"7d", "2026-05-02T09:00:00Z", 24 * 7},
		{"1m", "2026-04-09T09:00:00Z", 24 * 30},
	}
	for _, tc := range cases {
		t.Run(tc.rangeKey, func(t *testing.T) {
			start, end, err := overviewWindow(tc.rangeKey, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := end.UTC().Format(time.RFC3339); got != "2026-05-09T09:00:00Z" {
				t.Errorf("end = %s, want 2026-05-09T09:00:00Z", got)
			}
			if got := start.UTC().Format(time.RFC3339); got != tc.wantStr {
				t.Errorf("start = %s, want %s", got, tc.wantStr)
			}
			if got := int(end.Sub(start) / time.Hour); got != tc.wantHrs {
				t.Errorf("hours = %d, want %d", got, tc.wantHrs)
			}
		})
	}
}

func TestOverviewWindowInvalid(t *testing.T) {
	_, _, err := overviewWindow("bogus", time.Now())
	if err == nil {
		t.Fatal("expected error for invalid range")
	}
}

func TestOverviewBuckets(t *testing.T) {
	start := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	end := start.Add(12 * time.Hour)
	got := overviewBuckets(start, end, 4*time.Hour)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if !got[0].Equal(start) {
		t.Errorf("first = %v, want %v", got[0], start)
	}
	if !got[2].Equal(start.Add(8 * time.Hour)) {
		t.Errorf("last = %v, want %v", got[2], start.Add(8*time.Hour))
	}
}

func TestOverviewSeriesBucketInterval(t *testing.T) {
	cases := []struct {
		rangeKey string
		want     time.Duration
	}{
		{"1d", time.Hour},
		{"7d", 4 * time.Hour},
		{"1m", 8 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.rangeKey, func(t *testing.T) {
			got, err := overviewSeriesBucketInterval(tc.rangeKey)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("interval = %s, want %s", got, tc.want)
			}
		})
	}
	if _, err := overviewSeriesBucketInterval("bogus"); err == nil {
		t.Fatal("expected error for invalid range")
	}
}

func TestOverviewSeriesBucketCounts(t *testing.T) {
	now := time.Date(2026, 5, 9, 8, 17, 32, 0, time.UTC)
	cases := []struct {
		rangeKey string
		want     int
	}{
		{"1d", 24},
		{"7d", 42},
		{"1m", 90},
	}
	for _, tc := range cases {
		t.Run(tc.rangeKey, func(t *testing.T) {
			start, end, err := overviewWindow(tc.rangeKey, now)
			if err != nil {
				t.Fatalf("overviewWindow error: %v", err)
			}
			interval, err := overviewSeriesBucketInterval(tc.rangeKey)
			if err != nil {
				t.Fatalf("overviewSeriesBucketInterval error: %v", err)
			}
			got := overviewBuckets(start, end, interval)
			if len(got) != tc.want {
				t.Fatalf("len = %d, want %d", len(got), tc.want)
			}
		})
	}
}

func TestOverviewBucketAt(t *testing.T) {
	start := time.Date(2026, 5, 2, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		at   time.Time
		want time.Time
	}{
		{start, start},
		{start.Add(3 * time.Hour), start},
		{start.Add(4 * time.Hour), start.Add(4 * time.Hour)},
		{start.Add(7 * time.Hour), start.Add(4 * time.Hour)},
		{start.Add(8 * time.Hour), start.Add(8 * time.Hour)},
	}
	for _, tc := range cases {
		got := overviewBucketAt(start, tc.at, 4*time.Hour)
		if !got.Equal(tc.want) {
			t.Errorf("overviewBucketAt(%s) = %s, want %s", tc.at, got, tc.want)
		}
	}
}

func TestToPgInt4Zero(t *testing.T) {
	v := toPgInt4(0)
	if v.Valid {
		t.Errorf("Valid = true for zero, want false")
	}
	v = toPgInt4(7)
	if !v.Valid || v.Int32 != 7 {
		t.Errorf("got %+v, want {7 true}", v)
	}
}

func TestToPgTextEmpty(t *testing.T) {
	v := toPgText("")
	if v.Valid {
		t.Errorf("Valid = true for empty string, want false")
	}
	v = toPgText("Foo Bar ")
	if !v.Valid || v.String != "Foo Bar " {
		t.Errorf("got %+v, want raw passthrough (no trim)", v)
	}
}

func TestMergeBreakdownTokensOnly(t *testing.T) {
	tokens := []db.ListOverviewBreakdownTokensRow{
		{ApiKeyID: 1, Model: "claude-4", UpstreamModel: "claude-4-up", ProviderID: 2, TotalTokens: 100},
		{ApiKeyID: 0, Model: "", UpstreamModel: "", ProviderID: 0, TotalTokens: 50},
	}
	got := mergeBreakdown(tokens, nil)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].TotalTokens != 100 || got[1].TotalTokens != 50 {
		t.Fatalf("sort by tokens desc broken: %+v", got)
	}
	for _, row := range got {
		if row.Costs == nil {
			t.Errorf("Costs must be non-nil empty slice for row %+v", row)
		}
	}
}

func TestMergeBreakdownTokensAndCosts(t *testing.T) {
	tokens := []db.ListOverviewBreakdownTokensRow{
		{ApiKeyID: 1, Model: "m1", UpstreamModel: "u1", ProviderID: 7, TotalTokens: 200},
	}
	costs := []db.ListOverviewBreakdownCostsRow{
		{ApiKeyID: 1, Model: "m1", UpstreamModel: "u1", ProviderID: 7, Currency: "USD", Amount: 1.5},
		{ApiKeyID: 1, Model: "m1", UpstreamModel: "u1", ProviderID: 7, Currency: "CNY", Amount: 9.9},
	}
	got := mergeBreakdown(tokens, costs)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	row := got[0]
	if row.TotalTokens != 200 {
		t.Errorf("TotalTokens = %d, want 200 (must NOT double-count across currencies)", row.TotalTokens)
	}
	if len(row.Costs) != 2 {
		t.Fatalf("Costs len = %d, want 2", len(row.Costs))
	}
	if row.Costs[0].Currency != "CNY" || row.Costs[1].Currency != "USD" {
		t.Errorf("Costs not sorted by currency: %+v", row.Costs)
	}
}

func TestMergeBreakdownCostOnlyRowKept(t *testing.T) {
	costs := []db.ListOverviewBreakdownCostsRow{
		{ApiKeyID: 0, Model: "ghost", UpstreamModel: "ghost-up", ProviderID: 0, Currency: "USD", Amount: 0.05},
	}
	got := mergeBreakdown(nil, costs)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].TotalTokens != 0 || len(got[0].Costs) != 1 {
		t.Errorf("got %+v, want zero tokens with one cost", got[0])
	}
	if got[0].Model != "ghost" {
		t.Errorf("Model = %q, want ghost", got[0].Model)
	}
}

func TestMergeBreakdownStableSort(t *testing.T) {
	tokens := []db.ListOverviewBreakdownTokensRow{
		{ApiKeyID: 5, Model: "a", UpstreamModel: "a", ProviderID: 1, TotalTokens: 10},
		{ApiKeyID: 3, Model: "a", UpstreamModel: "a", ProviderID: 1, TotalTokens: 10},
	}
	got := mergeBreakdown(tokens, nil)
	if got[0].ApiKeyID != 3 || got[1].ApiKeyID != 5 {
		t.Errorf("tie-break must be ApiKeyID asc, got %+v", got)
	}
}
