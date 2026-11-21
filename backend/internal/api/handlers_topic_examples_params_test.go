package api

import (
	"net/http"
	"strings"
	"testing"
)

// Repeated `feature=` params collapse, and a list longer than the topic's own
// feature count is refused rather than fanned out into pack queries.
func TestTopicExamplesFeatureParamsAreDeduplicatedAndCapped(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)

	var got TopicExamplesResponse
	url := "/api/grammar/topics/locative-grid/examples?" + strings.Repeat("feature=Case=Ine&", 50) + "feature=Case=Ine"
	if resp := c.json(http.MethodGet, url, nil, &got); resp.StatusCode != 200 {
		t.Fatalf("duplicates: %d", resp.StatusCode)
	}
	if len(got.Features) != 1 || got.Features[0].FeatureID != "Case=Ine" {
		t.Errorf("duplicates must collapse to one feature, got %+v", got.Features)
	}

	// locative-grid owns 3 features in the fixture; a fourth distinct id is over the cap.
	resp, _ := c.do(http.MethodGet,
		"/api/grammar/topics/locative-grid/examples?feature=Case=Ine,Case=Ill,Case=Ela,Case=Sup", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("over the cap: want 400, got %d", resp.StatusCode)
	}
}
