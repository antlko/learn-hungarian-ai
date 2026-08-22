package api

import (
	"net/http"
	"testing"

	"tanulom/internal/srs"
)

// Undo end to end: answer a cloze that grades features, undo, and both state
// spaces must read exactly as if the answer never happened.
func TestUndoRevertsBothStateSpaces(t *testing.T) {
	_, c := newPackedServer(t)

	// A cloze note whose target carries Case=Ine.
	var created struct {
		NoteID      int64    `json:"noteId"`
		Directions  []string `json:"directions"`
		TargetFeats []string `json:"targetFeats"`
	}
	resp := c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban", "glossEn": "in the house",
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create note: %d", resp.StatusCode)
	}
	if len(created.TargetFeats) == 0 {
		t.Fatal("the target must carry features for this test to mean anything")
	}

	// Nothing yet — undo must say so rather than inventing work.
	resp, _ = c.do(http.MethodPost, "/api/review/undo", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("undo with no history: want 409, got %d", resp.StatusCode)
	}

	// Find the cloze card in the queue.
	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	var clozeID int64
	for _, item := range queue.Items {
		if item.Direction == "cloze" && item.NoteID == created.NoteID {
			clozeID = item.CardID
		}
	}
	if clozeID == 0 {
		t.Fatal("no cloze card in the queue")
	}

	// Answer it correctly.
	var ans AnswerResponse
	c.json(http.MethodPost, "/api/review/answer", AnswerRequest{
		CardID: clozeID, Typed: "házban", DurationMs: 1500,
	}, &ans)
	if !ans.Correct || len(ans.GradedFeatures) == 0 {
		t.Fatalf("answer did not grade features: %+v", ans)
	}

	var withState MasteryResponse
	c.json(http.MethodGet, "/api/stats/mastery", nil, &withState)
	if len(withState.Features) == 0 {
		t.Fatal("mastery must show the graded features")
	}

	// Undo.
	var undone srs.UndoResult
	if resp := c.json(http.MethodPost, "/api/review/undo", nil, &undone); resp.StatusCode != 200 {
		t.Fatalf("undo: %d", resp.StatusCode)
	}
	if undone.CardID != clozeID || len(undone.Features) == 0 {
		t.Fatalf("undo reverted the wrong thing: %+v", undone)
	}

	// The features must be GONE from the mastery map, not zeroed: no history is
	// "never seen", and never-seen must not render as failure.
	var after MasteryResponse
	c.json(http.MethodGet, "/api/stats/mastery", nil, &after)
	for _, m := range after.Features {
		for _, id := range undone.Features {
			if m.FeatureID == id {
				t.Errorf("feature %s still present after undo", id)
			}
		}
	}

	// The card is answerable again — same queue, same card, fresh state.
	var queue2 QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue2)
	found := false
	for _, item := range queue2.Items {
		if item.CardID == clozeID && item.IsNew {
			found = true
		}
	}
	if !found {
		t.Error("the undone card must return to the queue as new")
	}

	// A second undo has nothing left.
	resp, _ = c.do(http.MethodPost, "/api/review/undo", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("second undo: want 409, got %d", resp.StatusCode)
	}
}

// Undo after TWO answers reverts only the last, and the first answer's effect
// on the feature state survives — recomputed, not wiped.
func TestUndoRevertsOnlyTheLastAnswer(t *testing.T) {
	_, c := newPackedServer(t)

	c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban",
	}, nil)
	c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": "A könyvben sok kép van.", "targetForm": "könyvben",
	}, nil)

	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	var clozes []int64
	for _, item := range queue.Items {
		if item.Direction == "cloze" {
			clozes = append(clozes, item.CardID)
		}
	}
	if len(clozes) < 2 {
		t.Skipf("burying left %d clozes in today's queue", len(clozes))
	}

	c.json(http.MethodPost, "/api/review/answer", AnswerRequest{
		CardID: clozes[0], Typed: "házban", DurationMs: 1200}, nil)
	c.json(http.MethodPost, "/api/review/answer", AnswerRequest{
		CardID: clozes[1], Typed: "wrongwrong", DurationMs: 900}, nil)

	var undone srs.UndoResult
	c.json(http.MethodPost, "/api/review/undo", nil, &undone)
	if undone.CardID != clozes[1] {
		t.Fatalf("undid card %d, want the LAST answered %d", undone.CardID, clozes[1])
	}

	// The first answer's Case=Ine evidence must still be there.
	var mastery MasteryResponse
	c.json(http.MethodGet, "/api/stats/mastery", nil, &mastery)
	found := false
	for _, m := range mastery.Features {
		if m.FeatureID == "Case=Ine" && m.Observations >= 1 {
			found = true
		}
	}
	if !found {
		t.Error("undoing the second answer must not erase the first answer's evidence")
	}
}
