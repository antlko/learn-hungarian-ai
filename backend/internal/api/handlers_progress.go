package api

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/srs"
	"tanulom/internal/stats"
)

// Progress endpoints: the overview, the daily ledger, checkpoints and Loop 4's
// attestation. See internal/stats, [R14].

func localDay(t time.Time) string { return t.Format("2006-01-02") }

// recordInteraction folds a graded interaction into today's ledger. Failures
// here are logged-and-swallowed by callers: the answer itself already
// committed, and refusing it because the ledger hiccuped would lose real work.
func (s *Server) recordInteraction(userID int64, kind string, durationMs int64) {
	now := time.Now()
	due, err := srs.NewStore(s.db.DB).DueCount(userID, now)
	if err != nil {
		return
	}
	_ = stats.NewStore(s.db.DB).Record(userID, localDay(now), kind, durationMs, due)
}

// LemmaCurvePoint is one day's cumulative lemma count, from first_seen.
type LemmaCurvePoint struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

// OverviewResponse is the Progress screen's data.
type OverviewResponse struct {
	Lemmas struct {
		Known    int               `json:"known"`
		Learning int               `json:"learning"`
		Curve    []LemmaCurvePoint `json:"curve"`
		// Targets are the source plan's month milestones, rendered as reference
		// lines. They are targets, not judgements.
		Targets map[string]int `json:"targets"`
	} `json:"lemmas"`
	Streak         stats.Streak       `json:"streak"`
	Today          stats.Day          `json:"today"`
	DueNow         int                `json:"dueNow"`
	DailyMinutes   int                `json:"dailyMinutes"`
	ListeningHours float64            `json:"listeningHours"`
	Checkpoints    []stats.Checkpoint `json:"checkpoints"`
	// NextCheckpoint is the first spec with no passing verdict yet, with live
	// evidence so the learner can see where they stand BEFORE taking it.
	NextCheckpoint *NextCheckpointDTO `json:"nextCheckpoint,omitempty"`
	BlockedLevels  []string           `json:"blockedLevels"`
	ExamDate       string             `json:"examDate,omitempty"`
}

type NextCheckpointDTO struct {
	Spec     stats.CheckpointSpec `json:"spec"`
	Evidence stats.Evidence       `json:"evidence"`
}

func (s *Server) handleOverview(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	cfg := s.Config()
	st := stats.NewStore(s.db.DB)
	now := time.Now()
	today := localDay(now)

	var out OverviewResponse
	out.DailyMinutes = cfg.Review.DailyMinutes
	out.ExamDate = cfg.Curriculum.ExamDate

	// Lemma counts and the cumulative curve from first_seen.
	rows, err := s.db.Query(`
		SELECT substr(first_seen, 1, 10) AS day, COUNT(*)
		  FROM lemma_state
		 WHERE user_id = ? AND status IN ('learning','known')
		 GROUP BY day ORDER BY day`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	cum := 0
	for rows.Next() {
		var day string
		var n int
		if err := rows.Scan(&day, &n); err != nil {
			return err
		}
		cum += n
		out.Lemmas.Curve = append(out.Lemmas.Curve, LemmaCurvePoint{Day: day, Count: cum})
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM lemma_state WHERE user_id=? AND status='known'`,
		u.ID).Scan(&out.Lemmas.Known); err != nil {
		return err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM lemma_state WHERE user_id=? AND status='learning'`,
		u.ID).Scan(&out.Lemmas.Learning); err != nil {
		return err
	}
	out.Lemmas.Targets = map[string]int{"A1": 900, "A2": 2000, "B1": 3500, "B2": 6000}

	days, err := st.Days(u.ID, 400)
	if err != nil {
		return err
	}
	out.Streak = stats.ComputeStreak(days, today, cfg.Review.DailyMinutes)
	for _, d := range days {
		if d.Day == today {
			out.Today = d
			break
		}
	}
	out.Today.Day = today
	out.DueNow, _ = srs.NewStore(s.db.DB).DueCount(u.ID, now)
	out.ListeningHours = s.listeningHours(u.ID)

	history, err := st.Checkpoints(u.ID)
	if err != nil {
		return err
	}
	if history == nil {
		history = []stats.Checkpoint{}
	}
	out.Checkpoints = history
	for lvl := range stats.BlockedLevels(history) {
		out.BlockedLevels = append(out.BlockedLevels, lvl)
	}
	if out.BlockedLevels == nil {
		out.BlockedLevels = []string{}
	}

	// The next checkpoint: first spec whose latest verdict is not a pass.
	latest := map[string]string{}
	for i := len(history) - 1; i >= 0; i-- {
		latest[history[i].Code] = history[i].Status
	}
	for _, spec := range stats.Specs {
		if latest[spec.Code] == "passed" {
			continue
		}
		ev, err := st.EvidenceFor(u.ID, spec, days, today, cfg.Review.DailyMinutes,
			out.ListeningHours)
		if err != nil {
			return err
		}
		out.NextCheckpoint = &NextCheckpointDTO{Spec: spec, Evidence: ev}
		break
	}
	return c.JSON(out)
}

// CheckpointRequest records a verdict attempt.
type CheckpointRequest struct {
	Code string `json:"code"`
	// Attested lists the human-verified criteria the learner confirms. The app
	// records these as claims — it cannot check them and does not pretend to.
	Attested []string `json:"attested"`
	Notes    string   `json:"notes"`
}

// CheckpointResponse is the verdict plus, on failure, the diagnostic mapped to
// the three documented failure modes.
type CheckpointResponse struct {
	Checkpoint stats.Checkpoint  `json:"checkpoint"`
	Diagnostic *stats.Diagnostic `json:"diagnostic,omitempty"`
}

func (s *Server) handleCheckpoint(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req CheckpointRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	spec, ok := stats.SpecFor(req.Code)
	if !ok {
		return badRequest("unknown checkpoint %q", req.Code)
	}
	cfg := s.Config()
	st := stats.NewStore(s.db.DB)
	days, err := st.Days(u.ID, 60)
	if err != nil {
		return err
	}
	ev, err := st.EvidenceFor(u.ID, spec, days, localDay(time.Now()), cfg.Review.DailyMinutes,
		s.listeningHours(u.ID))
	if err != nil {
		return err
	}
	ev.Attested = req.Attested

	// The verdict: measured criteria AND every attested criterion confirmed.
	attested := map[string]bool{}
	for _, a := range req.Attested {
		attested[a] = true
	}
	status := "passed"
	if ev.Lemmas < spec.LemmaTarget {
		status = "failed"
	}
	for _, need := range spec.Attested {
		if !attested[need] {
			status = "failed"
		}
	}

	if err := st.RecordCheckpoint(u.ID, spec.Code, status, ev, req.Notes); err != nil {
		return err
	}
	out := CheckpointResponse{Checkpoint: stats.Checkpoint{
		Code: spec.Code, Status: status, Evidence: ev, Notes: req.Notes,
		DecidedAt: time.Now(),
	}}
	if status == "failed" {
		d := stats.Diagnose(ev)
		out.Diagnostic = &d
	}
	return c.JSON(out)
}

// HumanAttestRequest is Loop 4's "this happened today".
type HumanAttestRequest struct {
	Happened bool `json:"happened"`
}

func (s *Server) handleAttestHuman(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req HumanAttestRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if err := stats.NewStore(s.db.DB).AttestHuman(u.ID, localDay(time.Now()), req.Happened); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"ok": true})
}
