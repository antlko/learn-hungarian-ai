package drill

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"tanulom/internal/curriculum"
	"tanulom/internal/lang"
	"tanulom/internal/morph"
)

// A miniature pack with enough real Hungarian to exercise every generator.
//
// The forms are the attested ones, including the two the generator would get
// wrong on its own — `házon` (not `házan`) and the 1sg past `láttam`, which is
// identical in both conjugations and must therefore never become an item.
func newTestGenerator(t *testing.T) (*Generator, *curriculum.DAG) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "langpack-hu-v1.sqlite")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
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
		CREATE TABLE sentence_token (sentence_id INTEGER, idx INTEGER, form TEXT,
			lemma_id INTEGER, feats TEXT DEFAULT '');
		CREATE INDEX idx_stoken_lemma ON sentence_token(lemma_id);
		CREATE TABLE grammar_topic (id INTEGER PRIMARY KEY, code TEXT, level TEXT,
			title_ru TEXT, title_en TEXT, explain_md_ru TEXT, explain_md_en TEXT,
			prereq_codes TEXT, feature_ids TEXT, teach_order INTEGER);
		CREATE TABLE minimal_pair (id INTEGER PRIMARY KEY, a TEXT, b TEXT, contrast TEXT,
			priority INTEGER, gloss_a_ru TEXT, gloss_b_ru TEXT, gloss_a_en TEXT,
			gloss_b_en TEXT, context_a TEXT, context_b TEXT);`); err != nil {
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

	// ── features ─────────────────────────────────────────────────────────
	feature := func(id, key, val, allo string, order int, grid string) {
		exec(`INSERT INTO morph_feature VALUES (?,?,?,?,?,?,?,?,?,'',?)`,
			id, key, val, "ru:"+id, "en:"+id, allo, "ru", "en", order, grid)
	}
	const none = `{"back":"","front_unrounded":"","front_rounded":""}`
	feature("Skill=VowelHarmony", "Skill", "VowelHarmony", none, 1, "")
	feature("Case=Nom", "Case", "Nom", none, 10, "")
	feature("Number=Sing", "Number", "Sing", none, 15, "")
	feature("Case=Acc", "Case", "Acc", `{"back":"t","front_unrounded":"t","front_rounded":"t"}`, 20, "")
	feature("Number=Plur", "Number", "Plur", `{"back":"k","front_unrounded":"k","front_rounded":"k"}`, 25, "")
	feature("Case=Ine", "Case", "Ine", `{"back":"ban","front_unrounded":"ben","front_rounded":"ben"}`, 30, "interior/where")
	feature("Case=Ill", "Case", "Ill", `{"back":"ba","front_unrounded":"be","front_rounded":"be"}`, 31, "interior/whither")
	feature("Case=Ela", "Case", "Ela", `{"back":"ból","front_unrounded":"ből","front_rounded":"ből"}`, 32, "interior/whence")
	feature("Case=Sup", "Case", "Sup", `{"back":"on","front_unrounded":"en","front_rounded":"ön"}`, 33, "surface/where")
	feature("Definite=Ind", "Definite", "Ind", none, 80, "")
	feature("Definite=Def", "Definite", "Def", none, 81, "")
	feature("Skill=Politeness", "Skill", "Politeness", none, 5, "")

	// ── lexicon ──────────────────────────────────────────────────────────
	lemma := func(id int64, l, upos string, rank int) {
		exec(`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (?,?,?,?)`, id, l, upos, rank)
		exec(`INSERT INTO lemma_gloss VALUES (?,?,?,?)`, id, "en", 0, "gloss of "+l)
		exec(`INSERT INTO lemma_gloss VALUES (?,?,?,?)`, id, "ru", 0, "перевод "+l)
	}
	form := func(f string, id int64, upos, feats, source string) {
		exec(`INSERT INTO wordform VALUES (?,?,?,?,?,?)`, f, id, upos, feats, source, "high")
	}

	lemma(1, "ház", "NOUN", 100)
	form("ház", 1, "NOUN", "Case=Nom|Number=Sing", "wiktionary")
	form("házak", 1, "NOUN", "Case=Nom|Number=Plur", "wiktionary")
	form("házat", 1, "NOUN", "Case=Acc|Number=Sing", "wiktionary")
	form("házban", 1, "NOUN", "Case=Ine|Number=Sing", "wiktionary")
	form("házba", 1, "NOUN", "Case=Ill|Number=Sing", "wiktionary")
	form("házból", 1, "NOUN", "Case=Ela|Number=Sing", "wiktionary")
	form("házon", 1, "NOUN", "Case=Sup|Number=Sing", "wiktionary")
	// The generator's own guess, which the paradigm table would otherwise show.
	form("házan", 1, "NOUN", "Case=Sup|Number=Sing", "generated")
	exec(`INSERT INTO paradigm VALUES (?,?,?)`, 1, "Sup.Sing", "házan")
	// A possessed inessive, which must never be offered as the plain one.
	form("házaimban", 1, "NOUN", "Case=Ine|Number=Sing|Number[psed]=Plur|Person[psor]=1", "wiktionary")

	lemma(2, "könyv", "NOUN", 200)
	form("könyv", 2, "NOUN", "Case=Nom|Number=Sing", "wiktionary")
	form("könyvben", 2, "NOUN", "Case=Ine|Number=Sing", "wiktionary")
	form("könyvbe", 2, "NOUN", "Case=Ill|Number=Sing", "wiktionary")
	form("könyvből", 2, "NOUN", "Case=Ela|Number=Sing", "wiktionary")
	form("könyvön", 2, "NOUN", "Case=Sup|Number=Sing", "wiktionary")
	form("könyvet", 2, "NOUN", "Case=Acc|Number=Sing", "wiktionary")
	form("könyvek", 2, "NOUN", "Case=Nom|Number=Plur", "wiktionary")

	lemma(3, "lát", "VERB", 50)
	form("látom", 3, "VERB", "Definite=Def|Mood=Ind|Number=Sing|Person=1|Tense=Pres", "wiktionary")
	form("látok", 3, "VERB", "Definite=Ind|Mood=Ind|Number=Sing|Person=1|Tense=Pres", "wiktionary")
	form("látja", 3, "VERB", "Definite=Def|Mood=Ind|Number=Sing|Person=3|Tense=Pres", "wiktionary")
	form("lát", 3, "VERB", "Definite=Ind|Mood=Ind|Number=Sing|Person=3|Tense=Pres", "wiktionary")
	// 1sg past is the SAME string in both conjugations. An item built from it
	// would offer the learner one answer twice.
	form("láttam", 3, "VERB", "Definite=Def|Mood=Ind|Number=Sing|Person=1|Tense=Past", "wiktionary")
	form("láttam", 3, "VERB", "Definite=Ind|Mood=Ind|Number=Sing|Person=1|Tense=Past", "wiktionary")

	lemma(4, "az", "DET", 3)
	form("az", 4, "DET", "", "wiktionary")
	form("a", 4, "DET", "", "wiktionary")
	lemma(5, "az", "PRON", 4)
	form("azt", 5, "PRON", "Case=Acc", "wiktionary")
	lemma(6, "egy", "DET", 5)
	form("egy", 6, "DET", "", "wiktionary")

	// ── sentences ────────────────────────────────────────────────────────
	sentence := func(id int64, hu string, tokens map[int]int64) {
		exec(`INSERT INTO sentence VALUES (?,?,?,?,?,?)`, id, hu, "test", "CC0", len(tokens), 1.0)
		for idx, lid := range tokens {
			exec(`INSERT INTO sentence_token VALUES (?,?,?,?,'')`, id, idx, "", lid)
		}
	}
	sentence(101, "Látom a házat az utcán.", map[int]int64{0: 3, 1: 4, 2: 1})
	sentence(102, "Látok egy házat az utcán.", map[int]int64{0: 3, 1: 6, 2: 1})
	sentence(103, "Azt a könyvet látja a fiú.", map[int]int64{0: 5, 1: 4, 2: 2, 3: 3})
	sentence(104, "A házban van egy nagy könyv.", map[int]int64{0: 4, 1: 1, 3: 6, 5: 2})
	sentence(105, "A könyvben sok szép kép van.", map[int]int64{0: 4, 1: 2})

	// ── curriculum ───────────────────────────────────────────────────────
	topic := func(id int64, code, level, prereqs, feats string, order int) {
		exec(`INSERT INTO grammar_topic VALUES (?,?,?,?,?,?,?,?,?,?)`,
			id, code, level, "ru:"+code, "en:"+code, "# ru", "# en", prereqs, feats, order)
	}
	topic(1, "orthography", "A1", "", "", 10)
	topic(2, "vowel-harmony", "A1", "orthography", "Skill=VowelHarmony", 20)
	topic(3, "politeness", "A1", "orthography", "Skill=Politeness", 25)
	topic(4, "accusative", "A1", "vowel-harmony", "Case=Acc", 30)
	topic(5, "locative-grid", "A1", "accusative", "Case=Ine,Case=Ill,Case=Ela,Case=Sup", 40)
	topic(6, "definite-conjugation", "A2", "locative-grid", "Definite=Def,Definite=Ind", 60)

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	p, err := lang.Open(path)
	if err != nil {
		t.Fatalf("open test pack: %v", err)
	}
	t.Cleanup(func() { p.Close() })

	dag, err := curriculum.Load(p)
	if err != nil {
		t.Fatalf("load curriculum: %v", err)
	}
	return NewGenerator(p, morph.NewAnalyzer(p), dag), dag
}
