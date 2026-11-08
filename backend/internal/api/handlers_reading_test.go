package api

import (
	"net/http"
	"testing"
)

// Reading time in the ledger. ADR-044.
//
// The thing being tested is not really the arithmetic — it is that the reader
// counts AT ALL. Before this, an hour with a text moved no counter, so the
// day's obligation went unmet while the learner had been working: `active_ms`
// is one half of stats.Day.Met, and reading was not in it.

func TestReadingCountsTowardTheDaysMinutes(t *testing.T) {
	s, c := newPackedServer(t)

	var before OverviewResponse
	c.json(http.MethodGet, "/api/stats/overview", nil, &before)

	var out ReadingProgressResponse
	resp := c.json(http.MethodPost, "/api/reading/progress",
		map[string]any{"seconds": 45}, &out)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/reading/progress: %d", resp.StatusCode)
	}
	if out.Seconds != 45 {
		t.Errorf("credited %v seconds, want 45", out.Seconds)
	}

	var after OverviewResponse
	c.json(http.MethodGet, "/api/stats/overview", nil, &after)

	gained := after.Today.ActiveMs - before.Today.ActiveMs
	if gained != 45_000 {
		t.Errorf("active_ms gained %d, want 45000 — reading must count toward the "+
			"minutes the streak requires", gained)
	}
	if after.Today.ReadS != 45 {
		t.Errorf("read_s = %v, want 45 — the split is what lets the chart say "+
			"where an hour went", after.Today.ReadS)
	}
	_ = s
}

// The clamp is the whole defence against a tab left open: the server cannot
// see whether anyone is looking at the screen, so it makes a lying client
// cheap rather than impossible.
func TestOneReportCannotMintMoreThanAMinute(t *testing.T) {
	_, c := newPackedServer(t)

	var out ReadingProgressResponse
	c.json(http.MethodPost, "/api/reading/progress",
		map[string]any{"seconds": 86_400}, &out)
	if out.Seconds != maxReportSeconds {
		t.Errorf("credited %v seconds for a day-long report, want %d",
			out.Seconds, maxReportSeconds)
	}

	var ov OverviewResponse
	c.json(http.MethodGet, "/api/stats/overview", nil, &ov)
	if ov.Today.ActiveMs > maxReportSeconds*1000 {
		t.Errorf("a single report credited %d ms", ov.Today.ActiveMs)
	}
}

// An idle tick is the normal case, not a failure: the client reports on a
// timer and most ticks carry nothing.
func TestAnIdleTickIsAcceptedAndCreditsNothing(t *testing.T) {
	_, c := newPackedServer(t)

	var out ReadingProgressResponse
	resp := c.json(http.MethodPost, "/api/reading/progress",
		map[string]any{"seconds": 0}, &out)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("an idle tick should be accepted, got %d", resp.StatusCode)
	}
	if out.Seconds != 0 {
		t.Errorf("credited %v for an idle tick", out.Seconds)
	}

	var ov OverviewResponse
	c.json(http.MethodGet, "/api/stats/overview", nil, &ov)
	if ov.Today.ActiveMs != 0 {
		t.Errorf("an idle tick credited %d ms", ov.Today.ActiveMs)
	}
}

// `?days=` picks the window. The DEFAULT is 90 — the widest the chart offers —
// because the chart switches 7/30/90 client-side and the response therefore has
// to carry enough rows for the widest of them. Anything unrecognised gets the
// default rather than an error: a stale client should get a chart, not a
// failure.
func TestTheDayWindowIsClampedToTheRangesTheChartOffers(t *testing.T) {
	for raw, want := range map[string]int{
		"":         90,
		"7":        7,
		"30":       30,
		"90":       90,
		"365":      365,
		"10000":    90,
		"nonsense": 90,
		"-5":       90,
	} {
		if got := dayWindow(raw); got != want {
			t.Errorf("dayWindow(%q) = %d, want %d", raw, got, want)
		}
	}
}
