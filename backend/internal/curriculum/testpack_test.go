package curriculum

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"tanulom/internal/lang"
)

// A miniature pack carrying only a curriculum. The real one is ~260 MB and is a
// build artefact, so tests cannot depend on it.
func newTestPack(t *testing.T, topics [][]any) *lang.Pack {
	t.Helper()
	path := filepath.Join(t.TempDir(), "langpack-hu-v1.sqlite")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE pack_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
		CREATE TABLE grammar_topic (
			id INTEGER PRIMARY KEY, code TEXT, level TEXT,
			title_ru TEXT, title_en TEXT, explain_md_ru TEXT, explain_md_en TEXT,
			prereq_codes TEXT, feature_ids TEXT, teach_order INTEGER);`); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{
		"version": "hu-v1", "schema_version": "1", "gloss_langs": `["en","ru"]`,
	} {
		if _, err := db.Exec(`INSERT INTO pack_meta VALUES (?,?)`, k, v); err != nil {
			t.Fatal(err)
		}
	}
	for i, row := range topics {
		code, prereqs, feats, order := row[0].(string), row[1].(string), row[2].(string), row[3].(int)
		if _, err := db.Exec(`
			INSERT INTO grammar_topic
			  (id, code, level, title_ru, title_en, explain_md_ru, explain_md_en,
			   prereq_codes, feature_ids, teach_order)
			VALUES (?,?,'A1',?,?,'ru','en',?,?,?)`,
			i+1, code, code, code, prereqs, feats, order); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	p, err := lang.Open(path)
	if err != nil {
		t.Fatalf("open test pack: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

// sample is the real curriculum's shape in miniature: a chain, a fork, a
// motor-skill topic and the listening-gated one.
func sample() [][]any {
	return [][]any{
		{"orthography", "", "", 10},
		{"vowel-harmony", "orthography", FeatureVowelHarmony, 20},
		{"accusative", "vowel-harmony", "Case=Acc", 30},
		{"locative-grid", "accusative", "Case=Ine,Case=Ill,Case=Ela", 40},
		{"past-tense", "accusative", "Tense=Past", 50},
		{"definite-conjugation", "past-tense", "Definite=Def,Definite=Ind", 60},
		{"focus-word-order", "definite-conjugation", FeaturePreverbDetach, 70},
	}
}
