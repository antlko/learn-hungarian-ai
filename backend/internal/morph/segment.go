package morph

import "strings"

// Morpheme segmentation: `házaimban` → ház + a + i + m + ban.
//
// The product thesis says a wordform is not an atom, and this is where the app
// SHOWS that instead of asserting it. The segmentation drives the reader's
// breakdown panel and the post-answer explanation — explicit instruction in
// miniature, per [R4]: name the feature, name the allomorph rule, show why.
//
// The method is deliberately conservative. We only peel a suffix when the
// analysis says the form CARRIES that feature and the surface string ends in one
// of its allomorphs — so the segmentation can never claim a morpheme the
// analysis did not. Whatever the peel cannot justify is returned as a single
// unlabelled ending segment carrying the leftover features collectively, which
// is honest twice over: the learner sees exactly which part we can explain, and
// a fused portmanteau ending (`látta` = lát + [Tense=Past · Person=3 ·
// Definite=Def]) is genuinely one unsegmentable piece, not three.

// SegmentKind classifies one piece of a form.
type SegmentKind string

const (
	// SegStem is the lemma's surface in this form, including a regular stem
	// alternation such as low-vowel lengthening (alma → almá-).
	SegStem SegmentKind = "stem"
	// SegLinking is a linking vowel: house-plural `házak` = ház + a + k. It
	// belongs to neither the stem nor the suffix and carries no feature.
	SegLinking SegmentKind = "linking"
	// SegSuffix is one identified suffix with its feature and allomorph rule.
	SegSuffix SegmentKind = "suffix"
	// SegEnding is the unanalysed tail: real morphology we could not split.
	// It lists the features it must be carrying, without claiming boundaries.
	SegEnding SegmentKind = "ending"
)

// Segment is one piece of a segmented form.
type Segment struct {
	Form string      `json:"form"`
	Kind SegmentKind `json:"kind"`
	// FeatureID is set on suffix segments: the UD id this piece realises.
	FeatureID string `json:"featureId,omitempty"`
	// FeatureIDs is set on an ending segment: everything the tail carries.
	FeatureIDs []string `json:"featureIds,omitempty"`
	// AllomorphRule names which harmony variant this suffix is — "back",
	// "front_unrounded", "front_rounded" — found by REVERSE lookup in the
	// feature's allomorph table, so it cannot disagree with the surface form.
	AllomorphRule string `json:"allomorphRule,omitempty"`
	// Note marks a stem alternation: "a→á" on almá-.
	Note string `json:"note,omitempty"`
}

// linkingVowels can appear between a stem and a consonant-initial suffix.
var linkingVowels = map[rune]bool{'a': true, 'e': true, 'o': true, 'ö': true, 'u': true, 'ü': true}

// peelOrder is the order suffixes come off from the RIGHT of the word.
//
// Hungarian nominal morphology is templatic — reading outward from the stem:
// stem (+ linking) (+ possessed-plural `i`) (+ possessor `m/d/a…`) (+ case). So
// peeling right-to-left, case comes off first, then the POSSESSOR, and only
// then the possessed-plural marker: házaimban = ház·a·i·m·ban peels ban → m → i.
// Only keys listed here are peeled at all; anything else stays fused in the
// ending, because guessing an order we have not verified is how a wrong
// boundary gets taught.
var peelOrder = []string{
	"Case",
	"Person[psor]",
	"Number[psed]",
	"Number",
	"Degree",
	"VerbForm",
	"Mood",
	"Tense",
}

// SegmentForm splits a form according to its analysis.
//
// It needs the analyzer for the feature→allomorph tables and the lemma surface.
// A form whose analysis is unknown gets a single stem segment — no invention.
func (a *Analyzer) SegmentForm(form string, an Analysis) []Segment {
	lower := strings.ToLower(form)
	if a == nil || an.Certainty == UnknownAnalysis || an.Lemma == "" {
		return []Segment{{Form: form, Kind: SegStem}}
	}
	lemma := strings.ToLower(an.Lemma)

	type peeledSuffix struct {
		surface string
		feature string
		rule    string
	}
	var peeled []peeledSuffix

	rest := lower
	for _, key := range peelOrder {
		value, has := an.Feats[key]
		if !has {
			continue
		}
		allo := a.allomorphTable(key + "=" + value)
		if len(allo) == 0 {
			continue // an unmarked feature (Definite=Ind, Case=Nom): no exponent
		}
		// Longest allomorph first, so -ban wins over a hypothetical -n.
		best, rule := "", ""
		for r, s := range allo {
			if s == "" || strings.Contains(s, "…") {
				continue // circumfixes (leg…bb) are not peelable from the right
			}
			if strings.HasSuffix(rest, s) && len(s) > len(best) {
				best, rule = s, r
			}
		}
		if best == "" || len(rest)-len(best) < 1 {
			continue // this feature's exponent is fused or assimilated; leave it
		}
		stem := rest[:len(rest)-len(best)]
		// Refuse a peel that eats into the lemma itself: `nak` off `csónak`
		// would leave `csó`, which is not the boat we started with.
		if !stemPlausible(stem, lemma) {
			continue
		}
		rest = stem
		peeled = append(peeled, peeledSuffix{surface: best, feature: key + "=" + value, rule: rule})
	}

	// What is left of the form after the peel: lemma, lemma+linking vowel, or a
	// stem alternation — anything else means the peel went wrong and we fall
	// back to the honest unsplit rendering.
	segs := make([]Segment, 0, len(peeled)+2)
	stemSeg, linking, ok := classifyStem(rest, lemma)
	if !ok {
		// Could not justify the boundaries. One stem guess + one ending, with
		// the surface split at the longest common prefix with the lemma.
		p := commonPrefixLen(lower, lemma)
		if p < 1 || p >= len(lower) {
			return []Segment{{Form: form, Kind: SegStem}}
		}
		out := []Segment{{Form: form[:p], Kind: SegStem}}
		out = append(out, Segment{
			Form: form[p:], Kind: SegEnding, FeatureIDs: markedFeatureIDs(a, an.Feats),
		})
		return out
	}
	segs = append(segs, stemSeg)
	if linking != "" {
		segs = append(segs, Segment{Form: linking, Kind: SegLinking})
	}
	for i := len(peeled) - 1; i >= 0; i-- {
		segs = append(segs, Segment{
			Form: peeled[i].surface, Kind: SegSuffix,
			FeatureID: peeled[i].feature, AllomorphRule: peeled[i].rule,
		})
	}
	// Nothing more to add: classifyStem accepted, so the peel accounted for the
	// whole surface, and a feature whose exponent we never located has no
	// material left to hang a segment on. The unsplit fallback above is where an
	// unaccounted-for tail is reported, honestly and in one piece.
	return segs
}

// classifyStem explains the residue left after peeling: the lemma itself, the
// lemma plus one linking vowel, or a low-vowel-lengthened lemma.
func classifyStem(rest, lemma string) (Segment, string, bool) {
	if rest == lemma {
		return Segment{Form: rest, Kind: SegStem}, "", true
	}
	// ház + a + k: one trailing linking vowel.
	r := []rune(rest)
	if len(r) > 1 && string(r[:len(r)-1]) == lemma && linkingVowels[r[len(r)-1]] {
		return Segment{Form: lemma, Kind: SegStem}, string(r[len(r)-1]), true
	}
	// alma → almá-: final a/e lengthens before most suffixes.
	l := []rune(lemma)
	if len(r) == len(l) && len(r) > 0 {
		last := len(r) - 1
		if string(r[:last]) == string(l[:last]) {
			if (r[last] == 'á' && l[last] == 'a') || (r[last] == 'é' && l[last] == 'e') {
				return Segment{
					Form: rest, Kind: SegStem,
					Note: string(l[last]) + "→" + string(r[last]),
				}, "", true
			}
		}
	}
	return Segment{}, "", false
}

// stemPlausible keeps a peel from destroying the lemma: after removing a
// suffix, what remains must still start with the lemma's first runes (three of
// them, or all of a shorter lemma) — the same guard the drill generator uses.
func stemPlausible(stem, lemma string) bool {
	lr := []rune(lemma)
	n := min(len(lr), 3)
	return strings.HasPrefix(stem, string(lr[:n]))
}

// markedFeatureIDs lists the features that actually have a surface exponent —
// the ones a fused ending can honestly be said to carry. Unmarked defaults
// (Case=Nom, Definite=Ind) are not "in" the ending; they are the absence of one.
func markedFeatureIDs(a *Analyzer, feats map[string]string) []string {
	var out []string
	for k, v := range feats {
		if len(a.allomorphTable(k+"="+v)) > 0 {
			out = append(out, k+"="+v)
		}
	}
	sortStrings(out)
	return out
}

// allomorphTable returns a feature's {rule: suffix} map, cached per analyzer.
func (a *Analyzer) allomorphTable(featureID string) map[string]string {
	a.alloOnce.Do(func() {
		a.alloTables = map[string]map[string]string{}
		rows, err := a.pack.DB().Query(`SELECT id, allomorphs FROM morph_feature`)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id, raw string
			if err := rows.Scan(&id, &raw); err != nil {
				continue
			}
			m := map[string]string{}
			if jsonUnmarshal(raw, &m) != nil {
				continue
			}
			// Drop empty exponents so len()>0 means "this feature is marked".
			for k, v := range m {
				if v == "" {
					delete(m, k)
				}
			}
			a.alloTables[id] = m
		}
	})
	return a.alloTables[featureID]
}

func commonPrefixLen(a, b string) int {
	ar, br := []rune(a), []rune(b)
	i := 0
	for i < len(ar) && i < len(br) && ar[i] == br[i] {
		i++
	}
	return len(string(ar[:i]))
}
