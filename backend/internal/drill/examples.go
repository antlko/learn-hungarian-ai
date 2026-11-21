package drill

import (
	"sort"
	"strings"

	"tanulom/internal/curriculum"
	"tanulom/internal/morph"
)

// Lesson examples: real pack sentences showing a feature, for the rule card.
//
// Everything here is attested. A sentence is offered only if some token of it
// reads UNAMBIGUOUSLY as the wanted feature of a dictionary lemma through a
// non-generated reading (readingOf), the same standard as a cloze. A feature
// without a surface ending (Definite=Ind, Tense=Pres, ...) has nothing to point
// at, so it gets NO examples — never padding made up for the occasion.

const (
	// ExamplesPerFeature is the most examples a feature returns.
	ExamplesPerFeature = 4
	exampleMinWords    = 4
	exampleMaxWords    = 10
	// Bounds on the work for one feature: lemmas visited and sentences read.
	exampleLemmaBudget = 140
	exampleCandidates  = ExamplesPerFeature * 4
	exampleSentCap     = 10
)

// Example is one attested sentence with the feature-bearing token marked.
type Example struct {
	SentenceID    int64       `json:"sentenceId"`
	Parts         []FramePart `json:"parts"`
	TranslationRu string      `json:"translationRu,omitempty"`
	TranslationEn string      `json:"translationEn,omitempty"`

	lemmaID int64
	known   bool
	words   int
}

// Examples returns up to ExamplesPerFeature attested sentences for one feature
// of a topic. `known` biases toward lemmas the learner has met, `exclude` holds
// the sentence ids already shown. The bool is false when the feature is not one
// of the topic's.
func (g *Generator) Examples(t curriculum.Topic, featureID string, known map[int64]bool,
	exclude map[int64]bool) ([]Example, bool, error) {
	if err := g.load(); err != nil {
		return nil, false, err
	}
	owned := false
	for _, id := range t.FeatureIDs {
		owned = owned || id == featureID
	}
	f, ok := g.features[featureID]
	if !owned || !ok {
		return nil, false, nil
	}
	if !f.HasExponent() {
		return []Example{}, true, nil
	}

	pool, spec := g.poolFor(f)
	ceiling := bandFor(t.Level)
	end := sort.Search(len(pool), func(i int) bool { return pool[i].Rank > ceiling })
	if end < 20 {
		end = min(len(pool), 20)
	}
	// Known lemmas first, then the rest, both by frequency.
	var order []Word
	for _, w := range pool[:end] {
		if known[w.LemmaID] {
			order = append(order, w)
		}
	}
	nKnown := len(order)
	for _, w := range pool[:end] {
		if !known[w.LemmaID] {
			order = append(order, w)
		}
	}

	var cands []Example
	seen := map[int64]bool{}
	visited := 0
	for i, w := range order {
		if visited >= exampleLemmaBudget || len(cands) >= exampleCandidates {
			break
		}
		visited++
		for _, ex := range g.exampleSentences(w, f, spec, exclude, seen) {
			ex.known = i < nKnown
			cands = append(cands, ex)
		}
	}
	g.fillTranslations(cands)

	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.known != b.known {
			return a.known
		}
		if at, bt := a.TranslationRu != "" || a.TranslationEn != "", b.TranslationRu != "" || b.TranslationEn != ""; at != bt {
			return at
		}
		if a.words != b.words {
			return a.words < b.words
		}
		return a.SentenceID < b.SentenceID
	})

	// One example per lemma while the pool allows, so four sentences do not all
	// show the same noun.
	var out []Example
	usedLemma := map[int64]bool{}
	for _, c := range cands {
		if len(out) < ExamplesPerFeature && !usedLemma[c.lemmaID] {
			usedLemma[c.lemmaID] = true
			out = append(out, c)
		}
	}
	for _, c := range cands {
		if len(out) >= ExamplesPerFeature {
			break
		}
		dup := false
		for _, o := range out {
			dup = dup || o.SentenceID == c.SentenceID
		}
		if !dup {
			out = append(out, c)
		}
	}
	if out == nil {
		out = []Example{}
	}
	return out, true, nil
}

// exampleSentences finds the verified sentences of one lemma for a feature.
func (g *Generator) exampleSentences(w Word, f Feature, spec morph.FormSpec,
	exclude, seen map[int64]bool) []Example {
	forms, err := g.analyzer.FormsOf(w.LemmaID)
	if err != nil {
		return nil
	}
	var variants []string
	have := map[string]bool{}
	for _, fm := range forms {
		if !fm.Attested() || fm.Feats[f.Key] != f.Value || !specAllows(fm, spec) {
			continue
		}
		for _, v := range morph.CaseVariants(fm.Form) {
			if !have[v] {
				have[v] = true
				variants = append(variants, v)
			}
		}
		if len(variants) >= 8 {
			break
		}
	}
	if len(variants) == 0 {
		return nil
	}
	marks, args := morph.Placeholders(variants)
	args = append(args, exampleMinWords, exampleMaxWords+4, exampleSentCap)
	rows, err := g.pack.DB().Query(`
		SELECT DISTINCT s.id, s.hu FROM sentence_token t JOIN sentence s ON s.id = t.sentence_id
		 WHERE t.form IN (`+marks+`) AND s.source <> 'authored'
		   AND s.len_tokens BETWEEN ? AND ?
		 ORDER BY s.len_tokens LIMIT ?`,
		args...)
	if err != nil {
		return nil
	}
	type row struct {
		id int64
		hu string
	}
	var found []row
	for rows.Next() {
		var r row
		if rows.Scan(&r.id, &r.hu) == nil {
			found = append(found, r)
		}
	}
	rows.Close()

	var out []Example
	for _, r := range found {
		if seen[r.id] || exclude[r.id] {
			continue
		}
		tokens := morph.Tokenize(r.hu)
		n := wordCount(tokens)
		if n < exampleMinWords || n > exampleMaxWords {
			continue
		}
		for _, t := range tokens {
			if !t.IsWord || !have[t.Text] && !have[strings.ToLower(t.Text)] {
				continue
			}
			if _, ok := g.readingOf(t.Text, w.LemmaID, f); !ok {
				continue
			}
			seen[r.id] = true
			var parts []FramePart
			if t.Start > 0 {
				parts = append(parts, FramePart{Text: r.hu[:t.Start]})
			}
			parts = append(parts, FramePart{Text: r.hu[t.Start:t.End], Focus: true})
			if t.End < len(r.hu) {
				parts = append(parts, FramePart{Text: r.hu[t.End:]})
			}
			out = append(out, Example{SentenceID: r.id, Parts: parts, lemmaID: w.LemmaID, words: n})
			break
		}
	}
	return out
}

// specAllows keeps a form only if it is the plain cell the spec names: no
// possessed or possessor variant of the same case.
func specAllows(fm morph.Form, spec morph.FormSpec) bool {
	for _, k := range spec.Without {
		if _, has := fm.Feats[k]; has {
			return false
		}
	}
	return true
}

// fillTranslations attaches the pack's own translations, where it has them.
func (g *Generator) fillTranslations(exs []Example) {
	for i := range exs {
		rows, err := g.pack.DB().Query(
			`SELECT lang, text FROM sentence_trans WHERE sentence_id = ? AND lang IN ('ru','en')`,
			exs[i].SentenceID)
		if err != nil {
			return // a pack without translations is a pack without translations
		}
		for rows.Next() {
			var lang, text string
			if rows.Scan(&lang, &text) != nil {
				continue
			}
			if lang == "ru" {
				exs[i].TranslationRu = text
			} else {
				exs[i].TranslationEn = text
			}
		}
		rows.Close()
	}
}
