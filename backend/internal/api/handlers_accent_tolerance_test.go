package api

import (
	"net/http"
	"testing"

	"tanulom/internal/srs"
)

// Accent tolerance, end to end over the real handler.
//
// The scenario is the one from the field: the learner typed `viszlat` on an
// English keyboard for a card whose answer is `Viszlát`, and was failed on
// hardware. Tolerance accepts the word, rates it Hard so it returns sooner,
// keeps the diff so the missing mark is underlined, and says in the response
// that this was a tolerated pass. It is on by default, it can be turned off,
// and the `spell` direction never honours it — that direction exists to train
// exactly the diacritics being forgiven.
func TestAccentToleranceAcceptsAccentlessTyping(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)

	var created CreateNoteResponse
	if resp := c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban", "glossEn": "in the house",
	}, &created); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create note: %d", resp.StatusCode)
	}

	cardID := productiveCardID(t, c, created.NoteID)

	var res AnswerResponse
	c.json(http.MethodPost, "/api/review/answer", map[string]any{
		"cardId": cardID, "typed": "hazban", "durationMs": 3000,
	}, &res)

	if !res.Correct {
		t.Fatal("hazban for házban must be accepted with tolerance on (the default)")
	}
	if !res.AcceptedWithAccents {
		t.Error("a tolerated pass must say so — a silent pass is the one forbidden outcome")
	}
	if res.Rating != int(srs.Hard) {
		t.Errorf("a tolerated pass rates Hard so it returns sooner, got %d", res.Rating)
	}
	if res.Diff == nil || res.Diff.Match {
		t.Error("the diff must still show the missing accent")
	}
}

func TestAccentToleranceCanBeTurnedOff(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)

	var settings SettingsDTO
	c.json(http.MethodGet, "/api/settings", nil, &settings)
	settings.Review.AccentTolerance = false
	var upd UpdateSettingsResponse
	if resp := c.json(http.MethodPut, "/api/settings",
		map[string]any{"review": settings.Review}, &upd); resp.StatusCode != 200 {
		t.Fatalf("settings: %d", resp.StatusCode)
	}
	if upd.Settings.Review.AccentTolerance {
		t.Fatal("the setting did not stick")
	}

	var created CreateNoteResponse
	c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban", "glossEn": "in the house",
	}, &created)
	cardID := productiveCardID(t, c, created.NoteID)

	var res AnswerResponse
	c.json(http.MethodPost, "/api/review/answer", map[string]any{
		"cardId": cardID, "typed": "hazban", "durationMs": 3000,
	}, &res)
	if res.Correct {
		t.Error("with tolerance off, hazban for házban is wrong — that is what off means")
	}
}

// productiveCardID finds a typed-direction card of the note, preferring one
// that is not `spell`.
//
// The direction it lands on MATTERS now: `produce` grades the dictionary form
// and `cloze` grades the surface form, so a helper that took whichever came
// first would silently swap which string these tests type against. Cloze is
// preferred explicitly, because `házban` is what they are written around; the
// alternative is that a change to a direction WEIGHT reads as a tolerance
// regression. See ADR-017.
func productiveCardID(t *testing.T, c *client, noteID int64) int64 {
	t.Helper()
	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	var spellID int64
	for _, it := range queue.Items {
		if it.NoteID != noteID {
			continue
		}
		switch it.Direction {
		case srs.Cloze:
			return it.CardID
		case srs.Spell:
			spellID = it.CardID
		}
	}
	if spellID != 0 {
		t.Skip("only a spell card was planned; tolerance does not apply there")
	}
	t.Fatal("no productive card in the queue for this note")
	return 0
}
