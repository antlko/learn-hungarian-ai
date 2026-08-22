package api

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/config"
	"tanulom/internal/mining"
	"tanulom/internal/morph"
	"tanulom/internal/srs"
)

// The reading library.
//
// Texts are analysed once on ingest and stored token by token. Coverage,
// however, is recomputed on every read: it is a moving target that improves as
// the learner does, and a frozen number would quietly go stale.

// CreateTextRequest ingests something to read.
type CreateTextRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	// Kind selects the cleaner: paste | srt | html.
	Kind      string `json:"kind"`
	SourceURL string `json:"sourceUrl"`
}

func (s *Server) handleCreateText(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requirePack(); err != nil {
		return err
	}
	var req CreateTextRequest
	if err := decode(c, &req); err != nil {
		return err
	}

	var in mining.Ingested
	switch req.Kind {
	case "srt":
		in, err = mining.FromSRT(req.Title, req.Body)
	case "html", "url":
		in, err = mining.FromHTML(req.Title, req.Body)
	default:
		in, err = mining.FromPaste(req.Title, req.Body)
	}
	if err != nil {
		return badRequest("%s", err.Error())
	}

	tokens, sentences, err := s.analyzer.AnalyzeText(in.Body)
	if err != nil {
		return err
	}
	stored := toStoredTokens(tokens, sentences)

	cov, err := s.coverageFor(u.ID, mining.AsTokenViews(stored))
	if err != nil {
		return err
	}

	id, err := mining.NewStore(s.db.DB).Create(u.ID, in, req.SourceURL, stored, cov)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id": id, "title": in.Title, "kind": in.Kind, "coverage": cov,
	})
}

func (s *Server) handleListTexts(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	texts, err := mining.NewStore(s.db.DB).List(u.ID, 200)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"texts": texts})
}

// TextDetail is a text with its tokens and a fresh coverage report.
type TextDetail struct {
	Text     mining.Text          `json:"text"`
	Tokens   []mining.StoredToken `json:"tokens"`
	Coverage mining.Coverage      `json:"coverage"`
	Verdict  mining.Verdict       `json:"verdict"`
	// Known maps lemma id -> status, so the reader can colour every token
	// without a request per word.
	Known map[string]string `json:"known"`
	// Thresholds are sent so the UI can draw the lines without hardcoding them,
	// and label them honestly.
	Thresholds map[string]float64 `json:"thresholds"`
}

func (s *Server) handleGetText(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return badRequest("text id must be a number")
	}

	store := mining.NewStore(s.db.DB)
	text, err := store.Get(u.ID, id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "no such text")
	}
	tokens, err := store.Tokens(u.ID, id)
	if err != nil {
		return err
	}

	cov, err := s.coverageFor(u.ID, mining.AsTokenViews(tokens))
	if err != nil {
		return err
	}
	// Coverage improves as the learner does; keep the library figure current.
	_ = store.UpdateCoverage(u.ID, id, cov)
	_ = store.MarkRead(u.ID, id, time.Now())

	known, err := srs.NewStore(s.db.DB).KnownLemmaIDs(u.ID)
	if err != nil {
		return err
	}
	knownStr := make(map[string]string, len(known))
	for lemmaID, status := range known {
		knownStr[strconv.FormatInt(lemmaID, 10)] = status
	}

	return c.JSON(TextDetail{
		Text: text, Tokens: tokens, Coverage: cov, Verdict: cov.Judge(),
		Known: knownStr,
		Thresholds: map[string]float64{
			"minimal":    mining.MinimalComprehension,
			"unassisted": mining.UnassistedComprehension,
			"listening":  mining.ListeningFloor,
		},
	})
}

func (s *Server) handleDeleteText(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return badRequest("text id must be a number")
	}
	if err := mining.NewStore(s.db.DB).Delete(u.ID, id); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "no such text")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// handleNextSentences returns the n+1 rail for a text.
func (s *Server) handleNextSentences(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return badRequest("text id must be a number")
	}

	tokens, err := mining.NewStore(s.db.DB).Tokens(u.ID, id)
	if err != nil {
		return err
	}
	ctx, params, err := s.miningContext(u.ID)
	if err != nil {
		return err
	}

	limit := 10
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}
	ranked := mining.Rank(mining.SentenceCandidates(tokens), ctx, params, limit)
	if ranked == nil {
		ranked = []mining.Scored{}
	}
	return c.JSON(fiber.Map{"sentences": ranked})
}

// SetKnownRequest records a learner declaration about a word.
type SetKnownRequest struct {
	LemmaID int64  `json:"lemmaId"`
	Status  string `json:"status"`
}

// handleSetKnown records "I know this" / "ignore this" from the reader.
//
// A declaration deliberately creates no cards: marking words known while reading
// must not manufacture review load, or the reader becomes a trap the learner
// learns to avoid.
func (s *Server) handleSetKnown(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req SetKnownRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if req.LemmaID == 0 {
		return badRequest("lemmaId is required")
	}
	if err := srs.NewStore(s.db.DB).SetLemmaStatus(u.ID, req.LemmaID, req.Status); err != nil {
		return badRequest("%s", err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ── helpers ───────────────────────────────────────────────────────────────

func toStoredTokens(tokens []morph.AnalyzedToken, sentences []morph.Sentence) []mining.StoredToken {
	sentOf := make(map[int]int, len(tokens))
	for si, s := range sentences {
		for i := s.Start; i < s.End; i++ {
			sentOf[i] = si
		}
	}
	out := make([]mining.StoredToken, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, mining.StoredToken{
			Index: t.Index, SentIdx: sentOf[t.Index], Form: t.Text,
			Start: t.Start, End: t.End, IsWord: t.IsWord,
			LemmaID: t.Analysis.LemmaID, Feats: t.Analysis.FeatureID,
			Certainty: string(t.Analysis.Certainty),
		})
	}
	return out
}

// knownFeatureFn builds a predicate over the learner's morphological state.
//
// Loaded once per request rather than queried per token: a long article has
// thousands of tokens and only a few dozen distinct features.
func (s *Server) knownFeatureFn(userID int64, threshold float64) (func(string) bool, error) {
	engine := s.engine()
	mastery, err := engine.Mastery(userID, s.Config().Drill.HarmonyLatencyMs, time.Now())
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(mastery))
	for _, m := range mastery {
		known[m.FeatureID] = m.Retrievability >= threshold
	}
	return func(f string) bool { return known[f] }, nil
}

func (s *Server) coverageFor(userID int64, views []mining.TokenView) (mining.Coverage, error) {
	known, err := srs.NewStore(s.db.DB).KnownLemmaIDs(userID)
	if err != nil {
		return mining.Coverage{}, err
	}
	knownFeat, err := s.knownFeatureFn(userID, s.Config().Mining.KnownThreshold)
	if err != nil {
		return mining.Coverage{}, err
	}
	return mining.Compute(views, known, knownFeat), nil
}

func (s *Server) miningContext(userID int64) (mining.Context, mining.Params, error) {
	cfg := s.Config()

	known, err := srs.NewStore(s.db.DB).KnownLemmaIDs(userID)
	if err != nil {
		return mining.Context{}, mining.Params{}, err
	}
	knownFeat, err := s.knownFeatureFn(userID, cfg.Mining.KnownThreshold)
	if err != nil {
		return mining.Context{}, mining.Params{}, err
	}

	ctx := mining.Context{
		KnownLemma: func(id int64) bool {
			st := known[id]
			return st == "known" || st == "learning" || st == "ignored"
		},
		KnownFeature:  knownFeat,
		FrequencyRank: s.frequencyRankFn(),
	}
	return ctx, miningParams(cfg), nil
}

// frequencyRankFn looks a lemma's rank up in the pack, with a small cache: the
// scorer asks about the same handful of unknown lemmas repeatedly.
func (s *Server) frequencyRankFn() func(int64) int {
	cache := map[int64]int{}
	return func(id int64) int {
		if r, ok := cache[id]; ok {
			return r
		}
		var rank *int
		if s.pack != nil {
			_ = s.pack.DB().QueryRow(`SELECT freq_rank FROM lemma WHERE id = ?`, id).Scan(&rank)
		}
		r := 0
		if rank != nil {
			r = *rank
		}
		cache[id] = r
		return r
	}
}

func miningParams(cfg config.Config) mining.Params {
	return mining.Params{
		KnownThreshold: cfg.Mining.KnownThreshold,
		FeatureWeight:  cfg.Mining.FeatureWeight,
		TargetUnknown:  cfg.Mining.TargetUnknown,
		UnknownSigma:   cfg.Mining.UnknownSigma,
		MinCoverage:    cfg.Mining.MinCoverage,
		MinLength:      cfg.Mining.TargetLength[0],
		MaxLength:      cfg.Mining.TargetLength[1],
		Weights: mining.Weights{
			SweetSpot: cfg.Mining.Weights.SweetSpot,
			Coverage:  cfg.Mining.Weights.Coverage,
			Utility:   cfg.Mining.Weights.Utility,
			Relevance: cfg.Mining.Weights.Relevance,
			Length:    cfg.Mining.Weights.Length,
			Duplicate: cfg.Mining.Weights.Duplicate,
		},
	}
}
