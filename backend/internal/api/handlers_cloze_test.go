package api

import (
	"net/http"
	"strings"
	"testing"

	"tanulom/internal/srs"
)

// The cloze blank, end to end, on a sentence with accents before the target.
//
// The bug: `note.target_start/target_end` are BYTE offsets into UTF-8, and the
// queue used to put them on the wire for the client to slice `hu` with. A
// JavaScript string is indexed in UTF-16 code units, so every accented letter
// before the target shifted the gap one place right — `Jó éjszakát, Timmy!`
// reached the learner as `Jó é_____immy!`, a blank in the middle of a word the
// exercise was not asking about.
//
// The split now happens where the bytes are, and the offsets do not cross the
// wire at all. This test is over the real HTTP handler on purpose: the unit
// test in internal/srs covers splitCloze, and would not have caught the field
// still being serialised.
func TestQueueSplitsClozeAroundAccents(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)

	// Two accented letters sit before the target, so a byte offset used as a
	// UTF-16 index lands two places to the right of the word.
	const hu = "Már a házban van a könyv."
	var created CreateNoteResponse
	if resp := c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": hu, "targetForm": "házban", "glossEn": "in the house",
	}, &created); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create note: %d", resp.StatusCode)
	}

	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)

	var found bool
	for _, it := range queue.Items {
		if it.NoteID != created.NoteID || it.Direction != srs.Cloze {
			continue
		}
		found = true
		if it.Target == nil {
			t.Fatal("a located cloze target must arrive split; the client cannot do it")
		}
		if it.Target.Before != "Már a " || it.Target.After != " van a könyv." {
			t.Errorf("blank in the wrong place: %q + ___ + %q",
				it.Target.Before, it.Target.After)
		}
		// Reassembling the two halves plus the answer must give the sentence
		// back. Anything else means the split dropped or duplicated text.
		if got := it.Target.Before + it.TargetForm + it.Target.After; got != hu {
			t.Errorf("split does not reassemble: %q != %q", got, hu)
		}
	}
	if !found {
		t.Skip("no cloze direction was planned for this note in this configuration")
	}
}

// The byte offsets must not be on the wire at all. Leaving them there is an
// invitation to slice with them again.
func TestQueueDoesNotShipByteOffsets(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": "Már a házban van a könyv.", "targetForm": "házban",
		"glossEn": "in the house",
	}, nil)

	_, body := c.do(http.MethodGet, "/api/review/queue", nil)
	for _, field := range []string{"targetStart", "targetEnd"} {
		if strings.Contains(string(body), field) {
			t.Errorf("%q is a UTF-8 byte offset and must not reach a UTF-16 client", field)
		}
	}
}
