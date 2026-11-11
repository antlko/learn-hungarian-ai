package api

import (
	"database/sql"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"tanulom/internal/morph"
	"tanulom/internal/srs"
)

// Answer options for review cards (ADR-060).
//
// Typing is awkward on a phone, so a cloze or produce card may ship 3-5
// candidate strings and let the learner tap one. This is a different way to
// SUBMIT an answer, never a different way to grade it: the chosen string goes
// through the ordinary gradeTyped, against the string expectedFor already
// decided. Everything marked right here is the graded string; the distractors
// are real, attested forms or dictionary lemmas, so a wrong option is a word
// the learner could meet, not a typo.
//
// Recognition is weaker than recall [R12]; the client offers choice on touch
// devices only and typing stays the default elsewhere.

const (
	minOptions = 3
	maxOptions = 5
	// nearForms is how many same-lemma forms, closest in suffix, are always
	// preferred before the random remainder.
	nearForms = 2
)

// attachOptions fills QueueItem.Options on cloze and produce cards. It never
// fails a queue: a card whose options cannot be built simply has none.
func (s *Server) attachOptions(items []srs.QueueItem, now time.Time) {
	if s.pack == nil || s.analyzer == nil {
		return
	}
	for i := range items {
		it := &items[i]
		if it.Direction != srs.Cloze && it.Direction != srs.Produce {
			continue
		}
		if opts := s.buildOptions(*it, now); len(opts) >= minOptions {
			it.Options = opts
		}
	}
}

// optionSeed is deterministic per (card, local day): a reload shows the same
// options in the same order, tomorrow shows another draw.
func optionSeed(cardID int64, now time.Time) *rand.Rand {
	h := fnv.New64a()
	fmt.Fprintf(h, "%d|%d|%d|%d", cardID, now.Year(), int(now.Month()), now.Day())
	v := h.Sum64()
	return rand.New(rand.NewPCG(v, v^0x9e3779b97f4a7c15))
}

// graderAccepts reports whether the grader would accept cand for expected,
// including the accent-only tolerance, whatever the setting says: an option
// the grader might also take is an ambiguous question.
func graderAccepts(cand, expected string) bool {
	d := morph.DiffForms(cand, expected, nil)
	return d.Match || d.AccentOnly()
}

// buildOptions returns the shuffled options for one card, or nil.
func (s *Server) buildOptions(it srs.QueueItem, now time.Time) []string {
	expected := strings.TrimSpace(it.Expected)
	if expected == "" || it.LemmaID == nil || *it.LemmaID == 0 {
		return nil
	}
	// A produce card with no resolved dictionary form is graded against the
	// note's surface form: not something we can build a dictionary-form
	// choice around.
	if it.Direction == srs.Produce && it.Lemma == "" {
		return nil
	}
	rng := optionSeed(it.CardID, now)

	var distractors []string
	switch it.Direction {
	case srs.Cloze:
		distractors = s.clozeDistractors(*it.LemmaID, expected, rng)
	case srs.Produce:
		distractors = s.produceDistractors(*it.LemmaID, expected, rng)
	}
	if len(distractors) == 0 {
		return nil
	}
	if len(distractors) > maxOptions-1 {
		distractors = distractors[:maxOptions-1]
	}
	// One casing for the whole card: every option in the lemma's own case, so
	// no option is marked out by a sentence-initial capital (ADR-071). A form
	// that collapses onto one already taken is dropped, which keeps them unique.
	opts := make([]string, 0, maxOptions)
	taken := map[string]bool{}
	for _, o := range append([]string{expected}, distractors...) {
		o = lemmaCase(o, it.Lemma)
		if k := strings.ToLower(o); !taken[k] {
			taken[k] = true
			opts = append(opts, o)
		}
	}
	if len(opts) < minOptions {
		return nil
	}
	rng.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })
	return opts
}

// lemmaCase shows a form in its lemma's own case: lowercase unless the
// dictionary form is itself capitalised (Budapest, Magyarország). A form at the
// head of a sentence carries the SENTENCE's capital, which is not part of the
// word and, among options, would mark the answer. Only the first letter is
// touched — that is the only one a sentence can change. An unresolved lemma
// leaves the form as it is: nothing to compare against.
func lemmaCase(form, lemma string) string {
	lr, _ := utf8.DecodeRuneInString(lemma)
	fr, n := utf8.DecodeRuneInString(form)
	if lemma == "" || form == "" || unicode.IsUpper(lr) || !unicode.IsUpper(fr) {
		return form
	}
	return string(unicode.ToLower(fr)) + form[n:]
}

// admit appends cand when it is usable and not already taken.
func admit(out []string, seen map[string]bool, cand, expected string) []string {
	k := strings.ToLower(strings.TrimSpace(cand))
	if k == "" || seen[k] || graderAccepts(cand, expected) {
		return out
	}
	seen[k] = true
	return append(out, cand)
}

func commonPrefixRunes(a, b string) int {
	n := 0
	for len(a) > 0 && len(b) > 0 {
		ra, sa := utf8.DecodeRuneInString(a)
		rb, sb := utf8.DecodeRuneInString(b)
		if ra != rb {
			break
		}
		n++
		a, b = a[sa:], b[sb:]
	}
	return n
}

// clozeDistractors: other ATTESTED wordforms of the same lemma (different
// case, number, person, tense...), resolved through morph.FormsOf. The two
// closest in suffix come first, then a seeded draw of the rest. A lemma with
// fewer than two such forms is topped up with same-POS neighbours.
func (s *Server) clozeDistractors(lemmaID int64, expected string, rng *rand.Rand) []string {
	forms, err := s.analyzer.FormsOf(lemmaID)
	if err != nil {
		return nil
	}
	seen := map[string]bool{strings.ToLower(expected): true}
	var cands []string
	for _, f := range forms {
		if !f.Attested() {
			continue
		}
		cands = admit(cands, seen, f.Form, expected)
	}
	sort.Strings(cands) // stable base order before any seeded choice
	exp := strings.ToLower(expected)
	suffixDiff := func(c string) int {
		p := commonPrefixRunes(strings.ToLower(c), exp)
		return utf8.RuneCountInString(c) + utf8.RuneCountInString(exp) - 2*p
	}
	sort.SliceStable(cands, func(i, j int) bool { return suffixDiff(cands[i]) < suffixDiff(cands[j]) })

	var out []string
	near := min(nearForms, len(cands))
	out = append(out, cands[:near]...)
	rest := append([]string(nil), cands[near:]...)
	rng.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	out = append(out, rest...)
	if len(out) > maxOptions-1 {
		out = out[:maxOptions-1]
	}
	if len(out) >= minOptions-1 {
		return out
	}
	// Too few forms: same-POS neighbours by frequency rank. Their dictionary
	// forms are attested words, just not forms of this lemma.
	for _, n := range s.neighbourLemmas(lemmaID, expected, "", rng) {
		if len(out) >= maxOptions-1 {
			break
		}
		out = admit(out, seen, n, expected)
	}
	return out
}

// produceDistractors: same-POS lemmas close in frequency rank and similar in
// length, never a synonym of the target (same first-sense gloss).
func (s *Server) produceDistractors(lemmaID int64, expected string, rng *rand.Rand) []string {
	seen := map[string]bool{strings.ToLower(expected): true}
	var out []string
	for _, n := range s.neighbourLemmas(lemmaID, expected, expected, rng) {
		if len(out) >= maxOptions-1 {
			break
		}
		out = admit(out, seen, n, expected)
	}
	return out
}

// neighbourLemmas lists dictionary forms of same-POS lemmas near the target in
// frequency rank, with similar-length ones preferred, in a seeded order.
// When excludeGlossOf is non-empty, lemmas sharing a first-sense gloss with the
// target are dropped (they would be a second correct answer).
func (s *Server) neighbourLemmas(lemmaID int64, expected, excludeGlossOf string, rng *rand.Rand) []string {
	db := s.pack.DB()
	var upos string
	var rank sql.NullInt64
	if err := db.QueryRow(`SELECT upos, freq_rank FROM lemma WHERE id = ?`, lemmaID).Scan(&upos, &rank); err != nil {
		return nil
	}
	targetGloss := map[string]bool{}
	if excludeGlossOf != "" {
		rows, err := db.Query(`SELECT lang, gloss FROM lemma_gloss WHERE lemma_id = ? AND sense_idx = 0`, lemmaID)
		if err == nil {
			for rows.Next() {
				var lang, g string
				if rows.Scan(&lang, &g) == nil {
					targetGloss[lang+"|"+strings.ToLower(strings.TrimSpace(g))] = true
				}
			}
			rows.Close()
		}
	}

	var rows *sql.Rows
	var err error
	const limit = 40
	if rank.Valid {
		rows, err = db.Query(`
			SELECT l.id, l.lemma FROM lemma l
			 WHERE l.upos = ? AND l.id != ? AND l.freq_rank IS NOT NULL
			 ORDER BY ABS(l.freq_rank - ?), l.id LIMIT ?`, upos, lemmaID, rank.Int64, limit)
	} else {
		rows, err = db.Query(`
			SELECT l.id, l.lemma FROM lemma l
			 WHERE l.upos = ? AND l.id != ? AND l.freq_rank IS NOT NULL
			 ORDER BY l.freq_rank, l.id LIMIT ?`, upos, lemmaID, limit)
	}
	if err != nil {
		return nil
	}
	type cand struct {
		id    int64
		lemma string
	}
	var cands []cand
	for rows.Next() {
		var c cand
		if rows.Scan(&c.id, &c.lemma) == nil && !strings.ContainsAny(c.lemma, " -") {
			cands = append(cands, c)
		}
	}
	rows.Close()

	if len(targetGloss) > 0 {
		kept := cands[:0]
		for _, c := range cands {
			if s.sharesGloss(c.id, targetGloss) {
				continue
			}
			kept = append(kept, c)
		}
		cands = kept
	}

	// Similar length first (within two letters), each band in seeded order.
	want := utf8.RuneCountInString(expected)
	var near, far []string
	for _, c := range cands {
		d := utf8.RuneCountInString(c.lemma) - want
		if d >= -2 && d <= 2 {
			near = append(near, c.lemma)
		} else {
			far = append(far, c.lemma)
		}
	}
	shuffle := func(x []string) {
		rng.Shuffle(len(x), func(i, j int) { x[i], x[j] = x[j], x[i] })
	}
	// Only the closest-in-rank handful of the near band is shuffled, so the
	// options stay "plausible neighbours", not a random draw from the dictionary.
	if len(near) > 12 {
		near = near[:12]
	}
	shuffle(near)
	shuffle(far)
	return append(near, far...)
}

func (s *Server) sharesGloss(lemmaID int64, target map[string]bool) bool {
	rows, err := s.pack.DB().Query(`SELECT lang, gloss FROM lemma_gloss WHERE lemma_id = ? AND sense_idx = 0`, lemmaID)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var lang, g string
		if rows.Scan(&lang, &g) == nil && target[lang+"|"+strings.ToLower(strings.TrimSpace(g))] {
			return true
		}
	}
	return false
}

// Answer channel ("via").

const (
	viaTyped  = "typed"
	viaChoice = "choice"
)

// checkVia validates the optional `via` field against the card's direction.
// It is recorded nowhere that affects scheduling or replay.
func checkVia(req AnswerRequest, d srs.Direction) error {
	switch req.Via {
	case "", viaTyped:
		return nil
	case viaChoice:
		if d != srs.Cloze && d != srs.Produce {
			return badRequest("via=choice is only for cloze and produce cards")
		}
		return nil
	}
	return badRequest("via must be typed or choice")
}
