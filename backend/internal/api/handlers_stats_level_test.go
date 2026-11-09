package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"tanulom/internal/stats"
)

// newBandedServer is newPackedServer over a pack whose lemmas carry CEFR
// bands and frequencies: A1 = ház, könyv, lát, az(DET); A2 = az(PRON), egy.
func newBandedServer(t *testing.T) (*Server, *client) {
	t.Helper()
	dir := t.TempDir()
	writeTestPack(t, dir)
	pdb, err := sql.Open("sqlite", filepath.Join(dir, "langpack-hu-v1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []struct {
		id   int
		cefr string
		pm   float64
	}{{1, "A1", 1000}, {2, "A1", 500}, {3, "A1", 800}, {4, "A1", 5000}, {5, "A2", 100}, {6, "A2", 100}} {
		if _, err := pdb.Exec(`UPDATE lemma SET cefr=?, freq_pm=? WHERE id=?`, r.cefr, r.pm, r.id); err != nil {
			t.Fatal(err)
		}
	}
	pdb.Close()

	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	c := &client{t: t, app: s}
	if resp, body := c.do(http.MethodPost, "/api/setup", SetupRequest{
		Email: "anna@example.com", Password: goodPassword, UILocale: "ru", GlossLang: "both",
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: %d %s", resp.StatusCode, body)
	}
	return s, c
}

// declare writes a lemma_state row the way «Знаю» would.
func declare(t *testing.T, s *Server, lemmaID int64, status string) {
	t.Helper()
	var uid int64
	if err := s.db.QueryRow(`SELECT id FROM user LIMIT 1`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO lemma_state (user_id, lemma_id, status)
		VALUES (?,?,?)`, uid, lemmaID, status); err != nil {
		t.Fatal(err)
	}
	s.invalidateCounted(uid) // direct SQL bypasses recordInteraction
}

func TestStatsLevelEmptyDeckAndShape(t *testing.T) {
	_, c := newBandedServer(t)
	resp, body := c.do(http.MethodGet, "/api/stats/level", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("level: %d %s", resp.StatusCode, body)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"label", "band", "sublevel", "confidence", "coverage",
		"countedLemmas", "bands", "cappedBy", "methodKey"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("response lacks %q: %s", k, body)
		}
	}
	var got stats.LevelEstimate
	_ = json.Unmarshal(body, &got)
	if got.Label != "A1.1" || got.Confidence != "low" || got.CountedLemmas != 0 || got.Coverage != 0 {
		t.Errorf("empty deck = %+v", got)
	}
	if got.MethodKey != "level.method.v1" || len(got.Bands) != 5 || got.Bands[0].Total != 4 || got.Bands[1].Total != 2 {
		t.Errorf("bands/method = %+v", got)
	}
	band0 := raw["bands"].([]any)[0].(map[string]any)
	for _, k := range []string{"band", "counted", "total", "share", "target", "grammarReady", "grammarDrillable", "reached"} {
		if _, ok := band0[k]; !ok {
			t.Errorf("band lacks %q", k)
		}
	}
}

func TestStatsLevelCountsKnownNotLearningSteps(t *testing.T) {
	s, c := newBandedServer(t)
	// A word on its learning steps (a fresh card) does NOT count.
	noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})
	var got stats.LevelEstimate
	c.json(http.MethodGet, "/api/stats/level", nil, &got)
	if got.CountedLemmas != 0 {
		t.Fatalf("a word still in learning steps counted: %+v", got)
	}

	declare(t, s, 2, "known")
	declare(t, s, 3, "known")
	declare(t, s, 4, "known")
	c.json(http.MethodGet, "/api/stats/level", nil, &got)
	if got.CountedLemmas != 3 || got.Bands[0].Counted != 3 || got.Bands[0].Share != 0.75 {
		t.Fatalf("after three known: %+v", got)
	}
	if got.Band != "A1" || got.Sublevel != 3 {
		t.Errorf("label = %s", got.Label)
	}
	// coverage = (500+800+5000) / 7500
	if want := 6300.0 / 7500.0; got.Coverage < want-1e-9 || got.Coverage > want+1e-9 {
		t.Errorf("coverage = %v, want %v", got.Coverage, want)
	}
}

func TestStatsLevelBandListsHighestFrequencyUncounted(t *testing.T) {
	s, c := newBandedServer(t)
	declare(t, s, 4, "known") // the most frequent A1 word is already counted

	var got BandWordsResponse
	if resp := c.json(http.MethodGet, "/api/stats/level/band/A1", nil, &got); resp.StatusCode != 200 {
		t.Fatalf("band: %d", resp.StatusCode)
	}
	// Ranks: lát 50, ház 100, könyv 200.
	if len(got.Words) != 3 || got.Words[0].Lemma != "lát" || got.Words[1].Lemma != "ház" ||
		got.Words[0].FreqRank != 50 || got.Words[0].GlossRU != "lát (ru)" || got.Words[0].GlossEN != "lát (en)" {
		t.Fatalf("A1 words = %+v", got.Words)
	}
	c.json(http.MethodGet, "/api/stats/level/band/a1?limit=1", nil, &got)
	if len(got.Words) != 1 {
		t.Errorf("limit=1 returned %d", len(got.Words))
	}
	if resp := c.json(http.MethodGet, "/api/stats/level/band/Z9", nil, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown band answered %d, want 400", resp.StatusCode)
	}
	var empty BandWordsResponse
	c.json(http.MethodGet, "/api/stats/level/band/C1", nil, &empty)
	if empty.Words == nil || len(empty.Words) != 0 {
		t.Errorf("empty band must be [] not null: %+v", empty)
	}
}

func TestWordRowCarriesStageAndFilters(t *testing.T) {
	s, c := newBandedServer(t)
	noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})
	declare(t, s, 3, "known")

	var page WordsResponse
	c.json(http.MethodGet, "/api/words", nil, &page)
	stages := map[string]WordRow{}
	for _, w := range page.Words {
		stages[w.Lemma] = w
	}
	if w := stages["ház"]; w.Stage != "learning" || w.Reps != 0 {
		t.Errorf("fresh word = %+v", w)
	}
	if w := stages["lát"]; w.Stage != "known" {
		t.Errorf("known word = %+v", w)
	}

	// Answering «Снова» on a new card puts it on the learning steps with
	// figures from the library: steps are counted, the interval is minutes.
	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	c.json(http.MethodPost, "/api/review/answer",
		AnswerRequest{CardID: queue.Items[0].CardID, Rating: 1, Typed: "x"}, nil)
	c.json(http.MethodGet, "/api/words?stage=learning", nil, &page)
	if page.Total != 1 || page.Words[0].Lemma != "ház" || page.Words[0].Reps != 1 ||
		page.Words[0].StepsTotal == 0 {
		t.Errorf("stage=learning = %+v", page)
	}
	c.json(http.MethodGet, "/api/words?stage=known", nil, &page)
	if page.Total != 1 || page.Words[0].Lemma != "lát" {
		t.Errorf("stage=known = %+v", page)
	}
	c.json(http.MethodGet, "/api/words?stage=mature", nil, &page)
	if page.Total != 0 || page.Words == nil {
		t.Errorf("stage=mature = %+v", page)
	}
	if resp := c.json(http.MethodGet, "/api/words?stage=old", nil, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown stage answered %d, want 400", resp.StatusCode)
	}
}

func TestMasteryRowsNameTheirTopics(t *testing.T) {
	_, c := newPackedServer(t)
	created := noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})
	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	for _, it := range queue.Items {
		if it.Direction == "cloze" && it.NoteID == created.NoteID {
			c.json(http.MethodPost, "/api/review/answer",
				AnswerRequest{CardID: it.CardID, Typed: "házban", DurationMs: 1500}, nil)
		}
	}
	var m MasteryResponse
	c.json(http.MethodGet, "/api/stats/mastery", nil, &m)
	found := false
	for _, f := range m.Features {
		if f.Topics == nil {
			t.Errorf("%s: topics must be [] not null", f.FeatureID)
		}
		if f.FeatureID == "Case=Ine" {
			found = true
			if len(f.Topics) != 1 || f.Topics[0] != "locative-grid" {
				t.Errorf("Case=Ine topics = %v", f.Topics)
			}
		}
	}
	if !found {
		t.Fatalf("Case=Ine not graded: %+v", m.Features)
	}
}

// An unreadable pack must surface as an error, never as «nothing is in play»,
// which would reset every unit on the plan to untouched.
func TestInPlaySetReturnsItsError(t *testing.T) {
	s, c := newBandedServer(t)
	noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})
	var uid int64
	_ = s.db.QueryRow(`SELECT id FROM user LIMIT 1`).Scan(&uid)

	got, err := s.inPlaySet(uid)
	if err != nil || !got["ház"] {
		t.Fatalf("healthy: %v %v", got, err)
	}
	s.pack.DB().Close()
	if got, err := s.inPlaySet(uid); err == nil {
		t.Errorf("closed pack: got %v with no error", got)
	}
}

// More lemmas in play than one statement may bind: chunked, not an error.
func TestInPlaySetChunksLargeDecks(t *testing.T) {
	s, c := newBandedServer(t)
	noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})
	var uid int64
	_ = s.db.QueryRow(`SELECT id FROM user LIMIT 1`).Scan(&uid)
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for id := 10000; id < 10000+40000; id++ {
		if _, err := tx.Exec(`INSERT INTO lemma_state (user_id, lemma_id, status) VALUES (?,?, 'known')`, uid, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	got, err := s.inPlaySet(uid)
	if err != nil || !got["ház"] {
		t.Fatalf("40k lemmas in play: %v (ház=%v)", err, got["ház"])
	}
}

// GET /api/stats/level is hit on every page load: it must not write
// topic_progress (resolveCurriculum persists; the estimate must not).
func TestStatsLevelDoesNotWriteTopicProgress(t *testing.T) {
	s, c := newBandedServer(t)
	count := func() int {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM topic_progress`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count()
	for range 2 {
		if resp := c.json(http.MethodGet, "/api/stats/level", nil, nil); resp.StatusCode != 200 {
			t.Fatalf("level: %d", resp.StatusCode)
		}
	}
	if after := count(); after != before {
		t.Errorf("topic_progress rows %d -> %d: the level GET wrote", before, after)
	}
}

func TestStatsLevelBandCollapsesHomonyms(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, dir)
	pdb, err := sql.Open("sqlite", filepath.Join(dir, "langpack-hu-v1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`UPDATE lemma SET cefr='A1' WHERE id IN (1,2,3)`,
		// A better-ranked "Ház" with no gloss, and a worse "ház" duplicate of lát.
		`INSERT INTO lemma (id, lemma, upos, freq_rank, cefr) VALUES (90, 'Ház', 'VERB', 10, 'A1')`,
		`INSERT INTO lemma (id, lemma, upos, freq_rank, cefr) VALUES (91, 'lát', 'NOUN', 9000, 'A1')`,
	} {
		if _, err := pdb.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	pdb.Close()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	c := &client{t: t, app: s}
	if resp, body := c.do(http.MethodPost, "/api/setup", SetupRequest{
		Email: "anna@example.com", Password: goodPassword, UILocale: "ru", GlossLang: "both",
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: %d %s", resp.StatusCode, body)
	}

	var got BandWordsResponse
	c.json(http.MethodGet, "/api/stats/level/band/A1", nil, &got)
	seen := map[string]int{}
	for _, w := range got.Words {
		seen[strings.ToLower(w.Lemma)]++
	}
	for k, n := range seen {
		if n != 1 {
			t.Errorf("%q listed %d times: %+v", k, n, got.Words)
		}
	}
	if len(got.Words) != 3 {
		t.Fatalf("want 3 distinct lemmas, got %+v", got.Words)
	}
	// ház: the glossless best-ranked homonym loses to the one with a gloss.
	if got.Words[0].Lemma != "ház" || got.Words[0].GlossRU == "" {
		t.Errorf("first = %+v, want the glossed ház", got.Words[0])
	}
	// limit counts distinct lemmas, not rows.
	c.json(http.MethodGet, "/api/stats/level/band/A1?limit=2", nil, &got)
	if len(got.Words) != 2 {
		t.Errorf("limit=2 returned %d: %+v", len(got.Words), got.Words)
	}
}
