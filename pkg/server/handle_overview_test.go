package server

import (
	"testing"
	"time"
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
	end := start.Add(3 * time.Hour)
	got := overviewBuckets(start, end)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if !got[0].Equal(start) {
		t.Errorf("first = %v, want %v", got[0], start)
	}
	if !got[2].Equal(start.Add(2 * time.Hour)) {
		t.Errorf("last = %v, want %v", got[2], start.Add(2*time.Hour))
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
