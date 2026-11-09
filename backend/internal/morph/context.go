package morph

import (
	"sort"
	"strings"
)

// ContextHint is one token as an external contextual tagger saw it. Offsets
// are UTF-8 bytes into the text that was analysed. It carries no authority of
// its own: it can only select among readings the pack already has.
type ContextHint struct {
	Start, End int
	Lemma      string
	UPOS       string
}

// ApplyContextHints re-chooses, in place, the reading of every word the pack
// found ambiguous (`uncertain`), using the hint that overlaps its span.
//
// The invariants this exists to keep (ADR-054):
//   - a candidate is picked from Lookup(form) — the pack's own list — and a
//     hint with no matching candidate changes nothing;
//   - only `uncertain` words are touched; certain, derived, unknown and
//     corrected readings are never overridden;
//   - with no hints the tokens are byte-for-byte unchanged.
//
// It returns how many tokens were re-chosen (marked Via=context, whether or
// not the candidate differs from the default: the sidecar agreed or decided).
func (a *Analyzer) ApplyContextHints(tokens []AnalyzedToken, hints []ContextHint) (int, error) {
	if len(hints) == 0 {
		return 0, nil
	}
	hs := append([]ContextHint(nil), hints...)
	sort.SliceStable(hs, func(i, j int) bool { return hs[i].Start < hs[j].Start })

	cands := map[string][]Analysis{}
	changed := 0
	for i := range tokens {
		t := &tokens[i]
		if !t.IsWord || t.Analysis.Certainty != UncertainAnalysis {
			continue
		}
		h, ok := bestOverlap(hs, t.Start, t.End)
		if !ok {
			continue
		}
		key := strings.ToLower(t.Text)
		list, seen := cands[key]
		if !seen {
			var err error
			if list, err = a.Lookup(t.Text); err != nil {
				return changed, err
			}
			cands[key] = list
		}
		for _, c := range list {
			if c.Certainty == DerivedAnalysis {
				continue // generated rows are never offered as a choice
			}
			if strings.EqualFold(c.Lemma, h.Lemma) && strings.EqualFold(c.UPOS, h.UPOS) {
				c.Certainty = UncertainAnalysis
				c.Via = ViaContext
				t.Analysis = c
				changed++
				break
			}
		}
	}
	return changed, nil
}

// bestOverlap finds the hint sharing the most bytes with [start,end).
func bestOverlap(hs []ContextHint, start, end int) (ContextHint, bool) {
	// First hint that could overlap: its End > start. Hints are sorted by
	// Start, so a linear walk from a binary-searched point is enough.
	i := sort.Search(len(hs), func(k int) bool { return hs[k].Start >= start })
	for i > 0 && hs[i-1].End > start {
		i--
	}
	var best ContextHint
	bestLen := 0
	for ; i < len(hs) && hs[i].Start < end; i++ {
		lo, hi := max(hs[i].Start, start), min(hs[i].End, end)
		if hi-lo > bestLen {
			best, bestLen = hs[i], hi-lo
		}
	}
	return best, bestLen > 0
}
