package srs

import "math"

// A progress number per word, 0-100, on which the statuses are marks.
//
// [R20] see .claude/research/20-practice-and-progress.md, ADR-070.
//
// It is DERIVED from the stored FSRS state alone — stage and stability — so it
// moves only when an answer moves the state: up on a correct answer, down on a
// lapse (relearning is below 20), never on its own with the passing of time.
// The band comes from StageOf so the number can never disagree with the stage
// chip. Display-only: not a score, not points, not a level.

// ProgressFullDays is the stability at which the number reaches 100. It is a
// display convention, like MatureDays — "stable for half a year" — and not a
// memory threshold or a goal.
const ProgressFullDays = 180

// progressOf maps one card's memory to 0-100.
func progressOf(m Memory) int {
	switch m.Stage {
	case StageLearning, StageRelearning:
		done := m.StepsTotal - m.StepsLeft
		if done < 0 {
			done = 0
		}
		p := 20 * (done + 1) / (m.StepsTotal + 1)
		return min(max(p, 0), 19)
	case StageYoung:
		p := 20 + int(math.Floor(50*math.Min(m.StabilityDays/MatureDays, 1)))
		return min(max(p, 20), 69)
	case StageMature:
		x := 0.0
		if m.StabilityDays > 0 {
			x = math.Log(m.StabilityDays/MatureDays) / math.Log(float64(ProgressFullDays)/MatureDays)
		}
		x = math.Min(math.Max(x, 0), 1)
		p := 70 + int(math.Floor(30*x))
		return min(max(p, 70), 100)
	}
	return 0
}
