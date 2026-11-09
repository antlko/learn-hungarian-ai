package stats

import "math"

// The estimated level and text coverage (ADR-047).
//
// A MEASURED estimate with a stated method, computed per request from the
// learner's deck and never stored. There are no points here: both figures can
// go DOWN (a word whose retrievability decays below the threshold stops
// counting), which is what separates a competence signal from a score
// [R14, R16 §3]. The CEFR bands are the pack's frequency-rank estimate
// (`lemma.cefr`), not an exam, and the UI says so beside every figure
// (LevelMethodKey).
//
// This file is pure: the caller decides which lemmas count and supplies the
// pack's band sizes. See internal/api/handlers_stats_level.go.
// Why coverage of running text is the headline: [R6] see
// .claude/research/06-lexical-coverage.md (comprehension tracks the share of
// running words known; English figures, Hungarian transfer is open) and [R11]
// .claude/research/11-extensive-reading.md (volume of readable input).

const (
	// LevelBandTarget is the share of a CEFR band's lemmas that must count
	// before the band is "reached". 0.80 is a CHOSEN threshold, not a
	// literature figure (R6's 95%/98% are English text-coverage numbers); it is
	// the same number as the default mining known-threshold on purpose.
	LevelBandTarget = 0.80
	// LevelGrammarShare is the fraction of a band's DRILLABLE grammar topics
	// that must be interleaved or mastered. A band with none passes.
	LevelGrammarShare = 0.5
	// LevelLowDataLemmas: below this many counted lemmas the estimate is
	// labelled "low" (мало данных). It is never "high": there is no exam behind it.
	LevelLowDataLemmas = 100
	// LevelSublevels is how many steps a band is split into (A2.1 .. A2.3).
	LevelSublevels = 3
	// LevelMethodKey is the i18n key of the fixed method sentence.
	LevelMethodKey = "level.method.v1"

	ConfidenceLow      = "low"
	ConfidenceEstimate = "estimate"

	// shareEpsilon absorbs float error at the 0.80 edge (4/5 must reach).
	shareEpsilon = 1e-9
)

// LevelBands are the bands the estimate climbs, lowest first. C2 is not
// measured: the frequency list does not separate it honestly.
var LevelBands = []string{"A1", "A2", "B1", "B2", "C1"}

// TopicPhase is the part of a grammar topic the estimate reads.
type TopicPhase struct {
	Level string
	// Drillable: the topic has a drill generator. Rule-card-only topics
	// (phase "explained") cannot be practised and so never hold a band back.
	Drillable bool
	// Phase is drill.Phase as a string; "interleaved" and "mastered" count.
	Phase string
}

// LevelInput is everything EstimateLevel needs; none of it is read from a store.
type LevelInput struct {
	// Counted and Sizes are lemma counts per band (counted by the learner / in
	// the pack).
	Counted map[string]int
	Sizes   map[string]int
	// CountedLemmas is the total counted lemmas, including those with no band.
	CountedLemmas int
	// FreqCounted / FreqTotal are sums of freq_pm for counted lemmas and for
	// every frequency-ranked pack lemma.
	FreqCounted float64
	FreqTotal   float64
	Topics      []TopicPhase
	// Blocked is BlockedLevels(history): levels held shut by a failed checkpoint.
	Blocked map[string]bool
}

// BandLevel is one band's standing.
type BandLevel struct {
	Band             string  `json:"band"`
	Counted          int     `json:"counted"`
	Total            int     `json:"total"`
	Share            float64 `json:"share"`
	Target           float64 `json:"target"`
	GrammarReady     bool    `json:"grammarReady"`
	GrammarDrillable int     `json:"grammarDrillable"`
	Reached          bool    `json:"reached"`
}

// LevelEstimate is the response of GET /api/stats/level.
type LevelEstimate struct {
	Label         string      `json:"label"`
	Band          string      `json:"band"`
	Sublevel      int         `json:"sublevel"`
	Confidence    string      `json:"confidence"`
	Coverage      float64     `json:"coverage"`
	CountedLemmas int         `json:"countedLemmas"`
	Bands         []BandLevel `json:"bands"`
	// CappedBy names the checkpoint holding the working band shut ("m3"), when
	// vocabulary and grammar would otherwise have passed it; "" otherwise.
	CappedBy  string `json:"cappedBy"`
	MethodKey string `json:"methodKey"`
}

// EstimateLevel applies the model: a band is reached when its vocabulary share
// is at least LevelBandTarget AND at least LevelGrammarShare of its drillable
// topics are interleaved|mastered AND the band is not blocked by a failed
// checkpoint. The working band is the first measurable band not reached; the
// sublevel is its vocabulary progress split in thirds. A learner whose
// vocabulary is done but whose grammar is not stays at .3 — Bands[i].GrammarReady
// is how the screen names the grammar.
func EstimateLevel(in LevelInput) LevelEstimate {
	out := LevelEstimate{
		Confidence:    ConfidenceEstimate,
		CountedLemmas: in.CountedLemmas,
		MethodKey:     LevelMethodKey,
		Bands:         make([]BandLevel, 0, len(LevelBands)),
	}
	if in.CountedLemmas < LevelLowDataLemmas {
		out.Confidence = ConfidenceLow
	}
	if in.FreqTotal > 0 {
		out.Coverage = math.Min(1, in.FreqCounted/in.FreqTotal)
	}

	working := -1
	for i, band := range LevelBands {
		bl := BandLevel{Band: band, Counted: in.Counted[band], Total: in.Sizes[band],
			Target: LevelBandTarget, GrammarReady: true}
		if bl.Total > 0 {
			bl.Share = float64(bl.Counted) / float64(bl.Total)
		}
		done := 0
		for _, t := range in.Topics {
			if t.Level != band || !t.Drillable {
				continue
			}
			bl.GrammarDrillable++
			if t.Phase == "interleaved" || t.Phase == "mastered" {
				done++
			}
		}
		if bl.GrammarDrillable > 0 {
			bl.GrammarReady = float64(done) >= LevelGrammarShare*float64(bl.GrammarDrillable)
		}
		vocab := bl.Total > 0 && bl.Share >= LevelBandTarget-shareEpsilon
		bl.Reached = vocab && bl.GrammarReady && !in.Blocked[band]
		if working < 0 && bl.Total > 0 && !bl.Reached {
			working = i
			if vocab && bl.GrammarReady && in.Blocked[band] {
				out.CappedBy = cappedBy(in.Blocked)
			}
		}
		out.Bands = append(out.Bands, bl)
	}

	switch {
	case working >= 0:
		w := out.Bands[working]
		p := math.Min(1, w.Share/LevelBandTarget)
		out.Band = w.Band
		out.Sublevel = 1 + int(math.Floor(math.Min(p, 0.999)*LevelSublevels))
		out.Label = w.Band + "." + string(rune('0'+out.Sublevel))
	case anyMeasurable(out.Bands):
		// Every measurable band reached: the top of what is measured.
		top := out.Bands[len(out.Bands)-1].Band
		out.Band, out.Sublevel, out.Label = top, LevelSublevels, top+"+"
	default:
		// No frequency bands in this pack: say the floor, not a guess.
		out.Band, out.Sublevel, out.Label = LevelBands[0], 1, LevelBands[0]+".1"
	}
	return out
}

func anyMeasurable(bs []BandLevel) bool {
	for _, b := range bs {
		if b.Total > 0 {
			return true
		}
	}
	return false
}

// cappedBy names the checkpoint behind the LOWEST blocked level: a failed m3
// blocks A2, B1 and B2 together, and the one the learner can retake is the
// root.
func cappedBy(blocked map[string]bool) string {
	for _, lvl := range LevelBands {
		if !blocked[lvl] {
			continue
		}
		for _, spec := range Specs {
			if spec.GatesLevel == lvl {
				return spec.Code
			}
		}
	}
	return ""
}
