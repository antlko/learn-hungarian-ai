package api

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tanulom/internal/morph"
	"tanulom/internal/srs"
)

// newOptionsServer is the standard test pack plus a handful of extra nouns, so
// `produce` has same-POS neighbours to offer. "fal" glosses the same as "ház"
// (a synonym) and must never be offered as a distractor.
func newOptionsServer(t *testing.T) (*Server, *client) {
	t.Helper()
	dir := t.TempDir()
	writeTestPack(t, dir)
	db, err := sql.Open("sqlite", filepath.Join(dir, "langpack-hu-v1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	add := func(id int64, lemma string, rank int, gloss string) {
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (?,?,?,?)`, []any{id, lemma, "NOUN", rank}},
			{`INSERT INTO lemma_gloss VALUES (?,?,?,?)`, []any{id, "en", 0, gloss}},
			{`INSERT INTO lemma_gloss VALUES (?,?,?,?)`, []any{id, "ru", 0, gloss}},
			{`INSERT INTO wordform VALUES (?,?,'NOUN','Case=Nom|Number=Sing','wiktionary','high')`, []any{lemma, id}},
		} {
			if _, err := db.Exec(q.sql, q.args...); err != nil {
				t.Fatal(err)
			}
		}
	}
	add(20, "fal", 101, "ház (en)") // synonym of ház: same first-sense gloss
	add(21, "autó", 102, "car")
	add(22, "asztal", 103, "table")
	add(23, "kert", 104, "garden")
	add(24, "utca", 105, "street")
	add(25, "szék", 106, "chair")
	db.Close()

	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	c := &client{t: t, app: s}
	resp, body := c.do(http.MethodPost, "/api/setup", SetupRequest{
		Email: "anna@example.com", Password: goodPassword, UILocale: "ru", GlossLang: "both",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: %d %s", resp.StatusCode, body)
	}
	return s, c
}

func lemmaIDOf(t *testing.T, s *Server, lemma string) int64 {
	t.Helper()
	var id int64
	if err := s.pack.DB().QueryRow(`SELECT id FROM lemma WHERE lemma = ? ORDER BY id LIMIT 1`, lemma).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func optionItem(s *Server, dir srs.Direction, lemma string, expected string, cardID int64, t *testing.T) srs.QueueItem {
	id := lemmaIDOf(t, s, lemma)
	return srs.QueueItem{CardID: cardID, NoteID: cardID, Direction: dir, LemmaID: &id, Lemma: lemma, TargetForm: expected, Expected: expected}
}

func countOf(opts []string, want string) int {
	n := 0
	for _, o := range opts {
		if o == want {
			n++
		}
	}
	return n
}

func assertSound(t *testing.T, s *Server, opts []string, expected string) {
	t.Helper()
	if n := countOf(opts, expected); n != 1 {
		t.Errorf("expected %q appears %d times in %v", expected, n, opts)
	}
	if len(opts) < 3 || len(opts) > 5 {
		t.Errorf("want 3..5 options, got %v", opts)
	}
	for _, o := range opts {
		if o == expected {
			continue
		}
		d := morph.DiffForms(o, expected, nil)
		if d.Match || d.AccentOnly() {
			t.Errorf("distractor %q would also be accepted for %q", o, expected)
		}
		var n int
		s.pack.DB().QueryRow(`SELECT (SELECT COUNT(*) FROM wordform WHERE form = ? AND source != 'generated')
			+ (SELECT COUNT(*) FROM lemma WHERE lemma = ?)`, o, o).Scan(&n)
		if n == 0 {
			t.Errorf("distractor %q is neither an attested form nor a lemma", o)
		}
	}
}

func TestClozeOptionsAreSameLemmaAttestedForms(t *testing.T) {
	s, _ := newOptionsServer(t)
	it := optionItem(s, srs.Cloze, "ház", "házban", 7, t)
	opts := s.buildOptions(it, time.Now())
	if opts == nil {
		t.Fatal("ház has four other attested forms; options expected")
	}
	assertSound(t, s, opts, "házban")
	for _, o := range opts {
		if !strings.HasPrefix(o, "ház") {
			t.Errorf("cloze distractor %q is not a form of ház", o)
		}
	}
}

func TestProduceOptionsAreNeighbourLemmasWithoutSynonyms(t *testing.T) {
	s, _ := newOptionsServer(t)
	it := optionItem(s, srs.Produce, "ház", "ház", 8, t)
	for day := 0; day < 20; day++ {
		opts := s.buildOptions(it, time.Now().AddDate(0, 0, day))
		if opts == nil {
			t.Fatal("nouns are plentiful; options expected")
		}
		assertSound(t, s, opts, "ház")
		if countOf(opts, "fal") != 0 {
			t.Errorf("synonym fal offered: %v", opts)
		}
	}
}

func TestOptionsDeterministicPerDayAndVaryAcrossDays(t *testing.T) {
	s, _ := newOptionsServer(t)
	it := optionItem(s, srs.Produce, "ház", "ház", 9, t)
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	a, b := s.buildOptions(it, now), s.buildOptions(it, now.Add(5*time.Hour))
	if strings.Join(a, "|") != strings.Join(b, "|") {
		t.Errorf("same day, different options: %v vs %v", a, b)
	}
	seen := map[string]bool{}
	for d := 0; d < 14; d++ {
		seen[strings.Join(s.buildOptions(it, now.AddDate(0, 0, d)), "|")] = true
	}
	if len(seen) < 2 {
		t.Error("options never change across days")
	}
}

func TestNoOptionsWhenNothingToOffer(t *testing.T) {
	s, _ := newPackedServer(t)
	// `azt` is the only PRON lemma and has a single form.
	var pron int64
	if err := s.pack.DB().QueryRow(`SELECT id FROM lemma WHERE upos = 'PRON'`).Scan(&pron); err != nil {
		t.Fatal(err)
	}
	it := srs.QueueItem{CardID: 3, Direction: srs.Cloze, Lemma: "az", TargetForm: "azt", Expected: "azt"}
	it.LemmaID = &pron
	if opts := s.buildOptions(it, time.Now()); opts != nil {
		t.Errorf("single-form lemma with no neighbours must have no options, got %v", opts)
	}
}

func TestReaderMinedNoteGetsOptionsAndChoiceGradesLikeTyping(t *testing.T) {
	_, c := newOptionsServer(t)
	var lemma int64
	c.app.pack.DB().QueryRow(`SELECT id FROM lemma WHERE lemma = 'ház'`).Scan(&lemma)
	created := noteOn(t, c, map[string]any{
		"hu": "Látom a házat az utcán.", "targetForm": "házat", "lemmaId": lemma,
		"source": "reader", "glossEn": "house",
	})

	var q QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &q)
	var cards []srs.QueueItem
	for _, it := range q.Items {
		if it.NoteID == created.NoteID && (it.Direction == srs.Cloze || it.Direction == srs.Produce) {
			cards = append(cards, it)
		}
	}
	if len(cards) == 0 {
		t.Skip("no cloze/produce card planned")
	}
	for _, it := range cards {
		if len(it.Options) < 3 {
			t.Fatalf("%s card has no options: %+v", it.Direction, it.Options)
		}
		if countOf(it.Options, it.Expected) != 1 {
			t.Errorf("expected %q not exactly once in %v", it.Expected, it.Options)
		}
		// Reload: same options.
		var q2 QueueResponse
		c.json(http.MethodGet, "/api/review/queue", nil, &q2)
		for _, it2 := range q2.Items {
			if it2.CardID == it.CardID && strings.Join(it2.Options, "|") != strings.Join(it.Options, "|") {
				t.Errorf("reload changed options: %v vs %v", it.Options, it2.Options)
			}
		}
	}

	// Grading: a correct choice is exactly a correct typed answer (Good, hint ignored).
	it := cards[0]
	var res AnswerResponse
	resp := c.json(http.MethodPost, "/api/review/answer", map[string]any{
		"cardId": it.CardID, "typed": it.Expected, "via": "choice", "usedHint": true, "durationMs": 2000,
	}, &res)
	if resp.StatusCode != 200 || !res.Correct || res.Rating != int(srs.Good) {
		t.Errorf("correct choice: status %d %+v", resp.StatusCode, res)
	}
	if len(cards) > 1 {
		wrong := ""
		for _, o := range cards[1].Options {
			if o != cards[1].Expected {
				wrong = o
			}
		}
		var res2 AnswerResponse
		c.json(http.MethodPost, "/api/review/answer", map[string]any{
			"cardId": cards[1].CardID, "typed": wrong, "via": "choice", "durationMs": 2000,
		}, &res2)
		if res2.Correct || res2.Rating != int(srs.Again) {
			t.Errorf("wrong choice must be Again: %+v", res2)
		}
	}
}

func TestViaIsValidatedAndPracticeAcceptsIt(t *testing.T) {
	_, c := newOptionsServer(t)
	created := noteOn(t, c, map[string]any{"hu": "A házban van a könyv.", "targetForm": "házban"})
	cardID := productiveCardID(t, c, created.NoteID)

	resp := c.json(http.MethodPost, "/api/review/answer", map[string]any{
		"cardId": cardID, "typed": "házban", "via": "telepathy",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad via: want 400, got %d", resp.StatusCode)
	}
	var pr PracticeAnswerResponse
	resp = c.json(http.MethodPost, "/api/practice/answer", map[string]any{
		"cardId": cardID, "typed": "házban", "via": "choice", "durationMs": 1000,
	}, &pr)
	if resp.StatusCode != 200 || !pr.Correct {
		t.Errorf("practice via=choice: %d %+v", resp.StatusCode, pr)
	}
	var n int
	c.app.db.QueryRow(`SELECT COUNT(*) FROM review_log`).Scan(&n)
	if n != 0 {
		t.Errorf("practice wrote %d review_log rows", n)
	}
}
