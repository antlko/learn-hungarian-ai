package api

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/morph"
)

// Learner corrections to a bad pack analysis.
//
// The pack is read-only (ADR-002), so "this analysis is wrong" cannot fix the
// pack — it writes an override into the USER database, applied as an overlay at
// lookup time. Two properties matter:
//
//   - The overlay is per-user. One learner's correction must not rewrite what
//     another learner (or a future pack) sees; and a correction is a claim, not
//     a fact — the UI marks overlaid analyses as corrected-by-you.
//   - The report is kept even when the learner cannot supply the right answer.
//     "This is wrong, I don't know what's right" is valuable signal for the next
//     pack build, and requiring a correct replacement would suppress it.

// CorrectionRequest reports one wrong analysis.
type CorrectionRequest struct {
	// Form is the surface string whose analysis is wrong.
	Form string `json:"form"`
	// Lemma and Feats are the learner's proposed correction. Both optional:
	// an empty correction is a pure report.
	Lemma string `json:"lemma"`
	Feats string `json:"feats"`
	Note  string `json:"note"`
}

// CorrectionDTO is one stored correction.
type CorrectionDTO struct {
	Form      string `json:"form"`
	Lemma     string `json:"lemma"`
	Feats     string `json:"feats"`
	Note      string `json:"note"`
	CreatedAt string `json:"createdAt"`
}

func (s *Server) handleCreateCorrection(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req CorrectionRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	req.Form = strings.TrimSpace(req.Form)
	if req.Form == "" {
		return badRequest("form is required")
	}
	if len(req.Form) > 100 || len(req.Lemma) > 100 || len(req.Feats) > 300 || len(req.Note) > 1000 {
		return badRequest("correction fields are too long")
	}
	// Upsert: re-reporting the same form replaces the previous correction
	// rather than erroring — the learner's latest opinion wins.
	_, err = s.db.Exec(`
		INSERT INTO morph_correction (user_id, form, lemma, feats, note)
		VALUES (?,?,?,?,?)
		ON CONFLICT(user_id, form) DO UPDATE SET
		  lemma=excluded.lemma, feats=excluded.feats, note=excluded.note`,
		u.ID, req.Form, req.Lemma, req.Feats, req.Note)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

func (s *Server) handleDeleteCorrection(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	form := strings.TrimSpace(c.Query("form"))
	if form == "" {
		return badRequest("form is required")
	}
	if _, err := s.db.Exec(
		`DELETE FROM morph_correction WHERE user_id = ? AND form = ?`, u.ID, form); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (s *Server) handleListCorrections(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	rows, err := s.db.Query(`
		SELECT form, lemma, feats, note, created_at
		  FROM morph_correction WHERE user_id = ? ORDER BY created_at DESC`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []CorrectionDTO{}
	for rows.Next() {
		var d CorrectionDTO
		if err := rows.Scan(&d.Form, &d.Lemma, &d.Feats, &d.Note, &d.CreatedAt); err != nil {
			return err
		}
		out = append(out, d)
	}
	return c.JSON(fiber.Map{"corrections": out})
}

// correctionFor loads one user's override for a form, if any.
func (s *Server) correctionFor(userID int64, form string) (lemma, feats string, ok bool) {
	err := s.db.QueryRow(`
		SELECT lemma, feats FROM morph_correction WHERE user_id = ? AND form = ?`,
		userID, form).Scan(&lemma, &feats)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		return "", "", false
	}
	return lemma, feats, true
}

// applyCorrection overlays a learner's correction on an analysis.
//
// The result is marked `corrected`, a certainty of its own: the UI must show
// that this reading is the learner's override, not the pack's claim — an
// override that looked authoritative would hide its own fallibility.
func applyCorrection(an morph.Analysis, lemma, feats string) morph.Analysis {
	if lemma != "" {
		an.Lemma = lemma
		an.LemmaID = 0 // the pack's id no longer describes this reading
	}
	if feats != "" {
		an.Feats = morph.ParseFeats(feats)
		an.FeatureID = morph.FeatureIDs(an.Feats)
	}
	an.Source = "correction"
	an.Certainty = morph.CorrectedAnalysis
	return an
}
