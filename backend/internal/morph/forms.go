package morph

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Form resolution: given a lemma and the grammatical facts you want, what is the
// surface form — and how much should it be trusted?
//
// This exists because the pack's `paradigm` table is built entirely by the
// generator and does NOT consult the attested Wiktionary forms sitting in
// `wordform` beside it. For `ház` in the superessive that means the table says
// `házan` while the attested form is `házon`. A wrong cell in a display table is
// bad; a wrong cell used as the correct answer to a drill is worse, because the
// app would be marking the right answer wrong.
//
// So nothing that has to be RIGHT reads `paradigm`. It reads this, which applies
// the same trust hierarchy as Lookup: attested beats generated, high confidence
// beats low, and a shorter (less speculative) form breaks the remaining ties.
// See langpack/pipeline/inflect.py, which carries the same rule at build time.

// Form is one surface realisation with its provenance.
type Form struct {
	Form       string            `json:"form"`
	Feats      map[string]string `json:"feats"`
	Source     string            `json:"source"`
	Confidence string            `json:"confidence"`
}

// Attested reports whether the form came from a corpus or a dictionary rather
// than from our own generator. Only attested forms may be presented as the
// correct answer without a caveat.
func (f Form) Attested() bool { return f.Source == "corpus" || f.Source == "wiktionary" }

// trustRank orders forms the way Lookup does. Lower is better.
func trustRank(source, confidence string) int {
	s := map[string]int{"corpus": 0, "wiktionary": 1, "generated": 2}[source]
	c := map[string]int{"high": 0, "medium": 1, "low": 2}[confidence]
	return s*3 + c
}

// FormsOf returns every recorded form of a lemma, best-trusted first.
//
// Indexed on lemma_id, and a noun carries on the order of 175 rows, so this is a
// cheap query even in a request path.
func (a *Analyzer) FormsOf(lemmaID int64) ([]Form, error) {
	if a == nil || a.pack == nil {
		return nil, nil
	}
	rows, err := a.pack.DB().Query(`
		SELECT form, feats, source, confidence FROM wordform WHERE lemma_id = ?`, lemmaID)
	if err != nil {
		return nil, fmt.Errorf("forms of lemma %d: %w", lemmaID, err)
	}
	defer rows.Close()

	var out []Form
	for rows.Next() {
		var f Form
		var feats string
		if err := rows.Scan(&f.Form, &feats, &f.Source, &f.Confidence); err != nil {
			return nil, err
		}
		if !plausibleForm(f.Form) {
			continue
		}
		f.Feats = parseFeats(feats)
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := trustRank(out[i].Source, out[i].Confidence), trustRank(out[j].Source, out[j].Confidence)
		if ri != rj {
			return ri < rj
		}
		return len(out[i].Form) < len(out[j].Form)
	})
	return out, nil
}

// plausibleForm rejects rows that are not wordforms at all.
//
// Wiktionary declension tables write an empty cell as `-`, and the pack importer
// took those literally: `-` is currently stored as the essive of several thousand
// nouns. A form with no letters in it cannot be an answer to anything, and it
// would otherwise win the trust sort outright — it is attested, high-confidence
// and the shortest candidate. Also fixed in langpack/pipeline/wiktionary.py for
// future packs; this guard keeps existing packs safe.
func plausibleForm(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// Matches reports whether a form carries every requested feature. Extra features
// on the form are fine: asking for Case=Ine must match the inessive singular
// without having to spell out Number=Sing.
func (f Form) Matches(want map[string]string) bool {
	for k, v := range want {
		if got, ok := f.Feats[k]; !ok || got != v {
			return false
		}
	}
	return true
}

// exactlyOn reports whether the form carries the wanted features and nothing
// else from the same feature KEYS that would make it a different cell. Used when
// a drill needs the plain inessive and must not be handed the possessive one.
func (f Form) hasNoneOf(forbidden []string) bool {
	for _, key := range forbidden {
		if _, ok := f.Feats[key]; ok {
			return false
		}
	}
	return true
}

// FormSpec asks for one cell of a paradigm.
type FormSpec struct {
	// Want are the features the form must carry.
	Want map[string]string
	// Without are feature KEYS the form must not carry at all. This is how a
	// caller says "the plain inessive, not the possessed one" — `házaimban` also
	// matches Case=Ine, and offering it as the answer to a case drill would be a
	// different and much harder question than the one asked.
	Without []string
}

// BestForm returns the most trustworthy form matching a spec.
func (a *Analyzer) BestForm(lemmaID int64, spec FormSpec) (Form, bool) {
	forms, err := a.FormsOf(lemmaID)
	if err != nil {
		return Form{}, false
	}
	for _, f := range forms {
		if f.Matches(spec.Want) && f.hasNoneOf(spec.Without) {
			return f, true
		}
	}
	return Form{}, false
}

// IsAttested reports whether an exact surface string exists in the pack.
//
// Used as a guardrail on anything we construct rather than look up: a drill
// distractor built by swapping an allomorph is fine, but the CORRECT answer must
// be a form the language actually has.
func (a *Analyzer) IsAttested(form string) bool {
	if a == nil || a.pack == nil {
		return false
	}
	marks, args := placeholders(caseVariants(form))
	var n int
	err := a.pack.DB().QueryRow(
		`SELECT COUNT(*) FROM wordform
		  WHERE form IN (`+marks+`) AND source != 'generated'`, args...).Scan(&n)
	return err == nil && n > 0
}

// Nominal case slots a learner is shown, in the order the locative grid reads.
// Mirrors _DISPLAY_CASES in langpack/pipeline/inflect.py.
var displayCases = []string{
	"Nom", "Acc", "Dat", "Ins",
	"Ine", "Ill", "Ela", "Sup", "Sbl", "Del", "Ade", "All", "Abl",
}

// SlotFor derives the paradigm slot name of a feature set, or "" if the form
// does not belong in the display table. The naming matches the pack's `paradigm`
// table exactly, so the two can be merged cell by cell.
func SlotFor(feats map[string]string) string {
	if _, ok := feats["VerbForm"]; ok {
		return ""
	}
	if _, ok := feats["Person"]; ok {
		return "" // a conjugated verb, not a nominal cell
	}
	caseVal := feats["Case"]
	if caseVal == "" {
		caseVal = "Nom"
	}
	if psor, ok := feats["Person[psor]"]; ok {
		// Only the base possessive cells go in the table; stacked
		// possessive+case forms belong to the lookup index, not to a grid a
		// human is meant to read.
		if caseVal != "Nom" {
			return ""
		}
		num := feats["Number[psor]"]
		if num == "" {
			return ""
		}
		possessum := ""
		if feats["Number[psed]"] == "Plur" {
			possessum = "Pl"
		}
		return "Poss" + possessum + "." + psor + num
	}
	found := false
	for _, c := range displayCases {
		if c == caseVal {
			found = true
			break
		}
	}
	if !found {
		return ""
	}
	number := feats["Number"]
	if number == "" {
		number = "Sing"
	}
	return caseVal + "." + number
}

// Paradigm builds the display table from the lookup index, preferring attested
// forms, and falls back to the pack's precomputed table for any cell the index
// cannot fill.
//
// The fallback matters for lemmas outside the 5 000 the generator covered; the
// preference matters because the precomputed table is generator-only and
// therefore wrong wherever the generator and the dictionary disagree.
func (a *Analyzer) Paradigm(lemmaID int64) (map[string]string, map[string]bool, error) {
	if a == nil || a.pack == nil {
		return nil, nil, nil
	}
	forms, err := a.FormsOf(lemmaID)
	if err != nil {
		return nil, nil, err
	}
	out := map[string]string{}
	attested := map[string]bool{}
	for _, f := range forms {
		slot := SlotFor(f.Feats)
		if slot == "" {
			continue
		}
		if _, taken := out[slot]; taken {
			continue // forms are trust-ordered, so the first one wins
		}
		out[slot] = f.Form
		attested[slot] = f.Attested()
	}

	rows, err := a.pack.DB().Query(
		`SELECT slot, form FROM paradigm WHERE lemma_id = ?`, lemmaID)
	if err != nil {
		return out, attested, nil
	}
	defer rows.Close()
	for rows.Next() {
		var slot, form string
		if err := rows.Scan(&slot, &form); err != nil {
			continue
		}
		if _, taken := out[slot]; !taken {
			out[slot] = form
			attested[slot] = false
		}
	}
	return out, attested, nil
}

// FeatsKey renders a feature map the way the pack stores it, for logging and
// for round-tripping a spec through the API.
func FeatsKey(feats map[string]string) string {
	return strings.Join(featureIDs(feats), "|")
}
