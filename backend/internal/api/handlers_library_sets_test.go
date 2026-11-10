package api

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Ready-made reading sets (ADR-056). The sentences already exist in the pack;
// the handler only lists them and imports one as an ordinary text.

// addReadingSets writes two sets into a test pack before the server opens it.
func addReadingSets(t *testing.T, dir string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "langpack-hu-v1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE reading_set (code TEXT PRIMARY KEY, level TEXT, track TEXT, ord INTEGER,
			title_ru TEXT, title_en TEXT, theme_ru TEXT, theme_en TEXT)`,
		`CREATE TABLE reading_set_sentence (set_code TEXT, sentence_id INTEGER, ord INTEGER)`,
		// Inserted out of order: themes must still come after core units.
		`INSERT INTO reading_set VALUES ('t-x','A2','theme',701,'Живые примеры: Тема','Real examples: Theme','тема','theme')`,
		`INSERT INTO reading_set VALUES ('m1-u1','A1','core',101,'Живые примеры: Привет','Real examples: Hello','ситуация','situation')`,
		`INSERT INTO reading_set_sentence VALUES ('m1-u1',101,0),('m1-u1',102,1),('m1-u1',103,2)`,
		`INSERT INTO reading_set_sentence VALUES ('t-x',104,0)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

func newSetsServer(t *testing.T) (*Server, *client) {
	t.Helper()
	dir := t.TempDir()
	writeTestPack(t, dir)
	addReadingSets(t, dir)
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

func TestLibrarySetsListOrderLocaleAndShape(t *testing.T) {
	t.Parallel()
	_, c := newSetsServer(t)

	var ru []LibrarySetDTO
	if resp := c.json(http.MethodGet, "/api/library/sets", nil, &ru); resp.StatusCode != http.StatusOK {
		t.Fatalf("list: %d", resp.StatusCode)
	}
	if len(ru) != 2 || ru[0].Code != "m1-u1" || ru[1].Code != "t-x" {
		t.Fatalf("want core first then theme, got %+v", ru)
	}
	first := ru[0]
	if first.Sentences != 3 || first.Level != "A1" || first.Track != "core" {
		t.Errorf("bad row: %+v", first)
	}
	if first.Title != "Живые примеры: Привет" || first.TitleEN != "Real examples: Hello" ||
		first.TitleRU != first.Title || first.Theme != "ситуация" {
		t.Errorf("titles not localised: %+v", first)
	}
	if first.Imported || first.TextID != nil {
		t.Errorf("nothing imported yet: %+v", first)
	}

	var en []LibrarySetDTO
	c.json(http.MethodGet, "/api/library/sets?lang=en", nil, &en)
	if en[0].Title != "Real examples: Hello" || en[0].Theme != "situation" {
		t.Errorf("?lang=en not honoured: %+v", en[0])
	}
}

func TestLibrarySetImportMakesAnOrdinaryTextAndIsIdempotent(t *testing.T) {
	t.Parallel()
	s, c := newSetsServer(t)

	var first ImportSetResponse
	if resp := c.json(http.MethodPost, "/api/library/sets/m1-u1/import", nil, &first); resp.StatusCode != http.StatusCreated {
		t.Fatalf("import: %d", resp.StatusCode)
	}
	if !first.Created || first.TextID == 0 {
		t.Fatalf("first import: %+v", first)
	}
	var second ImportSetResponse
	if resp := c.json(http.MethodPost, "/api/library/sets/m1-u1/import", nil, &second); resp.StatusCode != http.StatusOK {
		t.Fatalf("repeat import: %d", resp.StatusCode)
	}
	if second.Created || second.TextID != first.TextID {
		t.Errorf("second import should return the same text, got %+v vs %+v", second, first)
	}

	// One text, not two, with the set's title and every sentence on its own
	// line-separated paragraph, source recorded, level estimated.
	var texts int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM text WHERE source_url = 'set:m1-u1'`).Scan(&texts); err != nil || texts != 1 {
		t.Fatalf("texts for the set: %d, %v", texts, err)
	}
	var title, body, level, kind string
	s.db.QueryRow(`SELECT title, body, level, kind FROM text WHERE id = ?`, first.TextID).Scan(&title, &body, &level, &kind)
	if title != "Живые примеры: Привет" || kind != "paste" {
		t.Errorf("title/kind: %q %q", title, kind)
	}
	if level == "" {
		t.Errorf("level not estimated: the set did not go through the ingestion path")
	}
	want := "Látom a házat az utcán.\n\nLátok egy házat az utcán.\n\nAzt a könyvet látja a fiú."
	if body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
	// The splitter keeps the three sentences apart.
	var sents int
	s.db.QueryRow(`SELECT COUNT(DISTINCT sent_idx) FROM text_token WHERE text_id = ?`, first.TextID).Scan(&sents)
	if sents != 3 {
		t.Errorf("stored sentences = %d, want 3", sents)
	}
	// The list reflects it; nothing was minted into the deck.
	var list []LibrarySetDTO
	c.json(http.MethodGet, "/api/library/sets", nil, &list)
	if !list[0].Imported || list[0].TextID == nil || *list[0].TextID != first.TextID || list[1].Imported {
		t.Errorf("list after import: %+v", list)
	}
	for _, table := range []string{"note", "card", "lemma_state", "morph_state"} {
		var n int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s has %d rows after an import (err %v): importing must not create cards", table, n, err)
		}
	}
}

func TestLibrarySetUnknownCodeIs404(t *testing.T) {
	t.Parallel()
	_, c := newSetsServer(t)
	if resp := c.json(http.MethodPost, "/api/library/sets/nope/import", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown code: %d, want 404", resp.StatusCode)
	}
}

func TestLibrarySetsAreEmptyOnAnOlderPack(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t) // test pack without the tables
	resp, body := c.do(http.MethodGet, "/api/library/sets", nil)
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "[]" {
		t.Errorf("older pack: %d %q, want 200 []", resp.StatusCode, body)
	}
	if resp := c.json(http.MethodPost, "/api/library/sets/m1-u1/import", nil, nil); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("import on a pack without sets: %d, want 503", resp.StatusCode)
	}
}

func TestLibrarySetsAreImportedPerLearner(t *testing.T) {
	t.Parallel()
	s, admin := newSetsServer(t)
	var mine ImportSetResponse
	admin.json(http.MethodPost, "/api/library/sets/m1-u1/import", nil, &mine)

	addUser(t, admin, "bea@example.com", goodPassword, "learner")
	bea := signIn(t, s, "bea@example.com", goodPassword)

	var list []LibrarySetDTO
	bea.json(http.MethodGet, "/api/library/sets", nil, &list)
	if list[0].Imported {
		t.Errorf("another learner's import shows as theirs: %+v", list[0])
	}
	var theirs ImportSetResponse
	bea.json(http.MethodPost, "/api/library/sets/m1-u1/import", nil, &theirs)
	if !theirs.Created || theirs.TextID == mine.TextID {
		t.Errorf("each learner gets their own copy: %+v vs %+v", theirs, mine)
	}
}

func TestLibrarySetsRequireSignIn(t *testing.T) {
	t.Parallel()
	s, _ := newSetsServer(t)
	anon := &client{t: t, app: s}
	if resp := anon.json(http.MethodGet, "/api/library/sets", nil, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous list: %d", resp.StatusCode)
	}
	if resp := anon.json(http.MethodPost, "/api/library/sets/m1-u1/import", nil, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous import: %d", resp.StatusCode)
	}
}

// Against the starter the binary really ships: every set imports, and the
// splitter never merges two of a set's sentences (the blank-line rule).
func TestEveryStarterSetImportsAsOneSentencePerParagraph(t *testing.T) {
	t.Parallel()
	s, c := newTestServer(t) // New installs the embedded starter
	if resp, body := c.do(http.MethodPost, "/api/setup", SetupRequest{
		Email: "anna@example.com", Password: goodPassword, UILocale: "ru", GlossLang: "both",
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: %d %s", resp.StatusCode, body)
	}
	var list []LibrarySetDTO
	c.json(http.MethodGet, "/api/library/sets", nil, &list)
	if len(list) != 36 {
		t.Fatalf("%d sets listed, want 36", len(list))
	}
	if list[0].Track != "core" || list[len(list)-1].Track != "theme" {
		t.Errorf("order: first %s last %s", list[0].Track, list[len(list)-1].Track)
	}
	for _, set := range list {
		var out ImportSetResponse
		if resp := c.json(http.MethodPost, "/api/library/sets/"+set.Code+"/import", nil, &out); resp.StatusCode != http.StatusCreated {
			t.Fatalf("import %s: %d", set.Code, resp.StatusCode)
		}
		var sents int
		s.db.QueryRow(`SELECT COUNT(DISTINCT sent_idx) FROM text_token WHERE text_id = ?`, out.TextID).Scan(&sents)
		// A pack sentence like "Szia! Hogy vagy?" is two sentences to the
		// splitter, so more is fine; fewer would mean two pack sentences merged.
		if sents < set.Sentences {
			t.Errorf("%s: stored %d sentences, the set has %d: sentences merged", set.Code, sents, set.Sentences)
		}
	}
}

func TestPasteCannotClaimASetSourceURL(t *testing.T) {
	t.Parallel()
	s, c := newSetsServer(t)
	for _, src := range []string{"set:m1-u1", "SET:m1-u1", "  set:x"} {
		resp, body := c.do(http.MethodPost, "/api/texts", CreateTextRequest{
			Title: "fake", Body: "Látom a házat.", Kind: "paste", SourceURL: src})
		if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "errors.badSourceUrl") {
			t.Errorf("%q: %d %s, want 400 errors.badSourceUrl", src, resp.StatusCode, body)
		}
	}
	// An ordinary URL still pastes, and the real import still works afterwards.
	if resp, body := c.do(http.MethodPost, "/api/texts", CreateTextRequest{
		Title: "ok", Body: "Látom a házat.", Kind: "paste", SourceURL: "https://example.com/a"}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("ordinary url: %d %s", resp.StatusCode, body)
	}
	var imp ImportSetResponse
	if resp := c.json(http.MethodPost, "/api/library/sets/m1-u1/import", nil, &imp); resp.StatusCode != http.StatusCreated || !imp.Created {
		t.Fatalf("import after rejected paste: %d %+v", resp.StatusCode, imp)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM text WHERE source_url LIKE 'set:%'`).Scan(&n)
	if n != 1 {
		t.Errorf("set-sourced texts = %d, want 1", n)
	}
}

func TestConcurrentImportsOfOneSetYieldOneText(t *testing.T) {
	t.Parallel()
	s, c := newSetsServer(t)
	const n = 6
	var wg sync.WaitGroup
	created := make(chan bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var r ImportSetResponse
			c.json(http.MethodPost, "/api/library/sets/m1-u1/import", nil, &r)
			created <- r.Created
		}()
	}
	wg.Wait()
	close(created)
	made := 0
	for ok := range created {
		if ok {
			made++
		}
	}
	var texts int
	s.db.QueryRow(`SELECT COUNT(*) FROM text WHERE source_url = 'set:m1-u1'`).Scan(&texts)
	if texts != 1 || made != 1 {
		t.Errorf("texts = %d, created responses = %d, want 1 and 1", texts, made)
	}
}

func TestImportLocksAreIndependentPerUserAndSet(t *testing.T) {
	t.Parallel()
	a := importLock(1, "m1-u1")
	if a != importLock(1, "m1-u1") {
		t.Fatal("same (user, set) must share one lock")
	}
	a.Lock()
	defer a.Unlock()
	done := make(chan struct{})
	go func() {
		for _, m := range []*sync.Mutex{importLock(2, "m1-u1"), importLock(1, "t-x")} {
			m.Lock()
			m.Unlock()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("another user's or another set's import waited on a held lock")
	}
}
