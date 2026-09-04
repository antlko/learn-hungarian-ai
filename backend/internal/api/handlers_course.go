package api

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/course"
	"tanulom/internal/morph"
	"tanulom/internal/srs"
)

// The guided course: the answer to "what do I actually do today".
//
// Everything else in this app is a mechanism — a scheduler, a miner, a gate.
// This is the only screen that tells the learner what to LEARN, in what order,
// and why, for the first six months. After that it stops and hands over to the
// resource catalogue, which is the point rather than a limitation: see
// internal/course.

func (s *Server) requireCourse() error {
	if err := s.requirePack(); err != nil {
		return err
	}
	if s.syllabus == nil || s.syllabus.Empty() {
		return fiber.NewError(fiber.StatusServiceUnavailable,
			"this language pack carries no course")
	}
	return nil
}

// inPlaySet reports which lemmas the learner already has in the review deck.
// "In play" is the completion criterion for a unit — a fact about the deck
// rather than a button, so the plan can never disagree with the deck.
func (s *Server) inPlaySet(userID int64) (map[string]bool, error) {
	// lemma_state keys on the pack's lemma_id and the pack is a separate,
	// read-only database, so the join happens here rather than in SQL.
	ids := []int64{}
	idRows, err := s.db.Query(`
		SELECT lemma_id FROM lemma_state
		 WHERE user_id = ? AND status IN ('learning','known')`, userID)
	if err != nil {
		return nil, err
	}
	defer idRows.Close()
	for idRows.Next() {
		var id int64
		if err := idRows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := idRows.Err(); err != nil {
		return nil, err
	}

	out := map[string]bool{}
	if len(ids) == 0 || s.pack == nil {
		return out, nil
	}
	// One query, not one per lemma: a learner at B1 has thousands of these.
	q := `SELECT lemma FROM lemma WHERE id IN (?` +
		repeatComma(len(ids)-1) + `)`
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	lrows, err := s.pack.DB().Query(q, args...)
	if err != nil {
		return out, nil
	}
	defer lrows.Close()
	for lrows.Next() {
		var lemma string
		if err := lrows.Scan(&lemma); err != nil {
			return out, nil
		}
		out[lemma] = true
	}
	return out, nil
}

func repeatComma(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, 0, n*2)
	for range n {
		b = append(b, ',', '?')
	}
	return string(b)
}

// CourseResponse is the plan screen's payload.
type CourseResponse struct {
	Plan course.Plan `json:"plan"`
	// Progress rows for units the learner has enrolled, keyed by unit code.
	Enrolled map[string]course.Progress `json:"enrolled"`
}

func (s *Server) handleCourse(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requireCourse(); err != nil {
		return err
	}
	progress, err := s.courseProgress(u.ID)
	if err != nil {
		return err
	}
	inPlay, err := s.inPlaySet(u.ID)
	if err != nil {
		return err
	}
	plan := s.syllabus.Resolve(progress, func(lemma string) bool { return inPlay[lemma] })
	return c.JSON(CourseResponse{Plan: plan, Enrolled: progress})
}

func (s *Server) courseProgress(userID int64) (map[string]course.Progress, error) {
	rows, err := s.db.Query(`
		SELECT unit_code, enrolled_at, notes_made, skipped
		  FROM course_progress WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]course.Progress{}
	for rows.Next() {
		var p course.Progress
		var enrolled sql.NullString
		if err := rows.Scan(&p.UnitCode, &enrolled, &p.NotesMade, &p.Skipped); err != nil {
			return nil, err
		}
		p.Enrolled = enrolled.Valid && enrolled.String != ""
		out[p.UnitCode] = p
	}
	return out, rows.Err()
}

// EnrolResponse reports what taking up a unit actually produced.
type EnrolResponse struct {
	UnitCode  string `json:"unitCode"`
	NotesMade int    `json:"notesMade"`
	Duplicate int    `json:"duplicate"`
	// Skipped names the words the pack has no attested sentence for. Surfaced,
	// never hidden: a card must be a sentence (a bare lemma teaches nothing
	// about the morphology that carries the meaning), so a word without one is
	// honestly left out rather than turned into a word-list card.
	Skipped []SkippedWord `json:"skipped"`
}

// SkippedWord is a course word that got no card, WITH ITS MEANING.
//
// It used to be the bare Hungarian string. Naming a word to a learner and then
// declining to say what it means is the one thing this panel must not do: the
// unit taught them the word exists, the deck does not carry it, and the screen
// was leaving them with an untranslated token and no way to look it up. The
// gloss is already in the pack beside the word — the course could not have
// listed it otherwise.
type SkippedWord struct {
	Lemma   string `json:"lemma"`
	GlossRU string `json:"glossRu"`
	GlossEN string `json:"glossEn"`
}

func skippedWord(w course.Word) SkippedWord {
	return SkippedWord{Lemma: w.Lemma, GlossRU: w.GlossRU, GlossEN: w.GlossEN}
}

func (s *Server) handleEnrolUnit(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requireCourse(); err != nil {
		return err
	}
	unit, ok := s.syllabus.Unit(c.Params("code"))
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "no such unit")
	}

	cfg := s.Config()
	store := srs.NewStore(s.db.DB)
	plan := srs.PlanFor(cfg.Review.Goal, directionWeights(cfg), s.audioAvailable())
	if len(plan.Directions) == 0 {
		return badRequest("no review directions are enabled")
	}

	out := EnrolResponse{UnitCode: unit.Code, Skipped: []SkippedWord{}}
	for _, w := range unit.Words {
		note, ok, err := s.noteForWord(w)
		if err != nil {
			return err
		}
		if !ok {
			out.Skipped = append(out.Skipped, skippedWord(w))
			continue
		}
		note.UserID = u.ID
		note.Source = "course:" + unit.Code

		if existing, err := store.FindDuplicate(u.ID, note.HU, note.TargetForm); err != nil {
			return err
		} else if existing != 0 {
			// A trashed word comes back when its unit is taken up again: the
			// duplicate check would otherwise match the trashed note forever and
			// leave the unit permanently unable to restock itself. Counted as a
			// duplicate either way — nothing new was created — but the word is in
			// the deck again, which is what enrolling asked for. See ADR-025.
			if _, err := store.Restore(u.ID, existing); err != nil {
				return err
			}
			out.Duplicate++
			continue
		}
		directions := plan.Directions
		// A cloze needs a SPAN, not features — the same rule note creation
		// applies, through the same predicate so the two cannot drift.
		if !clozeable(note) {
			directions = withoutCloze(directions)
		}
		if len(directions) == 0 {
			out.Skipped = append(out.Skipped, skippedWord(w))
			continue
		}
		if _, _, err := store.CreateNote(note, directions, time.Now()); err != nil {
			return err
		}
		if note.LemmaID != nil {
			if _, err := store.MarkLemmaLearning(u.ID, *note.LemmaID); err != nil {
				return err
			}
		}
		out.NotesMade++
	}

	if _, err := s.db.Exec(`
		INSERT INTO course_progress (user_id, unit_code, enrolled_at, notes_made, skipped)
		VALUES (?,?,?,?,?)
		ON CONFLICT(user_id, unit_code) DO UPDATE SET
		  enrolled_at = excluded.enrolled_at,
		  notes_made  = course_progress.notes_made + excluded.notes_made,
		  skipped     = excluded.skipped`,
		u.ID, unit.Code, time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		out.NotesMade, len(out.Skipped)); err != nil {
		return err
	}
	return c.JSON(out)
}

// noteForWord finds a short attested sentence containing the word and builds a
// cloze-able note from it.
//
// Short on purpose: the first encounter with a word should not also be a
// parsing exercise. `sentence.len_tokens` is indexed, so "shortest first" is
// free, and the n+1 scorer handles difficulty later in the reader.
func (s *Server) noteForWord(w course.Word) (srs.Note, bool, error) {
	if s.pack == nil || s.analyzer == nil {
		return srs.Note{}, false, nil
	}
	// One spelling can be several lemma rows — `igen` is an ADV, a NOUN and an
	// INTJ — and the corpus links its tokens to only some of them. Picking the
	// row whose part of speech the course happens to name would skip the word
	// whenever the corpus disagreed: `igen` is tagged INTJ here and has 147
	// sentences under its ADV row and none under INTJ.
	//
	// So take every candidate, prefer the requested part of speech, and then
	// prefer the one the corpus can actually illustrate.
	idRows, err := s.pack.DB().Query(`
		SELECT l.id FROM lemma l WHERE l.lemma = ?
		 ORDER BY CASE WHEN l.upos = ? THEN 0 ELSE 1 END,
		          (SELECT COUNT(*) FROM sentence_token t WHERE t.lemma_id = l.id) DESC,
		          COALESCE(l.freq_rank, 1000000)`, w.Lemma, w.UPOS)
	if err != nil {
		return srs.Note{}, false, err
	}
	var candidates []int64
	for idRows.Next() {
		var id int64
		if err := idRows.Scan(&id); err != nil {
			idRows.Close()
			return srs.Note{}, false, err
		}
		candidates = append(candidates, id)
	}
	idRows.Close()
	if len(candidates) == 0 {
		return srs.Note{}, false, nil
	}

	for _, lemmaID := range candidates {
		note, ok, err := s.noteFromSentences(lemmaID, w)
		if err != nil {
			return srs.Note{}, false, err
		}
		if ok {
			return note, true, nil
		}
	}
	return srs.Note{}, false, nil
}

// noteFromSentences builds a note from the shortest attested sentence that
// contains this exact lemma.
//
// Two searches, because ONE OF THEM IS NOT ENOUGH and the failure was silent.
//
// The corpus links each token to a lemma id chosen by the tagger that built the
// pack. The course names its words with the DICTIONARY's lemma. Those two
// disagree more often than you would hope, and when they do the join finds
// nothing and the word is dropped from the unit with a message saying the
// corpus has no sentence for it — which was simply false:
//
//   - `nagyon` (ADV, the dictionary's own headword) is lemmatised by the tagger
//     to the adjective `nagy`. The wordform `nagyon` occurs in 820 corpus
//     sentences and the join returned zero.
//   - `bemutatkozik` appears as `bemutatkozzak`, `bemutatkozzam`, `Bemutatkozna`
//     — all with a NULL lemma id — and as `bemutatkozó`, filed under a separate
//     lemma row for the participle.
//
// So the second pass searches by SURFACE FORM instead, over the forms the pack
// attests for this lemma. Both passes then go through the same verification:
// the runtime analyser has to agree that the token in the sentence really reads
// as this lemma. Nothing is loosened — a sentence still has to be attested and
// the token still has to be the word being taught — only the way candidates are
// found is widened.
func (s *Server) noteFromSentences(lemmaID int64, w course.Word) (srs.Note, bool, error) {
	seen := map[int64]bool{}

	for _, find := range []func(int64) (*sql.Rows, error){
		s.sentencesByLemmaID,
		s.sentencesBySurfaceForm,
	} {
		rows, err := find(lemmaID)
		if err != nil {
			return srs.Note{}, false, err
		}
		note, ok, err := s.firstUsableSentence(rows, lemmaID, w, seen)
		if err != nil {
			return srs.Note{}, false, err
		}
		if ok {
			return note, true, nil
		}
	}
	return srs.Note{}, false, nil
}

// sentencesByLemmaID: the corpus's own link. Indexed, exact, and preferred.
//
// Shortest first, but no hard upper bound: a long sentence is a worse first
// encounter than a short one, and no sentence at all is worse than both — the
// word would be skipped and the unit would quietly teach less than it says. The
// reader's n+1 scoring handles difficulty properly later.
func (s *Server) sentencesByLemmaID(lemmaID int64) (*sql.Rows, error) {
	return s.pack.DB().Query(`
		SELECT s.id, s.hu FROM sentence s
		  JOIN sentence_token t ON t.sentence_id = s.id
		 WHERE t.lemma_id = ? AND s.len_tokens >= 3
		 ORDER BY s.len_tokens LIMIT 12`, lemmaID)
}

// maxFormsSearched bounds the IN clause of the surface search. The forms come
// back best-trusted first, so the cut falls on the speculative tail; a verb with
// two hundred generated forms must not turn one enrolment into a scan.
const maxFormsSearched = 24

// sentencesBySurfaceForm: sentences containing any attested spelling of the
// lemma, whatever the tagger decided the token's lemma was.
//
// `sentence_token.form` is BINARY-collated like every other index in the pack,
// so the casings are enumerated rather than folded — `COLLATE NOCASE` here would
// scan all 331k rows. See internal/morph/casefold.go.
func (s *Server) sentencesBySurfaceForm(lemmaID int64) (*sql.Rows, error) {
	forms, err := s.analyzer.FormsOf(lemmaID)
	if err != nil {
		return nil, err
	}
	var variants []string
	seen := map[string]bool{}
	for _, f := range forms {
		// Only attested spellings. A generated form is exactly what must not be
		// used to go looking for "real Hungarian" — and if the generator has
		// invented a form, no corpus sentence contains it anyway.
		if !f.Attested() {
			continue
		}
		for _, v := range morph.CaseVariants(f.Form) {
			if !seen[v] {
				seen[v] = true
				variants = append(variants, v)
			}
		}
		if len(variants) >= maxFormsSearched {
			break
		}
	}
	if len(variants) == 0 {
		return nil, nil
	}
	marks, args := morph.Placeholders(variants)
	return s.pack.DB().Query(`
		SELECT s.id, s.hu FROM sentence s
		  JOIN sentence_token t ON t.sentence_id = s.id
		 WHERE t.form IN (`+marks+`) AND s.len_tokens >= 3
		 ORDER BY s.len_tokens LIMIT 12`, args...)
}

// firstUsableSentence returns a note from the first candidate whose tokens the
// analyser confirms as this lemma. `seen` carries across the two passes so a
// sentence found by both is only examined once.
func (s *Server) firstUsableSentence(
	rows *sql.Rows, lemmaID int64, w course.Word, seen map[int64]bool,
) (srs.Note, bool, error) {
	if rows == nil {
		return srs.Note{}, false, nil
	}
	defer rows.Close()

	for rows.Next() {
		var sentenceID int64
		var hu string
		if err := rows.Scan(&sentenceID, &hu); err != nil {
			return srs.Note{}, false, err
		}
		if seen[sentenceID] {
			continue
		}
		seen[sentenceID] = true

		// Locate the token that belongs to THIS lemma so the cloze blanks the
		// word being taught rather than whichever token happens to be first.
		for _, tok := range morph.Tokenize(hu) {
			if !tok.IsWord {
				continue
			}
			readings, err := s.analyzer.Lookup(tok.Text)
			if err != nil {
				return srs.Note{}, false, err
			}
			for _, r := range readings {
				if r.LemmaID != lemmaID {
					continue
				}
				id := lemmaID
				sid := sentenceID
				return srs.Note{
					Kind: "sentence", HU: hu,
					TargetForm: tok.Text, TargetStart: tok.Start, TargetEnd: tok.End,
					TargetFeats: r.FeatureID,
					GlossRU:     w.GlossRU, GlossEN: w.GlossEN,
					LemmaID: &id, SentenceID: &sid,
					Notes: w.Note,
				}, true, nil
			}
		}
	}
	return srs.Note{}, false, rows.Err()
}

// ResourceDTO is one entry of the hand-off catalogue.
type ResourceDTO struct {
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Level   string `json:"level"`
	BlurbRU string `json:"blurbRu"`
	BlurbEN string `json:"blurbEn"`
	Free    bool   `json:"free"`
}

// handleResources serves the curated list of places to read and listen once the
// course hands over. Shipped in the pack rather than hardcoded here so it can be
// updated with a pack release when a site dies.
func (s *Server) handleResources(c fiber.Ctx) error {
	if err := s.requirePack(); err != nil {
		return err
	}
	rows, err := s.pack.DB().Query(`
		SELECT kind, title, url, level, blurb_ru, blurb_en, free
		  FROM resource ORDER BY ord, title`)
	if err != nil {
		// An older pack has no resource table; an empty catalogue is honest.
		return c.JSON(fiber.Map{"resources": []ResourceDTO{}})
	}
	defer rows.Close()
	out := []ResourceDTO{}
	for rows.Next() {
		var r ResourceDTO
		var free int
		if err := rows.Scan(&r.Kind, &r.Title, &r.URL, &r.Level,
			&r.BlurbRU, &r.BlurbEN, &free); err != nil {
			return err
		}
		r.Free = free == 1
		out = append(out, r)
	}
	return c.JSON(fiber.Map{"resources": out})
}
