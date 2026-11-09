package api

import (
	"strconv"
	"strings"

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

// AnalyzedTokenDTO is a token plus its segmentation.
//
// Segments ride along on every word token: the map from surface to morphemes is
// the product thesis made visible, and the reader colours by it. Segmentation
// is cached per distinct form inside AnalyzeText's own cache, so the cost is
// per-vocabulary, not per-token.
type AnalyzedTokenDTO struct {
	morph.AnalyzedToken
	Segments []morph.Segment `json:"segments,omitempty"`
}

// AnalyzeResponse carries the analysed tokens plus sentence boundaries.
type AnalyzeResponse struct {
	Tokens    []AnalyzedTokenDTO `json:"tokens"`
	Sentences []morph.Sentence   `json:"sentences"`
	// Coverage is the share of word tokens that resolved to a known lemma. It is
	// the honest headline number for "can I read this".
	Coverage  float64 `json:"coverage"`
	WordCount int     `json:"wordCount"`
}

const maxAnalyzeBytes = 200_000

func (s *Server) requirePack() error {
	if s.analyzer == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable,
			"errors.packMissing")
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
	s.applyNLPHints(c.Context(), analyzeHintsTimeout, req.Text, tokens)

	segCache := map[string][]morph.Segment{}
	out := make([]AnalyzedTokenDTO, 0, len(tokens))
	words, known := 0, 0
	for _, t := range tokens {
		dto := AnalyzedTokenDTO{AnalyzedToken: t}
		if t.IsWord {
			words++
			if t.Analysis.Certainty != morph.UnknownAnalysis {
				known++
			}
			// Keyed by lemma too: a context-chosen reading can differ from the
			// default one for the same surface form.
			key := strings.ToLower(t.Text) + "|" + strconv.FormatInt(t.Analysis.LemmaID, 10)
			segs, ok := segCache[key]
			if !ok {
				segs = s.analyzer.SegmentForm(t.Text, t.Analysis)
				segCache[key] = segs
			}
			dto.Segments = segs
		}
		out = append(out, dto)
	}
	coverage := 0.0
	if words > 0 {
		coverage = float64(known) / float64(words)
	}

	return c.JSON(AnalyzeResponse{
		Tokens: out, Sentences: sentences,
		Coverage: coverage, WordCount: words,
	})
}

// BreakdownResponse is the full explanation of ONE form, for the reader's
// detail rail and the post-answer explanation.
type BreakdownResponse struct {
	Form     string              `json:"form"`
	Analysis morph.Analysis      `json:"analysis"`
	Segments []morph.Segment     `json:"segments"`
	Explain  []morph.ExplainLine `json:"explain"`
}

// handleBreakdown decomposes one form and explains each piece in both locales.
// This is [R4]'s "explicit instruction in miniature" as an endpoint.
func (s *Server) handleBreakdown(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requirePack(); err != nil {
		return err
	}
	form := strings.TrimSpace(c.Query("form"))
	if form == "" {
		return badRequest("form is required")
	}
	if len(form) > 100 {
		return badRequest("form is too long")
	}
	an, err := s.analyzer.Best(form)
	if err != nil {
		return err
	}
	// The learner's own correction overlays the pack's reading here — this is
	// the endpoint behind the detail rail, which is where a wrong analysis is
	// noticed and where the override must therefore be visible.
	if lemma, feats, ok := s.correctionFor(u.ID, form); ok {
		an = applyCorrection(an, lemma, feats)
	}
	segs := s.analyzer.SegmentForm(form, an)
	return c.JSON(BreakdownResponse{
		Form: form, Analysis: an, Segments: segs, Explain: s.analyzer.Explain(segs),
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
	// ParadigmDerived marks the cells our own generator produced rather than the
	// dictionary. The UI renders those differently: a generated form is a good
	// guess, and a learner is entitled to know which is which.
	ParadigmDerived map[string]bool `json:"paradigmDerived,omitempty"`
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

	// Russian glosses cover roughly 45% of the starter pack's lemmas and all
	// 853 course words; English covers the rest. A learner who asked for
	// Russian and hits one of the gaps sees English WITH a badge rather than
	// nothing, and never English silently presented as Russian.
	want := s.Config().Locale.Gloss
	if want != "both" && len(dto.Glosses[want]) == 0 && len(dto.Glosses) > 0 {
		dto.GlossFallback = true
	}

	// The paradigm comes from the lookup index, not from the pack's precomputed
	// `paradigm` table, because that table is generator-only: for `ház` it says
	// the superessive is `házan` while `házon` sits attested in `wordform` right
	// beside it. A wrong cell in the table a learner reads is a wrong form
	// actively taught. See internal/morph/forms.go and, for future packs,
	// langpack/pipeline/build.py.
	paradigm, attested, err := s.analyzer.Paradigm(id)
	if err != nil {
		return err
	}
	dto.Paradigm = paradigm
	dto.ParadigmDerived = map[string]bool{}
	for slot := range paradigm {
		if !attested[slot] {
			dto.ParadigmDerived[slot] = true
		}
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
