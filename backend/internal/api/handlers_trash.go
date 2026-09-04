package api

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/srs"
)

// The trash.
//
// «Слишком сложно» and «карточка плохая» are verdicts a learner is entitled to
// reach about an item, and until now the app had no way to hear either one: a
// bad card could be failed forever or left to sit at the front of every
// session. Both answers train the learner to distrust the queue.
//
// What the button promises, and therefore what these handlers must not break:
//
//   - the word LEAVES the queue, every direction of it, immediately;
//   - nothing is deleted — the note, its cards, their FSRS state and every row
//     in review_log survive, which is what makes restore a restore;
//   - it stays reachable. Free practice accepts `scope=trash`, so a set-aside
//     word can still be looked at, and practice schedules nothing (ADR-019) so
//     looking at it moves nothing. The review queue REFUSES that scope: a trash
//     that fed the scheduler would be a pile, not a trash.
//
// Every response carries fresh Breakdowns, because trashing changes what the
// tab strip owes and ADR-021 says those numbers come from one place. The
// alternative — the client subtracting one from a count it is holding — is how
// two numbers on one screen start disagreeing.

// TrashResponse reports the trash after a change, in the numbers every other
// review screen speaks.
type TrashResponse struct {
	NoteID int64 `json:"noteId"`
	// Cards is how many directions moved with the word — what the learner just
	// took out of (or put back into) the day's workload.
	Cards int `json:"cards"`
	// Changed is false when the note was already where the call would put it.
	// A second click is not an error; reporting one would make a double tap
	// look like a failure.
	Changed bool `json:"changed"`
	// Trashed is how many words are set aside now.
	Trashed int             `json:"trashed"`
	Scopes  []srs.Breakdown `json:"scopes"`
}

// TrashListResponse is the trash screen.
type TrashListResponse struct {
	Notes []srs.TrashedNote `json:"notes"`
	Total int               `json:"total"`
}

func (s *Server) handleTrashNote(c fiber.Ctx) error {
	return s.moveTrash(c, true)
}

func (s *Server) handleRestoreNote(c fiber.Ctx) error {
	return s.moveTrash(c, false)
}

// moveTrash is both directions, because they differ in one verb and in nothing
// else — and the response contract, which is the part that must not drift, is
// entirely shared.
func (s *Server) moveTrash(c fiber.Ctx, trash bool) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	noteID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || noteID <= 0 {
		return badRequest("note id must be a positive integer")
	}

	store := srs.NewStore(s.db.DB)
	// 404 on a note that is not the caller's, rather than the silent no-op an
	// `UPDATE ... WHERE user_id = ?` would give: a stale tab acting on a note
	// that is gone should be told, not humoured.
	ok, err := store.NoteExists(u.ID, noteID)
	if err != nil {
		return err
	}
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "no such note")
	}

	var changed bool
	if trash {
		changed, err = store.Trash(u.ID, noteID, time.Now())
	} else {
		changed, err = store.Restore(u.ID, noteID)
	}
	if err != nil {
		return err
	}
	cards, err := store.CardCount(noteID)
	if err != nil {
		return err
	}
	trashed, err := store.TrashCount(u.ID)
	if err != nil {
		return err
	}

	// `?scope=` is the pile the CALLER is looking at, so a text-scoped session
	// gets its own chip's number back too. Without it the chip went from «+13»
	// to no number at all the moment a word was trashed: the fixed three came
	// back fresh and the text scope simply was not in the list. An unparseable
	// or absent value costs the extra, never the response.
	var extras []srs.Scope
	if sc, ok := srs.ParseScope(c.Query("scope")); ok && sc.IsText() {
		extras = append(extras, sc)
	}

	return c.JSON(TrashResponse{
		NoteID: noteID, Cards: cards, Changed: changed,
		Trashed: trashed, Scopes: s.freshBreakdowns(u.ID, extras...),
	})
}

func (s *Server) handleTrashList(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	store := srs.NewStore(s.db.DB)
	notes, err := store.TrashedNotes(u.ID, 200)
	if err != nil {
		return err
	}
	if notes == nil {
		notes = []srs.TrashedNote{}
	}
	// The dictionary form, from the pack. A trashed row's `hu` is a whole
	// sentence and `target_form` is whatever inflected token it contained;
	// neither is the name a learner recognises the word by.
	ids := make([]int64, 0, len(notes))
	for _, n := range notes {
		if n.LemmaID != nil && *n.LemmaID != 0 {
			ids = append(ids, *n.LemmaID)
		}
	}
	byID, err := s.lemmaData(ids)
	if err != nil {
		return err
	}
	for i := range notes {
		if notes[i].LemmaID == nil {
			continue
		}
		d, ok := byID[*notes[i].LemmaID]
		if !ok {
			continue
		}
		notes[i].Lemma = d.Lemma
		if notes[i].GlossRU == "" {
			notes[i].GlossRU = d.GlossRU
		}
		if notes[i].GlossEN == "" {
			notes[i].GlossEN = d.GlossEN
		}
	}
	total, err := store.TrashCount(u.ID)
	if err != nil {
		return err
	}
	return c.JSON(TrashListResponse{Notes: notes, Total: total})
}

// freshBreakdowns re-reads every pile's numbers, so a trash or restore hands
// the client the same figures the tab strip would get from /api/review/queue.
// Best effort: the move itself is committed, and losing the counts to a read
// error must not turn a successful action into a failure — the client falls
// back to what it already has.
func (s *Server) freshBreakdowns(userID int64, extras ...srs.Scope) []srs.Breakdown {
	cfg := s.Config()
	bds, err := srs.NewStore(s.db.DB).Breakdowns(
		userID, cfg.Review.BurySiblingsEnabled(), time.Now(), extras...)
	if err != nil {
		return nil
	}
	return bds
}
