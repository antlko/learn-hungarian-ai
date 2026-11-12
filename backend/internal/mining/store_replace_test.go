package mining

import (
	"path/filepath"
	"testing"
	"time"

	"tanulom/internal/db"
)

func TestReplaceKeepsHistoryAndRebuildsContent(t *testing.T) {
	t.Parallel()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	uid, err := d.CreateUser("a@example.com", "x", "admin")
	if err != nil {
		t.Fatal(err)
	}
	st := NewStore(d.DB)
	old := []StoredToken{{Index: 0, Form: "régi", Start: 0, End: 5, IsWord: true, Certainty: "exact"}}
	id, err := st.Create(uid, Ingested{Title: "old", Body: "régi", Kind: KindPaste}, "set:x", old, Coverage{Words: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.MarkRead(uid, id, time.Now()); err != nil {
		t.Fatal(err)
	}
	res, err := d.Exec(`INSERT INTO note (user_id, kind, hu) VALUES (?, 'sentence', 'régi')`, uid)
	if err != nil {
		t.Fatal(err)
	}
	nid, _ := res.LastInsertId()
	for _, q := range []string{
		`INSERT INTO note_origin (note_id, text_id) VALUES (?, ?)`,
		`INSERT INTO listen_state (user_id, text_id) VALUES (?, ?)`,
	} {
		args := []any{nid, id}
		if q[12] == 'l' {
			args = []any{uid, id}
		}
		if _, err := d.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if _, err := d.Exec(`UPDATE text SET audio_path = 'a.mp3' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	var added, read string
	d.QueryRow(`SELECT added_at, read_at FROM text WHERE id = ?`, id).Scan(&added, &read)

	nt := []StoredToken{
		{Index: 0, Form: "új", Start: 0, End: 3, IsWord: true, Certainty: "exact"},
		{Index: 1, Form: "szöveg", Start: 4, End: 11, IsWord: true, Certainty: "exact"},
	}
	if err := st.Replace(uid, id, "new", "új szöveg", "generated", nt, Coverage{Words: 2}, "A1", "{}"); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get(uid, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "new" || got.Body != "új szöveg" || got.Kind != "generated" ||
		got.WordCount != 2 || got.AudioPath != "" || got.Level != "A1" || got.SourceURL != "set:x" {
		t.Errorf("text not replaced: %+v", got)
	}
	var added2, read2 string
	d.QueryRow(`SELECT added_at, read_at FROM text WHERE id = ?`, id).Scan(&added2, &read2)
	if added2 != added || read2 != read || read == "" {
		t.Errorf("added/read changed: %q/%q -> %q/%q", added, read, added2, read2)
	}
	var origins, listens int
	d.QueryRow(`SELECT COUNT(*) FROM note_origin WHERE text_id = ?`, id).Scan(&origins)
	d.QueryRow(`SELECT COUNT(*) FROM listen_state WHERE text_id = ?`, id).Scan(&listens)
	if origins != 1 || listens != 0 {
		t.Errorf("origins=%d listens=%d, want 1/0", origins, listens)
	}
	toks, _ := st.Tokens(uid, id)
	if len(toks) != 2 || toks[0].Form != "új" {
		t.Errorf("tokens not rebuilt: %+v", toks)
	}
	if err := st.Replace(uid+99, id, "x", "x", "paste", nil, Coverage{}, "", ""); err != ErrNotFound {
		t.Errorf("other user: %v, want ErrNotFound", err)
	}
}
