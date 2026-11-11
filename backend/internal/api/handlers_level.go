package api

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/llm"
	"tanulom/internal/mining"
)

// A text's CEFR level, three ways (ADR-023): the heuristic at ingest, an
// optional LLM second opinion, and the learner's own word — which always wins.
// Whoever answered is stored and shown; the label is the entire defence, the
// same rule ADR-014 set for machine translation.

// SetLevelRequest is the manual override.
type SetLevelRequest struct {
	Level string `json:"level"`
}

// handleSetLevel records the learner's own verdict.
func (s *Server) handleSetLevel(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return badRequest("text id must be a number")
	}
	var req SetLevelRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if !mining.ValidCEFR(req.Level) {
		return badRequest("level must be one of A1, A2, B1, B2, C1, C2")
	}
	store := mining.NewStore(s.db.DB)
	if err := store.SetLevel(u.ID, id, req.Level, "manual", ""); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "no such text")
	}
	text, err := store.Get(u.ID, id)
	if err != nil {
		return err
	}
	return c.JSON(text)
}

// llmLevelVerdict is the strict shape the model must answer in. Anything else
// is refused and the stored estimate stays untouched — a hallucinated band
// must not overwrite an honest heuristic.
type llmLevelVerdict struct {
	Level      string  `json:"level"`
	Confidence float64 `json:"confidence"`
	Rationale  string  `json:"rationale"`
}

// The prompt is versioned prose, not an inline improvisation: the template and
// its failure modes live in .claude/prompts/estimate-level.md per the project
// convention. Keep the two in step.
const levelSystemPrompt = `You judge the CEFR difficulty of Hungarian texts for learners. ` +
	`Answer with ONLY a JSON object: {"level":"A1|A2|B1|B2|C1|C2","confidence":0.0-1.0,"rationale":"one short sentence"}. ` +
	`No prose around it. Judge the language, not the topic.`

// handleLLMLevel asks the configured model for a second opinion.
func (s *Server) handleLLMLevel(c fiber.Ctx) error {
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

	client, err := llm.New(s.service("llm"), s.llmKey())
	if err != nil {
		// The same honest 503 shape every pack- or service-dependent route uses:
		// a real state, named, with the place to change it.
		return fiber.NewError(fiber.StatusServiceUnavailable, llmUnavailableKey(err))
	}

	excerpt := text.Body
	if len(excerpt) > 2000 {
		// A bounded, valid-UTF-8 excerpt; level does not need the whole article.
		cut := 2000
		for cut > 0 && excerpt[cut]&0xC0 == 0x80 {
			cut--
		}
		excerpt = excerpt[:cut]
	}
	user := "Title: " + text.Title + "\n\nText:\n" + excerpt
	if text.Level != "" {
		user += "\n\nA frequency-based heuristic estimated: " + text.Level
	}

	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	answer, err := client.Chat(ctx, levelSystemPrompt, user, 200)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, "errors.llmFailed")
	}

	verdict, ok := parseLevelVerdict(answer)
	if !ok || !mining.ValidCEFR(verdict.Level) {
		// Garbage in the answer leaves the stored estimate exactly as it was.
		return fiber.NewError(fiber.StatusBadGateway, "errors.llmFailed")
	}

	// The learner's own verdict is never overwritten by a machine's.
	if text.LevelSource == "manual" {
		return badRequest("errors.levelManual")
	}

	detail, _ := json.Marshal(map[string]any{
		"model":      client.Model(),
		"confidence": verdict.Confidence,
		"rationale":  verdict.Rationale,
	})
	if err := store.SetLevel(u.ID, id, verdict.Level, "llm", string(detail)); err != nil {
		return err
	}
	text, err = store.Get(u.ID, id)
	if err != nil {
		return err
	}
	return c.JSON(text)
}

// parseLevelVerdict digs the JSON object out of the answer — models love to
// wrap it in prose or fences however firmly told not to.
func parseLevelVerdict(answer string) (llmLevelVerdict, bool) {
	var v llmLevelVerdict
	start := strings.IndexByte(answer, '{')
	end := strings.LastIndexByte(answer, '}')
	if start < 0 || end <= start {
		return v, false
	}
	if err := json.Unmarshal([]byte(answer[start:end+1]), &v); err != nil {
		return v, false
	}
	return v, true
}

// llmKey reads the stored API key. Settings are conceptually GLOBAL — PUT
// /api/settings is admin-only — but secrets are stored per admin row, so the
// first stored key is the installation's key. Empty is fine: local endpoints
// need none. The value never enters a DTO or a log.
func (s *Server) llmKey() string {
	var key string
	_ = s.db.QueryRow(`
		SELECT value FROM settings
		 WHERE key = 'llmKey' AND secret = 1 AND value != '' LIMIT 1`).Scan(&key)
	return key
}
