package srs

import (
	"math/rand"
	"testing"
	"time"
)

// [R20] see .claude/research/20-practice-and-progress.md

func progressFor(t *testing.T, st CardState) int {
	t.Helper()
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	return MemoryFor(st, NewScheduler(0.9), now, false).Progress
}

func TestProgressBands(t *testing.T) {
	cases := []struct {
		name string
		st   CardState
		want int
	}{
		{"new", CardState{}, 0},
		{"learning step 0 of 2", CardState{State: 1, RemainingSteps: 2}, 6},
		{"learning step 1 of 2", CardState{State: 1, RemainingSteps: 1}, 13},
		{"learning capped at 19", CardState{State: 1, RemainingSteps: 0}, 19},
		{"relearning step 0 of 1", CardState{State: 3, RemainingSteps: 1}, 10},
		{"relearning never reaches 20", CardState{State: 3, RemainingSteps: 0}, 19},
		{"young S=0", CardState{State: 2, ScheduledDays: 1, Stability: 0}, 20},
		{"young S=11", CardState{State: 2, ScheduledDays: 10, Stability: 11}, 46},
		{"young S=20.9 interval 20", CardState{State: 2, ScheduledDays: 20, Stability: 20.9}, 69},
		{"young S far above 21 clamps to 69", CardState{State: 2, ScheduledDays: 20, Stability: 90}, 69},
		{"mature S=21", CardState{State: 2, ScheduledDays: 21, Stability: 21}, 70},
		{"mature S=35", CardState{State: 2, ScheduledDays: 35, Stability: 35}, 77},
		{"mature S=100", CardState{State: 2, ScheduledDays: 100, Stability: 100}, 91},
		{"mature S=180", CardState{State: 2, ScheduledDays: 180, Stability: 180}, 100},
		{"mature S=1000 clamps to 100", CardState{State: 2, ScheduledDays: 900, Stability: 1000}, 100},
	}
	for _, c := range cases {
		if got := progressFor(t, c.st); got != c.want {
			t.Errorf("%s: progress = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestProgressIsMonotoneInStabilityAndSteps(t *testing.T) {
	prev := -1
	for s := 0.0; s <= 400; s += 0.5 {
		days := uint64(s)
		st := CardState{State: 2, ScheduledDays: days, Stability: s}
		p := progressFor(t, st)
		// Crossing the young/mature split by interval is monotone as S rises
		// together with the interval here.
		if p < prev {
			t.Fatalf("S=%v: progress %d fell below %d", s, p, prev)
		}
		prev = p
	}
	for _, state := range []int{1, 3} {
		prev = -1
		for left := 2; left >= 0; left-- {
			p := progressFor(t, CardState{State: state, RemainingSteps: left})
			if p < prev || p >= 20 {
				t.Fatalf("state %d left %d: progress %d (prev %d) must rise and stay under 20", state, left, p, prev)
			}
			prev = p
		}
	}
}

// Random answers through the real scheduler: a correct answer never lowers the
// number, except the documented flip across the 21-day boundary (interval fuzz
// can drop a card from mature to young: -1, and the stage chip flips with it);
// and a lapse from Review always lands under 20.
func TestProgressRisesOnCorrectAndFallsOnLapse(t *testing.T) {
	rnd := rand.New(rand.NewSource(20261111))
	sched := NewScheduler(0.9)
	flips, maxDrop := 0, 0

	for trial := 0; trial < 300; trial++ {
		now := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
		st := NewCardState(now)
		prev := MemoryFor(st, sched, now, false)
		for i := 0; i < 40; i++ {
			rating := Rating(1 + rnd.Intn(4))
			var gap time.Duration
			switch rnd.Intn(4) {
			case 0:
				gap = time.Duration(1+rnd.Intn(50)) * time.Second
			case 1:
				gap = time.Duration(1+rnd.Intn(120)) * time.Minute
			case 2:
				gap = time.Duration(1+rnd.Intn(12)) * time.Hour
			default:
				gap = time.Duration(1+rnd.Intn(60*24)) * time.Hour
			}
			now = now.Add(gap)
			wasReview := st.State == 2
			next, _, err := sched.Review(st, rating, now)
			if err != nil {
				t.Fatal(err)
			}
			cur := MemoryFor(next, sched, now, false)

			switch {
			case rating == Again && wasReview:
				if cur.Progress >= 20 {
					t.Fatalf("trial %d: Again from Review left progress %d (%+v)", trial, cur.Progress, next)
				}
			case rating != Again:
				if cur.Progress < prev.Progress {
					if prev.Stage == StageMature && cur.Stage == StageYoung {
						flips++
						// Interval fuzz can put a mature card's interval a few
						// days above its stability, so the young formula can
						// read lower than the mature floor of 70 by more than 1
						// (ADR-070 states the edge; it is documented, not patched).
						maxDrop = max(maxDrop, prev.Progress-cur.Progress)
					} else {
						t.Fatalf("trial %d step %d: %s %d -> %s %d on rating %d",
							trial, i, prev.Stage, prev.Progress, cur.Stage, cur.Progress, rating)
					}
				}
			}
			st, prev = next, cur
		}
	}
	t.Logf("mature->young boundary flips seen: %d, largest drop %d", flips, maxDrop)
	if maxDrop > 15 {
		t.Fatalf("a boundary flip fell by %d points; fuzz alone should not explain that", maxDrop)
	}
}
