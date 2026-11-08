package api

import (
	"net/http"
	"testing"
)

// The trash, through the API. The promises the button makes are: the word
// leaves the queue and the counts on the same response, it is still reachable
// through free practice, the review queue refuses it, and a stale tab cannot
// schedule it. ADR-025.

func trashNote(t *testing.T, c *client, noteID int64) TrashResponse {
	t.Helper()
	var out TrashResponse
	resp := c.json(http.MethodPost, "/api/notes/"+itoa(noteID)+"/trash", nil, &out)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trash note %d: %d", noteID, resp.StatusCode)
	}
	return out
}

func TestTrashingAWordRemovesItFromTheQueueAndTheCounts(t *testing.T) {
	_, c := newPackedServer(t)

	noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})
	drop := noteOn(t, c, map[string]any{"hu": "Nagy a ház.", "targetForm": "ház"})

	var before QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &before)
	if len(before.Items) < 2 {
		t.Fatalf("expected both words in the queue, got %d items", len(before.Items))
	}

	out := trashNote(t, c, drop.NoteID)
	if !out.Changed || out.Trashed != 1 || out.Cards == 0 {
		t.Fatalf("trash response %+v", out)
	}
	// The response carries the fresh numbers, so the tab strip cannot disagree
	// with the queue it is labelling (ADR-021).
	if len(out.Scopes) == 0 {
		t.Error("trash returned no breakdowns; the tab strip has nothing to update from")
	}

	var after QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &after)
	for _, it := range after.Items {
		if it.NoteID == drop.NoteID {
			t.Error("the queue still serves a trashed word")
		}
	}
	if len(after.Items) != len(before.Items)-1 {
		t.Errorf("queue went from %d to %d items; want one fewer", len(before.Items), len(after.Items))
	}

	// The listing names it, with its dictionary form rather than the sentence.
	var list TrashListResponse
	c.json(http.MethodGet, "/api/trash", nil, &list)
	if list.Total != 1 || len(list.Notes) != 1 || list.Notes[0].NoteID != drop.NoteID {
		t.Fatalf("trash listing: %+v", list)
	}
	if list.Notes[0].Lemma == "" {
		t.Error("the trash row has no dictionary form; a sentence is not a word's name")
	}
}

// A text-scoped session must get its own chip's number back, not just the
// fixed three: it went from «+13» to no number at all the moment a word was
// trashed, because the text scope was simply missing from the fresh list.
func TestTrashReturnsTheCallersOwnScopeNumbers(t *testing.T) {
	_, c := newPackedServer(t)
	var created struct {
		ID int64 `json:"id"`
	}
	c.json(http.MethodPost, "/api/texts", map[string]any{
		"title": "t", "body": "A házban van a könyv.", "kind": "plain",
	}, &created)

	noteOn(t, c, map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban", "textId": created.ID,
	})
	drop := noteOn(t, c, map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "könyv", "textId": created.ID,
	})

	scope := "text:" + itoa(created.ID)
	var out TrashResponse
	if resp := c.json(http.MethodPost,
		"/api/notes/"+itoa(drop.NoteID)+"/trash?scope="+scope, nil, &out); resp.StatusCode != http.StatusOK {
		t.Fatalf("trash: %d", resp.StatusCode)
	}
	for _, b := range out.Scopes {
		if string(b.Scope) == scope {
			if b.NewNotes != 1 {
				t.Errorf("text scope newNotes = %d after trashing one of two, want 1", b.NewNotes)
			}
			return
		}
	}
	t.Errorf("the caller's own text scope is missing from the fresh breakdowns: %+v", out.Scopes)
}

// The trash is practiseable and never schedulable.
func TestTheTrashIsPractisedNeverReviewed(t *testing.T) {
	_, c := newPackedServer(t)
	drop := noteOn(t, c, map[string]any{"hu": "Nagy a ház.", "targetForm": "ház"})
	trashNote(t, c, drop.NoteID)

	if resp := c.json(http.MethodGet, "/api/review/queue?scope=trash", nil, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("review queue accepted scope=trash: %d", resp.StatusCode)
	}

	var practice PracticeQueueResponse
	if resp := c.json(http.MethodGet, "/api/practice/queue?scope=trash", nil, &practice); resp.StatusCode != http.StatusOK {
		t.Fatalf("practice over the trash: %d", resp.StatusCode)
	}
	if practice.Pool != 1 || len(practice.Items) != 1 || practice.Items[0].NoteID != drop.NoteID {
		t.Fatalf("practising the trash served %+v", practice)
	}
}

// A stale tab holding a card that has since been trashed must not schedule it —
// the whole promise is that nothing about the word moves afterwards.
func TestAnsweringATrashedCardIsRefused(t *testing.T) {
	_, c := newPackedServer(t)
	note := noteOn(t, c, map[string]any{"hu": "Nagy a ház.", "targetForm": "ház"})
	item := queueItem(t, c, note.NoteID)

	trashNote(t, c, note.NoteID)

	resp := c.json(http.MethodPost, "/api/review/answer", map[string]any{
		"cardId": item.CardID, "rating": 3, "typed": item.Expected,
	}, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("answering a trashed card returned %d, want 409", resp.StatusCode)
	}
}

// Restore puts it back, and re-adding the word from the reader does the same —
// otherwise «добавить» reported success on a word that stayed invisible.
func TestRestoreAndReAddBothBringAWordBack(t *testing.T) {
	_, c := newPackedServer(t)
	note := noteOn(t, c, map[string]any{"hu": "Nagy a ház.", "targetForm": "ház"})

	trashNote(t, c, note.NoteID)
	var back TrashResponse
	if resp := c.json(http.MethodPost, "/api/notes/"+itoa(note.NoteID)+"/restore", nil, &back); resp.StatusCode != http.StatusOK {
		t.Fatalf("restore: %d", resp.StatusCode)
	}
	if !back.Changed || back.Trashed != 0 {
		t.Fatalf("restore response %+v", back)
	}
	if queueItem(t, c, note.NoteID).NoteID != note.NoteID {
		t.Error("a restored word did not come back to the queue")
	}

	// ...and again, this time via the reader's own «добавить».
	trashNote(t, c, note.NoteID)
	var dup CreateNoteResponse
	if resp := c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": "Nagy a ház.", "targetForm": "ház",
	}, &dup); resp.StatusCode != http.StatusOK {
		t.Fatalf("re-add: %d", resp.StatusCode)
	}
	if !dup.Duplicate || !dup.Restored {
		t.Fatalf("re-adding a trashed word reported %+v; it must say it came back", dup)
	}
	if queueItem(t, c, note.NoteID).NoteID != note.NoteID {
		t.Error("re-adding a trashed word left it out of the queue")
	}
}

// A note that is not the caller's, or is gone, says so rather than reporting a
// silent success.
func TestTrashingAnUnknownNoteIs404(t *testing.T) {
	_, c := newPackedServer(t)
	if resp := c.json(http.MethodPost, "/api/notes/98765/trash", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown note returned %d, want 404", resp.StatusCode)
	}
	if resp := c.json(http.MethodPost, "/api/notes/0/trash", nil, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("note id 0 returned %d, want 400", resp.StatusCode)
	}
}

// ── «Знаю» marks the word, not just the cards (ADR-041) ─────────────────────

// noteLemma is the lemma a note resolved to, or 0. Not on CreateNoteResponse,
// which reports the lemma's STATUS rather than its id.
func noteLemma(t *testing.T, s *Server, noteID int64) int64 {
	t.Helper()
	var id *int64
	if err := s.db.QueryRow(`SELECT lemma_id FROM note WHERE id = ?`, noteID).Scan(&id); err != nil {
		t.Fatalf("read note lemma: %v", err)
	}
	if id == nil {
		return 0
	}
	return *id
}

// lemmaStatus reads what the deck believes about a word. Not exposed by an
// endpoint, so the test reads it where it lives.
func lemmaStatus(t *testing.T, s *Server, userID, lemmaID int64) string {
	t.Helper()
	var st string
	err := s.db.QueryRow(
		`SELECT status FROM lemma_state WHERE user_id = ? AND lemma_id = ?`,
		userID, lemmaID).Scan(&st)
	if err != nil {
		return ""
	}
	return st
}

// Pressing «Знаю» says something about the WORD. Setting the cards aside is
// only half of it: without the lemma mark the reader goes on treating the word
// as unknown in its n+1 scoring and the plan goes on counting it as missing,
// while the learner has just said they know it.
func TestKnowingAWordMarksItsLemmaKnownAndRestorePutsItBack(t *testing.T) {
	s, c := newPackedServer(t)

	n := noteOn(t, c, map[string]any{"hu": "Nagy a ház.", "targetForm": "ház"})
	lemma := noteLemma(t, s, n.NoteID)
	if lemma == 0 {
		t.Skip("this pack did not resolve the fixture word to a lemma")
	}
	if got := lemmaStatus(t, s, 1, lemma); got != "learning" {
		t.Fatalf("a freshly added word should be learning, got %q", got)
	}

	var out TrashResponse
	resp := c.json(http.MethodPost, "/api/notes/"+itoa(n.NoteID)+"/know", nil, &out)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /know: %d", resp.StatusCode)
	}
	if !out.Changed {
		t.Fatal("the note was not set aside")
	}
	if got := lemmaStatus(t, s, 1, lemma); got != "known" {
		t.Errorf("lemma status after «Знаю» = %q, want known", got)
	}

	// And back again: `learning`, the state a word has from being added at
	// all — not `new`, which would claim it had never been seen.
	resp = c.json(http.MethodPost, "/api/notes/"+itoa(n.NoteID)+"/restore", nil, &out)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /restore: %d", resp.StatusCode)
	}
	if got := lemmaStatus(t, s, 1, lemma); got != "learning" {
		t.Errorf("lemma status after restore = %q, want learning", got)
	}
}

// The old path is the same handler under its old name, so a tab that has not
// reloaded keeps working — and gets the new behaviour too.
func TestTheTrashPathStillWorksAndAlsoMarksTheWord(t *testing.T) {
	s, c := newPackedServer(t)

	n := noteOn(t, c, map[string]any{"hu": "Nagy a ház.", "targetForm": "ház"})
	lemma := noteLemma(t, s, n.NoteID)
	if lemma == 0 {
		t.Skip("this pack did not resolve the fixture word to a lemma")
	}
	trashNote(t, c, n.NoteID)
	if got := lemmaStatus(t, s, 1, lemma); got != "known" {
		t.Errorf("lemma status after /trash = %q, want known", got)
	}
}
