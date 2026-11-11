package api

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"testing"

	"tanulom/internal/course"
)

// newGreetingServer is a server whose course has one unit of words the test
// pack must illustrate in unusual ways. `extra` seeds the pack before it opens.
func newGreetingServer(t *testing.T, words [][2]string, extra func(exec func(q string, args ...any))) (*Server, *client) {
	t.Helper()
	dir := t.TempDir()
	writeTestPack(t, dir)
	pdb, err := sql.Open("sqlite", filepath.Join(dir, "langpack-hu-v1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := pdb.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`CREATE TABLE course_unit (id INTEGER PRIMARY KEY, code TEXT, month INTEGER, ord INTEGER,
		title_ru TEXT, title_en TEXT, goal_ru TEXT, goal_en TEXT, situation_ru TEXT,
		situation_en TEXT, topic_codes TEXT, track TEXT NOT NULL DEFAULT 'core',
		tier TEXT NOT NULL DEFAULT 'basic')`)
	exec(`CREATE TABLE course_word (unit_code TEXT, lemma TEXT, upos TEXT, gloss_ru TEXT,
		gloss_en TEXT, note TEXT, ord INTEGER)`)
	exec(`INSERT INTO course_unit (code, month, ord, title_ru, title_en, goal_ru, goal_en,
		situation_ru, situation_en, topic_codes, track, tier)
		VALUES ('g1',1,101,'t','t','g','g','s','s','','core','basic')`)
	for i, w := range words {
		exec(`INSERT INTO course_word VALUES ('g1',?,?,'р','e','',?)`, w[0], w[1], i)
	}
	extra(exec)
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

// A greeting lives in a one-word sentence. The three-token floor made `szia`
// unillustratable; the last-resort pass takes the attested one-token sentence.
func TestCourseWordTakesAOneTokenSentenceAsALastResort(t *testing.T) {
	t.Parallel()
	s, c := newGreetingServer(t, [][2]string{{"szia", "INTJ"}}, func(exec func(string, ...any)) {
		exec(`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (20,'szia','INTJ',900)`)
		exec(`INSERT INTO wordform VALUES ('szia',20,'INTJ','','wiktionary','high')`)
		exec(`INSERT INTO sentence VALUES (201,'Szia!','test','CC0',1,1.0)`)
		exec(`INSERT INTO sentence_token VALUES (201,0,'',20,'')`)
	})
	note, ok, err := s.noteForWord(course.Word{Lemma: "szia", UPOS: "INTJ"})
	if err != nil || !ok {
		t.Fatalf("noteForWord ok=%v err=%v, want a note from the one-token sentence", ok, err)
	}
	if note.HU != "Szia!" || note.TargetStart != 0 || note.TargetEnd != len("Szia") {
		t.Errorf("note = %q span %d..%d, want Szia! 0..4", note.HU, note.TargetStart, note.TargetEnd)
	}
	if !clozeable(note) {
		t.Error("the note has a span and must be cloze-able")
	}

	var out EnrolResponse
	if resp := c.json(http.MethodPost, "/api/course/g1/enrol", nil, &out); resp.StatusCode != 200 {
		t.Fatalf("enrol: %d", resp.StatusCode)
	}
	if out.NotesMade != 1 || len(out.Skipped) != 0 {
		t.Errorf("enrol = %+v, want one note and nothing skipped", out)
	}
}

// The analyser files `nagyon` under `nagy`: no reading maps the token to the
// course lemma id. The headword spelling still locates the word, but the
// features are ANOTHER lemma's and must not be graded.
func TestCourseWordFallsBackToTheHeadwordWithNoFeatures(t *testing.T) {
	t.Parallel()
	s, c := newGreetingServer(t, [][2]string{{"nagyon", "ADV"}}, func(exec func(string, ...any)) {
		exec(`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (30,'nagyon','ADV',90)`)
		exec(`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (31,'nagy','ADJ',91)`)
		exec(`INSERT INTO wordform VALUES ('nagyon',31,'ADJ','Case=Ine|Number=Sing','corpus','high')`)
		exec(`INSERT INTO sentence VALUES (202,'Nagyon jó nap van.','test','CC0',4,1.0)`)
		exec(`INSERT INTO sentence_token VALUES (202,0,'Nagyon',30,'')`)
	})
	note, ok, err := s.noteForWord(course.Word{Lemma: "nagyon", UPOS: "ADV"})
	if err != nil || !ok {
		t.Fatalf("noteForWord ok=%v err=%v, want the headword fallback", ok, err)
	}
	if note.TargetForm != "Nagyon" || note.TargetStart != 0 || note.TargetEnd != len("Nagyon") {
		t.Errorf("span = %q %d..%d, want Nagyon 0..6", note.TargetForm, note.TargetStart, note.TargetEnd)
	}
	if len(note.TargetFeats) != 0 {
		t.Errorf("TargetFeats = %v, want none: they belong to the lemma `nagy`", note.TargetFeats)
	}
	if note.LemmaID == nil || *note.LemmaID != 30 {
		t.Errorf("LemmaID = %v, want the course lemma 30", note.LemmaID)
	}

	var out EnrolResponse
	if resp := c.json(http.MethodPost, "/api/course/g1/enrol", nil, &out); resp.StatusCode != 200 || out.NotesMade != 1 {
		t.Errorf("enrol = %d %+v, want one note", resp.StatusCode, out)
	}
}

// noExample names the enrolled unit's words nothing can illustrate, and only those.
func TestCourseNamesWordsWithNoExample(t *testing.T) {
	t.Parallel()
	_, c := newGreetingServer(t, [][2]string{{"szia", "INTJ"}, {"viszlát", "INTJ"}}, func(exec func(string, ...any)) {
		exec(`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (20,'szia','INTJ',900)`)
		exec(`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (21,'viszlát','INTJ',901)`)
		exec(`INSERT INTO wordform VALUES ('szia',20,'INTJ','','wiktionary','high')`)
		exec(`INSERT INTO sentence VALUES (201,'Szia!','test','CC0',1,1.0)`)
		exec(`INSERT INTO sentence_token VALUES (201,0,'',20,'')`)
	})
	noExample := func() []string {
		var plan CourseResponse
		c.json(http.MethodGet, "/api/course", nil, &plan)
		for _, st := range plan.Plan.Units {
			if st.Unit.Code == "g1" {
				return st.NoExample
			}
		}
		t.Fatal("unit g1 missing")
		return nil
	}
	if got := noExample(); len(got) != 0 {
		t.Errorf("a unit not taken up reports %v, want none", got)
	}
	var out EnrolResponse
	c.json(http.MethodPost, "/api/course/g1/enrol", nil, &out)
	if got := noExample(); len(got) != 1 || got[0] != "viszlát" {
		t.Errorf("noExample = %v, want [viszlát]", got)
	}
	if len(out.Skipped) != 1 || out.Skipped[0].Lemma != "viszlát" {
		t.Errorf("enrol skipped = %+v, want viszlát", out.Skipped)
	}
}

// Unit phrases are pack sentences with a human RU translation, ranked by how many
// unit words they hold.
func TestUnitPhrases(t *testing.T) {
	t.Parallel()
	_, c := newGreetingServer(t, [][2]string{{"szia", "INTJ"}, {"jó", "ADJ"}}, func(exec func(string, ...any)) {
		exec(`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (20,'szia','INTJ',900)`)
		exec(`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (22,'jó','ADJ',20)`)
		exec(`INSERT INTO wordform VALUES ('szia',20,'INTJ','','wiktionary','high')`)
		exec(`INSERT INTO wordform VALUES ('jó',22,'ADJ','','wiktionary','high')`)
		add := func(id int, hu string, n int, src string, ru string, lemmas ...int) {
			exec(`INSERT INTO sentence VALUES (?,?,?,'CC0',?,1.0)`, id, hu, src, n)
			for i, l := range lemmas {
				exec(`INSERT INTO sentence_token VALUES (?,?,'',?,'')`, id, i, l)
			}
			if ru != "" {
				exec(`INSERT INTO sentence_trans VALUES (?,'ru',?)`, id, ru)
				exec(`INSERT INTO sentence_trans VALUES (?,'en','en text')`, id)
			}
		}
		add(301, "Szia!", 1, "tatoeba", "Привет!", 20)
		add(302, "Szia, jó napot!", 3, "tatoeba", "Привет, добрый день!", 20, 0, 22)
		add(303, "Jó.", 1, "tatoeba", "", 22)                     // no translation
		add(304, "Szia, szia!", 2, "authored", "Привет!", 20, 20) // authored
		add(305, "Egy két három négy öt hat hét szia.", 8, "tatoeba", "Длинное", 20)
	})
	var got struct {
		Phrases []PhraseDTO `json:"phrases"`
	}
	if resp := c.json(http.MethodGet, "/api/course/units/g1/phrases", nil, &got); resp.StatusCode != 200 {
		t.Fatalf("phrases: %d", resp.StatusCode)
	}
	if len(got.Phrases) != 2 || got.Phrases[0].ID != 302 || got.Phrases[1].ID != 301 {
		t.Fatalf("phrases = %+v, want [302 (two unit words), 301]", got.Phrases)
	}
	if got.Phrases[0].RU == "" || got.Phrases[0].EN != "en text" {
		t.Errorf("translations = %+v", got.Phrases[0])
	}
	if resp := c.json(http.MethodGet, "/api/course/units/nope/phrases", nil, nil); resp.StatusCode != 404 {
		t.Errorf("unknown unit = %d, want 404", resp.StatusCode)
	}
}
