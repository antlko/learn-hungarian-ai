package api

import (
	"net/http"
	"testing"
)

// Sentence boundaries are frozen at ingest, so a text added before the splitter
// learned about line breaks kept a heading welded to the sentence below it —
// and every card mined from that text carried the join. The reader repairs it
// on open rather than asking the learner to re-add their library.
func TestOpeningATextRepairsItsSentenceBoundaries(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)

	const body = "A ház\n\nA könyv nagy."
	var created struct {
		ID int64 `json:"id"`
	}
	if resp := c.json(http.MethodPost, "/api/texts", map[string]any{
		"title": "teszt", "body": body, "kind": "plain",
	}, &created); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create text: %d", resp.StatusCode)
	}

	// Break it the way the old splitter would have: one sentence for the lot.
	if _, err := c.app.db.Exec(
		`UPDATE text_token SET sent_idx = 0 WHERE text_id = ?`, created.ID); err != nil {
		t.Fatalf("seed the old split: %v", err)
	}

	var detail struct {
		Tokens []struct {
			Form    string `json:"form"`
			SentIdx int    `json:"sentIdx"`
		} `json:"tokens"`
	}
	c.json(http.MethodGet, "/api/texts/"+itoa(created.ID), nil, &detail)

	sent := map[string]int{}
	for _, tk := range detail.Tokens {
		sent[tk.Form] = tk.SentIdx
	}
	if sent["ház"] == sent["könyv"] {
		t.Errorf("the heading and the sentence are still one unit: %+v", sent)
	}
}
