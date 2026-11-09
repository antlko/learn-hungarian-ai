package api

import (
	"testing"
	"time"

	"tanulom/internal/srs"
)

// BenchmarkRecordInteraction measures the cost recordInteraction adds to every
// answer: ActionableDue, a full-deck Breakdown. Perf note only (polish plan,
// decision 11) — behaviour is deliberately unchanged, and it is never
// throttled, because due_left must stay fresh. Run:
//
//	go test ./internal/api/ -run '^$' -bench RecordInteraction -benchtime 20x
func BenchmarkRecordInteraction(b *testing.B) {
	dir := b.TempDir()
	writeTestPack(b, dir) // the pack is irrelevant to the cost; New wants one
	s, err := New(dir)
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	if _, err := s.db.Exec(`INSERT INTO user (email, password_hash, role) VALUES ('b@x', 'x', 'admin')`); err != nil {
		b.Skipf("cannot seed a user: %v", err)
	}
	var uid int64
	_ = s.db.QueryRow(`SELECT id FROM user LIMIT 1`).Scan(&uid)

	// ~5000 cards: 2500 notes x 2 directions, a mix of due, later and new.
	tx, _ := s.db.Begin()
	now := time.Now().UTC()
	for i := 0; i < 2500; i++ {
		res, err := tx.Exec(`INSERT INTO note (user_id, kind, lemma_id, hu, target_form)
			VALUES (?, 'vocab', ?, 'szó', 'szó')`, uid, 1000+i)
		if err != nil {
			b.Fatal(err)
		}
		nid, _ := res.LastInsertId()
		for j, dir := range []string{"recognize", "produce"} {
			due := now.Add(time.Duration((i%40)-20) * 24 * time.Hour).Format("2006-01-02T15:04:05.000Z")
			state := 2
			if i%10 == 0 {
				state = 0
			}
			if _, err := tx.Exec(`INSERT INTO card (note_id, direction, due, stability, difficulty,
				scheduled_days, reps, state, last_review) VALUES (?,?,?,?,?,?,?,?,?)`,
				nid, dir, due, 10+float64(j), 5, 10, 3, state, due); err != nil {
				b.Fatal(err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}

	store := srs.NewStore(s.db.DB)
	bury := s.Config().Review.BurySiblingsEnabled()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.ActionableDue(uid, bury, time.Now()); err != nil {
			b.Fatal(err)
		}
	}
}
