package api

import (
	"net/http"
	"testing"
)

// The words browser answers «что выучилось, что нет» as a list: status from
// lemma_state, memory figures derived from the cards, names from the pack —
// searchable without ever touching COLLATE NOCASE.
func TestWordsBrowserListsSearchesAndMeasures(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})
	noteOn(t, c, map[string]any{"hu": "Itt van a könyvben ma.", "targetForm": "könyvben"})

	var page WordsResponse
	if resp := c.json(http.MethodGet, "/api/words", nil, &page); resp.StatusCode != http.StatusOK {
		t.Fatalf("words: %d", resp.StatusCode)
	}
	if page.Total < 2 {
		t.Fatalf("total = %d, want the two words in play", page.Total)
	}
	for _, w := range page.Words {
		if w.Status != "learning" {
			t.Errorf("%s: status %q, want learning", w.Lemma, w.Status)
		}
		if w.Cards == 0 {
			t.Errorf("%s: no cards counted", w.Lemma)
		}
		if w.Retrievability != 0 {
			t.Errorf("%s: retrievability %v before any review", w.Lemma, w.Retrievability)
		}
	}

	// Answer one card: its word gains a retrievability and a next due.
	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	if len(queue.Items) == 0 {
		t.Fatal("no queue")
	}
	answered := queue.Items[0]
	c.json(http.MethodPost, "/api/review/answer",
		AnswerRequest{CardID: answered.CardID, Rating: 3, Typed: answered.Expected}, nil)

	c.json(http.MethodGet, "/api/words?sort=retrievability&order=desc", nil, &page)
	reviewed := 0
	for _, w := range page.Words {
		if w.Retrievability > 0 {
			reviewed++
			if w.NextDue == "" {
				t.Errorf("%s: reviewed but no next due", w.Lemma)
			}
		}
	}
	if reviewed != 1 {
		t.Errorf("%d words carry a retrievability, want exactly the answered one", reviewed)
	}

	// Search by the accented dictionary form, lowercase — the case-variants
	// path, not NOCASE.
	c.json(http.MethodGet, "/api/words?q=h%C3%A1z", nil, &page)
	found := false
	for _, w := range page.Words {
		if w.Lemma == "ház" {
			found = true
		}
	}
	if !found {
		t.Errorf("searching «ház» did not find the word; got %d rows", len(page.Words))
	}

	// Filters are validated, not silently widened.
	if resp := c.json(http.MethodGet, "/api/words?status=everything", nil, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown status answered %d, want 400", resp.StatusCode)
	}
}
