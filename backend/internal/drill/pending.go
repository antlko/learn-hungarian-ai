package drill

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Served-but-unanswered items.
//
// The expected answer never goes to the client. A drill screen that ships the
// right answer in its JSON is not a retrieval test, and — more importantly — a
// client that echoes back "this item graded Case=Ine and I got it right" could
// corrupt the mastery heatmap, which is the one number in this app that has to
// be trustworthy. So the server keeps the item and the client posts back only an
// id, its answer and how long it took. See ADR-008.
//
// In memory rather than in a table, deliberately. These records are worth
// nothing after the answer: persisting them would add a migration, a cleanup job
// and a class of stale-row bugs, in exchange for surviving a restart that costs
// the learner one keystroke. An item that has expired returns a clear error and
// the client fetches a new one.
//
// The store is generic because the ear trainer needs precisely this, for
// precisely this reason, and used to hold a line-for-line copy of it that had
// already fallen behind — no Forget, no configurable limits. The reasoning
// above is the whole justification for both, so it lives with the one
// implementation rather than in a comment pointing at another comment.

const (
	pendingTTL        = 2 * time.Hour
	maxPendingPerUser = 400
)

type pendingEntry[T any] struct {
	item   T
	served time.Time
}

// Served holds items that have been handed out and not yet answered.
//
// The two functions are supplied by the owning package because only it knows
// which field is the answer: withID returns a copy stamped with a fresh id,
// strip returns the answer-free copy that goes on the wire.
type Served[T any] struct {
	mu     sync.Mutex
	byUser map[int64]map[string]pendingEntry[T]
	ttl    time.Duration
	max    int
	withID func(T, string) T
	strip  func(T) T
}

// NewServed builds an empty store.
func NewServed[T any](withID func(T, string) T, strip func(T) T) *Served[T] {
	return &Served[T]{
		byUser: map[int64]map[string]pendingEntry[T]{},
		ttl:    pendingTTL,
		max:    maxPendingPerUser,
		withID: withID,
		strip:  strip,
	}
}

// Pending holds drill items that have been served and not yet answered.
type Pending = Served[Item]

// NewPending builds an empty store.
func NewPending() *Pending {
	return NewServed(
		func(it Item, id string) Item { it.ID = id; return it },
		func(it Item) Item { return it.WithExpected("") },
	)
}

// newItemID returns an unguessable id. crypto/rand rather than math/rand: ids
// are the only thing standing between a served item and its answer.
func newItemID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Put stores items and stamps each with a fresh id, returning them ready to send.
func (p *Served[T]) Put(userID int64, items []T) ([]T, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	bucket := p.byUser[userID]
	if bucket == nil {
		bucket = map[string]pendingEntry[T]{}
		p.byUser[userID] = bucket
	}
	now := time.Now()
	p.evictLocked(bucket, now)

	out := make([]T, 0, len(items))
	for _, it := range items {
		id, err := newItemID()
		if err != nil {
			return nil, err
		}
		it = p.withID(it, id)
		bucket[id] = pendingEntry[T]{item: it, served: now}
		// The copy handed to the caller must not carry the answer, even though
		// the field is unexported: the JSON encoder cannot see it, and this makes
		// that guarantee explicit rather than incidental.
		out = append(out, p.strip(it))
	}
	return out, nil
}

// Take removes and returns a served item. The second result is false when the
// id is unknown or has expired.
func (p *Served[T]) Take(userID int64, id string) (T, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var zero T
	bucket := p.byUser[userID]
	if bucket == nil {
		return zero, false
	}
	e, ok := bucket[id]
	if !ok {
		return zero, false
	}
	delete(bucket, id)
	if time.Since(e.served) > p.ttl {
		return zero, false
	}
	return e.item, true
}

// Forget drops every pending item for a user, used when a session is abandoned.
func (p *Served[T]) Forget(userID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.byUser, userID)
}

// evictLocked drops expired entries, and the oldest entries beyond the cap.
//
// The cap matters: a client that fetches items in a loop without answering them
// would otherwise grow this map without bound.
func (p *Served[T]) evictLocked(bucket map[string]pendingEntry[T], now time.Time) {
	for id, e := range bucket {
		if now.Sub(e.served) > p.ttl {
			delete(bucket, id)
		}
	}
	for len(bucket) > p.max {
		oldestID, oldest := "", now
		for id, e := range bucket {
			if oldestID == "" || e.served.Before(oldest) {
				oldestID, oldest = id, e.served
			}
		}
		delete(bucket, oldestID)
	}
}
