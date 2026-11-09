package morph

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"tanulom/internal/lang"
)

// jsonUnmarshal and sortStrings keep segment.go free of direct imports it would
// otherwise need for two one-liners.
func jsonUnmarshal(s string, dst any) error { return json.Unmarshal([]byte(s), dst) }
func sortStrings(s []string)                { sort.Strings(s) }

// Certainty describes how much to trust an analysis. It is surfaced in the API
// and rendered in the UI: an uncertain analysis must never look like a confident
// one, because a learner who trusts a wrong breakdown learns the wrong thing.
type Certainty string

const (
	// CertainAnalysis: a single attested analysis for this form.
	CertainAnalysis Certainty = "certain"
	// UncertainAnalysis: several analyses matched; the frequency prior chose.
	UncertainAnalysis Certainty = "uncertain"
	// DerivedAnalysis: no attested form; produced by the generator or the
	// suffix fallback.
	DerivedAnalysis Certainty = "derived"
	// UnknownAnalysis: nothing matched at all.
	UnknownAnalysis Certainty = "unknown"
	// CorrectedAnalysis: the learner overrode the pack's reading. Shown as
	// such — an override must not masquerade as the dictionary.
	CorrectedAnalysis Certainty = "corrected"
)

// Analysis is one reading of a wordform.
type Analysis struct {
	Form      string            `json:"form"`
	Lemma     string            `json:"lemma"`
	LemmaID   int64             `json:"lemmaId"`
	UPOS      string            `json:"upos"`
	Feats     map[string]string `json:"feats"`
	FeatureID []string          `json:"featureIds"`
	Source    string            `json:"source"`
	Certainty Certainty         `json:"certainty"`
	FreqRank  *int              `json:"freqRank,omitempty"`
	// Alternatives counts the other readings that were rejected, so the UI can
	// say "3 possible readings" rather than silently picking one.
	Alternatives int `json:"alternatives"`
	// Via records HOW this reading was chosen when it was not the pack's own
	// default: "context" = an opt-in sidecar picked it among the pack's
	// candidates (ADR-054). Empty is the ordinary case. The Certainty is left
	// as it was — a contextual choice is still a choice, not a fact.
	Via string `json:"via,omitempty"`
}

// ViaContext marks a reading chosen by the HuSpaCy sidecar.
const ViaContext = "context"

// Analyzer resolves wordforms against a language pack.
type Analyzer struct {
	pack     *lang.Pack
	suffixes suffixTable

	// alloTables caches morph_feature.allomorphs per feature id, for the
	// segmenter and the explainer. Loaded once per analyzer, read-only after.
	alloOnce   sync.Once
	alloTables map[string]map[string]string

	labelCache featureLabels
}

func NewAnalyzer(p *lang.Pack) *Analyzer { return &Analyzer{pack: p} }

// featureIDs renders a feats map as sorted UD identifiers, which is the form
// morph_state is keyed by.
func featureIDs(feats map[string]string) []string {
	out := make([]string, 0, len(feats))
	for k, v := range feats {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

// ParseFeats turns "Case=Ine|Number=Sing" into a map.
func ParseFeats(s string) map[string]string { return parseFeats(s) }

// FeatureIDs renders a feats map as sorted UD identifiers.
func FeatureIDs(feats map[string]string) []string { return featureIDs(feats) }

// parseFeats turns "Case=Ine|Number=Sing" into a map.
func parseFeats(s string) map[string]string {
	out := map[string]string{}
	if s == "" {
		return out
	}
	for _, part := range strings.Split(s, "|") {
		if k, v, ok := strings.Cut(part, "="); ok {
			out[k] = v
		}
	}
	return out
}

// Lookup returns every reading of a form, best first.
//
// Ordering encodes the trust hierarchy: attested beats generated, high
// confidence beats low, and a frequent lemma beats a rare one. That last tie-break
// is the frequency prior — without a sidecar we cannot disambiguate `vár`
// (castle) from `vár` (waits) in context, so we pick the commoner and say so.
func (a *Analyzer) Lookup(form string) ([]Analysis, error) {
	if a == nil || a.pack == nil {
		return nil, nil
	}
	// Case-insensitively, but through the index — see casefold.go.
	marks, args := placeholders(caseVariants(form))
	rows, err := a.pack.DB().Query(`
		SELECT w.form, l.lemma, l.id, w.upos, w.feats, w.source, w.confidence, l.freq_rank
		  FROM wordform w
		  JOIN lemma l ON l.id = w.lemma_id
		 WHERE w.form IN (`+marks+`)
		 ORDER BY
		   CASE w.source     WHEN 'corpus' THEN 0 WHEN 'wiktionary' THEN 1 ELSE 2 END,
		   CASE w.confidence WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END,
		   COALESCE(l.freq_rank, 1000000)
		 LIMIT 25`, args...)
	if err != nil {
		return nil, fmt.Errorf("lookup %q: %w", form, err)
	}
	defer rows.Close()

	var out []Analysis
	for rows.Next() {
		var an Analysis
		var feats, confidence string
		var rank sql.NullInt64
		if err := rows.Scan(&an.Form, &an.Lemma, &an.LemmaID, &an.UPOS,
			&feats, &an.Source, &confidence, &rank); err != nil {
			return nil, fmt.Errorf("scan analysis: %w", err)
		}
		an.Feats = parseFeats(feats)
		an.FeatureID = featureIDs(an.Feats)
		if rank.Valid {
			r := int(rank.Int64)
			an.FreqRank = &r
		}
		if an.Source == "generated" {
			an.Certainty = DerivedAnalysis
		} else {
			an.Certainty = CertainAnalysis
		}
		out = append(out, an)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Distinct lemma readings are what "ambiguous" means. Two rows for the same
	// lemma differing only in how completely they are tagged are not a conflict.
	lemmas := map[int64]struct{}{}
	for _, an := range out {
		lemmas[an.LemmaID] = struct{}{}
	}
	if len(lemmas) > 1 {
		for i := range out {
			if out[i].Certainty == CertainAnalysis {
				out[i].Certainty = UncertainAnalysis
			}
		}
	}
	for i := range out {
		out[i].Alternatives = len(lemmas) - 1
	}
	return out, nil
}

// Best returns the single most likely reading, or an unknown-marked analysis.
func (a *Analyzer) Best(form string) (Analysis, error) {
	all, err := a.Lookup(form)
	if err != nil {
		return Analysis{}, err
	}
	if len(all) > 0 {
		return all[0], nil
	}
	// Nothing attested: try to strip suffixes before giving up.
	if derived, ok := a.fallback(form); ok {
		return derived, nil
	}
	return Analysis{
		Form: form, Lemma: form, Feats: map[string]string{},
		FeatureID: []string{}, Certainty: UnknownAnalysis,
	}, nil
}

// AnalyzedToken pairs a token with its reading.
type AnalyzedToken struct {
	Token
	Analysis Analysis `json:"analysis"`
}

// AnalyzeText tokenises and analyses a whole text.
func (a *Analyzer) AnalyzeText(text string) ([]AnalyzedToken, []Sentence, error) {
	tokens := Tokenize(text)
	out := make([]AnalyzedToken, 0, len(tokens))

	// A text repeats words heavily; caching turns an article into a few hundred
	// queries instead of a few thousand.
	cache := map[string]Analysis{}

	for _, t := range tokens {
		at := AnalyzedToken{Token: t}
		if t.IsWord {
			key := strings.ToLower(t.Text)
			an, ok := cache[key]
			if !ok {
				var err error
				an, err = a.Best(t.Text)
				if err != nil {
					return nil, nil, err
				}
				cache[key] = an
			}
			at.Analysis = an
		}
		out = append(out, at)
	}
	return out, SplitSentences(text, tokens), nil
}
