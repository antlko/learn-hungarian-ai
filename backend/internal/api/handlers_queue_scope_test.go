package api

import (
	"net/http"
	"testing"

	"tanulom/internal/srs"
)

func TestQueueDefaultsToAll(t *testing.T) {
	_, c := newPackedServer(t)
	noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})

	for _, path := range []string{"/api/review/queue", "/api/review/queue?scope="} {
		var queue QueueResponse
		if resp := c.json(http.MethodGet, path, nil, &queue); resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d", path, resp.StatusCode)
		}
		if queue.Scope != srs.ScopeAll {
			t.Errorf("%s served scope %q, want all", path, queue.Scope)
		}
		if len(queue.Items) == 0 {
			t.Errorf("%s served nothing", path)
		}
	}
}

// Refused rather than quietly widened: a typo that returns the whole deck looks
// exactly like the feature working, and would be found only by counting cards.
func TestQueueRejectsAnUnknownScope(t *testing.T) {
	_, c := newPackedServer(t)
	resp := c.json(http.MethodGet, "/api/review/queue?scope=everything", nil, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got %d, want 400 naming the three scopes", resp.StatusCode)
	}
}

// A word added from a text is not course material and must not appear under
// «План», however the note was made.
func TestQueueScopeFiltersBySource(t *testing.T) {
	_, c := newPackedServer(t)
	created := noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})

	var reader, course QueueResponse
	c.json(http.MethodGet, "/api/review/queue?scope=reader", nil, &reader)
	c.json(http.MethodGet, "/api/review/queue?scope=course", nil, &course)

	var inReader bool
	for _, it := range reader.Items {
		if it.NoteID == created.NoteID {
			inReader = true
		}
	}
	if !inReader {
		t.Error("a word added from a text is missing from the reader pile")
	}
	for _, it := range course.Items {
		if it.NoteID == created.NoteID {
			t.Error("a word added from a text appeared under the course pile")
		}
	}
}

// The control has to label the tab the learner is NOT standing on, so every
// scope is counted whichever one was asked for.
func TestQueueAlwaysReportsEveryScopesCount(t *testing.T) {
	_, c := newPackedServer(t)
	noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})

	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue?scope=course", nil, &queue)
	if len(queue.Scopes) != len(srs.Scopes) {
		t.Fatalf("got %d scope counts, want one per %v", len(queue.Scopes), srs.Scopes)
	}
	by := map[srs.Scope]srs.Breakdown{}
	for _, sc := range queue.Scopes {
		by[sc.Scope] = sc
	}
	if by[srs.ScopeReader].NewCards == 0 {
		t.Error("the reader pile reports nothing waiting while asking for the course pile")
	}
	if got, want := by[srs.ScopeAll].NewCards, by[srs.ScopeCourse].NewCards+by[srs.ScopeReader].NewCards; got != want {
		t.Errorf("all = %d but course+reader = %d; the tabs would not add up", got, want)
	}
}
