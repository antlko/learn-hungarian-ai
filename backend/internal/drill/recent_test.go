package drill

import (
	"math/rand"
	"path/filepath"
	"testing"

	"tanulom/internal/db"
)

// servedAll plays a learner who is served `sessions` batches and answers them
// all: every served item is folded into the memory the next request reads.
func servedAll(t *testing.T, g *Generator, req Request, sessions int) [][]Item {
	t.Helper()
	if req.Recent == nil {
		req.Recent = NewRecent()
	}
	var out [][]Item
	for i := range sessions {
		req.Rand = rand.New(rand.NewSource(int64(100 + i)))
		items, _, err := g.Next(req)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, items)
		for _, it := range items {
			req.Recent.AddItem(it)
		}
	}
	return out
}

// N sequential requests share no item and no sentence, while the pool allows.
func TestRecentNoRepeat(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	for _, code := range []string{"locative-grid", "accusative"} {
		topic := topicOf(t, dag, code)
		for _, phase := range []Phase{Blocked, Interleaved} {
			batches := servedAll(t, g, Request{Topic: topic, Phase: phase, Count: 10,
				Definiteness: Weights{DefiniteObjectPct: 55, DemonstrativePct: 25}}, 6)
			keys := map[string]bool{}
			sents := map[int64]bool{}
			lemmas := map[int64]int{}
			for si, items := range batches {
				if len(items) != 10 {
					t.Errorf("%s/%s session %d: %d items, want 10", code, phase, si, len(items))
				}
				for _, it := range items {
					if keys[it.Key()] {
						t.Errorf("%s/%s session %d: repeated item %s", code, phase, si, it.Key())
					}
					keys[it.Key()] = true
					if it.SentenceID != 0 {
						if sents[it.SentenceID] {
							t.Errorf("%s/%s session %d: sentence %d served twice", code, phase, si, it.SentenceID)
						}
						sents[it.SentenceID] = true
					}
					if it.LemmaID != 0 { // a sort item is six lemmas, not one
						lemmas[it.LemmaID]++
					}
				}
			}
			for id, n := range lemmas {
				if n > maxLemmaUses {
					t.Errorf("%s/%s: lemma %d used %d times in 60 items", code, phase, id, n)
				}
			}
		}
	}
}

// A pool smaller than the memory must still fill the request: relaxing is
// better than coming back short.
func TestRecentRelaxes(t *testing.T) {
	g, dag := newTestGenerator(t)
	topic := topicOf(t, dag, "locative-grid")

	// Exhaust everything the tiny fixture can build, many times over.
	req := Request{Topic: topic, Phase: Blocked, Count: 6, Recent: NewRecent()}
	for i := range 12 {
		req.Rand = rand.New(rand.NewSource(int64(i)))
		items, _, err := g.Next(req)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 6 {
			t.Fatalf("session %d came back with %d items, want 6 (recent keys %d)",
				i, len(items), len(req.Recent.Keys))
		}
		seen := map[string]bool{}
		for _, it := range items {
			if k := it.Key() + "|" + it.Frame; seen[k] {
				t.Fatalf("session %d: an item twice in one request: %s", i, k)
			} else {
				seen[k] = true
			}
			req.Recent.AddItem(it)
		}
	}
	if len(req.Recent.Keys) == 0 {
		t.Fatal("nothing was remembered")
	}
}

// When only repeats remain, the oldest goes first.
func TestRecentOldestRepeatFirst(t *testing.T) {
	r := NewRecent()
	old := Item{Kind: KindCase, LemmaID: 1, Headline: "Case=Ine"}
	newer := Item{Kind: KindCase, LemmaID: 2, Headline: "Case=Ine"}
	r.AddItem(old)
	r.AddItem(newer)
	ro, _ := r.keyRank(old)
	rn, _ := r.keyRank(newer)
	if ro <= rn {
		t.Errorf("old item rank %d must be larger (older) than the newer's %d", ro, rn)
	}
	if got := stageOf(old, r, nil, nil); got != 3 {
		t.Errorf("an exact repeat is stage 3, got %d", got)
	}
}

// The least recently used choice kind goes first, and typed kinds join only
// after enough correct blocked practice.
func TestBlockedRotation(t *testing.T) {
	avail := []Kind{KindAllomorph, KindCase, KindMCQ, KindQword, KindCloze, KindTransform, KindPerson}

	r := NewRecent()
	r.KindRank = map[Kind]int{KindCase: 0, KindMCQ: 1, KindQword: 2, KindSort: 4, KindMatch: 5, KindAllomorph: 6}
	order := blockedOrder(avail, r)
	if order[0] != KindAllomorph {
		t.Errorf("least recently used choice kind must lead, got %v", order)
	}
	for i, k := range order {
		if k.Typed() && i < 4 {
			t.Errorf("typed kind %s rotates before 10 correct blocked answers: %v", k, order)
		}
	}
	// Typed kinds are still the fallback, in preference order.
	if got := order[len(order)-3:]; got[0] != KindCloze || got[1] != KindTransform || got[2] != KindPerson {
		t.Errorf("typed fallbacks out of order: %v", order)
	}

	r.CorrectBlocked = typedUnlockCorrect
	order = blockedOrder(avail, r)
	if !order[0].Typed() {
		t.Errorf("never-used typed kinds are the stalest once unlocked, got %v", order)
	}

	// A topic with only typed kinds is not starved.
	if got := blockedOrder([]Kind{KindPerson}, NewRecent()); len(got) != 1 || got[0] != KindPerson {
		t.Errorf("a typed-only topic must still be served, got %v", got)
	}
	// No memory: the old preference order.
	if got := blockedOrder(avail, nil); got[0] != KindAllomorph || got[1] != KindCase {
		t.Errorf("without memory the order is the preference order, got %v", got)
	}
}

func TestBlockedRotationServesTheStalestKind(t *testing.T) {
	g, dag := newTestGenerator(t)
	topic := topicOf(t, dag, "locative-grid")
	r := NewRecent()
	r.KindRank = map[Kind]int{KindCase: 0, KindMCQ: 1, KindQword: 2, KindSort: 4, KindMatch: 5, KindAllomorph: 6}
	items, _, err := g.Next(Request{Topic: topic, Phase: Blocked, Count: 4, Recent: r, Rand: seededRand(3)})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.Kind != KindAllomorph {
			t.Errorf("served %s, want the least recently used choice kind allomorph", it.Kind)
		}
	}
}

// Anti-repeat filters the concrete lemma and sentence, never the feature: the
// weak-feature oversampling must look the same with and without memory.
func TestWeakStillOversampled(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	topic := topicOf(t, dag, "locative-grid")
	const weak = "Case=Ela"

	share := func(withRecent bool) float64 {
		var rec *Recent
		if withRecent {
			rec = NewRecent()
		}
		hit, total := 0, 0
		kinds := map[Kind]int{}
		for i := range 15 {
			items, _, err := g.Next(Request{
				Topic: topic, Phase: Blocked, Count: 12, Recent: rec,
				WeakFeatures: []string{weak},
				Rand:         rand.New(rand.NewSource(int64(500 + i))),
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, it := range items {
				rec.AddItem(it)
				kinds[it.Kind]++
				if it.Kind != KindCase && it.Kind != KindAllomorph && it.Kind != KindScene {
					continue // only these draw a feature through weightedFeature
				}
				total++
				if it.Headline == weak {
					hit++
				}
			}
		}
		if total < 60 {
			t.Fatalf("only %d feature-drawn items (kinds %v)", total, kinds)
		}
		return float64(hit) / float64(total)
	}
	without, with := share(false), share(true)
	t.Logf("weak share: without memory %.2f, with memory %.2f", without, with)
	if without < 0.2 {
		t.Fatalf("the weak feature is not oversampled at all (%.2f)", without)
	}
	if d := without - with; d > 0.10 || d < -0.10 {
		t.Errorf("anti-repeat changed the weak share from %.2f to %.2f", without, with)
	}
}

// The store side: the log is the memory, legacy rows are matched by prompt, and
// a kind the schema no longer polices is policed here.
func TestStoreRecent(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	uid, err := d.CreateUser("a@example.com", "h", db.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(d.DB)

	it := Item{Kind: KindPerson, TopicCode: "past-tense", PromptKey: "drill.prompt.person",
		Lemma: "lát", LemmaID: 3, Headline: "Tense=Past", SentenceID: 108, Frame: "x ___"}
	err = s.AppendAttempt(nil, uid, Attempt{TopicCode: "past-tense", Kind: KindPerson, Phase: Blocked,
		Prompt: promptOf(it), Expected: "láttál", Correct: true,
		ItemKey: it.Key(), SentenceID: it.SentenceID})
	if err != nil {
		t.Fatal(err)
	}
	// A row written before migration 00025: no key, no sentence.
	legacy := Item{Kind: KindCloze, PromptKey: "drill.prompt.cloze", Lemma: "ház", Headline: "Case=Ine", Frame: "A ___ van"}
	if err := s.AppendAttempt(nil, uid, Attempt{TopicCode: "past-tense", Kind: KindCloze, Phase: Blocked,
		Prompt: promptOf(legacy), Expected: "házban", Correct: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendAttempt(nil, uid, Attempt{TopicCode: "x", Kind: Kind("bogus"), Phase: Blocked,
		Prompt: "p", Expected: "e"}); err == nil {
		t.Error("AppendAttempt must reject an unknown kind now that the schema does not")
	}

	r, err := s.Recent(uid, "past-tense", 300)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.keyRank(it); !ok {
		t.Error("the keyed item is not remembered")
	}
	if !r.sentenceUsed(108) || r.lemmaUses(3) != 1 {
		t.Errorf("sentence/lemma memory missing: %+v", r)
	}
	if _, ok := r.keyRank(legacy); !ok {
		t.Error("a legacy row (no key) must still be matched by its prompt")
	}
	if r.kindAge(KindCloze) >= r.kindAge(KindPerson) {
		t.Errorf("cloze was logged last, so it is the more recent kind: %v", r.KindRank)
	}
	if r.CorrectBlocked != 2 {
		t.Errorf("correct blocked = %d, want 2", r.CorrectBlocked)
	}
	if other, _ := s.Recent(uid, "elsewhere", 300); len(other.Keys) != 0 {
		t.Error("memory leaked across topics")
	}
}
