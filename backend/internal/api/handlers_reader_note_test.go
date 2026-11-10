package api

import (
	"net/http"
	"testing"

	"tanulom/internal/srs"
)

// A word mined from a text is the shape the review screen handled worst: no
// meaning, no sentence card, and a produce prompt that rendered as nothing at
// all. These pin the three repairs.

func noteOn(t *testing.T, c *client, body map[string]any) CreateNoteResponse {
	t.Helper()
	var created CreateNoteResponse
	resp := c.json(http.MethodPost, "/api/notes", body, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create note: %d", resp.StatusCode)
	}
	return created
}

func queueItem(t *testing.T, c *client, noteID int64) srs.QueueItem {
	t.Helper()
	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	for _, it := range queue.Items {
		if it.NoteID == noteID {
			return it
		}
	}
	t.Fatalf("note %d produced no queue item", noteID)
	return srs.QueueItem{}
}

// The reader sends a gloss only when the learner typed one, and almost nobody
// does — so every word added from a text reached `produce`, whose entire prompt
// IS the gloss, with an empty question. The pack has a dictionary entry.
func TestReaderNoteBorrowsThePacksGloss(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	created := noteOn(t, c, map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban",
	})
	it := queueItem(t, c, created.NoteID)

	if it.GlossRU != "ház (ru)" {
		t.Errorf("glossRu = %q, want the pack's entry", it.GlossRU)
	}
	if it.GlossEN != "ház (en)" {
		t.Errorf("glossEn = %q, want the pack's entry", it.GlossEN)
	}
}

// Mislabelling a person's own words as the dictionary's is the one direction
// this must never fail in. The other locale is still filled: the reader sends
// only the learner's own, and a cross-locale fallback is already rendered
// behind the `review.glossFallback` badge.
func TestAGlossTheLearnerWroteIsNeverOverwritten(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	created := noteOn(t, c, map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban",
		"glossRu": "мой перевод",
	})
	it := queueItem(t, c, created.NoteID)

	if it.GlossRU != "мой перевод" {
		t.Errorf("glossRu = %q — the learner's own meaning was overwritten", it.GlossRU)
	}
	if it.GlossEN != "ház (en)" {
		t.Errorf("glossEn = %q, want the empty side filled from the pack", it.GlossEN)
	}
}

// A cloze needs a SPAN, not features. `a` is a determiner the pack carries with
// an EMPTY feature string, so before this it got no sentence card at all — and
// neither did 23 of 37 words mined from a real text.
func TestAFeaturelessTargetStillGetsAClozeCard(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	created := noteOn(t, c, map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "a",
	})

	if len(created.TargetFeats) != 0 {
		t.Fatalf("fixture drift: %q was expected to carry no features, got %v",
			"a", created.TargetFeats)
	}
	var found bool
	for _, d := range created.Directions {
		if d == string(srs.Cloze) {
			found = true
		}
	}
	if !found {
		t.Fatalf("directions = %v, want a cloze among them", created.Directions)
	}
	if it := queueItem(t, c, created.NoteID); it.Target == nil && it.Direction == srs.Cloze {
		t.Error("the cloze card carries no split, so there is no blank to fill")
	}
}

// The invariant the loosened rule must not cost us: cloze is the only direction
// that writes to the morphological state space, and a cloze with no features
// must write NOTHING rather than inventing something to grade.
func TestAFeaturelessClozeGradesNoFeatures(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	created := noteOn(t, c, map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "a",
	})
	id := cardOfDirection(t, c, created.NoteID, srs.Cloze)

	var out AnswerResponse
	c.json(http.MethodPost, "/api/review/answer",
		AnswerRequest{CardID: id, Typed: "a"}, &out)

	if !out.Correct {
		t.Fatalf("the answer was the target itself; expected = %q", out.Expected)
	}
	if len(out.GradedFeatures) != 0 {
		t.Errorf("graded %v — a featureless cloze must not touch the second state space",
			out.GradedFeatures)
	}
}

// Without a span there is nothing to blank, so there is still no cloze.
func TestANoteWithNoSpanStillGetsNoCloze(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	created := noteOn(t, c, map[string]any{"hu": "A házban van a könyv."})

	for _, d := range created.Directions {
		if d == string(srs.Cloze) {
			t.Fatal("a note with no located target got a cloze with nothing to blank")
		}
	}
}

// The reader recolours the word from this. Guessing `learning` client-side
// would recolour a word the learner declared KNOWN backwards on every re-add.
func TestCreateNoteReportsTheLemmaStatus(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	created := noteOn(t, c, map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban",
	})
	if created.LemmaStatus != "learning" {
		t.Fatalf("lemmaStatus = %q, want learning", created.LemmaStatus)
	}

	// The learner then declares they know it outright.
	c.json(http.MethodPost, "/api/knowledge/lemma",
		SetKnownRequest{LemmaID: 1, Status: "known"}, nil)

	again := noteOn(t, c, map[string]any{
		"hu": "A házba megyek.", "targetForm": "házba",
	})
	if again.LemmaStatus != "known" {
		t.Errorf("lemmaStatus = %q — minting a card must not undo a learner's declaration",
			again.LemmaStatus)
	}
}

// The duplicate branch returned before the lemma was ever touched, which left
// a re-added word with no status to recolour from.
func TestDuplicateNoteStillReportsTheLemmaStatus(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	body := map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"}
	noteOn(t, c, body)

	var dup CreateNoteResponse
	if resp := c.json(http.MethodPost, "/api/notes", body, &dup); resp.StatusCode != http.StatusOK {
		t.Fatalf("duplicate: %d", resp.StatusCode)
	}
	if !dup.Duplicate {
		t.Fatal("want the duplicate flag")
	}
	if dup.LemmaStatus == "" {
		t.Error("a re-added word reports no status, so the reader cannot colour it")
	}
}
