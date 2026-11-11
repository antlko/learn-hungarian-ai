package drill

import (
	"database/sql"
	"math/rand"
	"testing"
)

// A drill's sentence is a real corpus sentence. The AI-written dialogue lines
// (`source = 'authored'`, ADR-066) must never be sampled — even when they carry
// token rows and sit above every corpus id, where the random-floor lookup would
// otherwise land on them almost every time.
func TestSampledSentenceIsNeverAnAuthoredDialogueLine(t *testing.T) {
	g, _ := newTestGenerator(t)

	var path string
	{
		var seq int
		var name string
		if err := g.pack.DB().QueryRow(`SELECT seq, name, file FROM pragma_database_list WHERE name = 'main'`).
			Scan(&seq, &name, &path); err != nil {
			t.Fatal(err)
		}
	}
	w, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO sentence VALUES (9000001,'Látom a házat.','authored','CC BY-SA 4.0',3,NULL)`,
		`INSERT INTO sentence_token VALUES (9000001,0,'Látom',3,''),(9000001,1,'a',4,''),(9000001,2,'házat',1,'')`,
		`INSERT INTO sentence VALUES (9000002,'Ez a ház nagy.','authored','CC BY-SA 4.0',3,NULL)`,
		`INSERT INTO sentence_token VALUES (9000002,0,'Ez',4,''),(9000002,1,'a',4,''),(9000002,2,'ház',1,'')`,
	} {
		if _, err := w.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	w.Close()

	rnd := rand.New(rand.NewSource(7))
	got := 0
	for range 400 {
		id, hu, ok := g.sampleSentenceContaining([]int64{1}, rnd) // lemma 1: ház
		if !ok {
			continue
		}
		got++
		if id >= 9000000 {
			t.Fatalf("a drill sampled the authored sentence %d (%q)", id, hu)
		}
	}
	if got == 0 {
		t.Fatal("no sentence was sampled at all: the check is vacuous")
	}
}
