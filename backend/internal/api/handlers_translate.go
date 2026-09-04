package api

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/mt"
)

// Sentence translation for the reader.
//
// This is the one place in the app that puts machine-written text in front of
// the learner, so it is the one place that has to be loudest about it: every
// row it returns carries the model that produced it, and the client renders
// that beside the translation rather than in a tooltip. See ADR-014.
//
// It is also the only feature whose absence is NORMAL. `services.mt` is off on
// a fresh install and most learners will never turn it on, so the reader must
// look complete without it — the key hint does not mention `t` and the button
// is not drawn when the capability is unreachable.

// Batch limits. Refused with the numbers rather than silently truncated: a
// learner who asked for a paragraph and got four sentences would reasonably
// conclude the rest of the paragraph translated to nothing.
const (
	maxTranslateTexts = 40
	maxTranslateChars = 8000
)

func (s *Server) handleTranslate(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req TranslateRequest
	if err := decode(c, &req); err != nil {
		return err
	}

	target := strings.TrimSpace(req.Target)
	if target == "" {
		target = s.Config().Locale.Gloss
	}
	if !mt.Supported(target) {
		return badRequest("target %q is not a language this translator has", target)
	}

	texts := make([]string, 0, len(req.Texts))
	total := 0
	for _, t := range req.Texts {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		texts = append(texts, t)
		total += len(t)
	}
	if len(texts) == 0 {
		return badRequest("nothing to translate")
	}
	if len(texts) > maxTranslateTexts {
		return badRequest("too much at once: %d sentences, the limit is %d", len(texts), maxTranslateTexts)
	}
	if total > maxTranslateChars {
		return badRequest("too much at once: %d characters, the limit is %d", total, maxTranslateChars)
	}

	out := make([]TranslationDTO, len(texts))
	// Position in `out` of every text still needing the service, keyed by the
	// text itself: a paragraph repeats "Igen." and asking twice would cost two
	// model runs for one answer.
	misses := map[string][]int{}
	var order []string

	for i, text := range texts {
		out[i] = TranslationDTO{Source: text}
		hit, model, err := s.db.CachedTranslation(u.ID, hashText(text), target)
		if err != nil {
			return err
		}
		if hit != "" {
			out[i].Text, out[i].Model, out[i].Cached = hit, model, true
			continue
		}
		if _, seen := misses[text]; !seen {
			order = append(order, text)
		}
		misses[text] = append(misses[text], i)
	}

	if len(order) == 0 {
		// Fully cached. Deliberately answered even with the sidecar stopped:
		// the learner already paid for these and nothing here needs the model.
		return c.JSON(TranslateResponse{Translations: out})
	}

	client := mt.New(s.Config().Services.MT)
	if client == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "translation.notConfigured")
	}

	res, err := client.Translate(c.Context(), order, target)
	if err != nil {
		// The endpoint being wrong, the container being down and the model
		// still loading are all the same thing from here, and all of them are
		// "not now" rather than "broken forever".
		slog.WarnContext(c.Context(), "translation failed", "err", err, "target", target, "n", len(order))
		return fiber.NewError(fiber.StatusServiceUnavailable, "translation.unreachable")
	}

	for n, text := range order {
		translated := res.Texts[n]
		for _, i := range misses[text] {
			out[i].Text, out[i].Model = translated, res.Model
		}
		// A cache write that fails is not worth failing the request over: the
		// learner has their translation, and the only cost is asking again.
		if err := s.db.PutTranslation(u.ID, hashText(text), text, target, translated, res.Model); err != nil {
			slog.WarnContext(c.Context(), "translation cache write failed", "err", err)
		}
	}
	return c.JSON(TranslateResponse{Translations: out})
}

func hashText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
