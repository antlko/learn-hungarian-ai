package api

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"tanulom/internal/mt"
	"tanulom/internal/srs"
)

// The crib: what a reader's own sentence means, on the card that blanks a word
// out of it.
//
// A note mined from the learner's reading has no pack sentence behind it
// (`sentence_id` is NULL), so `attachSentenceTranslations` — which joins the
// pack's human translations — has nothing to give it. The sentence came out of
// a text the learner chose, and only the MT sidecar can say what it means.
//
// ADR-014 forbids machine translation from filling a note's gloss, a card's
// ANSWER, or anything graded. A crib is none of those: the graded string is the
// Hungarian target form, and the crib is prompt context printed beside it,
// carrying the name of the model that wrote it. The label is the whole defence,
// because a wrong translation is perfectly well-formed and no attestation can
// catch it. See ADR-018.

// cribTimeout bounds one background translation flight. Generous, because this
// is off the response path and a cold 600M model on a CPU is not quick.
const cribTimeout = 2 * time.Minute

// attachCribs fills the sentence translation for notes the pack cannot cover.
//
// Reads the CACHE and nothing else. The sidecar is never called here: the
// review queue is the app's highest-frequency screen and it has to build with
// every service stopped, uninstalled, or halfway through downloading its
// weights. ADR-014 already established that a cached answer is served without
// the model running; this is the same promise on a different screen.
func (s *Server) attachCribs(userID int64, items []srs.QueueItem) error {
	target := s.cribLang()
	for i := range items {
		it := &items[i]
		// A pack sentence has a human translation or none; either way it is
		// not the crib's business.
		if it.SentenceID != nil || strings.TrimSpace(it.HU) == "" {
			continue
		}
		if it.SentenceRU != "" || it.SentenceEN != "" {
			continue
		}
		text, model, err := s.db.CachedTranslation(userID, hashText(it.HU), target)
		if err != nil {
			return err
		}
		if text == "" {
			continue
		}
		if target == "ru" {
			it.SentenceRU = text
		} else {
			it.SentenceEN = text
		}
		it.SentenceModel = model
	}
	return nil
}

// cribLang is the language a crib is written in.
//
// Locale.UI, never Locale.Gloss: `gloss` is allowed to be "both", which is not
// a translation target and which mt.Supported rejects. The setup wizard writes
// exactly that value, so deriving the target from it would 400 on a default
// install.
func (s *Server) cribLang() string {
	if s.Config().Locale.UI == "en" {
		return "en"
	}
	return "ru"
}

// warmCribs translates, in the background, the sentences a queue had no crib
// for.
//
// Off the response path on purpose. The sidecar answers a sentence in a second
// or two on a CPU, so a cold queue of forty would hold the review screen for a
// minute — and the app has to stay fully usable with every sidecar stopped. The
// card renders without its crib and gains one on the next load, which is also
// exactly what happens when MT is off: one behaviour to reason about, not two.
//
// One flight per learner at a time, so two queue loads in quick succession do
// not translate the same forty sentences twice.
func (s *Server) warmCribs(userID int64, items []srs.QueueItem) {
	client := mt.New(s.service("mt"))
	if client == nil {
		return
	}
	target := s.cribLang()

	seen := map[string]bool{}
	var texts []string
	var chars int
	for _, it := range items {
		if it.SentenceID != nil || it.SentenceRU != "" || it.SentenceEN != "" {
			continue
		}
		hu := strings.TrimSpace(it.HU)
		if hu == "" || seen[hu] {
			continue
		}
		// The same two bounds POST /api/translate enforces. Forty sentences is
		// a batch; forty PARAGRAPHS is a request the sidecar should never be
		// handed, and a learner may paste anything into the reader.
		if chars+len(hu) > maxTranslateChars {
			break
		}
		seen[hu] = true
		chars += len(hu)
		texts = append(texts, hu)
		if len(texts) == maxTranslateTexts {
			break
		}
	}
	if len(texts) == 0 {
		return
	}
	if _, busy := s.cribFlights.LoadOrStore(userID, true); busy {
		return
	}
	// Tracked so Close can wait: a goroutine still writing to s.db after the
	// database is closed fails the race detector and, in production, logs an
	// error nobody asked for.
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		defer s.cribFlights.Delete(userID)
		// NOT the request context: Fiber releases it the moment the handler
		// returns, and every call made with it would be cancelled instantly.
		// Rooted in the server's background context so a shutdown cancels the
		// call rather than waiting it out.
		ctx, cancel := context.WithTimeout(s.bgCtx, cribTimeout)
		defer cancel()

		res, err := client.Translate(ctx, texts, target)
		if err != nil {
			// The endpoint being wrong, the container being down and the model
			// still loading are one thing from here, and all of them are "not
			// now" rather than "broken".
			slog.WarnContext(ctx, "crib warm failed", "err", err, "n", len(texts))
			return
		}
		for i, text := range texts {
			if i >= len(res.Texts) {
				break
			}
			if err := s.db.PutTranslation(userID, hashText(text), text, target, res.Texts[i], res.Model); err != nil {
				slog.WarnContext(ctx, "crib cache write failed", "err", err)
				return
			}
		}
	}()
}
