package api

import (
	"net/http"
	"testing"
)

// A note remembers its texts (ADR-022): the create branch links, the
// DUPLICATE branch links too — a word re-added from a second text creates no
// new note but must not lose that text's claim — and a foreign text id is
// refused rather than stored.
func TestCreateNoteLinksItsText(t *testing.T) {
	_, c := newPackedServer(t)

	makeText := func(title string) int64 {
		var created struct {
			ID int64 `json:"id"`
		}
		if resp := c.json(http.MethodPost, "/api/texts", map[string]any{
			"title": title, "body": "A házban van a könyv.", "kind": "plain",
		}, &created); resp.StatusCode != http.StatusCreated {
			t.Fatalf("create text: %d", resp.StatusCode)
		}
		return created.ID
	}
	textA, textB := makeText("a"), makeText("b")

	created := noteOn(t, c, map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban", "textId": textA,
	})

	var dup CreateNoteResponse
	if resp := c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban", "textId": textB,
	}, &dup); resp.StatusCode != http.StatusOK {
		t.Fatalf("duplicate add: %d", resp.StatusCode)
	}
	if !dup.Duplicate || dup.NoteID != created.NoteID {
		t.Fatalf("expected the same note back as a duplicate, got %+v", dup)
	}

	links := map[int64]bool{}
	rows, err := c.app.db.Query(`SELECT text_id FROM note_origin WHERE note_id = ?`, created.NoteID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		links[id] = true
	}
	rows.Close()
	if !links[textA] || !links[textB] {
		t.Errorf("note should be linked to both texts, got %v", links)
	}

	// A text the caller does not own is refused.
	if resp := c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": "Nagy a ház.", "targetForm": "ház", "textId": int64(9999),
	}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("foreign text id accepted: %d", resp.StatusCode)
	}

	// And the queue's text scope serves exactly this word.
	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue?scope=text:"+itoa(textA), nil, &queue)
	if len(queue.Items) == 0 || queue.Items[0].NoteID != created.NoteID {
		t.Errorf("text scope did not serve the linked word: %d items", len(queue.Items))
	}
	found := false
	for _, b := range queue.Scopes {
		if string(b.Scope) == "text:"+itoa(textA) {
			found = true
			if b.NewNotes != 1 {
				t.Errorf("text breakdown newNotes = %d, want 1", b.NewNotes)
			}
		}
	}
	if !found {
		t.Error("the text scope's breakdown is missing from the response")
	}
}
