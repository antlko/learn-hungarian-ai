package morph

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"tanulom/internal/lang"
)

// newTestPack builds a miniature language pack on disk.
//
// The real pack is ~260 MB and is a build artefact, so tests cannot depend on
// it. This fixture carries just enough to exercise every branch of the lookup
// ladder: an unambiguous form, an ambiguous one, a generated form, and a form
// that only the suffix fallback can reach.
func newTestPack(t *testing.T) *lang.Pack {
	t.Helper()
	path := filepath.Join(t.TempDir(), "langpack-hu-v1.sqlite")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	schema := `
	CREATE TABLE pack_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
	CREATE TABLE lemma (
		id INTEGER PRIMARY KEY, lemma TEXT NOT NULL, upos TEXT NOT NULL,
		freq_rank INTEGER, freq_pm REAL, cefr TEXT, ipa TEXT);
	CREATE TABLE lemma_gloss (
		lemma_id INTEGER, lang TEXT, sense_idx INTEGER, gloss TEXT);
	CREATE TABLE wordform (
		form TEXT NOT NULL, lemma_id INTEGER NOT NULL, upos TEXT NOT NULL,
		feats TEXT NOT NULL DEFAULT '', source TEXT NOT NULL, confidence TEXT NOT NULL);
	CREATE INDEX idx_wordform_form ON wordform(form);
	CREATE TABLE paradigm (lemma_id INTEGER, slot TEXT, form TEXT);
	CREATE TABLE morph_feature (
		id TEXT PRIMARY KEY, key TEXT, value TEXT, label_ru TEXT, label_en TEXT,
		allomorphs TEXT, explain_ru TEXT, explain_en TEXT, teach_order INTEGER,
		prereq_ids TEXT, grid TEXT);
	CREATE TABLE sentence (id INTEGER PRIMARY KEY, hu TEXT, source TEXT, license TEXT,
		len_tokens INTEGER, difficulty REAL);
	CREATE TABLE grammar_topic (id INTEGER PRIMARY KEY, code TEXT, level TEXT,
		title_ru TEXT, title_en TEXT, explain_md_ru TEXT, explain_md_en TEXT,
		prereq_codes TEXT, feature_ids TEXT, teach_order INTEGER);
	CREATE TABLE minimal_pair (id INTEGER PRIMARY KEY, a TEXT, b TEXT, contrast TEXT,
		priority INTEGER, gloss_a_ru TEXT, gloss_b_ru TEXT, gloss_a_en TEXT,
		gloss_b_en TEXT, context_a TEXT, context_b TEXT);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	exec(`INSERT INTO pack_meta VALUES ('version','1'),('schema_version','1'),('gloss_langs','["en"]')`)

	// `vár` is the classic Hungarian ambiguity: castle (noun) or waits (verb).
	exec(`INSERT INTO lemma (id,lemma,upos,freq_rank,ipa) VALUES
		(1,'ház','NOUN',300,'[ˈhaːz]'),
		(2,'kert','NOUN',900,'[ˈkɛrt]'),
		(3,'vár','NOUN',2500,'[ˈvaːr]'),
		(4,'vár','VERB',400,'[ˈvaːr]'),
		(5,'gyümölcs','NOUN',1800,NULL)`)
	exec(`INSERT INTO lemma_gloss VALUES (1,'en',0,'house'),(2,'en',0,'garden')`)

	exec(`INSERT INTO wordform (form,lemma_id,upos,feats,source,confidence) VALUES
		('ház',1,'NOUN','','wiktionary','high'),
		('házban',1,'NOUN','Case=Ine|Number=Sing','wiktionary','high'),
		('házat',1,'NOUN','Case=Acc|Number=Sing','wiktionary','high'),
		('házaimban',1,'NOUN','Case=Ine|Number[psed]=Plur|Person[psor]=1','generated','medium'),
		('kert',2,'NOUN','','wiktionary','high'),
		('kertben',2,'NOUN','Case=Ine|Number=Sing','wiktionary','high'),
		('vár',3,'NOUN','','wiktionary','high'),
		('vár',4,'VERB','Person=3|Number=Sing','wiktionary','high'),
		('gyümölcs',5,'NOUN','','wiktionary','high')`)

	exec(`INSERT INTO paradigm VALUES (1,'Nom.Sing','ház'),(1,'Ine.Sing','házban')`)

	exec(`INSERT INTO morph_feature (id,key,value,label_ru,label_en,allomorphs,
		explain_ru,explain_en,teach_order,prereq_ids,grid) VALUES
		('Case=Ine','Case','Ine','инессив','inessive',
		 '{"back":"ban","front_unrounded":"ben","front_rounded":"ben"}','x','x',30,'','interior/where'),
		('Case=All','Case','All','аллатив','allative',
		 '{"back":"hoz","front_unrounded":"hez","front_rounded":"höz"}','x','x',37,'','proximity/whither'),
		('Number=Plur','Number','Plur','мн. число','plural',
		 '{"back":"k","front_unrounded":"k","front_rounded":"k"}','x','x',25,'',NULL)`)

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	p, err := lang.Open(path)
	if err != nil {
		t.Fatalf("open test pack: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}
