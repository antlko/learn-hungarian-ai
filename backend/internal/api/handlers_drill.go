package api

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/curriculum"
	"tanulom/internal/drill"
	"tanulom/internal/srs"
)

// Drill and placement endpoints.
//
// One invariant runs through both: the correct answer never leaves the server.
// Items are held in memory between being served and being answered, and the
// client posts back an id. A drill screen that ships the answer in its JSON is
// not a retrieval test, and a client that could name the features it graded
// could corrupt the mastery heatmap — the one number in this app that has to be
// trustworthy. See internal/drill/pending.go.

const (
	defaultDrillCount = 10
	maxDrillCount     = 30
)

// DrillNextResponse is a batch of items plus the gate state they were served under.
type DrillNextResponse struct {
	Topic TopicDTO     `json:"topic"`
	Items []drill.Item `json:"items"`
	// FollowUp is what recent mistakes say to work on, including the honest
	// cases a grammar drill cannot fix — a vowel-length error is an ear problem
	// and the app says so instead of drilling typing. See from_error.go.
	FollowUp []drill.Target `json:"followUp,omitempty"`
	// Unavailable explains an empty batch rather than showing a blank screen.
	Unavailable string `json:"unavailable,omitempty"`
}

func (s *Server) handleDrillNext(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requireCurriculum(); err != nil {
		return err
	}
	cfg := s.Config()

	states, err := s.resolveCurriculum(u.ID)
	if err != nil {
		return err
	}

	var state drill.TopicState
	if code := c.Query("topic"); code != "" {
		found := false
		for _, st := range states {
			if st.Topic.Code == code {
				state, found = st, true
				break
			}
		}
		if !found {
			return fiber.NewError(fiber.StatusNotFound, "no such topic")
		}
	} else {
		var ok bool
		state, ok = drill.Current(states)
		if !ok {
			return fiber.NewError(fiber.StatusServiceUnavailable,
				"no topic is open for practice yet")
		}
	}

	out := DrillNextResponse{Topic: topicDTO(state), Items: []drill.Item{}}

	// A locked topic is not an error — the learner may legitimately open it to
	// read the rule card. It simply serves no items, and says why.
	if !state.Decision.Phase.Servable() {
		out.Unavailable = string(state.Decision.Reason)
		return c.JSON(out)
	}
	if !state.Availability.Available {
		out.Unavailable = string(state.Availability.Reason)
		return c.JSON(out)
	}

	count := defaultDrillCount
	if raw := c.Query("count"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxDrillCount {
			return badRequest("count must be between 1 and %d", maxDrillCount)
		}
		count = n
	}

	req, err := s.drillRequest(u.ID, state.Topic, state.Decision.Phase, count, cfg, states)
	if err != nil {
		return err
	}
	items, avail, err := s.drills.Next(req)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		out.Unavailable = string(avail.Reason)
		return c.JSON(out)
	}

	served, err := s.pending.Put(u.ID, items)
	if err != nil {
		return err
	}
	out.Items = served

	store := drill.NewStore(s.db.DB)
	errs, err := store.RecentErrors(u.ID, time.Now().AddDate(0, 0, -30), 12)
	if err != nil {
		return err
	}
	out.FollowUp = drill.FollowUp(errs, s.dag, drill.UnlockedTopics(states), 4)
	return c.JSON(out)
}

// DrillAnswerRequest is one answered item. It carries no expected answer and no
// feature list: the server holds both.
type DrillAnswerRequest struct {
	ItemID     string `json:"itemId"`
	Answer     string `json:"answer"`
	UsedHint   bool   `json:"usedHint"`
	DurationMs int64  `json:"durationMs"`
}

// DrillAnswerResponse mirrors the review loop's answer: the outcome, the
// features it graded, and — new here — what the gate did about it.
type DrillAnswerResponse struct {
	drill.AnswerResult
	Topic TopicDTO `json:"topic"`
	// FeatureState is the post-answer state of the features this item graded.
	//
	// Returned because the grammar screen shows both the topic's accuracy and
	// each feature's, and for a single-feature topic they are the same
	// measurement. Leaving the feature numbers to a later refresh put 45% beside
	// 50% on screen — two figures that should agree, disagreeing, which is
	// exactly the kind of thing that makes a learner stop trusting the numbers.
	FeatureState []srs.FeatureMastery `json:"featureState"`
}

func (s *Server) handleDrillAnswer(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requireCurriculum(); err != nil {
		return err
	}
	var req DrillAnswerRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if req.ItemID == "" {
		return badRequest("itemId is required")
	}

	item, ok := s.pending.Take(u.ID, req.ItemID)
	if !ok {
		// Gone rather than not-found: the id was valid, the item has expired or
		// was already answered. The client fetches a new batch.
		return fiber.NewError(fiber.StatusGone,
			"this drill item has expired — fetch a new one")
	}

	states, err := s.resolveCurriculum(u.ID)
	if err != nil {
		return err
	}
	topic, _ := s.dag.Topic(item.TopicCode)
	gate := s.gateFor(topic, states)

	engine := drill.NewEngine(
		drill.NewStore(s.db.DB),
		s.engine(),
		s.dag,
		s.analyzer,
		s.gateConfig(),
	)
	res, err := engine.Answer(drill.AnswerInput{
		UserID: u.ID, Item: item, Answer: req.Answer,
		UsedHint: req.UsedHint, DurationMs: req.DurationMs, Gate: gate,
		AccentTolerance: s.Config().Review.AccentToleranceEnabled(),
	}, time.Now())
	if err != nil {
		return err
	}
	if res.GradedFeatures == nil {
		res.GradedFeatures = []srs.FeatureGrade{}
	}

	s.recordInteraction(u.ID, "drill", req.DurationMs)

	// The topic as it now stands: an answer that opened the gate should say so
	// on the same screen that caused it.
	//
	// Assembled from what Answer already decided rather than by resolving the
	// whole curriculum a second time. Resolving reads every topic's row off a
	// single-connection SQLite pool, twice per answer, to learn something the
	// engine just computed — and it computed it better: res.Decision was made
	// against the pre-answer phase, so it still carries "the gate cleared"
	// rather than the flatter verdict a re-read of the saved phase produces.
	out := DrillAnswerResponse{AnswerResult: res, FeatureState: []srs.FeatureMastery{}}
	out.Topic = topicDTO(drill.TopicState{
		Topic: topic, Progress: res.Progress, Decision: res.Decision,
		Availability: s.drills.Availability(topic), Unlocks: s.dag.Unlocks(item.TopicCode),
	})

	graded := map[string]bool{}
	for _, g := range res.GradedFeatures {
		graded[g.FeatureID] = true
	}
	if len(graded) > 0 {
		mastery, err := s.mastery(u.ID)
		if err != nil {
			return err
		}
		for _, m := range mastery {
			if graded[m.FeatureID] {
				out.FeatureState = append(out.FeatureState, m)
			}
		}
	}
	return c.JSON(out)
}

// gateFor rebuilds one topic's external gates from an already-resolved graph.
func (s *Server) gateFor(topic curriculum.Topic, states []drill.TopicState) drill.Gate {
	phases := make(map[string]drill.Phase, len(states))
	avail := map[string]bool{}
	for _, st := range states {
		phases[st.Topic.Code] = st.Decision.Phase
		avail[st.Topic.Code] = st.Availability.Available
	}
	g := drill.Gate{MotorSkill: topic.MotorSkill(), Drillable: avail[topic.Code]}
	for _, code := range topic.PrereqCodes {
		if !phases[code].Opens() {
			g.UnmetPrereqs = append(g.UnmetPrereqs, code)
		}
	}
	if topic.NeedsListeningVolume() {
		g.Listening = &drill.ListeningGate{
			HeardHours: 0,
			NeedHours:  float64(s.Config().Curriculum.FocusUnlockListeningHours),
		}
	}
	return g
}

// ── placement ─────────────────────────────────────────────────────────────

// PlacementResponse is the state of the optional placement test.
type PlacementResponse struct {
	Done  bool         `json:"done"`
	Topic *TopicDTO    `json:"topic,omitempty"`
	Items []drill.Item `json:"items"`
	Index int          `json:"index"`
	Total int          `json:"total"`
	// Results so far, so the learner can see the ladder they are climbing.
	Results []curriculum.TopicResult `json:"results"`
	// Seeds is the initial state the test WOULD write, shown before it is
	// applied. Placement changes where a learner starts; doing that without
	// showing them what it decided would be a silent, unexplained jump.
	Seeds []curriculum.Seed `json:"seeds,omitempty"`
}

func (s *Server) handlePlacement(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requireCurriculum(); err != nil {
		return err
	}
	cfg := s.Config()
	store := drill.NewStore(s.db.DB)

	results, err := store.PlacementResults(u.ID)
	if err != nil {
		return err
	}
	ladder := s.dag.Ladder(func(t curriculum.Topic) bool {
		return s.drills.Availability(t).Available
	})
	state := curriculum.NextProbe(ladder, results)

	out := PlacementResponse{
		Done: state.Done, Index: state.Index, Total: state.Total,
		Items: []drill.Item{}, Results: resultList(ladder, results),
	}
	if state.Done {
		out.Seeds = curriculum.SeedFrom(ladder, results)
		return c.JSON(out)
	}

	states, err := s.resolveCurriculum(u.ID)
	if err != nil {
		return err
	}
	// Probes are served as BLOCKED items: a placement probe is not interleaved
	// practice, and letting it count towards the mastery window would let three
	// lucky answers claim a mastery nobody demonstrated.
	req, err := s.drillRequest(u.ID, state.Next, drill.Blocked, state.Remaining, cfg, states)
	if err != nil {
		return err
	}
	req.Origin = "placement"
	// Placement must measure, not teach: no bias towards known vocabulary and no
	// over-sampling of weak features, or the result reflects our prompts rather
	// than the learner.
	req.KnownLemmas, req.WeakFeatures = nil, nil

	items, avail, err := s.drills.Next(req)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fiber.NewError(fiber.StatusServiceUnavailable, string(avail.Reason))
	}
	served, err := s.pending.Put(u.ID, items)
	if err != nil {
		return err
	}
	out.Items = served

	for _, st := range states {
		if st.Topic.Code == state.Next.Code {
			dto := topicDTO(st)
			out.Topic = &dto
			break
		}
	}
	return c.JSON(out)
}

func resultList(ladder []curriculum.Topic, results map[string]curriculum.TopicResult) []curriculum.TopicResult {
	out := make([]curriculum.TopicResult, 0, len(results))
	for _, t := range ladder {
		if r, ok := results[t.Code]; ok {
			out = append(out, r)
		}
	}
	return out
}

// PlacementActionRequest applies or discards a finished placement test.
type PlacementActionRequest struct {
	// Action is "apply" or "reset".
	Action string `json:"action"`
}

// PlacementActionResponse reports what changed.
type PlacementActionResponse struct {
	Applied int    `json:"applied"`
	Action  string `json:"action"`
}

func (s *Server) handlePlacementAction(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requireCurriculum(); err != nil {
		return err
	}
	var req PlacementActionRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	store := drill.NewStore(s.db.DB)

	switch req.Action {
	case "reset":
		if err := store.ClearPlacement(u.ID); err != nil {
			return err
		}
		s.pending.Forget(u.ID)
		return c.JSON(PlacementActionResponse{Action: "reset"})
	case "apply":
		results, err := store.PlacementResults(u.ID)
		if err != nil {
			return err
		}
		ladder := s.dag.Ladder(func(t curriculum.Topic) bool {
			return s.drills.Availability(t).Available
		})
		seeds := curriculum.SeedFrom(ladder, results)
		n, err := store.ApplySeeds(u.ID, seeds, s.gateConfig(), time.Now())
		if err != nil {
			return err
		}
		return c.JSON(PlacementActionResponse{Applied: n, Action: "apply"})
	}
	return badRequest("action must be apply or reset")
}

// cfgHarmonyLatency is the automaticity budget for motor skills. [R15]
func cfgHarmonyLatency(s *Server) int { return s.Config().Drill.HarmonyLatencyMs }

// mastery loads the feature heatmap with the thresholds config already defines.
//
// Every caller wants the same two: `drill.gateWindow` decides whether a rolling
// accuracy rests on enough answers to report, and `drill.masteryThreshold`
// decides whether it counts as held. Reading them here keeps the screen and the
// scheduler quoting the same numbers.
func (s *Server) mastery(userID int64) ([]srs.FeatureMastery, error) {
	cfg := s.Config()
	return s.engine().Mastery(userID, cfg.Drill.HarmonyLatencyMs, cfg.Drill.MasteryThreshold,
		time.Now())
}
