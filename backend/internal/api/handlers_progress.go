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
//
// due_left is the ACTIONABLE count — what a queue built now would serve, after
// burial. The raw card count it used to store included the siblings burial had
// hidden until tomorrow, so a learner whose words carry more than one due card
// could clear everything the app would ever show and still fail the streak's
// "queue cleared" obligation, every day, forever. ADR-021.
func (s *Server) recordInteraction(userID int64, kind string, durationMs int64) {
	now := time.Now()
	due, err := srs.NewStore(s.db.DB).ActionableDue(userID,
		s.Config().Review.BurySiblingsEnabled(), now)
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
	Streak stats.Streak `json:"streak"`
	Today  stats.Day    `json:"today"`
	// Practice is free-practice volume and accuracy, reported SEPARATELY from
	// reviews because it means something different: it schedules nothing, so a
	// learner reading one number must not mistake it for progress through the
	// deck. See ADR-019.
	Practice srs.PracticeStats `json:"practice"`
	// PracticeDays and PracticeByDirection are the practice series for the
	// Progress screen — volume AND accuracy, per day and per direction, still
	// reported apart from reviews and still moving no schedule (ADR-019/024).
	PracticeDays        []srs.PracticeDay       `json:"practiceDays"`
	PracticeByDirection []srs.PracticeDirection `json:"practiceByDirection"`
	// Days is the recent daily ledger, newest first — the load sparkline the
	// Today screen draws. The rows were always loaded here for the streak and
	// then thrown away; exposing them costs no query.
	Days []stats.Day `json:"days"`
	// Queue is the whole deck's Breakdown — the same numbers, in the same
	// coordinate system, that the review screen's tabs show (ADR-021). DueNow
	// duplicates Queue.DueNow so existing clients keep a number that is now
	// honest instead of the raw unburied card count it used to be.
	Queue          srs.Breakdown      `json:"queue"`
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

// dayWindow reads `?days=`, clamped to the three ranges the chart offers.
//
// An arbitrary number is not accepted: the chart's three buttons are the whole
// vocabulary, and a client asking for 10 000 days would be asking the server
// to render something no screen has. Anything unrecognised is 30.
func dayWindow(raw string) int {
	switch raw {
	case "7":
		return 7
	case "30":
		return 30
	case "365":
		return 365
	default:
		// 90 by default, which is the widest window the chart offers.
		//
		// The client switches 7/30/90 WITHOUT a round trip — ninety rows of
		// eight small fields is nothing on the wire, and refetching to change
		// a chart's window made the switch feel like a page load. Asking for
		// less is still honoured; asking for something unrecognised gets the
		// default rather than an error, because a stale client should get a
		// chart and not a failure.
		return 90
	}
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
	if out.Practice, err = srs.NewStore(s.db.DB).Practice(u.ID, now); err != nil {
		return err
	}
	if out.PracticeDays, err = srs.NewStore(s.db.DB).PracticeDays(u.ID, 30, now); err != nil {
		return err
	}
	if out.PracticeDays == nil {
		out.PracticeDays = []srs.PracticeDay{}
	}
	if out.PracticeByDirection, err = srs.NewStore(s.db.DB).PracticeByDirection(u.ID); err != nil {
		return err
	}
	if out.PracticeByDirection == nil {
		out.PracticeByDirection = []srs.PracticeDirection{}
	}

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
	// The window the statistics screen asked for, newest first — Days()
	// already orders them that way. 90 by default: the chart offers 7/30/90
	// and switches between them client-side, so the response has to carry
	// the widest of them.
	//
	// The STREAK is always computed over the full 400 rows below, never over
	// this window: a learner looking at 30 days must not see their streak
	// truncated to 30 by the act of looking.
	out.Days = days
	if n := dayWindow(c.Query("days")); len(out.Days) > n {
		out.Days = out.Days[:n]
	}
	if out.Days == nil {
		out.Days = []stats.Day{}
	}
	out.Streak = stats.ComputeStreak(days, today, cfg.Review.DailyMinutes)
	for _, d := range days {
		if d.Day == today {
			out.Today = d
			break
		}
	}
	out.Today.Day = today
	if bds, err := srs.NewStore(s.db.DB).Breakdowns(u.ID,
		cfg.Review.BurySiblingsEnabled(), now); err == nil {
		for _, b := range bds {
			if b.Scope == srs.ScopeAll {
				out.Queue = b
				out.DueNow = b.DueNow
			}
		}
	}
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
