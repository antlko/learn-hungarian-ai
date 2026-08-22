// Package mining ingests text, measures how readable it is for a given learner,
// and selects sentences at the right difficulty.
package mining

// Coverage measures how much of a text a learner can read.
//
// TWO numbers, deliberately, because the familiar one is misleading for an
// agglutinative language:
//
//   - LemmaCoverage is the share of running tokens whose LEMMA is known. This is
//     the figure comparable to the published 95%/98% thresholds — which were
//     derived from English word families, where a family bundles a handful of
//     inflections.
//   - ElementCoverage additionally requires every morphological feature on the
//     token to be known. A Hungarian learner can know `ház` perfectly and still
//     be stopped by `házaimban`, so lemma coverage systematically overstates how
//     readable a text actually is.
//
// The reader shows both and labels the threshold lines as English-derived. The
// n+1 scorer uses the element figure, because it is the honest one.
// See .claude/research/06-lexical-coverage.md.
type Coverage struct {
	Words           int     `json:"words"`
	KnownLemmas     int     `json:"knownLemmas"`
	KnownElements   int     `json:"knownElements"`
	Unresolved      int     `json:"unresolved"`
	LemmaCoverage   float64 `json:"lemmaCoverage"`
	ElementCoverage float64 `json:"elementCoverage"`
	// UnknownLemmaIDs and UnknownFeatures are what a learner would have to
	// acquire to read this text, which is more actionable than a percentage.
	UnknownLemmaIDs []int64  `json:"unknownLemmaIds"`
	UnknownFeatures []string `json:"unknownFeatures"`
}

// Thresholds from the literature. Held here as named constants so that the one
// place they are used can also carry the caveat.
const (
	// MinimalComprehension is the coverage below which a text is not
	// comprehensible input at all — it is decoding.
	MinimalComprehension = 0.95
	// UnassistedComprehension is the coverage at which most learners can read
	// without a dictionary.
	UnassistedComprehension = 0.98
	// ListeningFloor is the practical floor for audio, from the source brief:
	// below this, listening is noise rather than input.
	ListeningFloor = 0.90
)

// Verdict is a plain-language reading of a coverage figure.
type Verdict string

const (
	VerdictTooHard     Verdict = "too_hard"
	VerdictWithHelp    Verdict = "with_help"
	VerdictComfortable Verdict = "comfortable"
)

// Judge classifies a coverage figure. It uses ELEMENT coverage on purpose: a
// learner who knows every word but not the suffixes cannot read the text.
func (c Coverage) Judge() Verdict {
	switch {
	case c.ElementCoverage >= UnassistedComprehension:
		return VerdictComfortable
	case c.ElementCoverage >= MinimalComprehension:
		return VerdictWithHelp
	default:
		return VerdictTooHard
	}
}

// Compute builds a coverage report.
//
// `known` maps lemma id -> status; `knownFeature` reports whether a feature is
// currently retrievable. A token counts toward element coverage only if BOTH its
// lemma and every one of its features are known.
func Compute(
	tokens []TokenView,
	known map[int64]string,
	knownFeature func(string) bool,
) Coverage {
	var c Coverage
	unknownLemmas := map[int64]struct{}{}
	unknownFeats := map[string]struct{}{}

	for _, t := range tokens {
		if !t.IsWord {
			continue
		}
		c.Words++

		if t.LemmaID == 0 {
			// Nothing in the pack matched: it cannot count as known, and it is
			// not evidence of a gap in the learner either.
			c.Unresolved++
			continue
		}

		status := known[t.LemmaID]
		lemmaKnown := status == "known" || status == "ignored" || status == "learning"
		if lemmaKnown {
			c.KnownLemmas++
		} else {
			unknownLemmas[t.LemmaID] = struct{}{}
		}

		featsKnown := true
		for _, f := range t.FeatureIDs {
			if !knownFeature(f) {
				featsKnown = false
				unknownFeats[f] = struct{}{}
			}
		}
		if lemmaKnown && featsKnown {
			c.KnownElements++
		}
	}

	if c.Words > 0 {
		c.LemmaCoverage = float64(c.KnownLemmas) / float64(c.Words)
		c.ElementCoverage = float64(c.KnownElements) / float64(c.Words)
	}
	c.UnknownLemmaIDs = keysOf(unknownLemmas)
	c.UnknownFeatures = sortedKeys(unknownFeats)
	return c
}

// TokenView is the slice of a token that coverage and scoring care about.
type TokenView struct {
	Index      int
	SentIdx    int
	Form       string
	IsWord     bool
	LemmaID    int64
	FeatureIDs []string
}
