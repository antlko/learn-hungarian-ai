package morph

import (
	"encoding/json"
	"strings"
	"unicode"
)

// Morpheme-aware answer diffing.
//
// "Wrong" is almost useless feedback. What a learner needs is *which part* was
// wrong and *why*, because the remedy differs completely: a wrong harmony
// variant needs volume, a wrong case needs the rule restated, and a dropped
// vowel length needs ear training rather than more writing.
//
// Corrective feedback that names the error and prompts a repair outperforms
// simply supplying the answer, so the diff feeds both the display and the
// feature attribution that decides what gets drilled next. See [R9], [R15].

// ErrorClass names the kind of mistake, which is what drives the follow-up.
type ErrorClass string

const (
	NoError        ErrorClass = ""
	ErrHarmony     ErrorClass = "harmony"      // right suffix, wrong allomorph
	ErrVowelLength ErrorClass = "vowel_length" // hat/hát — phonemic, needs the ear
	// ErrVowelQuality is rounding: tor/tör. The front rounded vowels have no
	// counterpart in Russian or English at all, so this is a perception problem
	// before it is a writing problem. Separate from length because it routes to
	// a different minimal-pair contrast in the ear trainer.
	ErrVowelQuality ErrorClass = "vowel_quality"
	ErrCase         ErrorClass = "case"
	ErrDefinite     ErrorClass = "definiteness"
	ErrPossessor    ErrorClass = "possessor"
	ErrStem         ErrorClass = "stem" // wrong word entirely
	ErrSpelling     ErrorClass = "spelling"
	ErrUnknown      ErrorClass = "unknown"
)

// Diff is the comparison of a typed answer against the target.
type Diff struct {
	Typed    string `json:"typed"`
	Expected string `json:"expected"`
	Match    bool   `json:"match"`
	// NearMatch means a slip rather than a knowledge failure — a doubled letter
	// or a transposition. Rated Hard rather than Again.
	NearMatch bool       `json:"nearMatch"`
	Class     ErrorClass `json:"class"`
	// WrongFeature names the single feature to blame, when the error can be
	// attributed precisely. Empty means the failure was not localisable, and the
	// caller must not invent an attribution.
	WrongFeature string `json:"wrongFeature,omitempty"`
	// CommonPrefix/Suffix let the UI underline exactly the differing span.
	CommonPrefix int `json:"commonPrefix"`
	CommonSuffix int `json:"commonSuffix"`
	// Explanation is a one-line, locale-independent hint key plus its arguments;
	// the frontend renders it. Keeping prose out of the backend keeps both
	// locales honest.
	Hint     string            `json:"hint,omitempty"`
	HintArgs map[string]string `json:"hintArgs,omitempty"`
}

// normalise lowercases and trims for comparison. It deliberately does NOT strip
// accents: in Hungarian vowel length is phonemic, so `hat` and `hát` are
// different words and treating them as equal would teach the learner nothing.
func normalise(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// Vowel folding, used only to DETECT an error, never to accept one.
//
// Two separate folds because they are two separate mistakes with two separate
// remedies, even though both route to ear training:
//
//   - LENGTH: hat/hát, tör/tőr. Phonemic in Hungarian, absent from Russian and
//     English, and by far the highest-frequency error.
//   - QUALITY (rounding): tor/tör, tud/tüd. The front rounded vowels have no
//     counterpart in either L1 at all.
//
// `tor` vs `tőr` differs in BOTH, which is why it is not a length error.
var lengthPairs = map[rune]rune{
	'á': 'a', 'é': 'e', 'í': 'i', 'ó': 'o', 'ő': 'ö', 'ú': 'u', 'ű': 'ü',
}

var qualityPairs = map[rune]rune{
	'ö': 'o', 'ő': 'o', 'ü': 'u', 'ű': 'u', 'ó': 'o', 'ú': 'u',
	'á': 'a', 'é': 'e', 'í': 'i',
}

func fold(s string, table map[rune]rune) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if mapped, ok := table[r]; ok {
			b.WriteRune(mapped)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func stripLength(s string) string { return fold(s, lengthPairs) }
func stripVowels(s string) string { return fold(s, qualityPairs) }

// DiffForms compares a typed answer with the expected form.
//
// The analyzer is optional: without a language pack the diff still reports
// character-level differences and vowel-length errors, it simply cannot name a
// feature. Degrading rather than failing keeps the review loop working before a
// pack is installed.
func DiffForms(typed, expected string, a *Analyzer) Diff {
	t, e := normalise(typed), normalise(expected)
	d := Diff{Typed: typed, Expected: expected}

	if t == e {
		d.Match = true
		return d
	}
	if t == "" {
		d.Class = ErrUnknown
		return d
	}

	d.CommonPrefix, d.CommonSuffix = commonAffixes(t, e)

	// Vowel length first: it is the highest-frequency error for a learner whose
	// L1 has no phonemic length, and its remedy is ear training rather than more
	// writing, so it must not be mislabelled as a spelling slip.
	if stripLength(t) == stripLength(e) {
		d.Class = ErrVowelLength
		d.Hint = "diff.vowelLength"
		d.HintArgs = map[string]string{"expected": expected}
		return d
	}
	if stripVowels(t) == stripVowels(e) {
		d.Class = ErrVowelQuality
		d.Hint = "diff.vowelQuality"
		d.HintArgs = map[string]string{"expected": expected}
		return d
	}

	// A wrong allomorph of the right suffix: the learner knew the case and got
	// the harmony wrong. Distinguishing this matters because it is a motor-skill
	// failure, not a knowledge gap.
	if a != nil {
		if feature, ok := harmonyMismatch(t, e, a); ok {
			d.Class = ErrHarmony
			d.WrongFeature = feature
			d.Hint = "diff.harmony"
			d.HintArgs = map[string]string{"expected": expected, "feature": feature}
			return d
		}
		if feature, class, ok := featureMismatch(t, e, a); ok {
			d.Class = class
			d.WrongFeature = feature
			d.Hint = "diff.feature"
			d.HintArgs = map[string]string{"expected": expected, "feature": feature}
			return d
		}
		// Different lemma entirely: this is a vocabulary failure, not morphology.
		if differentLemma(t, e, a) {
			d.Class = ErrStem
			d.Hint = "diff.stem"
			d.HintArgs = map[string]string{"expected": expected}
			return d
		}
	}

	// A single-character slip with the stem intact reads as a typo.
	if editDistanceAtMost(t, e, 1) {
		d.NearMatch = true
		d.Class = ErrSpelling
		d.Hint = "diff.spelling"
		d.HintArgs = map[string]string{"expected": expected}
		return d
	}

	d.Class = ErrUnknown
	d.Hint = "diff.generic"
	d.HintArgs = map[string]string{"expected": expected}
	return d
}

// harmonyMismatch reports whether the typed form is the expected form with a
// different allomorph of the same feature substituted.
func harmonyMismatch(typed, expected string, a *Analyzer) (string, bool) {
	for _, e := range a.loadSuffixes() {
		if !strings.HasSuffix(expected, e.suffix) {
			continue
		}
		stem := expected[:len(expected)-len(e.suffix)]
		// Try every other allomorph of the SAME feature.
		for _, alt := range a.allomorphsOf(e.featureID) {
			if alt == e.suffix {
				continue
			}
			if stem+alt == typed {
				return e.featureID, true
			}
		}
	}
	return "", false
}

// featureMismatch reports a differing morphological feature between two forms
// that share a lemma — a wrong case, definiteness or possessor.
func featureMismatch(typed, expected string, a *Analyzer) (string, ErrorClass, bool) {
	ta, err1 := a.Best(typed)
	ea, err2 := a.Best(expected)
	if err1 != nil || err2 != nil {
		return "", NoError, false
	}
	if ta.Certainty == UnknownAnalysis || ea.Certainty == UnknownAnalysis {
		return "", NoError, false
	}
	if ta.LemmaID != ea.LemmaID || ta.LemmaID == 0 {
		return "", NoError, false
	}

	for key, want := range ea.Feats {
		if got, ok := ta.Feats[key]; !ok || got != want {
			return key + "=" + want, classForKey(key), true
		}
	}
	return "", NoError, false
}

func classForKey(key string) ErrorClass {
	switch {
	case key == "Case":
		return ErrCase
	case key == "Definite":
		return ErrDefinite
	case strings.HasPrefix(key, "Person[psor]"), strings.HasPrefix(key, "Number[psor]"),
		strings.HasPrefix(key, "Number[psed]"):
		return ErrPossessor
	default:
		return ErrUnknown
	}
}

func differentLemma(typed, expected string, a *Analyzer) bool {
	ta, _ := a.Best(typed)
	ea, _ := a.Best(expected)
	return ta.Certainty != UnknownAnalysis && ea.Certainty != UnknownAnalysis &&
		ta.LemmaID != 0 && ea.LemmaID != 0 && ta.LemmaID != ea.LemmaID
}

// allomorphsOf returns every surface variant of one feature.
func (a *Analyzer) allomorphsOf(featureID string) []string {
	var raw string
	if err := a.pack.DB().QueryRow(
		`SELECT allomorphs FROM morph_feature WHERE id = ?`, featureID).Scan(&raw); err != nil {
		return nil
	}
	var m map[string]string
	if json.Unmarshal([]byte(raw), &m) != nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(m))
	for _, v := range m {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// commonAffixes returns the shared prefix and suffix lengths in runes, so the UI
// can underline exactly the span that differs.
func commonAffixes(a, b string) (int, int) {
	ar, br := []rune(a), []rune(b)
	p := 0
	for p < len(ar) && p < len(br) && ar[p] == br[p] {
		p++
	}
	s := 0
	for s < len(ar)-p && s < len(br)-p && ar[len(ar)-1-s] == br[len(br)-1-s] {
		s++
	}
	return p, s
}

// editDistanceAtMost reports whether a and b are within `max` edits. Bounded
// rather than exact because we only ever ask "is this a typo".
func editDistanceAtMost(a, b string, max int) bool {
	ar, br := []rune(a), []rune(b)
	if abs(len(ar)-len(br)) > max {
		return false
	}
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		best := cur[0]
		for j := 1; j <= len(br); j++ {
			cost := 1
			if unicode.ToLower(ar[i-1]) == unicode.ToLower(br[j-1]) {
				cost = 0
			}
			cur[j] = min(prev[j]+1, min(cur[j-1]+1, prev[j-1]+cost))
			if cur[j] < best {
				best = cur[j]
			}
		}
		if best > max {
			return false
		}
		prev, cur = cur, prev
	}
	return prev[len(br)] <= max
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
