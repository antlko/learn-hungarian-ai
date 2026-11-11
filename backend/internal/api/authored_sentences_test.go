package api

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// The AI-written dialogue lines (`sentence.source = 'authored'`, ADR-066) are
// reading material only. A drill, an example, a course note or the practice
// pool must never be built on one: a card's sentence is a real, attested corpus
// sentence. This test is ADVERSARIAL: the authored lines here are the SHORTEST
// sentences that carry the word, and they carry token rows (the build writes
// none, but nothing but this filter should be keeping them out), so any query
// that forgot `source <> 'authored'` would return them first.

const authoredFirstID = 9000001

func newServerWithAuthoredTokens(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	writeTestPack(t, dir)
	db, err := sql.Open("sqlite", filepath.Join(dir, "langpack-hu-v1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		// `ház` (lemma 1), three tokens: shorter than every real sentence.
		`INSERT INTO sentence VALUES (9000001,'Látom a házat.','authored','CC BY-SA 4.0',3,NULL)`,
		`INSERT INTO sentence_token VALUES (9000001,0,'Látom',3,''),(9000001,1,'a',4,''),(9000001,2,'házat',1,'')`,
		`INSERT INTO sentence VALUES (9000002,'Ez a ház nagy.','authored','CC BY-SA 4.0',3,NULL)`,
		`INSERT INTO sentence_token VALUES (9000002,0,'Ez',4,''),(9000002,1,'a',4,''),(9000002,2,'ház',1,'')`,
		`INSERT INTO sentence_trans VALUES (9000001,'ru','Я вижу дом.')`,
	} {
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
	return s
}

func TestExampleSentencesNeverIncludeAuthoredLines(t *testing.T) {
	t.Parallel()
	s := newServerWithAuthoredTokens(t)

	examples, err := s.examplesFor(1, maxExamples) // lemma 1: ház
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) == 0 {
		t.Fatal("the test pack has real sentences with ház: the check would be vacuous")
	}
	for _, ex := range examples {
		if strings.Contains(ex.HU, "Látom a házat.") || ex.HU == "Ez a ház nagy." || ex.RU == "Я вижу дом." {
			t.Errorf("an authored dialogue line came back as an example: %+v", ex)
		}
	}
}

func TestCourseSentenceSearchesNeverReturnAuthoredLines(t *testing.T) {
	t.Parallel()
	s := newServerWithAuthoredTokens(t)

	// Not vacuous: without the filter the authored lines WOULD be found, by id
	// and by surface form (the test pack's real sentences carry no forms).
	var byID, byForm int
	s.pack.DB().QueryRow(`SELECT COUNT(*) FROM sentence_token WHERE lemma_id = 1 AND sentence_id >= ?`,
		authoredFirstID).Scan(&byID)
	s.pack.DB().QueryRow(`SELECT COUNT(*) FROM sentence_token WHERE form IN ('ház','házat')`).Scan(&byForm)
	if byID == 0 || byForm == 0 {
		t.Fatalf("the authored lines are not reachable by token (%d, %d): vacuous", byID, byForm)
	}

	for name, find := range map[string]func(int64) (*sql.Rows, error){
		"by lemma id":     s.sentencesByLemmaID,
		"by surface form": s.sentencesBySurfaceForm,
		"example by id":   s.exampleRowsByLemmaID,
		"example by form": s.exampleRowsBySurfaceForm,
	} {
		rows, err := find(1)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if rows == nil {
			continue
		}
		for rows.Next() {
			var id int64
			var hu string
			if err := rows.Scan(&id, &hu); err != nil {
				t.Fatal(err)
			}
			if id >= authoredFirstID {
				t.Errorf("%s returned the authored sentence %d (%q)", name, id, hu)
			}
		}
		rows.Close()
	}

	// The lemma-id searches do find the real sentences (so they are exercised).
	rows, err := s.sentencesByLemmaID(1)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for rows.Next() {
		n++
	}
	rows.Close()
	if n == 0 {
		t.Error("the lemma search found no real sentence either: vacuous")
	}
}
