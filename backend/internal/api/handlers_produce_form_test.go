package api

import (
	"net/http"
	"testing"

	"tanulom/internal/config"
	"tanulom/internal/srs"
)

// A produce card must ask for the word, not for the sentence's inflection.
//
// The field failure this pins: a card printed «день; солнце» — the dictionary
// entry for `nap` — and graded the answer against `napokra`, the sublative
// plural that happened to occur in the corpus sentence the note was built from.
// Nothing on the card named a case, the placeholder promised «одно слово
// по-венгерски», and the learner could only fail. [R12] defines `produce` as
// form recall of a WORD; the form inside a sentence is what `cloze` asks.

// cardOfDirection finds the note's card for one specific direction.
//
// Explicit about the direction because two of them grade DIFFERENT strings: a
// helper that returned "whichever typed card came first" would silently test
// cloze while claiming to test produce, and the next weight tweak would look
// like a regression in the wrong place.
//
// Burial is turned off to find it, because burial is precisely what hides a
// note's other directions — one card per note per day is the whole point of it.
// Answering the siblings to dig the wanted one out, which is what this used to
// do, now buries them for the rest of the day and finds nothing.
func cardOfDirection(t *testing.T, c *client, noteID int64, want srs.Direction) int64 {
	t.Helper()
	if _, err := c.app.updateConfig(func(cfg *config.Config) {
		off := false
		cfg.Review.BurySiblings = &off
	}); err != nil {
		t.Fatalf("disable burial: %v", err)
	}
	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	for _, it := range queue.Items {
		if it.NoteID == noteID && it.Direction == want {
			return it.CardID
		}
	}
	t.Fatalf("no %s card for note %d", want, noteID)
	return 0
}

// houseNote creates a note whose target is INFLECTED — `házban` for `ház` —
// which is the shape every course note has and the shape the bug needed.
func houseNote(t *testing.T, c *client) CreateNoteResponse {
	t.Helper()
	var created CreateNoteResponse
	if resp := c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban",
		"glossRu": "дом", "glossEn": "house",
	}, &created); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create note: %d", resp.StatusCode)
	}
	return created
}

func TestProduceGradesTheDictionaryForm(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	created := houseNote(t, c)
	id := cardOfDirection(t, c, created.NoteID, srs.Produce)

	var out AnswerResponse
	c.json(http.MethodPost, "/api/review/answer",
		AnswerRequest{CardID: id, Typed: "ház"}, &out)

	if !out.Correct {
		t.Errorf("typing the dictionary form was marked wrong; expected = %q", out.Expected)
	}
	if out.Expected != "ház" {
		t.Errorf("expected = %q, want the dictionary form %q", out.Expected, "ház")
	}
}

// The assertion that fails before this change: the prompt is the lemma's gloss,
// so the lemma is the only answer it can be asking for.
func TestProduceRejectsTheSentencesInflectedForm(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	created := houseNote(t, c)
	id := cardOfDirection(t, c, created.NoteID, srs.Produce)

	var out AnswerResponse
	c.json(http.MethodPost, "/api/review/answer",
		AnswerRequest{CardID: id, Typed: "házban"}, &out)

	if out.Correct {
		t.Error("a produce card accepted the inflected form; that is the cloze card's question")
	}
}

// Cloze must NOT have moved. It blanks one exact span, and it is the only
// direction that writes to the morphological state space.
func TestClozeStillGradesTheSurfaceForm(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	created := houseNote(t, c)
	id := cardOfDirection(t, c, created.NoteID, srs.Cloze)

	var out AnswerResponse
	c.json(http.MethodPost, "/api/review/answer",
		AnswerRequest{CardID: id, Typed: "ház"}, &out)
	if out.Correct {
		t.Error("a cloze accepted the bare lemma; the FORM is its exercise")
	}
	if out.Expected != "házban" {
		t.Errorf("expected = %q, want the surface form %q", out.Expected, "házban")
	}
}

// Without a lemma there is nothing to fall back TO but the surface form. A card
// that grades the inflected form is wrong; one that grades nothing is broken.
func TestProduceFallsBackToTheSurfaceFormWithoutALemma(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)

	var created CreateNoteResponse
	// `Zebra` is in no pack table, so the analyser resolves no lemma for it.
	if resp := c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": "Zebra van itt.", "targetForm": "Zebra",
		"glossRu": "зебра", "glossEn": "zebra",
	}, &created); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create note: %d", resp.StatusCode)
	}
	id := cardOfDirection(t, c, created.NoteID, srs.Produce)

	var out AnswerResponse
	c.json(http.MethodPost, "/api/review/answer",
		AnswerRequest{CardID: id, Typed: "Zebra"}, &out)
	if !out.Correct {
		t.Errorf("no lemma to fall back on, so the surface form must grade; expected = %q", out.Expected)
	}
}

// «Подсказка» must be able to show the answer without grading the card.
//
// The bug: pressing it on a produce card revealed nothing at all. Revealing is
// the entire point of a hint, and the client had no way to know that `produce`
// wants the dictionary form while `cloze` wants the form in the sentence — so
// the queue now ships the string the card is graded against.
func TestTheQueueShipsTheStringEachCardIsGradedAgainst(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	created := houseNote(t, c)

	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)

	var seen int
	for _, it := range queue.Items {
		if it.NoteID != created.NoteID {
			continue
		}
		seen++
		want := "házban"
		if it.Direction == srs.Produce {
			want = "ház"
		}
		if it.Expected != want {
			t.Errorf("%s ships expected %q, want %q", it.Direction, it.Expected, want)
		}
	}
	if seen == 0 {
		t.Fatal("the note produced no queue item")
	}
}

// The shipped string and the graded string are the same string. If these ever
// part, the hint shows one answer and the grader wants another — which is worse
// than no hint at all.
func TestTheShippedAnswerIsTheOneTheGraderUses(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	created := houseNote(t, c)
	id := cardOfDirection(t, c, created.NoteID, srs.Produce)

	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	var shipped string
	for _, it := range queue.Items {
		if it.CardID == id {
			shipped = it.Expected
		}
	}
	if shipped == "" {
		t.Fatal("no expected answer shipped for the produce card")
	}

	// Typing exactly what the hint would have shown must be accepted.
	var out AnswerResponse
	c.json(http.MethodPost, "/api/review/answer",
		AnswerRequest{CardID: id, Typed: shipped}, &out)
	if !out.Correct {
		t.Errorf("the hint would have shown %q, which the grader rejected in favour of %q",
			shipped, out.Expected)
	}
}
