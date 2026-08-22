package drill

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"tanulom/internal/morph"
)

// The definite/indefinite conjugation drill.
//
// `Látok egy házat` versus `Látom a házat`. There is no analogue in Russian or
// English, and [R15] identifies it as the single hardest thing in the language
// for our learner — plus the topic the source brief wanted interleaved from day
// one and the evidence says must not be. See schedule.go.
//
// What makes this file more than "generate some verbs" is the WEIGHTING. The
// HunLearner error-detection study gives a documented distribution:
//
//	definite object taking the INDEFINITE conjugation   55%
//	demonstrative-pronoun object                        25%
//
// So the items are drawn in those proportions rather than uniformly. Practising
// what learners actually get wrong is worth more than practising the paradigm
// evenly, and the numbers are configurable (`drill.definiteness` in config.yaml)
// because they come from one corpus and should be treated as indicative.

// Weights is the item distribution, as percentages out of 100.
type Weights struct {
	// DefiniteObjectPct: a plain definite object — `a`/`az` + noun.
	DefiniteObjectPct int
	// DemonstrativePct: a demonstrative-pronoun object — `azt a filmet`.
	DemonstrativePct int
}

// Remainder is the share left for indefinite-object items, which exist so the
// drill is a genuine choice rather than a pattern to memorise: a learner who
// only ever sees definite objects learns "always use the definite form".
func (w Weights) Remainder() int {
	rest := 100 - w.DefiniteObjectPct - w.DemonstrativePct
	if rest < 0 {
		return 0
	}
	return rest
}

// mode is which shape of object an item uses.
type mode int

const (
	modeDefinite mode = iota
	modeDemonstrative
	modeIndefinite
)

func (m mode) String() string {
	switch m {
	case modeDefinite:
		return "definite"
	case modeDemonstrative:
		return "demonstrative"
	}
	return "indefinite"
}

// pickMode draws a mode from the configured distribution.
func pickMode(w Weights, rnd *rand.Rand) mode {
	def, dem := w.DefiniteObjectPct, w.DemonstrativePct
	if def <= 0 && dem <= 0 {
		def, dem = 55, 25
	}
	roll := rnd.Intn(100)
	switch {
	case roll < def:
		return modeDefinite
	case roll < def+dem:
		return modeDemonstrative
	}
	return modeIndefinite
}

// anchors are the lemmas whose presence marks a sentence as carrying an object
// of a given definiteness. Resolved once against the pack, because the ids are
// pack-specific and hardcoding them would break on the next build.
type anchors struct {
	once sync.Once
	err  error
	// definite: the article `a` / `az` in its determiner reading.
	definite []int64
	// demonstrative: `az` / `ez` in their pronoun reading, which is what
	// produces `azt a filmet`.
	demonstrative []int64
	// indefinite: `egy`.
	indefinite []int64
	minSentID  int64
	maxSentID  int64
}

func (g *Generator) loadAnchors() (*anchors, error) {
	g.anchors.once.Do(func() {
		rows, err := g.pack.DB().Query(
			`SELECT id, lemma, upos FROM lemma WHERE lemma IN ('a','az','ez','egy')`)
		if err != nil {
			g.anchors.err = fmt.Errorf("load definiteness anchors: %w", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var lemma, upos string
			if err := rows.Scan(&id, &lemma, &upos); err != nil {
				g.anchors.err = err
				return
			}
			switch {
			case lemma == "egy":
				g.anchors.indefinite = append(g.anchors.indefinite, id)
			case upos == "DET":
				g.anchors.definite = append(g.anchors.definite, id)
			default:
				g.anchors.demonstrative = append(g.anchors.demonstrative, id)
			}
		}
		if err := rows.Err(); err != nil {
			g.anchors.err = err
			return
		}
		if err := g.pack.DB().QueryRow(
			`SELECT MIN(id), MAX(id) FROM sentence`).Scan(&g.anchors.minSentID, &g.anchors.maxSentID); err != nil {
			g.anchors.err = fmt.Errorf("sentence id range: %w", err)
		}
	})
	return &g.anchors, g.anchors.err
}

func (a *anchors) forMode(m mode) []int64 {
	switch m {
	case modeDefinite:
		return a.definite
	case modeDemonstrative:
		return a.demonstrative
	}
	return a.indefinite
}

// wantDefinite reports which conjugation the mode's object requires. This is the
// rule the drill is teaching, stated once: a definite object — including a
// demonstrative one — takes the definite conjugation.
func (m mode) wantDefinite() string {
	if m == modeIndefinite {
		return "Ind"
	}
	return "Def"
}

// conjugationItem builds one definite/indefinite choice from an attested pack
// sentence.
//
// The distractor is the SAME verb in the other conjugation, at the same person,
// number, tense and mood. That is the minimal contrast R5 asks for, and it is
// also the only honest distractor: anything else would let the learner answer by
// eliminating obviously-wrong forms instead of by making the choice.
func (g *Generator) conjugationItem(req Request) (Item, bool) {
	an, err := g.loadAnchors()
	if err != nil {
		return Item{}, false
	}
	m := pickMode(req.Definiteness, req.Rand)
	if len(an.forMode(m)) == 0 {
		return Item{}, false
	}
	want := m.wantDefinite()

	for range maxCandidateTries {
		sentID, hu, ok := g.sampleSentenceContaining(an.forMode(m), req.Rand)
		if !ok {
			continue
		}
		tokens := morph.Tokenize(hu)
		if n := wordCount(tokens); n < 3 || n > 14 {
			continue
		}
		// The demonstrative case is specifically the accusative pronoun — `azt`,
		// `ezt`, `azokat`. A sentence merely containing `az` as a subject is a
		// different (and much easier) item.
		if m == modeDemonstrative && !hasDemonstrativeObject(tokens) {
			continue
		}

		for _, t := range tokens {
			if !t.IsWord {
				continue
			}
			verb, ok := g.verbReading(t.Text, want)
			if !ok {
				continue
			}
			contrast, ok := g.flippedConjugation(verb, want)
			if !ok || strings.EqualFold(contrast.Form, t.Text) {
				// 1sg past is `láttam` in BOTH conjugations. An item whose two
				// options are the same string is not a choice; skip the token.
				continue
			}

			frame := hu[:t.Start] + BlankMark + hu[t.End:]
			opts := []Option{
				{Text: t.Text, Attested: true},
				{Text: contrast.Form, Attested: contrast.Attested()},
			}
			req.Rand.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })

			ru, en := g.gloss(verb.LemmaID)
			return Item{
				TopicCode: req.Topic.Code, Kind: KindConjugation, Phase: req.Phase,
				PromptKey: "drill.prompt.conjugation",
				PromptArgs: map[string]string{
					"object": m.String(), "lemma": verb.Lemma,
				},
				Frame: frame, Lemma: verb.Lemma, LemmaID: verb.LemmaID,
				GlossRU: ru, GlossEN: en, Options: opts,
				// Only definiteness is at stake: the person, number and tense are
				// identical across the two options, so the learner was never asked
				// about them.
				FeatureIDs: []string{"Definite=" + want},
				Headline:   "Definite=" + want,
				Attested:   true, SentenceID: sentID, expected: t.Text,
			}, true
		}
	}
	return Item{}, false
}

// verbReading returns the verb analysis of a form carrying the wanted
// definiteness, ignoring any noun reading of the same string. `Lookup` orders by
// trust, not by part of speech, so `látok` comes back as a possessed noun first.
func (g *Generator) verbReading(form, wantDefinite string) (morph.Analysis, bool) {
	all, err := g.analyzer.Lookup(form)
	if err != nil {
		return morph.Analysis{}, false
	}
	for _, a := range all {
		if a.UPOS != "VERB" || a.Source == "generated" {
			continue
		}
		if a.Feats["Definite"] != wantDefinite {
			continue
		}
		if a.Feats["Person"] == "" || a.Feats["Number"] == "" {
			continue
		}
		return a, true
	}
	return morph.Analysis{}, false
}

// flippedConjugation finds the same verb, same person/number/tense/mood, in the
// other conjugation.
func (g *Generator) flippedConjugation(a morph.Analysis, wantDefinite string) (morph.Form, bool) {
	other := "Ind"
	if wantDefinite == "Ind" {
		other = "Def"
	}
	want := map[string]string{"Definite": other}
	for _, k := range []string{"Person", "Number", "Tense", "Mood"} {
		if v, ok := a.Feats[k]; ok && v != "" {
			want[k] = v
		}
	}
	f, ok := g.analyzer.BestForm(a.LemmaID, morph.FormSpec{Want: want})
	if !ok || !f.Attested() {
		return morph.Form{}, false
	}
	return f, true
}

// hasDemonstrativeObject reports whether the sentence contains an accusative
// demonstrative — the 25% error case.
func hasDemonstrativeObject(tokens []morph.Token) bool {
	for _, t := range tokens {
		switch strings.ToLower(t.Text) {
		case "azt", "ezt", "azokat", "ezeket":
			return true
		}
	}
	return false
}

func wordCount(tokens []morph.Token) int {
	n := 0
	for _, t := range tokens {
		if t.IsWord {
			n++
		}
	}
	return n
}

// sampleSentenceContaining picks a random pack sentence containing any of the
// given lemmas.
//
// The random floor plus `sentence_id >=` walks the lemma's index from an
// arbitrary point rather than scanning it, so this stays an indexed lookup even
// though `az` occurs in eleven thousand sentences. ORDER BY RANDOM() would read
// all of them on every call.
func (g *Generator) sampleSentenceContaining(lemmaIDs []int64, rnd *rand.Rand) (int64, string, bool) {
	an, err := g.loadAnchors()
	if err != nil || len(lemmaIDs) == 0 || an.maxSentID <= an.minSentID {
		return 0, "", false
	}
	lemmaID := lemmaIDs[rnd.Intn(len(lemmaIDs))]
	span := an.maxSentID - an.minSentID
	floor := an.minSentID + rnd.Int63n(span)

	var sentID int64
	err = g.pack.DB().QueryRow(`
		SELECT sentence_id FROM sentence_token
		 WHERE lemma_id = ? AND sentence_id >= ? LIMIT 1`, lemmaID, floor).Scan(&sentID)
	if err != nil {
		// Past the end of this lemma's occurrences: wrap to the beginning.
		if err2 := g.pack.DB().QueryRow(`
			SELECT sentence_id FROM sentence_token
			 WHERE lemma_id = ? LIMIT 1`, lemmaID).Scan(&sentID); err2 != nil {
			return 0, "", false
		}
	}
	var hu string
	if err := g.pack.DB().QueryRow(
		`SELECT hu FROM sentence WHERE id = ?`, sentID).Scan(&hu); err != nil {
		return 0, "", false
	}
	return sentID, hu, true
}
