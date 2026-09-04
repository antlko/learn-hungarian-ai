package tutor

import (
	"sort"
	"strings"
	"unicode"
)

// Topic inference: which lesson a question is about, when the thread does not
// say (ADR-029).
//
// The learner asks in a free chat; the app finds the lesson. Deterministic and
// explainable, like retrieval: the pack's analyser reads the Hungarian words in
// the question and their features point at the topic that teaches them
// (`házban` → Case=Ine → the locative grid), and the topic's own vocabulary —
// its titles and feature labels in both languages, its suffix shapes, its
// lesson text — scores the rest. Below the threshold the thread's last
// grounding is kept, so «а ещё примеры?» stays on the lesson the previous
// answer used; with nothing at all the prompt says so and the answer is built
// from the reference alone. The verdict is stored on the answer and printed
// under it — a choice the learner can see is a choice the learner can correct
// by pinning.

// TopicIndex is one curriculum topic's vocabulary, built by the caller from
// the DAG and the pack (this package imports neither).
type TopicIndex struct {
	Code string
	// Terms maps a lowercase token to its weight; the same token from two
	// sources keeps the higher weight.
	Terms map[string]float64
	// Features are the ids the topic teaches.
	Features map[string]bool
}

// Weights, in the order of how specific each signal is.
const (
	weightFeature   = 5.0 // the analyser read a form and it carries a feature this topic teaches
	weightAllomorph = 4.0 // the learner wrote a suffix shape this topic's features take
	weightTitle     = 3.0 // a word of the topic's title, in either language
	weightLabel     = 2.0 // a word of one of its feature labels
	weightLesson    = 1.0 // a word of its lesson prose
	// lessonCap bounds what lesson vocabulary alone can score, so a long
	// lesson that mentions everything does not win every question.
	lessonCap = 3.0
	// uniqueBonus is added to a title, label or suffix term that only one
	// topic carries — it names that lesson on its own.
	uniqueBonus = 1.0
	// InferThreshold is the score below which no topic is chosen.
	InferThreshold = 3.0
)

// NewTopicIndex builds one topic's vocabulary. titles and labels are given in
// every language the learner may ask in; allomorphs are bare suffix shapes
// (`ban`, `ek`); lesson is the prose in one or more languages.
func NewTopicIndex(code string, titles, labels, allomorphs []string, lesson string, featureIDs []string) TopicIndex {
	ix := TopicIndex{Code: code, Terms: map[string]float64{}, Features: map[string]bool{}}
	add := func(tok string, w float64) {
		if tok == "" {
			return
		}
		if ix.Terms[tok] < w {
			ix.Terms[tok] = w
		}
	}
	for _, t := range titles {
		for _, tok := range inferTokens(t) {
			add(tok, weightTitle)
		}
	}
	for _, l := range labels {
		for _, tok := range inferTokens(l) {
			add(tok, weightLabel)
		}
	}
	for _, a := range allomorphs {
		a = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(a), "-"))
		if a == "" {
			continue
		}
		add("-"+a, weightAllomorph)
	}
	for _, tok := range inferTokens(lesson) {
		if _, seen := ix.Terms[tok]; !seen {
			ix.Terms[tok] = weightLesson
		}
	}
	for _, f := range featureIDs {
		ix.Features[f] = true
	}
	return ix
}

// Inference is the verdict: the topic chosen, or blank, and why.
type Inference struct {
	Topic string
	Score float64
	// Sticky is true when the question itself scored nothing and the thread's
	// previous grounding was kept.
	Sticky bool
}

// InferTopic chooses the lesson for one question. formFeatures are the
// feature ids the analyser found on the question's Hungarian words; sticky is
// the topic the thread's last answer was grounded in, or blank.
func InferTopic(question string, formFeatures []string, sticky string, index []TopicIndex) Inference {
	toks := inferTokens(question)
	feats := map[string]bool{}
	for _, f := range formFeatures {
		feats[f] = true
	}
	// A term only one topic carries is worth a little more than its source
	// weight says: «инессив» names a lesson by itself, «число» does not.
	// Lesson prose stays out of this — its cap is its whole discipline.
	df := map[string]int{}
	for _, ix := range index {
		for tok, w := range ix.Terms {
			if w > weightLesson {
				df[tok]++
			}
		}
	}
	type scored struct {
		code  string
		score float64
	}
	var best []scored
	for _, ix := range index {
		score, lesson := 0.0, 0.0
		seen := map[string]bool{}
		for _, tok := range toks {
			if seen[tok] {
				continue
			}
			seen[tok] = true
			w, ok := ix.Terms[tok]
			if !ok {
				continue
			}
			switch {
			case w == weightLesson:
				lesson += w
			case df[tok] == 1:
				score += w + uniqueBonus
			default:
				score += w
			}
		}
		score += min(lesson, lessonCap)
		for f := range feats {
			if ix.Features[f] {
				score += weightFeature
			}
		}
		if score > 0 {
			best = append(best, scored{ix.Code, score})
		}
	}
	// Deterministic: score, then index order — a tie must not flicker.
	sort.SliceStable(best, func(i, j int) bool { return best[i].score > best[j].score })
	if len(best) > 0 && best[0].score >= InferThreshold {
		return Inference{Topic: best[0].code, Score: best[0].score}
	}
	if sticky != "" {
		return Inference{Topic: sticky, Sticky: true}
	}
	return Inference{}
}

// inferTokens is retrieval's tokenizer with one difference: a hyphen-led
// token keeps two letters, because `-ek` and `-em` are the suffix shapes a
// learner asks about most and retrieval's three-letter floor drops them.
func inferTokens(s string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		t := cur.String()
		cur.Reset()
		if t == "" || t == "-" {
			return
		}
		bare := strings.TrimPrefix(t, "-")
		floor := 3
		if t != bare {
			floor = 2
		}
		if len([]rune(bare)) < floor || stopwords[bare] {
			return
		}
		out = append(out, t)
	}
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r):
			cur.WriteRune(r)
		case r == '-' && cur.Len() == 0:
			cur.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return out
}
