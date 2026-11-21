package drill

import (
	"path/filepath"
	"time"

	"math/rand"
	"slices"
	"strings"
	"tanulom/internal/db"
	"tanulom/internal/srs"
	"testing"
	"unicode"

	"tanulom/internal/curriculum"
)

// personYield counts the distinct (lemma, sentence) pairs the pack can turn into
// a person item for a topic — an exhaustive count, not a sample. Capped per
// lemma so the walk stays bounded.
func personYield(t *testing.T, g *Generator, topic curriculum.Topic) (items, lemmas int) {
	t.Helper()
	vf := g.personFeatures(topic)
	allowDef := topic.TeachOrder >= g.definiteFloor()
	ceiling := bandFor(topic.Level)
	for _, w := range g.verbs {
		if w.Rank > ceiling {
			break
		}
		if n := len(g.personCandidates(topic, vf, allowDef, w.LemmaID)); n > 0 {
			lemmas++
			items += n
		}
	}
	return items, lemmas
}

// The yield of the person drill on the real pack, per verb topic. The numbers
// are reported rather than asserted against a floor: a thin topic is a finding
// for the curriculum, not something to paper over with invented material.
func TestRealPackPersonYield(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	for _, topic := range dag.Topics() {
		if !g.personTopic(topic) {
			continue
		}
		items, lemmas := personYield(t, g, topic)
		avail := g.KindsFor(topic)
		t.Logf("PERSON YIELD %-20s items=%-5d verbs=%-4d offered=%v", topic.Code, items, lemmas,
			slices.Contains(avail.Kinds, KindPerson))
		if items > 0 && !slices.Contains(avail.Kinds, KindPerson) {
			t.Errorf("%s: %d items exist but person is not offered", topic.Code, items)
		}
		if items < 30 {
			t.Logf("WARNING %s: fewer than 30 distinct person items (%d)", topic.Code, items)
		}
	}
	pi, _ := dag.Topic("present-indefinite")
	if a := g.Availability(pi); !a.Available {
		t.Errorf("present-indefinite has no drills at all on the real pack: %+v", a)
	}
}

func TestRealPackPersonAndQwordItemsAreSound(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	seen := map[Kind]int{}
	for _, topic := range dag.Topics() {
		avail := g.KindsFor(topic)
		for _, k := range []Kind{KindPerson, KindQword} {
			if !slices.Contains(avail.Kinds, k) {
				continue
			}
			for seed := int64(1); seed <= 40; seed++ {
				it, ok := g.build(k, Request{Topic: topic, Phase: Blocked, Count: 1,
					Rand: rand.New(rand.NewSource(seed))})
				if !ok {
					continue
				}
				seen[k]++
				where := topic.Code + "/" + string(k)
				for _, id := range it.FeatureIDs {
					if !slices.Contains(topic.FeatureIDs, id) {
						t.Errorf("%s: grades %q, not a feature of the topic", where, id)
					}
				}
				if !it.Attested || len(it.FeatureIDs) == 0 {
					t.Errorf("%s: unattested or ungraded item", where)
				}
				switch k {
				case KindPerson:
					if !g.analyzer.IsAttested(it.Expected()) {
						t.Errorf("%s: %q is not attested", where, it.Expected())
					}
					if _, ok := g.personReading(strings.TrimSpace(it.Expected()), it.LemmaID,
						g.personFeatures(topic), true); !ok {
						t.Errorf("%s: %q no longer reads unambiguously", where, it.Expected())
					}
				case KindQword:
					focus := ""
					for _, p := range it.Parts {
						if p.Focus {
							focus = p.Text
						}
					}
					f := g.features[it.FeatureIDs[0]]
					if !g.caseReading(focus, it.LemmaID, f) {
						t.Errorf("%s: %q is not an unambiguous %s", where, focus, f.ID)
					}
				}
			}
		}
	}
	if seen[KindPerson] == 0 || seen[KindQword] == 0 {
		t.Fatalf("the real pack produced person=%d qword=%d items", seen[KindPerson], seen[KindQword])
	}
	t.Logf("built %d person and %d qword items", seen[KindPerson], seen[KindQword])
}

// Six sessions of six on present-indefinite: no frame twice in a session, no
// lemma more than twice in a session, and nothing repeated across sessions while
// the pool still has fresh items.
func TestRealPackPersonSessionsDoNotRepeat(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	topic := topicOf(t, dag, "present-indefinite")
	pool, _ := personYield(t, g, topic)

	rec := NewRecent()
	seenKey, seenFrame := map[string]bool{}, map[string]bool{}
	total := 0
	for s := range 6 {
		// The person path itself: Next would rotate onto match, a choice kind.
		items := g.fill([]Kind{KindPerson}, Request{Topic: topic, Phase: Blocked, Count: 6, Recent: rec,
			Rand: rand.New(rand.NewSource(int64(40 + s)))})
		if len(items) != 6 {
			t.Fatalf("session %d: %d items", s, len(items))
		}
		frames, lemmas := map[string]bool{}, map[int64]int{}
		for _, it := range items {
			total++
			if frames[it.Frame] {
				t.Errorf("session %d: frame %q twice", s, it.Frame)
			}
			frames[it.Frame] = true
			if lemmas[it.LemmaID]++; lemmas[it.LemmaID] > 2 {
				t.Errorf("session %d: lemma %s more than twice", s, it.Lemma)
			}
			if total <= pool && (seenKey[it.Key()] || seenFrame[it.Frame]) {
				t.Errorf("session %d: %q repeated across sessions with %d fresh items left", s, it.Frame, pool-total)
			}
			seenKey[it.Key()], seenFrame[it.Frame] = true, true
			rec.AddItem(it)
		}
	}
}

// A wrong-person answer typed against the REAL analyzer: the pack lists some
// dictionary forms with no features (`kerül`), and the engine must still call a
// right-verb-wrong-person answer what it is.
func TestRealPackPersonSlipOnRealForms(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	uid, _ := d.CreateUser("a@example.com", "h", db.RoleAdmin)
	eng := NewEngine(NewStore(d.DB), srs.NewEngine(srs.NewStore(d.DB), 0.9, 20), dag, g.analyzer, GateConfig{})

	for _, c := range []struct{ lemma, expected, typed, person string }{
		{"kerül", "kerülsz", "kerül", "2|Sing"},
		{"ül", "ülnek", "ülök", "3|Plur"},
	} {
		var id int64
		if err := g.pack.DB().QueryRow(`SELECT id FROM lemma WHERE lemma = ? AND upos = 'VERB' LIMIT 1`, c.lemma).Scan(&id); err != nil {
			t.Skipf("no verb %s in this pack", c.lemma)
		}
		item := Item{TopicCode: "present-indefinite", Kind: KindPerson, Phase: Blocked, Lemma: c.lemma, LemmaID: id,
			PromptKey: "drill.prompt.person", Frame: "___ itt.", answerPerson: c.person,
			FeatureIDs: []string{"Definite=Ind", "Tense=Pres"}, Headline: "Definite=Ind"}.WithExpected(c.expected)
		res, err := eng.Answer(AnswerInput{UserID: uid, Item: item, Answer: c.typed}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if res.Diff == nil || res.Diff.Class != ErrPerson || res.Diff.Hint != "diff.person" {
			t.Errorf("%s typed for %s: %+v, want a person error", c.typed, c.expected, res.Diff)
		}
		if len(res.GradedFeatures) != 0 {
			t.Errorf("%s: a person slip must grade nothing, got %v", c.typed, res.GradedFeatures)
		}
	}
}

// Words the pack's own glosses flag as vulgar (or that are plainly so) never
// become the headword of an exercise, tiles and qword/person/cloze included.
func TestRealPackVocabularyHasNoVulgarWords(t *testing.T) {
	t.Parallel()
	g, _ := realGenerator(t)
	loaded(t, g)
	banned := map[string]bool{"seggfej": true, "fasz": true, "kurva": true, "picsa": true, "geci": true}
	for _, pool := range [][]Word{g.nouns, g.verbs, g.adjs} {
		for _, w := range pool {
			if banned[w.Lemma] {
				t.Errorf("%q is in the drill vocabulary", w.Lemma)
			}
		}
	}
	harmless := 0
	for _, pool := range [][]Word{g.nouns, g.adjs} {
		for _, w := range pool {
			if harmlessDespiteMarkers[w.Lemma] {
				harmless++
			}
		}
	}
	t.Logf("harmless-but-marked lemmas kept in the vocabulary: %d", harmless)
	if len(g.vulgarLemmas()) == 0 {
		t.Error("no vulgar glosses found: the markers do not match this pack")
	}
}

func TestSubjunctiveLike(t *testing.T) {
	for _, c := range []struct {
		form, lemma string
		want        bool
	}{
		{"aggódj", "aggódik", true}, {"vegyek", "vesz", true}, {"tegyek", "tesz", true},
		{"adjon", "ad", true}, {"egyek", "eszik", true}, {"legyek", "van", true},
		{"vagyok", "van", false}, {"vagy", "van", false}, {"vagyunk", "van", false},
		{"látja", "lát", false}, {"látok", "lát", false}, {"fáj", "fáj", false},
		{"megyek", "megy", false}, {"kerülsz", "kerül", false},
	} {
		if got := subjunctiveLike(c.form, c.lemma); got != c.want {
			t.Errorf("subjunctiveLike(%q, %q) = %v, want %v", c.form, c.lemma, got, c.want)
		}
	}
}

// Imperatives and subjunctives the pack leaves untagged are never person items
// for a topic that does not teach them: `Értünk ne aggódj!`, `vegyek`, `tegyek`.
func TestRealPackPersonRefusesUntaggedSubjunctives(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	bad := map[string]bool{"aggódj": true, "vegyek": true, "tegyek": true}
	for _, code := range []string{"present-indefinite", "definite-conjugation", "past-tense"} {
		topic := topicOf(t, dag, code)
		vf := g.personFeatures(topic)
		allowDef := topic.TeachOrder >= g.definiteFloor()
		n := 0
		for _, w := range g.verbs {
			for _, c := range g.personCandidates(topic, vf, allowDef, w.LemmaID) {
				n++
				form := strings.ToLower(c.hu[c.start:c.end])
				if bad[form] {
					t.Errorf("%s: served %q in %q", code, form, c.hu)
				}
				// A verb straight after `ne` is an imperative.
				words := strings.FieldsFunc(strings.ToLower(c.hu), func(r rune) bool { return !unicode.IsLetter(r) })
				for i := 1; i < len(words); i++ {
					if words[i] == form && words[i-1] == "ne" {
						t.Errorf("%s: %q follows the negative particle in %q", code, form, c.hu)
					}
				}
			}
		}
		if n == 0 {
			t.Errorf("%s: no candidates at all", code)
		}
	}
}

// QA: 8 sessions of 6, each answered through the engine so the log is the only
// memory. No frame may repeat across sessions while the pool is far from empty.
func TestRealPackPersonEightSessionsViaEngine(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	topic := topicOf(t, dag, "present-indefinite")
	pool, _ := personYield(t, g, topic)
	if pool < 96 {
		t.Skipf("pool of %d is too small for this check", pool)
	}
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	uid, _ := d.CreateUser("a@example.com", "h", db.RoleAdmin)
	store := NewStore(d.DB)
	eng := NewEngine(store, srs.NewEngine(srs.NewStore(d.DB), 0.9, 20), dag, g.analyzer, GateConfig{})

	seen := map[string]int{}
	for s := range 8 {
		rec, err := store.Recent(uid, topic.Code, 300)
		if err != nil {
			t.Fatal(err)
		}
		items := g.fill([]Kind{KindPerson}, Request{Topic: topic, Phase: Blocked, Count: 6, Recent: rec,
			Rand: rand.New(rand.NewSource(int64(900 + s)))})
		if len(items) != 6 {
			t.Fatalf("session %d: %d items", s, len(items))
		}
		for _, it := range items {
			if prev, ok := seen[it.Frame]; ok {
				t.Errorf("session %d repeats the frame %q from session %d (pool %d)", s, it.Frame, prev, pool)
			}
			seen[it.Frame] = s
			if _, err := eng.Answer(AnswerInput{UserID: uid, Item: it, Answer: it.Expected()}, time.Now()); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestPoorTileMarkersAreWholeWords(t *testing.T) {
	for gloss, want := range map[string]bool{
		"henry (si unit for electrical inductance)": true, "the name of the latin script letter n/n.": true,
		"chap, dude, guy": true, "(colloquial) boyfriend": true,
		"community": false, "opportunity": false, "chapel": false, "chapter": false,
		"updated": false, "letter (a message)": false, "abbr. of x": true,
	} {
		if got := poorTileRe.MatchString(gloss); got != want {
			t.Errorf("%q: %v, want %v", gloss, got, want)
		}
	}
}

func TestRealPackTilesSkipUnitsLettersAndSlang(t *testing.T) {
	t.Parallel()
	g, _ := realGenerator(t)
	loaded(t, g)
	for _, lemma := range []string{"henry", "enn", "pasi"} {
		var id int64
		if err := g.pack.DB().QueryRow(`SELECT id FROM lemma WHERE lemma = ? AND upos = 'NOUN' LIMIT 1`, lemma).Scan(&id); err != nil {
			continue
		}
		if g.tileWorthy(Word{LemmaID: id, Lemma: lemma, UPOS: "NOUN"}, true) {
			t.Errorf("%q is a poor tile", lemma)
		}
	}
	if !g.tileWorthy(g.nouns[0], true) && !g.tileWorthy(g.nouns[1], true) && !g.tileWorthy(g.nouns[2], true) {
		t.Error("the commonest nouns should make fine tiles")
	}
}

// The same noun in the same case never appears twice in one question-word
// session while other items exist.
func TestRealPackQwordNoLemmaCaseTwiceInASession(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	for _, code := range []string{"locative-grid", "accusative"} {
		topic := topicOf(t, dag, code)
		for seed := int64(0); seed < 15; seed++ {
			items := g.fill([]Kind{KindQword}, Request{Topic: topic, Phase: Blocked, Count: 6,
				Recent: NewRecent(), Rand: rand.New(rand.NewSource(seed))})
			seen := map[string]bool{}
			for _, it := range items {
				k := it.Lemma + "|" + it.Headline
				if seen[k] {
					t.Errorf("%s seed %d: %s in %s twice in one session", code, seed, it.Lemma, it.Headline)
				}
				seen[k] = true
			}
		}
	}
}

// Sort tiles are plain headwords: no letters, abbreviations, inflected forms,
// slang or insults. Scanned over several hundred generated tiles.
func TestRealPackSortTilesAreCleanWords(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	banned := map[string]bool{
		"b": true, "amin": true, "főn": true, "jötte": true, "tudta": true, "légy": true,
		"dolcsi": true, "doki": true, "tesó": true, "meló": true, "cucc": true,
		"szemétláda": true, "idióta": true, "kretén": true, "barom": true, "szex": true, "drog": true,
		"henry": true, "enn": true, "pasi": true,
	}
	tiles := 0
	for _, code := range []string{"vowel-harmony", "locative-grid", "dative", "possessive"} {
		topic := topicOf(t, dag, code)
		for seed := int64(0); seed < 60; seed++ {
			it, ok := g.sortItem(Request{Topic: topic, Phase: Blocked, Count: 1,
				Rand: rand.New(rand.NewSource(seed))})
			if !ok {
				continue
			}
			for _, tl := range it.Tiles {
				tiles++
				if banned[tl.Text] || len([]rune(tl.Text)) < minTileLemma {
					t.Errorf("%s: %q is not a clean tile", code, tl.Text)
				}
			}
		}
	}
	if tiles < 300 {
		t.Fatalf("scanned only %d tiles", tiles)
	}
	t.Logf("scanned %d tiles", tiles)
}

// 80 generated match items from the real pack, across every topic that offers
// them: every form attested, every invariant held.
func TestRealPackMatchSweep(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	built := 0
	perTopic := map[string]int{}
	for _, topic := range dag.Topics() {
		if !slices.Contains(g.KindsFor(topic).Kinds, KindMatch) {
			continue
		}
		for seed := int64(0); seed < 40 && perTopic[topic.Code] < 16; seed++ {
			it, ok := g.matchItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: rand.New(rand.NewSource(seed))})
			if !ok {
				continue
			}
			checkMatchInvariants(t, g, topic.Code, topic.FeatureIDs, it)
			perTopic[topic.Code]++
			built++
		}
	}
	t.Logf("match items built per topic: %v", perTopic)
	if built < 80 {
		t.Fatalf("only %d match items", built)
	}
}

// Bare person items on the real pack: the conditional gains material, thick
// topics are unaffected, and no cell with two attested strings is offered.
func TestRealPackBarePerson(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	for _, code := range []string{"conditional", "imperative", "past-tense", "present-indefinite"} {
		topic := topicOf(t, dag, code)
		vf := g.personFeatures(topic)
		allowDef := topic.TeachOrder >= g.definiteFloor()
		sentences := g.personSentenceTotal(topic, vf, allowDef)
		valid := 0
		refs := g.bareRefs(topic, vf)
		for _, r := range refs {
			if g.bareValid(topic, vf, r) {
				valid++
			}
		}
		t.Logf("BARE %-20s sentenceItems>=%-4d bareCells=%-5d validBare=%d", code, sentences, len(refs), valid)

		// Session: thick topics stay sentence-only.
		rec := NewRecent()
		var items []Item
		for s := range 4 {
			batch := g.fillPerson(Request{Topic: topic, Phase: Blocked, Count: 8, Recent: rec, Rand: rand.New(rand.NewSource(int64(5 + s)))})
			for _, it := range batch {
				rec.AddItem(it)
			}
			items = append(items, batch...)
		}
		bare := 0
		for _, it := range items {
			if it.Bare {
				bare++
				if it.Frame != BlankMark || it.SentenceID != 0 || !g.analyzer.IsAttested(it.Expected()) {
					t.Errorf("%s: bad bare item %+v", code, it)
				}
				forms, _ := g.analyzer.FormsOf(it.LemmaID)
				n := map[string]bool{}
				for _, f := range forms {
					if f.Attested() && personCell(f.Feats) == it.answerPerson &&
						f.Feats["Mood"] == mapGet(g.bareRefFeats(topic, vf, it), "Mood") &&
						f.Feats["Tense"] == mapGet(g.bareRefFeats(topic, vf, it), "Tense") &&
						f.Feats["Definite"] != "Def" {
						n[strings.ToLower(f.Form)] = true
					}
				}
				if len(n) != 1 {
					t.Errorf("%s: %s %s has %d attested strings in its cell", code, it.Lemma, it.answerPerson, len(n))
				}
			}
		}
		if (code == "past-tense" || code == "present-indefinite") && bare > 0 && sentences >= bareThreshold {
			t.Errorf("%s has %d sentence items yet served %d bare", code, sentences, bare)
		}
		t.Logf("%s: 4 sessions of 8 gave %d items, %d bare", code, len(items), bare)
		if sentences < bareThreshold && valid > 0 && bare == 0 {
			t.Errorf("%s: a thin topic (%d sentence items) never topped up with bare items", code, sentences)
		}
	}
}

func mapGet(m map[string]string, k string) string { return m[k] }

// bareRefFeats finds the cell tags of a served bare item.
func (g *Generator) bareRefFeats(t curriculum.Topic, vf []Feature, it Item) map[string]string {
	for _, r := range g.bareRefs(t, vf) {
		if r.w.LemmaID == it.LemmaID && r.cell == it.cell {
			return r.feats
		}
	}
	return nil
}

// A thin topic mixes bare items into every session: conditional on the starter
// pack has about a dozen sentence items, so a sentence-first rule used to serve
// no bare item at all. Reports the bare share.
func TestRealPackThinTopicMixesBareItems(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	topic := topicOf(t, dag, "conditional")
	vf := g.personFeatures(topic)
	sentences := g.personSentenceTotal(topic, vf, topic.TeachOrder >= g.definiteFloor())
	if sentences >= bareThreshold {
		t.Skipf("conditional has %d sentence items here: not a thin topic", sentences)
	}
	rec := NewRecent()
	keys := map[string]int{}
	bare, total := 0, 0
	for s := range 10 {
		items := g.fillPerson(Request{Topic: topic, Phase: Blocked, Count: 10, Recent: rec,
			Rand: rand.New(rand.NewSource(int64(60 + s)))})
		inSession := map[string]bool{}
		sessionBare := 0
		for _, it := range items {
			total++
			if it.Bare {
				bare++
				sessionBare++
			}
			if inSession[it.Key()] {
				t.Errorf("session %d: %s twice", s, it.Key())
			}
			inSession[it.Key()] = true
			keys[it.Key()]++
			rec.AddItem(it)
		}
		if sessionBare < 3 {
			t.Errorf("session %d has only %d bare items", s, sessionBare)
		}
	}
	repeats := 0
	for _, n := range keys {
		if n > 1 {
			repeats++
		}
	}
	t.Logf("conditional (%d sentence items): %d of %d bare (%.0f%%), %d distinct keys, %d repeated",
		sentences, bare, total, 100*float64(bare)/float64(total), len(keys), repeats)
	if float64(bare)/float64(total) < 0.3 {
		t.Errorf("bare share %.2f, want at least 0.30", float64(bare)/float64(total))
	}
}

// A person session of 10 takes at most two sentences of one verb while the topic
// has enough distinct verbs to fill it. Reports the worst repeat per topic.
func TestRealPackPersonSessionsCapRepeatsPerVerb(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	for _, code := range []string{"past-tense", "present-indefinite", "definite-conjugation"} {
		topic := topicOf(t, dag, code)
		rec := NewRecent()
		worst := 0
		for s := range 10 {
			items := g.fillPerson(Request{Topic: topic, Phase: Blocked, Count: 10, Recent: rec,
				Rand: rand.New(rand.NewSource(int64(300 + s)))})
			per := map[int64]int{}
			for _, it := range items {
				per[it.LemmaID]++
				worst = max(worst, per[it.LemmaID])
				rec.AddItem(it)
			}
			if len(items) != 10 {
				t.Errorf("%s session %d: %d items", code, s, len(items))
			}
			for id, n := range per {
				if n > 2 && len(per) >= 5 {
					t.Errorf("%s session %d: lemma %d %d times", code, s, id, n)
				}
			}
		}
		t.Logf("%s: worst repeat of one verb in a session of 10: %d", code, worst)
	}
}
