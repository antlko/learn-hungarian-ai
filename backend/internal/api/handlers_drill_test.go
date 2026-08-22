package api

import (
	"net/http"
	"testing"

	"tanulom/internal/drill"
)

// End-to-end tests for the grammar loop. What they are really checking is that
// the gate a learner sees on screen is the gate the server applied — that the
// phase, the reason and the numbers all come from one place.

func topicByCode(topics []TopicDTO, code string) (TopicDTO, bool) {
	for _, t := range topics {
		if t.Code == code {
			return t, true
		}
	}
	return TopicDTO{}, false
}

func TestCurriculumStartsAtTheFirstDrillableTopic(t *testing.T) {
	_, c := newPackedServer(t)

	var got CurriculumResponse
	if resp := c.json(http.MethodGet, "/api/curriculum", nil, &got); resp.StatusCode != 200 {
		t.Fatalf("curriculum: %d", resp.StatusCode)
	}
	if len(got.Topics) != 5 {
		t.Fatalf("want 5 topics, got %d", len(got.Topics))
	}

	// The alphabet has no drills, so it is `explained` — and crucially it does
	// NOT wall off everything downstream.
	ortho, _ := topicByCode(got.Topics, "orthography")
	if ortho.Phase != string(drill.Explained) {
		t.Errorf("orthography phase = %q, want explained", ortho.Phase)
	}
	if ortho.Drills.Available {
		t.Error("orthography reports drills it does not have")
	}

	acc, _ := topicByCode(got.Topics, "accusative")
	if acc.Phase != string(drill.Blocked) {
		t.Errorf("accusative phase = %q, want blocked — a new topic always starts blocked", acc.Phase)
	}
	if acc.Decision.Reason != string(drill.ReasonWarmup) {
		t.Errorf("reason = %q, want the warm-up reason", acc.Decision.Reason)
	}

	// Downstream topics are locked, and say which topic is in the way.
	loc, _ := topicByCode(got.Topics, "locative-grid")
	if loc.Phase != string(drill.Locked) {
		t.Errorf("locative-grid phase = %q, want locked", loc.Phase)
	}
	if loc.Decision.Args["topic"] != "accusative" {
		t.Errorf("locked reason must name the blocker, got %v", loc.Decision.Args)
	}

	// [R15]: word order carries an extra listening gate on top of its
	// prerequisites. The nearer blocker is reported first — naming a topic the
	// learner can actually work on is more useful than naming one they cannot —
	// so here it is still the prerequisite. The listening gate itself is
	// exercised in internal/drill/schedule_test.go.
	focus, _ := topicByCode(got.Topics, "focus-word-order")
	if focus.Phase != string(drill.Locked) {
		t.Errorf("focus-word-order phase = %q, want locked", focus.Phase)
	}

	if got.Current != "accusative" {
		t.Errorf("current topic = %q, want accusative", got.Current)
	}
	if got.Gate.Threshold != 0.80 || got.Gate.Window != 20 {
		t.Errorf("the gate config must be echoed for the UI, got %+v", got.Gate)
	}
}

// [R4]: the rule card comes first and is a required field. A topic detail
// without both locales' explanations is an incomplete topic.
func TestTopicDetailCarriesTheRuleCardInBothLocales(t *testing.T) {
	_, c := newPackedServer(t)

	var got TopicDetailResponse
	if resp := c.json(http.MethodGet, "/api/curriculum/locative-grid", nil, &got); resp.StatusCode != 200 {
		t.Fatalf("topic: %d", resp.StatusCode)
	}
	if got.ExplainRU == "" || got.ExplainEN == "" {
		t.Error("both explanations are required — see ADR-004")
	}
	if len(got.Features) != 3 {
		t.Fatalf("want the three locative features, got %d", len(got.Features))
	}
	for _, f := range got.Features {
		if f.Seen {
			t.Errorf("%s: reported as seen before any answer", f.ID)
		}
		if f.Grid == "" {
			t.Errorf("%s: the grid cell is what the 3x3 table renders from", f.ID)
		}
	}
	if len(got.Blocking) == 0 {
		t.Error("a locked topic must name what is blocking it")
	}

	resp, _ := c.do(http.MethodGet, "/api/curriculum/no-such-topic", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown topic: want 404, got %d", resp.StatusCode)
	}
}

// The whole loop: fetch items, answer them, watch the gate move — and never see
// the answer in the payload.
func TestDrillLoopAdvancesTheGate(t *testing.T) {
	_, c := newPackedServer(t)

	var batch DrillNextResponse
	if resp := c.json(http.MethodGet, "/api/drill/next?topic=accusative&count=5", nil, &batch); resp.StatusCode != 200 {
		t.Fatalf("drill/next: %d", resp.StatusCode)
	}
	if len(batch.Items) == 0 {
		t.Fatalf("no items served (%s)", batch.Unavailable)
	}
	for _, it := range batch.Items {
		if it.ID == "" {
			t.Error("an item with no id cannot be answered")
		}
		if it.Expected() != "" {
			t.Error("the expected answer must never leave the server")
		}
		if len(it.FeatureIDs) == 0 {
			t.Errorf("%s: an item that grades nothing should not exist", it.Kind)
		}
	}

	// Answer wrongly first: the response must say so, and grade the feature.
	var wrong DrillAnswerResponse
	resp := c.json(http.MethodPost, "/api/drill/answer", DrillAnswerRequest{
		ItemID: batch.Items[0].ID, Answer: "definitely-not-hungarian", DurationMs: 3000,
	}, &wrong)
	if resp.StatusCode != 200 {
		t.Fatalf("drill/answer: %d", resp.StatusCode)
	}
	if wrong.Correct {
		t.Error("a nonsense answer was graded correct")
	}
	if wrong.Expected == "" {
		t.Error("the expected answer must be revealed AFTER the attempt")
	}
	if len(wrong.GradedFeatures) == 0 {
		t.Error("a drill answer must reach the morphological state space")
	}
	for _, g := range wrong.GradedFeatures {
		if g.Correct {
			t.Errorf("%s graded correct on a wrong answer", g.FeatureID)
		}
	}
	if wrong.Topic.Progress.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", wrong.Topic.Progress.Attempts)
	}

	// An item can be answered only once.
	resp, _ = c.do(http.MethodPost, "/api/drill/answer", DrillAnswerRequest{
		ItemID: batch.Items[0].ID, Answer: "x",
	})
	if resp.StatusCode != http.StatusGone {
		t.Errorf("replaying an item: want 410, got %d", resp.StatusCode)
	}

	// Now grind correct answers until the gate opens. Twenty-five is the window
	// plus a margin; the exact count is the gate's business, not the test's.
	phase := ""
	for range 40 {
		var next DrillNextResponse
		c.json(http.MethodGet, "/api/drill/next?topic=accusative&count=1", nil, &next)
		if len(next.Items) == 0 {
			t.Fatalf("ran out of items: %s", next.Unavailable)
		}
		item := next.Items[0]

		// The client never sees the answer, so the test reads it out of the
		// server's own pending store — the same place the answer handler looks.
		id, expected := peekAnswer(t, c.app, item.ID)

		var res DrillAnswerResponse
		c.json(http.MethodPost, "/api/drill/answer", DrillAnswerRequest{
			ItemID: id, Answer: expected, DurationMs: 900,
		}, &res)
		if !res.Correct {
			t.Fatalf("answering %q with the attested form was graded wrong", expected)
		}
		phase = res.Topic.Phase
		if phase == string(drill.Interleaved) {
			break
		}
	}
	if phase != string(drill.Interleaved) {
		t.Fatalf("the gate never opened; phase = %q", phase)
	}

	// And the mastery heatmap has real evidence in it now.
	var mastery MasteryResponse
	c.json(http.MethodGet, "/api/stats/mastery", nil, &mastery)
	if len(mastery.Features) == 0 {
		t.Error("drill answers must show up in the feature mastery map")
	}
}

// peekAnswer reads an item's expected answer out of the server's pending store
// and returns the id to answer it under.
//
// Taking an item consumes it, so it is put back — which mints a fresh id, hence
// the two return values. Tests need the answer; clients never get it, which is
// exactly the property being relied on here.
func peekAnswer(t *testing.T, s *Server, itemID string) (string, string) {
	t.Helper()
	item, ok := s.pending.Take(1, itemID)
	if !ok {
		t.Fatalf("item %s is not pending", itemID)
	}
	restored, err := s.pending.Put(1, []drill.Item{item})
	if err != nil {
		t.Fatal(err)
	}
	return restored[0].ID, item.Expected()
}

func TestLockedTopicServesNoItemsButExplainsWhy(t *testing.T) {
	_, c := newPackedServer(t)

	var batch DrillNextResponse
	if resp := c.json(http.MethodGet, "/api/drill/next?topic=locative-grid", nil, &batch); resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if len(batch.Items) != 0 {
		t.Error("a locked topic must not serve items")
	}
	if batch.Unavailable != string(drill.ReasonPrereq) {
		t.Errorf("unavailable = %q, want the prerequisite reason", batch.Unavailable)
	}

	// An explanation-only topic is a different, equally honest message.
	c.json(http.MethodGet, "/api/drill/next?topic=orthography", nil, &batch)
	if batch.Unavailable != string(drill.ReasonExplainedOnly) &&
		batch.Unavailable != string(drill.ReasonNoGenerator) {
		t.Errorf("unavailable = %q, want an explanation-only reason", batch.Unavailable)
	}

	resp, _ := c.do(http.MethodGet, "/api/drill/next?topic=nope", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown topic: want 404, got %d", resp.StatusCode)
	}
}

// The out-of-the-box promise: a server started against an EMPTY data directory
// installs the embedded starter pack and every language feature works. The old
// version of this test asserted the opposite — 503 everywhere — which is the
// behaviour that made a fresh install useless until the user solved a
// distribution problem by hand.
func TestFreshInstallHasAWorkingPack(t *testing.T) {
	s, err := New(t.TempDir()) // no writeTestPack: nothing but an empty dir
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	c := &client{t: t, app: s}
	if resp, body := c.do(http.MethodPost, "/api/setup", SetupRequest{
		Email: "anna@example.com", Password: goodPassword, UILocale: "ru", GlossLang: "both",
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: %d %s", resp.StatusCode, body)
	}

	var boot BootstrapDTO
	c.json(http.MethodGet, "/api/bootstrap", nil, &boot)
	if !boot.PackPresent {
		t.Fatal("a fresh install must report a language pack")
	}

	var curr CurriculumResponse
	if resp := c.json(http.MethodGet, "/api/curriculum", nil, &curr); resp.StatusCode != 200 {
		t.Fatalf("curriculum on a fresh install: %d", resp.StatusCode)
	}
	if len(curr.Topics) == 0 {
		t.Fatal("the starter pack must carry the curriculum")
	}
	drillable := 0
	for _, topic := range curr.Topics {
		if topic.Drills.Available {
			drillable++
		}
	}
	if drillable == 0 {
		t.Error("no topic can produce drills from the starter pack")
	}

	var batch DrillNextResponse
	if resp := c.json(http.MethodGet, "/api/drill/next?count=3", nil, &batch); resp.StatusCode != 200 {
		t.Fatalf("drill/next: %d", resp.StatusCode)
	}
	if len(batch.Items) == 0 {
		t.Fatalf("a fresh install served no drill items (%s)", batch.Unavailable)
	}
}

func TestPlacementLadderAndSeeding(t *testing.T) {
	s, c := newPackedServer(t)

	var pl PlacementResponse
	if resp := c.json(http.MethodGet, "/api/placement", nil, &pl); resp.StatusCode != 200 {
		t.Fatalf("placement: %d", resp.StatusCode)
	}
	if pl.Done {
		t.Fatal("a fresh placement test is not done")
	}
	if pl.Topic == nil || pl.Topic.Code != "accusative" {
		t.Fatalf("first probe = %v, want the first drillable topic", pl.Topic)
	}
	if len(pl.Items) == 0 {
		t.Fatal("no probe items")
	}

	// Answer the whole ladder correctly and check that the seeds never claim
	// more than three items' worth of evidence.
	for range 30 {
		var st PlacementResponse
		c.json(http.MethodGet, "/api/placement", nil, &st)
		if st.Done {
			pl = st
			break
		}
		for _, item := range st.Items {
			id, expected := peekAnswer(t, s, item.ID)
			var res DrillAnswerResponse
			c.json(http.MethodPost, "/api/drill/answer", DrillAnswerRequest{
				ItemID: id, Answer: expected, DurationMs: 1000,
			}, &res)
		}
	}
	if !pl.Done {
		t.Fatal("the placement ladder never finished")
	}
	if len(pl.Seeds) == 0 {
		t.Fatal("a finished placement must propose seeds")
	}
	for _, seed := range pl.Seeds {
		if seed.Phase == "mastered" {
			t.Errorf("%s seeded as mastered from three items", seed.Code)
		}
		if seed.Observations > 3 {
			t.Errorf("%s seeded %d observations from a 3-item probe", seed.Code, seed.Observations)
		}
	}

	var applied PlacementActionResponse
	if resp := c.json(http.MethodPost, "/api/placement", PlacementActionRequest{Action: "apply"},
		&applied); resp.StatusCode != 200 {
		t.Fatalf("apply: %d", resp.StatusCode)
	}
	if applied.Applied == 0 {
		t.Error("applying a passed placement must move at least one topic")
	}

	var after CurriculumResponse
	c.json(http.MethodGet, "/api/curriculum", nil, &after)
	acc, _ := topicByCode(after.Topics, "accusative")
	if acc.Phase == string(drill.Locked) {
		t.Error("a topic passed at placement must not be locked")
	}

	// Reset discards the test but not the feature evidence it produced.
	var reset PlacementActionResponse
	c.json(http.MethodPost, "/api/placement", PlacementActionRequest{Action: "reset"}, &reset)
	var mastery MasteryResponse
	c.json(http.MethodGet, "/api/stats/mastery", nil, &mastery)
	if len(mastery.Features) == 0 {
		t.Error("resetting placement must not erase the retrievals it recorded")
	}

	resp, _ := c.do(http.MethodPost, "/api/placement", PlacementActionRequest{Action: "nonsense"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown action: want 400, got %d", resp.StatusCode)
	}
}
