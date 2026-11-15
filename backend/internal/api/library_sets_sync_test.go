package api

import (
	"net/http"
	"testing"
)

// The sync that keeps imported sets current with the binary's catalogue
// (ADR-074, ADR-076). The test pack has no reading_set tables, like the starter
// and like any pack built after ADR-076: the catalogue alone drives the sync.

func TestSyncSetTextsReplacesStaleCopiesInPlaceAndIsIdempotent(t *testing.T) {
	t.Parallel()
	s, c := newSetsServer(t)
	ids := map[string]int64{}
	var uid int64
	for _, code := range []string{"m1-u1-dialogue", "m1-u1", "m1-u1-long"} {
		var out ImportSetResponse
		c.json(http.MethodPost, "/api/library/sets/"+code+"/import", nil, &out)
		ids[code] = out.TextID
		s.db.QueryRow(`SELECT user_id FROM text WHERE id = ?`, out.TextID).Scan(&uid)
		// Make the copy look like one imported from an older binary, as read,
		// with a word mined from it.
		if _, err := s.db.Exec(`UPDATE text SET title = 'Старое', body = 'Régi mondat.', kind = 'paste',
			read_at = '2026-01-01T00:00:00Z' WHERE id = ?`, out.TextID); err != nil {
			t.Fatal(err)
		}
		res, err := s.db.Exec(`INSERT INTO note (user_id, kind, hu) VALUES (?, 'sentence', ?)`, uid, "régi "+code)
		if err != nil {
			t.Fatal(err)
		}
		nid, _ := res.LastInsertId()
		if _, err := s.db.Exec(`INSERT INTO note_origin (note_id, text_id) VALUES (?, ?)`, nid, out.TextID); err != nil {
			t.Fatal(err)
		}
	}
	// A copy of a set the catalogue does not have must never be touched or deleted.
	res, err := s.db.Exec(`INSERT INTO text (user_id, title, source_url, kind, body)
		VALUES (?, 'Gone', 'set:gone', 'paste', 'Maradok.')`, uid)
	if err != nil {
		t.Fatal(err)
	}
	goneID, _ := res.LastInsertId()

	if r, sk, f := s.syncSetTexts(); r != 3 || sk != 1 || f != 0 {
		t.Fatalf("first run: replaced=%d skipped=%d failed=%d", r, sk, f)
	}
	want := map[string][2]string{
		"m1-u1-dialogue": {"Диалог: Привет", "– Szia! Hogy vagy?\n– Köszönöm, jól."},
		"m1-u1":          {"История: Привет", "Látom a házat. Az utcán van.\n\nKöszönöm, jól."},
		"m1-u1-long":     {"Длинный: Привет", "Látom a házat.\n\nKöszönöm, jól."},
	}
	for code, id := range ids {
		var gid int64
		var title, body, kind, readAt string
		var origins int
		s.db.QueryRow(`SELECT id, title, body, kind, read_at FROM text WHERE source_url = ?`, "set:"+code).
			Scan(&gid, &title, &body, &kind, &readAt)
		s.db.QueryRow(`SELECT COUNT(*) FROM note_origin WHERE text_id = ?`, id).Scan(&origins)
		if gid != id || title != want[code][0] || body != want[code][1] || kind != "generated" ||
			readAt != "2026-01-01T00:00:00Z" || origins != 1 {
			t.Errorf("%s: %d %q %q %q %q origins=%d", code, gid, title, body, kind, readAt, origins)
		}
		var sents int
		s.db.QueryRow(`SELECT COUNT(DISTINCT sent_idx) FROM text_token WHERE text_id = ?`, id).Scan(&sents)
		if sents < 2 {
			t.Errorf("%s: tokens not rebuilt (%d sentences)", code, sents)
		}
	}
	var goneBody string
	if err := s.db.QueryRow(`SELECT body FROM text WHERE id = ?`, goneID).Scan(&goneBody); err != nil || goneBody != "Maradok." {
		t.Errorf("unknown code was touched: %q %v", goneBody, err)
	}
	if r, sk, f := s.syncSetTexts(); r != 0 || sk != 4 || f != 0 {
		t.Errorf("second run must change nothing: replaced=%d skipped=%d failed=%d", r, sk, f)
	}
}
