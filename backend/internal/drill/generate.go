package drill

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
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
)

// AllKinds in a stable order.
var AllKinds = []Kind{KindAllomorph, KindCase, KindConjugation, KindCloze, KindTransform, KindMCQ}

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
func (k Kind) Typed() bool { return k == KindCloze || k == KindTransform }

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
	anchors anchors
	nouns   []Word
	verbs   []Word
	adjs    []Word
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
		if w.Lemma != strings.ToLower(w.Lemma) || excludedLemmas[w.Lemma] {
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
	Rand         *rand.Rand
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
	if req.Rand == nil {
		req.Rand = rand.New(rand.NewSource(1))
	}

	// Blocked practice is ONE kind. If the preferred kind cannot find attested
	// material — a topic whose only suffix is invariant, say — we fall through
	// to the next kind rather than serving a mixture, because the mixture is the
	// thing blocked practice exists to avoid.
	if req.Phase == Blocked {
		for _, k := range blockedPreference(avail.Kinds) {
			if items := g.fill([]Kind{k}, req); len(items) > 0 {
				return items, avail, nil
			}
		}
		return nil, Availability{Kinds: avail.Kinds, Reason: ReasonNoMaterial}, nil
	}

	if items := g.fill(shuffled(avail.Kinds, req.Rand), req); len(items) > 0 {
		return items, avail, nil
	}
	return nil, Availability{Kinds: avail.Kinds, Reason: ReasonNoMaterial}, nil
}

// fill attempts to build req.Count items, cycling through the given kinds.
//
// The over-generation factor exists because a generator may decline to build an
// item when the pack has no attested material for a particular lemma. Declining
// is correct, so we ask for more attempts than we need.
func (g *Generator) fill(kinds []Kind, req Request) []Item {
	items := make([]Item, 0, req.Count)
	seen := map[string]bool{}

	// A floor as well as a multiple of the request. A generator declines an item
	// whenever the pack has no attested material for the lemma it drew, so a
	// one-item request that gave up after four attempts would report "no
	// material" for a topic a six-item request handles fine — and the
	// availability probe asks for exactly one item.
	attempts := max(req.Count*4, 24)
	for i := range attempts {
		it, ok := g.build(kinds[i%len(kinds)], req)
		if !ok {
			continue
		}
		// Never serve the same question twice in one session: a repeat is not
		// retrieval practice, it is copying.
		key := string(it.Kind) + "|" + it.Frame + "|" + it.Lemma + "|" + it.Headline
		if seen[key] {
			continue
		}
		seen[key] = true
		it.Origin = req.Origin
		items = append(items, it)
		if len(items) >= req.Count {
			break
		}
	}
	return items
}

// blockedPreference orders the kinds for blocked practice.
//
// Recognition-style choices come before typed production: blocked practice is
// where the form–meaning association is still being built, and asking for
// production before it exists is the load Hwang found to be counterproductive.
func blockedPreference(available []Kind) []Kind {
	order := []Kind{KindAllomorph, KindConjugation, KindCase, KindMCQ, KindCloze, KindTransform}
	out := make([]Kind, 0, len(available))
	for _, k := range order {
		if contains(available, k) {
			out = append(out, k)
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
		var known []Word
		for _, w := range pool[:end] {
			if req.KnownLemmas[w.LemmaID] {
				known = append(known, w)
			}
		}
		if len(known) > 0 {
			return known[req.Rand.Intn(len(known))], true
		}
	}
	return pool[req.Rand.Intn(end)], true
}

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
		correct, ok := g.analyzer.BestForm(w.LemmaID, spec)
		if !ok || !correct.Attested() {
			continue
		}

		// Which allomorph did the attested form actually use? If none of them
		// matches the ending, the form involves an assimilation or a linking
		// vowel we cannot decompose (`ház` + `-val` → `házzal`), and we decline
		// to build an item rather than invent a decomposition.
		used := ""
		for _, allo := range Distinct(f.Allomorphs) {
			if allo != "" && strings.HasSuffix(correct.Form, allo) && len(allo) > len(used) {
				used = allo
			}
		}
		if used == "" {
			continue
		}
		stem := correct.Form[:len(correct.Form)-len(used)]

		// The stem must be recognisably the lemma, or the item is really a
		// stem-alternation question wearing a harmony costume.
		if !strings.HasPrefix(strings.ToLower(stem), strings.ToLower(w.Lemma[:min(len(w.Lemma), 3)])) {
			continue
		}

		alternatives, ok := g.alternativeForms(w.LemmaID, stem, used, f, correct.Form)
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
