package mining

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Store persists the reading library.
type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// ErrNotFound is returned when a text does not exist or belongs to someone else.
var ErrNotFound = errors.New("not found")

// Text is a library entry.
type Text struct {
	ID              int64     `json:"id"`
	Title           string    `json:"title"`
	SourceURL       string    `json:"sourceUrl"`
	Kind            string    `json:"kind"`
	Body            string    `json:"body,omitempty"`
	WordCount       int       `json:"wordCount"`
	LemmaCoverage   float64   `json:"lemmaCoverage"`
	ElementCoverage float64   `json:"elementCoverage"`
	AddedAt         time.Time `json:"addedAt"`
}

// StoredToken is one analysed token as persisted.
type StoredToken struct {
	Index     int      `json:"index"`
	SentIdx   int      `json:"sentIdx"`
	Form      string   `json:"form"`
	Start     int      `json:"start"`
	End       int      `json:"end"`
	IsWord    bool     `json:"isWord"`
	LemmaID   int64    `json:"lemmaId"`
	Feats     []string `json:"feats"`
	Certainty string   `json:"certainty"`
}

// Create stores a text and its tokens in one transaction.
//
// Tokens are persisted rather than re-derived on every view: re-analysing a
// long article on each page load is wasteful, and coverage over a learner's own
// accumulated corpus is only meaningful if the corpus persists.
func (s *Store) Create(userID int64, in Ingested, sourceURL string, tokens []StoredToken, cov Coverage) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	res, err := tx.Exec(`
		INSERT INTO text (user_id, title, source_url, kind, body,
		                  word_count, lemma_coverage, element_coverage)
		VALUES (?,?,?,?,?,?,?,?)`,
		userID, in.Title, sourceURL, string(in.Kind), in.Body,
		cov.Words, cov.LemmaCoverage, cov.ElementCoverage)
	if err != nil {
		return 0, fmt.Errorf("create text: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO text_token (text_id, idx, sent_idx, form, start_b, end_b,
		                        is_word, lemma_id, feats, certainty)
		VALUES (?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, t := range tokens {
		var lemma any
		if t.LemmaID != 0 {
			lemma = t.LemmaID
		}
		if _, err := stmt.Exec(id, t.Index, t.SentIdx, t.Form, t.Start, t.End,
			boolInt(t.IsWord), lemma, strings.Join(t.Feats, "|"), t.Certainty); err != nil {
			return 0, fmt.Errorf("store token %d: %w", t.Index, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// List returns the library, newest first, without bodies.
func (s *Store) List(userID int64, limit int) ([]Text, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT id, title, source_url, kind, word_count,
		       lemma_coverage, element_coverage, added_at
		  FROM text WHERE user_id = ? ORDER BY added_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list texts: %w", err)
	}
	defer rows.Close()

	out := []Text{}
	for rows.Next() {
		var t Text
		var added string
		if err := rows.Scan(&t.ID, &t.Title, &t.SourceURL, &t.Kind, &t.WordCount,
			&t.LemmaCoverage, &t.ElementCoverage, &added); err != nil {
			return nil, err
		}
		t.AddedAt, _ = time.Parse(time.RFC3339, added)
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get loads one text with its body.
func (s *Store) Get(userID, id int64) (Text, error) {
	var t Text
	var added string
	err := s.db.QueryRow(`
		SELECT id, title, source_url, kind, body, word_count,
		       lemma_coverage, element_coverage, added_at
		  FROM text WHERE id = ? AND user_id = ?`, id, userID,
	).Scan(&t.ID, &t.Title, &t.SourceURL, &t.Kind, &t.Body, &t.WordCount,
		&t.LemmaCoverage, &t.ElementCoverage, &added)
	if errors.Is(err, sql.ErrNoRows) {
		return Text{}, ErrNotFound
	}
	if err != nil {
		return Text{}, fmt.Errorf("load text: %w", err)
	}
	t.AddedAt, _ = time.Parse(time.RFC3339, added)
	return t, nil
}

// Tokens loads a text's analysed tokens.
func (s *Store) Tokens(userID, textID int64) ([]StoredToken, error) {
	rows, err := s.db.Query(`
		SELECT t.idx, t.sent_idx, t.form, t.start_b, t.end_b, t.is_word,
		       COALESCE(t.lemma_id, 0), t.feats, t.certainty
		  FROM text_token t JOIN text x ON x.id = t.text_id
		 WHERE t.text_id = ? AND x.user_id = ? ORDER BY t.idx`, textID, userID)
	if err != nil {
		return nil, fmt.Errorf("load tokens: %w", err)
	}
	defer rows.Close()

	out := []StoredToken{}
	for rows.Next() {
		var t StoredToken
		var isWord int
		var feats string
		if err := rows.Scan(&t.Index, &t.SentIdx, &t.Form, &t.Start, &t.End,
			&isWord, &t.LemmaID, &feats, &t.Certainty); err != nil {
			return nil, err
		}
		t.IsWord = isWord == 1
		if feats != "" {
			t.Feats = strings.Split(feats, "|")
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Delete removes a text and its tokens.
func (s *Store) Delete(userID, id int64) error {
	res, err := s.db.Exec(`DELETE FROM text WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete text: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkRead records that a text was opened.
func (s *Store) MarkRead(userID, id int64, at time.Time) error {
	_, err := s.db.Exec(`UPDATE text SET read_at = ? WHERE id = ? AND user_id = ?`,
		at.UTC().Format(time.RFC3339), id, userID)
	return err
}

// UpdateCoverage refreshes the stored coverage figures.
//
// Coverage is a moving target: it improves as the learner does, so the number
// shown in the library must be recomputed rather than frozen at ingest time.
func (s *Store) UpdateCoverage(userID, id int64, cov Coverage) error {
	_, err := s.db.Exec(`
		UPDATE text SET lemma_coverage = ?, element_coverage = ?, word_count = ?
		 WHERE id = ? AND user_id = ?`,
		cov.LemmaCoverage, cov.ElementCoverage, cov.Words, id, userID)
	return err
}

// AsTokenViews converts stored tokens for the scorer and coverage calculator.
func AsTokenViews(tokens []StoredToken) []TokenView {
	out := make([]TokenView, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, TokenView{
			Index: t.Index, SentIdx: t.SentIdx, Form: t.Form,
			IsWord: t.IsWord, LemmaID: t.LemmaID, FeatureIDs: t.Feats,
		})
	}
	return out
}

// SentenceCandidates groups tokens into scoreable sentences.
func SentenceCandidates(tokens []StoredToken) []Candidate {
	bySent := map[int][]TokenView{}
	textOf := map[int]*strings.Builder{}
	order := []int{}

	for _, t := range tokens {
		if _, seen := bySent[t.SentIdx]; !seen {
			order = append(order, t.SentIdx)
			textOf[t.SentIdx] = &strings.Builder{}
		}
		bySent[t.SentIdx] = append(bySent[t.SentIdx], TokenView{
			Index: t.Index, SentIdx: t.SentIdx, Form: t.Form,
			IsWord: t.IsWord, LemmaID: t.LemmaID, FeatureIDs: t.Feats,
		})
		b := textOf[t.SentIdx]
		if b.Len() > 0 && t.IsWord {
			b.WriteString(" ")
		}
		b.WriteString(t.Form)
	}

	out := make([]Candidate, 0, len(order))
	for _, idx := range order {
		out = append(out, Candidate{
			SentIdx: idx,
			Text:    strings.TrimSpace(textOf[idx].String()),
			Tokens:  bySent[idx],
		})
	}
	return out
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
