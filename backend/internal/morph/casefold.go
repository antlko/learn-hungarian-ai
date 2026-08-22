package morph

import (
	"strings"
	"unicode"
)

// Case-insensitive lookup that still uses the index.
//
// The obvious spelling — `WHERE form = ? COLLATE NOCASE` — silently defeats
// `idx_wordform_form`, which is a BINARY-collated index: SQLite falls back to
// scanning all two million rows of the covering index, about 110 ms per lookup.
// The reader analyses a few hundred distinct forms per article and the drill
// generator checks attestation dozens of times per request, so that is the
// difference between an instant screen and a stalled one.
//
// `WHERE form IN (?,?,?)` over the plausible casings is an indexed SEARCH and
// gives the same answers for real text: Hungarian words appear lowercase, at the
// start of a sentence, or shouted.

// caseVariants returns the spellings of a form worth looking up, deduplicated
// and in most-likely-first order.
func caseVariants(form string) []string {
	out := []string{form}
	seen := map[string]bool{form: true}
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	lower := strings.ToLower(form)
	add(lower)

	// Sentence-initial capital: Ház, Látom. Note ToUpper on the first RUNE, so
	// the Hungarian accented letters fold correctly — ToTitle on the whole
	// string would not.
	r := []rune(lower)
	if len(r) > 0 {
		r[0] = unicode.ToUpper(r[0])
		add(string(r))
	}
	return out
}

// placeholders renders "?,?,?" for an IN clause of n values, and the values as
// a []any ready to splat into Query.
func placeholders(values []string) (string, []any) {
	args := make([]any, len(values))
	for i, v := range values {
		args[i] = v
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(values)), ","), args
}
