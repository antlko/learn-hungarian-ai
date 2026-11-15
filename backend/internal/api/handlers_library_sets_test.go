package api

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"tanulom/internal/readsets"
)

// Ready-made reading texts (ADR-056, ADR-066, ADR-076). They come from the
// catalogue embedded in the binary; the pack carries none (the test pack has no
// reading_set tables, like the starter).

// testCatalogJSON is a hand-written catalogue: a unit's short, medium, long and
// colloquial talk texts, and one theme text. Out of order on purpose.
const testCatalogJSON = `{"format":1,"sets":[
{"checked":"ai","code":"t-x","genre":"story","level":"A2","lines":[{"en":"I see.","hu":"Látok.","para":0,"ru":"Вижу."}],"ord":701,"register":"standard","themeEn":"theme","themeRu":"тема","tier":"medium","titleEn":"Theme","titleRu":"Тема","track":"theme","unit":"t-x","unitOrd":701,"words":1},
{"checked":"ai","code":"m1-u1","genre":"note","level":"A1","lines":[{"en":"I see the house.","hu":"Látom a házat.","para":0,"ru":"Вижу дом."},{"en":"It is on the street.","hu":"Az utcán van.","para":0,"ru":""},{"en":"","hu":"Köszönöm, jól.","para":1,"ru":""}],"ord":101,"register":"standard","themeEn":"situation","themeRu":"ситуация","tier":"medium","titleEn":"Story: Hello","titleRu":"История: Привет","track":"core","unit":"m1-u1","unitOrd":1,"words":8},
{"checked":"ai","code":"m1-u1-dialogue","genre":"dialogue","level":"A1","lines":[{"en":"Hi! How are you?","hu":"Szia! Hogy vagy?","para":0,"ru":"Привет! Как дела?","who":"A"},{"en":"Fine, thanks.","hu":"Köszönöm, jól.","para":1,"ru":"","who":"B"}],"ord":100,"register":"standard","themeEn":"situation","themeRu":"ситуация","tier":"short","titleEn":"Dialogue: Hello","titleRu":"Диалог: Привет","track":"core","unit":"m1-u1","unitOrd":1,"words":5},
{"checked":"ai","code":"m1-u1-long","genre":"story","level":"A1","lines":[{"en":"a","hu":"Látom a házat.","para":0,"ru":"а"},{"en":"b","hu":"Köszönöm, jól.","para":1,"ru":"б"}],"ord":102,"register":"standard","themeEn":"situation","themeRu":"ситуация","tier":"long","titleEn":"Long: Hello","titleRu":"Длинный: Привет","track":"core","unit":"m1-u1","unitOrd":1,"words":5},
{"checked":"ai","code":"m1-u1-talk","genre":"phone","level":"A1","lines":[{"en":"Hello","hu":"Halló!","para":0,"ru":"Алло","who":"A"},{"en":"Hi","hu":"Szia!","para":0,"ru":"Привет","who":"B"}],"ord":103,"register":"colloquial","themeEn":"situation","themeRu":"ситуация","tier":"talk","titleEn":"Talk","titleRu":"Разговор","track":"core","unit":"m1-u1","unitOrd":1,"words":2}
]}`

func testCatalog(t testing.TB) *readsets.Catalog {
	t.Helper()
	c, err := readsets.Parse([]byte(testCatalogJSON))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func newSetsServer(t *testing.T) (*Server, *client) {
	t.Helper()
	dir := t.TempDir()
	writeTestPack(t, dir)
	s, err := NewWithCatalog(dir, testCatalog(t))
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
	var codes []string
	for _, d := range ru {
		codes = append(codes, d.Code)
	}
	if got := strings.Join(codes, ","); got != "m1-u1-dialogue,m1-u1,m1-u1-long,m1-u1-talk,t-x" {
		t.Fatalf("order: %s", got)
	}
	m := ru[1]
	if m.Sentences != 3 || m.Level != "A1" || m.Track != "core" || !m.Authored || m.Unit != "m1-u1" ||
		m.Register != "standard" || m.Tier != "medium" || m.Genre != "note" || m.Words != 8 || m.Checked != "ai" {
		t.Errorf("bad row: %+v", m)
	}
	if m.Title != "История: Привет" || m.TitleEN != "Story: Hello" || m.TitleRU != m.Title || m.Theme != "ситуация" {
		t.Errorf("titles not localised: %+v", m)
	}
	if m.Imported || m.TextID != nil {
		t.Errorf("nothing imported yet: %+v", m)
	}
	if talk := ru[3]; talk.Tier != "talk" || talk.Register != "colloquial" || talk.Unit != "m1-u1" {
		t.Errorf("talk: %+v", talk)
	}

	var en []LibrarySetDTO
	c.json(http.MethodGet, "/api/library/sets?lang=en", nil, &en)
	if en[1].Title != "Story: Hello" || en[1].Theme != "situation" || en[0].Title != "Dialogue: Hello" {
		t.Errorf("?lang=en not honoured: %+v", en)
	}
}

// The pack plays no part: a pack WITHOUT reading tables (the test pack) lists
// and imports the full catalogue.
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
	var texts int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM text WHERE source_url = 'set:m1-u1'`).Scan(&texts); err != nil || texts != 1 {
		t.Fatalf("texts for the set: %d, %v", texts, err)
	}
	var title, body, level, kind string
	s.db.QueryRow(`SELECT title, body, level, kind FROM text WHERE id = ?`, first.TextID).Scan(&title, &body, &level, &kind)
	if title != "История: Привет" || kind != "generated" {
		t.Errorf("title/kind: %q %q", title, kind)
	}
	if level == "" {
		t.Errorf("level not estimated: the set did not go through the ingestion path")
	}
	if want := "Látom a házat. Az utcán van.\n\nKöszönöm, jól."; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
	var sents int
	s.db.QueryRow(`SELECT COUNT(DISTINCT sent_idx) FROM text_token WHERE text_id = ?`, first.TextID).Scan(&sents)
	if sents != 3 {
		t.Errorf("stored sentences = %d, want 3", sents)
	}
	var list []LibrarySetDTO
	c.json(http.MethodGet, "/api/library/sets", nil, &list)
	if !list[1].Imported || list[1].TextID == nil || *list[1].TextID != first.TextID || list[0].Imported || list[4].Imported {
		t.Errorf("list after import: %+v", list)
	}
	for _, table := range []string{"note", "card", "lemma_state", "morph_state"} {
		var n int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s has %d rows after an import (err %v): importing must not create cards", table, n, err)
		}
	}
}

func TestLibrarySetPreviewIsInSetOrderWithTranslations(t *testing.T) {
	t.Parallel()
	s, c := newSetsServer(t)

	var d LibrarySetDetailDTO
	if resp := c.json(http.MethodGet, "/api/library/sets/m1-u1", nil, &d); resp.StatusCode != http.StatusOK {
		t.Fatalf("preview: %d", resp.StatusCode)
	}
	if d.Code != "m1-u1" || d.Unit != "m1-u1" || d.Level != "A1" || d.Track != "core" || !d.Authored ||
		d.Register != "standard" || d.Title != "История: Привет" || d.TitleEN != "Story: Hello" || d.Theme != "ситуация" {
		t.Errorf("header: %+v", d)
	}
	if len(d.Sentences) != 3 || d.Sentences[0].HU != "Látom a házat." || d.Sentences[2].HU != "Köszönöm, jól." {
		t.Fatalf("not in set order: %+v", d.Sentences)
	}
	if d.Sentences[0].RU != "Вижу дом." || d.Sentences[0].EN != "I see the house." || d.Sentences[2].RU != "" {
		t.Errorf("translations: %+v", d.Sentences)
	}
	var en LibrarySetDetailDTO
	c.json(http.MethodGet, "/api/library/sets/m1-u1?lang=en", nil, &en)
	if en.Title != "Story: Hello" || en.Theme != "situation" || en.TitleRU != "История: Привет" {
		t.Errorf("?lang=en: %+v", en)
	}
	var texts int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM text`).Scan(&texts); err != nil || texts != 0 {
		t.Errorf("a preview created %d texts (err %v)", texts, err)
	}
	// The ETag is the catalogue hash: a repeat with it is a 304.
	resp, _ := c.do(http.MethodGet, "/api/library/sets/m1-u1", nil)
	if tag := resp.Header.Get("ETag"); !strings.Contains(tag, s.sets.Hash()) {
		t.Errorf("ETag %q lacks the catalogue hash", tag)
	}
}

func TestLibrarySetPreviewOfADialogueKeepsTurnOrderAndSpeakers(t *testing.T) {
	t.Parallel()
	_, c := newSetsServer(t)
	var d LibrarySetDetailDTO
	if resp := c.json(http.MethodGet, "/api/library/sets/m1-u1-dialogue", nil, &d); resp.StatusCode != http.StatusOK {
		t.Fatalf("preview: %d", resp.StatusCode)
	}
	if !d.Authored || d.Tier != "short" || d.Genre != "dialogue" || d.Title != "Диалог: Привет" {
		t.Errorf("header: %+v", d)
	}
	if len(d.Sentences) != 2 || d.Sentences[0].HU != "Szia! Hogy vagy?" || d.Sentences[1].HU != "Köszönöm, jól." {
		t.Fatalf("turn order: %+v", d.Sentences)
	}
	// Lines are shown clean: the turn marker is added on import only.
	if strings.HasPrefix(d.Sentences[0].HU, "–") {
		t.Errorf("a line carries a turn marker: %q", d.Sentences[0].HU)
	}
	if d.Sentences[0].RU != "Привет! Как дела?" || d.Sentences[0].EN != "Hi! How are you?" || d.Sentences[0].Who != "A" {
		t.Errorf("translations: %+v", d.Sentences)
	}
}

func TestLibrarySetPreviewUnknownCodeIs404AndNeedsSignIn(t *testing.T) {
	t.Parallel()
	s, c := newSetsServer(t)
	if resp := c.json(http.MethodGet, "/api/library/sets/nope", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown code: %d, want 404", resp.StatusCode)
	}
	anon := &client{t: t, app: s}
	if resp := anon.json(http.MethodGet, "/api/library/sets/m1-u1", nil, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous preview: %d, want 401", resp.StatusCode)
	}
}

func TestLibraryDialogueAndTalkImportAsOneTurnPerLineMarkedGenerated(t *testing.T) {
	t.Parallel()
	s, c := newSetsServer(t)
	var out ImportSetResponse
	if resp := c.json(http.MethodPost, "/api/library/sets/m1-u1-dialogue/import", nil, &out); resp.StatusCode != http.StatusCreated {
		t.Fatalf("import: %d", resp.StatusCode)
	}
	var title, body, kind string
	s.db.QueryRow(`SELECT title, body, kind FROM text WHERE id = ?`, out.TextID).Scan(&title, &body, &kind)
	if body != "– Szia! Hogy vagy?\n– Köszönöm, jól." || kind != "generated" || title != "Диалог: Привет" {
		t.Errorf("title=%q kind=%q body=%q", title, kind, body)
	}
	// The dash is not a word, and a line break ends a sentence.
	var sents, words int
	s.db.QueryRow(`SELECT COUNT(DISTINCT sent_idx) FROM text_token WHERE text_id = ?`, out.TextID).Scan(&sents)
	s.db.QueryRow(`SELECT COUNT(*) FROM text_token WHERE text_id = ? AND is_word = 1`, out.TextID).Scan(&words)
	if sents < 2 || words != 5 {
		t.Errorf("sentences=%d words=%d, want >=2 and 5", sents, words)
	}
	// A long text: one paragraph per blank-line group.
	c.json(http.MethodPost, "/api/library/sets/m1-u1-long/import", nil, &out)
	s.db.QueryRow(`SELECT body FROM text WHERE id = ?`, out.TextID).Scan(&body)
	if body != "Látom a házat.\n\nKöszönöm, jól." {
		t.Errorf("long body = %q", body)
	}
}

func TestLibrarySetUnknownCodeIs404(t *testing.T) {
	t.Parallel()
	_, c := newSetsServer(t)
	if resp := c.json(http.MethodPost, "/api/library/sets/nope/import", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown code: %d, want 404", resp.StatusCode)
	}
}

// Against the catalogue the binary really ships (starter pack, no reading
// tables): every set imports, and the splitter never merges two lines.
func TestEveryEmbeddedSetImports(t *testing.T) {
	t.Parallel()
	s, c := newTestServer(t) // New installs the embedded starter and catalogue
	if resp, body := c.do(http.MethodPost, "/api/setup", SetupRequest{
		Email: "anna@example.com", Password: goodPassword, UILocale: "ru", GlossLang: "both",
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: %d %s", resp.StatusCode, body)
	}
	var list []LibrarySetDTO
	c.json(http.MethodGet, "/api/library/sets", nil, &list)
	if len(list) != len(s.sets.Sets()) {
		t.Fatalf("%d sets listed, catalogue has %d", len(list), len(s.sets.Sets()))
	}
	if len(list) == 0 {
		t.Skip("catalogue not generated yet")
	}
	for _, set := range list {
		if !set.Authored || set.Unit == "" {
			t.Errorf("%s: authored=%v unit=%q", set.Code, set.Authored, set.Unit)
		}
		var out ImportSetResponse
		if resp := c.json(http.MethodPost, "/api/library/sets/"+set.Code+"/import", nil, &out); resp.StatusCode != http.StatusCreated {
			t.Fatalf("import %s: %d", set.Code, resp.StatusCode)
		}
		var sents int
		s.db.QueryRow(`SELECT COUNT(DISTINCT sent_idx) FROM text_token WHERE text_id = ?`, out.TextID).Scan(&sents)
		if sents < set.Sentences {
			t.Errorf("%s: stored %d sentences, the set has %d: sentences merged", set.Code, sents, set.Sentences)
		}
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
	if list[1].Imported {
		t.Errorf("another learner's import shows as theirs: %+v", list[1])
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

// A mini-dialogue carries a one-line scene (shown in the UI only, in the request
// locale); it imports as a script titled by its own title, and a voice message
// stays prose.
func TestLibraryMiniSceneLocaleAndImport(t *testing.T) {
	t.Parallel()
	const cat = `{"format":1,"sets":[
{"checked":"ai","code":"m1-u1-talk-bakery","genre":"dialogue","level":"A1","lines":[{"en":"One roll, please.","hu":"Egy kiflit kérek.","para":0,"ru":"Одну булочку.","who":"A"},{"en":"Here you are.","hu":"Tessék.","para":1,"ru":"Пожалуйста.","who":"B"}],"ord":110,"register":"colloquial","sceneEn":"At the bakery","sceneRu":"В пекарне","themeEn":"x","themeRu":"x","tier":"mini","titleEn":"Bakery","titleRu":"Пекарня","track":"core","unit":"m1-u1","unitOrd":1,"words":4},
{"checked":"ai","code":"m1-u1-talk","genre":"voicemsg","level":"A1","lines":[{"en":"Hi.","hu":"Szia.","para":0,"ru":"Привет."},{"en":"Bye.","hu":"Szia.","para":0,"ru":"Пока."}],"ord":109,"register":"colloquial","themeEn":"x","themeRu":"x","tier":"talk","titleEn":"Voice","titleRu":"Голосовое","track":"core","unit":"m1-u1","unitOrd":1,"words":2}
]}`
	dir := t.TempDir()
	writeTestPack(t, dir)
	k, err := readsets.Parse([]byte(cat))
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewWithCatalog(dir, k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	c := &client{t: t, app: s}
	if resp, body := c.do(http.MethodPost, "/api/setup", SetupRequest{
		Email: "anna@example.com", Password: goodPassword, UILocale: "ru", GlossLang: "both",
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: %d %s", resp.StatusCode, body)
	}
	var ru, en []LibrarySetDTO
	c.json(http.MethodGet, "/api/library/sets", nil, &ru)
	c.json(http.MethodGet, "/api/library/sets?lang=en", nil, &en)
	if len(ru) != 2 || ru[1].Code != "m1-u1-talk-bakery" || ru[1].Scene != "В пекарне" || ru[1].Register != "colloquial" || ru[1].Tier != "mini" {
		t.Errorf("ru list: %+v", ru)
	}
	if en[1].Scene != "At the bakery" || ru[1].SceneRU != "В пекарне" || ru[1].SceneEN != "At the bakery" || ru[0].Scene != "" {
		t.Errorf("en scene %q, talk scene %q", en[1].Scene, ru[0].Scene)
	}
	var d LibrarySetDetailDTO
	c.json(http.MethodGet, "/api/library/sets/m1-u1-talk-bakery?lang=en", nil, &d)
	if d.Scene != "At the bakery" || d.SceneRU != "В пекарне" || d.SceneEN != "At the bakery" || d.Title != "Bakery" {
		t.Errorf("detail: %+v", d)
	}
	var out ImportSetResponse
	c.json(http.MethodPost, "/api/library/sets/m1-u1-talk-bakery/import", nil, &out)
	var title, body string
	s.db.QueryRow(`SELECT title, body FROM text WHERE id = ?`, out.TextID).Scan(&title, &body)
	if title != "Пекарня" || body != "– Egy kiflit kérek.\n– Tessék." {
		t.Errorf("mini import: title=%q body=%q", title, body)
	}
	c.json(http.MethodPost, "/api/library/sets/m1-u1-talk/import", nil, &out)
	s.db.QueryRow(`SELECT body FROM text WHERE id = ?`, out.TextID).Scan(&body)
	if body != "Szia. Szia." {
		t.Errorf("voicemsg is prose: %q", body)
	}
}
