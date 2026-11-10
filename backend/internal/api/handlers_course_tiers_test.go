package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
)

// newCourseServer is newPackedServer over a pack that also carries a three-tier
// course: b1 (basic: ház, könyv), i1 (intermediate: lát), a1 (advanced: egy)
// and one theme unit t1 (az), which must never count toward the totals.
func newCourseServer(t *testing.T, withTier bool) (*Server, *client) {
	t.Helper()
	dir := t.TempDir()
	writeTestPack(t, dir)
	pdb, err := sql.Open("sqlite", filepath.Join(dir, "langpack-hu-v1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		if _, err := pdb.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	tier := ""
	if withTier {
		tier = ", tier TEXT NOT NULL DEFAULT 'basic'"
	}
	exec(`CREATE TABLE course_unit (id INTEGER PRIMARY KEY, code TEXT, month INTEGER, ord INTEGER,
		title_ru TEXT, title_en TEXT, goal_ru TEXT, goal_en TEXT, situation_ru TEXT,
		situation_en TEXT, topic_codes TEXT, track TEXT NOT NULL DEFAULT 'core'` + tier + `)`)
	exec(`CREATE TABLE course_word (unit_code TEXT, lemma TEXT, upos TEXT, gloss_ru TEXT,
		gloss_en TEXT, note TEXT, ord INTEGER)`)
	unit := func(code string, month, ord int, track, tr string) {
		if withTier {
			exec(`INSERT INTO course_unit (code, month, ord, title_ru, title_en, goal_ru, goal_en,
				situation_ru, situation_en, topic_codes, track, tier)
				VALUES (?,?,?,'t','t','g','g','s','s','',?,?)`, code, month, ord, track, tr)
		} else {
			exec(`INSERT INTO course_unit (code, month, ord, title_ru, title_en, goal_ru, goal_en,
				situation_ru, situation_en, topic_codes, track)
				VALUES (?,?,?,'t','t','g','g','s','s','',?)`, code, month, ord, track)
		}
	}
	word := func(code, lemma, upos string, i int) {
		exec(`INSERT INTO course_word VALUES (?,?,?,'р','e','',?)`, code, lemma, upos, i)
	}
	unit("b1", 1, 101, "core", "basic")
	word("b1", "ház", "NOUN", 0)
	word("b1", "könyv", "NOUN", 1)
	unit("i1", 7, 701, "core", "intermediate")
	word("i1", "lát", "VERB", 0)
	unit("a1", 13, 801, "core", "advanced")
	word("a1", "egy", "DET", 0)
	unit("t1", 6, 1001, "theme", "basic")
	word("t1", "az", "DET", 0)
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

func unitByCode(p CourseResponse, code string) (u struct {
	Tier        string
	WordsInPlay int
	Learned     int
	Status      string
}, ok bool) {
	for _, st := range p.Plan.Units {
		if st.Unit.Code == code {
			u.Tier, u.WordsInPlay, u.Learned, u.Status =
				st.Unit.Tier, st.WordsInPlay, st.Learned, string(st.Status)
			return u, true
		}
	}
	return u, false
}

// «Пройден» used to be inferred from "every word is in the deck", which is true
// the moment a unit is enrolled. `learned` is the honest figure: only counted
// lemmas (known, or graduated and still remembered) — ADR-057.
func TestCourseReportsLearnedSeparatelyFromInDeck(t *testing.T) {
	t.Parallel()
	s, c := newCourseServer(t, true)

	var fresh CourseResponse
	c.json(http.MethodGet, "/api/course", nil, &fresh)
	if fresh.Totals.Words != 4 || fresh.Totals.InDeck != 0 || fresh.Totals.Learned != 0 {
		t.Fatalf("fresh totals = %+v (theme word must not count)", fresh.Totals)
	}
	if fresh.Plan.Total != 3 {
		t.Errorf("plan.total = %d, want the 3 core units", fresh.Plan.Total)
	}

	// ház and könyv are both in play; only ház is `known`. lát is learning, on
	// its first step (not counted); egy is known.
	declare(t, s, 1, "known")
	declare(t, s, 2, "learning")
	declare(t, s, 3, "learning")
	declare(t, s, 6, "known") // egy is lemma 6 in the fixture (A2 band helper)

	var got CourseResponse
	c.json(http.MethodGet, "/api/course", nil, &got)
	b1, _ := unitByCode(got, "b1")
	if b1.WordsInPlay != 2 || b1.Learned != 1 || b1.Status != "done" {
		t.Errorf("b1 (all words in the deck) = %+v: done but only 1 learned", b1)
	}
	if b1.Tier != "basic" {
		t.Errorf("tier = %q", b1.Tier)
	}
	i1, _ := unitByCode(got, "i1")
	if i1.WordsInPlay != 1 || i1.Learned != 0 {
		t.Errorf("i1 = %+v, want in the deck but not learned", i1)
	}

	tot := got.Totals
	if tot.InDeck != 4 || tot.Words != 4 {
		t.Errorf("totals words/inDeck = %d/%d", tot.Words, tot.InDeck)
	}
	if want := (struct{ w, d, l int }{2, 2, 1}); tot.PerTier["basic"].Words != want.w ||
		tot.PerTier["basic"].InDeck != want.d || tot.PerTier["basic"].Learned != want.l {
		t.Errorf("basic = %+v", tot.PerTier["basic"])
	}
	if tot.PerTier["intermediate"].Learned != 0 || tot.PerTier["intermediate"].InDeck != 1 {
		t.Errorf("intermediate = %+v", tot.PerTier["intermediate"])
	}
	if tot.PerTier["advanced"].Words != 1 {
		t.Errorf("advanced = %+v", tot.PerTier["advanced"])
	}
	if tot.Learned != tot.PerTier["basic"].Learned+tot.PerTier["intermediate"].Learned+tot.PerTier["advanced"].Learned {
		t.Errorf("tier learned does not sum to the total: %+v", tot)
	}
}

func TestCourseTotalsJSONShape(t *testing.T) {
	t.Parallel()
	_, c := newCourseServer(t, true)
	resp, body := c.do(http.MethodGet, "/api/course", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("course: %d %s", resp.StatusCode, body)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	totals, ok := raw["totals"].(map[string]any)
	if !ok {
		t.Fatalf("no totals: %s", body)
	}
	for _, k := range []string{"words", "inDeck", "learned", "perTier"} {
		if _, ok := totals[k]; !ok {
			t.Errorf("totals lacks %q", k)
		}
	}
	per := totals["perTier"].(map[string]any)
	for _, tier := range []string{"basic", "intermediate", "advanced"} {
		row, ok := per[tier].(map[string]any)
		if !ok {
			t.Fatalf("perTier lacks %q", tier)
		}
		for _, k := range []string{"words", "inDeck", "learned"} {
			if _, ok := row[k]; !ok {
				t.Errorf("perTier.%s lacks %q", tier, k)
			}
		}
	}
	unit0 := raw["plan"].(map[string]any)["units"].([]any)[0].(map[string]any)
	if _, ok := unit0["learned"]; !ok {
		t.Errorf("unit state lacks learned: %v", unit0)
	}
	if u := unit0["unit"].(map[string]any); u["tier"] != "basic" {
		t.Errorf("unit.tier = %v", u["tier"])
	}
}

// A pack built before `tier` existed reads every unit as basic and the course
// still resolves.
func TestCourseFromAPackWithoutTierIsAllBasic(t *testing.T) {
	t.Parallel()
	_, c := newCourseServer(t, false)
	var got CourseResponse
	if resp := c.json(http.MethodGet, "/api/course", nil, &got); resp.StatusCode != 200 {
		t.Fatalf("course: %d", resp.StatusCode)
	}
	for _, code := range []string{"b1", "i1", "a1", "t1"} {
		if u, ok := unitByCode(got, code); !ok || u.Tier != "basic" {
			t.Errorf("%s: %+v ok=%v, want basic", code, u, ok)
		}
	}
	if got.Totals.PerTier["basic"].Words != 4 || got.Totals.PerTier["advanced"].Words != 0 {
		t.Errorf("perTier = %+v", got.Totals.PerTier)
	}
}
