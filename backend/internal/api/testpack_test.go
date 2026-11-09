package api

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// A miniature language pack written into a data directory before the server
// opens it, so the pack-dependent endpoints can be tested end to end.
//
// Small on purpose: the real pack is ~260 MB and is a build artefact. This one
// carries a four-topic curriculum, the locative grid and one verb whose two
// conjugations differ — which is everything the grammar screen needs.
func writeTestPack(t testing.TB, dataDir string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "langpack-hu-v1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE pack_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
		CREATE TABLE lemma (id INTEGER PRIMARY KEY, lemma TEXT, upos TEXT,
			freq_rank INTEGER, freq_pm REAL, cefr TEXT, ipa TEXT);
		CREATE TABLE lemma_gloss (lemma_id INTEGER, lang TEXT, sense_idx INTEGER, gloss TEXT);
		CREATE TABLE wordform (form TEXT NOT NULL, lemma_id INTEGER NOT NULL, upos TEXT,
			feats TEXT NOT NULL DEFAULT '', source TEXT NOT NULL, confidence TEXT NOT NULL);
		CREATE INDEX idx_wordform_form ON wordform(form);
		CREATE INDEX idx_wordform_lemma ON wordform(lemma_id);
		CREATE TABLE paradigm (lemma_id INTEGER, slot TEXT, form TEXT);
		CREATE TABLE morph_feature (id TEXT PRIMARY KEY, key TEXT, value TEXT,
			label_ru TEXT, label_en TEXT, allomorphs TEXT, explain_ru TEXT,
			explain_en TEXT, teach_order INTEGER, prereq_ids TEXT, grid TEXT);
		CREATE TABLE sentence (id INTEGER PRIMARY KEY, hu TEXT, source TEXT,
			license TEXT, len_tokens INTEGER, difficulty REAL);
		CREATE TABLE sentence_trans (sentence_id INTEGER, lang TEXT, text TEXT);
		CREATE TABLE sentence_token (sentence_id INTEGER, idx INTEGER, form TEXT,
			lemma_id INTEGER, feats TEXT DEFAULT '');
		CREATE INDEX idx_stoken_lemma ON sentence_token(lemma_id);
		CREATE TABLE grammar_topic (id INTEGER PRIMARY KEY, code TEXT, level TEXT,
			title_ru TEXT, title_en TEXT, explain_md_ru TEXT, explain_md_en TEXT,
			prereq_codes TEXT, feature_ids TEXT, teach_order INTEGER);
		CREATE TABLE minimal_pair (id INTEGER PRIMARY KEY, a TEXT, b TEXT, contrast TEXT,
			priority INTEGER, gloss_a_ru TEXT, gloss_b_ru TEXT, gloss_a_en TEXT,
			gloss_b_en TEXT, context_a TEXT, context_b TEXT);
		CREATE TABLE pair_audio (pair_id INTEGER, member TEXT, talker_id TEXT,
			talker_kind TEXT, rate REAL, path TEXT, license TEXT);
		CREATE TABLE ngram (gram TEXT, n INTEGER, count INTEGER);`); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	for k, v := range map[string]string{
		"version": "hu-v1", "schema_version": "1", "gloss_langs": `["en","ru"]`,
	} {
		exec(`INSERT INTO pack_meta VALUES (?,?)`, k, v)
	}

	const none = `{"back":"","front_unrounded":"","front_rounded":""}`
	feature := func(id, key, val, allo string, order int, grid string) {
		exec(`INSERT INTO morph_feature VALUES (?,?,?,?,?,?,?,?,?,'',?)`,
			id, key, val, "ru "+id, "en "+id, allo, "объяснение", "explanation", order, grid)
	}
	feature("Skill=VowelHarmony", "Skill", "VowelHarmony", none, 1, "")
	feature("Case=Nom", "Case", "Nom", none, 10, "")
	feature("Case=Acc", "Case", "Acc", `{"back":"t","front_unrounded":"t","front_rounded":"t"}`, 20, "")
	feature("Case=Ine", "Case", "Ine", `{"back":"ban","front_unrounded":"ben","front_rounded":"ben"}`, 30, "interior/where")
	feature("Case=Ill", "Case", "Ill", `{"back":"ba","front_unrounded":"be","front_rounded":"be"}`, 31, "interior/whither")
	feature("Case=Ela", "Case", "Ela", `{"back":"ból","front_unrounded":"ből","front_rounded":"ből"}`, 32, "interior/whence")
	feature("Definite=Ind", "Definite", "Ind", none, 80, "")
	feature("Definite=Def", "Definite", "Def", none, 81, "")
	feature("Skill=PreverbDetach", "Skill", "PreverbDetach", none, 150, "")

	lemma := func(id int64, l, upos string, rank int) {
		exec(`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (?,?,?,?)`, id, l, upos, rank)
		exec(`INSERT INTO lemma_gloss VALUES (?,?,?,?)`, id, "en", 0, l+" (en)")
		exec(`INSERT INTO lemma_gloss VALUES (?,?,?,?)`, id, "ru", 0, l+" (ru)")
	}
	form := func(f string, id int64, upos, feats string) {
		exec(`INSERT INTO wordform VALUES (?,?,?,?,'wiktionary','high')`, f, id, upos, feats)
	}
	lemma(1, "ház", "NOUN", 100)
	for f, feats := range map[string]string{
		"ház": "Case=Nom|Number=Sing", "házat": "Case=Acc|Number=Sing",
		"házban": "Case=Ine|Number=Sing", "házba": "Case=Ill|Number=Sing",
		"házból": "Case=Ela|Number=Sing",
	} {
		form(f, 1, "NOUN", feats)
	}
	lemma(2, "könyv", "NOUN", 200)
	for f, feats := range map[string]string{
		"könyv": "Case=Nom|Number=Sing", "könyvet": "Case=Acc|Number=Sing",
		"könyvben": "Case=Ine|Number=Sing", "könyvbe": "Case=Ill|Number=Sing",
		"könyvből": "Case=Ela|Number=Sing",
	} {
		form(f, 2, "NOUN", feats)
	}
	lemma(3, "lát", "VERB", 50)
	form("látom", 3, "VERB", "Definite=Def|Mood=Ind|Number=Sing|Person=1|Tense=Pres")
	form("látok", 3, "VERB", "Definite=Ind|Mood=Ind|Number=Sing|Person=1|Tense=Pres")
	form("látja", 3, "VERB", "Definite=Def|Mood=Ind|Number=Sing|Person=3|Tense=Pres")
	form("lát", 3, "VERB", "Definite=Ind|Mood=Ind|Number=Sing|Person=3|Tense=Pres")
	lemma(4, "az", "DET", 3)
	form("a", 4, "DET", "")
	form("az", 4, "DET", "")
	lemma(5, "az", "PRON", 4)
	form("azt", 5, "PRON", "Case=Acc")
	lemma(6, "egy", "DET", 5)
	form("egy", 6, "DET", "")

	sentence := func(id int64, hu string, tokens map[int]int64) {
		exec(`INSERT INTO sentence VALUES (?,?,'test','CC0',?,1.0)`, id, hu, len(tokens))
		for idx, lid := range tokens {
			exec(`INSERT INTO sentence_token VALUES (?,?,'',?,'')`, id, idx, lid)
		}
	}
	sentence(101, "Látom a házat az utcán.", map[int]int64{0: 3, 1: 4, 2: 1})
	sentence(102, "Látok egy házat az utcán.", map[int]int64{0: 3, 1: 6, 2: 1})
	sentence(103, "Azt a könyvet látja a fiú.", map[int]int64{0: 5, 1: 4, 2: 2, 3: 3})
	sentence(104, "A házban van egy nagy könyv.", map[int]int64{0: 4, 1: 1, 3: 6, 5: 2})

	topic := func(id int64, code, level, prereqs, feats string, order int) {
		exec(`INSERT INTO grammar_topic VALUES (?,?,?,?,?,?,?,?,?,?)`,
			id, code, level, "ru "+code, "en "+code,
			"# Правило\n\n"+code, "# Rule\n\n"+code, prereqs, feats, order)
	}
	topic(1, "orthography", "A1", "", "", 10)
	topic(2, "accusative", "A1", "orthography", "Case=Acc", 30)
	topic(3, "locative-grid", "A1", "accusative", "Case=Ine,Case=Ill,Case=Ela", 40)
	topic(4, "definite-conjugation", "A2", "locative-grid", "Definite=Def,Definite=Ind", 60)
	topic(5, "focus-word-order", "B2", "definite-conjugation", "Skill=PreverbDetach", 70)
}

// newPackedServer returns a signed-in admin on a server that has a pack.
func newPackedServer(t *testing.T) (*Server, *client) {
	t.Helper()
	dir := t.TempDir()
	writeTestPack(t, dir)

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
