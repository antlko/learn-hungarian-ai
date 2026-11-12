package api

import (
	"log/slog"

	"tanulom/internal/mining"
)

// Keeping imported sets current with the pack (ADR-074).
//
// A set a learner imported is an ordinary text whose source_url is
// "set:<code>". When a newer pack rewrites that set (the «Фразы» of a unit
// became one connected story), the learner's copy would otherwise stay the old
// body forever. Users cannot edit a text's body, so "the body equals what the
// pack would import now" is the whole version check: no hash column, no
// migration. A differing copy is REPLACED IN PLACE (mining.Store.Replace): the
// id, dates and the notes mined from it stay; tokens, coverage, level and
// audio are rebuilt. A code the pack no longer knows is left untouched — this
// code never deletes the learner's data.

// syncSetTextsOnce runs the sync off the boot path. The pack is only swapped
// on restart, so boot is the one moment that matters.
func (s *Server) syncSetTextsOnce() {
	if s.pack == nil || s.analyzer == nil {
		return
	}
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		replaced, skipped, failed := s.syncSetTexts()
		if replaced+failed > 0 {
			slog.Info("reading-set texts synced with the pack",
				"replaced", replaced, "skipped", skipped, "failed", failed)
		}
	}()
}

// syncSetTexts rebuilds every user's set-sourced text from the current pack.
// Idempotent: a second run replaces nothing.
func (s *Server) syncSetTexts() (replaced, skipped, failed int) {
	if s.pack == nil || s.analyzer == nil || !s.hasReadingSets() {
		return 0, 0, 0
	}
	users, err := s.db.ListUsers()
	if err != nil {
		slog.Error("set sync: list users", "error", err)
		return 0, 0, 1
	}
	store := mining.NewStore(s.db.DB)
	bodies := map[string]struct {
		title, body, kind string
		ok                bool
	}{}
	for _, u := range users {
		rows, err := s.db.Query(`
			SELECT id, source_url, title, body FROM text
			 WHERE user_id = ? AND source_url LIKE 'set:%'`, u.ID)
		if err != nil {
			slog.Error("set sync: list texts", "user", u.ID, "error", err)
			failed++
			continue
		}
		type row struct {
			id                   int64
			code, title, curBody string
		}
		var texts []row
		for rows.Next() {
			var r row
			var src string
			if err := rows.Scan(&r.id, &src, &r.title, &r.curBody); err == nil {
				r.code = src[len(setSourcePrefix):]
				texts = append(texts, r)
			}
		}
		rows.Close()

		for _, r := range texts {
			if s.bgCtx.Err() != nil {
				return
			}
			b, seen := bodies[r.code]
			if !seen {
				title, body, kind, ok, err := s.setBody(r.code)
				if err != nil {
					slog.Error("set sync: build body", "code", r.code, "error", err)
					failed++
					continue
				}
				b = struct {
					title, body, kind string
					ok                bool
				}{title, body, kind, ok}
				bodies[r.code] = b
			}
			if !b.ok || (b.body == r.curBody && b.title == r.title) {
				skipped++
				continue
			}
			if err := s.replaceSetText(store, u.ID, r.id, b.title, b.body, b.kind); err != nil {
				slog.Error("set sync: replace", "text", r.id, "code", r.code, "error", err)
				failed++
				continue
			}
			replaced++
		}
	}
	return replaced, skipped, failed
}

func (s *Server) replaceSetText(store *mining.Store, userID, textID int64, title, body, kind string) error {
	tokens, sentences, err := s.analyzer.AnalyzeText(body)
	if err != nil {
		return err
	}
	stored := toStoredTokens(tokens, sentences)
	cov, err := s.coverageFor(userID, mining.AsTokenViews(stored))
	if err != nil {
		return err
	}
	est := mining.EstimateLevel(mining.AsTokenViews(stored), s.levelInputs())
	return store.Replace(userID, textID, title, body, kind, stored, cov, est.Level, est.Detail())
}
