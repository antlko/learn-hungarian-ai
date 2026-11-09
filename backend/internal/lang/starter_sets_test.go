package lang

import (
	"testing"
)

// The starter carries ready-made reading sets (ADR-056): sentences that already
// exist in it, for every course unit. A set that names a sentence the starter
// does not hold, or a Tatoeba-less one, would import as an empty text.
func TestStarterCarriesReadingSetsWithinBounds(t *testing.T) {
	dir := t.TempDir()
	version, _, err := EnsurePack(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Open(Path(dir, version))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	db := p.DB()

	var sets int
	if err := db.QueryRow(`SELECT COUNT(*) FROM reading_set`).Scan(&sets); err != nil {
		t.Fatalf("the starter has no reading_set table: %v", err)
	}
	if sets != 36 {
		t.Errorf("%d sets, want 36 (24 core units + 12 themes)", sets)
	}

	rows, err := db.Query(`
		SELECT r.code, r.level, COUNT(x.sentence_id)
		  FROM reading_set r LEFT JOIN reading_set_sentence x ON x.set_code = r.code
		 GROUP BY r.code`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var code, level string
		var n int
		if err := rows.Scan(&code, &level, &n); err != nil {
			t.Fatal(err)
		}
		if n < 20 || n > 30 {
			t.Errorf("set %s has %d sentences, want 20-30", code, n)
		}
		switch level {
		case "A1", "A2", "B1":
		default:
			t.Errorf("set %s has level %q", code, level)
		}
	}

	var bad int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM reading_set_sentence x
		  LEFT JOIN sentence s ON s.id = x.sentence_id
		 WHERE s.id IS NULL OR s.source <> 'tatoeba'`).Scan(&bad); err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Errorf("%d set sentences are missing or not Tatoeba: the starter takes nothing else", bad)
	}

	var dup int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM (SELECT sentence_id FROM reading_set_sentence
		                       GROUP BY sentence_id HAVING COUNT(*) > 1)`).Scan(&dup); err != nil {
		t.Fatal(err)
	}
	if dup != 0 {
		t.Errorf("%d sentences appear in more than one set", dup)
	}
}
