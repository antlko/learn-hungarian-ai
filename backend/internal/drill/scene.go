package drill

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"tanulom/internal/curriculum"
	"tanulom/internal/morph"
	"tanulom/internal/pictures"
)

// The scene drill: a picture shows an OBJECT at a PLACE with a motion, and the
// learner picks the locative FORM of the place noun.
//
//	cat + box, static      -> Hol?     -> a dobozban  (Ine)
//	cat + box, arrow into  -> Hova?    -> a dobozba   (Ill)
//	cat + box, arrow out   -> Honnan?  -> a dobozból  (Ela)
//
// The picture is a CUE, never the authority. The right form is an attested form
// of the place noun in the case that (relation, motion) names, found through
// morph.FormsOf, and a place or case whose form is missing or has more than one
// attested string is skipped — nothing is ever generated. Ade/All/Abl are left
// out (postpositions and nominative-versus-case trouble).

// Picture is what the client draws and reads aloud. Place and Object are
// catalogue ids (GET /api/pictures/:id.svg); Relation is "in" or "on"; Motion
// is "static", "into" or "out". The sentences are composed here so a client
// that wants no template of its own still has an accessible description; the
// component alts let it build a localised one.
type Picture struct {
	Place       string `json:"place"`
	Object      string `json:"object"`
	Relation    string `json:"relation"`
	Motion      string `json:"motion"`
	AltRu       string `json:"altRu"`
	AltEn       string `json:"altEn"`
	PlaceAltRu  string `json:"placeAltRu"`
	PlaceAltEn  string `json:"placeAltEn"`
	ObjectAltRu string `json:"objectAltRu"`
	ObjectAltEn string `json:"objectAltEn"`
}

// sceneSpec names one cell of the relation x motion grid.
type sceneSpec struct{ relation, motion, question string }

// sceneCases maps a pack case to what a picture shows for it. The six cases are
// the whole of the scene drill.
var sceneCases = map[string]sceneSpec{
	"Ine": {"in", "static", "hol"}, "Ill": {"in", "into", "hova"}, "Ela": {"in", "out", "honnan"},
	"Sup": {"on", "static", "hol"}, "Sbl": {"on", "into", "hova"}, "Del": {"on", "out", "honnan"},
}

// sceneTriples are the three cases of one relation, in question order.
var sceneTriples = map[string][3]string{
	"in": {"Ine", "Ill", "Ela"},
	"on": {"Sup", "Sbl", "Del"},
}

// sceneCaseFor is the inverse of sceneCases, used by tests and the grader.
func sceneCaseFor(relation, motion string) string {
	for c, s := range sceneCases {
		if s.relation == relation && s.motion == motion {
			return c
		}
	}
	return ""
}

type sceneState struct {
	once    sync.Once
	places  []pictures.Resolved
	objects []pictures.Resolved
	dropped []string
	total   int
	// fits maps a place id to the objects small enough to go in or on it.
	fits map[string][]pictures.Resolved
}

// sceneCatalogue resolves the picture catalogue against the pack once. The
// counts are logged; the dropped lemmas are the catalogue's spelling problems.
func (g *Generator) sceneCatalogue() *sceneState {
	st := &g.scene
	st.once.Do(func() {
		cat, err := pictures.Default()
		if err != nil || g.pack == nil {
			slog.Warn("pictures: catalogue unavailable", "error", err)
			return
		}
		all := cat.All()
		st.total = len(all)
		resolved := cat.Resolve(func(lemma, upos string) (int64, bool) {
			var id int64
			err := g.pack.DB().QueryRow(`
				SELECT id FROM lemma WHERE lemma = ? AND upos = ?
				 ORDER BY freq_rank IS NULL, freq_rank LIMIT 1`, lemma, upos).Scan(&id)
			return id, err == nil
		})
		got := map[string]bool{}
		for _, r := range resolved {
			got[r.ID] = true
			if r.Role == pictures.RolePlace {
				st.places = append(st.places, r)
			} else {
				st.objects = append(st.objects, r)
			}
		}
		for _, e := range all {
			if !got[e.ID] {
				st.dropped = append(st.dropped, e.Lemma)
			}
		}
		sort.Strings(st.dropped)
		st.fits = map[string][]pictures.Resolved{}
		usable := map[string]int{}
		for _, p := range st.places {
			for _, o := range st.objects {
				if o.LemmaID != p.LemmaID && pictures.FitsEntry(o.Entry, p.Entry) {
					st.fits[p.ID] = append(st.fits[p.ID], o)
				}
			}
			usable[p.ID] = len(st.fits[p.ID])
		}
		slog.Info("pictures: usable objects per place", "counts", usable)
		slog.Info("pictures: catalogue resolved against the pack",
			"resolved", len(resolved), "dropped", len(st.dropped), "of", st.total,
			"places", len(st.places), "objects", len(st.objects))
	})
	return st
}

// DroppedPictureLemmas lists the catalogue lemmas the loaded pack does not
// know, for the operator to fix.
func (g *Generator) DroppedPictureLemmas() []string {
	if g.load() != nil {
		return nil
	}
	return append([]string(nil), g.sceneCatalogue().dropped...)
}

// sceneTopicCases are the topic's own scene cases.
func (g *Generator) sceneTopicCases(t curriculum.Topic) []Feature {
	var out []Feature
	for _, id := range t.FeatureIDs {
		if f, ok := g.features[id]; ok && f.Key == "Case" {
			if _, ok := sceneCases[f.Value]; ok {
				out = append(out, f)
			}
		}
	}
	return out
}

// sceneTopic reports whether a topic carries at least two of the six cases.
func (g *Generator) sceneTopic(t curriculum.Topic) bool {
	return len(g.sceneTopicCases(t)) >= 2
}

// sceneForms returns the three attested place forms of one relation, keyed by
// case, or false when any is missing, ambiguous or not pairwise distinct.
func (g *Generator) sceneForms(placeLemmaID int64, relation string) (map[string]string, bool) {
	out := map[string]string{}
	seen := map[string]bool{}
	for _, c := range sceneTriples[relation] {
		f, ok := g.features["Case="+c]
		if !ok {
			return nil, false
		}
		form, ok := g.uniqueCell(placeLemmaID, nominalSpec(f))
		if !ok || seen[strings.ToLower(form.Form)] {
			return nil, false
		}
		seen[strings.ToLower(form.Form)] = true
		out[c] = form.Form
	}
	return out, true
}

func (g *Generator) sceneItem(req Request) (Item, bool) {
	cases := g.sceneTopicCases(req.Topic)
	st := g.sceneCatalogue()
	if len(cases) < 2 || len(st.places) == 0 || len(st.objects) == 0 {
		return Item{}, false
	}
	for range maxCandidateTries {
		f := g.weightedFeature(cases, req)
		spec := sceneCases[f.Value]

		var places []pictures.Resolved
		for _, p := range st.places {
			// Relations names the STATIC case of a relation (in -> Ine); a place
			// that has the relation has all three of its cases.
			if _, has := p.Relations[spec.relation]; has {
				places = append(places, p)
			}
		}
		if len(places) == 0 {
			continue
		}
		place := places[req.Rand.Intn(len(places))]
		forms, ok := g.sceneForms(place.LemmaID, spec.relation)
		if !ok {
			continue
		}
		obj, ok := g.sceneObject(req, place, f.Value, spec.motion)
		if !ok {
			continue
		}

		var opts []Option
		for _, c := range sceneTriples[spec.relation] {
			opts = append(opts, Option{Text: forms[c], Attested: true})
		}
		req.Rand.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })
		ru, en := g.gloss(place.LemmaID)
		return Item{
			TopicCode: req.Topic.Code, Kind: KindScene, Phase: req.Phase,
			PromptKey:  "drill.prompt.scene",
			PromptArgs: map[string]string{"question": spec.question, "place": place.Lemma},
			Lemma:      place.Lemma, LemmaID: place.LemmaID, GlossRU: ru, GlossEN: en,
			Options: opts, FeatureIDs: []string{f.ID}, Headline: f.ID, Attested: true,
			Picture:  scenePicture(place, obj, spec),
			expected: forms[f.Value], sceneObjectID: obj.LemmaID, sceneForms: invert(forms),
		}, true
	}
	return Item{}, false
}

// sceneObject picks an object other than the place, preferring one this
// learner has not seen with this place and case lately (the key is
// scene|place|object|case|motion).
func (g *Generator) sceneObject(req Request, place pictures.Resolved, caseValue, motion string) (pictures.Resolved, bool) {
	// Only objects that physically fit: a cup holds a key, never a taxi.
	objs := g.sceneCatalogue().fits[place.ID]
	if len(objs) == 0 {
		return pictures.Resolved{}, false
	}
	var pick pictures.Resolved
	found := false
	for range 8 {
		o := objs[req.Rand.Intn(len(objs))]
		if o.LemmaID == place.LemmaID || o.Lemma == place.Lemma {
			continue
		}
		pick, found = o, true
		probe := Item{Kind: KindScene, LemmaID: place.LemmaID, sceneObjectID: o.LemmaID,
			Headline: "Case=" + caseValue, sceneMotion: motion}
		if _, seen := req.Recent.keyRank(probe); !seen {
			break
		}
	}
	return pick, found
}

func invert(m map[string]string) map[string]string {
	out := map[string]string{}
	for c, form := range m {
		out[strings.ToLower(form)] = c
	}
	return out
}

func scenePicture(place, obj pictures.Resolved, spec sceneSpec) *Picture {
	p := &Picture{
		Place: place.ID, Object: obj.ID, Relation: spec.relation, Motion: spec.motion,
		PlaceAltRu: place.AltRu, PlaceAltEn: place.AltEn, ObjectAltRu: obj.AltRu, ObjectAltEn: obj.AltEn,
	}
	enRel := map[string]map[string]string{
		"in": {"static": "in", "into": "going into", "out": "coming out of"},
		"on": {"static": "on", "into": "going onto", "out": "coming off"},
	}[spec.relation][spec.motion]
	ruRel := map[string]map[string]string{
		"in": {"static": "внутри", "into": "движется внутрь", "out": "движется наружу из"},
		"on": {"static": "сверху на", "into": "движется на", "out": "движется с"},
	}[spec.relation][spec.motion]
	p.AltEn = obj.AltEn + " " + enRel + " the " + place.AltEn
	p.AltRu = obj.AltRu + ": " + ruRel + " (" + place.AltRu + ")"
	return p
}

// classifyScene says what a wrong form means: the question word of the chosen
// case against the expected one, as for qword.
func classifyScene(item Item, chosen string) morph.ErrorClass {
	chosenCase := item.sceneForms[strings.ToLower(strings.TrimSpace(chosen))]
	if chosenCase == "" {
		return morph.ErrUnknown
	}
	exp := strings.TrimPrefix(item.Headline, "Case=")
	return classifyQword(sceneCases[exp].question, sceneCases[chosenCase].question)
}

// sceneKey is the anti-repeat key of a scene item.
func sceneKey(i Item) string {
	motion := i.sceneMotion
	if i.Picture != nil {
		motion = i.Picture.Motion
	}
	return "scene|" + strconv.FormatInt(i.LemmaID, 10) + "|" + strconv.FormatInt(i.sceneObjectID, 10) +
		"|" + strings.TrimPrefix(i.Headline, "Case=") + "|" + motion
}

// Warm precomputes what the first request per topic would otherwise pay for —
// the vocabulary, the availability probes, the person and match caches and the
// picture catalogue — so the first /api/drill/next is not the slow one. It
// returns early when ctx is cancelled and is never started implicitly: the
// server starts it after boot, tests do not.
func (g *Generator) Warm(ctx context.Context) {
	start := time.Now()
	if ctx.Err() != nil || g.load() != nil {
		return
	}
	g.sceneCatalogue()
	n := 0
	for _, t := range g.dag.Topics() {
		if ctx.Err() != nil {
			return
		}
		av := g.Availability(t)
		if !av.Available {
			continue
		}
		n++
		if g.personTopic(t) {
			vf := g.personFeatures(t)
			allowDef := t.TeachOrder >= g.definiteFloor()
			if g.personSentenceTotal(t, vf, allowDef) < bareThreshold && ctx.Err() == nil {
				g.bareRefs(t, vf)
			}
		}
		for _, k := range []Kind{KindMatch, KindScene} {
			if contains(av.Kinds, k) {
				for i := range 3 {
					g.build(k, Request{Topic: t, Phase: Blocked, Count: 1, Rand: newProbeRandSeed(int64(i))})
				}
			}
		}
	}
	slog.Info("drill warm-up finished", "topics", n, "ms", time.Since(start).Milliseconds())
}
