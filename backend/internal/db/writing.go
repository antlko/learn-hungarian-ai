package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// «Письмо» (Writing, ADR-085): the learner's pieces and the checks on them,
// migration 00024. Every call is scoped by user: another learner's id is
// ErrNotFound, never a row.

// MaxChecksPerPiece bounds the history kept on one piece.
const MaxChecksPerPiece = 30

// ErrTooManyChecks is returned by AddWritingCheck at the per-piece cap.
var ErrTooManyChecks = errors.New("too many checks on one piece")

// WritingPiece is one essay.
type WritingPiece struct {
	ID        int64
	TopicCode string
	Title     string
	Level     string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WritingPieceRow is a list entry: the piece plus its check summary. Body is
// included so the caller can count words without a second query.
type WritingPieceRow struct {
	WritingPiece
	Checks    int
	LastCheck *WritingCheck // without Raw and CheckedText
}

// WritingCheck is one stored answer of a model (or of an external chat).
type WritingCheck struct {
	ID            int64
	PieceID       int64
	Source        string // "local" | "external"
	Model         string
	Lang          string
	PromptVersion int
	CheckedText   string
	Raw           string
	Interrupted   bool
	CreatedAt     time.Time
}

// CreateWritingPiece starts a piece and returns it.
func (d *DB) CreateWritingPiece(userID int64, topicCode, title, level string) (WritingPiece, error) {
	now := FmtTime(time.Now())
	var id int64
	err := d.QueryRow(`
		INSERT INTO writing_piece (user_id, topic_code, title, level, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		userID, topicCode, title, level, now, now).Scan(&id)
	if err != nil {
		return WritingPiece{}, fmt.Errorf("create writing piece: %w", err)
	}
	return d.WritingPiece(userID, id)
}

// WritingPiece reads one piece of the user's.
func (d *DB) WritingPiece(userID, id int64) (WritingPiece, error) {
	var p WritingPiece
	var created, updated sql.NullString
	err := d.QueryRow(`
		SELECT id, topic_code, title, level, body, created_at, updated_at
		FROM writing_piece WHERE id = ? AND user_id = ?`, id, userID).
		Scan(&p.ID, &p.TopicCode, &p.Title, &p.Level, &p.Body, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return WritingPiece{}, ErrNotFound
	}
	if err != nil {
		return WritingPiece{}, fmt.Errorf("read writing piece: %w", err)
	}
	p.CreatedAt, p.UpdatedAt = ParseTime(created), ParseTime(updated)
	return p, nil
}

// WritingPieces lists the user's pieces, newest edit first.
func (d *DB) WritingPieces(userID int64) ([]WritingPieceRow, error) {
	rows, err := d.Query(`
		SELECT p.id, p.topic_code, p.title, p.level, p.body, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM writing_check c WHERE c.piece_id = p.id),
		       COALESCE((SELECT c.id FROM writing_check c WHERE c.piece_id = p.id
		                 ORDER BY c.created_at DESC, c.id DESC LIMIT 1), 0)
		FROM writing_piece p WHERE p.user_id = ?
		ORDER BY p.updated_at DESC, p.id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list writing pieces: %w", err)
	}
	defer rows.Close()
	var out []WritingPieceRow
	var lastIDs []int64
	for rows.Next() {
		var r WritingPieceRow
		var created, updated sql.NullString
		var last int64
		if err := rows.Scan(&r.ID, &r.TopicCode, &r.Title, &r.Level, &r.Body, &created, &updated, &r.Checks, &last); err != nil {
			return nil, fmt.Errorf("scan writing piece: %w", err)
		}
		r.CreatedAt, r.UpdatedAt = ParseTime(created), ParseTime(updated)
		out = append(out, r)
		lastIDs = append(lastIDs, last)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i, id := range lastIDs {
		if id == 0 {
			continue
		}
		var c WritingCheck
		var created sql.NullString
		err := d.QueryRow(`SELECT id, piece_id, source, model, lang, prompt_version, interrupted, created_at
			FROM writing_check WHERE id = ?`, id).
			Scan(&c.ID, &c.PieceID, &c.Source, &c.Model, &c.Lang, &c.PromptVersion, &c.Interrupted, &created)
		if err != nil {
			return nil, fmt.Errorf("read last writing check: %w", err)
		}
		c.CreatedAt = ParseTime(created)
		out[i].LastCheck = &c
	}
	return out, nil
}

// UpdateWritingPiece saves the title and body (autosave) and touches
// updated_at. ErrNotFound when the piece is not the user's.
func (d *DB) UpdateWritingPiece(userID, id int64, title, body string) (WritingPiece, error) {
	res, err := d.Exec(`UPDATE writing_piece SET title = ?, body = ?, updated_at = ?
		WHERE id = ? AND user_id = ?`, title, body, FmtTime(time.Now()), id, userID)
	if err != nil {
		return WritingPiece{}, fmt.Errorf("update writing piece: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return WritingPiece{}, ErrNotFound
	}
	return d.WritingPiece(userID, id)
}

// DeleteWritingPiece removes a piece; its checks go with it (ON DELETE CASCADE).
func (d *DB) DeleteWritingPiece(userID, id int64) error {
	res, err := d.Exec(`DELETE FROM writing_piece WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete writing piece: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// AddWritingCheck stores a check on the user's piece. ErrTooManyChecks at the
// cap; ErrNotFound when the piece is not the user's.
func (d *DB) AddWritingCheck(userID int64, c WritingCheck) (WritingCheck, error) {
	var n int
	err := d.QueryRow(`SELECT COUNT(*) FROM writing_check c JOIN writing_piece p ON p.id = c.piece_id
		WHERE p.id = ? AND p.user_id = ?`, c.PieceID, userID).Scan(&n)
	if err != nil {
		return WritingCheck{}, fmt.Errorf("count writing checks: %w", err)
	}
	if n >= MaxChecksPerPiece {
		return WritingCheck{}, ErrTooManyChecks
	}
	if _, err := d.WritingPiece(userID, c.PieceID); err != nil {
		return WritingCheck{}, err
	}
	err = d.QueryRow(`
		INSERT INTO writing_check (piece_id, user_id, source, model, lang, prompt_version, checked_text, raw, interrupted, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		c.PieceID, userID, c.Source, c.Model, c.Lang, c.PromptVersion, c.CheckedText, c.Raw, c.Interrupted,
		FmtTime(time.Now())).Scan(&c.ID)
	if err != nil {
		return WritingCheck{}, fmt.Errorf("add writing check: %w", err)
	}
	return d.WritingCheck(userID, c.ID)
}

// WritingCheck reads one check of the user's.
func (d *DB) WritingCheck(userID, id int64) (WritingCheck, error) {
	row := d.QueryRow(`SELECT id, piece_id, source, model, lang, prompt_version, checked_text, raw, interrupted, created_at
		FROM writing_check WHERE id = ? AND user_id = ?`, id, userID)
	c, err := scanWritingCheck(row)
	if errors.Is(err, sql.ErrNoRows) {
		return WritingCheck{}, ErrNotFound
	}
	return c, err
}

// WritingChecks lists a piece's checks, newest first, with their text.
func (d *DB) WritingChecks(userID, pieceID int64) ([]WritingCheck, error) {
	rows, err := d.Query(`SELECT id, piece_id, source, model, lang, prompt_version, checked_text, raw, interrupted, created_at
		FROM writing_check WHERE piece_id = ? AND user_id = ?
		ORDER BY created_at DESC, id DESC`, pieceID, userID)
	if err != nil {
		return nil, fmt.Errorf("list writing checks: %w", err)
	}
	defer rows.Close()
	var out []WritingCheck
	for rows.Next() {
		c, err := scanWritingCheck(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteWritingCheck removes one check of the user's piece.
func (d *DB) DeleteWritingCheck(userID, pieceID, id int64) error {
	res, err := d.Exec(`DELETE FROM writing_check WHERE id = ? AND piece_id = ? AND user_id = ?`, id, pieceID, userID)
	if err != nil {
		return fmt.Errorf("delete writing check: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanWritingCheck(r rowScanner) (WritingCheck, error) {
	var c WritingCheck
	var created sql.NullString
	if err := r.Scan(&c.ID, &c.PieceID, &c.Source, &c.Model, &c.Lang, &c.PromptVersion,
		&c.CheckedText, &c.Raw, &c.Interrupted, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WritingCheck{}, err
		}
		return WritingCheck{}, fmt.Errorf("scan writing check: %w", err)
	}
	c.CreatedAt = ParseTime(created)
	return c, nil
}
