package morph

import (
	"encoding/json"
	"log/slog"
	"sort"
	"strings"
	"sync"
)

// The suffix-stripping fallback for forms the pack has never seen.
//
// Hungarian is agglutinative and productive: no finite table can contain every
// form, and proper nouns, loanwords and compounds guarantee misses. Rather than
// give up, we peel known suffixes off the end and check whether what remains is
// a lemma we know.
//
// Anything this produces is marked DerivedAnalysis, because a stripped analysis
// is a guess: `koros` could be `kor` + a derivational suffix, or its own lemma.
// The UI shows derived readings differently, and the review scheduler does not
// credit morphological features from them.

type suffixEntry struct {
	suffix    string
	featureID string
	key       string
	value     string
}

// suffixTable is per-Analyzer, not package-level.
//
// A package-level sync.Once would load the table from whichever pack happened to
// be opened first and then serve it to every later pack — silently wrong after a
// pack upgrade, and wrong across tests that use different fixtures.
type suffixTable struct {
	once    sync.Once
	entries []suffixEntry
}

// loadSuffixes reads the allomorph tables out of the pack once per Analyzer,
// longest suffix first so that stripping is greedy: `-ban` before `-n`.
func (a *Analyzer) loadSuffixes() []suffixEntry {
	a.suffixes.once.Do(func() {
		rows, err := a.pack.DB().Query(
			`SELECT id, key, value, allomorphs FROM morph_feature WHERE allomorphs != '{}'`)
		if err != nil {
			slog.Warn("suffix fallback unavailable", "error", err)
			return
		}
		defer rows.Close()

		seen := map[string]bool{}
		for rows.Next() {
			var id, key, value, raw string
			if err := rows.Scan(&id, &key, &value, &raw); err != nil {
				continue
			}
			var allo map[string]string
			if json.Unmarshal([]byte(raw), &allo) != nil {
				continue
			}
			for _, s := range allo {
				if s == "" || strings.Contains(s, "…") {
					continue
				}
				k := s + "\x00" + id
				if seen[k] {
					continue
				}
				seen[k] = true
				a.suffixes.entries = append(a.suffixes.entries,
					suffixEntry{suffix: s, featureID: id, key: key, value: value})
			}
		}
		sort.Slice(a.suffixes.entries, func(i, j int) bool {
			return len(a.suffixes.entries[i].suffix) > len(a.suffixes.entries[j].suffix)
		})
	})
	return a.suffixes.entries
}

// lemmaExists reports whether a string is a known lemma, and returns its row.
func (a *Analyzer) lemmaExists(s string) (id int64, lemma, upos string, ok bool) {
	// IN over the case variants rather than COLLATE NOCASE: the latter cannot
	// use idx_lemma_lemma and turns every miss into a full scan. See casefold.go.
	marks, args := placeholders(caseVariants(s))
	err := a.pack.DB().QueryRow(
		`SELECT id, lemma, upos FROM lemma WHERE lemma IN (`+marks+`)
		  ORDER BY COALESCE(freq_rank, 1000000) LIMIT 1`, args...,
	).Scan(&id, &lemma, &upos)
	return id, lemma, upos, err == nil
}

// fallback strips up to two suffixes and checks whether a lemma remains.
//
// Two, not more: Hungarian stacks further than that (`ház+a+im+ban`), but each
// additional layer multiplies false positives, and the pack already contains the
// generated stacked forms for frequent nouns. This is the safety net for the
// tail, not the main path.
func (a *Analyzer) fallback(form string) (Analysis, bool) {
	if a == nil || a.pack == nil || len(form) < 3 {
		return Analysis{}, false
	}
	lower := strings.ToLower(form)
	table := a.loadSuffixes()

	type peeled struct {
		stem  string
		feats map[string]string
	}

	// Depth 1, then depth 2.
	level := []peeled{{stem: lower, feats: map[string]string{}}}
	for depth := 0; depth < 2; depth++ {
		var next []peeled
		for _, p := range level {
			for _, e := range table {
				if len(p.stem) <= len(e.suffix)+1 || !strings.HasSuffix(p.stem, e.suffix) {
					continue
				}
				if _, taken := p.feats[e.key]; taken {
					continue // a form cannot carry two values of the same feature
				}
				stem := p.stem[:len(p.stem)-len(e.suffix)]

				feats := make(map[string]string, len(p.feats)+1)
				for k, v := range p.feats {
					feats[k] = v
				}
				feats[e.key] = e.value

				if id, lemma, upos, ok := a.lemmaExists(stem); ok {
					return Analysis{
						Form: form, Lemma: lemma, LemmaID: id, UPOS: upos,
						Feats: feats, FeatureID: featureIDs(feats),
						Source: "fallback", Certainty: DerivedAnalysis,
					}, true
				}
				next = append(next, peeled{stem: stem, feats: feats})
			}
		}
		if len(next) > 64 {
			next = next[:64] // bound the search; the tail is not worth the time
		}
		level = next
	}
	return Analysis{}, false
}
