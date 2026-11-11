package srs

import (
	"database/sql"
	"fmt"
	"time"

	"tanulom/internal/db"
)

// The trash: words the learner set aside.
//
// «Слишком сложно» and «карточка плохая» are legitimate verdicts on an item,
// and before this the only ways to act on them were to fail the card forever
// or to let it sit at the front of every session. A learner who cannot say no
// to a card ends up saying no to the app.
//
// Three rules hold it together, and they are the whole design:
//
//   - **It is a NOTE-level fact.** The learner is setting aside a WORD. Hiding
//     only the direction on screen would hand the same word back through a
//     different door ten minutes later.
//   - **It is not deleting.** The note, its cards, their FSRS state and every
//     row in review_log survive untouched — which is what makes restore a real
//     restore rather than a re-add, and what keeps undo (delete-and-replay over
//     that log) working across a trashing.
//   - **It leaves the scheduler, not the app.** Free practice can still draw
//     from it (ADR-064: a trashed word is never reviewed by practice), so a set-aside word is
//     still a word you may look at. Nothing about it moves until it is restored.

// TrashedNote is one row of the trash screen.
type TrashedNote struct {
	NoteID     int64  `json:"noteId"`
	HU         string `json:"hu"`
	TargetForm string `json:"targetForm"`
	GlossRU    string `json:"glossRu"`
	GlossEN    string `json:"glossEn"`
	LemmaID    *int64 `json:"lemmaId,omitempty"`
	// Lemma is the dictionary form, filled by the API layer from the pack.
	Lemma string `json:"lemma,omitempty"`
	// Source is the note's origin string (`course:<code>` or `manual`), kept so
	// the screen can say WHERE the word came from. Never printed raw — a unit
	// code is not a name a learner has any reason to read.
	Source    string `json:"source"`
	TrashedAt string `json:"trashedAt"`
	// Cards is how many directions come back with a restore, so the button can
	// say what it will do to the day's workload.
	Cards int `json:"cards"`
}

// Trash sets a note aside. The bool is false when the note was already there —
// an idempotent second click is not an error, and reporting one would make a
// double tap look like a failure.
func (s *Store) Trash(userID, noteID int64, now time.Time) (bool, error) {
	res, err := s.db.Exec(`
		UPDATE note SET trashed_at = ?
		 WHERE id = ? AND user_id = ? AND trashed_at IS NULL`,
		db.FmtTime(now), noteID, userID)
	if err != nil {
		return false, fmt.Errorf("trash note: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// TrashWord is «Знаю»: it sets aside EVERY live note of the lemma the given note
// belongs to, in one transaction and with one `trashed_at` instant, and marks
// the lemma `known`. ADR-071 (amends ADR-041): the claim is about the word, and
// a lemma owns one note per sentence, text and course unit — trashing only the
// tapped note handed the same word back through the next one.
//
// The link is `note.lemma_id` (a pack lemma id). A note without one cannot be
// grouped and is trashed alone, which is the pre-existing behaviour.
//
// It returns how many notes moved. Zero means there was nothing live to move —
// the idempotent second tap.
func (s *Store) TrashWord(userID, noteID int64, now time.Time) (notes, cards int, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("trash word: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var lemma sql.NullInt64
	if err := tx.QueryRow(`SELECT lemma_id FROM note WHERE id = ? AND user_id = ?`,
		noteID, userID).Scan(&lemma); err != nil {
		return 0, 0, fmt.Errorf("trash word: %w", err)
	}
	at := db.FmtTime(now)
	res, err := tx.Exec(`
		UPDATE note SET trashed_at = ?
		 WHERE user_id = ? AND trashed_at IS NULL
		   AND (id = ? OR (? AND lemma_id = ?))`,
		at, userID, noteID, lemma.Valid, lemma.Int64)
	if err != nil {
		return 0, 0, fmt.Errorf("trash word: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, 0, err
	}
	if lemma.Valid && n > 0 {
		if err := setLemmaStatusTx(tx, userID, lemma.Int64, "known"); err != nil {
			return 0, 0, err
		}
	}
	cards, err = wordCards(tx, userID, noteID, lemma, at)
	if err != nil {
		return 0, 0, err
	}
	return int(n), cards, tx.Commit()
}

// RestoreWord is the way back from TrashWord: the note plus every note of its
// lemma that was set aside at the SAME instant, and the lemma back to
// `learning`. Notes trashed at another time (a separate, deliberate act) stay
// where they are.
func (s *Store) RestoreWord(userID, noteID int64) (notes, cards int, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("restore word: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var lemma sql.NullInt64
	var at sql.NullString
	if err := tx.QueryRow(`SELECT lemma_id, trashed_at FROM note WHERE id = ? AND user_id = ?`,
		noteID, userID).Scan(&lemma, &at); err != nil {
		return 0, 0, fmt.Errorf("restore word: %w", err)
	}
	if !at.Valid {
		c, err := wordCards(tx, userID, noteID, lemma, "")
		return 0, c, err
	}
	cards, err = wordCards(tx, userID, noteID, lemma, at.String)
	if err != nil {
		return 0, 0, err
	}
	res, err := tx.Exec(`
		UPDATE note SET trashed_at = NULL
		 WHERE user_id = ? AND trashed_at IS NOT NULL
		   AND (id = ? OR (? AND lemma_id = ? AND trashed_at = ?))`,
		userID, noteID, lemma.Valid, lemma.Int64, at.String)
	if err != nil {
		return 0, 0, fmt.Errorf("restore word: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, 0, err
	}
	if lemma.Valid && n > 0 {
		if err := setLemmaStatusTx(tx, userID, lemma.Int64, "learning"); err != nil {
			return 0, 0, err
		}
	}
	return int(n), cards, tx.Commit()
}

func setLemmaStatusTx(tx *sql.Tx, userID, lemmaID int64, status string) error {
	_, err := tx.Exec(`
		INSERT INTO lemma_state (user_id, lemma_id, status) VALUES (?,?,?)
		ON CONFLICT(user_id, lemma_id) DO UPDATE SET status = excluded.status`,
		userID, lemmaID, status)
	if err != nil {
		return fmt.Errorf("set lemma status: %w", err)
	}
	return nil
}

// wordCards counts the directions of the note plus every note of its lemma
// that carries the given trashed_at instant — what moved in one TrashWord or
// RestoreWord, so the response quotes what the learner actually gained or lost.
func wordCards(tx *sql.Tx, userID, noteID int64, lemma sql.NullInt64, at string) (int, error) {
	var n int
	err := tx.QueryRow(`
		SELECT COUNT(*) FROM card WHERE note_id IN (
		  SELECT id FROM note WHERE user_id = ?
		   AND (id = ? OR (? AND lemma_id = ? AND trashed_at = ?)))`,
		userID, noteID, lemma.Valid, lemma.Int64, at).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count word cards: %w", err)
	}
	return n, nil
}

// CardCount is how many directions a note owns — what moves with it in either
// direction, and the number the button quotes.
func (s *Store) CardCount(noteID int64) (int, error) {
	var n int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM card WHERE note_id = ?`, noteID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count note cards: %w", err)
	}
	return n, nil
}

// Restore puts a note back in the deck.
//
// Nothing is rescheduled: the cards come back with exactly the FSRS state they
// left with. A word set aside for a fortnight returns overdue, which is the
// truth about it — resetting the state to "new" would be the app inventing a
// memory the learner does not have.
func (s *Store) Restore(userID, noteID int64) (bool, error) {
	res, err := s.db.Exec(`
		UPDATE note SET trashed_at = NULL
		 WHERE id = ? AND user_id = ? AND trashed_at IS NOT NULL`, noteID, userID)
	if err != nil {
		return false, fmt.Errorf("restore note: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// NoteExists reports whether the id names one of this user's notes, so trashing
// a foreign or stale id can 404 instead of silently doing nothing.
func (s *Store) NoteExists(userID, noteID int64) (bool, error) {
	var one int
	err := s.db.QueryRow(
		`SELECT 1 FROM note WHERE id = ? AND user_id = ?`, noteID, userID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check note: %w", err)
	}
	return true, nil
}

// wordKey groups the notes one «Знаю» tap set aside into one word: same lemma,
// same instant. A note with no lemma is its own word.
const wordKey = `CASE WHEN n.lemma_id IS NULL THEN 'n' || n.id
	ELSE 'l' || n.lemma_id || '@' || n.trashed_at END`

// TrashCount is how many words are set aside — the number the screen and the
// review card's confirmation both quote.
func (s *Store) TrashCount(userID int64) (int, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(DISTINCT `+wordKey+`) FROM note n
		  WHERE n.user_id = ? AND n.trashed_at IS NOT NULL`,
		userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count trash: %w", err)
	}
	return n, nil
}

// TrashedNotes lists the trash, most recently set aside first — the order a
// learner looks for a word they just threw away.
func (s *Store) TrashedNotes(userID int64, limit int) ([]TrashedNote, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.db.Query(`
		SELECT MIN(n.id), n.hu, n.target_form, n.gloss_ru, n.gloss_en, n.lemma_id,
		       n.source, n.trashed_at, COUNT(c.id)
		  FROM note n LEFT JOIN card c ON c.note_id = n.id
		 WHERE n.user_id = ? AND n.trashed_at IS NOT NULL
		 GROUP BY `+wordKey+`
		 ORDER BY n.trashed_at DESC, n.id DESC
		 LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list trash: %w", err)
	}
	defer rows.Close()

	var out []TrashedNote
	for rows.Next() {
		var t TrashedNote
		var at sql.NullString
		if err := rows.Scan(&t.NoteID, &t.HU, &t.TargetForm, &t.GlossRU, &t.GlossEN,
			&t.LemmaID, &t.Source, &at, &t.Cards); err != nil {
			return nil, fmt.Errorf("scan trash row: %w", err)
		}
		t.TrashedAt = at.String
		out = append(out, t)
	}
	return out, rows.Err()
}
