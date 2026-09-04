package api

import (
	"math/rand"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/config"
	"tanulom/internal/curriculum"
	"tanulom/internal/drill"
	"tanulom/internal/srs"
	"tanulom/internal/stats"
)

// Curriculum and grammar-drill endpoints.
//
// The shape to notice: every topic carries not just its phase but the REASON for
// it, as an i18n key plus arguments. That is not decoration. A learner who does
// not know why the app is holding interleaving back will assume it is broken,
// and the honest answer — "we are deliberately not mixing topics until your
// accuracy reaches 80%, because mixing too early stops the rule consolidating" —
// is only possible if the gate exports its reasoning. See [R5] and docs/UX.md §8.

func (s *Server) requireCurriculum() error {
	if err := s.requirePack(); err != nil {
		return err
	}
	if s.dag == nil || s.drills == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable,
			"the language pack has no usable curriculum")
	}
	return nil
}

// gateConfig lifts the drill settings out of config.yaml.
func (s *Server) gateConfig() drill.GateConfig {
	cfg := s.Config()
	return drill.GateConfig{
		Threshold:        cfg.Drill.GateThreshold,
		Window:           cfg.Drill.GateWindow,
		MasteryThreshold: cfg.Drill.MasteryThreshold,
		MasteryWindow:    cfg.Drill.MasteryWindow,
		Hysteresis:       cfg.Drill.Hysteresis,
		HarmonyLatencyMs: cfg.Drill.HarmonyLatencyMs,
	}
}

// listeningHours is the learner's accumulated listening volume, from the daily
// ledger. Hours accumulate from reported playback — they are never claimed in
// one lump — and this figure is what unlocks the word-order topics. [R15]
//
// The single source for the whole app: every screen that shows hours and the
// gate that spends them read this one number, so they cannot disagree. See
// stats.Store.ListeningHours for why the ledger and not the raw play log.
//
// A read that fails resolves to zero rather than an error: an unproven gate
// stays shut, and a curriculum resolve must not fail because a sum did.
func (s *Server) listeningHours(userID int64) float64 {
	hours, err := stats.NewStore(s.db.DB).ListeningHours(userID)
	if err != nil {
		return 0
	}
	return hours
}

// resolveCurriculum computes every topic's phase for a learner and persists any
// phase the resolution moved.
func (s *Server) resolveCurriculum(userID int64) ([]drill.TopicState, error) {
	store := drill.NewStore(s.db.DB)
	stored, err := store.AllProgress(userID)
	if err != nil {
		return nil, err
	}
	cfg := s.Config()
	history, err := stats.NewStore(s.db.DB).Checkpoints(userID)
	if err != nil {
		return nil, err
	}
	states := drill.Resolve(s.dag, drill.ResolveInput{
		Progress:           stored,
		Config:             s.gateConfig(),
		ListeningHours:     s.listeningHours(userID),
		NeedListeningHours: float64(cfg.Curriculum.FocusUnlockListeningHours),
		BlockedLevels:      stats.BlockedLevels(history),
		Kinds:              s.drills.Availability,
	})
	if err := drill.Persist(store, userID, states, stored, time.Now()); err != nil {
		return nil, err
	}
	return states, nil
}

// TopicDTO is one topic as the UI needs it.
type TopicDTO struct {
	Code        string   `json:"code"`
	Level       string   `json:"level"`
	TitleRU     string   `json:"titleRu"`
	TitleEN     string   `json:"titleEn"`
	PrereqCodes []string `json:"prereqCodes"`
	Unlocks     []string `json:"unlocks"`
	FeatureIDs  []string `json:"featureIds"`
	TeachOrder  int      `json:"teachOrder"`
	// MotorSkill topics are mastered on latency, not accuracy alone. [R15]
	MotorSkill bool `json:"motorSkill"`

	Phase    string        `json:"phase"`
	Decision DecisionDTO   `json:"decision"`
	Progress ProgressDTO   `json:"progress"`
	Drills   DrillAvailDTO `json:"drills"`
}

// DecisionDTO is the gate's verdict, renderable in either locale.
type DecisionDTO struct {
	Phase  string            `json:"phase"`
	From   string            `json:"from"`
	Reason string            `json:"reason"`
	Args   map[string]string `json:"args,omitempty"`
	// Current and Target drive the progress bar under the phase badge.
	Current      float64 `json:"current"`
	Target       float64 `json:"target"`
	Observations int     `json:"observations"`
	Window       int     `json:"window"`
	// Ready is false while the estimate is too thin to act on — three lucky
	// answers are not evidence, and the bar says so rather than implying it.
	Ready   bool `json:"ready"`
	Demoted bool `json:"demoted"`
	// Advice names what is worth finishing first. A RECOMMENDATION: the topic
	// is open and practisable either way. Prerequisites stopped being locks
	// because a floor nobody can cross is a wall — see drill.Decision.Advice.
	Advice     string            `json:"advice,omitempty"`
	AdviceArgs map[string]string `json:"adviceArgs,omitempty"`
}

type ProgressDTO struct {
	BlockedAccuracy     float64 `json:"blockedAccuracy"`
	BlockedTotal        int     `json:"blockedTotal"`
	InterleavedAccuracy float64 `json:"interleavedAccuracy"`
	InterleavedTotal    int     `json:"interleavedTotal"`
	MedianMs            int64   `json:"medianMs"`
	Attempts            int     `json:"attempts"`
	Correct             int     `json:"correct"`
	Fallbacks           int     `json:"fallbacks"`
}

type DrillAvailDTO struct {
	Available bool     `json:"available"`
	Kinds     []string `json:"kinds"`
	Reason    string   `json:"reason,omitempty"`
}

func topicDTO(st drill.TopicState) TopicDTO {
	kinds := make([]string, 0, len(st.Availability.Kinds))
	for _, k := range st.Availability.Kinds {
		kinds = append(kinds, string(k))
	}
	return TopicDTO{
		Code: st.Topic.Code, Level: st.Topic.Level,
		TitleRU: st.Topic.TitleRU, TitleEN: st.Topic.TitleEN,
		PrereqCodes: orEmpty(st.Topic.PrereqCodes), Unlocks: orEmpty(st.Unlocks),
		FeatureIDs: orEmpty(st.Topic.FeatureIDs), TeachOrder: st.Topic.TeachOrder,
		MotorSkill: st.Topic.MotorSkill(),
		Phase:      string(st.Decision.Phase),
		Decision: DecisionDTO{
			Phase: string(st.Decision.Phase), From: string(st.Decision.From),
			Reason: string(st.Decision.Reason), Args: st.Decision.Args,
			Current: st.Decision.Current, Target: st.Decision.Target,
			Observations: st.Decision.Observations, Window: st.Decision.Window,
			Ready: st.Decision.Ready, Demoted: st.Decision.Demoted,
			Advice: string(st.Decision.Advice), AdviceArgs: st.Decision.AdviceArgs,
		},
		Progress: ProgressDTO{
			BlockedAccuracy: st.Progress.BlockedAccuracy, BlockedTotal: st.Progress.BlockedTotal,
			InterleavedAccuracy: st.Progress.InterleavedAccuracy,
			InterleavedTotal:    st.Progress.InterleavedTotal,
			MedianMs:            st.Progress.MedianMs, Attempts: st.Progress.Attempts,
			Correct: st.Progress.Correct, Fallbacks: st.Progress.Fallbacks,
		},
		Drills: DrillAvailDTO{
			Available: st.Availability.Available, Kinds: kinds,
			Reason: string(st.Availability.Reason),
		},
	}
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// CurriculumResponse is the whole DAG plus where the learner stands in it.
type CurriculumResponse struct {
	Topics []TopicDTO `json:"topics"`
	// Current is the topic the app would work on next.
	Current string `json:"current,omitempty"`
	// Placement reports whether the optional placement test has been taken.
	PlacementTaken bool `json:"placementTaken"`
	// Thresholds are echoed so the UI renders the same numbers the gate used,
	// rather than a copy that can drift.
	Gate GateDTO `json:"gate"`
}

// GateDTO echoes the configured gate, for the "why?" explanation.
type GateDTO struct {
	Threshold        float64 `json:"threshold"`
	Window           int     `json:"window"`
	MasteryThreshold float64 `json:"masteryThreshold"`
	MasteryWindow    int     `json:"masteryWindow"`
	Hysteresis       float64 `json:"hysteresis"`
	HarmonyLatencyMs int     `json:"harmonyLatencyMs"`
}

func (s *Server) handleCurriculum(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requireCurriculum(); err != nil {
		return err
	}
	states, err := s.resolveCurriculum(u.ID)
	if err != nil {
		return err
	}
	out := CurriculumResponse{Topics: make([]TopicDTO, 0, len(states))}
	for _, st := range states {
		out.Topics = append(out.Topics, topicDTO(st))
	}
	if cur, ok := drill.Current(states); ok {
		out.Current = cur.Topic.Code
	}
	n, err := drill.NewStore(s.db.DB).PlacementAnswered(u.ID)
	if err != nil {
		return err
	}
	out.PlacementTaken = n > 0

	g := s.gateConfig()
	out.Gate = GateDTO{
		Threshold: g.Threshold, Window: g.Window,
		MasteryThreshold: g.MasteryThreshold, MasteryWindow: g.MasteryWindow,
		Hysteresis: g.Hysteresis, HarmonyLatencyMs: g.HarmonyLatencyMs,
	}
	return c.JSON(out)
}

// TopicDetailResponse is the rule card plus progress.
//
// The rule card comes FIRST and is a required field in the pack: explicit
// instruction beats exposure by a wide margin, and a topic without an
// explanation cannot be taught explicitly. [R4]
type TopicDetailResponse struct {
	Topic TopicDTO `json:"topic"`
	// ExplainRU/EN are markdown. Both, always — a topic explained in one locale
	// is an incomplete topic. See ADR-004.
	ExplainRU string `json:"explainRu"`
	ExplainEN string `json:"explainEn"`
	// Features are the atoms this topic teaches, with the learner's mastery of
	// each. This is the two-state-space thesis made visible on the grammar
	// screen: the topic is not one thing you know, it is N things.
	Features []TopicFeatureDTO `json:"features"`
	// Blocking names the prerequisite topics standing in the way, if any.
	Blocking []string `json:"blocking,omitempty"`
}

// TopicFeatureDTO is one atom with the learner's state.
type TopicFeatureDTO struct {
	ID         string            `json:"id"`
	LabelRU    string            `json:"labelRu"`
	LabelEN    string            `json:"labelEn"`
	ExplainRU  string            `json:"explainRu"`
	ExplainEN  string            `json:"explainEn"`
	Allomorphs map[string]string `json:"allomorphs"`
	Grid       string            `json:"grid,omitempty"`
	// Seen is false when the learner has never been graded on this feature; the
	// UI shows a dash rather than 0%, because "no data" is not "you failed".
	Seen           bool    `json:"seen"`
	Retrievability float64 `json:"retrievability"`
	Accuracy       float64 `json:"accuracy"`
	Observations   int     `json:"observations"`
	MedianMs       int64   `json:"medianMs"`
	Automated      bool    `json:"automated"`
}

func (s *Server) handleTopic(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requireCurriculum(); err != nil {
		return err
	}
	code := c.Params("code")
	topic, ok := s.dag.Topic(code)
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "no such topic")
	}
	states, err := s.resolveCurriculum(u.ID)
	if err != nil {
		return err
	}
	var st drill.TopicState
	for _, candidate := range states {
		if candidate.Topic.Code == code {
			st = candidate
			break
		}
	}

	out := TopicDetailResponse{
		Topic:     topicDTO(st),
		ExplainRU: topic.ExplainRU, ExplainEN: topic.ExplainEN,
		Features: []TopicFeatureDTO{},
	}
	if st.Decision.Phase == drill.Locked {
		// Name the specific prerequisites, not "prerequisites unmet". A learner
		// who is told which topic to finish can act on it.
		out.Blocking = unmetPrereqs(states, topic)
	}

	mastery, err := s.mastery(u.ID)
	if err != nil {
		return err
	}
	byID := map[string]srs.FeatureMastery{}
	for _, m := range mastery {
		byID[m.FeatureID] = m
	}

	for _, id := range topic.FeatureIDs {
		dto := TopicFeatureDTO{ID: id, Allomorphs: map[string]string{}}
		row := s.pack.DB().QueryRow(`
			SELECT label_ru, label_en, explain_ru, explain_en, allomorphs, COALESCE(grid,'')
			  FROM morph_feature WHERE id = ?`, id)
		var allo string
		if err := row.Scan(&dto.LabelRU, &dto.LabelEN, &dto.ExplainRU, &dto.ExplainEN,
			&allo, &dto.Grid); err != nil {
			continue
		}
		_ = jsonUnmarshalString(allo, &dto.Allomorphs)
		if m, ok := byID[id]; ok {
			dto.Seen = true
			dto.Retrievability, dto.Accuracy = m.Retrievability, m.Accuracy
			dto.Observations, dto.MedianMs, dto.Automated = m.Observations, m.MedianMs, m.Automated
		}
		out.Features = append(out.Features, dto)
	}
	return c.JSON(out)
}

// drillRequest assembles everything the generator needs about this learner.
func (s *Server) drillRequest(userID int64, topic curriculum.Topic, phase drill.Phase,
	count int, cfg config.Config, states []drill.TopicState) (drill.Request, error) {
	store := drill.NewStore(s.db.DB)

	known, err := srs.NewStore(s.db.DB).KnownLemmaIDs(userID)
	if err != nil {
		return drill.Request{}, err
	}
	knownIDs := make(map[int64]bool, len(known))
	for id, status := range known {
		if status != "ignored" {
			knownIDs[id] = true
		}
	}

	// Recent mistakes steer the mix. A month is long enough to catch a real
	// weakness and short enough that a fixed one stops being over-drilled.
	errs, err := store.RecentErrors(userID, time.Now().AddDate(0, 0, -30), 12)
	if err != nil {
		return drill.Request{}, err
	}
	targets := drill.FollowUp(errs, s.dag, drill.UnlockedTopics(states), 6)

	return drill.Request{
		Topic: topic, Phase: phase, Count: count,
		KnownLemmas: knownIDs, WeakFeatures: drill.WeakFeatures(targets),
		Definiteness: drill.Weights{
			DefiniteObjectPct: cfg.Drill.Definiteness.DefiniteObjectPct,
			DemonstrativePct:  cfg.Drill.Definiteness.DemonstrativePct,
		},
		Rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

// unmetPrereqs lists the prerequisite topics that have not yet opened.
func unmetPrereqs(states []drill.TopicState, topic curriculum.Topic) []string {
	phases := make(map[string]drill.Phase, len(states))
	for _, st := range states {
		phases[st.Topic.Code] = st.Decision.Phase
	}
	var out []string
	for _, code := range topic.PrereqCodes {
		if !phases[code].Opens() {
			out = append(out, code)
		}
	}
	return out
}
