package tutor

import (
	"sort"
	"strings"
	"unicode"
)

// The honesty check on the model's Hungarian.
//
// The tutor is asked to put every Hungarian word in backticks. Each one is
// looked up in the pack; the ones the dictionary does not carry are returned
// so the UI can list them under the answer as *not found in the dictionary* —
// the same `uncertain` treatment an uncertain analysis gets. This is not the
// corpus attestation of ADR-027's future generation path: it cannot say a form
// is RIGHT in context, only that the dictionary has never seen it. That is the
// one thing a weak model gets wrong most often, and the one thing a learner
// cannot check for themselves.

// Backticked returns the distinct words inside `…` spans, in first-seen
// order. A span with spaces is a phrase; each of its words is checked
// separately, because a phrase is attested word by word — and a whole
// translated sentence (ADR-030) is a span like any other, so a stem the model
// invented inside it is listed under the answer. Only a multi-line span or
// one past 200 bytes is skipped, as a code block rather than words; the cap
// was 80, which let every translated sentence through unchecked. Punctuation
// around a word is dropped; a hyphen-led suffix (`-ban`) is kept as written
// and is never looked up — it is a suffix, not a word.
func Backticked(s string) []string {
	var out []string
	seen := map[string]bool{}
	for {
		start := strings.IndexByte(s, '`')
		if start < 0 {
			break
		}
		end := strings.IndexByte(s[start+1:], '`')
		if end < 0 {
			break
		}
		span := s[start+1 : start+1+end]
		s = s[start+1+end+1:]
		if strings.Contains(span, "\n") || len(span) > 200 {
			continue // a code block, not words
		}
		for _, w := range strings.FieldsFunc(span, func(r rune) bool {
			return !unicode.IsLetter(r) && r != '-' && r != '\''
		}) {
			w = strings.Trim(w, "-'")
			if w == "" || !hasLetter(w) {
				continue
			}
			key := strings.ToLower(w)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, w)
		}
	}
	return out
}

// Unattested filters words down to the ones `exists` does not know. Words with
// no Hungarian-specific letters AND a Cyrillic or obviously foreign shape are
// still checked — the model was told to backtick Hungarian only, and a Russian
// word in backticks is itself worth flagging.
//
// The result is never nil. An answer with no backticked word at all — the
// commonest kind from a small model — once travelled on the `done` event as
// `"unattested": null`, and the panel's render threw on `.length` and froze
// mid-answer until a reload read the stored copy back as `[]`. The list is
// a list, empty or not.
func Unattested(words []string, exists func(string) bool) []string {
	out := []string{}
	for _, w := range words {
		if isSuffixLike(w) {
			continue
		}
		if !exists(w) {
			out = append(out, w)
		}
	}
	sort.Strings(out)
	return out
}

// isSuffixLike: `ban`, `ből`, `nak` alone are suffixes the model wrote without
// the hyphen; the pack has no wordform for them and flagging them would be
// noise on every answer about cases. Three letters or fewer, all lowercase,
// and a known suffix shape.
func isSuffixLike(w string) bool {
	if len([]rune(w)) > 4 || w != strings.ToLower(w) {
		return false
	}
	_, ok := suffixShapes[w]
	return ok
}

var suffixShapes = map[string]struct{}{
	"ban": {}, "ben": {}, "ba": {}, "be": {}, "ból": {}, "ből": {}, "on": {}, "en": {}, "ön": {},
	"ra": {}, "re": {}, "ról": {}, "ről": {}, "nál": {}, "nél": {}, "hoz": {}, "hez": {}, "höz": {},
	"tól": {}, "től": {}, "nak": {}, "nek": {}, "val": {}, "vel": {}, "ért": {}, "ig": {}, "vá": {},
	"vé": {}, "kor": {}, "ként": {}, "ul": {}, "ül": {}, "ok": {}, "ek": {}, "ök": {}, "ak": {},
	"om": {}, "em": {}, "öm": {}, "od": {}, "ed": {}, "öd": {}, "uk": {}, "ük": {}, "juk": {},
	"jük": {}, "lak": {}, "lek": {}, "ni": {}, "hat": {}, "het": {}, "nék": {}, "tat": {}, "tet": {},
	"gat": {}, "get": {}, "va": {}, "ve": {}, "ván": {}, "vén": {}, "bb": {}, "abb": {}, "ebb": {},
	"nként": {}, "szor": {}, "szer": {}, "ször": {}, "ás": {}, "és": {}, "ság": {}, "ség": {},
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}
