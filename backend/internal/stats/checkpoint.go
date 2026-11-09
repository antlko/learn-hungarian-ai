package stats

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"tanulom/internal/db"
)

// Checkpoints: the source plan's M1/M3/M6/M12/M18 assessments, made structural.
//
// Levels replaced by assessments is [R14]'s point 4: pass/fail with substance
// behind it, and a failed checkpoint BLOCKS curriculum advance beyond its level
// until retaken — the "правило отката". What the app can measure it measures;
// what only a human can verify (a monologue a teacher understands, a phone
// call, a real exam) is recorded as the learner's own attestation and labelled
// as exactly that. The app never pretends to have verified a conversation.

// CheckpointSpec defines one assessment.
type CheckpointSpec struct {
	Code string `json:"code"`
	// LemmaTarget is the measured criterion: lemmas in learning or known.
	LemmaTarget int `json:"lemmaTarget"`
	// GatesLevel is the CEFR level a failure blocks ('' = blocks nothing).
	GatesLevel string `json:"gatesLevel"`
	// Attested lists the human-verified criteria, as i18n keys. The UI renders
	// each with a checkbox and the standing caveat that the app cannot check it.
	Attested []string `json:"attested"`
}

// Specs in curriculum order. Targets from the source plan's month milestones.
var Specs = []CheckpointSpec{
	{Code: "m1", LemmaTarget: 300, GatesLevel: "",
		Attested: []string{"checkpoint.m1.readAloud"}},
	{Code: "m3", LemmaTarget: 900, GatesLevel: "A2",
		Attested: []string{"checkpoint.m3.monologue"}},
	{Code: "m6", LemmaTarget: 2000, GatesLevel: "B1",
		Attested: []string{"checkpoint.m6.phoneCall", "checkpoint.m6.mockExam"}},
	{Code: "m12", LemmaTarget: 3500, GatesLevel: "B2",
		Attested: []string{"checkpoint.m12.realExam"}},
	{Code: "m18", LemmaTarget: 6000, GatesLevel: "",
		Attested: []string{"checkpoint.m18.b2Exam"}},
}

// SpecFor finds one spec.
func SpecFor(code string) (CheckpointSpec, bool) {
	for _, s := range Specs {
		if s.Code == code {
			return s, true
		}
	}
	return CheckpointSpec{}, false
}

// Evidence is the measured state at decision time, stored with the verdict so
// a later reader can see what it rested on.
type Evidence struct {
	Lemmas         int      `json:"lemmas"`
	LemmaTarget    int      `json:"lemmaTarget"`
	MissedDays30   int      `json:"missedDays30"`
	AvgCoverage30  float64  `json:"avgCoverage30"`
	HumanDays30    int      `json:"humanDays30"`
	ListeningHours float64  `json:"listeningHours"`
	Attested       []string `json:"attested"`
}

// Diagnostic names which of the three documented failure modes the data
// supports. Verbatim from the source plan: "Обычно это одно из трёх:
// пропущенные дни в Anki, слишком сложный входной материал, отсутствие живой
// практики." Each finding is an i18n key plus the number that triggered it.
type Diagnostic struct {
	Findings []DiagnosticFinding `json:"findings"`
}

type DiagnosticFinding struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
}

// Diagnose applies the three-failure-mode rubric to the evidence.
func Diagnose(ev Evidence) Diagnostic {
	var d Diagnostic
	if ev.MissedDays30 > 6 {
		d.Findings = append(d.Findings, DiagnosticFinding{
			Key: "checkpoint.diag.missedDays", Value: float64(ev.MissedDays30)})
	}
	// The reading-comprehension floor: texts the learner actually read.
	// AvgCoverage30 == 0 means no reading happened, which is its own version
	// of "input too hard to even start".
	if ev.AvgCoverage30 < 0.90 {
		d.Findings = append(d.Findings, DiagnosticFinding{
			Key: "checkpoint.diag.inputTooHard", Value: ev.AvgCoverage30})
	}
	if ev.HumanDays30 == 0 {
		d.Findings = append(d.Findings, DiagnosticFinding{
			Key: "checkpoint.diag.noLivePractice", Value: 0})
	}
	return d
}

// Checkpoint is one recorded verdict.
type Checkpoint struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Status    string    `json:"status"`
	Evidence  Evidence  `json:"evidence"`
	Notes     string    `json:"notes"`
	DecidedAt time.Time `json:"decidedAt"`
}

// RecordCheckpoint stores a verdict.
func (s *Store) RecordCheckpoint(userID int64, code, status string, ev Evidence, notes string) error {
	raw, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO checkpoint (user_id, code, status, evidence, notes)
		VALUES (?,?,?,?,?)`, userID, code, status, string(raw), notes)
	if err != nil {
		return fmt.Errorf("record checkpoint: %w", err)
	}
	return nil
}

// Checkpoints loads history, newest first.
func (s *Store) Checkpoints(userID int64) ([]Checkpoint, error) {
	rows, err := s.db.Query(`
		SELECT id, code, status, evidence, notes, decided_at
		  FROM checkpoint WHERE user_id = ? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("load checkpoints: %w", err)
	}
	defer rows.Close()
	var out []Checkpoint
	for rows.Next() {
		var c Checkpoint
		var raw, when string
		if err := rows.Scan(&c.ID, &c.Code, &c.Status, &raw, &c.Notes, &when); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(raw), &c.Evidence)
		c.DecidedAt = db.ParseTime(sql.NullString{String: when, Valid: true})
		out = append(out, c)
	}
	return out, rows.Err()
}

// BlockedLevels returns the CEFR levels currently blocked by the "правило
// отката": for each checkpoint whose LATEST verdict is a failure, its gated
// level and everything above. A pass at any later attempt lifts the block.
func BlockedLevels(history []Checkpoint) map[string]bool {
	latest := map[string]string{}
	for i := len(history) - 1; i >= 0; i-- { // oldest → newest: newest wins
		latest[history[i].Code] = history[i].Status
	}
	order := []string{"A1", "A2", "B1", "B2"}
	blocked := map[string]bool{}
	for _, spec := range Specs {
		if spec.GatesLevel == "" || latest[spec.Code] != "failed" {
			continue
		}
		hit := false
		for _, lvl := range order {
			if lvl == spec.GatesLevel {
				hit = true
			}
			if hit {
				blocked[lvl] = true
			}
		}
	}
	return blocked
}

// LemmaCount is the measured "лемм в Anki" figure: lemmas the learner has in
// play — learning or known — in the lemma state space.
func (s *Store) LemmaCount(userID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM lemma_state
		 WHERE user_id = ? AND status IN ('learning','known')`, userID).Scan(&n)
	return n, err
}

// EvidenceFor gathers the measured half of a checkpoint's evidence.
//
// listeningHours is passed in rather than queried: the caller already holds the
// figure for its own screen, and two reads of the same ledger in one request
// can only disagree.
func (s *Store) EvidenceFor(userID int64, spec CheckpointSpec, days []Day, today string,
	dailyMinutes int, listeningHours float64) (Evidence, error) {
	ev := Evidence{LemmaTarget: spec.LemmaTarget}
	var err error
	if ev.Lemmas, err = s.LemmaCount(userID); err != nil {
		return ev, err
	}

	met := map[string]bool{}
	human := map[string]bool{}
	for _, d := range days {
		met[d.Day] = d.Met(dailyMinutes)
		if d.Human {
			human[d.Day] = true
		}
	}
	cursor, _ := time.Parse("2006-01-02", today)
	for i := 0; i < 30; i++ {
		key := cursor.Format("2006-01-02")
		if !met[key] {
			ev.MissedDays30++
		}
		if human[key] {
			ev.HumanDays30++
		}
		cursor = cursor.AddDate(0, 0, -1)
	}

	// Average element coverage of texts actually opened in the last 30 days.
	err = s.db.QueryRow(`
		SELECT COALESCE(AVG(element_coverage), 0) FROM text
		 WHERE user_id = ? AND read_at IS NOT NULL AND read_at >= ?`,
		userID, time.Now().AddDate(0, 0, -30).UTC().Format("2006-01-02")).Scan(&ev.AvgCoverage30)
	if err != nil && err != sql.ErrNoRows {
		return ev, err
	}

	ev.ListeningHours = listeningHours
	return ev, nil
}

// chartTargetCheckpoints maps each CEFR band to the checkpoint whose lemma
// target marks reaching it: m3 → A1 (900), m6 → A2 (2000), m12 → B1 (3500),
// m18 → B2 (6000). The Progress chart's reference lines are read from Specs,
// so the milestone numbers live in exactly one place.
var chartTargetCheckpoints = []struct{ Level, Code string }{
	{"A1", "m3"}, {"A2", "m6"}, {"B1", "m12"}, {"B2", "m18"},
}

// LemmaTargets returns the reference lines for the lemma curve, by band.
func LemmaTargets() map[string]int {
	out := make(map[string]int, len(chartTargetCheckpoints))
	for _, t := range chartTargetCheckpoints {
		if spec, ok := SpecFor(t.Code); ok {
			out[t.Level] = spec.LemmaTarget
		}
	}
	return out
}
