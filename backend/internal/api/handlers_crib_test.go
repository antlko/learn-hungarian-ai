package api

import (
	"net/http"
	"tanulom/internal/config"
	"testing"
)

// A note mined from the learner's own reading has no pack sentence behind it,
// so the only thing that can say what its sentence means is the MT sidecar.
// ADR-014 makes the model label the entire defence, and CLAUDE.md requires the
// screen to work with every sidecar stopped. These pin both.

func packedWithMT(t *testing.T, endpoint string) (*Server, *client) {
	t.Helper()
	s, c := newPackedServer(t)
	if endpoint != "" {
		if _, err := s.updateConfig(func(cfg *config.Config) {
			cfg.Services.MT = config.Service{Provider: config.ProviderLocal, Endpoint: endpoint}
		}); err != nil {
			t.Fatalf("configure mt: %v", err)
		}
	}
	return s, c
}

// The queue is the app's highest-frequency screen. It must never be hostage to
// a sidecar that may be off, undeployed, or still loading 600 MB of weights.
func TestTheQueueNeverCallsTheTranslator(t *testing.T) {
	t.Parallel()
	fake := newFakeMT(t)
	_, c := packedWithMT(t, fake.URL)
	noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})

	before := fake.calls.Load()
	var queue QueueResponse
	if resp := c.json(http.MethodGet, "/api/review/queue", nil, &queue); resp.StatusCode != http.StatusOK {
		t.Fatalf("queue: %d", resp.StatusCode)
	}
	if got := fake.calls.Load(); got != before {
		t.Errorf("the queue made %d translation calls; it must read the cache and nothing else",
			got-before)
	}
}

// With MT off entirely the card is Hungarian plus its gloss, and nothing on the
// screen reports a failure — the same behaviour as a cache miss, so there is
// one thing to reason about rather than two.
func TestTheQueueWorksWithNoTranslatorAtAll(t *testing.T) {
	t.Parallel()
	_, c := packedWithMT(t, "")
	created := noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})

	it := queueItem(t, c, created.NoteID)
	if it.SentenceRU != "" || it.SentenceEN != "" {
		t.Errorf("a crib appeared with no translator configured: %q / %q", it.SentenceRU, it.SentenceEN)
	}
	if it.SentenceModel != "" {
		t.Errorf("sentenceModel = %q with no translator", it.SentenceModel)
	}
}

// The label is what makes a machine crib permissible on a card at all.
func TestAReaderCardCarriesALabelledCrib(t *testing.T) {
	t.Parallel()
	fake := newFakeMT(t)
	_, c := packedWithMT(t, fake.URL)
	const hu = "A házban van a könyv."

	// Warm the cache the same way the reader does.
	if resp := c.json(http.MethodPost, "/api/translate",
		TranslateRequest{Texts: []string{hu}, Target: "ru"}, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("translate: %d", resp.StatusCode)
	}
	created := noteOn(t, c, map[string]any{"hu": hu, "targetForm": "házban"})

	it := queueItem(t, c, created.NoteID)
	// The fake echoes the FLORES code the Go client maps `ru` to.
	if it.SentenceRU != "rus_Cyrl: "+hu {
		t.Errorf("sentenceRu = %q, want the cached crib", it.SentenceRU)
	}
	if it.SentenceModel != "test/nllb-200" {
		t.Errorf("sentenceModel = %q — an unlabelled machine translation is exactly what ADR-014 forbids",
			it.SentenceModel)
	}
}

// A pack translation was written by a person. Printing a model beside it would
// libel the translator and make the label meaningless where it matters.
func TestAPackTranslationCarriesNoModel(t *testing.T) {
	t.Parallel()
	_, c := packedWithMT(t, "")
	// sentenceId is normally set by the course path; passed directly here
	// because what matters is only that the sentence came from the pack.
	var created CreateNoteResponse
	sid := int64(104)
	if resp := c.json(http.MethodPost, "/api/notes", map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban", "sentenceId": sid,
	}, &created); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create note: %d", resp.StatusCode)
	}
	if it := queueItem(t, c, created.NoteID); it.SentenceModel != "" {
		t.Errorf("sentenceModel = %q on a pack sentence, which no machine wrote", it.SentenceModel)
	}
}

// The background top-up is what fills the cache for a deck that predates the
// feature. It runs OFF the response path, so the test waits for it.
func TestTheBackgroundWarmFillsTheCache(t *testing.T) {
	t.Parallel()
	fake := newFakeMT(t)
	s, c := packedWithMT(t, fake.URL)
	created := noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})

	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	s.bg.Wait()

	if it := queueItem(t, c, created.NoteID); it.SentenceModel == "" {
		t.Errorf("the crib never arrived: %+v", it)
	}
}
