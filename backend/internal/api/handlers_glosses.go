package api

import (
	"database/sql"
	"errors"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// The learner's own dictionary, for the words the pack does not have.
//
// The pack is read-only and finite. `Kinyitom` resolves to no lemma, so the
// reader had nothing to show for it and a card made from it was blank on the
// back. This is the overlay — same shape as `morph_correction`, which corrects
// an analysis the same way.
//
// Two things may be saved: a meaning the learner typed, and a meaning the
// translation model produced. They are stored with `source` set accordingly and
// they render differently forever after, because they ARE different: one is a
// person's understanding and the other is a machine's guess that nobody has
// checked. Dropping that distinction on save would launder the second into the
// first, which is precisely what ADR-014 exists to prevent.

// GlossDTO is one entry in the learner's own dictionary.
type GlossDTO struct {
	Form  string `json:"form"`
	Gloss string `json:"gloss"`
	// Source is "manual" or "machine". The UI labels the second and never the
	// first, so this field is what keeps a machine gloss honest across a save.
	Source string `json:"source"`
	Model  string `json:"model,omitempty"`
}

// PutGlossRequest saves or replaces one entry.
type PutGlossRequest struct {
	Gloss  string `json:"gloss"`
	Source string `json:"source"`
	Model  string `json:"model"`
}

// glossKey normalises a surface form into the storage key.
//
// Lowercased here, on the way in AND on the way out, so a lookup is an exact
// primary-key hit. Doing it with `COLLATE NOCASE` at read time would throw the
// index away — the same reason `morph.CaseVariants` exists for the pack.
func glossKey(raw string) (string, error) {
	form, err := url.PathUnescape(raw)
	if err != nil {
		return "", badRequest("form is not a valid path segment")
	}
	form = strings.ToLower(strings.TrimSpace(form))
	if form == "" {
		return "", badRequest("form is required")
	}
	if len([]rune(form)) > 64 {
		return "", badRequest("form is longer than 64 characters")
	}
	return form, nil
}

func (s *Server) handleGetGloss(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	form, err := glossKey(c.Params("form"))
	if err != nil {
		return err
	}

	var out GlossDTO
	err = s.db.QueryRow(
		`SELECT form, gloss, source, model FROM user_gloss WHERE user_id = ? AND form = ?`,
		u.ID, form,
	).Scan(&out.Form, &out.Gloss, &out.Source, &out.Model)
	if errors.Is(err, sql.ErrNoRows) {
		// Not an error: "you have not written one" is the normal answer for
		// almost every word, and a 404 would have every caller special-case it.
		return c.JSON(GlossDTO{Form: form})
	}
	if err != nil {
		return err
	}
	return c.JSON(out)
}

func (s *Server) handlePutGloss(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	form, err := glossKey(c.Params("form"))
	if err != nil {
		return err
	}
	var req PutGlossRequest
	if err := decode(c, &req); err != nil {
		return err
	}

	gloss := strings.TrimSpace(req.Gloss)
	if gloss == "" {
		return badRequest("gloss is required")
	}
	if len([]rune(gloss)) > 500 {
		return badRequest("gloss is longer than 500 characters")
	}
	// Anything that is not explicitly machine output is the learner's own. The
	// default has to fall this way round: mislabelling a person's note as
	// machine output is a cosmetic insult, and the reverse quietly passes off an
	// unchecked translation as something a human stands behind.
	source, model := "manual", ""
	if req.Source == "machine" {
		source, model = "machine", strings.TrimSpace(req.Model)
	}

	if _, err := s.db.Exec(`
		INSERT INTO user_gloss (user_id, form, gloss, source, model)
		VALUES (?,?,?,?,?)
		ON CONFLICT(user_id, form) DO UPDATE SET
		  gloss=excluded.gloss, source=excluded.source, model=excluded.model`,
		u.ID, form, gloss, source, model); err != nil {
		return err
	}
	return c.JSON(GlossDTO{Form: form, Gloss: gloss, Source: source, Model: model})
}

func (s *Server) handleDeleteGloss(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	form, err := glossKey(c.Params("form"))
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(
		`DELETE FROM user_gloss WHERE user_id = ? AND form = ?`, u.ID, form); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
