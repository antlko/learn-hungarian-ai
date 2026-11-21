package drill

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"tanulom/internal/curriculum"
	"tanulom/internal/morph"
)

// The person drill: type the verb of a real sentence in the person a pronoun
// asks for.
//
//	___ a házat. (én, látni)   ->   Látom
//
// What makes it safe is where the answer comes from: it is the token the corpus
// sentence actually contains, read UNAMBIGUOUSLY for the lemma by the pack's
// analyses. Verb forms are generated for recognition, never production (see
// INVARIANTS), so a generated reading is refused outright — a person drill
// marked against our own guess of `látsz` would be teaching from a guess.

// ErrPerson is a typed answer that IS a form of the right verb, in another
// person. It lives here rather than in morph: only this drill can tell it, since
// only it knows which person was asked for.
const ErrPerson morph.ErrorClass = "person"

// personPronouns is the closed table of Person+Number -> the pronoun shown.
// Third person singular is "ő/Ön": Hungarian polite address takes the same verb
// form, so there is no separate Ön row. The plural third person is ők.
var personPronouns = map[string]string{
	"1|Sing": "én", "2|Sing": "te", "3|Sing": "ő/Ön",
	"1|Plur": "mi", "2|Plur": "ti", "3|Plur": "ők",
}

func personCell(a map[string]string) string { return a["Person"] + "|" + a["Number"] }

// verbalFeature reports whether a feature is one the person drill can ask for:
// the conjugation, the present and past, the indicative, conditional and
// imperative. The potential carries no features on its attested forms and the
// infinitive has no person, so neither is here.
func verbalFeature(f Feature) bool {
	switch f.Key {
	case "Definite":
		return f.Value == "Ind" || f.Value == "Def"
	case "Tense":
		return f.Value == "Pres" || f.Value == "Past"
	case "Mood":
		return f.Value == "Ind" || f.Value == "Cnd" || f.Value == "Imp"
	}
	return false
}

// personFeatures are the topic's verbal features, in teach order.
func (g *Generator) personFeatures(t curriculum.Topic) []Feature {
	var out []Feature
	for _, id := range t.FeatureIDs {
		if f, ok := g.features[id]; ok && verbalFeature(f) {
			out = append(out, f)
		}
	}
	return out
}

// personTopic reports whether a topic is about the verb: it has a verbal
// feature and nothing nominal. `van-nincs` carries Case=Nom next to Tense=Pres
// and is the copula, not a conjugation to practise; `pronouns` teaches Person
// itself, which this drill deliberately never grades.
func (g *Generator) personTopic(t curriculum.Topic) bool {
	verbal := false
	for _, id := range t.FeatureIDs {
		f, ok := g.features[id]
		if !ok {
			continue
		}
		switch {
		case verbalFeature(f):
			verbal = true
		case f.Key == "Skill":
		default:
			return false
		}
	}
	return verbal
}

// definiteFloor is the teach order from which Definite=Def tokens may appear.
// Before the definite conjugation is taught, an item whose only reading is the
// definite form (`láttad` for a sentence with a definite object) would ask the
// learner for something nobody has explained yet.
func (g *Generator) definiteFloor() int {
	if t, ok := g.dag.Topic("definite-conjugation"); ok {
		return t.TeachOrder
	}
	return math.MaxInt
}

// personReading returns the verbal features of a token when it reads
// UNAMBIGUOUSLY, for the given lemma, as one cell of the person x number grid
// and as the topic's own features. The second result is false for any doubt.
//
// Refusals, each load-bearing:
//   - generated readings (the answer must be attested);
//   - a reading of the lemma with no person (an infinitive or participle
//     spelled the same), or two readings in different cells — the pronoun would
//     not determine the answer;
//   - a reading whose value for one of the topic's features differs from the
//     others (`láttam` is both definite and indefinite; for a topic that
//     teaches the contrast the string says nothing about it).
//
// Before the definite conjugation is taught a token must also have at least one
// non-definite reading.
func (g *Generator) personReading(form string, lemmaID int64, vf []Feature, allowDef bool) (map[string]string, bool) {
	all, err := g.analyzer.Lookup(form)
	if err != nil {
		return nil, false
	}
	allowed := map[string]map[string]bool{}
	for _, f := range vf {
		if allowed[f.Key] == nil {
			allowed[f.Key] = map[string]bool{}
		}
		allowed[f.Key][f.Value] = true
	}
	vals := map[string]string{}
	cell, person, number := "", "", ""
	matched, nonDefinite := false, false
	// The topic's moods; a topic that names none teaches the indicative only, so
	// a subjunctive (`Kérte, hogy marad`) is outside what it asks for.
	moods := allowed["Mood"]
	for _, a := range all {
		if a.Source == "generated" {
			continue
		}
		// Any reading of the exact token as imperative, subjunctive or
		// conditional (of any verb) settles it, unless the topic teaches it.
		if m := a.Feats["Mood"]; (m == "Imp" || m == "Sub" || m == "Cnd") && !moods[m] {
			return nil, false
		}
		// The same string as a form of ANOTHER verb (`jelennek` is jelenik, not
		// jelent; the corpus lemma is wrong or the form is a homograph): the
		// pronoun would not determine the answer.
		if a.LemmaID != lemmaID {
			if a.UPOS == "VERB" {
				return nil, false
			}
			continue
		}
		if m, ok := a.Feats["Mood"]; ok && moods == nil && m != "Ind" {
			return nil, false
		}
		p, n := a.Feats["Person"], a.Feats["Number"]
		if p == "" || n == "" {
			return nil, false
		}
		if cell == "" {
			cell, person, number = personCell(a.Feats), p, n
		} else if cell != personCell(a.Feats) {
			return nil, false
		}
		for key, ok := range allowed {
			v, present := a.Feats[key]
			if !present {
				continue
			}
			if !ok[v] {
				return nil, false
			}
			if prev, seen := vals[key]; seen && prev != v {
				return nil, false
			}
			vals[key] = v
		}
		if a.Feats["Definite"] != "Def" {
			nonDefinite = true
		}
		matched = true
	}
	if !matched {
		return nil, false
	}
	for key := range allowed {
		if _, ok := vals[key]; !ok {
			return nil, false
		}
	}
	if !allowDef && !nonDefinite {
		return nil, false
	}
	vals["Person"], vals["Number"] = person, number
	return vals, true
}

// maxPersonSentences bounds how many corpus sentences of one verb are read when
// looking for usable tokens.
const maxPersonSentences = 400

// personCand is one usable (sentence, token) of a verb for a topic.
type personCand struct {
	sentID     int64
	hu         string
	start, end int
	feats      map[string]string
}

// personCandidates lists the sentences of a verb whose verb token qualifies for
// a topic. Most random (verb, sentence) draws do not qualify — the verb is in the
// wrong tense, or the form is ambiguous — so qualifying ones are found once per
// (topic, verb) and cached, rather than re-discovered by trial on every request.
func (g *Generator) personCandidates(t curriculum.Topic, vf []Feature, allowDef bool, lemmaID int64) []personCand {
	key := t.Code + "|" + strconv.FormatInt(lemmaID, 10)
	g.probeMu.Lock()
	cached, ok := g.personCache[key]
	g.probeMu.Unlock()
	if ok {
		return cached
	}

	var out []personCand
	rows, err := g.pack.DB().Query(`
		SELECT t.sentence_id, COALESCE(t.form,''), s.hu FROM sentence_token t
		  JOIN sentence s ON s.id = t.sentence_id
		 WHERE t.lemma_id = ? AND s.source <> 'authored' LIMIT ?`, lemmaID, maxPersonSentences)
	if err == nil {
		type row struct {
			sid      int64
			form, hu string
		}
		var all []row
		for rows.Next() {
			var r row
			if rows.Scan(&r.sid, &r.form, &r.hu) == nil {
				all = append(all, r)
			}
		}
		rows.Close()

		formOK := map[string]map[string]string{}
		read := func(form string) map[string]string {
			if v, known := formOK[form]; known {
				return v
			}
			feats, ok := g.personReading(form, lemmaID, vf, allowDef)
			if !ok || personPronouns[personCell(feats)] == "" {
				feats = nil
			}
			formOK[form] = feats
			return feats
		}
		var lemma string
		_ = g.pack.DB().QueryRow(`SELECT lemma FROM lemma WHERE id = ?`, lemmaID).Scan(&lemma)
		teachesImp := false
		for _, f := range vf {
			if f.Key == "Mood" && (f.Value == "Imp" || f.Value == "Cnd") {
				teachesImp = true
			}
		}
		seen := map[int64]bool{}
		for _, r := range all {
			if seen[r.sid] {
				continue
			}
			tokens := morph.Tokenize(r.hu)
			if n := wordCount(tokens); n < 3 || n > 14 {
				continue
			}
			prev := ""
			for _, tk := range tokens {
				if tk.IsWord {
					before := prev
					prev = strings.ToLower(tk.Text)
					// `Ne aggódj!` — a verb right after the negative particle is
					// an imperative or subjunctive, which the pack leaves untagged.
					if before == "ne" && !teachesImp {
						continue
					}
					// Untagged subjunctive stems: -gy-/-j- that the lemma lacks.
					if !teachesImp && subjunctiveLike(tk.Text, lemma) {
						continue
					}
				}
				// The pack names the token's surface form; without it (a bare
				// fixture) every token is tried.
				if !tk.IsWord || (r.form != "" && tk.Text != r.form) {
					continue
				}
				if feats := read(tk.Text); feats != nil {
					out = append(out, personCand{r.sid, r.hu, tk.Start, tk.End, feats})
					seen[r.sid] = true
					break
				}
			}
		}
	}

	g.probeMu.Lock()
	if g.personCache == nil {
		g.personCache = map[string][]personCand{}
	}
	g.personCache[key] = out
	g.probeMu.Unlock()
	return out
}

var subjunctiveEnd = regexp.MustCompile(`j(ak|ek|ál|él|on|en|unk|ünk|atok|etek|anak|enek)?$`)

// subjunctiveLike spots a subjunctive/imperative the pack left untagged
// (`aggódj`, `vegyek`, `tegyek`, `adjon`): the form has a -gy- or a final -j- (with
// a person ending) that the dictionary form lacks. Definite -ja/-juk forms do
// not match, and a lemma that itself contains the letters is left alone.
func subjunctiveLike(form, lemma string) bool {
	f, l := strings.ToLower(form), strings.ToLower(lemma)
	if l == "" {
		return false
	}
	// The present of `van` is indicative; its subjunctive (legyek) is not.
	switch f {
	case "vagyok", "vagy", "vagyunk", "vagytok":
		return false
	}
	if strings.Contains(f, "gy") && !strings.Contains(l, "gy") {
		return true
	}
	return subjunctiveEnd.MatchString(f) && !strings.Contains(l, "j")
}

// personItemFrom builds the item for one (verb, sentence) candidate.
func (g *Generator) personItemFrom(req Request, vf []Feature, w Word, c personCand) Item {
	// Only the topic's own features the form carries are graded. Person and
	// number are the question, not the lesson: they are never charged.
	carried := personCarried(vf, c.feats)
	ru, en := g.gloss(w.LemmaID)
	return Item{
		TopicCode: req.Topic.Code, Kind: KindPerson, Phase: req.Phase,
		PromptKey: "drill.prompt.person",
		PromptArgs: map[string]string{
			"lemma": w.Lemma, "pronoun": personPronouns[personCell(c.feats)],
		},
		Frame: c.hu[:c.start] + BlankMark + c.hu[c.end:],
		Lemma: w.Lemma, LemmaID: w.LemmaID, GlossRU: ru, GlossEN: en,
		// The headline is the first carried feature, not the one the draw aimed
		// at: one (verb, sentence) must have one key.
		FeatureIDs: carried, Headline: carried[0],
		Attested: true, SentenceID: c.sentID, expected: c.hu[c.start:c.end],
		answerPerson: personCell(c.feats),
	}
}

func personCarried(vf []Feature, feats map[string]string) []string {
	var carried []string
	for _, f := range vf {
		if feats[f.Key] == f.Value {
			carried = append(carried, f.ID)
		}
	}
	return carried
}

// personItem builds one person item from a random verb and sentence. Used by the
// availability probe and by interleaved practice; a person-only session goes
// through fillPerson, which enumerates instead of drawing.
func (g *Generator) personItem(req Request) (Item, bool) {
	vf := g.personFeatures(req.Topic)
	if len(vf) == 0 {
		return Item{}, false
	}
	allowDef := req.Topic.TeachOrder >= g.definiteFloor()

	for range maxCandidateTries {
		target := g.weightedFeature(vf, req)
		w, ok := g.pickWord(g.verbs, req)
		if !ok {
			continue
		}
		var cands []personCand
		for _, c := range g.personCandidates(req.Topic, vf, allowDef, w.LemmaID) {
			if c.feats[target.Key] == target.Value {
				cands = append(cands, c)
			}
		}
		if len(cands) == 0 {
			continue
		}
		return g.personItemFrom(req, vf, w, cands[req.Rand.Intn(len(cands))]), true
	}
	// No sentence found (a thin topic): a sentence-less item rather than none.
	if g.personSentenceTotal(req.Topic, vf, allowDef) < bareThreshold {
		return g.randomBare(req, vf)
	}
	return Item{}, false
}

type personRef struct {
	w Word
	c personCand
}

// fillPerson builds a person-only session by ENUMERATING the topic's (verb,
// sentence) candidates rather than drawing at random. With a few dozen usable
// verbs, random draws mostly land on lemmas or sentences already used, the
// attempt budget runs out and the relaxation stages let exact repeats through
// while fresh items still exist. Here the rejection sets are applied first and
// the choice is made from what survives; a stage relaxes only when the
// survivors are genuinely too few. Weak-feature oversampling still picks the
// target feature per item.
func (g *Generator) fillPerson(req Request) []Item {
	vf := g.personFeatures(req.Topic)
	if len(vf) == 0 {
		return nil
	}
	allowDef := req.Topic.TeachOrder >= g.definiteFloor()

	// Verbs in random order, taken until enough strictly fresh candidates exist
	// (or the band is exhausted).
	ceiling := bandFor(req.Topic.Level)
	end := sort.Search(len(g.verbs), func(i int) bool { return g.verbs[i].Rank > ceiling })
	if end < 20 {
		end = min(len(g.verbs), 20)
	}
	order := req.Rand.Perm(end)
	// Two candidates per verb, not all of them: one verb with hundreds of
	// sentences would otherwise fill the pool and starve the lemma variety. The
	// rest are kept in reserve for a pool that turns out too small.
	const perVerb = 2
	var refs, reserve []personRef
	strict, verbsUsed := 0, 0
	for _, i := range order {
		w := g.verbs[i]
		cands := g.personCandidates(req.Topic, vf, allowDef, w.LemmaID)
		cands = append([]personCand(nil), cands...)
		req.Rand.Shuffle(len(cands), func(a, b int) { cands[a], cands[b] = cands[b], cands[a] })
		if len(cands) > 0 {
			verbsUsed++
		}
		for k, c := range cands {
			if k >= perVerb {
				reserve = append(reserve, personRef{w, c})
				continue
			}
			refs = append(refs, personRef{w, c})
			if stageOf(g.personProbe(vf, w, c), req.Recent, nil, nil) == 0 {
				strict++
			}
		}
		if strict >= req.Count*4 {
			break
		}
	}
	// At most perVerb sentences of one verb per request while there are enough
	// distinct verbs (half the request) to fill it; a smaller pool opens the rest.
	if verbsUsed < (req.Count+1)/2 || len(refs) < req.Count {
		refs = append(refs, reserve...)
	}
	req.Rand.Shuffle(len(refs), func(i, j int) { refs[i], refs[j] = refs[j], refs[i] })

	// A thin topic also offers sentence-less items. They rank one half-stage
	// behind a sentence item of the same freshness, so sentences win a tie — but
	// a thin topic would otherwise serve its few sentences over and over and never
	// a bare item, so about half of each session (at least 3 of 10) is MIXED in
	// from the bare cells while fresh ones exist.
	var brefs []bareRef
	if g.personSentenceTotal(req.Topic, vf, allowDef) < bareThreshold {
		brefs = append(brefs, g.bareRefs(req.Topic, vf)...)
		req.Rand.Shuffle(len(brefs), func(i, j int) { brefs[i], brefs[j] = brefs[j], brefs[i] })
	}

	bareQuota := 0
	if len(brefs) > 0 {
		bareQuota = req.Count / 2
		if req.Count >= 6 {
			bareQuota = max(bareQuota, 3)
		}
	}
	bareTaken := 0

	items := make([]Item, 0, req.Count)
	lemmaUse := map[int64]int{}
	sentences := map[int64]bool{}
	for len(items) < req.Count && (len(refs) > 0 || len(brefs) > 0) {
		forceBare := bareTaken < bareQuota
		target := g.weightedFeature(vf, req)
		best, bestStage, bestAge := -1, 99, -1
		bestBare, bestBareScore := -1, 99
		for i, r := range brefs {
			if r.feats[target.Key] != target.Value {
				continue
			}
			st := stageOf(g.bareProbe(vf, r), req.Recent, lemmaUse, sentences)
			if score := st*2 + 1; score < bestBareScore {
				bestBare, bestBareScore = i, score
			}
			if st == 0 {
				break
			}
		}
		if forceBare && bestBare < 0 {
			// The weak target has no bare cell: any fresh one will do.
			for i, r := range brefs {
				if st := stageOf(g.bareProbe(vf, r), req.Recent, lemmaUse, sentences); st <= 1 {
					bestBare, bestBareScore = i, st*2+1
					break
				}
			}
		}
		for pass := 0; pass < 2 && best < 0; pass++ {
			for i, r := range refs {
				if pass == 0 && r.c.feats[target.Key] != target.Value {
					continue // the second pass ignores the weak-feature preference
				}
				probe := g.personProbe(vf, r.w, r.c)
				st := stageOf(probe, req.Recent, lemmaUse, sentences)
				age := 0
				if st == 3 {
					age, _ = req.Recent.keyRank(probe)
				}
				if st < bestStage || (st == bestStage && st == 3 && age > bestAge) {
					best, bestStage, bestAge = i, st, age
				}
				if st == 0 {
					break
				}
			}
		}
		if bestBare >= 0 && (best < 0 || bestBareScore < bestStage*2 || (forceBare && bestBareScore <= 3)) {
			r := brefs[bestBare]
			brefs = append(brefs[:bestBare], brefs[bestBare+1:]...)
			if !g.bareValid(req.Topic, vf, r) {
				continue
			}
			it := g.bareItemFrom(req, vf, r)
			it.Origin = req.Origin
			items = append(items, it)
			lemmaUse[it.LemmaID]++
			bareTaken++
			continue
		}
		if best < 0 {
			if bestBare < 0 && len(brefs) > 0 && len(refs) == 0 {
				// Only bare refs left but none carries the weak target: any.
				r := brefs[0]
				brefs = brefs[1:]
				if g.bareValid(req.Topic, vf, r) {
					it := g.bareItemFrom(req, vf, r)
					it.Origin = req.Origin
					items = append(items, it)
					lemmaUse[it.LemmaID]++
				}
				continue
			}
			if len(refs) == 0 {
				break
			}
			// Nothing matched the weak target in this pass: take any.
			best = 0
		}
		r := refs[best]
		refs = append(refs[:best], refs[best+1:]...)
		it := g.personItemFrom(req, vf, r.w, r.c)
		it.Origin = req.Origin
		items = append(items, it)
		lemmaUse[it.LemmaID]++
		sentences[it.SentenceID] = true
	}
	// Sentence and bare items were taken in two streams; serve them mixed.
	req.Rand.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	return items
}

// bareProbe is the lightweight Item stageOf needs to judge a bare candidate.
func (g *Generator) bareProbe(vf []Feature, r bareRef) Item {
	return Item{Kind: KindPerson, Bare: true, LemmaID: r.w.LemmaID, cell: r.cell}
}

// personProbe is the lightweight Item stageOf needs to judge a candidate.
func (g *Generator) personProbe(vf []Feature, w Word, c personCand) Item {
	carried := personCarried(vf, c.feats)
	return Item{Kind: KindPerson, LemmaID: w.LemmaID, Headline: carried[0], SentenceID: c.sentID}
}

// classifyPerson relabels a wrong typed answer as a person error when it is an
// attested form of the same verb in a different person. The second result is
// true when, besides, the typed form carries every feature the item required: a
// learner who wrote the right tense for the wrong person has shown the tense,
// and charging the tense for it would put noise in the heatmap.
func classifyPerson(d morph.Diff, item Item, typed string, a *morph.Analyzer) (morph.Diff, bool) {
	if d.Match || a == nil || item.Kind != KindPerson || item.answerPerson == "" {
		return d, false
	}
	all, err := a.Lookup(strings.ToLower(strings.TrimSpace(typed)))
	if err != nil {
		return d, false
	}
	// The pack tags many dictionary forms with no features at all (`kerül` is
	// listed bare), yet the citation form of a Hungarian verb IS the third
	// person singular present indefinite. When the typed string is the lemma
	// and no reading of it says otherwise, read it as that.
	tagged := false
	for _, an := range all {
		if an.LemmaID == item.LemmaID && an.Source != "generated" && an.Feats["Person"] != "" {
			tagged = true
		}
	}
	if !tagged && item.Lemma != "" && strings.EqualFold(strings.TrimSpace(typed), item.Lemma) {
		all = append(all, morph.Analysis{LemmaID: item.LemmaID, Source: "citation", Feats: map[string]string{
			"Person": "3", "Number": "Sing", "Tense": "Pres", "Mood": "Ind", "Definite": "Ind"}})
	}
	// Order-independent: every reading of the typed form in another person
	// decides together. It is a person slip when there is one, and the features
	// are left alone only if EVERY such reading carries all the item's features.
	slip, keeps := false, true
	for _, an := range all {
		if an.LemmaID != item.LemmaID || an.Source == "generated" {
			continue
		}
		if an.Feats["Person"] == "" || personCell(an.Feats) == item.answerPerson {
			continue
		}
		slip = true
		keeps = keeps && carriesAll(an.Feats, item.FeatureIDs)
	}
	if slip {
		d.Class = ErrPerson
		d.NearMatch = false
		d.WrongFeature = ""
		d.Hint = "diff.person"
		d.HintArgs = map[string]string{"expected": d.Expected}
		return d, keeps
	}
	return d, false
}

// carriesAll reports whether a reading has every feature id ("Key=Value").
func carriesAll(feats map[string]string, ids []string) bool {
	for _, id := range ids {
		k, v, ok := strings.Cut(id, "=")
		if !ok || feats[k] != v {
			return false
		}
	}
	return len(ids) > 0
}

// ── bare mode ────────────────────────────────────────────────────────────

// bareThreshold: a person topic with fewer distinct sentence items than this
// also serves sentence-less items, so a thin topic (the conditional) is not a
// handful of repeating sentences.
const bareThreshold = 30

// bareRef is one (verb, paradigm cell) with its single attested form.
type bareRef struct {
	w     Word
	form  string
	feats map[string]string
	cell  string
}

// personSentenceTotal counts the distinct sentence items of a topic, cached.
func (g *Generator) personSentenceTotal(t curriculum.Topic, vf []Feature, allowDef bool) int {
	g.probeMu.Lock()
	n, ok := g.personTotal[t.Code]
	g.probeMu.Unlock()
	if ok {
		return n
	}
	ceiling := bandFor(t.Level)
	n = 0
	for _, w := range g.verbs {
		if w.Rank > ceiling {
			break
		}
		n += len(g.personCandidates(t, vf, allowDef, w.LemmaID))
		if n >= bareThreshold {
			break // enough; the exact count is not needed
		}
	}
	g.probeMu.Lock()
	if g.personTotal == nil {
		g.personTotal = map[string]int{}
	}
	g.personTotal[t.Code] = n
	g.probeMu.Unlock()
	return n
}

// bareRefs enumerates the (verb, cell) pairs of a topic from the lemma's
// attested forms. A cell with more than one attested string (menj / menjél) is
// skipped: the cue would not determine the answer. Definite forms are left out
// unless the topic itself teaches Definite=Def, because a bare cue cannot say
// what the object is. Unambiguity of the reading is checked when an item is
// picked (bareValid), not here, to keep the enumeration to one query per verb.
func (g *Generator) bareRefs(t curriculum.Topic, vf []Feature) []bareRef {
	g.probeMu.Lock()
	cached, ok := g.bareCache[t.Code]
	g.probeMu.Unlock()
	if ok {
		return cached
	}
	allowed := map[string]map[string]bool{}
	for _, f := range vf {
		if allowed[f.Key] == nil {
			allowed[f.Key] = map[string]bool{}
		}
		allowed[f.Key][f.Value] = true
	}
	teachesDef := allowed["Definite"]["Def"]

	var out []bareRef
	ceiling := bandFor(t.Level)
	for _, w := range g.verbs {
		if w.Rank > ceiling {
			break
		}
		forms, err := g.analyzer.FormsOf(w.LemmaID)
		if err != nil {
			continue
		}
		type group struct {
			strs  map[string]bool
			feats map[string]string
		}
		cells := map[string]*group{}
		for _, f := range forms {
			if !f.Attested() || personPronouns[personCell(f.Feats)] == "" || !verbalOK(f.Feats, allowed, true) {
				continue
			}
			if f.Feats["Definite"] == "Def" && !teachesDef {
				continue
			}
			key := f.Feats["Definite"] + "|" + f.Feats["Mood"] + "|" + f.Feats["Tense"] + "|" + personCell(f.Feats)
			if cells[key] == nil {
				cells[key] = &group{strs: map[string]bool{}, feats: f.Feats}
			}
			cells[key].strs[strings.ToLower(f.Form)] = true
		}
		keys := make([]string, 0, len(cells))
		for k := range cells {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			c := cells[k]
			if len(c.strs) != 1 {
				continue
			}
			for form := range c.strs {
				out = append(out, bareRef{w: w, form: form, feats: c.feats, cell: k})
			}
		}
	}
	g.probeMu.Lock()
	if g.bareCache == nil {
		g.bareCache = map[string][]bareRef{}
	}
	g.bareCache[t.Code] = out
	g.probeMu.Unlock()
	return out
}

// bareValid applies the same guards as a sentence token: attested, not
// generated, unambiguous, the topic's features all carried.
func (g *Generator) bareValid(t curriculum.Topic, vf []Feature, r bareRef) bool {
	feats, ok := g.personReading(r.form, r.w.LemmaID, vf, true)
	return ok && personCell(feats) == personCell(r.feats)
}

// bareItemFrom builds a sentence-less person item.
func (g *Generator) bareItemFrom(req Request, vf []Feature, r bareRef) Item {
	carried := personCarried(vf, r.feats)
	args := map[string]string{"lemma": r.w.Lemma, "pronoun": personPronouns[personCell(r.feats)]}
	for _, f := range vf {
		switch {
		case f.Key == "Mood" && f.Value != "Ind" && r.feats["Mood"] == f.Value:
			args["mood"] = f.ID
		case f.Key == "Tense" && f.Value == "Past" && r.feats["Tense"] == "Past":
			args["tense"] = f.ID
		}
	}
	ru, en := g.gloss(r.w.LemmaID)
	return Item{
		TopicCode: req.Topic.Code, Kind: KindPerson, Phase: req.Phase, Bare: true,
		PromptKey: "drill.prompt.person", PromptArgs: args, Frame: BlankMark,
		Lemma: r.w.Lemma, LemmaID: r.w.LemmaID, GlossRU: ru, GlossEN: en,
		FeatureIDs: carried, Headline: carried[0], Attested: true,
		expected: r.form, answerPerson: personCell(r.feats), cell: r.cell,
	}
}

// randomBare draws one valid bare item, or false when the topic has none.
func (g *Generator) randomBare(req Request, vf []Feature) (Item, bool) {
	refs := g.bareRefs(req.Topic, vf)
	if len(refs) == 0 {
		return Item{}, false
	}
	for range maxCandidateTries {
		r := refs[req.Rand.Intn(len(refs))]
		if g.bareValid(req.Topic, vf, r) {
			return g.bareItemFrom(req, vf, r), true
		}
	}
	return Item{}, false
}
