package drill

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"tanulom/internal/curriculum"
	"tanulom/internal/lang"
	"tanulom/internal/morph"
)

// Item generation.
//
// Drills are GENERATED, not drawn from a fixed bank, and that is a pedagogical
// requirement rather than an engineering preference: [R15] argues vowel harmony
// is automated by volume, and a finite exercise bank cannot supply volume — the
// learner memorises the bank instead of the rule.
//
// Two rules constrain everything here:
//
//   - The CORRECT answer must be attested in the pack. Distractors may be
//     constructed (a wrong allomorph is not a word, which is the point), but the
//     form we mark right has to be one the language actually has. Items we cannot
//     build from attested material are not built. See internal/morph/forms.go.
//   - An item declares exactly the features it REQUIRES, and only those are
//     graded. An allomorph choice requires the harmony skill and the case; a case
//     choice between forms of the same noun does not require harmony at all,
//     because every option shares the stem's harmony class. Getting this wrong is
//     what would turn the mastery heatmap into noise.

// Kind is the exercise type. Each has its own renderer in web-ui.
type Kind string

const (
	// KindAllomorph: -ban or -ben? The vowel-harmony motor drill. [R15]
	KindAllomorph Kind = "allomorph"
	// KindCase: which cell of the locative grid expresses this meaning?
	KindCase Kind = "case"
	// KindConjugation: definite or indefinite? Weighted to the documented error
	// distribution — see definiteness.go.
	KindConjugation Kind = "conjugation"
	// KindCloze: type the blanked form of an attested sentence.
	KindCloze Kind = "cloze"
	// KindTransform: produce a different cell of the paradigm, typed.
	KindTransform Kind = "transform"
	// KindMCQ: choose the right cell among minimally different distractors.
	KindMCQ Kind = "mcq"
	// KindPerson: type the verb of a real sentence in the person a pronoun asks
	// for. Typed; see person.go.
	KindPerson Kind = "person"
	// KindQword: which question word does the highlighted noun answer (hol,
	// hova, honnan...)? Choice; see qword.go.
	KindQword Kind = "qword"
	// KindSort: tap tiles into suffix columns (-ban / -ben). A multi-part
	// choice item graded once per feature; see sort.go.
	KindSort Kind = "sort"
	// KindMatch: pair 3-5 cues (pronouns, question words) with the forms of ONE
	// lemma. A multi-part recognition item graded once per feature; see match.go.
	KindMatch Kind = "match"
	// KindScene: pick the locative form of a PLACE noun from a picture of an
	// object at it (static, into, out). Choice; see scene.go.
	KindScene Kind = "scene"
)

// MultiPart reports whether the item is answered as a set of tile -> column
// placements (`parts`) rather than one string.
func (k Kind) MultiPart() bool { return k == KindSort || k == KindMatch }

// AllKinds in a stable order.
var AllKinds = []Kind{KindAllomorph, KindCase, KindConjugation, KindCloze, KindTransform, KindMCQ, KindPerson, KindQword, KindSort, KindMatch, KindScene}

// Valid reports whether k is a known kind.
func (k Kind) Valid() bool {
	for _, x := range AllKinds {
		if x == k {
			return true
		}
	}
	return false
}

// Typed reports whether the learner types the answer rather than choosing it.
// Typed items are strictly harder and are what transfers to production. [R12]
func (k Kind) Typed() bool { return k == KindCloze || k == KindTransform || k == KindPerson }

// Feature is one teachable morphological atom, as the generator needs it.
type Feature struct {
	ID         string
	Key        string
	Value      string
	Allomorphs map[string]string
	TeachOrder int
	// Grid is "interior/where" and friends — the 3×3 locative table. Present
	// only on the nine locative cases. [R15]
	Grid string
}

// HasExponent reports whether the feature is marked by an actual suffix.
//
// Unmarked features — the indicative, the present, the singular, the nominative,
// the indefinite conjugation, every `Skill=*` — are the DEFAULTS. They are real
// grammatical facts, but they have nothing a learner can be asked to produce or
// locate, so no generator may target one.
func (f Feature) HasExponent() bool { return len(Distinct(f.Allomorphs)) > 0 }

// Row and Column split a grid cell: "interior/where" -> "interior", "where".
func (f Feature) Row() string    { part, _, _ := strings.Cut(f.Grid, "/"); return part }
func (f Feature) Column() string { _, part, _ := strings.Cut(f.Grid, "/"); return part }

// nominalSpec names ONE cell of the noun paradigm, with every dimension pinned.
//
// Leaving a dimension unspecified means "any value", and BestForm then returns
// whichever matching row sorts first. That is how a plural item came to expect
// an accusative: the spec said Number=Plur and nothing about Case, so the
// shortest plural cell of any case won. Every generator that asks for a nominal
// cell goes through here.
func nominalSpec(f Feature) morph.FormSpec {
	spec := morph.FormSpec{Want: map[string]string{}}
	switch f.Key {
	case "Case":
		spec.Want["Case"] = f.Value
		spec.Want["Number"] = "Sing"
		spec.Without = []string{"Person[psor]", "Number[psor]", "Number[psed]"}
	case "Number":
		spec.Want["Number"] = f.Value
		spec.Want["Case"] = "Nom"
		spec.Without = []string{"Person[psor]", "Number[psor]", "Number[psed]"}
	case "Person[psor]":
		// The possessed thing is singular and the possessor is singular unless
		// the feature says otherwise: `házam`, not `házaim` and not `házunk`.
		//
		// Case is EXCLUDED rather than pinned to Nom. Wiktionary's attested
		// possessive rows carry no Case at all (`ára` is tagged only
		// Number[psor]=Sing|Person[psor]=3); only the generated rows add
		// Case=Nom. Pinning it would throw away every attested form and leave us
		// with our own guesses, which is the one thing a drill answer may not be.
		spec.Want["Person[psor]"] = f.Value
		spec.Want["Number[psor]"] = "Sing"
		spec.Without = []string{"Case", "Number[psed]"}
	case "Number[psed]":
		// Several things possessed by one third person: `házai`.
		spec.Want["Number[psed]"] = f.Value
		spec.Want["Person[psor]"] = "3"
		spec.Want["Number[psor]"] = "Sing"
		spec.Without = []string{"Case"}
	case "Degree":
		spec.Want["Degree"] = f.Value
	default:
		spec.Want[f.Key] = f.Value
	}
	return spec
}

// verbalSpec names one cell of the verb paradigm for the features that have a
// suffix of their own (the potential, the conditional, the past).
func verbalSpec(f Feature) morph.FormSpec {
	return morph.FormSpec{Want: map[string]string{
		f.Key: f.Value, "Person": "3", "Number": "Sing", "Definite": "Ind",
	}}
}

// Word is a vocabulary entry the generator may build an item around.
type Word struct {
	LemmaID int64
	Lemma   string
	UPOS    string
	Rank    int
}

// Generator builds items from the language pack.
//
// Safe for concurrent use: the catalogue and vocabulary are loaded once and then
// only read.
type Generator struct {
	pack     *lang.Pack
	analyzer *morph.Analyzer
	dag      *curriculum.DAG

	once     sync.Once
	loadErr  error
	features map[string]Feature

	probeMu sync.Mutex
	probed  map[string]Availability
	// kindProbed caches probeKind verdicts, "topic|kind" -> an item can be built.
	kindProbed map[string]bool
	scene      sceneState
	// personCache holds the usable (sentence, token) pairs per topic and verb.
	personCache map[string][]personCand
	personTotal map[string]int
	bareCache   map[string][]bareRef
	anchors     anchors
	nouns       []Word
	verbs       []Word
	adjs        []Word
}

// NewGenerator wires a generator. All three dependencies are required; without a
// pack there is no attested material and therefore no items.
func NewGenerator(p *lang.Pack, a *morph.Analyzer, d *curriculum.DAG) *Generator {
	return &Generator{pack: p, analyzer: a, dag: d, probed: map[string]Availability{}}
}

// vocabularyCeiling caps how much of the frequency list is loaded. Drills are
// grammar practice, not vocabulary acquisition: building an inessive item around
// the 12 000th commonest noun teaches a rare word badly instead of teaching the
// inessive well.
const (
	vocabularyCeiling = 6000
	maxCandidateTries = 24
)

func (g *Generator) load() error {
	g.once.Do(func() {
		if g.pack == nil || g.analyzer == nil || g.dag == nil {
			g.loadErr = fmt.Errorf("generator needs a language pack")
			return
		}
		g.features = map[string]Feature{}
		rows, err := g.pack.DB().Query(`
			SELECT id, key, value, allomorphs, teach_order, COALESCE(grid,'')
			  FROM morph_feature`)
		if err != nil {
			g.loadErr = fmt.Errorf("load feature catalogue: %w", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var f Feature
			var raw string
			if err := rows.Scan(&f.ID, &f.Key, &f.Value, &raw, &f.TeachOrder, &f.Grid); err != nil {
				g.loadErr = err
				return
			}
			f.Allomorphs = map[string]string{}
			_ = json.Unmarshal([]byte(raw), &f.Allomorphs)
			g.features[f.ID] = f
		}
		if err := rows.Err(); err != nil {
			g.loadErr = err
			return
		}

		for _, spec := range []struct {
			upos string
			dst  *[]Word
		}{{"NOUN", &g.nouns}, {"VERB", &g.verbs}, {"ADJ", &g.adjs}} {
			words, err := g.loadVocabulary(spec.upos)
			if err != nil {
				g.loadErr = err
				return
			}
			*spec.dst = words
		}
	})
	return g.loadErr
}

// excludedLemmas are words we will not build a drill around.
//
// The frequency list is derived from film subtitles, so a handful of obscenities
// sit inside the first thousand ranks — `fasz` outranks `kenyér`. They are real
// Hungarian and the reader will still gloss them; what the app must not do is
// spontaneously choose one as the headword of a grammar exercise. Cloze frames
// come from Tatoeba sentences and are not filtered, so this is a floor rather
// than a guarantee.
var excludedLemmas = map[string]bool{
	"fasz": true, "szar": true, "kurva": true, "picsa": true, "geci": true,
	"baszik": true, "szarik": true, "kefél": true, "segg": true, "fing": true,
}

// vulgarMarkers are the substrings (lower case) by which the pack's glosses
// flag a word as vulgar or offensive. The pack is inconsistent — `fasz` is
// «вульг.» in Russian, `seggfej` has only "arsehole" in English and no marker at
// all — so both the dictionary labels and the plain words are matched.
var vulgarMarkers = []string{
	"vulg", "вульг", "offens", "derog", "obscen", "pej.", "груб", "whore",
	"prostitut", "arsehole", "asshole", "fuck", "dick", "cunt", "penis", "bitch",
	"shit", "piss", "slut",
}

// harmlessDespiteMarkers are lemmas the substring markers catch by accident:
// `kerítés` (fence; pimping is its second sense), `farok` (tail), `makk`
// (acorn), `támadó` (attacking), `közönséges` (common), `bántó` (hurtful).
var harmlessDespiteMarkers = map[string]bool{
	"kerítés": true, "farok": true, "makk": true, "támadó": true,
	"közönséges": true, "bántó": true,
}

// vulgarLemmas returns the lemma ids with a vulgar or offensive gloss. Such a
// word is real Hungarian and the reader will still gloss it, but a grammar drill
// must not spontaneously build an exercise around it.
func (g *Generator) vulgarLemmas() map[int64]bool {
	out := map[int64]bool{}
	cond := make([]string, len(vulgarMarkers))
	args := make([]any, len(vulgarMarkers))
	for i, m := range vulgarMarkers {
		cond[i] = "g.gloss LIKE ?"
		args[i] = "%" + m + "%"
	}
	rows, err := g.pack.DB().Query(
		`SELECT DISTINCT g.lemma_id, l.lemma FROM lemma_gloss g JOIN lemma l ON l.id = g.lemma_id WHERE `+
			strings.Join(cond, " OR "), args...)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var lemma string
		if rows.Scan(&id, &lemma) == nil && !harmlessDespiteMarkers[lemma] {
			out[id] = true
		}
	}
	return out
}

// evidenceFeature is the inflected form a part of speech must actually have
// attested before we will build drills around it. See loadVocabulary.
var evidenceFeature = map[string]string{
	"NOUN": "Case=Ine",
	"VERB": "Definite=Def",
	"ADJ":  "Degree=Cmp",
}

// loadVocabulary reads the drillable words of one part of speech.
//
// Three filters, and each of them removes a specific kind of nonsense the pack
// would otherwise put in front of a learner:
//
//  1. **Unambiguous part of speech.** The pack assigns frequency ranks per lemma
//     STRING, so the NOUN row of `hisz` inherits the rank of the verb `hisz` (to
//     believe) and lands at rank 50 — as does `nem`, `mi`, `fel`, `volt`. Drilling
//     the inessive of "no" is not a lesson. Requiring the string to have exactly
//     one part of speech drops all of them.
//  2. **Lower case only**, applied Unicode-aware in Go because SQLite's lower()
//     is ASCII-only. Removes acronyms (`MI`) and proper nouns (`Nap`), which are
//     bad drill vocabulary and often decline irregularly.
//  3. **An attested inflected form.** A word the dictionary never shows in the
//     inessive is not a noun we should be inflecting; this removes verb forms
//     that Wiktionary happens to also list as nouns.
//
// Run once per process, so the cost is paid at first use rather than per request.
func (g *Generator) loadVocabulary(upos string) ([]Word, error) {
	evidence := evidenceFeature[upos]
	vulgar := g.vulgarLemmas()
	rows, err := g.pack.DB().Query(`
		SELECT l.id, l.lemma, l.upos, l.freq_rank FROM lemma l
		 WHERE l.upos = ? AND l.freq_rank IS NOT NULL AND l.freq_rank <= ?
		   AND NOT EXISTS (
		       SELECT 1 FROM lemma o WHERE o.lemma = l.lemma AND o.upos <> l.upos)
		   AND EXISTS (
		       SELECT 1 FROM wordform w
		        WHERE w.lemma_id = l.id AND w.source != 'generated'
		          AND w.feats LIKE ?)
		 ORDER BY l.freq_rank`, upos, vocabularyCeiling, "%"+evidence+"%")
	if err != nil {
		return nil, fmt.Errorf("load %s vocabulary: %w", upos, err)
	}
	defer rows.Close()
	var out []Word
	for rows.Next() {
		var w Word
		if err := rows.Scan(&w.LemmaID, &w.Lemma, &w.UPOS, &w.Rank); err != nil {
			return nil, err
		}
		if w.Lemma != strings.ToLower(w.Lemma) || excludedLemmas[w.Lemma] || vulgar[w.LemmaID] {
			continue
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// Feature returns one catalogue entry.
func (g *Generator) Feature(id string) (Feature, bool) {
	if g.load() != nil {
		return Feature{}, false
	}
	f, ok := g.features[id]
	return f, ok
}

// Option is one choice in a multiple-choice item.
type Option struct {
	Text string `json:"text"`
	// Attested reports whether this exact string exists in the pack. Shown in
	// the post-answer explanation: "-házben is not a Hungarian word" is a more
	// useful correction than a red cross.
	Attested bool `json:"attested"`
}

// FramePart is one run of a sentence. Exactly one part of a qword frame has
// Focus set: the word the question is about.
type FramePart struct {
	Text  string `json:"text"`
	Focus bool   `json:"focus,omitempty"`
}

// Item is one exercise, ready to render.
//
// Prose is absent by design. PromptKey is an i18n key and PromptArgs are its
// substitutions, so the same item renders in both locales from one server
// response. See ADR-004.
type Item struct {
	ID         string            `json:"id"`
	TopicCode  string            `json:"topicCode"`
	Kind       Kind              `json:"kind"`
	Phase      Phase             `json:"phase"`
	PromptKey  string            `json:"promptKey"`
	PromptArgs map[string]string `json:"promptArgs,omitempty"`
	// Frame is the Hungarian shown with the answer replaced by BlankMark.
	Frame   string `json:"frame,omitempty"`
	Lemma   string `json:"lemma,omitempty"`
	LemmaID int64  `json:"lemmaId,omitempty"`
	GlossRU string `json:"glossRu,omitempty"`
	GlossEN string `json:"glossEn,omitempty"`
	// Parts is a frame split server-side into runs of text, one of them focused
	// (qword). The client never slices a sentence with offsets; see ADR on UTF-8
	// versus UTF-16 offsets.
	Parts []FramePart `json:"parts,omitempty"`
	// Columns and Tiles make a sort item: 2-3 allomorph labels of ONE feature
	// and six lemmas to put under them. The right column of each tile stays in
	// `expected` on the server.
	// A match item reuses them: Columns are the CUES (pronouns, question words),
	// Tiles the FORMS, and exactly one tile belongs under each column.
	Columns []string `json:"columns,omitempty"`
	Tiles   []Tile   `json:"tiles,omitempty"`
	// Bare marks a person item with no sentence: the frame is just the gap and
	// the client shows the lemma, pronoun and mood as a cue chip.
	Bare bool `json:"bare,omitempty"`
	// Picture is what a `scene` item draws: catalogue ids of the place and the
	// object, the motion, and descriptions for accessibility.
	Picture *Picture `json:"picture,omitempty"`
	// Options empty means the learner types the answer.
	Options []Option `json:"options,omitempty"`
	// FeatureIDs are the features this item REQUIRES, and the only ones a
	// wrong answer may be charged against.
	FeatureIDs []string `json:"featureIds"`
	// Headline is the feature the item is really about, for the chip and for
	// error attribution.
	Headline string `json:"headline,omitempty"`
	// Attested reports whether the expected answer is attested. Always true for
	// items we serve; carried on the wire so a future relaxation stays visible.
	Attested bool `json:"attested"`
	// SentenceID records provenance when the frame came from the pack corpus.
	SentenceID int64 `json:"sentenceId,omitempty"`
	// Origin distinguishes ordinary practice from a placement probe. Set at
	// generation time and trusted at answer time, because the client never sees
	// it — the alternative would let a client tag ordinary practice as placement
	// and skip the curriculum.
	Origin string `json:"-"`

	// sceneObjectID, sceneMotion and sceneForms (lower-case form -> case) are a
	// scene item's server-side facts: the key and the error classification.
	sceneObjectID int64
	sceneMotion   string
	sceneForms    map[string]string
	// cell names a bare person item's paradigm cell, for its key.
	cell string
	// pairFeats maps a match tile id to the features that pair carries, so each
	// distinct feature is judged once. Server-side only.
	pairFeats map[string][]string

	// answerPerson is "Person|Number" of a person item's expected form, used to
	// tell a wrong person from any other slip. Server-side only.
	answerPerson string

	// expected never leaves the server. The client posts back an item id and its
	// answer; the server holds the truth. See pending.go.
	expected string
}

// Expected exposes the answer to the packages that must grade it.
func (i Item) Expected() string { return i.expected }

// WithExpected is used by the pending store when restoring a served item.
func (i Item) WithExpected(s string) Item { i.expected = s; return i }

// BlankMark is what replaces the answer in a frame. Three underscores, because
// a single one is invisible next to Hungarian's long words.
const BlankMark = "___"

// Request is one call for items.
type Request struct {
	Topic curriculum.Topic
	Phase Phase
	Count int
	// KnownLemmas biases vocabulary towards words the learner has met. It is a
	// preference, never a filter: a drill starved of vocabulary teaches nothing,
	// and the grammar point does not depend on knowing the noun.
	KnownLemmas map[int64]bool
	// WeakFeatures come from recent errors and are over-sampled. See from_error.go.
	WeakFeatures []string
	// Origin is stamped onto every item this request produces.
	Origin string
	// Definiteness carries the [R15] error-distribution weights.
	Definiteness Weights
	// Recent is what the learner met lately in this topic; nil disables
	// anti-repeat (placement, the availability probe).
	Recent *Recent
	Rand   *rand.Rand
}

// Availability reports whether a topic can produce items, and why not if it cannot.
//
// "We have not built drills for this topic yet" is a thing the app says out loud.
// The alternative — an empty screen, or worse, filler items — is the kind of
// quiet overclaiming this project exists to avoid.
type Availability struct {
	Available bool   `json:"available"`
	Kinds     []Kind `json:"kinds"`
	Reason    Reason `json:"reason,omitempty"`
}

const (
	// ReasonNoGenerator: the topic's rule card is real, its drills are not built.
	ReasonNoGenerator Reason = "drill.unavailable.noGenerator"
	// ReasonNoPack: no language pack installed.
	ReasonNoPack Reason = "drill.unavailable.noPack"
	// ReasonNoMaterial: a generator exists but the pack yielded nothing usable
	// for this learner right now.
	ReasonNoMaterial Reason = "drill.unavailable.noMaterial"
)

// Availability reports whether a topic can produce items, PROVING it by
// building one.
//
// KindsFor below is structural: it reads the topic's features and says which
// generators could apply. That is not the same as being able to produce
// anything, and the gap is not hypothetical — the pack's attested `olvashat`
// carries no features at all, so the potential mood passes every structural test
// and yields nothing. A topic that advertises drills and then serves an empty
// screen is exactly the kind of quiet overclaiming this project exists to avoid.
//
// The probe runs once per topic per process and is cached, because the answer
// depends on the pack, not on the learner.
func (g *Generator) Availability(t curriculum.Topic) Availability {
	structural := g.KindsFor(t)
	if !structural.Available {
		return structural
	}
	g.probeMu.Lock()
	cached, ok := g.probed[t.Code]
	g.probeMu.Unlock()
	if ok {
		return cached
	}

	// A fixed seed: the probe must give the same verdict on every start, or a
	// topic would flicker in and out of the topic map between restarts.
	items, avail, err := g.Next(Request{
		Topic: t, Phase: Blocked, Count: 1,
		Definiteness: Weights{DefiniteObjectPct: DefaultDefiniteObjectPct,
			DemonstrativePct: DefaultDemonstrativePct},
		Rand: rand.New(rand.NewSource(1)),
	})
	result := structural
	if err != nil || len(items) == 0 {
		result = Availability{Kinds: structural.Kinds, Reason: ReasonNoMaterial}
		if avail.Reason != "" {
			result.Reason = avail.Reason
		}
	}
	g.probeMu.Lock()
	g.probed[t.Code] = result
	g.probeMu.Unlock()
	return result
}

func newProbeRand() *rand.Rand { return rand.New(rand.NewSource(1)) }

func newProbeRandSeed(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

// Definiteness defaults used by the availability probe, so that a probe run
// cannot depend on a learner's settings. Mirrors config.Drill.Definiteness.
const (
	DefaultDefiniteObjectPct = 55
	DefaultDemonstrativePct  = 25
)

// KindsFor reports which generators STRUCTURALLY apply to a topic, derived from
// the features it teaches rather than from a hardcoded table — so adding a topic
// to the pack does not require a code change. Callers deciding what to show a
// learner want Availability, which also proves it.
func (g *Generator) KindsFor(t curriculum.Topic) Availability {
	if err := g.load(); err != nil {
		return Availability{Reason: ReasonNoPack}
	}
	var kinds []Kind

	// The harmony topic is special: it drills the CHOICE, using whatever suffixes
	// have distinct allomorphs. A learner practising harmony does not need to
	// know what the inessive means — that is precisely why it can come second in
	// the curriculum, before any case is taught.
	if t.MotorSkill() {
		kinds = append(kinds, KindAllomorph)
	}

	hasDef, hasInd := false, false
	grids := 0
	for _, id := range t.FeatureIDs {
		f, ok := g.features[id]
		if !ok {
			continue
		}
		if f.ID == "Definite=Def" {
			hasDef = true
		}
		if f.ID == "Definite=Ind" {
			hasInd = true
		}
		if f.Grid != "" {
			grids++
		}
		if !t.MotorSkill() && len(Distinct(f.Allomorphs)) >= 2 && !contains(kinds, KindAllomorph) {
			kinds = append(kinds, KindAllomorph)
		}
	}
	if grids >= 2 {
		kinds = append(kinds, KindCase)
	}
	// Only the topic that teaches BOTH sides may drill the contrast. The
	// present-indefinite topic mentions Definite=Ind but the choice does not
	// exist for the learner yet, and drilling a choice nobody has been taught is
	// how you manufacture failure.
	if hasDef && hasInd {
		kinds = append(kinds, KindConjugation)
	}
	if g.nominalSlots(t) {
		kinds = append(kinds, KindMCQ, KindTransform)
	}
	// Cloze needs a feature with a SURFACE exponent — something that can be
	// located in a real sentence. `Skill=Politeness` cannot, so a topic carrying
	// only skill features reports "no generator" rather than promising items and
	// then serving none.
	if len(g.clozeTargets(t)) > 0 {
		kinds = append(kinds, KindCloze)
	}

	// The two newest kinds are offered only when a probe proves the pack can
	// build one: a verb topic whose forms carry no person (the potential), or a
	// case with no unambiguous noun in a real sentence, would otherwise
	// advertise exercises and serve nothing.
	if g.personTopic(t) && g.probeKind(t, KindPerson) {
		kinds = append(kinds, KindPerson)
	}
	if len(g.qwordCases(t)) > 0 && g.probeKind(t, KindQword) {
		kinds = append(kinds, KindQword)
	}
	if g.sceneTopic(t) && g.probeKind(t, KindScene) {
		kinds = append(kinds, KindScene)
	}
	if g.matchTopic(t) && g.probeKind(t, KindMatch) {
		kinds = append(kinds, KindMatch)
	}
	if len(g.sortFeatures(t)) > 0 && g.probeKind(t, KindSort) {
		kinds = append(kinds, KindSort)
	}

	if len(kinds) == 0 {
		return Availability{Reason: ReasonNoGenerator}
	}
	return Availability{Available: true, Kinds: kinds}
}

// nominalSlots reports whether a topic's features map onto cells of the noun
// paradigm, which is what the transform and MCQ generators need.
//
// It asks nominalTargets rather than re-deriving the rule, because the two
// disagreeing means the topic advertises item kinds it then cannot produce —
// which is worse than admitting up front that we have no drills for it.
func (g *Generator) nominalSlots(t curriculum.Topic) bool {
	return len(g.nominalTargets(t)) > 0
}

func contains(ks []Kind, k Kind) bool {
	for _, x := range ks {
		if x == k {
			return true
		}
	}
	return false
}

// Next builds up to req.Count items for a topic.
//
// The mix depends on the phase, which is the operational half of [R5]:
//
//   - BLOCKED: one feature at a time, one kind at a time. The learner is
//     building a form–meaning association and the load has to stay low.
//   - INTERLEAVED: minimally contrasting items shuffled together — `-ban` versus
//     `-ba` of the SAME noun, definite versus indefinite of the SAME verb. Not
//     random topics: Brunmair & Richter's mechanism is discriminability, and
//     variety for its own sake is not the same thing.
//   - MASTERED: the interleaved mix, at maintenance volume.
func (g *Generator) Next(req Request) ([]Item, Availability, error) {
	if err := g.load(); err != nil {
		return nil, Availability{Reason: ReasonNoPack}, nil
	}
	avail := g.KindsFor(req.Topic)
	if !avail.Available {
		return nil, avail, nil
	}
	if req.Count <= 0 {
		req.Count = 10
	}
	// Placement measures with the kinds it always used; a multi-tile sort is a
	// teaching format, not a probe.
	if req.Origin == "placement" {
		avail.Kinds = slices.DeleteFunc(slices.Clone(avail.Kinds), func(k Kind) bool { return k == KindSort || k == KindMatch || k == KindScene })
	}
	if req.Rand == nil {
		req.Rand = rand.New(rand.NewSource(1))
	}

	// Blocked practice is ONE kind per session. Which one rotates: the least
	// recently used kind with material goes first, so two sessions in a row do
	// not ask the same question type. If that kind cannot find attested material
	// — a topic whose only suffix is invariant, say — we fall through to the
	// next rather than serving a mixture, because the mixture is the thing
	// blocked practice exists to avoid.
	if req.Phase == Blocked {
		// A kind whose material is thin (a handful of sentences) can fall short
		// of the request; the next kind is tried and the fullest session wins,
		// so rotating onto a thin kind never costs the learner items.
		var best []Item
		for _, k := range blockedOrder(avail.Kinds, req.Recent) {
			items := g.fill([]Kind{k}, req)
			// Without memory (placement, the probe) the first kind with any
			// material wins, as it always did.
			if len(items) >= req.Count || (req.Recent == nil && len(items) > 0) {
				return items, avail, nil
			}
			if len(items) > len(best) {
				best = items
			}
		}
		if len(best) > 0 {
			return best, avail, nil
		}
		return nil, Availability{Kinds: avail.Kinds, Reason: ReasonNoMaterial}, nil
	}

	if items := g.fill(interleavedOrder(avail.Kinds, req), req); len(items) > 0 {
		return items, avail, nil
	}
	return nil, Availability{Kinds: avail.Kinds, Reason: ReasonNoMaterial}, nil
}

// attemptBudget is how many items fill may try to build for a request. A
// generator declines whenever the pack has no attested material for the lemma it
// drew, and anti-repeat discards more; the floor keeps a one-item request (the
// availability probe) from giving up early.
func attemptBudget(count int) int { return max(count*8, 48) }

// fill builds req.Count items, cycling through the given kinds.
//
// Candidates are collected first and chosen second, because "which candidates
// are acceptable" depends on what the session has already taken. The choice
// relaxes in stages, and a stage is only used when the stricter ones cannot
// fill the request — recency must never make a session come back short:
//
//  1. no exact repeat of a recent item, no recent sentence, no lemma that is
//     already used twice (recently or in this session);
//  2. the lemma limit is lifted;
//  3. the sentence limit is lifted;
//  4. exact repeats are allowed, the oldest first.
//
// Inside one request an item is never served twice (a repeat is not retrieval
// practice, it is copying), a sentence is used once and a lemma at most twice
// for as long as a stricter stage can supply them.
func (g *Generator) fill(kinds []Kind, req Request) []Item {
	if len(kinds) == 1 && kinds[0] == KindPerson {
		return g.fillPerson(req)
	}
	type cand struct {
		it    Item
		order int
	}
	var cands []cand
	var spill []Item
	seen := map[string]bool{}
	strict := 0

	need := req.Count
	if req.Recent != nil {
		need = req.Count + req.Count/2 + 2
		// A sort item costs a dozen lookups; six fresh lemmas collide rarely, so
		// no slack is bought for a session made of nothing else.
		if len(kinds) == 1 && kinds[0] == KindSort {
			need = req.Count
		}
	}
	budget := attemptBudget(req.Count)
	// Only a small share of lemmas can make a match or a scene (three or more
	// attested cells, an object that fits); distinct ones take more draws.
	if len(kinds) == 1 && (kinds[0] == KindMatch || kinds[0] == KindScene) {
		budget *= 4
	}
	for i := range budget {
		it, ok := g.build(kinds[i%len(kinds)], req)
		if !ok {
			continue
		}
		key := it.Key() + "|" + it.Frame
		if seen[key] || (it.Frame != "" && it.Frame != BlankMark && seen["frame|"+it.Frame]) {
			continue
		}
		// Items with no frame of their own would look new each time: a question
		// word item (same noun, same case, another sentence), a scene (same place
		// and case, or the same object again) and a match (the same lemma). One
		// per request while alternatives exist; the rest wait in reserve and are
		// used only if the request cannot be filled without them.
		if dks := dedupeKeys(it); len(dks) > 0 {
			dup := false
			for _, k := range dks {
				dup = dup || seen[k]
			}
			if dup {
				if !seen[key] {
					seen[key] = true
					spill = append(spill, it)
				}
				continue
			}
			for _, k := range dks {
				seen[k] = true
			}
		}
		seen[key] = true
		if it.Frame != "" && it.Frame != BlankMark {
			seen["frame|"+it.Frame] = true
		}
		cands = append(cands, cand{it, len(cands)})
		if stageOf(it, req.Recent, nil, nil) == 0 {
			strict++
		}
		// Enough clean candidates (with slack for in-session collisions): stop
		// spending lookups.
		if strict >= need {
			break
		}
	}

	for _, it := range spill {
		if len(cands) >= req.Count {
			break
		}
		cands = append(cands, cand{it, len(cands)})
	}

	items := make([]Item, 0, req.Count)
	lemmaUse := map[int64]int{}
	sentences := map[int64]bool{}
	for len(items) < req.Count && len(cands) > 0 {
		best, bestStage, bestAge := -1, 99, -1
		for i, c := range cands {
			st := stageOf(c.it, req.Recent, lemmaUse, sentences)
			age := 0
			if st == 3 {
				age, _ = req.Recent.keyRank(c.it)
			}
			if st < bestStage || (st == bestStage && st == 3 && age > bestAge) {
				best, bestStage, bestAge = i, st, age
			}
			if st == 0 && bestStage == 0 {
				break // candidates are already in random order
			}
		}
		it := cands[best].it
		cands = append(cands[:best], cands[best+1:]...)
		it.Origin = req.Origin
		items = append(items, it)
		if it.LemmaID != 0 {
			lemmaUse[it.LemmaID]++
		}
		if it.SentenceID != 0 {
			sentences[it.SentenceID] = true
		}
	}
	return items
}

// dedupeKeys are the in-request uniqueness keys of the kinds that carry no
// frame (see fill).
func dedupeKeys(it Item) []string {
	id := strconv.FormatInt(it.LemmaID, 10)
	switch it.Kind {
	case KindQword:
		return []string{"lc|" + id + "|" + it.Headline}
	case KindScene:
		return []string{"lc|" + id + "|" + it.Headline, "obj|" + strconv.FormatInt(it.sceneObjectID, 10)}
	case KindMatch:
		return []string{"lm|" + id}
	}
	return nil
}

// maxLemmaUses is how often one lemma may appear (recently, plus in the session
// being built) before a stricter stage prefers another.
const maxLemmaUses = 2

// stageOf is the relaxation stage an item needs: 0 is fully fresh, 3 is an exact
// repeat of a recent item. `lemmaUse` and `sentences` are what the session being
// built has already taken; nil means nothing.
func stageOf(it Item, r *Recent, lemmaUse map[int64]int, sentences map[int64]bool) int {
	if _, repeat := r.keyRank(it); repeat {
		return 3
	}
	if it.SentenceID != 0 && (r.sentenceUsed(it.SentenceID) || sentences[it.SentenceID]) {
		return 2
	}
	if it.LemmaID != 0 && r.lemmaUses(it.LemmaID)+lemmaUse[it.LemmaID] >= maxLemmaUses {
		return 1
	}
	return 0
}

// blockedPreference orders the kinds for blocked practice.
//
// Recognition-style choices come before typed production: blocked practice is
// where the form–meaning association is still being built, and asking for
// production before it exists is the load Hwang found to be counterproductive.
func blockedPreference(available []Kind) []Kind {
	order := []Kind{KindAllomorph, KindConjugation, KindCase, KindMCQ, KindQword, KindScene, KindSort, KindMatch,
		KindCloze, KindTransform, KindPerson}
	out := make([]Kind, 0, len(available))
	for _, k := range order {
		if contains(available, k) {
			out = append(out, k)
		}
	}
	return out
}

// blockedOrder is the order in which blocked practice tries kinds: the kinds in
// rotation, least recently used first, then everything else in preference
// order. Only choice kinds rotate until the topic has typedUnlockCorrect
// correct blocked answers; typed kinds are still the fallback when nothing else
// has material, so a typed-only topic is never starved.
func blockedOrder(available []Kind, r *Recent) []Kind {
	pref := blockedPreference(available)
	var rotation, rest []Kind
	for _, k := range pref {
		if !k.Typed() || (r != nil && r.CorrectBlocked >= typedUnlockCorrect) {
			rotation = append(rotation, k)
		} else {
			rest = append(rest, k)
		}
	}
	sort.SliceStable(rotation, func(i, j int) bool {
		return r.kindAge(rotation[i]) > r.kindAge(rotation[j])
	})
	return append(rotation, rest...)
}

// interleavedOrder draws the kinds for an interleaved request, weighted toward
// those used least recently so a long run of sessions does not collapse onto one
// or two kinds. Without recency it is a plain shuffle.
func interleavedOrder(kinds []Kind, req Request) []Kind {
	if req.Recent == nil {
		return shuffled(kinds, req.Rand)
	}
	pool := append([]Kind(nil), kinds...)
	sort.SliceStable(pool, func(i, j int) bool {
		return req.Recent.kindAge(pool[i]) < req.Recent.kindAge(pool[j])
	})
	// pool[0] is the most recently used: weight 1 .. len for the stalest.
	out := make([]Kind, 0, len(pool))
	for len(pool) > 0 {
		total := 0
		for i := range pool {
			total += i + 1
		}
		pick := req.Rand.Intn(total)
		for i := range pool {
			pick -= i + 1
			if pick < 0 {
				out = append(out, pool[i])
				pool = append(pool[:i], pool[i+1:]...)
				break
			}
		}
	}
	return out
}

func shuffled(kinds []Kind, rnd *rand.Rand) []Kind {
	out := append([]Kind(nil), kinds...)
	rnd.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func (g *Generator) build(k Kind, req Request) (Item, bool) {
	switch k {
	case KindAllomorph:
		return g.allomorphItem(req)
	case KindCase:
		return g.caseItem(req)
	case KindConjugation:
		return g.conjugationItem(req)
	case KindCloze:
		return g.clozeItem(req)
	case KindTransform:
		return g.transformItem(req)
	case KindMCQ:
		return g.mcqItem(req)
	case KindPerson:
		return g.personItem(req)
	case KindQword:
		return g.qwordItem(req)
	case KindSort:
		return g.sortItem(req)
	case KindMatch:
		return g.matchItem(req)
	case KindScene:
		return g.sceneItem(req)
	}
	return Item{}, false
}

// ── vocabulary sampling ───────────────────────────────────────────────────

// bandFor maps a topic's CEFR level to a frequency band. An A1 topic drilled on
// B2 vocabulary is two lessons at once, and the learner fails the one they were
// not being taught.
func bandFor(level string) int {
	switch level {
	case "A1":
		return 1200
	case "A2":
		return 2500
	case "B1":
		return 4000
	default:
		return vocabularyCeiling
	}
}

// pickWord samples vocabulary, preferring words the learner already knows.
//
// The preference is soft: two thirds of picks try the known set first and fall
// through when it is empty. A hard filter would leave a brand-new learner with
// no drills at all, which is worse than drilling the inessive on a word they
// have not met — the grammar point does not depend on knowing the noun.
func (g *Generator) pickWord(pool []Word, req Request) (Word, bool) {
	if len(pool) == 0 {
		return Word{}, false
	}
	ceiling := bandFor(req.Topic.Level)

	end := sort.Search(len(pool), func(i int) bool { return pool[i].Rank > ceiling })
	if end < 20 {
		end = min(len(pool), 20)
	}
	preferKnown := len(req.KnownLemmas) > 0 && req.Rand.Intn(3) < 2
	if preferKnown {
		// Known words are a preference, never a trap: a learner who knows few
		// words in the band would be handed the same handful over and over. So
		// the preference only applies while at least minKnownPool known lemmas
		// remain AFTER the recently used ones are taken out.
		var known []Word
		for _, w := range pool[:end] {
			if req.KnownLemmas[w.LemmaID] && req.Recent.lemmaUses(w.LemmaID) == 0 {
				known = append(known, w)
			}
		}
		if len(known) >= minKnownPool || (req.Recent == nil && len(known) > 0) {
			return known[req.Rand.Intn(len(known))], true
		}
	}
	w := pool[req.Rand.Intn(end)]
	// A few redraws steer away from words just used; fill enforces the limit.
	for range 3 {
		if req.Recent.lemmaUses(w.LemmaID) == 0 {
			break
		}
		w = pool[req.Rand.Intn(end)]
	}
	return w, true
}

// minKnownPool is how many unseen-lately known lemmas the band must offer
// before pickWord prefers known words.
const minKnownPool = 30

// gloss loads a lemma's meanings. Both locales, always — a drill that can only
// explain itself in English is an incomplete drill. See ADR-004.
func (g *Generator) gloss(lemmaID int64) (ru, en string) {
	rows, err := g.pack.DB().Query(`
		SELECT lang, gloss FROM lemma_gloss WHERE lemma_id = ?
		 ORDER BY sense_idx LIMIT 8`, lemmaID)
	if err != nil {
		return "", ""
	}
	defer rows.Close()
	for rows.Next() {
		var lg, gl string
		if err := rows.Scan(&lg, &gl); err != nil {
			continue
		}
		switch lg {
		case "ru":
			if ru == "" {
				ru = gl
			}
		case "en":
			if en == "" {
				en = gl
			}
		}
	}
	return ru, en
}

// ── the generators ────────────────────────────────────────────────────────

// harmonyDrillFeatures returns the features usable for pure harmony practice:
// those with at least two distinct allomorphs, in teach order.
//
// Restricted to the early, high-frequency suffixes. Harmony practice is about
// the CHOICE, so the suffix only has to be one whose variants differ — but
// drilling it on the translative `-vá/-vé` or the distributive `-nként` would be
// teaching an exotic suffix under the guise of teaching harmony, and the harmony
// topic sits at teach order 20, before any case has been introduced.
const harmonyDrillCeiling = 46

func (g *Generator) harmonyDrillFeatures() []Feature {
	var out []Feature
	for _, f := range g.features {
		if f.TeachOrder > harmonyDrillCeiling || len(Distinct(f.Allomorphs)) < 2 {
			continue
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TeachOrder < out[j].TeachOrder })
	return out
}

// topicAllomorphFeatures returns the topic's own features that admit a choice.
func (g *Generator) topicAllomorphFeatures(t curriculum.Topic) []Feature {
	var out []Feature
	for _, id := range t.FeatureIDs {
		f, ok := g.features[id]
		if !ok || len(Distinct(f.Allomorphs)) < 2 {
			continue
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TeachOrder < out[j].TeachOrder })
	return out
}

// allomorphItem: `ház` + inessive → házban or házben?
//
// The single most valuable item type in the app, because harmony is the skill
// the learner needs several thousand repetitions of. [R15]
func (g *Generator) allomorphItem(req Request) (Item, bool) {
	pool := g.topicAllomorphFeatures(req.Topic)
	harmonyOnly := req.Topic.MotorSkill()
	if harmonyOnly {
		pool = g.harmonyDrillFeatures()
	}
	if len(pool) == 0 {
		return Item{}, false
	}

	for range maxCandidateTries {
		f := g.weightedFeature(pool, req)
		words, spec := g.poolFor(f)
		w, ok := g.pickWord(words, req)
		if !ok {
			continue
		}
		correct, _, alternatives, ok := g.attestedAllomorph(w, f, spec)
		if !ok {
			continue
		}

		opts := append([]Option{{Text: correct.Form, Attested: true}}, alternatives...)
		req.Rand.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })

		ru, en := g.gloss(w.LemmaID)
		// Only the harmony skill is required when the item is pure harmony
		// practice: the learner picking `házban` over `házben` has not been
		// asked what the inessive means, so charging them for the inessive
		// would be recording a fact the item never tested.
		feats := []string{curriculum.FeatureVowelHarmony}
		headline := curriculum.FeatureVowelHarmony
		if !harmonyOnly {
			feats = []string{f.ID, curriculum.FeatureVowelHarmony}
			headline = f.ID
		}
		return Item{
			TopicCode: req.Topic.Code, Kind: KindAllomorph, Phase: req.Phase,
			PromptKey: "drill.prompt.allomorph",
			PromptArgs: map[string]string{
				"lemma": w.Lemma, "feature": f.ID,
				"harmony": string(Classify(w.Lemma)),
			},
			Lemma: w.Lemma, LemmaID: w.LemmaID, GlossRU: ru, GlossEN: en,
			Options: opts, FeatureIDs: feats, Headline: headline,
			Attested: true, expected: correct.Form,
		}, true
	}
	return Item{}, false
}

// attestedAllomorph finds the attested form of a lemma for a feature and which
// allomorph of the feature it uses, plus the wrong-allomorph distractors. It
// declines (ok=false) whenever the form cannot be decomposed honestly or the
// pack attests a rival allomorph for the same lemma and feature.
func (g *Generator) attestedAllomorph(w Word, f Feature, spec morph.FormSpec) (correct morph.Form, used string, alternatives []Option, ok bool) {
	correct, found := g.analyzer.BestForm(w.LemmaID, spec)
	if !found || !correct.Attested() {
		return
	}

	// Which allomorph did the attested form actually use? If none of them
	// matches the ending, the form involves an assimilation or a linking
	// vowel we cannot decompose (`ház` + `-val` → `házzal`), and we decline
	// to build an item rather than invent a decomposition.
	for _, allo := range Distinct(f.Allomorphs) {
		if allo != "" && strings.HasSuffix(correct.Form, allo) && len(allo) > len(used) {
			used = allo
		}
	}
	if used == "" {
		return
	}
	stem := correct.Form[:len(correct.Form)-len(used)]

	// The stem must be recognisably the lemma, or the item is really a
	// stem-alternation question wearing a harmony costume.
	if !strings.HasPrefix(strings.ToLower(stem), strings.ToLower(w.Lemma[:min(len(w.Lemma), 3)])) {
		return
	}

	alternatives, ok = g.alternativeForms(w.LemmaID, stem, used, f, correct.Form)
	return
}

// poolFor picks the vocabulary and the paradigm cell a feature belongs to.
//
// Keyed off the feature, not off the word list, so an empty pool cannot be
// indexed into — and so the spec can never disagree with the vocabulary it was
// built for.
func (g *Generator) poolFor(f Feature) ([]Word, morph.FormSpec) {
	switch f.Key {
	case "Mood", "Voice", "Aspect", "VerbForm", "Tense", "Definite":
		return g.verbs, verbalSpec(f)
	case "Degree":
		return g.adjs, nominalSpec(f)
	}
	return g.nouns, nominalSpec(f)
}

// alternativeForms builds the wrong-allomorph distractors.
//
// A distractor is dropped if the pack attests it for the SAME lemma with the
// SAME feature: some stems genuinely waver, and marking a real alternative wrong
// would be teaching an error. Losing the item entirely is the right outcome when
// no clean distractor survives.
func (g *Generator) alternativeForms(lemmaID int64, stem, used string, f Feature, correctForm string) ([]Option, bool) {
	forms, err := g.analyzer.FormsOf(lemmaID)
	if err != nil {
		return nil, false
	}
	attestedSame := map[string]bool{}
	for _, fm := range forms {
		if fm.Attested() && fm.Feats[f.Key] == f.Value {
			attestedSame[strings.ToLower(fm.Form)] = true
		}
	}

	var out []Option
	for _, allo := range Distinct(f.Allomorphs) {
		if allo == used || allo == "" {
			continue
		}
		cand := stem + allo
		if strings.EqualFold(cand, correctForm) || attestedSame[strings.ToLower(cand)] {
			return nil, false
		}
		out = append(out, Option{Text: cand, Attested: g.analyzer.IsAttested(cand)})
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// weightedFeature over-samples features the learner has recently got wrong.
// See from_error.go — this is the "targeted follow-up" half of that loop.
func (g *Generator) weightedFeature(pool []Feature, req Request) Feature {
	if len(req.WeakFeatures) > 0 && req.Rand.Intn(2) == 0 {
		var weak []Feature
		for _, f := range pool {
			for _, id := range req.WeakFeatures {
				if f.ID == id {
					weak = append(weak, f)
				}
			}
		}
		if len(weak) > 0 {
			return weak[req.Rand.Intn(len(weak))]
		}
	}
	return pool[req.Rand.Intn(len(pool))]
}

// caseItem: "into the house" → házban / házba / házból?
//
// The distractors are the SAME noun in the contrasting cells of the locative
// grid, which is R5's requirement made concrete: the items differ in exactly the
// dimension being learned, and in nothing else. Every option shares the stem's
// harmony class, so harmony is NOT among the graded features.
func (g *Generator) caseItem(req Request) (Item, bool) {
	var grid []Feature
	for _, id := range req.Topic.FeatureIDs {
		if f, ok := g.features[id]; ok && f.Grid != "" {
			grid = append(grid, f)
		}
	}
	if len(grid) < 2 {
		return Item{}, false
	}

	for range maxCandidateTries {
		target := g.weightedFeature(grid, req)
		w, ok := g.pickWord(g.nouns, req)
		if !ok {
			continue
		}
		correct, ok := g.analyzer.BestForm(w.LemmaID, nominalSpec(target))
		if !ok || !correct.Attested() {
			continue
		}

		// Contrast within the same plane first (`-ban`/`-ba`/`-ból`), falling
		// back to the same direction across planes (`-ban`/`-n`/`-nál`). Both
		// are minimal contrasts; the row is the sharper one because only the
		// direction changes.
		contrasts := g.contrastCells(grid, target, true)
		if len(contrasts) < 1 {
			contrasts = g.contrastCells(grid, target, false)
		}
		opts := []Option{{Text: correct.Form, Attested: true}}
		feats := []string{target.ID}
		for _, c := range contrasts {
			alt, ok := g.analyzer.BestForm(w.LemmaID, nominalSpec(c))
			if !ok || strings.EqualFold(alt.Form, correct.Form) {
				continue
			}
			opts = append(opts, Option{Text: alt.Form, Attested: g.analyzer.IsAttested(alt.Form)})
			if len(opts) >= 3 {
				break
			}
		}
		if len(opts) < 2 {
			continue
		}
		req.Rand.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })

		ru, en := g.gloss(w.LemmaID)
		return Item{
			TopicCode: req.Topic.Code, Kind: KindCase, Phase: req.Phase,
			PromptKey: "drill.prompt.case",
			PromptArgs: map[string]string{
				"lemma": w.Lemma, "feature": target.ID,
				"row": target.Row(), "column": target.Column(),
			},
			Lemma: w.Lemma, LemmaID: w.LemmaID, GlossRU: ru, GlossEN: en,
			Options: opts, FeatureIDs: feats, Headline: target.ID,
			Attested: true, expected: correct.Form,
		}, true
	}
	return Item{}, false
}

// contrastCells returns the grid neighbours of a cell: same row (plane) when
// sameRow, otherwise same column (direction).
func (g *Generator) contrastCells(grid []Feature, target Feature, sameRow bool) []Feature {
	var out []Feature
	for _, f := range grid {
		if f.ID == target.ID {
			continue
		}
		if sameRow && f.Row() == target.Row() {
			out = append(out, f)
		} else if !sameRow && f.Column() == target.Column() {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TeachOrder < out[j].TeachOrder })
	return out
}

// transformItem: `ház` → plural → ? Typed, because production is the skill that
// transfers to speech. [R12]
func (g *Generator) transformItem(req Request) (Item, bool) {
	targets := g.nominalTargets(req.Topic)
	if len(targets) == 0 {
		return Item{}, false
	}
	for range maxCandidateTries {
		f := g.weightedFeature(targets, req)
		w, ok := g.pickWord(g.nouns, req)
		if !ok {
			continue
		}
		correct, ok := g.analyzer.BestForm(w.LemmaID, nominalSpec(f))
		if !ok || !correct.Attested() || strings.EqualFold(correct.Form, w.Lemma) {
			continue
		}
		ru, en := g.gloss(w.LemmaID)
		feats := []string{f.ID}
		// Producing the form means choosing the allomorph too, whenever the
		// suffix has more than one.
		if len(Distinct(f.Allomorphs)) >= 2 {
			feats = append(feats, curriculum.FeatureVowelHarmony)
		}
		return Item{
			TopicCode: req.Topic.Code, Kind: KindTransform, Phase: req.Phase,
			PromptKey:  "drill.prompt.transform",
			PromptArgs: map[string]string{"lemma": w.Lemma, "feature": f.ID},
			Lemma:      w.Lemma, LemmaID: w.LemmaID, GlossRU: ru, GlossEN: en,
			FeatureIDs: feats, Headline: f.ID,
			Attested: true, expected: correct.Form,
		}, true
	}
	return Item{}, false
}

// mcqItem: the same question as transform, but recognition rather than recall.
// Blocked practice leans on this one; interleaved practice mixes both.
func (g *Generator) mcqItem(req Request) (Item, bool) {
	targets := g.nominalTargets(req.Topic)
	if len(targets) < 1 {
		return Item{}, false
	}
	for range maxCandidateTries {
		f := g.weightedFeature(targets, req)
		w, ok := g.pickWord(g.nouns, req)
		if !ok {
			continue
		}
		correct, ok := g.analyzer.BestForm(w.LemmaID, nominalSpec(f))
		if !ok || !correct.Attested() {
			continue
		}
		opts := []Option{{Text: correct.Form, Attested: true}}
		seen := map[string]bool{strings.ToLower(correct.Form): true}
		for _, alt := range g.mcqDistractors(req.Topic, f) {
			if alt.ID == f.ID {
				continue
			}
			af, ok := g.analyzer.BestForm(w.LemmaID, nominalSpec(alt))
			if !ok || seen[strings.ToLower(af.Form)] {
				continue
			}
			seen[strings.ToLower(af.Form)] = true
			opts = append(opts, Option{Text: af.Form, Attested: g.analyzer.IsAttested(af.Form)})
			if len(opts) >= 4 {
				break
			}
		}
		if len(opts) < 2 {
			continue
		}
		req.Rand.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })
		ru, en := g.gloss(w.LemmaID)
		return Item{
			TopicCode: req.Topic.Code, Kind: KindMCQ, Phase: req.Phase,
			PromptKey:  "drill.prompt.mcq",
			PromptArgs: map[string]string{"lemma": w.Lemma, "feature": f.ID},
			Lemma:      w.Lemma, LemmaID: w.LemmaID, GlossRU: ru, GlossEN: en,
			Options: opts, FeatureIDs: []string{f.ID}, Headline: f.ID,
			Attested: true, expected: correct.Form,
		}, true
	}
	return Item{}, false
}

// mcqDistractors is the pool a multiple-choice item draws its wrong answers from.
//
// The topic's own cells first — `-ban` against `-ba` is the sharpest contrast
// there is. But a topic that teaches exactly ONE cell (the accusative) has no
// sibling to contrast with, and the citation form is then the right distractor:
// `ház` against `házat` IS the accusative question. Without this a single-cell
// topic silently produces no items at all.
func (g *Generator) mcqDistractors(t curriculum.Topic, target Feature) []Feature {
	out := g.nominalTargets(t)
	if target.Key == "Case" {
		if nom, ok := g.features["Case=Nom"]; ok {
			out = append(out, nom)
		}
	}
	if target.Key == "Number" && target.Value == "Plur" {
		if sing, ok := g.features["Number=Sing"]; ok {
			out = append(out, sing)
		}
	}
	return out
}

// nominalTargets returns the topic's features that name a cell of the noun
// paradigm, which is what transform and MCQ can build.
func (g *Generator) nominalTargets(t curriculum.Topic) []Feature {
	var out []Feature
	for _, id := range t.FeatureIDs {
		f, ok := g.features[id]
		if !ok || !f.HasExponent() {
			continue
		}
		switch f.Key {
		case "Case":
			if f.Value == "Nom" {
				continue // the citation form is not a transformation
			}
			out = append(out, f)
		case "Number":
			if f.Value == "Plur" {
				out = append(out, f)
			}
		case "Person[psor]", "Number[psed]", "Degree":
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TeachOrder < out[j].TeachOrder })
	return out
}

// clozeItem: an attested sentence with one form blanked, typed from the lemma.
//
// The highest-value item in the app after the harmony drill, and the one that
// most resembles the review loop: it is production inside real context rather
// than a paradigm exercise, which is the direction that transfers. [R12]
//
// The prompt names both the lemma and the feature, so the answer is determined —
// `A ___ van` with nothing else stated would accept half the dictionary.
func (g *Generator) clozeItem(req Request) (Item, bool) {
	targets := g.clozeTargets(req.Topic)
	if len(targets) == 0 {
		return Item{}, false
	}
	for range maxCandidateTries {
		f := g.weightedFeature(targets, req)
		pool, _ := g.poolFor(f)
		w, ok := g.pickWord(pool, req)
		if !ok {
			continue
		}
		sentID, hu, ok := g.sampleSentenceContaining([]int64{w.LemmaID}, req.Rand)
		if !ok {
			continue
		}
		tokens := morph.Tokenize(hu)
		if n := wordCount(tokens); n < 4 || n > 14 {
			continue
		}
		for _, t := range tokens {
			if !t.IsWord {
				continue
			}
			an, ok := g.readingOf(t.Text, w.LemmaID, f)
			if !ok {
				continue
			}
			ru, en := g.gloss(w.LemmaID)
			feats := []string{f.ID}
			if len(Distinct(f.Allomorphs)) >= 2 {
				feats = append(feats, curriculum.FeatureVowelHarmony)
			}
			_ = an
			return Item{
				TopicCode: req.Topic.Code, Kind: KindCloze, Phase: req.Phase,
				PromptKey:  "drill.prompt.cloze",
				PromptArgs: map[string]string{"lemma": w.Lemma, "feature": f.ID},
				Frame:      hu[:t.Start] + BlankMark + hu[t.End:],
				Lemma:      w.Lemma, LemmaID: w.LemmaID, GlossRU: ru, GlossEN: en,
				FeatureIDs: feats, Headline: f.ID,
				Attested: true, SentenceID: sentID, expected: t.Text,
			}, true
		}
	}
	return Item{}, false
}

// clozeTargets are the topic's features that can actually be found on a token.
//
// The test is whether the feature has a SURFACE EXPONENT — a suffix in the
// pack's allomorph map. That single rule removes every vacuous target at once:
// `Tense=Pres`, `Mood=Ind`, `Number=Sing`, `Definite=Ind` and `Case=Nom` are all
// unmarked defaults carried by nearly every word, so blanking a token "because
// it is indicative" tests nothing and produces items whose stated feature has no
// visible relationship to the answer.
//
// `Definite=Def` is unmarked too, and is not a loss: the definite conjugation
// has its own generator, which contrasts the two forms directly.
func (g *Generator) clozeTargets(t curriculum.Topic) []Feature {
	var out []Feature
	for _, id := range t.FeatureIDs {
		f, ok := g.features[id]
		if !ok || !f.HasExponent() {
			continue
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TeachOrder < out[j].TeachOrder })
	return out
}

// readingOf returns the analysis of a token that belongs to the wanted lemma and
// UNAMBIGUOUSLY carries the wanted feature.
//
// Two refusals, both load-bearing:
//
//   - Generated readings. A cloze whose correct answer is a form our own
//     generator invented would be marking the learner against a guess.
//   - Syncretic forms. If the same string also reads as another value of the same
//     feature, blanking it does not test that feature. Hungarian has real
//     syncretism, and the pack has mis-parses on top of it — `csapdába` is tagged
//     both singular and plural — so a plural cloze on it would be unanswerable.
func (g *Generator) readingOf(form string, lemmaID int64, f Feature) (morph.Analysis, bool) {
	all, err := g.analyzer.Lookup(form)
	if err != nil {
		return morph.Analysis{}, false
	}
	var match morph.Analysis
	found := false
	for _, a := range all {
		if a.LemmaID != lemmaID || a.Source == "generated" {
			continue
		}
		other, present := a.Feats[f.Key]
		if present && other != f.Value {
			return morph.Analysis{}, false
		}
		if other == f.Value {
			match, found = a, true
		}
	}
	return match, found
}
