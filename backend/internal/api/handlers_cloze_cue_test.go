package api

import (
	"net/http"
	"testing"

	"tanulom/internal/srs"
)

// A cloze must name the word it wants.
//
// `Jó ____, Timmy!` accepts reggelt, napot, estét and éjszakát — four real
// Hungarian greetings. A card that names none of them is asking the learner to
// guess our intent, and cloze is the ONLY direction that writes to the
// morphological state space: the guess lands in the grammar heatmap as a
// Case=Acc failure. The word is given; the FORM is the exercise.
func TestClozeCarriesTheLemmaAsItsCue(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)

	var created CreateNoteResponse
	if resp := c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban",
		"glossRu": "дом", "glossEn": "house",
	}, &created); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create note: %d", resp.StatusCode)
	}

	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)

	var found bool
	for _, it := range queue.Items {
		if it.NoteID != created.NoteID {
			continue
		}
		found = true
		if it.Lemma == "" {
			t.Fatalf("%s card carries no lemma; a cloze on it names no word", it.Direction)
		}
		// The cue is the DICTIONARY form, never the answer.
		if it.Direction == srs.Cloze && it.Lemma == it.TargetForm {
			t.Errorf("the cue %q is the answer itself", it.Lemma)
		}
	}
	if !found {
		t.Fatal("the note produced no cards")
	}
}
