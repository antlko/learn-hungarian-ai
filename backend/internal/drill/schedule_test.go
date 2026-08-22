package drill

import (
	"testing"
	"time"
)

// The gate is the single most consequential piece of logic in the project — it
// is where the evidence overrides the source brief — so it is tested transition
// by transition, including every way back down.

func testConfig() GateConfig {
	return GateConfig{
		Threshold: 0.80, Window: 20,
		MasteryThreshold: 0.90, MasteryWindow: 50,
		Hysteresis: 0.10, HarmonyLatencyMs: 2500,
	}
}

func openGate() Gate { return Gate{Drillable: true} }

func TestLockedUntilPrerequisitesOpen(t *testing.T) {
	cfg := testConfig()
	d := Evaluate(Progress{Phase: Blocked}, Gate{Drillable: true,
		UnmetPrereqs: []string{"past-tense", "possessive"}}, cfg)

	if d.Phase != Locked {
		t.Fatalf("phase = %v, want locked", d.Phase)
	}
	if d.Reason != ReasonPrereq {
		t.Errorf("reason = %q, want the prerequisite reason", d.Reason)
	}
	// Naming the blocker is the difference between advice a learner can act on
	// and a shrug.
	if d.Args["topic"] != "past-tense" {
		t.Errorf("args must name the nearest unmet prerequisite, got %v", d.Args)
	}
	if !d.Demoted {
		t.Error("falling back to locked is a demotion and must be announced")
	}
}

// A newly opened topic ALWAYS starts blocked. This is not conditional on the
// learner's level: "initial blocked practice is important" is the finding.
func TestUnlockingStartsBlocked(t *testing.T) {
	d := Evaluate(Progress{Phase: Locked}, openGate(), testConfig())
	if d.Phase != Blocked {
		t.Fatalf("phase = %v, want blocked", d.Phase)
	}
	if d.Reason != ReasonWarmup {
		t.Errorf("reason = %q, want the warm-up reason", d.Reason)
	}
}

// A thin estimate is not a failing one, and the two must not read the same to a
// learner staring at a progress bar.
func TestThinEstimateIsNotAFailure(t *testing.T) {
	cfg := testConfig()
	p := Progress{Phase: Blocked, BlockedAccuracy: 1.0, BlockedTotal: 3}

	d := Evaluate(p, openGate(), cfg)
	if d.Phase != Blocked {
		t.Fatalf("three perfect answers must not open the gate, got %v", d.Phase)
	}
	if d.Reason != ReasonWarmup {
		t.Errorf("reason = %q, want warm-up, not 'below the gate'", d.Reason)
	}
	if d.Ready {
		t.Error("Ready must be false while the estimate is thin")
	}
	if d.Args["answered"] != "3" || d.Args["need"] != "20" {
		t.Errorf("the warm-up reason must state the shortfall, got %v", d.Args)
	}
}

func TestGateOpensAtTheThreshold(t *testing.T) {
	cfg := testConfig()
	p := Progress{Phase: Blocked, BlockedAccuracy: 0.80, BlockedTotal: 20}

	d := Evaluate(p, openGate(), cfg)
	if d.Phase != Interleaved {
		t.Fatalf("phase = %v, want interleaved at exactly the threshold", d.Phase)
	}
	if d.Reason != ReasonGateCleared {
		t.Errorf("reason = %q", d.Reason)
	}
	// Just below must NOT open it.
	p.BlockedAccuracy = 0.799
	if got := Evaluate(p, openGate(), cfg); got.Phase != Blocked {
		t.Errorf("0.799 must stay blocked, got %v", got.Phase)
	}
}

// The gate is not a one-way door — but it is not a revolving one either. The
// hysteresis band is what stops a topic oscillating on every answer.
func TestHysteresisBandPreventsOscillation(t *testing.T) {
	cfg := testConfig()

	// Inside the band: below the gate, above the floor. Stays interleaved.
	inBand := Progress{Phase: Interleaved, BlockedAccuracy: 0.75, BlockedTotal: 30,
		InterleavedAccuracy: 0.75, InterleavedTotal: 50}
	if d := Evaluate(inBand, openGate(), cfg); d.Phase != Interleaved {
		t.Errorf("0.75 is inside the hysteresis band and must not demote, got %v", d.Phase)
	}

	// Below the floor: falls back.
	belowFloor := inBand
	belowFloor.BlockedAccuracy = 0.69
	d := Evaluate(belowFloor, openGate(), cfg)
	if d.Phase != Blocked {
		t.Fatalf("below the floor must fall back to blocked, got %v", d.Phase)
	}
	if d.Reason != ReasonFellBack || !d.Demoted {
		t.Errorf("a fallback must be announced as one: reason=%q demoted=%v", d.Reason, d.Demoted)
	}
}

// A collapse from `mastered` lands in `blocked`, not one rung down. Falling all
// the way is the point: if the learner can no longer do it at all, mixing it
// with contrasting items is exactly the wrong response.
func TestMasteredCanFallAllTheWayToBlocked(t *testing.T) {
	cfg := testConfig()
	p := Progress{Phase: Mastered, BlockedAccuracy: 0.55, BlockedTotal: 40,
		InterleavedAccuracy: 0.60, InterleavedTotal: 60}

	d := Evaluate(p, openGate(), cfg)
	if d.Phase != Blocked {
		t.Fatalf("phase = %v, want blocked", d.Phase)
	}
	if !d.Demoted {
		t.Error("demotion flag missing")
	}
}

func TestMasteryNeedsTheInterleavedWindow(t *testing.T) {
	cfg := testConfig()

	// Accurate, but not enough interleaved evidence yet.
	thin := Progress{Phase: Interleaved, BlockedAccuracy: 0.95, BlockedTotal: 40,
		InterleavedAccuracy: 0.95, InterleavedTotal: 10}
	d := Evaluate(thin, openGate(), cfg)
	if d.Phase != Interleaved || d.Reason != ReasonMasteryWarmup {
		t.Errorf("phase=%v reason=%q, want interleaved + mastery warm-up", d.Phase, d.Reason)
	}

	full := thin
	full.InterleavedTotal = 50
	if got := Evaluate(full, openGate(), cfg); got.Phase != Mastered {
		t.Errorf("phase = %v, want mastered", got.Phase)
	}
}

// [R15]: harmony is a motor skill. Accurate-but-slow is not mastery, and the
// gate must say which half is missing.
func TestMotorSkillMasteryRequiresSpeed(t *testing.T) {
	cfg := testConfig()
	slow := Progress{Phase: Interleaved, BlockedAccuracy: 0.97, BlockedTotal: 60,
		InterleavedAccuracy: 0.97, InterleavedTotal: 60, MedianMs: 4200}

	d := Evaluate(slow, Gate{Drillable: true, MotorSkill: true}, cfg)
	if d.Phase != Interleaved {
		t.Fatalf("phase = %v, want interleaved — the skill is not automated", d.Phase)
	}
	if d.Reason != ReasonNeedsSpeed {
		t.Errorf("reason = %q, want the speed reason", d.Reason)
	}
	if d.Args["median"] != "4200" || d.Args["budget"] != "2500" {
		t.Errorf("the reason must state both numbers, got %v", d.Args)
	}

	// The same numbers on a NON-motor topic are mastery.
	if got := Evaluate(slow, openGate(), cfg); got.Phase != Mastered {
		t.Errorf("a non-motor topic masters on accuracy alone, got %v", got.Phase)
	}

	fast := slow
	fast.MedianMs = 1400
	got := Evaluate(fast, Gate{Drillable: true, MotorSkill: true}, cfg)
	if got.Phase != Mastered || got.Reason != ReasonMasteredMotor {
		t.Errorf("phase=%v reason=%q, want mastered by automaticity", got.Phase, got.Reason)
	}
}

// [R15]: word order is locked behind listening volume regardless of everything
// else, and the app states the shortfall rather than just refusing.
func TestListeningGateLocksWordOrder(t *testing.T) {
	cfg := testConfig()
	p := Progress{Phase: Interleaved, BlockedAccuracy: 1.0, BlockedTotal: 100}

	d := Evaluate(p, Gate{Drillable: true, Listening: &ListeningGate{HeardHours: 3.5, NeedHours: 40}}, cfg)
	if d.Phase != Locked || d.Reason != ReasonListening {
		t.Fatalf("phase=%v reason=%q, want locked on listening volume", d.Phase, d.Reason)
	}
	if d.Args["heard"] != "3.5" || d.Args["need"] != "40.0" {
		t.Errorf("the shortfall must be stated, got %v", d.Args)
	}

	met := Evaluate(p, Gate{Drillable: true, Listening: &ListeningGate{HeardHours: 40, NeedHours: 40}}, cfg)
	if met.Phase == Locked {
		t.Error("meeting the threshold must open the topic")
	}
}

// A topic we have no generator for must not wall off the rest of the curriculum,
// and must not claim progress it has not measured.
func TestUndrillableTopicIsExplainedNotLocked(t *testing.T) {
	d := Evaluate(Progress{Phase: Locked}, Gate{Drillable: false}, testConfig())
	if d.Phase != Explained {
		t.Fatalf("phase = %v, want explained", d.Phase)
	}
	if d.Reason != ReasonExplainedOnly {
		t.Errorf("reason = %q", d.Reason)
	}
	if !d.Phase.Opens() {
		t.Error("an explained topic must satisfy its dependents' prerequisites")
	}
	if d.Phase.Servable() {
		t.Error("an explained topic has no items to serve")
	}
}

// Which window an answer lands in is decided by the phase it was SERVED in.
// Otherwise a learner could reach mastery on blocked answers alone, without ever
// having had to discriminate between the contrasting forms.
func TestFoldRoutesByServedPhase(t *testing.T) {
	cfg := testConfig()
	p := Progress{Phase: Interleaved}

	p = Fold(p, Blocked, true, 1000, cfg)
	if p.InterleavedTotal != 0 {
		t.Error("a blocked answer must not enter the mastery window")
	}
	if p.BlockedTotal != 1 {
		t.Error("every answer feeds the gate window")
	}

	p = Fold(p, Interleaved, true, 1000, cfg)
	if p.InterleavedTotal != 1 {
		t.Error("an interleaved answer must enter the mastery window")
	}
	if p.BlockedTotal != 2 {
		t.Error("interleaved answers also feed the gate window — the fallback needs them")
	}
}

// Latency is evidence only on a correct answer: a fast wrong answer is guessing.
func TestFoldOnlyTimesCorrectAnswers(t *testing.T) {
	cfg := testConfig()
	p := Fold(Progress{}, Blocked, false, 300, cfg)
	if p.MedianMs != 0 {
		t.Errorf("a wrong answer must not lower the median, got %d", p.MedianMs)
	}
	p = Fold(p, Blocked, true, 1500, cfg)
	if p.MedianMs != 1500 {
		t.Errorf("median = %d, want the first correct answer's duration", p.MedianMs)
	}
}

func TestApplyRecordsFallbacks(t *testing.T) {
	now := time.Now()
	p := Progress{Phase: Interleaved}
	p = Apply(p, Decision{Phase: Blocked, From: Interleaved, Demoted: true}, now)

	if p.Phase != Blocked {
		t.Fatalf("phase = %v", p.Phase)
	}
	if p.Fallbacks != 1 {
		t.Errorf("fallbacks = %d, want 1 — a demotion the learner can see", p.Fallbacks)
	}
	if !p.PhaseSince.Equal(now) {
		t.Error("phase_since must move with the phase")
	}
	// A no-op decision must not manufacture a fallback.
	p = Apply(p, Decision{Phase: Blocked, From: Blocked}, now.Add(time.Hour))
	if p.Fallbacks != 1 {
		t.Errorf("fallbacks = %d, want 1 — no phase change happened", p.Fallbacks)
	}
}

// A full run up the ladder and back down, the way a learner actually experiences
// it. This is the acceptance test for R5.
func TestLifecycleBlockedToInterleavedToMasteredAndBack(t *testing.T) {
	cfg := testConfig()
	p := Progress{TopicCode: "definite-conjugation", Phase: Locked}
	now := time.Now()

	step := func(correct bool, ms int64) Decision {
		p = Fold(p, p.Phase, correct, ms, cfg)
		d := Evaluate(p, openGate(), cfg)
		p = Apply(p, d, now)
		return d
	}

	// Twenty-five right answers under blocked practice opens the gate.
	var d Decision
	for range 25 {
		d = step(true, 1200)
	}
	if d.Phase != Interleaved {
		t.Fatalf("after 25 correct blocked answers, phase = %v, want interleaved", d.Phase)
	}

	// Sixty right answers under interleaving reaches mastery.
	for range 60 {
		d = step(true, 1200)
	}
	if d.Phase != Mastered {
		t.Fatalf("after 60 correct interleaved answers, phase = %v, want mastered", d.Phase)
	}

	// A sustained collapse takes it all the way back to blocked practice.
	for range 40 {
		d = step(false, 5000)
	}
	if d.Phase != Blocked {
		t.Fatalf("after a sustained collapse, phase = %v, want blocked", d.Phase)
	}
	if p.Fallbacks == 0 {
		t.Error("the fallbacks counter must record the demotion")
	}

	// And it can be earned back — the gate is not punitive.
	for range 40 {
		d = step(true, 1100)
	}
	if d.Phase == Blocked {
		t.Error("a recovered learner must be able to climb back out of blocked practice")
	}
}
