package mining

import (
	"encoding/json"
	"sort"
)

// Estimating a text's CEFR level from what the pack already knows (ADR-023).
//
// No model and no magic: the pack carries a frequency-derived CEFR band per
// lemma (`lemma.cefr`, bands 900/2000/4500/8000/16000 from
// langpack/pipeline/frequency.py — the ONE band table; drill/generate.go's
// bandFor is a different dial for a different purpose and is deliberately not
// consulted) and a CEFR level per grammar topic (`grammar_topic.level`, mapped
// from the features a token carries). Those two, plus sentence length, are
// enough for an honest estimate — and when too little of the text resolves,
// the honest estimate is "no estimate", never a guess.

// levels in teaching order; index is used for max/bump arithmetic.
var cefrOrder = []string{"A1", "A2", "B1", "B2", "C1", "C2"}

func cefrIndex(level string) int {
	for i, l := range cefrOrder {
		if l == level {
			return i
		}
	}
	return -1
}

// CEFRBefore reports whether a comes before b in teaching order. For the api
// layer's "a feature taught by two topics belongs to the earlier one" rule.
func CEFRBefore(a, b string) bool { return cefrIndex(a) < cefrIndex(b) }

// ValidCEFR reports whether a string is one of the six bands — the manual
// override endpoint refuses anything else.
func ValidCEFR(level string) bool { return cefrIndex(level) >= 0 }

// LevelInputs are the pack lookups, injected the way the rest of this package
// takes its data: mining never touches the pack directly.
type LevelInputs struct {
	// CEFRForLemma returns pack lemma.cefr, or "" when the lemma is absent from
	// the frequency list.
	CEFRForLemma func(int64) string
	// TopicLevelFor maps a UD feature id to the CEFR level of the grammar topic
	// that teaches it, or "" when no topic claims it.
	TopicLevelFor func(string) string
}

// LevelEstimate is the verdict plus the working that produced it — the UI
// shows the level, and the disclosure can show why.
type LevelEstimate struct {
	// Level is "" when the estimate refuses itself (see Resolved).
	Level        string `json:"level"`
	LexicalLevel string `json:"lexicalLevel,omitempty"`
	GrammarLevel string `json:"grammarLevel,omitempty"`
	// BandShare is the share of resolved word tokens at or below each band,
	// cumulative — "0.91 of the running words are A2 vocabulary or easier".
	BandShare map[string]float64 `json:"bandShare,omitempty"`
	// MeanSentenceLen is words per sentence, the crudest readability dial.
	MeanSentenceLen float64 `json:"meanSentenceLen,omitempty"`
	// Resolved is the share of word tokens the pack could resolve at all. The
	// estimate rests on this much of the text; below half, it abstains.
	Resolved float64 `json:"resolved"`
}

// Tunables, documented rather than configurable: a text's level is a fact to
// estimate, not a preference.
const (
	// lexicalShare: the lexical level is the smallest band covering at least
	// this share of resolved running words.
	lexicalShare = 0.90
	// grammarMinHits: a topic's level counts only once its features occur this
	// many times — one stray conditional does not make a text B2.
	grammarMinHits = 3
	// longSentence / shortSentence bump the estimate one step up or down.
	longSentence  = 16.0
	shortSentence = 8.0
	// minResolved: below this share of resolved tokens the estimate abstains.
	minResolved = 0.5
)

// EstimateLevel judges one text from its stored tokens. Pure: everything it
// reads arrives through `in`.
func EstimateLevel(tokens []TokenView, in LevelInputs) LevelEstimate {
	var est LevelEstimate

	words, resolved := 0, 0
	perBand := map[string]int{}
	grammarHits := map[string]int{}
	sentences := map[int]bool{}

	for _, t := range tokens {
		if !t.IsWord {
			continue
		}
		words++
		sentences[t.SentIdx] = true
		if t.LemmaID == 0 {
			continue
		}
		band := in.CEFRForLemma(t.LemmaID)
		if band == "" {
			// In the pack but off the frequency list: rare by construction.
			band = "C2"
		}
		resolved++
		perBand[band]++
		for _, f := range t.FeatureIDs {
			if lvl := in.TopicLevelFor(f); lvl != "" {
				grammarHits[lvl]++
			}
		}
	}

	if words == 0 {
		return est
	}
	est.Resolved = float64(resolved) / float64(words)
	if len(sentences) > 0 {
		est.MeanSentenceLen = float64(words) / float64(len(sentences))
	}

	if resolved == 0 || est.Resolved < minResolved {
		// Not enough of the text resolves for a claim; "" renders as «уровень
		// не определён», which is the true statement.
		return est
	}

	// Lexical level: the smallest band whose cumulative share of resolved
	// running words reaches the threshold. BandShare keeps the whole curve for
	// the disclosure.
	est.BandShare = map[string]float64{}
	cum := 0
	lexical := "C2"
	found := false
	for _, band := range cefrOrder {
		cum += perBand[band]
		share := float64(cum) / float64(resolved)
		est.BandShare[band] = share
		if !found && share >= lexicalShare {
			lexical = band
			found = true
		}
	}
	est.LexicalLevel = lexical

	// Grammar level: the highest topic level with enough evidence.
	grammar := ""
	var seen []string
	for lvl, n := range grammarHits {
		if n >= grammarMinHits {
			seen = append(seen, lvl)
		}
	}
	sort.Slice(seen, func(i, j int) bool { return cefrIndex(seen[i]) < cefrIndex(seen[j]) })
	if len(seen) > 0 {
		grammar = seen[len(seen)-1]
	}
	est.GrammarLevel = grammar

	// The verdict: the harder of the two, nudged by sentence length.
	idx := cefrIndex(lexical)
	if g := cefrIndex(grammar); g > idx {
		idx = g
	}
	if est.MeanSentenceLen > longSentence && idx < len(cefrOrder)-1 {
		idx++
	} else if est.MeanSentenceLen > 0 && est.MeanSentenceLen < shortSentence && idx > 0 {
		idx--
	}
	est.Level = cefrOrder[idx]
	return est
}

// Detail serialises the working for storage in text.level_detail.
func (e LevelEstimate) Detail() string {
	b, err := json.Marshal(e)
	if err != nil {
		return ""
	}
	return string(b)
}
