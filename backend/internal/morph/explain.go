package morph

import (
	"fmt"
	"strings"
	"sync"
)

// Human-readable explanation of a form: explicit instruction in miniature.
//
// [R4] — .claude/research/04-explicit-instruction.md — point 3: the breakdown
// shown after an answer *names the feature*, *names the allomorph rule* and
// *shows why*. This file assembles that from what the pack already knows —
// per-feature labels and explanations in BOTH locales — so the backend never
// writes prose and never picks a language (ADR-004).
//
// Everything here derives from a segmentation that was itself verified against
// the analysis, so an explanation cannot claim a morpheme the form does not
// carry. A fused ending is explained collectively, which is what it is.

// ExplainLine is one line of the breakdown, ready to render.
type ExplainLine struct {
	// Surface is the piece of the form being explained: "ban", "a", "ház".
	Surface string      `json:"surface"`
	Kind    SegmentKind `json:"kind"`
	// FeatureID is set for suffix lines.
	FeatureID string `json:"featureId,omitempty"`
	LabelRU   string `json:"labelRu,omitempty"`
	LabelEN   string `json:"labelEn,omitempty"`
	// RuleRU/EN state the allomorph choice: «задняя гармония → -ban».
	RuleRU string `json:"ruleRu,omitempty"`
	RuleEN string `json:"ruleEn,omitempty"`
}

// harmonyRuleNames renders the allomorph-rule key for humans, in both locales.
// The keys are the pack's harmony-class names, stable by contract.
var harmonyRuleNames = map[string][2]string{
	"back":            {"задняя гармония", "back harmony"},
	"front_unrounded": {"передняя неогублённая", "front unrounded"},
	"front_rounded":   {"передняя огублённая", "front rounded"},
}

// featureLabels caches morph_feature labels per id.
type featureLabels struct {
	once sync.Once
	byID map[string][2]string // id -> {ru, en}
}

func (a *Analyzer) labels() map[string][2]string {
	a.labelCache.once.Do(func() {
		a.labelCache.byID = map[string][2]string{}
		rows, err := a.pack.DB().Query(`SELECT id, label_ru, label_en FROM morph_feature`)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id, ru, en string
			if err := rows.Scan(&id, &ru, &en); err != nil {
				continue
			}
			a.labelCache.byID[id] = [2]string{ru, en}
		}
	})
	return a.labelCache.byID
}

// Explain renders a segmentation as explanation lines.
func (a *Analyzer) Explain(segs []Segment) []ExplainLine {
	if a == nil {
		return nil
	}
	labels := a.labels()
	out := make([]ExplainLine, 0, len(segs))
	for _, s := range segs {
		line := ExplainLine{Surface: s.Form, Kind: s.Kind}
		switch s.Kind {
		case SegSuffix:
			line.FeatureID = s.FeatureID
			if l, ok := labels[s.FeatureID]; ok {
				line.LabelRU, line.LabelEN = l[0], l[1]
			}
			if rule, ok := harmonyRuleNames[s.AllomorphRule]; ok {
				// Only worth saying when there was a choice: "-t is -t" teaches
				// nothing, so an invariant suffix gets no rule line.
				if distinctExponents(a.allomorphTable(s.FeatureID)) > 1 {
					line.RuleRU = fmt.Sprintf("%s → -%s", rule[0], s.Form)
					line.RuleEN = fmt.Sprintf("%s → -%s", rule[1], s.Form)
				}
			}
		case SegEnding:
			// A fused ending lists what it carries, joined; no boundary claims.
			var ru, en []string
			for _, id := range s.FeatureIDs {
				if l, ok := labels[id]; ok {
					ru, en = append(ru, l[0]), append(en, l[1])
				} else {
					ru, en = append(ru, id), append(en, id)
				}
			}
			line.LabelRU = strings.Join(ru, " · ")
			line.LabelEN = strings.Join(en, " · ")
		}
		out = append(out, line)
	}
	return out
}

func distinctExponents(m map[string]string) int {
	seen := map[string]bool{}
	for _, v := range m {
		if v != "" {
			seen[v] = true
		}
	}
	return len(seen)
}
