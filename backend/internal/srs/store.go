package srs

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"tanulom/internal/db"
)

// Store persists notes, cards and both knowledge state spaces.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// querier is satisfied by both *sql.DB and *sql.Tx.
//
// This exists because the connection pool is capped at one connection (SQLite
// has a single writer, so serialising is cheaper than fighting for locks). Any
// read issued against the pool while a transaction holds that connection
// deadlocks instantly. Every method that can run inside a transaction therefore
// takes the transaction and uses it.
type querier interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
}

// q returns tx when inside a transaction, otherwise the pool.
func (s *Store) q(tx *sql.Tx) querier {
	if tx != nil {
		return tx
	}
	return s.db
}

// Note is a thing to learn: normally a sentence, because a bare lemma teaches
// nothing about the morphology that carries the meaning.
type Note struct {
	ID          int64
	UserID      int64
	Kind        string
	LemmaID     *int64
	SentenceID  *int64
	HU          string
	TargetForm  string
	TargetStart int
	TargetEnd   int
	TargetFeats []string
	GlossRU     string
	GlossEN     string
	Notes       string
	Source      string
}

// Card is one direction of a note, with its own FSRS state.
type Card struct {
	ID        int64
	NoteID    int64
	Direction Direction
	State     CardState
	Suspended bool
}

// FindDuplicate returns the id of an existing note with the same sentence and
// target form, or 0.
//
// Adding the same word twice is not a hypothetical: a learner rereading a
// paragraph will press "add card" on a word they already added, and silently
// creating a second identical note doubles their review load for no gain while
// looking like the app worked.
func (s *Store) FindDuplicate(userID int64, hu, targetForm string) (int64, error) {
	var id int64
	err := s.db.QueryRow(`
		SELECT id FROM note
		 WHERE user_id = ? AND hu = ? AND target_form = ?
		 LIMIT 1`, userID, hu, targetForm).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("check duplicate note: %w", err)
	}
	return id, nil
}

// CreateNote inserts a note and its cards in one transaction: a note with no
// cards is unreviewable and would sit invisible in the database forever.
func (s *Store) CreateNote(n Note, directions []Direction, now time.Time) (int64, []Card, error) {
	if len(directions) == 0 {
		return 0, nil, errors.New("a note needs at least one direction")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after a successful commit

	res, err := tx.Exec(`
		INSERT INTO note (user_id, kind, lemma_id, sentence_id, hu, target_form,
		                  target_start, target_end, target_feats, gloss_ru, gloss_en,
		                  notes, source)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		n.UserID, n.Kind, n.LemmaID, n.SentenceID, n.HU, n.TargetForm,
		n.TargetStart, n.TargetEnd, strings.Join(n.TargetFeats, "|"),
		n.GlossRU, n.GlossEN, n.Notes, orDefault(n.Source, "manual"))
	if err != nil {
		return 0, nil, fmt.Errorf("create note: %w", err)
	}
	noteID, err := res.LastInsertId()
	if err != nil {
		return 0, nil, err
	}

	fresh := NewCardState(now)
	cards := make([]Card, 0, len(directions))
	for _, d := range directions {
		if !d.Valid() {
			return 0, nil, fmt.Errorf("unknown direction %q", d)
		}
		r, err := tx.Exec(`
			INSERT INTO card (note_id, direction, due, stability, difficulty,
			                  scheduled_days, reps, lapses, state, last_review)
			VALUES (?,?,?,0,0,0,0,0,0,NULL)`,
			noteID, string(d), db.FmtTime(fresh.Due))
		if err != nil {
			return 0, nil, fmt.Errorf("create card %s: %w", d, err)
		}
		id, _ := r.LastInsertId()
		cards = append(cards, Card{ID: id, NoteID: noteID, Direction: d, State: fresh})
	}

	if err := tx.Commit(); err != nil {
		return 0, nil, err
	}
	return noteID, cards, nil
}

// Note loads one note.
func (s *Store) Note(userID, noteID int64) (Note, error) {
	var n Note
	var feats string
	err := s.db.QueryRow(`
		SELECT id, user_id, kind, lemma_id, sentence_id, hu, target_form,
		       target_start, target_end, target_feats, gloss_ru, gloss_en, notes, source
		  FROM note WHERE id = ? AND user_id = ?`, noteID, userID,
	).Scan(&n.ID, &n.UserID, &n.Kind, &n.LemmaID, &n.SentenceID, &n.HU, &n.TargetForm,
		&n.TargetStart, &n.TargetEnd, &feats, &n.GlossRU, &n.GlossEN, &n.Notes, &n.Source)
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrNotFound
	}
	if err != nil {
		return Note{}, fmt.Errorf("load note: %w", err)
	}
	if feats != "" {
		n.TargetFeats = strings.Split(feats, "|")
	}
	return n, nil
}

// ErrNotFound is returned when a lookup matches nothing.
var ErrNotFound = errors.New("not found")

// Card loads one card, checking it belongs to the user.
func (s *Store) Card(userID, cardID int64) (Card, error) {
	var c Card
	var due string
	var last sql.NullString
	err := s.db.QueryRow(`
		SELECT c.id, c.note_id, c.direction, c.due, c.stability, c.difficulty,
		       c.scheduled_days, c.reps, c.lapses, c.state, c.last_review, c.suspended
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE c.id = ? AND n.user_id = ?`, cardID, userID,
	).Scan(&c.ID, &c.NoteID, &c.Direction, &due, &c.State.Stability, &c.State.Difficulty,
		&c.State.ScheduledDays, &c.State.Reps, &c.State.Lapses, &c.State.State,
		&last, &c.Suspended)
	if errors.Is(err, sql.ErrNoRows) {
		return Card{}, ErrNotFound
	}
	if err != nil {
		return Card{}, fmt.Errorf("load card: %w", err)
	}
	c.State.Due = db.ParseTime(sql.NullString{String: due, Valid: true})
	c.State.LastReview = db.ParseTime(last)
	return c, nil
}

// SaveCard writes a card's FSRS state.
func (s *Store) SaveCard(tx *sql.Tx, c Card) error {
	exec := s.q(tx).Exec
	var last any
	if !c.State.LastReview.IsZero() {
		last = db.FmtTime(c.State.LastReview)
	}
	_, err := exec(`
		UPDATE card SET due=?, stability=?, difficulty=?, scheduled_days=?,
		                reps=?, lapses=?, state=?, last_review=?
		 WHERE id=?`,
		db.FmtTime(c.State.Due), c.State.Stability, c.State.Difficulty,
		c.State.ScheduledDays, c.State.Reps, c.State.Lapses, c.State.State, last, c.ID)
	if err != nil {
		return fmt.Errorf("save card: %w", err)
	}
	return nil
}

// AppendLog writes a review_log row.
func (s *Store) AppendLog(tx *sql.Tx, cardID int64, e LogEntry) error {
	exec := s.q(tx).Exec
	correct := 0
	if e.Correct {
		correct = 1
	}
	_, err := exec(`
		INSERT INTO review_log (card_id, rating, state, due, stability, difficulty,
		                        elapsed_days, last_elapsed_days, scheduled_days,
		                        review_time, duration_ms, answer, correct)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		cardID, int(e.Rating), e.State, db.FmtTime(e.Due), e.Stability, e.Difficulty,
		e.ElapsedDays, e.LastElapsedDays, e.ScheduledDays,
		db.FmtTime(e.ReviewTime), e.DurationMs, e.Answer, correct)
	if err != nil {
		return fmt.Errorf("append review log: %w", err)
	}
	return nil
}

// FeatureState is one row of the morphological state space.
type FeatureState struct {
	FeatureID string
	State     CardState
	Accuracy  float64
	Total     int
	MedianMs  int64
}

// FeatureState loads one feature's state, returning a fresh one if absent.
//
// Pass the enclosing transaction when inside one; see the querier comment above.
func (s *Store) FeatureState(tx *sql.Tx, userID int64, featureID string, now time.Time) (FeatureState, error) {
	fs := FeatureState{FeatureID: featureID, State: NewCardState(now)}
	var due, last sql.NullString
	err := s.q(tx).QueryRow(`
		SELECT due, stability, difficulty, scheduled_days, reps, lapses, state,
		       last_review, recent_accuracy, recent_total, median_ms
		  FROM morph_state WHERE user_id = ? AND feature_id = ?`, userID, featureID,
	).Scan(&due, &fs.State.Stability, &fs.State.Difficulty, &fs.State.ScheduledDays,
		&fs.State.Reps, &fs.State.Lapses, &fs.State.State, &last,
		&fs.Accuracy, &fs.Total, &fs.MedianMs)
	if errors.Is(err, sql.ErrNoRows) {
		return fs, nil
	}
	if err != nil {
		return fs, fmt.Errorf("load feature state: %w", err)
	}
	fs.State.Due = db.ParseTime(due)
	fs.State.LastReview = db.ParseTime(last)
	return fs, nil
}

// SaveFeatureState upserts a feature's state.
func (s *Store) SaveFeatureState(tx *sql.Tx, userID int64, fs FeatureState) error {
	exec := s.q(tx).Exec
	var due, last any
	if !fs.State.Due.IsZero() {
		due = db.FmtTime(fs.State.Due)
	}
	if !fs.State.LastReview.IsZero() {
		last = db.FmtTime(fs.State.LastReview)
	}
	_, err := exec(`
		INSERT INTO morph_state (user_id, feature_id, due, stability, difficulty,
		                         scheduled_days, reps, lapses, state, last_review,
		                         recent_accuracy, recent_total, median_ms)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(user_id, feature_id) DO UPDATE SET
		  due=excluded.due, stability=excluded.stability, difficulty=excluded.difficulty,
		  scheduled_days=excluded.scheduled_days, reps=excluded.reps,
		  lapses=excluded.lapses, state=excluded.state, last_review=excluded.last_review,
		  recent_accuracy=excluded.recent_accuracy, recent_total=excluded.recent_total,
		  median_ms=excluded.median_ms`,
		userID, fs.FeatureID, due, fs.State.Stability, fs.State.Difficulty,
		fs.State.ScheduledDays, fs.State.Reps, fs.State.Lapses, fs.State.State, last,
		fs.Accuracy, fs.Total, fs.MedianMs)
	if err != nil {
		return fmt.Errorf("save feature state: %w", err)
	}
	return nil
}

// AppendFeatureReview records one graded feature.
func (s *Store) AppendFeatureReview(tx *sql.Tx, userID int64, g FeatureGrade,
	cardID int64, durationMs int64, now time.Time, origin string) error {
	exec := s.q(tx).Exec
	correct := 0
	if g.Correct {
		correct = 1
	}
	_, err := exec(`
		INSERT INTO feature_review (user_id, feature_id, rating, correct,
		                            duration_ms, review_time, card_id, origin)
		VALUES (?,?,?,?,?,?,?,?)`,
		userID, g.FeatureID, int(g.Rating), correct, durationMs,
		db.FmtTime(now), nullableID(cardID), orDefault(origin, "review"))
	if err != nil {
		return fmt.Errorf("append feature review: %w", err)
	}
	return nil
}

// LemmaState loads a lemma's knowledge state, returning a fresh one if absent.
func (s *Store) LemmaState(userID, lemmaID int64, now time.Time) (string, CardState, error) {
	status := "new"
	st := NewCardState(now)
	var due, last sql.NullString
	err := s.db.QueryRow(`
		SELECT status, due, stability, difficulty, scheduled_days, reps, lapses,
		       state, last_review
		  FROM lemma_state WHERE user_id = ? AND lemma_id = ?`, userID, lemmaID,
	).Scan(&status, &due, &st.Stability, &st.Difficulty, &st.ScheduledDays,
		&st.Reps, &st.Lapses, &st.State, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return status, st, nil
	}
	if err != nil {
		return status, st, fmt.Errorf("load lemma state: %w", err)
	}
	st.Due = db.ParseTime(due)
	st.LastReview = db.ParseTime(last)
	return status, st, nil
}

// MarkLemmaLearning puts a lemma into the learning state — the effect of
// creating a note for it. Deliberately weaker than SetLemmaStatus: it never
// overrides a learner's own declaration, so a word marked `known` in the
// reader stays known when a card is later minted for it, and an `ignored`
// word stays ignored. Only `new` (or absent) becomes `learning`.
//
// This is what feeds the lemma curve and the checkpoints' "лемм в Anki"
// criterion: a lemma is IN PLAY from the moment it has a card.
func (s *Store) MarkLemmaLearning(userID, lemmaID int64) error {
	_, err := s.db.Exec(`
		INSERT INTO lemma_state (user_id, lemma_id, status) VALUES (?,?,'learning')
		ON CONFLICT(user_id, lemma_id) DO UPDATE SET status='learning'
		 WHERE lemma_state.status = 'new'`, userID, lemmaID)
	if err != nil {
		return fmt.Errorf("mark lemma learning: %w", err)
	}
	return nil
}

// SetLemmaStatus records a learner declaration such as "I know this word".
//
// A declaration bypasses scheduling on purpose: marking words known while
// reading must not manufacture review load, or the reader becomes a trap.
func (s *Store) SetLemmaStatus(userID, lemmaID int64, status string) error {
	switch status {
	case "new", "learning", "known", "ignored":
	default:
		return fmt.Errorf("unknown lemma status %q", status)
	}
	_, err := s.db.Exec(`
		INSERT INTO lemma_state (user_id, lemma_id, status) VALUES (?,?,?)
		ON CONFLICT(user_id, lemma_id) DO UPDATE SET status = excluded.status`,
		userID, lemmaID, status)
	if err != nil {
		return fmt.Errorf("set lemma status: %w", err)
	}
	return nil
}

// KnownLemmaIDs returns the lemmas a learner has declared known or is learning
// well enough to count. Used by n+1 scoring and by the LLM prompt builder.
func (s *Store) KnownLemmaIDs(userID int64) (map[int64]string, error) {
	rows, err := s.db.Query(
		`SELECT lemma_id, status FROM lemma_state WHERE user_id = ? AND status != 'new'`, userID)
	if err != nil {
		return nil, fmt.Errorf("list known lemmas: %w", err)
	}
	defer rows.Close()

	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var status string
		if err := rows.Scan(&id, &status); err != nil {
			return nil, err
		}
		out[id] = status
	}
	return out, rows.Err()
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func nullableID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

// DueCount counts review-due cards right now — the streak's "queue cleared"
// obligation. New cards are not obligations (they are budget), and suspended
// cards are not due by definition.
func (s *Store) DueCount(userID int64, now time.Time) (int, error) {
	var n int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND c.suspended = 0 AND c.state != 0 AND c.due <= ?`,
		userID, db.FmtTime(now)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count due: %w", err)
	}
	return n, nil
}
