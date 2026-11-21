package api

import (
	"database/sql"
	"encoding/json"
	"math/rand"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"tanulom/internal/drill"
)

// A packed server whose pack also has enough nouns for a six-tile sort item and
// a handful of attested sentences with and without translations.
func newSortServer(t *testing.T) (*Server, *client) {
	t.Helper()
	dir := t.TempDir()
	writeTestPack(t, dir)

	db, err := sql.Open("sqlite", filepath.Join(dir, "langpack-hu-v1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	nouns := []struct {
		id              int64
		lemma, ine, ill string
	}{
		{20, "kert", "kertben", "kertbe"}, {21, "szék", "székben", "székbe"},
		{22, "lány", "lányban", "lányba"}, {23, "híd", "hídban", "hídba"},
		{24, "tó", "tóban", "tóba"}, {25, "ember", "emberben", "emberbe"},
		{26, "hotel", "hotelben", "hotelbe"},
	}
	for i, n := range nouns {
		exec(`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (?,?,?,?)`, n.id, n.lemma, "NOUN", 300+i)
		exec(`INSERT INTO lemma_gloss VALUES (?,?,?,?)`, n.id, "en", 0, n.lemma+" (en)")
		exec(`INSERT INTO lemma_gloss VALUES (?,?,?,?)`, n.id, "ru", 0, n.lemma+" (ru)")
		for f, feats := range map[string]string{
			n.lemma: "Case=Nom|Number=Sing", n.ine: "Case=Ine|Number=Sing", n.ill: "Case=Ill|Number=Sing",
		} {
			exec(`INSERT INTO wordform VALUES (?,?,?,?,'wiktionary','high')`, f, n.id, "NOUN", feats)
		}
	}
	sent := func(id int64, hu, form, source string, tokens int) {
		exec(`INSERT INTO sentence VALUES (?,?,?,'CC0',?,1.0)`, id, hu, source, tokens)
		exec(`INSERT INTO sentence_token VALUES (?,1,?,NULL,'')`, id, form)
	}
	sent(201, "A kertben van egy kis fa.", "kertben", "test", 6)
	sent(202, "Ő a székben ül most.", "székben", "test", 5)
	sent(203, "A hídban sok kő van.", "hídban", "test", 5)
	sent(204, "Mi a hotelben alszunk ma.", "hotelben", "test", 5)
	sent(205, "Az emberben sok erő van.", "emberben", "test", 5)
	sent(206, "A tóban van egy hal.", "tóban", "test", 5)
	sent(207, "Te a házban vagy ma itt.", "házban", "test", 6)
	sent(208, "A kertben van egy nagy fa és egy kis ház itt ma.", "kertben", "test", 12) // too long
	sent(209, "A kertben fa.", "kertben", "test", 3)                                     // too short
	sent(210, "Az AI a kertben dolgozik most.", "kertben", "authored", 6)                // not attested
	exec(`INSERT INTO sentence_trans VALUES (201,'ru','В саду есть маленькое дерево.')`)
	exec(`INSERT INTO sentence_trans VALUES (201,'en','There is a small tree in the garden.')`)
	exec(`INSERT INTO sentence_trans VALUES (204,'en','We sleep in the hotel today.')`)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

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

// servedSort serves one sort item for the locative grid through the pending
// store, the way /api/drill/next does.
func servedSort(t *testing.T, s *Server) drill.Item {
	t.Helper()
	topic, _ := s.dag.Topic("locative-grid")
	r := drill.NewRecent()
	for i, k := range drill.AllKinds {
		if k != drill.KindSort {
			r.KindRank[k] = i
		}
	}
	for seed := int64(1); seed < 60; seed++ {
		items, _, err := s.drills.Next(drill.Request{Topic: topic, Phase: drill.Blocked, Count: 1,
			Recent: r, Rand: rngOf(seed)})
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 1 && items[0].Kind == drill.KindSort {
			served, err := s.pending.Put(1, items)
			if err != nil {
				t.Fatal(err)
			}
			return served[0]
		}
	}
	t.Fatal("no sort item could be served")
	return drill.Item{}
}

func featureReviews(t *testing.T, s *Server) (ratings map[string]int, n int) {
	t.Helper()
	rows, err := s.db.DB.Query(`SELECT feature_id, rating FROM feature_review`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	ratings = map[string]int{}
	for rows.Next() {
		var id string
		var r int
		if err := rows.Scan(&id, &r); err != nil {
			t.Fatal(err)
		}
		ratings[id] = r
		n++
	}
	return
}

func TestSortItemIsServedWithoutItsAnswer(t *testing.T) {
	t.Parallel()
	s, _ := newSortServer(t)
	it := servedSort(t, s)
	raw, _ := json.Marshal(it)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	for k := range m {
		if strings.EqualFold(k, "expected") || strings.EqualFold(k, "answer") {
			t.Errorf("served sort item has a %q field", k)
		}
	}
	if strings.Contains(string(raw), "=-b") || len(it.Tiles) != 6 || len(it.Columns) < 2 {
		t.Errorf("unexpected shape: %s", raw)
	}
}

func TestSortAnswerIsValidatedBeforeTheItemIsConsumed(t *testing.T) {
	t.Parallel()
	s, c := newSortServer(t)
	it := servedSort(t, s)
	id, expected := peekAnswer(t, s, it.ID)

	full := map[string]string{}
	for _, tile := range it.Tiles {
		full[tile.ID] = it.Columns[0]
	}
	missing := map[string]string{}
	for k, v := range full {
		missing[k] = v
		break
	}
	badCol := map[string]string{}
	for k := range full {
		badCol[k] = "-xyz"
	}
	extra := map[string]string{"nope": it.Columns[0]}
	for k, v := range full {
		extra[k] = v
	}
	for name, parts := range map[string]map[string]string{
		"none": nil, "missing a tile": missing, "invalid column": badCol, "unknown tile": extra,
	} {
		resp, body := c.do(http.MethodPost, "/api/drill/answer", DrillAnswerRequest{ItemID: id, Parts: parts})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d %s", name, resp.StatusCode, body)
		}
	}
	if _, n := featureReviews(t, s); n != 0 {
		t.Errorf("a refused answer graded %d features", n)
	}
	// The item survived every refusal and is still answerable.
	right := map[string]string{}
	for line := range strings.SplitSeq(expected, "\n") {
		tid, col, _ := strings.Cut(line, "=")
		right[tid] = col
	}
	var res DrillAnswerResponse
	if resp := c.json(http.MethodPost, "/api/drill/answer", DrillAnswerRequest{ItemID: id, Parts: right, DurationMs: 900}, &res); resp.StatusCode != 200 {
		t.Fatalf("answer after refusals: %d", resp.StatusCode)
	}
	if !res.Correct {
		t.Error("the attested columns were graded wrong")
	}
}

// One feature review per feature however many tiles: Good when everything is
// right, exactly one Again for the feature when a tile is wrong.
func TestSortGradesEachFeatureOnce(t *testing.T) {
	t.Parallel()
	for _, wrong := range []bool{false, true} {
		s, c := newSortServer(t)
		it := servedSort(t, s)
		id, expected := peekAnswer(t, s, it.ID)

		parts := map[string]string{}
		for line := range strings.SplitSeq(expected, "\n") {
			tid, col, _ := strings.Cut(line, "=")
			parts[tid] = col
		}
		wrongTiles := 0
		if wrong {
			// Two wrong tiles: still ONE Again per feature.
			for tid, col := range parts {
				if wrongTiles == 2 {
					break
				}
				for _, other := range it.Columns {
					if other != col {
						parts[tid] = other
						wrongTiles++
						break
					}
				}
			}
		}
		var res DrillAnswerResponse
		if resp := c.json(http.MethodPost, "/api/drill/answer", DrillAnswerRequest{ItemID: id, Parts: parts, DurationMs: 50}, &res); resp.StatusCode != 200 {
			t.Fatalf("answer: %d", resp.StatusCode)
		}
		if res.Correct == wrong {
			t.Fatalf("wrong=%v but correct=%v", wrong, res.Correct)
		}
		if res.Expected != "" {
			t.Errorf("the canonical encoding leaked into expected: %q", res.Expected)
		}
		if len(res.PartResults) != len(it.Tiles) {
			t.Fatalf("partResults = %d", len(res.PartResults))
		}
		bad := 0
		for _, pr := range res.PartResults {
			if pr.Expected == "" {
				t.Errorf("part %s carries no expected column", pr.ID)
			}
			if !pr.Correct {
				bad++
			}
		}
		if bad != wrongTiles {
			t.Errorf("%d wrong parts reported, %d made", bad, wrongTiles)
		}
		ratings, n := featureReviews(t, s)
		if n != len(it.FeatureIDs) || len(ratings) != len(it.FeatureIDs) {
			t.Fatalf("wrong=%v: %d feature reviews for %d features %v", wrong, n, len(it.FeatureIDs), it.FeatureIDs)
		}
		for fid, r := range ratings {
			if !slices.Contains(it.FeatureIDs, fid) {
				t.Errorf("graded %s, not required by the item", fid)
			}
			// 3 = Good (a tap never reaches Easy, whatever the speed), 1 = Again.
			if want := map[bool]int{false: 3, true: 1}[wrong]; r != want {
				t.Errorf("wrong=%v %s rated %d, want %d", wrong, fid, r, want)
			}
		}
	}
}

func TestTopicExamples(t *testing.T) {
	t.Parallel()
	_, c := newSortServer(t)

	get := func(path string) TopicExamplesResponse {
		t.Helper()
		var out TopicExamplesResponse
		if resp := c.json(http.MethodGet, path, nil, &out); resp.StatusCode != 200 {
			t.Fatalf("%s: %d", path, resp.StatusCode)
		}
		return out
	}
	res := get("/api/grammar/topics/locative-grid/examples?feature=Case=Ine")
	if len(res.Features) != 1 || res.Features[0].FeatureID != "Case=Ine" {
		t.Fatalf("shape: %+v", res)
	}
	exs := res.Features[0].Examples
	if len(exs) != drill.ExamplesPerFeature {
		t.Fatalf("want %d examples, got %d: %+v", drill.ExamplesPerFeature, len(exs), exs)
	}
	attested := map[int64]bool{201: true, 202: true, 203: true, 204: true, 205: true, 206: true, 207: true}
	for _, ex := range exs {
		if !attested[ex.SentenceID] {
			t.Errorf("sentence %d is not an attested 4-10 word pack sentence", ex.SentenceID)
		}
		focus, text := 0, ""
		for _, p := range ex.Parts {
			text += p.Text
			if p.Focus {
				focus++
				if !strings.HasSuffix(p.Text, "ban") && !strings.HasSuffix(p.Text, "ben") {
					t.Errorf("focus %q is not an inessive", p.Text)
				}
			}
		}
		if focus != 1 || strings.Count(text, " ") < 3 {
			t.Errorf("sentence %d: %d focus parts in %q", ex.SentenceID, focus, text)
		}
	}
	// Translated sentences come first, the shorter of them first.
	if exs[0].SentenceID != 204 || exs[0].TranslationEn == "" || exs[1].SentenceID != 201 ||
		exs[1].TranslationRu == "" || exs[1].TranslationEn == "" {
		t.Errorf("translated sentences first: %+v", exs[:2])
	}
	if exs[2].TranslationRu != "" || exs[2].TranslationEn != "" {
		t.Errorf("an untranslated sentence must not carry invented text: %+v", exs[2])
	}

	// exclude is honoured, and "more" gives different sentences.
	var shown []string
	for _, ex := range exs {
		shown = append(shown, itoa(ex.SentenceID))
	}
	more := get("/api/grammar/topics/locative-grid/examples?feature=Case=Ine&exclude=" + strings.Join(shown, ","))
	for _, ex := range more.Features[0].Examples {
		if slices.Contains(shown, itoa(ex.SentenceID)) {
			t.Errorf("sentence %d was excluded and came back", ex.SentenceID)
		}
	}
	if len(more.Features[0].Examples) != 3 {
		t.Errorf("3 attested sentences remain, got %d", len(more.Features[0].Examples))
	}

	// No feature given: every feature of the topic, no invented text for those
	// without material.
	all := get("/api/grammar/topics/locative-grid/examples")
	if len(all.Features) != 3 {
		t.Errorf("locative-grid has 3 features, got %d", len(all.Features))
	}

	// Features without a surface ending are empty.
	def := get("/api/grammar/topics/definite-conjugation/examples")
	if len(def.Features) != 2 {
		t.Fatalf("features: %+v", def)
	}
	for _, f := range def.Features {
		if len(f.Examples) != 0 {
			t.Errorf("%s has no ending but got examples: %+v", f.FeatureID, f.Examples)
		}
	}
	raw, _ := json.Marshal(def)
	if !strings.Contains(string(raw), `"examples":[]`) {
		t.Errorf("an empty list must be [] not null: %s", raw)
	}

	// Errors.
	for path, want := range map[string]int{
		"/api/grammar/topics/nope/examples":                           404,
		"/api/grammar/topics/locative-grid/examples?feature=Case=Acc": 400,
		"/api/grammar/topics/locative-grid/examples?exclude=a,b":      400,
	} {
		if resp, _ := c.do(http.MethodGet, path, nil); resp.StatusCode != want {
			t.Errorf("%s: want %d, got %d", path, want, resp.StatusCode)
		}
	}
}

func TestTopicExamplesRequireAuth(t *testing.T) {
	t.Parallel()
	s, _ := newSortServer(t)
	anon := &client{t: t, app: s}
	if resp, _ := anon.do(http.MethodGet, "/api/grammar/topics/locative-grid/examples", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func rngOf(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }
