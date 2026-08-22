package mining

import (
	"math"
	"sort"
	"strings"
)

// n+1 sentence selection.
//
// The point of the two state spaces, made concrete. A sentence is at the right
// difficulty when it contains ONE OR TWO things the learner does not yet know —
// and "things" means lemmas AND morphological features, not words.
//
// `Elmentem volna.` contains zero unknown lemmas and two unknown features
// (Mood=Cnd and the preverb). A word-counting scorer calls it trivial and shows
// it to a beginner who cannot parse it; this one does not.
//
// See docs/ARCHITECTURE.md §5.1 and .claude/research/06-lexical-coverage.md.

// Weights configure the scorer. They live in config.yaml under `mining:` so they
// can be tuned against a real learner without a rebuild.
type Weights struct {
	SweetSpot float64
	Coverage  float64
	Utility   float64
	Relevance float64
	Length    float64
	Duplicate float64
}

// Params are the scorer's tunables.
type Params struct {
	// KnownThreshold is the retrievability above which an element counts as
	// known. Below it, the learner would have to reconstruct rather than recall.
	KnownThreshold float64
	// FeatureWeight discounts an unknown FEATURE relative to an unknown LEMMA.
	// A new suffix on a familiar word is easier to absorb than a new word, but
	// it is not free — which is exactly what a word-only scorer assumes.
	FeatureWeight float64
	// TargetUnknown is the sweet spot, in weighted unknown elements.
	TargetUnknown float64
	UnknownSigma  float64
	// MinCoverage is the floor below which a sentence stops being input and
	// becomes decoding.
	//
	// It is deliberately LOWER than the 95%/98% reading thresholds, and the
	// distinction is easy to get wrong. Those figures describe comprehending a
	// whole text unaided. A MINED sentence is the opposite situation: you want
	// exactly one or two unknowns and you look them up. A ten-word sentence with
	// one unknown word is 90% coverage; a seven-word one is 86%. Applying the
	// reading threshold here would penalise precisely the sentences the scorer
	// exists to find, and the sweet-spot term already handles "how much is new".
	// This floor is only a guard against sentences that are mostly unknown.
	MinCoverage float64
	MinLength   int
	MaxLength   int
	Weights     Weights
}

// DefaultParams mirrors the config defaults, for tests and for callers without
// a config to hand.
func DefaultParams() Params {
	return Params{
		KnownThreshold: 0.80,
		FeatureWeight:  0.70,
		TargetUnknown:  1.5,
		UnknownSigma:   0.8,
		MinCoverage:    0.75,
		MinLength:      6,
		MaxLength:      14,
		Weights: Weights{
			SweetSpot: 1.0, Coverage: 0.8, Utility: 0.6,
			Relevance: 0.5, Length: 0.3, Duplicate: 1.0,
		},
	}
}

// Candidate is a sentence being considered.
type Candidate struct {
	SentIdx int         `json:"sentIdx"`
	Text    string      `json:"text"`
	Tokens  []TokenView `json:"-"`
}

// Scored is a candidate with its verdict and the reasoning behind it.
type Scored struct {
	Candidate
	Score float64 `json:"score"`
	// UnknownLemmas and UnknownFeatures are shown to the learner: "this sentence
	// teaches you `szállít` and the plural definite" is far more useful than a
	// bare relevance number.
	UnknownLemmas   []int64  `json:"unknownLemmas"`
	UnknownFeatures []string `json:"unknownFeatures"`
	UnknownWeighted float64  `json:"unknownWeighted"`
	Coverage        float64  `json:"coverage"`
}

// Context is everything the scorer needs to know about the learner.
type Context struct {
	// KnownLemma reports whether a lemma is retrievable now.
	KnownLemma func(int64) bool
	// KnownFeature reports whether a morphological feature is retrievable now.
	KnownFeature func(string) bool
	// FrequencyRank of a lemma, for the utility term. 0 means unranked.
	FrequencyRank func(int64) int
	// TopicFeatures are the features the learner is currently studying; a
	// sentence exercising them is worth more than an equally difficult one that
	// does not.
	TopicFeatures map[string]bool
	// SeenText lets the scorer penalise near-duplicates already in the queue.
	SeenText map[string]bool
}

// Score evaluates one sentence.
func Score(c Candidate, ctx Context, p Params) Scored {
	out := Scored{Candidate: c}

	unknownLemmas := map[int64]struct{}{}
	unknownFeats := map[string]struct{}{}
	words, knownWords := 0, 0

	for _, t := range c.Tokens {
		if !t.IsWord {
			continue
		}
		words++

		lemmaKnown := t.LemmaID != 0 && ctx.KnownLemma != nil && ctx.KnownLemma(t.LemmaID)
		if lemmaKnown {
			knownWords++
		} else if t.LemmaID != 0 {
			unknownLemmas[t.LemmaID] = struct{}{}
		}

		for _, f := range t.FeatureIDs {
			if ctx.KnownFeature == nil || !ctx.KnownFeature(f) {
				unknownFeats[f] = struct{}{}
			}
		}
	}

	out.UnknownLemmas = keysOf(unknownLemmas)
	out.UnknownFeatures = sortedKeys(unknownFeats)
	out.UnknownWeighted = float64(len(unknownLemmas)) +
		p.FeatureWeight*float64(len(unknownFeats))
	if words > 0 {
		out.Coverage = float64(knownWords) / float64(words)
	}

	w := p.Weights
	score := w.SweetSpot * gaussian(out.UnknownWeighted, p.TargetUnknown, p.UnknownSigma)

	if out.Coverage >= p.MinCoverage {
		score += w.Coverage
	} else {
		// Below the comprehensibility floor this is not input, it is decoding.
		// Penalise hard rather than merely not rewarding.
		score -= w.Coverage * (p.MinCoverage - out.Coverage) * 4
	}

	score += w.Utility * utility(out.UnknownLemmas, ctx.FrequencyRank)
	score += w.Relevance * relevance(out.UnknownFeatures, ctx.TopicFeatures)
	score += w.Length * lengthFit(words, p.MinLength, p.MaxLength)

	if ctx.SeenText != nil && ctx.SeenText[normaliseText(c.Text)] {
		score -= w.Duplicate
	}

	out.Score = score
	return out
}

// Rank scores and orders candidates, best first, dropping anything below zero.
func Rank(cands []Candidate, ctx Context, p Params, limit int) []Scored {
	out := make([]Scored, 0, len(cands))
	for _, c := range cands {
		s := Score(c, ctx, p)
		if s.Score <= 0 {
			continue
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// gaussian peaks at mu. This is what makes the scorer target a BAND rather than
// a minimum: a sentence with nothing new is as useless as one with ten new
// things, and a monotonic "fewer unknowns is better" score would rank the former
// highest.
func gaussian(x, mu, sigma float64) float64 {
	if sigma <= 0 {
		sigma = 1
	}
	d := (x - mu) / sigma
	return math.Exp(-0.5 * d * d)
}

// utility rewards sentences whose new words are frequent ones. Learning a word
// you will meet weekly is worth more than learning one you will meet once.
func utility(lemmas []int64, rank func(int64) int) float64 {
	if len(lemmas) == 0 || rank == nil {
		return 0
	}
	total := 0.0
	for _, id := range lemmas {
		r := rank(id)
		if r <= 0 {
			continue
		}
		// Rank 1 ≈ 1.0, rank 5000 ≈ 0.35, rank 50000 ≈ 0.09.
		total += 1.0 / (1.0 + math.Log10(float64(r)))
	}
	return total / float64(len(lemmas))
}

// relevance rewards a sentence that exercises what is being studied now.
func relevance(feats []string, topic map[string]bool) float64 {
	if len(topic) == 0 || len(feats) == 0 {
		return 0
	}
	hits := 0
	for _, f := range feats {
		if topic[f] {
			hits++
		}
	}
	return float64(hits) / float64(len(feats))
}

// lengthFit is 1 inside the window and falls off outside it. Short sentences
// carry too little context to teach a suffix; long ones stop being input.
func lengthFit(n, minLen, maxLen int) float64 {
	switch {
	case n >= minLen && n <= maxLen:
		return 1
	case n < minLen:
		if minLen == 0 {
			return 0
		}
		return math.Max(0, float64(n)/float64(minLen))
	default:
		over := float64(n - maxLen)
		return math.Max(0, 1-over/float64(maxLen))
	}
}

func normaliseText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
