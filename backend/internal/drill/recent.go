package drill

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// What the learner met lately, so repeated drills vary.
//
// A drill session built only from the topic and the learner's weak features is
// the same session every time: the same frequent lemmas, and — worse — the same
// corpus sentences, because a lemma often has only a handful. Two things fix
// that without touching the scheduler, which never hears about any of this:
//
//   - the attempt log already records every item, so recency is READ from it
//     (item_key, corpus_sentence; see migration 00025) rather than kept in a second
//     structure that could drift from it;
//   - items served but not yet answered are merged in from the pending store,
//     because a learner who fetches the next batch before finishing the last one
//     would otherwise be handed the same items again.
//
// Anti-repeat filters the CONCRETE lemma and sentence only. Which feature is
// drilled stays the business of weightedFeature, so weak-feature oversampling is
// unaffected. Recency never makes a session come back short: see fill.

// recentWindow is how many past attempts of a topic count as "recent".
const recentWindow = 300

// typedUnlockCorrect is the number of correct blocked answers a topic needs
// before typed kinds join the blocked-phase rotation. Until then production
// is the load blocked practice exists to avoid. [R5]
const typedUnlockCorrect = 10

// Recent is the memory of one topic's recent items. A nil *Recent means "no
// memory" and every method is safe on it, which is what placement and the
// availability probe pass.
type Recent struct {
	// Keys are the exact item keys seen, mapped to their recency rank (0 =
	// newest). Oldest-first reuse reads this.
	Keys map[string]int
	// Prompts are the log `prompt` strings of rows written before item_key
	// existed; matched against promptOf for those rows only.
	Prompts map[string]int
	// Sentences are the corpus sentence ids already used.
	Sentences map[int64]bool
	// Lemmas counts how often each lemma appeared.
	Lemmas map[int64]int
	// KindRank is the recency rank of each kind's most recent use (0 = newest).
	// A kind absent from the map has not been used in the window.
	KindRank map[Kind]int
	// CorrectBlocked is the topic's lifetime count of correct blocked answers.
	CorrectBlocked int

	n int // items folded so far; the next one is older than every earlier one
}

// NewRecent builds an empty memory.
func NewRecent() *Recent {
	return &Recent{
		Keys: map[string]int{}, Prompts: map[string]int{}, Sentences: map[int64]bool{},
		Lemmas: map[int64]int{}, KindRank: map[Kind]int{},
	}
}

// add folds one item in. Callers add NEWEST first, so rank grows with age.
func (r *Recent) add(kind Kind, key, prompt string, sentenceID, lemmaID int64) {
	rank := r.n
	r.n++
	if key != "" {
		if _, ok := r.Keys[key]; !ok {
			r.Keys[key] = rank
		}
	} else if prompt != "" {
		if _, ok := r.Prompts[prompt]; !ok {
			r.Prompts[prompt] = rank
		}
	}
	if sentenceID != 0 {
		r.Sentences[sentenceID] = true
	}
	if lemmaID != 0 {
		r.Lemmas[lemmaID]++
	}
	if _, ok := r.KindRank[kind]; !ok {
		r.KindRank[kind] = rank
	}
}

// AddItem folds in an item that was served and not yet answered. It counts as
// newer than anything in the log, so call it AFTER the log has been read.
func (r *Recent) AddItem(it Item) {
	if r == nil {
		return
	}
	// Shift every rank so the unanswered item is the newest of all.
	for k, v := range r.Keys {
		r.Keys[k] = v + 1
	}
	for k, v := range r.Prompts {
		r.Prompts[k] = v + 1
	}
	for k, v := range r.KindRank {
		r.KindRank[k] = v + 1
	}
	r.n++
	r.Keys[it.Key()] = 0
	if it.SentenceID != 0 {
		r.Sentences[it.SentenceID] = true
	}
	if it.LemmaID != 0 {
		r.Lemmas[it.LemmaID]++
	}
	for _, t := range it.Tiles {
		r.Lemmas[t.lemmaID]++
	}
	r.KindRank[it.Kind] = 0
}

// keyRank reports whether the item was seen recently and how long ago (a larger
// rank is older).
func (r *Recent) keyRank(it Item) (int, bool) {
	if r == nil {
		return 0, false
	}
	if rank, ok := r.Keys[it.Key()]; ok {
		return rank, true
	}
	if len(r.Prompts) > 0 {
		if rank, ok := r.Prompts[promptOf(it)]; ok {
			return rank, true
		}
	}
	return 0, false
}

func (r *Recent) lemmaUses(id int64) int {
	if r == nil || id == 0 {
		return 0
	}
	return r.Lemmas[id]
}

func (r *Recent) sentenceUsed(id int64) bool {
	return r != nil && id != 0 && r.Sentences[id]
}

// kindAge orders kinds by how long ago they were last used: larger is staler,
// and a kind never used in the window is the stalest.
func (r *Recent) kindAge(k Kind) int {
	if r == nil {
		return 1 << 30
	}
	if rank, ok := r.KindRank[k]; ok {
		return rank
	}
	return 1 << 30
}

// Recent loads a learner's recent memory of one topic.
func (s *Store) Recent(userID int64, topic string, limit int) (*Recent, error) {
	if limit <= 0 {
		limit = recentWindow
	}
	rows, err := s.db.Query(`
		SELECT kind, item_key, prompt, corpus_sentence
		  FROM drill_attempt
		 WHERE user_id = ? AND topic_code = ?
		 ORDER BY id DESC LIMIT ?`, userID, topic, limit)
	if err != nil {
		return nil, fmt.Errorf("load recent drill items: %w", err)
	}
	defer rows.Close()

	r := NewRecent()
	for rows.Next() {
		var kind, key, prompt string
		var sentID int64
		if err := rows.Scan(&kind, &key, &prompt, &sentID); err != nil {
			return nil, err
		}
		r.add(Kind(kind), key, prompt, sentID, lemmaOfKey(key))
		if Kind(kind) == KindSort {
			// A sort item is six lemmas at once; each one counts as used.
			for _, id := range sortLemmasOfKey(key) {
				r.Lemmas[id]++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM drill_attempt
		 WHERE user_id = ? AND topic_code = ? AND phase = 'blocked' AND correct = 1`,
		userID, topic).Scan(&r.CorrectBlocked); err != nil {
		return nil, fmt.Errorf("count correct blocked drills: %w", err)
	}
	return r, nil
}

// Key identifies a question: the same kind, the same word, the same feature and
// the same corpus sentence is "the same item" however the options were shuffled.
func (i Item) Key() string {
	if i.Kind == KindSort {
		return sortKey(i)
	}
	if i.Kind == KindScene {
		return sceneKey(i)
	}
	if i.Kind == KindMatch {
		cues := append([]string(nil), i.Columns...)
		sort.Strings(cues)
		return "match|" + strconv.FormatInt(i.LemmaID, 10) + "|" + i.Headline + "|" + strings.Join(cues, ",")
	}
	if i.Kind == KindPerson && i.Bare {
		return "person|bare|" + strconv.FormatInt(i.LemmaID, 10) + "|" + i.cell
	}
	return string(i.Kind) + "|" + strconv.FormatInt(i.LemmaID, 10) + "|" + i.Headline +
		"|" + strconv.FormatInt(i.SentenceID, 10)
}

// lemmaOfKey reads the lemma id back out of an item key; 0 for legacy rows.
func lemmaOfKey(key string) int64 {
	parts := strings.SplitN(key, "|", 4)
	if len(parts) >= 3 && parts[0] == "person" && parts[1] == "bare" {
		id, _ := strconv.ParseInt(parts[2], 10, 64)
		return id
	}
	if len(parts) < 2 {
		return 0
	}
	id, _ := strconv.ParseInt(parts[1], 10, 64)
	return id
}
