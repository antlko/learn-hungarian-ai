package srs

import (
	"strconv"
	"strings"
)

// Scope selects which part of the deck a queue is built from.
//
// The two piles answer different questions. The course deck is a syllabus
// obligation the plan screen reports on; the reader deck is what the learner
// mined from their own reading. Merged, "clear today's course work" is
// impossible without also clearing every word you ever clicked in a text.
//
// This is a FILTER, not a second scheduler: one FSRS state space, one daily
// new-card budget shared across scopes, and no stored preference.
// See ADR-016.
type Scope string

const (
	ScopeAll    Scope = "all"
	ScopeCourse Scope = "course"
	ScopeReader Scope = "reader"
	// ScopeTrash is the words the learner set aside. It is NOT a fourth pile:
	// `all == course + reader` still holds, because all three exclude it.
	//
	// The trash is PRACTISEABLE and never SCHEDULABLE. Free practice accepts
	// it — a word you set aside is still a word you may want to look at, and
	// practice schedules nothing (ADR-019), so nothing about the trashed card
	// moves. `/api/review/queue` refuses it outright: a scope that fed the
	// scheduler would make the trash a place cards come back FROM, which is
	// the opposite of what the button promised. Getting a word back into
	// review is one explicit action — restore.
	ScopeTrash Scope = "trash"
)

// Scopes in the order the UI draws them. The trash is absent on purpose: it has
// its own screen, and a tab strip whose numbers must add up cannot carry a pile
// that is outside the arithmetic.
var Scopes = []Scope{ScopeAll, ScopeCourse, ScopeReader}

// ParseScope maps a query parameter to a Scope.
//
// Blank is ScopeAll, because a client that has never heard of scopes must keep
// working. Anything else unknown is REFUSED rather than quietly widened to the
// whole deck: a typo that returns everything looks exactly like the feature
// working, and would be found only by counting cards.
//
// `text:<id>` selects the words mined from ONE text — a SUB-slice of the
// reader pile, not a fourth one (ADR-022): `all == course + reader` keeps
// holding, and the daily new budget stays global (ADR-016). The id is parsed
// strictly and canonicalised, which is what lets filter() below inline it.
//
// `trash` parses here but is refused by the review queue, not by this function:
// practice accepts it and the two callers must not share a whitelist that would
// have to be right in two different ways.
func ParseScope(s string) (Scope, bool) {
	switch Scope(s) {
	case "":
		return ScopeAll, true
	case ScopeAll, ScopeCourse, ScopeReader, ScopeTrash:
		return Scope(s), true
	}
	if rest, ok := strings.CutPrefix(s, "text:"); ok {
		id, err := strconv.ParseInt(rest, 10, 64)
		if err != nil || id <= 0 {
			return "", false
		}
		return Scope("text:" + strconv.FormatInt(id, 10)), true
	}
	return "", false
}

// TextID returns the id of a text scope, or false for the fixed three.
func (s Scope) TextID() (int64, bool) {
	rest, ok := strings.CutPrefix(string(s), "text:")
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// IsText reports whether this is a per-text scope.
func (s Scope) IsText() bool { _, ok := s.TextID(); return ok }

// filter restricts a `card c JOIN note n` query to the scope.
//
// **Every scope carries the trash clause**, and that is why it lives here
// rather than at the call sites: a query that forgot it would serve a learner
// the exact cards they asked never to see again, and would do it silently. The
// three normal scopes exclude the trash; `trash` is the one that selects it.
//
// `reader` is the COMPLEMENT of `course`, not `source = 'manual'`. A third
// source added later — an LLM tutor, an import — would otherwise be invisible
// under every tab, and the `all == course + reader` invariant would stop
// holding without anything failing. The literal carries no bound arguments, so
// callers' `args...` threading is unaffected — for the text scope too: the id
// was strictly parsed and canonicalised by ParseScope (an int64 round-tripped
// through ParseInt/FormatInt), so inlining it cannot inject anything. Pinned
// by TestParseScopeRejectsMalformedTextIds.
func (s Scope) filter() string {
	switch s {
	case ScopeTrash:
		return ` AND n.trashed_at IS NOT NULL`
	case ScopeCourse:
		return NotTrashed + ` AND n.source LIKE 'course:%'`
	case ScopeReader:
		return NotTrashed + ` AND n.source NOT LIKE 'course:%'`
	}
	if id, ok := s.TextID(); ok {
		return NotTrashed + ` AND n.id IN (SELECT note_id FROM note_origin WHERE text_id = ` +
			strconv.FormatInt(id, 10) + `)`
	}
	return NotTrashed
}

// NotTrashed is the clause every deck query needs, exported for the handful
// that cannot go through filter() — the two-pile breakdown pass, the forecast,
// the words browser. One string, so "is this row in the deck" has one answer.
const NotTrashed = ` AND n.trashed_at IS NULL`

// Per-scope numbers live on Breakdown (breakdown.go, ADR-021): the ScopeCount
// that used to be here reported raw pre-burial card counts, which is how the
// tab strip came to promise 26 items over a 10-item session.
