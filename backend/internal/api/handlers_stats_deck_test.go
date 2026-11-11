package api

import (
	"net/http"
	"testing"
	"time"
)

// seedStateCard writes a note + one card in an FSRS state for a lemma.
func seedStateCard(t *testing.T, s *Server, lemmaID int64, state, days, reps int, trashed bool) {
	t.Helper()
	var uid int64
	if err := s.db.QueryRow(`SELECT id FROM user LIMIT 1`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	trash := any(nil)
	if trashed {
		trash = now
	}
	res, err := s.db.Exec(`INSERT INTO note (user_id, kind, lemma_id, hu, target_form, trashed_at)
		VALUES (?, 'vocab', ?, 'szó', 'szó', ?)`, uid, lemmaID, trash)
	if err != nil {
		t.Fatal(err)
	}
	nid, _ := res.LastInsertId()
	var last any
	if reps > 0 {
		last = now
	}
	if _, err := s.db.Exec(`INSERT INTO card (note_id, direction, due, stability, difficulty,
		scheduled_days, reps, state, last_review, suspended) VALUES (?,'recognize',?,?,5,?,?,?,?,0)`,
		nid, now, float64(max(days, 1)), days, reps, state, last); err != nil {
		t.Fatal(err)
	}
	s.invalidateCounted(uid)
}

func TestStatsDeckCountsLemmasByWordsListStage(t *testing.T) {
	t.Parallel()
	s, c := newBandedServer(t)
	// lemma 1 learning, 2 relearning, 3 young, 4 mature, 5 known, 6 never
	// reviewed; a seventh (trashed) word must not count.
	for id := int64(1); id <= 4; id++ {
		declare(t, s, id, "learning")
	}
	declare(t, s, 5, "known")
	declare(t, s, 6, "learning")
	seedStateCard(t, s, 1, 1, 0, 1, false)
	seedStateCard(t, s, 2, 3, 0, 4, false)
	seedStateCard(t, s, 3, 2, 5, 3, false)
	seedStateCard(t, s, 4, 2, 40, 5, false)
	seedStateCard(t, s, 6, 0, 0, 0, false)
	// A trashed duplicate of lemma 6 with a reviewed state is invisible.
	seedStateCard(t, s, 6, 2, 40, 5, true)
	// A trashed-only word is in lemma_state as learning but has no live card.
	declare(t, s, 7, "learning")
	seedStateCard(t, s, 7, 2, 40, 5, true)

	var got DeckStages
	if resp := c.json(http.MethodGet, "/api/stats/deck", nil, &got); resp.StatusCode != 200 {
		t.Fatalf("deck: %d", resp.StatusCode)
	}
	want := DeckStages{Total: 7, New: 2, Stages: DeckStageCounts{
		Learning: 1, Relearning: 1, Young: 1, Mature: 1, Known: 1}}
	// The deciles count WORDS with a reviewed card, so they sum to the four
	// reviewed stages — and never include the unreviewed `new` word or «Знаю».
	deciles := got.Progress
	got.Progress = [10]int{}
	if got != want {
		t.Fatalf("deck = %+v, want %+v", got, want)
	}
	sum := 0
	for _, n := range deciles {
		sum += n
	}
	if reviewed := want.Stages.Learning + want.Stages.Relearning + want.Stages.Young + want.Stages.Mature; sum != reviewed {
		t.Fatalf("progress deciles %v sum to %d, want %d reviewed words", deciles, sum, reviewed)
	}
	got.Progress = deciles

	// Consistency with the Words list, status=all.
	var page WordsResponse
	c.json(http.MethodGet, "/api/words?status=all&pageSize=200", nil, &page)
	if page.Total != got.Total {
		t.Fatalf("words total %d != deck total %d", page.Total, got.Total)
	}
	by := map[string]int{}
	for _, w := range page.Words {
		by[w.Stage]++
	}
	if by["learning"] != got.Stages.Learning+got.New || by["relearning"] != got.Stages.Relearning ||
		by["young"] != got.Stages.Young || by["mature"] != got.Stages.Mature ||
		by["known"] != got.Stages.Known {
		t.Fatalf("words stages %v disagree with deck %+v", by, got)
	}
}

// The Words list and the deck histogram read the same best card, so the
// deciles are the list's progress numbers bucketed.
func TestDeckProgressDecilesMatchTheWordsList(t *testing.T) {
	t.Parallel()
	s, c := newBandedServer(t)
	for id := int64(1); id <= 4; id++ {
		declare(t, s, id, "learning")
	}
	declare(t, s, 5, "known")
	declare(t, s, 6, "learning")
	seedStateCard(t, s, 1, 1, 0, 1, false)
	seedStateCard(t, s, 2, 3, 0, 4, false)
	seedStateCard(t, s, 3, 2, 5, 3, false)
	seedStateCard(t, s, 4, 2, 40, 5, false)
	seedStateCard(t, s, 6, 0, 0, 0, false)

	var deck DeckStages
	c.json(http.MethodGet, "/api/stats/deck", nil, &deck)
	var page WordsResponse
	c.json(http.MethodGet, "/api/words?status=all&pageSize=200", nil, &page)

	var want [10]int
	for _, w := range page.Words {
		if w.Stage == "known" {
			if w.Progress != nil {
				t.Errorf("known word %d carries progress %d", w.LemmaID, *w.Progress)
			}
			continue
		}
		if w.Progress == nil {
			t.Fatalf("word %d (%s) has no progress", w.LemmaID, w.Stage)
		}
		if w.Reps > 0 {
			want[min(*w.Progress/10, 9)]++
		} else if *w.Progress != 0 {
			t.Errorf("unreviewed word %d has progress %d, want 0", w.LemmaID, *w.Progress)
		}
	}
	if deck.Progress != want {
		t.Errorf("deck deciles %v, words list says %v", deck.Progress, want)
	}
}

// The word inspector's memory block: present for a word with a card, with the
// best card's figures and every card listed; omitted for a word not in the deck.
func TestLemmaDetailCarriesMemoryOnlyForWordsInTheDeck(t *testing.T) {
	t.Parallel()
	s, c := newBandedServer(t)
	declare(t, s, 4, "learning")
	seedStateCard(t, s, 4, 2, 40, 5, false)

	var in LemmaDTO
	if resp := c.json(http.MethodGet, "/api/lemma/4", nil, &in); resp.StatusCode != 200 {
		t.Fatalf("lemma 4: %d", resp.StatusCode)
	}
	if in.Memory == nil {
		t.Fatal("a word with a card has no memory block")
	}
	m := in.Memory
	if m.Stage != "mature" || m.Progress < 70 || m.Progress > 100 || m.Reps != 5 || len(m.Cards) != 1 ||
		m.Cards[0].Direction != "recognize" || m.Cards[0].Progress != m.Progress || m.History == nil {
		t.Errorf("memory = %+v", m)
	}

	var out LemmaDTO
	if resp := c.json(http.MethodGet, "/api/lemma/3", nil, &out); resp.StatusCode != 200 {
		t.Fatalf("lemma 3: %d", resp.StatusCode)
	}
	if out.Memory != nil {
		t.Errorf("a word outside the deck carries memory: %+v", out.Memory)
	}
}

func TestWordsSortByProgressLowestFirst(t *testing.T) {
	t.Parallel()
	s, c := newBandedServer(t)
	for id := int64(1); id <= 3; id++ {
		declare(t, s, id, "learning")
	}
	seedStateCard(t, s, 1, 2, 40, 5, false) // mature
	seedStateCard(t, s, 2, 1, 0, 1, false)  // learning
	seedStateCard(t, s, 3, 2, 5, 3, false)  // young
	var page WordsResponse
	c.json(http.MethodGet, "/api/words?status=all&sort=progress", nil, &page)
	last := -1
	for _, w := range page.Words {
		if w.Progress == nil || *w.Progress < last {
			t.Fatalf("not ascending by progress: %+v", page.Words)
		}
		last = *w.Progress
	}
	if len(page.Words) != 3 {
		t.Fatalf("got %d words", len(page.Words))
	}
}
