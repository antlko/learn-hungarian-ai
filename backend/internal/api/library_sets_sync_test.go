package api

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"testing"
)

// Authored stories and the sync that keeps imported sets current (ADR-074).

// newStoryServer opens a pack with one authored story ('s1', two paragraphs
// when the pack has `para`) and one authored dialogue.
func newStoryServer(t *testing.T, withPara bool) (*Server, *client) {
	t.Helper()
	dir := t.TempDir()
	writeTestPack(t, dir)
	db, err := sql.Open("sqlite", filepath.Join(dir, "langpack-hu-v1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	para := ""
	if withPara {
		para = ", para INTEGER NOT NULL DEFAULT 0"
	}
	qs := []string{
		`CREATE TABLE reading_set (code TEXT PRIMARY KEY, level TEXT, track TEXT, ord INTEGER,
			title_ru TEXT, title_en TEXT, theme_ru TEXT, theme_en TEXT)`,
		`CREATE TABLE reading_set_sentence (set_code TEXT, sentence_id INTEGER, ord INTEGER` + para + `)`,
		`INSERT INTO reading_set VALUES ('s1','A1','core',101,'История','Story','тема','theme')`,
		`INSERT INTO reading_set VALUES ('s1-dialogue','A1','dialogue',100,'Диалог','Dialogue','тема','theme')`,
		`INSERT INTO sentence VALUES (9100001,'Látom a házat.','authored','CC BY-SA 4.0',3,NULL)`,
		`INSERT INTO sentence VALUES (9100002,'Az utcán van.','authored','CC BY-SA 4.0',3,NULL)`,
		`INSERT INTO sentence VALUES (9100003,'Köszönöm, jól.','authored','CC BY-SA 4.0',2,NULL)`,
		`INSERT INTO sentence VALUES (9100004,'Szia!','authored','CC BY-SA 4.0',1,NULL)`,
	}
	if withPara {
		qs = append(qs,
			`INSERT INTO reading_set_sentence VALUES ('s1',9100001,0,0),('s1',9100002,1,0),('s1',9100003,2,1)`,
			`INSERT INTO reading_set_sentence VALUES ('s1-dialogue',9100004,0,0)`)
	} else {
		qs = append(qs,
			`INSERT INTO reading_set_sentence VALUES ('s1',9100001,0),('s1',9100002,1),('s1',9100003,2)`,
			`INSERT INTO reading_set_sentence VALUES ('s1-dialogue',9100004,0)`)
	}
	for _, q := range qs {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	db.Close()
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

func TestStoryImportJoinsParagraphSentencesBySpaceAndIsGenerated(t *testing.T) {
	t.Parallel()
	s, c := newStoryServer(t, true)
	var list []LibrarySetDTO
	c.json(http.MethodGet, "/api/library/sets", nil, &list)
	if len(list) != 2 || !list[0].Authored || !list[1].Authored || list[1].Track != "core" {
		t.Fatalf("stories and dialogues are both authored: %+v", list)
	}
	var out ImportSetResponse
	if resp := c.json(http.MethodPost, "/api/library/sets/s1/import", nil, &out); resp.StatusCode != http.StatusCreated {
		t.Fatalf("import: %d", resp.StatusCode)
	}
	var body, kind string
	s.db.QueryRow(`SELECT body, kind FROM text WHERE id = ?`, out.TextID).Scan(&body, &kind)
	if want := "Látom a házat. Az utcán van.\n\nKöszönöm, jól."; body != want || kind != "generated" {
		t.Errorf("body=%q kind=%q, want %q generated", body, kind, want)
	}
	// The dialogue is unchanged: one turn per line.
	c.json(http.MethodPost, "/api/library/sets/s1-dialogue/import", nil, &out)
	s.db.QueryRow(`SELECT body FROM text WHERE id = ?`, out.TextID).Scan(&body)
	if body != "– Szia!" {
		t.Errorf("dialogue body = %q", body)
	}
}

func TestStoryImportOnAPackWithoutParaIsOneParagraphPerSentence(t *testing.T) {
	t.Parallel()
	s, c := newStoryServer(t, false)
	var out ImportSetResponse
	if resp := c.json(http.MethodPost, "/api/library/sets/s1/import", nil, &out); resp.StatusCode != http.StatusCreated {
		t.Fatalf("import: %d", resp.StatusCode)
	}
	var body string
	s.db.QueryRow(`SELECT body FROM text WHERE id = ?`, out.TextID).Scan(&body)
	if want := "Látom a házat.\n\nAz utcán van.\n\nKöszönöm, jól."; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

func TestSyncSetTextsReplacesStaleCopiesInPlaceAndIsIdempotent(t *testing.T) {
	t.Parallel()
	s, c := newStoryServer(t, true)
	var out ImportSetResponse
	c.json(http.MethodPost, "/api/library/sets/s1/import", nil, &out)
	var uid int64
	s.db.QueryRow(`SELECT user_id FROM text WHERE id = ?`, out.TextID).Scan(&uid)

	// Make the copy look like an older pack's version, as the learner read it.
	if _, err := s.db.Exec(`UPDATE text SET title = 'Фразы', body = 'Régi mondat.', kind = 'paste',
		read_at = '2026-01-01T00:00:00Z' WHERE id = ?`, out.TextID); err != nil {
		t.Fatal(err)
	}
	// A copy of a set the pack no longer has must never be touched or deleted.
	res, err := s.db.Exec(`INSERT INTO text (user_id, title, source_url, kind, body)
		VALUES (?, 'Gone', 'set:gone', 'paste', 'Maradok.')`, uid)
	if err != nil {
		t.Fatal(err)
	}
	goneID, _ := res.LastInsertId()

	replaced, skipped, failed := s.syncSetTexts()
	if replaced != 1 || skipped != 1 || failed != 0 {
		t.Fatalf("first run: replaced=%d skipped=%d failed=%d", replaced, skipped, failed)
	}
	var id int64
	var title, body, kind, readAt string
	s.db.QueryRow(`SELECT id, title, body, kind, read_at FROM text WHERE source_url = 'set:s1'`).
		Scan(&id, &title, &body, &kind, &readAt)
	if id != out.TextID || title != "История" || kind != "generated" ||
		body != "Látom a házat. Az utcán van.\n\nKöszönöm, jól." || readAt != "2026-01-01T00:00:00Z" {
		t.Errorf("not replaced in place: %d %q %q %q %q", id, title, kind, body, readAt)
	}
	var sents int
	s.db.QueryRow(`SELECT COUNT(DISTINCT sent_idx) FROM text_token WHERE text_id = ?`, id).Scan(&sents)
	if sents != 3 {
		t.Errorf("tokens not rebuilt: %d sentences, want 3", sents)
	}
	var goneBody string
	if err := s.db.QueryRow(`SELECT body FROM text WHERE id = ?`, goneID).Scan(&goneBody); err != nil || goneBody != "Maradok." {
		t.Errorf("unknown code was touched: %q %v", goneBody, err)
	}

	if r, sk, f := s.syncSetTexts(); r != 0 || sk != 2 || f != 0 {
		t.Errorf("second run must change nothing: replaced=%d skipped=%d failed=%d", r, sk, f)
	}
}
