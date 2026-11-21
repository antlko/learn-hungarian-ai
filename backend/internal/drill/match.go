package drill

import (
	"sort"
	"strconv"
	"strings"

	"tanulom/internal/curriculum"
	"tanulom/internal/morph"
	"tanulom/internal/srs"
)

// The match drill: pair 3-5 cues with the forms of ONE word.
//
//	én → látnék   te → látnál   ő → látna        (verb topics: pronoun → form)
//	hol → házban  hova → házba  honnan → házból  (case topics: question → form)
//
// A multi-part recognition item that reuses sort's shape: `Columns` are the CUES
// (one tile belongs under each), `Tiles` are the FORMS, the learner's answer is
// `parts` (tile id -> cue). Every form is an attested form of the lemma, read
// unambiguously, and pairwise distinct; a cell the pack attests with more than
// one string is skipped rather than offered with an alternate.
//
// Grading, like sort, is one review per DISTINCT required feature: Again when
// any pair carrying it is wrong, Good at most otherwise. Person and number are
// the question, never charged (ADR-086). Other nominal topics have no clean cue
// (a feature label is not something to read out), so they get no match item.

const (
	matchMinPairs = 3
	matchMaxPairs = 5
)

// matchCaseGroups groups the topic's own cases by question word. A match needs
// one case per question word, so a topic with fewer than three groups cannot
// make one.
func (g *Generator) matchCaseGroups(t curriculum.Topic) map[string][]Feature {
	out := map[string][]Feature{}
	for _, f := range g.qwordCases(t) {
		out[qwordFor[f.Value]] = append(out[qwordFor[f.Value]], f)
	}
	return out
}

// matchTopic reports whether a topic can structurally have a match item.
func (g *Generator) matchTopic(t curriculum.Topic) bool {
	return g.personTopic(t) || len(g.matchCaseGroups(t)) >= matchMinPairs
}

// cueOrder fixes the column order of a verb match.
var cueOrder = []string{"1|Sing", "2|Sing", "3|Sing", "1|Plur", "2|Plur", "3|Plur"}

type matchPair struct {
	cue, form string
	feats     []string
}

// noRecent drops the recency memory for the word draw. Only a small share of
// lemmas can make a match, and pickWord's steering away from recent lemmas would
// steer away from exactly those, leaving the draws to land on lemmas that cannot
// build one. Freshness is decided afterwards, by fill's stages.
func noRecent(req Request) Request {
	req.Recent = nil
	return req
}

func (g *Generator) matchItem(req Request) (Item, bool) {
	if g.personTopic(req.Topic) {
		return g.matchVerb(req)
	}
	return g.matchCase(req)
}

// uniqueCell returns the one attested form of a lemma for a spec, or false when
// the pack has none or has more than one distinct string.
func (g *Generator) uniqueCell(lemmaID int64, spec morph.FormSpec) (morph.Form, bool) {
	forms, err := g.analyzer.FormsOf(lemmaID)
	if err != nil {
		return morph.Form{}, false
	}
	var found morph.Form
	seen := map[string]bool{}
	for _, f := range forms {
		if !f.Attested() || !fitsSpec(f.Feats, spec) {
			continue
		}
		if !seen[strings.ToLower(f.Form)] {
			seen[strings.ToLower(f.Form)] = true
			found = f
		}
	}
	if len(seen) != 1 {
		return morph.Form{}, false
	}
	return found, true
}

// fitsSpec reports whether a form's tags satisfy a spec exactly.
func fitsSpec(feats map[string]string, spec morph.FormSpec) bool {
	for k, v := range spec.Want {
		if feats[k] != v {
			return false
		}
	}
	for _, k := range spec.Without {
		if _, has := feats[k]; has {
			return false
		}
	}
	return true
}

func (g *Generator) matchCase(req Request) (Item, bool) {
	groups := g.matchCaseGroups(req.Topic)
	if len(groups) < matchMinPairs {
		return Item{}, false
	}
	words := make([]string, 0, len(groups))
	for q := range groups {
		words = append(words, q)
	}
	sort.Strings(words)

	for range maxCandidateTries {
		w, ok := g.pickWord(g.nouns, noRecent(req))
		if !ok {
			continue
		}
		// One case per question word, weak features first, then random.
		req.Rand.Shuffle(len(words), func(i, j int) { words[i], words[j] = words[j], words[i] })
		sort.SliceStable(words, func(i, j int) bool {
			return g.anyWeak(groups[words[i]], req) && !g.anyWeak(groups[words[j]], req)
		})
		var pairs []matchPair
		usedForm := map[string]bool{}
		for _, q := range words {
			cases := groups[q]
			f := cases[req.Rand.Intn(len(cases))]
			form, ok := g.uniqueCell(w.LemmaID, nominalSpec(f))
			if !ok || strings.EqualFold(form.Form, w.Lemma) || usedForm[strings.ToLower(form.Form)] {
				continue
			}
			if _, ok := g.readingOf(form.Form, w.LemmaID, f); !ok {
				continue
			}
			usedForm[strings.ToLower(form.Form)] = true
			pairs = append(pairs, matchPair{cue: q, form: form.Form, feats: []string{f.ID}})
			if len(pairs) >= matchMaxPairs {
				break
			}
		}
		if len(pairs) < matchMinPairs {
			continue
		}
		pairs = pairs[:matchMinPairs+req.Rand.Intn(len(pairs)-matchMinPairs+1)]
		return g.buildMatch(req, w, pairs, "qword"), true
	}
	return Item{}, false
}

func (g *Generator) anyWeak(fs []Feature, req Request) bool {
	for _, f := range fs {
		for _, id := range req.WeakFeatures {
			if f.ID == id {
				return true
			}
		}
	}
	return false
}

func (g *Generator) matchVerb(req Request) (Item, bool) {
	vf := g.personFeatures(req.Topic)
	if len(vf) == 0 {
		return Item{}, false
	}
	allowDef := req.Topic.TeachOrder >= g.definiteFloor()
	allowed := map[string]map[string]bool{}
	for _, f := range vf {
		if allowed[f.Key] == nil {
			allowed[f.Key] = map[string]bool{}
		}
		allowed[f.Key][f.Value] = true
	}

	for range maxCandidateTries {
		w, ok := g.pickWord(g.verbs, noRecent(req))
		if !ok {
			continue
		}
		forms, err := g.analyzer.FormsOf(w.LemmaID)
		if err != nil {
			continue
		}
		// Attested forms of this verb grouped by their Definite/Mood/Tense
		// signature, then by person cell: a group is one "tense set".
		type cellForms map[string]map[string]bool // cell -> distinct lower-case strings
		groups := map[string]cellForms{}
		for _, f := range forms {
			if !f.Attested() {
				continue
			}
			cell := personCell(f.Feats)
			if personPronouns[cell] == "" || !verbalOK(f.Feats, allowed, allowDef) {
				continue
			}
			sig := f.Feats["Definite"] + "|" + f.Feats["Mood"] + "|" + f.Feats["Tense"]
			if groups[sig] == nil {
				groups[sig] = cellForms{}
			}
			if groups[sig][cell] == nil {
				groups[sig][cell] = map[string]bool{}
			}
			groups[sig][cell][strings.ToLower(f.Form)] = true
		}
		sigs := make([]string, 0, len(groups))
		for s := range groups {
			sigs = append(sigs, s)
		}
		sort.Strings(sigs)
		req.Rand.Shuffle(len(sigs), func(i, j int) { sigs[i], sigs[j] = sigs[j], sigs[i] })

		for _, sig := range sigs {
			// Cells with exactly one attested string, and no string shared by
			// two cells (the form would belong under two cues).
			count := map[string]int{}
			for _, strs := range groups[sig] {
				for s := range strs {
					count[s]++
				}
			}
			var pairs []matchPair
			for _, cell := range cueOrder {
				strs := groups[sig][cell]
				if len(strs) != 1 {
					continue
				}
				var form string
				for s := range strs {
					form = s
				}
				if count[form] != 1 {
					continue
				}
				feats, ok := g.personReading(form, w.LemmaID, vf, allowDef)
				if !ok || personCell(feats) != cell {
					continue
				}
				pairs = append(pairs, matchPair{cue: personPronouns[cell], form: form,
					feats: personCarried(vf, feats)})
			}
			if len(pairs) < matchMinPairs {
				continue
			}
			req.Rand.Shuffle(len(pairs), func(i, j int) { pairs[i], pairs[j] = pairs[j], pairs[i] })
			pairs = pairs[:matchMinPairs+req.Rand.Intn(min(len(pairs), matchMaxPairs)-matchMinPairs+1)]
			// Columns keep the pronoun order whatever the draw.
			sort.SliceStable(pairs, func(i, j int) bool { return cueRank(pairs[i].cue) < cueRank(pairs[j].cue) })
			return g.buildMatch(req, w, pairs, "person"), true
		}
	}
	return Item{}, false
}

func cueRank(cue string) int {
	for i, c := range cueOrder {
		if personPronouns[c] == cue {
			return i
		}
	}
	return len(cueOrder)
}

// verbalOK reports whether a form's tags are within what a topic teaches: its
// own features' values; no definite form before the definite conjugation; and
// the indicative unless the topic names the mood.
func verbalOK(feats map[string]string, allowed map[string]map[string]bool, allowDef bool) bool {
	for key, ok := range allowed {
		v, present := feats[key]
		if !present || !ok[v] {
			return false
		}
	}
	if allowed["Mood"] == nil {
		if m := feats["Mood"]; m != "" && m != "Ind" {
			return false
		}
	}
	if !allowDef && allowed["Definite"] == nil && feats["Definite"] == "Def" {
		return false
	}
	return true
}

// buildMatch assembles the item: shuffled tiles, canonical expected lines.
func (g *Generator) buildMatch(req Request, w Word, pairs []matchPair, cueKind string) Item {
	order := req.Rand.Perm(len(pairs))
	tiles := make([]Tile, len(pairs))
	pairFeats := map[string][]string{}
	var lines []string
	columns := make([]string, len(pairs))
	featSet := map[string]bool{}
	for i, p := range pairs {
		columns[i] = p.cue
	}
	for slot, idx := range order {
		p := pairs[idx]
		id := "t" + strconv.Itoa(slot)
		tiles[slot] = Tile{ID: id, Text: p.form, lemmaID: w.LemmaID}
		lines = append(lines, id+"="+p.cue)
		pairFeats[id] = p.feats
		for _, f := range p.feats {
			featSet[f] = true
		}
	}
	feats := make([]string, 0, len(featSet))
	for f := range featSet {
		feats = append(feats, f)
	}
	sort.Strings(feats)
	ru, en := g.gloss(w.LemmaID)
	return Item{
		TopicCode: req.Topic.Code, Kind: KindMatch, Phase: req.Phase,
		PromptKey:  "drill.prompt.match",
		PromptArgs: map[string]string{"lemma": w.Lemma, "cue": cueKind},
		Lemma:      w.Lemma, LemmaID: w.LemmaID, GlossRU: ru, GlossEN: en,
		Columns: columns, Tiles: tiles,
		FeatureIDs: feats, Headline: strings.Join(feats, "+"),
		Attested: true, expected: strings.Join(lines, "\n"), pairFeats: pairFeats,
	}
}

// matchErrorClass is the class of a wrong match: person for a verb, case for a noun.
func (i Item) matchErrorClass() morph.ErrorClass {
	if i.PromptArgs["cue"] == "person" {
		return ErrPerson
	}
	return morph.ErrCase
}

// matchGrades gives one review per DISTINCT required feature: Again if any pair
// carrying it is wrong, otherwise Good at most (a tap is recognition).
func (i Item) matchGrades(results []PartResult, o srs.Outcome) []srs.FeatureGrade {
	wrong := map[string]bool{}
	for _, r := range results {
		for _, f := range i.pairFeats[r.ID] {
			if !r.Correct {
				wrong[f] = true
			}
		}
	}
	var out []srs.FeatureGrade
	for _, f := range i.FeatureIDs {
		oc := o
		oc.Correct = !wrong[f]
		out = append(out, srs.RequiredFeatures([]string{f}, oc, "")...)
	}
	return out
}
