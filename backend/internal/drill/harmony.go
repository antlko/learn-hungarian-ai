// Package drill generates and grades grammar exercises, and owns the
// blocked -> interleaved gate that decides which of them a learner sees.
//
// It is the one package in the app that deliberately contradicts the source
// brief. The brief asked for interleaved practice from the moment a topic is
// introduced; the evidence says that is precisely wrong for an explicit,
// rule-based distinction with no L1 analogue, taught early. See
// .claude/research/05-interleaving.md, and do not "fix" it back.
package drill

import "strings"

// Hungarian vowel harmony, as an executable rule.
//
// [R15] — .claude/research/15-hungarian-difficulty.md — makes the case that
// harmony is a MOTOR SKILL rather than knowledge: understood in twenty minutes,
// automated in about three months. Two consequences shape this file.
//
//  1. The classifier exists so the app can generate unlimited allomorph-choice
//     items on demand. Volume is the whole treatment, and volume cannot come out
//     of a fixed exercise bank.
//  2. Mastery of a motor skill is measured in LATENCY. A learner who picks the
//     right allomorph correctly but slowly has not automated it, and calling that
//     mastered would stop the practice exactly when it is starting to work.
//
// This mirrors langpack/pipeline/harmony.py, which does the same job at build
// time for generated forms. The two are tested against the same cases; if you
// change one, change the other.

// Class is the harmony class of a stem: which allomorph it selects.
type Class string

const (
	// Back: a á o ó u ú decide the word. -ban, -hoz, -tól.
	Back Class = "back"
	// FrontUnrounded: e é i í. -ben, -hez, -től.
	FrontUnrounded Class = "front_unrounded"
	// FrontRounded: ö ő ü ű. -ben, but -höz.
	FrontRounded Class = "front_rounded"
)

// IsFront reports whether the class takes front suffixes.
func (c Class) IsFront() bool { return c != Back }

var (
	backVowels           = map[rune]bool{'a': true, 'á': true, 'o': true, 'ó': true, 'u': true, 'ú': true}
	frontRoundedVowels   = map[rune]bool{'ö': true, 'ő': true, 'ü': true, 'ű': true}
	frontUnroundedVowels = map[rune]bool{'e': true, 'é': true, 'i': true, 'í': true}

	// Transparent to ROUNDING harmony only. `i`/`í` do not decide whether a
	// front word takes -hez or -höz, so we look past them to the vowel that
	// does; `e`/`é` are not transparent here and do select the unrounded form.
	// This is the rule as it is taught, and a simplification of the full
	// phonological picture — attested forms in the pack always win over it.
	roundingTransparent = map[rune]bool{'i': true, 'í': true}
)

// antiharmonic stems look front but take back suffixes: híd -> hídban.
//
// A genuinely closed, memorisable list — learners are taught it as a list, so it
// is stored as one rather than dressed up as a rule. Far from exhaustive; it only
// governs forms we GENERATE, and an attested form from the pack overrides it.
var antiharmonic = map[string]bool{
	"híd": true, "cél": true, "derék": true, "férj": true, "héj": true,
	"nyíl": true, "sír": true, "szíj": true, "víz": true, "zsír": true,
	"cím": true, "ír": true, "híg": true, "nyit": true, "szid": true,
}

func isVowel(r rune) bool {
	return backVowels[r] || frontRoundedVowels[r] || frontUnroundedVowels[r]
}

// Vowels returns a word's vowels in order.
func Vowels(word string) []rune {
	var out []rune
	for _, r := range strings.ToLower(word) {
		if isVowel(r) {
			out = append(out, r)
		}
	}
	return out
}

// Classify returns the harmony class of a stem.
//
// The order of the tests is the order the exceptions actually apply:
//
//  1. A known antiharmonic stem is back despite its front vowels.
//  2. Otherwise any back vowel makes the word back. Neutral vowels never
//     override a back one: `papír` takes -ban, not -ben.
//  3. Otherwise the word is front, and rounded-vs-unrounded is decided by the
//     LAST non-transparent front vowel — the one nearest the suffix. This is why
//     `könyv` takes -höz while its plural `könyvek` takes -hez.
//  4. No vowels at all (an acronym, a fragment) defaults to front unrounded,
//     which is the less jarring guess.
func Classify(word string) Class {
	lowered := strings.ToLower(strings.TrimSpace(word))
	if antiharmonic[lowered] {
		return Back
	}
	vs := Vowels(lowered)
	if len(vs) == 0 {
		return FrontUnrounded
	}
	for _, v := range vs {
		if backVowels[v] {
			return Back
		}
	}
	for i := len(vs) - 1; i >= 0; i-- {
		v := vs[i]
		if roundingTransparent[v] {
			continue
		}
		if frontRoundedVowels[v] {
			return FrontRounded
		}
		return FrontUnrounded
	}
	// Nothing but transparent vowels (`ing`): unrounded.
	return FrontUnrounded
}

// Pick selects an allomorph for a class from the pack's allomorph map, which is
// keyed by the class names above.
//
// A two-form suffix (-ban/-ben) stores the same string under both front keys,
// which is exactly how Hungarian behaves: `köd` -> `ködben`, not a third variant.
func Pick(allomorphs map[string]string, c Class) string {
	if s, ok := allomorphs[string(c)]; ok && s != "" {
		return s
	}
	// A pack that omits the front-rounded key still gets sane behaviour.
	if c == FrontRounded {
		return allomorphs[string(FrontUnrounded)]
	}
	return ""
}

// Distinct returns the distinct allomorphs of a feature, in a stable order:
// back, front unrounded, front rounded. An item can only be an allomorph CHOICE
// if there are at least two.
func Distinct(allomorphs map[string]string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 3)
	for _, c := range []Class{Back, FrontUnrounded, FrontRounded} {
		s := Pick(allomorphs, c)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// EndsInVowel reports whether a stem ends in a vowel, which decides whether a
// linking vowel is needed before a consonant-initial suffix.
func EndsInVowel(word string) bool {
	r := []rune(strings.ToLower(word))
	return len(r) > 0 && isVowel(r[len(r)-1])
}

// LengthenFinalVowel applies low-vowel lengthening: alma + -t -> almát.
//
// One of the first things a Russian or English speaker gets wrong, because the
// change is inaudible to an ear with no phonemic length contrast. See [R7].
func LengthenFinalVowel(word string) string {
	r := []rune(word)
	if len(r) == 0 {
		return word
	}
	switch r[len(r)-1] {
	case 'a':
		r[len(r)-1] = 'á'
	case 'e':
		r[len(r)-1] = 'é'
	}
	return string(r)
}

// Automaticity is the motor-skill mastery test.
//
// Accuracy alone is not enough for harmony, and this is the whole of [R15]'s
// second point: the bottleneck is repetition volume, not comprehension, so the
// observable target is SPEED. `medianMs` is the streaming median of the
// learner's correct answers; `latencyBudgetMs` is `drill.harmonyLatencyMs`,
// default 2500, tunable because nobody has published a defensible number for it.
type Automaticity struct {
	Fast      bool  `json:"fast"`
	Accurate  bool  `json:"accurate"`
	Enough    bool  `json:"enough"`
	Automated bool  `json:"automated"`
	MedianMs  int64 `json:"medianMs"`
	BudgetMs  int   `json:"budgetMs"`
}

// MeasureAutomaticity reports whether a motor skill has been automated.
//
// A zero median means the learner has never answered one of these correctly, and
// that is NOT the same as being fast — so it fails the test rather than passing
// it by accident.
func MeasureAutomaticity(medianMs int64, accuracy float64, observations, window, latencyBudgetMs int, masteryThreshold float64) Automaticity {
	a := Automaticity{MedianMs: medianMs, BudgetMs: latencyBudgetMs}
	a.Fast = medianMs > 0 && medianMs <= int64(latencyBudgetMs)
	a.Accurate = accuracy >= masteryThreshold
	a.Enough = observations >= window
	a.Automated = a.Fast && a.Accurate && a.Enough
	return a
}
