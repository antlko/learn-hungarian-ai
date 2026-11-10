package api

import (
	"net/http"
	"testing"
)

func glossSetup(t *testing.T) *client {
	t.Helper()
	_, c := newTestServer(t)
	c.json(http.MethodPost, "/api/setup", SetupRequest{Email: "anna@example.com", Password: goodPassword}, nil)
	return c
}

// A word nobody has glossed is not an error. Almost every word is in that
// state, and a 404 would put a special case in every caller.
func TestGlossMissIsAnEmptyAnswerNotAnError(t *testing.T) {
	t.Parallel()
	c := glossSetup(t)

	var out GlossDTO
	if resp := c.json(http.MethodGet, "/api/glosses/Kinyitom", nil, &out); resp.StatusCode != http.StatusOK {
		t.Fatalf("get: %d", resp.StatusCode)
	}
	if out.Gloss != "" || out.Source != "" {
		t.Fatalf("want an empty entry, got %+v", out)
	}
	// Normalised on the way out too, so the client can key on what it sent.
	if out.Form != "kinyitom" {
		t.Fatalf("form = %q, want the lowercased key", out.Form)
	}
}

// The whole point of `source`: a meaning the model produced stays labelled as
// the model's, through storage and back. Losing it on save would launder an
// unchecked translation into something a person appears to stand behind —
// exactly what ADR-014 exists to prevent.
func TestMachineGlossStaysLabelledAcrossASave(t *testing.T) {
	t.Parallel()
	c := glossSetup(t)

	body := PutGlossRequest{Gloss: "Я открываю.", Source: "machine", Model: "facebook/nllb-200-distilled-600M"}
	if resp := c.json(http.MethodPut, "/api/glosses/Kinyitom", body, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("put: %d", resp.StatusCode)
	}

	var out GlossDTO
	c.json(http.MethodGet, "/api/glosses/kinyitom", nil, &out)
	if out.Source != "machine" {
		t.Fatalf("source = %q, want machine", out.Source)
	}
	if out.Model != "facebook/nllb-200-distilled-600M" {
		t.Fatalf("model = %q — the label must be able to name it", out.Model)
	}
	if out.Gloss != "Я открываю." {
		t.Fatalf("gloss = %q", out.Gloss)
	}
}

// Anything not explicitly machine output is the learner's own. The default has
// to fall this way: mislabelling a person's note as machine output is a
// cosmetic insult; the reverse passes off a guess as a person's understanding.
func TestUnknownSourceDefaultsToManual(t *testing.T) {
	t.Parallel()
	c := glossSetup(t)

	c.json(http.MethodPut, "/api/glosses/Kinyitom",
		PutGlossRequest{Gloss: "открываю", Source: "something-else", Model: "sneaky"}, nil)

	var out GlossDTO
	c.json(http.MethodGet, "/api/glosses/Kinyitom", nil, &out)
	if out.Source != "manual" || out.Model != "" {
		t.Fatalf("want manual with no model, got %+v", out)
	}
}

// `Kinyitom` at the head of a sentence and `kinyitom` inside one are the same
// word, and a learner who glossed one has glossed both.
func TestGlossIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	c := glossSetup(t)

	c.json(http.MethodPut, "/api/glosses/Kinyitom", PutGlossRequest{Gloss: "открываю"}, nil)

	var out GlossDTO
	c.json(http.MethodGet, "/api/glosses/KINYITOM", nil, &out)
	if out.Gloss != "открываю" {
		t.Fatalf("case must not matter: %+v", out)
	}
}

func TestGlossOverwriteAndDelete(t *testing.T) {
	t.Parallel()
	c := glossSetup(t)

	c.json(http.MethodPut, "/api/glosses/Kinyitom", PutGlossRequest{Gloss: "первое"}, nil)
	c.json(http.MethodPut, "/api/glosses/Kinyitom", PutGlossRequest{Gloss: "второе"}, nil)

	var out GlossDTO
	c.json(http.MethodGet, "/api/glosses/Kinyitom", nil, &out)
	if out.Gloss != "второе" {
		t.Fatalf("gloss = %q, want the newer one", out.Gloss)
	}

	if resp := c.json(http.MethodDelete, "/api/glosses/Kinyitom", nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	var gone GlossDTO
	c.json(http.MethodGet, "/api/glosses/Kinyitom", nil, &gone)
	if gone.Gloss != "" {
		t.Fatalf("still there after delete: %+v", gone)
	}
}

func TestGlossRefusesEmptyAndOverlong(t *testing.T) {
	t.Parallel()
	c := glossSetup(t)

	if resp := c.json(http.MethodPut, "/api/glosses/Kinyitom",
		PutGlossRequest{Gloss: "   "}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("blank gloss: %d, want 400", resp.StatusCode)
	}

	long := make([]rune, 501)
	for i := range long {
		long[i] = 'a'
	}
	if resp := c.json(http.MethodPut, "/api/glosses/Kinyitom",
		PutGlossRequest{Gloss: string(long)}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("overlong gloss: %d, want 400", resp.StatusCode)
	}
}
