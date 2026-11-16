package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tanulom/internal/config"
)

// ADR-078: the Teacher translates without saving; the learner's press keeps it;
// precedence is override > curated fix > pack > cache.

func withFakeLLM(t *testing.T, s *Server, answer string, prompts *[]string) {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if prompts != nil {
			*prompts = append(*prompts, string(body))
		}
		out, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
			"message": map[string]any{"role": "assistant", "content": answer}}}})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(out)
	}))
	t.Cleanup(fake.Close)
	if _, err := s.updateConfig(func(cfg *config.Config) {
		cfg.Services.LLM.Provider = "openai"
		cfg.Services.LLM.Endpoint = fake.URL
		cfg.Services.LLM.Model = "teacher-model"
	}); err != nil {
		t.Fatalf("configure llm: %v", err)
	}
}

func TestTeacherTranslateSavesNothingAndRefusesWhenOff(t *testing.T) {
	t.Parallel()
	s, c := newPackedServer(t)
	req := map[string]any{"hu": "Mindenkinek a magáét.", "target": "ru"}
	if resp := c.json(http.MethodPost, "/api/sentence-translation/teacher", req, nil); resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("llm off answered %d, want 503", resp.StatusCode)
	}
	var prompts []string
	withFakeLLM(t, s, "  «Каждому своё.»\n", &prompts)
	var out SentenceTranslationDTO
	if resp := c.json(http.MethodPost, "/api/sentence-translation/teacher", req, &out); resp.StatusCode != http.StatusOK {
		t.Fatalf("teacher: %d", resp.StatusCode)
	}
	if out.Text != "Каждому своё." || out.Model != "teacher-model" {
		t.Errorf("got %+v", out)
	}
	if len(prompts) != 1 || strings.Contains(strings.ToUpper(prompts[0]), "ONLY") {
		t.Errorf("prompt must be sent once and never say ONLY: %v", prompts)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sentence_translation_override`).Scan(&n); err != nil || n != 0 {
		t.Errorf("asking saved %d rows (err %v)", n, err)
	}
	if resp := c.json(http.MethodPost, "/api/sentence-translation/teacher",
		map[string]any{"hu": "x", "target": "de"}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad target: %d", resp.StatusCode)
	}
}

func TestSentenceOverridePrecedenceAndReset(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	// A note whose sentence has a curated fix, and one that has none.
	fixed := noteOn(t, c, map[string]any{"hu": "Mindenkinek a magáét.", "targetForm": "magáét"})
	plain := noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})

	it := queueItem(t, c, fixed.NoteID)
	if it.SentenceRU != "Каждому своё." || it.SentenceEN != "To each their own." || it.SentenceModel != "" {
		t.Errorf("curated fix not applied or labelled as a model: %+v", it)
	}

	save := map[string]any{"hu": "Mindenkinek a magáét.", "target": "ru", "text": "Всем своё.", "model": "teacher-model"}
	if resp := c.json(http.MethodPut, "/api/sentence-translation", save, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("save: %d", resp.StatusCode)
	}
	it = queueItem(t, c, fixed.NoteID)
	if it.SentenceRU != "Всем своё." || it.SentenceModel != "teacher-model" {
		t.Errorf("override must beat the curated fix and carry its model: %+v", it)
	}
	if other := queueItem(t, c, plain.NoteID); other.SentenceRU == "Всем своё." {
		t.Error("an override leaked onto another sentence")
	}

	// Unlabelled saves are refused: they would read as human work.
	save["model"] = ""
	if resp := c.json(http.MethodPut, "/api/sentence-translation", save, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("save without model: %d", resp.StatusCode)
	}

	if !queueItem(t, c, fixed.NoteID).SentenceKept {
		t.Error("an active override must be flagged so the crib can offer a reset")
	}
	var orig OriginalSentenceDTO
	if resp := c.json(http.MethodDelete, "/api/sentence-translation?hu=Mindenkinek+a+mag%C3%A1%C3%A9t.&target=ru", nil, &orig); resp.StatusCode != http.StatusOK {
		t.Fatalf("reset: %d", resp.StatusCode)
	}
	if orig.SentenceRU != "Каждому своё." || orig.SentenceModel != "" {
		t.Errorf("reset must hand back the original: %+v", orig)
	}
	it = queueItem(t, c, fixed.NoteID)
	if it.SentenceRU != "Каждому своё." || it.SentenceModel != "" {
		t.Errorf("reset must fall back to the curated fix: %+v", it)
	}
}
