package api

import (
	"log/slog"
	"regexp"
	"strings"

	"tanulom/internal/mining"
	"tanulom/internal/srs"
)

// One-time, BEST-EFFORT backfill of note_origin for notes that predate the
// link table (ADR-022).
//
// A reader note's `hu` is the sentence the CLIENT assembled: every token of
// the sentence joined with single spaces, then whitespace stripped before
// `.,!?;:…` (ReaderView.sentenceOf). That is deliberately NOT the same join
// the server's sentence code uses — the two disagree around quotes and dashes
// — so the reconstruction below replicates the client's rule byte for byte and
// links only exact matches. Misses stay unlinked and simply do not appear
// under the text's pile; the docs say so. A sentence present in two texts
// links to both, which is the link table's own semantics.
var clientPunctJoin = regexp.MustCompile(`\s+([.,!?;:…])`)

func clientSentence(forms []string) string {
	return clientPunctJoin.ReplaceAllString(strings.Join(forms, " "), "$1")
}

func (s *Server) backfillNoteOriginsOnce() {
	const marker = "note_origin_backfilled"
	if _, done, err := s.db.Meta(marker); err != nil || done {
		return
	}

	linked, err := s.backfillNoteOrigins()
	if err != nil {
		slog.Error("note origin backfill failed; will retry next start", "error", err)
		return
	}
	slog.Info("note origins backfilled", "linked", linked)
	if err := s.db.SetMeta(marker, "v1"); err != nil {
		slog.Error("could not record note origin backfill", "error", err)
	}
}

func (s *Server) backfillNoteOrigins() (int, error) {
	users, err := s.db.ListUsers()
	if err != nil {
		return 0, err
	}
	notes := srs.NewStore(s.db.DB)
	texts := mining.NewStore(s.db.DB)

	linked := 0
	for _, u := range users {
		// Only reader-side notes can have a text behind them.
		rows, err := s.db.Query(`
			SELECT id, hu FROM note
			 WHERE user_id = ? AND source NOT LIKE 'course:%'`, u.ID)
		if err != nil {
			return linked, err
		}
		byHU := map[string][]int64{}
		for rows.Next() {
			var id int64
			var hu string
			if err := rows.Scan(&id, &hu); err != nil {
				rows.Close()
				return linked, err
			}
			byHU[hu] = append(byHU[hu], id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return linked, err
		}
		if len(byHU) == 0 {
			continue
		}

		list, err := texts.List(u.ID, 10_000)
		if err != nil {
			return linked, err
		}
		for _, txt := range list {
			tokens, err := texts.Tokens(u.ID, txt.ID)
			if err != nil {
				return linked, err
			}
			var forms []string
			sent := -1
			flush := func() {
				if len(forms) == 0 {
					return
				}
				for _, noteID := range byHU[clientSentence(forms)] {
					if err := notes.LinkNoteOrigin(noteID, txt.ID); err == nil {
						linked++
					}
				}
				forms = forms[:0]
			}
			for _, tk := range tokens {
				if tk.SentIdx != sent {
					flush()
					sent = tk.SentIdx
				}
				forms = append(forms, tk.Form)
			}
			flush()
		}
	}
	return linked, nil
}
