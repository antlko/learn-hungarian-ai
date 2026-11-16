package api

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/llm"
	"tanulom/internal/sentfix"
	"tanulom/internal/srs"
)

// The AI Teacher's translation of one sentence, and the learner's choice to
// keep it. ADR-078.
//
// Two rules hold it together. Asking saves nothing: POST teacher only returns
// text, and only the learner's explicit press (PUT) writes a row. And what is
// kept is labelled: the row carries the model, and the crib prints it beside
// the text exactly as it does for the machine translator (ADR-014). The kept
// line replaces ONLY the line under the sentence — it mints no note, card,
// gloss or schedule state.

const (
	teacherTranslateTimeout = 45 * time.Second
	teacherTranslateTokens  = 300
	maxSentenceChars        = 1000
	maxOverrideChars        = 2000
	maxModelChars           = 200
)

// teacherTranslateSystem asks for a natural translation, not a literal one.
// The word that forbids chatter is deliberately not the one TUTOR.md bans
// from the tutor's prompts.
func teacherTranslateSystem(target string) string {
	lang := "English"
	if target == "ru" {
		lang = "Russian"
	}
	return "You translate one Hungarian sentence into " + lang + ". " +
		"Give the natural, idiomatic translation a fluent speaker would say: " +
		"an idiom or proverb is rendered by its real equivalent in " + lang +
		", never word for word. " +
		"Reply with the translated sentence and nothing else: no quotes, no notes, no explanation, no transliteration."
}

// cleanTranslation strips what chatty models wrap around a one-line answer.
func cleanTranslation(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "</think>"); i >= 0 {
		s = strings.TrimSpace(s[i+len("</think>"):])
	}
	s = strings.Trim(s, "\"'`«»“”„ \n\t")
	return strings.TrimSpace(s)
}

func parseSentenceTarget(target string) (string, error) {
	switch strings.TrimSpace(target) {
	case "ru":
		return "ru", nil
	case "en":
		return "en", nil
	}
	return "", badRequest("target must be ru or en")
}

func parseSentenceSource(hu string) (string, error) {
	hu = strings.TrimSpace(hu)
	if hu == "" {
		return "", badRequest("nothing to translate")
	}
	if utf8.RuneCountInString(hu) > maxSentenceChars {
		return "", badRequest("sentence too long: the limit is %d characters", maxSentenceChars)
	}
	return hu, nil
}

func (s *Server) handleTeacherTranslate(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req TeacherTranslateRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	hu, err := parseSentenceSource(req.HU)
	if err != nil {
		return err
	}
	target, err := parseSentenceTarget(req.Target)
	if err != nil {
		return err
	}
	client, err := llm.New(s.service("llm"), s.llmKey())
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, llmUnavailableKey(err))
	}
	ctx, cancel := context.WithTimeout(c.Context(), teacherTranslateTimeout)
	defer cancel()
	text, err := client.Chat(ctx, teacherTranslateSystem(target), hu, teacherTranslateTokens)
	if err != nil {
		slog.WarnContext(c.Context(), "teacher translation failed",
			"user", u.ID, "model", client.Model(), "error", err)
		return fiber.NewError(fiber.StatusBadGateway, "errors.teacherTranslateFailed")
	}
	text = cleanTranslation(text)
	if text == "" {
		return fiber.NewError(fiber.StatusBadGateway, "errors.teacherTranslateFailed")
	}
	return c.JSON(SentenceTranslationDTO{Text: text, Model: client.Model()})
}

func (s *Server) handleSaveSentenceTranslation(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req SaveSentenceTranslationRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	hu, err := parseSentenceSource(req.HU)
	if err != nil {
		return err
	}
	target, err := parseSentenceTarget(req.Target)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(req.Text)
	model := strings.TrimSpace(req.Model)
	if text == "" || utf8.RuneCountInString(text) > maxOverrideChars {
		return badRequest("translation must be 1-%d characters", maxOverrideChars)
	}
	// Always labelled: an unlabelled kept line would read as human work.
	if model == "" || utf8.RuneCountInString(model) > maxModelChars {
		return badRequest("model is required")
	}
	if err := s.db.PutSentenceOverride(u.ID, hashText(hu), hu, target, text, model); err != nil {
		return err
	}
	return c.JSON(SentenceTranslationDTO{Text: text, Model: model})
}

func (s *Server) handleDeleteSentenceTranslation(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	hu, err := parseSentenceSource(c.Query("hu"))
	if err != nil {
		return err
	}
	target, err := parseSentenceTarget(c.Query("target"))
	if err != nil {
		return err
	}
	if err := s.db.DeleteSentenceOverride(u.ID, hashText(hu), target); err != nil {
		return err
	}
	// Hand back what the crib shows now, so the screen needs no reload: the
	// pack's line (when the card has a pack sentence), the cache, the curated
	// fix — everything but the override just deleted.
	item := srs.QueueItem{HU: hu}
	// The client never learns a pack sentence id, so find it by text. A scan
	// of the pack's sentence table, once, on an explicit press.
	if s.pack != nil {
		var id int64
		err := s.pack.DB().QueryRow(
			`SELECT s.id FROM sentence s JOIN sentence_trans t ON t.sentence_id = s.id
			  WHERE s.hu = ? AND t.lang = ? LIMIT 1`, hu, target).Scan(&id)
		if err == nil {
			item.SentenceID = &id
		}
	}
	items := []srs.QueueItem{item}
	if err := s.attachSentenceTranslations(items); err != nil {
		return err
	}
	if err := s.attachCribCache(u.ID, items); err != nil {
		return err
	}
	if err := s.applySentenceFixes(u.ID, items, false); err != nil {
		return err
	}
	return c.JSON(OriginalSentenceDTO{
		SentenceRU: items[0].SentenceRU, SentenceEN: items[0].SentenceEN, SentenceModel: items[0].SentenceModel,
	})
}

// applySentenceFixes puts the learner's kept translation, or failing that the
// curated fix, in front of whatever the pack and the cache gave. Precedence:
// learner override > curated fix > pack > machine cache. ADR-078.
//
// Runs last in attachCribs, which every queue builder already calls after
// enrichFromPack, so the review queue, the practice queue and the tutor's card
// context all agree on the line.
func (s *Server) applySentenceFixes(userID int64, items []srs.QueueItem, withOverride bool) error {
	target := s.cribLang()
	for i := range items {
		it := &items[i]
		hu := strings.TrimSpace(it.HU)
		if hu == "" {
			continue
		}
		// Curated: human-written, both languages, no model label.
		for _, lang := range []string{"ru", "en"} {
			fix := sentfix.Lookup(hu, lang)
			if fix == "" {
				continue
			}
			if lang == "ru" {
				it.SentenceRU = fix
			} else {
				it.SentenceEN = fix
			}
			if lang == target {
				it.SentenceModel = ""
			}
		}
		if !withOverride {
			continue
		}
		// The learner's own choice beats it. Only the crib language: the model
		// label travels in one field and must describe the line shown.
		ov, err := s.db.SentenceOverrides(userID, hashText(hu))
		if err != nil {
			return err
		}
		if v, ok := ov[target]; ok {
			if target == "ru" {
				it.SentenceRU = v[0]
			} else {
				it.SentenceEN = v[0]
			}
			it.SentenceModel = v[1]
			it.SentenceKept = true
		}
	}
	return nil
}
