package api

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/morph"
)

// Morphological analysis endpoints.
//
// These are what make the reader and the review diff possible, and they are the
// clearest demonstration of the product thesis: a form comes back decomposed
// into a lemma plus the individual grammatical facts it carries, each of which
// is separately learnable and separately scheduled.

// AnalyzeRequest is a chunk of Hungarian text.
type AnalyzeRequest struct {
	Text string `json:"text"`
}

// AnalyzeResponse carries the analysed tokens plus sentence boundaries.
type AnalyzeResponse struct {
	Tokens    []morph.AnalyzedToken `json:"tokens"`
	Sentences []morph.Sentence      `json:"sentences"`
	// Coverage is the share of word tokens that resolved to a known lemma. It is
	// the honest headline number for "can I read this".
	Coverage  float64 `json:"coverage"`
	WordCount int     `json:"wordCount"`
}

const maxAnalyzeBytes = 200_000

func (s *Server) requirePack() error {
	if s.analyzer == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable,
			"no language pack is installed — download one from Settings → Data")
	}
	return nil
}

func (s *Server) handleAnalyze(c fiber.Ctx) error {
	if err := s.requirePack(); err != nil {
		return err
	}
	var req AnalyzeRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if req.Text == "" {
		return badRequest("text is required")
	}
	if len(req.Text) > maxAnalyzeBytes {
		return badRequest("text is too long (%d bytes, limit %d)", len(req.Text), maxAnalyzeBytes)
	}

	tokens, sentences, err := s.analyzer.AnalyzeText(req.Text)
	if err != nil {
		return err
	}

	words, known := 0, 0
	for _, t := range tokens {
		if !t.IsWord {
			continue
		}
		words++
		if t.Analysis.Certainty != morph.UnknownAnalysis {
			known++
		}
	}
	coverage := 0.0
	if words > 0 {
		coverage = float64(known) / float64(words)
	}

	return c.JSON(AnalyzeResponse{
		Tokens: tokens, Sentences: sentences,
		Coverage: coverage, WordCount: words,
	})
}

// LemmaDTO is a dictionary entry with its paradigm.
type LemmaDTO struct {
	ID       int64               `json:"id"`
	Lemma    string              `json:"lemma"`
	UPOS     string              `json:"upos"`
	IPA      string              `json:"ipa,omitempty"`
	FreqRank *int                `json:"freqRank,omitempty"`
	CEFR     string              `json:"cefr,omitempty"`
	Glosses  map[string][]string `json:"glosses"`
	// GlossFallback marks that the requested language had no gloss and another
	// was substituted. Never silent — see ADR-004.
	GlossFallback bool              `json:"glossFallback"`
	Paradigm      map[string]string `json:"paradigm,omitempty"`
}

func (s *Server) handleLemma(c fiber.Ctx) error {
	if err := s.requirePack(); err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return badRequest("lemma id must be a number")
	}

	dto := LemmaDTO{ID: id, Glosses: map[string][]string{}}
	var ipa, cefr *string
	var rank *int
	row := s.pack.DB().QueryRow(
		`SELECT lemma, upos, ipa, cefr, freq_rank FROM lemma WHERE id = ?`, id)
	if err := row.Scan(&dto.Lemma, &dto.UPOS, &ipa, &cefr, &rank); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "no such lemma")
	}
	if ipa != nil {
		dto.IPA = *ipa
	}
	if cefr != nil {
		dto.CEFR = *cefr
	}
	dto.FreqRank = rank

	rows, err := s.pack.DB().Query(
		`SELECT lang, gloss FROM lemma_gloss WHERE lemma_id = ? ORDER BY lang, sense_idx`, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var lg, g string
		if err := rows.Scan(&lg, &g); err != nil {
			return err
		}
		dto.Glosses[lg] = append(dto.Glosses[lg], g)
	}

	// The pack currently ships English glosses only; a learner who asked for
	// Russian sees English WITH a badge rather than nothing, and never English
	// silently presented as Russian.
	want := s.Config().Locale.Gloss
	if want != "both" && len(dto.Glosses[want]) == 0 && len(dto.Glosses) > 0 {
		dto.GlossFallback = true
	}

	prows, err := s.pack.DB().Query(
		`SELECT slot, form FROM paradigm WHERE lemma_id = ? ORDER BY slot`, id)
	if err != nil {
		return err
	}
	defer prows.Close()
	dto.Paradigm = map[string]string{}
	for prows.Next() {
		var slot, form string
		if err := prows.Scan(&slot, &form); err != nil {
			return err
		}
		dto.Paradigm[slot] = form
	}
	return c.JSON(dto)
}

// FeatureDTO is one teachable morphological atom.
type FeatureDTO struct {
	ID         string            `json:"id"`
	Key        string            `json:"key"`
	Value      string            `json:"value"`
	LabelRU    string            `json:"labelRu"`
	LabelEN    string            `json:"labelEn"`
	Allomorphs map[string]string `json:"allomorphs"`
	ExplainRU  string            `json:"explainRu"`
	ExplainEN  string            `json:"explainEn"`
	TeachOrder int               `json:"teachOrder"`
	Grid       string            `json:"grid,omitempty"`
}

func (s *Server) handleFeatures(c fiber.Ctx) error {
	if err := s.requirePack(); err != nil {
		return err
	}
	rows, err := s.pack.DB().Query(`
		SELECT id, key, value, label_ru, label_en, allomorphs,
		       explain_ru, explain_en, teach_order, COALESCE(grid, '')
		  FROM morph_feature ORDER BY teach_order`)
	if err != nil {
		return err
	}
	defer rows.Close()

	out := []FeatureDTO{}
	for rows.Next() {
		var f FeatureDTO
		var allo string
		if err := rows.Scan(&f.ID, &f.Key, &f.Value, &f.LabelRU, &f.LabelEN,
			&allo, &f.ExplainRU, &f.ExplainEN, &f.TeachOrder, &f.Grid); err != nil {
			return err
		}
		f.Allomorphs = map[string]string{}
		_ = jsonUnmarshalString(allo, &f.Allomorphs)
		out = append(out, f)
	}
	return c.JSON(out)
}
