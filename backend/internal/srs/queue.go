package srs

import (
	"database/sql"
	"fmt"
	"time"
)

// QueueItem is one card ready to be shown, with everything the UI needs.
type QueueItem struct {
	CardID      int64     `json:"cardId"`
	NoteID      int64     `json:"noteId"`
	Direction   Direction `json:"direction"`
	IsNew       bool      `json:"isNew"`
	Due         time.Time `json:"due"`
	HU          string    `json:"hu"`
	TargetForm  string    `json:"targetForm"`
	TargetStart int       `json:"targetStart"`
	TargetEnd   int       `json:"targetEnd"`
	TargetFeats []string  `json:"targetFeats"`
	GlossRU     string    `json:"glossRu"`
	GlossEN     string    `json:"glossEn"`
	LemmaID     *int64    `json:"lemmaId,omitempty"`
}

// QueueCounts summarises what is waiting.
type QueueCounts struct {
	Due       int `json:"due"`
	New       int `json:"new"`
	Learning  int `json:"learning"`
	Total     int `json:"total"`
	NewCapped int `json:"newCapped"`
}

// Queue assembles today's review queue.
//
// Two rules that matter more than they look:
//
//   - **New cards are capped and reviews are not.** Reviews are a debt already
//     incurred; refusing to show them just defers the problem. New cards are the
//     tap, and the cap is the only thing standing between a learner and the
//     review avalanche that is the most common reason people quit.
//   - **Siblings are buried.** Showing `recognize` and `produce` of the same note
//     in one session turns the second into a copying exercise; the answer is
//     still on screen in the learner's memory, so it tests nothing.
func (s *Store) Queue(userID int64, newLimit, maxReviews int, burySiblings bool, now time.Time) ([]QueueItem, QueueCounts, error) {
	nowStr := fmtTime(now)

	counts, err := s.queueCounts(userID, nowStr)
	if err != nil {
		return nil, QueueCounts{}, err
	}

	limit := maxReviews
	if limit <= 0 {
		limit = 10_000
	}

	// Due cards first, oldest first: a card overdue by a month is decaying
	// faster than one due today.
	items, err := s.selectQueue(userID, `
		AND c.state != 0 AND c.due <= ?
		ORDER BY c.due ASC LIMIT ?`, nowStr, limit)
	if err != nil {
		return nil, QueueCounts{}, err
	}

	if newLimit > 0 {
		introducedToday, err := s.newCardsIntroducedToday(userID, now)
		if err != nil {
			return nil, QueueCounts{}, err
		}
		remaining := newLimit - introducedToday
		if remaining > 0 {
			fresh, err := s.selectQueue(userID, `
				AND c.state = 0
				ORDER BY c.note_id ASC, c.id ASC LIMIT ?`, remaining)
			if err != nil {
				return nil, QueueCounts{}, err
			}
			items = append(items, fresh...)
			counts.NewCapped = max(0, counts.New-remaining)
		} else {
			counts.NewCapped = counts.New
		}
	} else {
		counts.NewCapped = counts.New
	}

	if burySiblings {
		items = burySiblingCards(items)
	}
	counts.Total = len(items)
	return items, counts, nil
}

func (s *Store) selectQueue(userID int64, where string, args ...any) ([]QueueItem, error) {
	q := `
		SELECT c.id, c.note_id, c.direction, c.state, c.due,
		       n.hu, n.target_form, n.target_start, n.target_end, n.target_feats,
		       n.gloss_ru, n.gloss_en, n.lemma_id
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND c.suspended = 0 ` + where

	rows, err := s.db.Query(q, append([]any{userID}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("build queue: %w", err)
	}
	defer rows.Close()

	var out []QueueItem
	for rows.Next() {
		var it QueueItem
		var state int
		var due, feats string
		if err := rows.Scan(&it.CardID, &it.NoteID, &it.Direction, &state, &due,
			&it.HU, &it.TargetForm, &it.TargetStart, &it.TargetEnd, &feats,
			&it.GlossRU, &it.GlossEN, &it.LemmaID); err != nil {
			return nil, fmt.Errorf("scan queue item: %w", err)
		}
		it.IsNew = state == 0
		it.Due = parseTime(sql.NullString{String: due, Valid: true})
		it.TargetFeats = splitFeats(feats)
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) queueCounts(userID int64, nowStr string) (QueueCounts, error) {
	var c QueueCounts
	err := s.db.QueryRow(`
		SELECT
		  SUM(CASE WHEN c.state != 0 AND c.due <= ? THEN 1 ELSE 0 END),
		  SUM(CASE WHEN c.state  = 0 THEN 1 ELSE 0 END),
		  SUM(CASE WHEN c.state IN (1,3) THEN 1 ELSE 0 END)
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND c.suspended = 0`, nowStr, userID,
	).Scan(&nullInt{&c.Due}, &nullInt{&c.New}, &nullInt{&c.Learning})
	if err != nil {
		return c, fmt.Errorf("count queue: %w", err)
	}
	return c, nil
}

// newCardsIntroducedToday counts first reviews since local midnight, so the
// daily cap means a day rather than a rolling 24 hours.
func (s *Store) newCardsIntroducedToday(userID int64, now time.Time) (int, error) {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var n int
	err := s.db.QueryRow(`
		SELECT COUNT(DISTINCT r.card_id)
		  FROM review_log r
		  JOIN card c ON c.id = r.card_id
		  JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND r.state = 0 AND r.review_time >= ?`,
		userID, fmtTime(midnight)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count new cards today: %w", err)
	}
	return n, nil
}

// burySiblingCards keeps at most one card per note in a session.
func burySiblingCards(items []QueueItem) []QueueItem {
	seen := map[int64]bool{}
	out := make([]QueueItem, 0, len(items))
	for _, it := range items {
		if seen[it.NoteID] {
			continue
		}
		seen[it.NoteID] = true
		out = append(out, it)
	}
	return out
}

func splitFeats(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '|' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}

// nullInt scans a possibly-NULL SUM into an int.
type nullInt struct{ dst *int }

func (n *nullInt) Scan(v any) error {
	switch t := v.(type) {
	case nil:
		*n.dst = 0
	case int64:
		*n.dst = int(t)
	case float64:
		*n.dst = int(t)
	default:
		return fmt.Errorf("cannot scan %T into int", v)
	}
	return nil
}
