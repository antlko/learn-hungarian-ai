package lang

import (
	"testing"
)

// The starter carries ready-made reading sets (ADR-056, ADR-066) for every
// course unit: «Фразы» sets of Tatoeba sentences that already exist in it, and
// one written «Диалог» per unit (AI-written, `source = 'authored'`). A set that
// names a sentence the starter does not hold, or the wrong kind of one, would
// import as an empty or mislabelled text.
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
	if sets != 72 {
		t.Errorf("%d sets, want 72 (24 core units + 12 themes, and a dialogue for each)", sets)
	}

	rows, err := db.Query(`
		SELECT r.code, r.level, r.track, COUNT(x.sentence_id)
		  FROM reading_set r LEFT JOIN reading_set_sentence x ON x.set_code = r.code
		 GROUP BY r.code`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var code, level, track string
		var n int
		if err := rows.Scan(&code, &level, &track, &n); err != nil {
			t.Fatal(err)
		}
		lo, hi := 20, 30 // «Фразы»
		if track == "dialogue" {
			lo, hi = 8, 14
		}
		if n < lo || n > hi {
			t.Errorf("set %s has %d sentences, want %d-%d", code, n, lo, hi)
		}
		switch level {
		case "A1", "A2", "B1":
		default:
			t.Errorf("set %s has level %q", code, level)
		}
	}

	// «Фразы» are Tatoeba only; a dialogue is authored only. Never mixed.
	var bad int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM reading_set_sentence x
		  JOIN reading_set r ON r.code = x.set_code
		  LEFT JOIN sentence s ON s.id = x.sentence_id
		 WHERE s.id IS NULL
		    OR (r.track <> 'dialogue' AND s.source <> 'tatoeba')
		    OR (r.track = 'dialogue' AND s.source <> 'authored')`).Scan(&bad); err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Errorf("%d set sentences are missing or of the wrong provenance (Фразы: tatoeba, Диалог: authored)", bad)
	}

	// Authored lines exist only as dialogue lines and carry no tokens, so no
	// token-reading query can reach them.
	var stray, tokens int
	db.QueryRow(`SELECT COUNT(*) FROM sentence WHERE source = 'authored' AND id NOT IN
		(SELECT sentence_id FROM reading_set_sentence)`).Scan(&stray)
	db.QueryRow(`SELECT COUNT(*) FROM sentence_token WHERE sentence_id IN
		(SELECT id FROM sentence WHERE source = 'authored')`).Scan(&tokens)
	if stray != 0 || tokens != 0 {
		t.Errorf("authored sentences outside a dialogue: %d, with token rows: %d", stray, tokens)
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
