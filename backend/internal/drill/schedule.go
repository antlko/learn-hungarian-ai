package drill

import (
	"time"

	"tanulom/internal/srs"
)

// The blocked -> interleaved gate.
//
// [R5] — .claude/research/05-interleaving.md — is the most consequential
// correction in this project, and this file is where it lives.
//
// The source brief (§2.3) asked for interleaved practice from the moment the
// definite conjugation is introduced, on the "desirable difficulty" argument.
// Two findings say that is exactly wrong here:
//
//   - Brunmair & Richter (2019), 59 studies: the interleaving effect is medium
//     overall (g = 0.42) and specifically ATTENUATED for explicit, rule-based and
//     language-learning tasks. The mechanism is discriminability — interleaving
//     helps when categories are hard to tell apart, and blocking can win when
//     noticing within-category regularity is what matters.
//   - Hwang (2025), Language Learning: below a threshold of prior achievement or
//     in-practice accuracy, interleaving's difficulty PREVENTS low achievers from
//     building the form-meaning associations in the first place. Initial blocked
//     practice is what builds them.
//
// So: blocked practice until a competence floor, then interleaving, and a way
// back down. Every threshold is configurable, because the study populations are
// not our learner — an advanced user may lower the floor. See config.Drill.
//
// Nothing here touches the database; the whole gate is a pure function of the
// learner's counters, which is what makes it testable and what makes the
// explanation shown in the UI provably the same rule the app applied.

// Phase is a topic's position in the gate.
type Phase string

const (
	// Locked: prerequisites unmet, or an external gate (listening volume) is closed.
	Locked Phase = "locked"
	// Explained: the topic has a rule card and no drill generator. It teaches
	// by explanation ([R4]) and deliberately does NOT gate its dependents —
	// otherwise a topic we have not built exercises for would wall off the rest
	// of the curriculum forever.
	Explained Phase = "explained"
	// Blocked: one topic, one contrast at a time. The default for a new topic.
	Blocked Phase = "blocked"
	// Interleaved: mixed with MINIMALLY CONTRASTING items — never random topics.
	Interleaved Phase = "interleaved"
	// Mastered: maintenance only. Reachable only through interleaved practice.
	Mastered Phase = "mastered"
)

// Valid reports whether p is a known phase.
func (p Phase) Valid() bool {
	switch p {
	case Locked, Explained, Blocked, Interleaved, Mastered:
		return true
	}
	return false
}

// Servable reports whether items may be generated for a topic in this phase.
// A mastered topic is still servable: mastery is maintenance, not completion.
// An explained one is not — there is nothing to serve.
func (p Phase) Servable() bool { return p == Blocked || p == Interleaved || p == Mastered }

// Opens reports whether a topic in this phase satisfies a dependent's
// prerequisite. `interleaved` is the competence floor; `explained` passes
// because there is no floor to clear.
func (p Phase) Opens() bool { return p == Explained || p == Interleaved || p == Mastered }

// GateConfig is the tunable half of the gate, from config.yaml `drill:`.
type GateConfig struct {
	Threshold        float64
	Window           int
	MasteryThreshold float64
	MasteryWindow    int
	Hysteresis       float64
	// HarmonyLatencyMs is the automaticity budget for motor-skill topics.
	HarmonyLatencyMs int
}

// Progress is a learner's stored position on one topic. Mirrors topic_progress.
type Progress struct {
	TopicCode           string    `json:"topicCode"`
	Phase               Phase     `json:"phase"`
	BlockedAccuracy     float64   `json:"blockedAccuracy"`
	BlockedTotal        int       `json:"blockedTotal"`
	InterleavedAccuracy float64   `json:"interleavedAccuracy"`
	InterleavedTotal    int       `json:"interleavedTotal"`
	MedianMs            int64     `json:"medianMs"`
	Attempts            int       `json:"attempts"`
	Correct             int       `json:"correct"`
	Fallbacks           int       `json:"fallbacks"`
	PhaseSince          time.Time `json:"phaseSince"`
}

// Gate is everything outside the topic's own counters that constrains it.
type Gate struct {
	// UnmetPrereqs names the topics standing in the way, nearest first. Empty
	// means the prerequisites are satisfied.
	//
	// A prerequisite counts as MET at `interleaved`, not at `mastered`. Requiring
	// mastery of every ancestor would serialise the curriculum into a crawl for
	// no evidential reason: R5's floor is about whether the learner can absorb
	// the load of THIS topic, and clearing the competence floor upstream is what
	// establishes that. Mastery is a maintenance state, not a gate.
	UnmetPrereqs []string
	// MotorSkill requires latency as well as accuracy for mastery. [R15]
	MotorSkill bool
	// Listening is the extra gate on focus/word order. Nil when not applicable.
	Listening *ListeningGate
	// Drillable reports whether the generator can build items for this topic.
	// False means the topic resolves to `explained` — see Phase.
	Drillable bool
	// CheckpointBlocked marks a level shut by a failed checkpoint. Outranks
	// everything except prerequisites in the explanation, because "your m3
	// assessment failed" is the actionable fact.
	CheckpointBlocked bool
}

// ListeningGate is the [R15] rule that word order cannot be reached by rules
// alone. `curriculum.focusUnlockListeningHours` defaults to 40.
//
// In v1.0 the listening loop is not built yet, so HeardHours is 0 and the topic
// stays locked. That is the honest outcome rather than a bug: the app says why,
// and says how many hours are missing.
type ListeningGate struct {
	HeardHours float64 `json:"heardHours"`
	NeedHours  float64 `json:"needHours"`
}

// Met reports whether enough listening has accumulated.
func (l ListeningGate) Met() bool { return l.HeardHours >= l.NeedHours }

// Reason is an i18n key, never prose.
//
// The backend does not pick a language — the same discipline as the answer diff.
// Both locales render these; a key without a translation in either is an
// incomplete change. See ADR-004.
type Reason string

const (
	ReasonPrereq        Reason = "gate.locked.prereq"
	ReasonListening     Reason = "gate.locked.listening"
	ReasonWarmup        Reason = "gate.blocked.warmup"
	ReasonBelowGate     Reason = "gate.blocked.accuracy"
	ReasonFellBack      Reason = "gate.blocked.fallback"
	ReasonGateCleared   Reason = "gate.interleaved.cleared"
	ReasonMasteryWarmup Reason = "gate.interleaved.warmup"
	ReasonBelowMastery  Reason = "gate.interleaved.accuracy"
	ReasonNeedsSpeed    Reason = "gate.interleaved.speed"
	ReasonMasteryLost   Reason = "gate.interleaved.lost"
	ReasonMastered      Reason = "gate.mastered.reached"
	ReasonMasteredMotor Reason = "gate.mastered.automated"
	ReasonExplainedOnly Reason = "gate.explained.ruleCardOnly"
	ReasonCheckpoint    Reason = "gate.locked.checkpoint"
)

// Decision is the gate's verdict plus everything the UI needs to explain it.
//
// The explanation is not decoration. A learner who does not know why the app is
// holding interleaving back will assume it is broken, and a phase badge with no
// reason is worse than no badge. See docs/UX.md §8.
type Decision struct {
	Phase  Phase  `json:"phase"`
	From   Phase  `json:"from"`
	Reason Reason `json:"reason"`
	// Args are substituted into the translated reason. Numbers are pre-rounded
	// here so both locales render identically.
	Args map[string]string `json:"args"`
	// Current and Target drive the progress bar: "78% of 80% (window 20)".
	Current float64 `json:"current"`
	Target  float64 `json:"target"`
	// Observations and Window make the bar honest while the estimate is thin —
	// three lucky answers are not evidence and the UI says so.
	Observations int  `json:"observations"`
	Window       int  `json:"window"`
	Ready        bool `json:"ready"`
	// Demoted marks a fall back down the ladder, which the UI announces rather
	// than silently applying.
	Demoted bool `json:"demoted"`
}

// Changed reports whether the phase moved.
func (d Decision) Changed() bool { return d.Phase != d.From }

// Evaluate applies the gate. It is a pure function: same counters, same verdict.
func Evaluate(p Progress, g Gate, cfg GateConfig) Decision {
	cfg = cfg.normalized()
	from := p.Phase
	if !from.Valid() {
		from = Locked
	}
	d := Decision{From: from, Window: cfg.Window}

	// 1. External gates first. They outrank everything: a topic whose
	//    prerequisites regressed is locked no matter how good its own numbers
	//    used to be.
	//
	//    The checkpoint block comes before even the prerequisites, because it
	//    is the deeper fact: when a level is shut by a failed assessment,
	//    "finish past-tense first" would be misleading advice — finishing it
	//    would not open anything. The правило отката names itself. [R14]
	if g.CheckpointBlocked {
		d.Phase = Locked
		d.Reason = ReasonCheckpoint
		d.Demoted = from != Locked
		return d
	}
	if len(g.UnmetPrereqs) > 0 {
		d.Phase = Locked
		d.Reason = ReasonPrereq
		d.Args = map[string]string{
			"topic":     g.UnmetPrereqs[0],
			"remaining": itoa(len(g.UnmetPrereqs)),
		}
		d.Demoted = from != Locked
		return d
	}
	if g.Listening != nil && !g.Listening.Met() {
		d.Phase = Locked
		d.Reason = ReasonListening
		d.Args = map[string]string{
			"heard": oneDecimal(g.Listening.HeardHours),
			"need":  oneDecimal(g.Listening.NeedHours),
		}
		d.Current, d.Target = g.Listening.HeardHours, g.Listening.NeedHours
		d.Demoted = from != Locked
		return d
	}

	// 2. A topic with no generator is explained, not practised. Said plainly
	//    rather than dressed up as progress: the rule card is the teaching act
	//    ([R4]), and the app does not claim to have drilled what it has not.
	if !g.Drillable {
		d.Phase = Explained
		d.Reason = ReasonExplainedOnly
		return d
	}

	floor := cfg.Threshold - cfg.Hysteresis
	gateReady := srs.GateReady(p.BlockedTotal, cfg.Window)
	masteryReady := srs.GateReady(p.InterleavedTotal, cfg.MasteryWindow)

	// 3. The deep fallback. Checked before anything else so a collapse from
	//    `mastered` lands in `blocked` rather than resting one rung down.
	//    The hysteresis band is what stops a topic oscillating on every answer.
	if from != Blocked && from != Locked && gateReady && p.BlockedAccuracy < floor {
		d.Phase = Blocked
		d.Reason = ReasonFellBack
		d.Args = map[string]string{
			"accuracy": pct(p.BlockedAccuracy),
			"floor":    pct(floor),
			"gate":     pct(cfg.Threshold),
		}
		d.Current, d.Target = p.BlockedAccuracy, cfg.Threshold
		d.Observations = p.BlockedTotal
		d.Ready = true
		d.Demoted = true
		return d
	}

	switch from {
	case Locked, Explained:
		// Prerequisites just cleared. A newly opened topic always starts blocked:
		// this is the "initial blocked practice is important" finding, and it is
		// not conditional on the learner's level.
		d.Phase = Blocked
		return blockedDecision(d, p, cfg, gateReady)

	case Blocked:
		if gateReady && p.BlockedAccuracy >= cfg.Threshold {
			d.Phase = Interleaved
			d.Reason = ReasonGateCleared
			d.Args = map[string]string{
				"accuracy": pct(p.BlockedAccuracy), "gate": pct(cfg.Threshold),
			}
			d.Current, d.Target = p.InterleavedAccuracy, cfg.MasteryThreshold
			d.Observations, d.Window = p.InterleavedTotal, cfg.MasteryWindow
			return d
		}
		d.Phase = Blocked
		return blockedDecision(d, p, cfg, gateReady)

	case Interleaved, Mastered:
		d.Phase = Interleaved
		d.Window = cfg.MasteryWindow
		d.Observations = p.InterleavedTotal
		d.Current, d.Target = p.InterleavedAccuracy, cfg.MasteryThreshold
		d.Ready = masteryReady

		if !masteryReady {
			d.Reason = ReasonMasteryWarmup
			d.Args = map[string]string{
				"answered": itoa(p.InterleavedTotal), "need": itoa(cfg.MasteryWindow),
			}
			return d
		}
		if from == Mastered && p.InterleavedAccuracy < cfg.MasteryThreshold-cfg.Hysteresis {
			d.Reason = ReasonMasteryLost
			d.Args = map[string]string{
				"accuracy": pct(p.InterleavedAccuracy),
				"mastery":  pct(cfg.MasteryThreshold),
			}
			d.Demoted = true
			return d
		}
		if p.InterleavedAccuracy < cfg.MasteryThreshold {
			// Between the mastery floor and the mastery threshold: a topic that
			// was already mastered stays mastered (that is the hysteresis band),
			// one that was not yet has not got there.
			if from == Mastered {
				d.Phase = Mastered
				d.Reason = ReasonMastered
				d.Args = map[string]string{
					"accuracy": pct(p.InterleavedAccuracy), "mastery": pct(cfg.MasteryThreshold),
				}
				return d
			}
			d.Reason = ReasonBelowMastery
			d.Args = map[string]string{
				"accuracy": pct(p.InterleavedAccuracy), "mastery": pct(cfg.MasteryThreshold),
			}
			return d
		}

		// Accurate enough. For a motor skill that is still not mastery: harmony
		// is automated when it is FAST, and a correct-but-laboured answer means
		// the practice must continue. [R15]
		if g.MotorSkill {
			auto := MeasureAutomaticity(p.MedianMs, p.InterleavedAccuracy,
				p.InterleavedTotal, cfg.MasteryWindow, cfg.HarmonyLatencyMs, cfg.MasteryThreshold)
			if !auto.Automated {
				d.Reason = ReasonNeedsSpeed
				d.Args = map[string]string{
					"median":   itoa64(p.MedianMs),
					"budget":   itoa(cfg.HarmonyLatencyMs),
					"accuracy": pct(p.InterleavedAccuracy),
				}
				return d
			}
			d.Phase = Mastered
			d.Reason = ReasonMasteredMotor
			d.Args = map[string]string{
				"median": itoa64(p.MedianMs), "budget": itoa(cfg.HarmonyLatencyMs),
			}
			return d
		}

		d.Phase = Mastered
		d.Reason = ReasonMastered
		d.Args = map[string]string{
			"accuracy": pct(p.InterleavedAccuracy), "mastery": pct(cfg.MasteryThreshold),
		}
		return d
	}

	d.Phase = Blocked
	return blockedDecision(d, p, cfg, gateReady)
}

// blockedDecision fills in why a topic is still in blocked practice: either the
// estimate is too thin to act on, or the accuracy is genuinely below the floor.
// Separating the two matters — "we do not know yet" is not "you are failing".
func blockedDecision(d Decision, p Progress, cfg GateConfig, ready bool) Decision {
	d.Current, d.Target = p.BlockedAccuracy, cfg.Threshold
	d.Observations, d.Window = p.BlockedTotal, cfg.Window
	d.Ready = ready
	if !ready {
		d.Reason = ReasonWarmup
		d.Args = map[string]string{
			"answered": itoa(p.BlockedTotal), "need": itoa(cfg.Window),
		}
		return d
	}
	d.Reason = ReasonBelowGate
	d.Args = map[string]string{
		"accuracy": pct(p.BlockedAccuracy), "gate": pct(cfg.Threshold),
	}
	return d
}

// Fold applies one answer to a topic's counters, returning the updated progress.
//
// Which window an answer lands in is decided by the phase it was SERVED in, not
// the phase the topic is in now. An answer given under blocked practice must
// never count towards the interleaved mastery window — otherwise a learner could
// reach "mastered" without ever having had to discriminate between the
// contrasting forms, which is the only thing the mastery test is testing.
func Fold(p Progress, servedIn Phase, correct bool, durationMs int64, cfg GateConfig) Progress {
	cfg = cfg.normalized()
	p.Attempts++
	if correct {
		p.Correct++
	}

	// The blocked window tracks EVERY answer, including interleaved ones: it is
	// the "can this learner still do this at all" signal that drives the
	// fallback, so excluding interleaved answers would make the fallback blind
	// exactly when it is needed.
	p.BlockedAccuracy, p.BlockedTotal =
		srs.RollingAccuracy(p.BlockedAccuracy, p.BlockedTotal, cfg.Window, correct)

	if servedIn == Interleaved || servedIn == Mastered {
		p.InterleavedAccuracy, p.InterleavedTotal =
			srs.RollingAccuracy(p.InterleavedAccuracy, p.InterleavedTotal, cfg.MasteryWindow, correct)
	}
	if correct {
		// Latency is only evidence on a correct answer: a fast wrong answer is
		// not automaticity, it is guessing.
		p.MedianMs = srs.UpdateMedian(p.MedianMs, durationMs)
	}
	return p
}

// Apply moves a topic to the phase the gate decided, keeping the bookkeeping the
// UI needs — when the phase changed and how many times it has fallen back.
func Apply(p Progress, d Decision, now time.Time) Progress {
	if d.Phase == p.Phase {
		return p
	}
	if d.Demoted {
		p.Fallbacks++
	}
	p.Phase = d.Phase
	p.PhaseSince = now
	return p
}

func (c GateConfig) normalized() GateConfig {
	if c.Threshold <= 0 {
		c.Threshold = 0.80
	}
	if c.Window < 5 {
		c.Window = 20
	}
	if c.MasteryThreshold < c.Threshold {
		c.MasteryThreshold = 0.90
	}
	if c.MasteryWindow < c.Window {
		c.MasteryWindow = 50
	}
	if c.Hysteresis < 0 || c.Hysteresis >= c.Threshold {
		c.Hysteresis = 0.10
	}
	if c.HarmonyLatencyMs <= 0 {
		c.HarmonyLatencyMs = 2500
	}
	return c
}
