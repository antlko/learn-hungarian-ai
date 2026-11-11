package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/course"
	"tanulom/internal/llm"
	"tanulom/internal/morph"
	"tanulom/internal/srs"
)

// AI-written sentences for course words the pack cannot illustrate. ADR-073.
//
// A word like `sziasztok` has no attested pack sentence, so the unit leaves it
// without a card. The learner may ask the configured model for a few short
// sentences, READ them, and accept the ones they want. Only then is a note made.
//
// This is the one narrow exception to "no LLM Hungarian becomes a card"
// (ADR-042, ADR-066): the learner reviews every sentence, the card is labelled
// as AI-written everywhere it is shown, and the guardrails below hold:
//
//   - propose stores nothing; accept re-validates the text itself, so a client
//     can never smuggle in a sentence the validator would have refused;
//   - only a word that is currently `noExample` for an enrolled unit, never one
//     in the deck or marked «Знаю»;
//   - the note carries a span on the headword and NO features (the model's
//     morphology is not graded), and no sentence_id — which is the marker:
//     `source = 'course:<code>'` with `sentence_id IS NULL` is an AI-written
//     card (srs.QueueItem.AIWritten). It stays a course note, so the unit,
//     scope and in-deck accounting treat it like any other course card.

const courseAIPromptVersion = 1
const courseAITimeout = 60 * time.Second
const courseAIMaxTokens = 500
const courseAIMaxProposals = 3
const courseAIMaxTokensInSentence = 12
const courseAIMaxRunes = 160

// CourseProposal is one sentence a model suggested, already validated.
type CourseProposal struct {
	HU string `json:"hu"`
	TR string `json:"tr"`
}

// CourseProposeResponse is the proposal for one word. Nothing in it is stored.
type CourseProposeResponse struct {
	Lemma     string           `json:"lemma"`
	Model     string           `json:"model"`
	Lang      string           `json:"lang"`
	Proposals []CourseProposal `json:"proposals"`
}

// CourseAcceptRequest is the sentence the learner chose.
type CourseAcceptRequest struct {
	HU string `json:"hu"`
	TR string `json:"tr"`
}

// CourseAcceptResponse reports the outcome. Duplicate is true when the sentence
// was already a card (restored if it had been trashed).
type CourseAcceptResponse struct {
	Lemma     string `json:"lemma"`
	Duplicate bool   `json:"duplicate"`
}

func courseAISystem(lang string) string {
	langName := "Russian"
	if lang != "ru" {
		langName = "English"
	}
	return `You write example sentences for a beginner learning Hungarian.

Reply with a JSON object of this shape:
{"sentences": [{"hu": "a short Hungarian sentence", "tr": "its translation into ` + langName + `"}]}

Rules:
- Write 2 or 3 sentences, each natural, simple, level A1, at most 8 words.
- Every sentence must contain the given headword exactly as written, spelled the same way.
- Use everyday situations; no names of real people, no rare words.
- "tr" is a plain ` + langName + ` translation, with no notes.
- Reply with the JSON object and nothing else: no markdown fences, no commentary.`
}

// unitWordForAI resolves and gates the (unit, word) pair both endpoints share.
// It returns the course word, its pack lemma id and the acting user's store.
func (s *Server) unitWordForAI(c fiber.Ctx, userID int64) (course.Unit, course.Word, int64, error) {
	if err := s.requireCourse(); err != nil {
		return course.Unit{}, course.Word{}, 0, err
	}
	unit, ok := s.syllabus.Unit(c.Params("code"))
	if !ok {
		return unit, course.Word{}, 0, fiber.NewError(fiber.StatusNotFound, "no such unit")
	}
	lemma, err := url.PathUnescape(c.Params("lemma"))
	if err != nil {
		return unit, course.Word{}, 0, badRequest("lemma is not a valid path segment")
	}
	var word course.Word
	found := false
	for _, w := range unit.Words {
		if w.Lemma == lemma {
			word, found = w, true
			break
		}
	}
	if !found {
		return unit, word, 0, fiber.NewError(fiber.StatusNotFound, "no such word in this unit")
	}

	progress, err := s.courseProgress(userID)
	if err != nil {
		return unit, word, 0, err
	}
	if !progress[unit.Code].Enrolled {
		return unit, word, 0, fiber.NewError(fiber.StatusConflict, "errors.courseWordNotEligible")
	}

	// «Знаю» and already-in-deck words are both in play; neither is offered.
	inPlay, err := s.inPlaySet(userID)
	if err != nil {
		return unit, word, 0, err
	}
	if inPlay[word.Lemma] {
		return unit, word, 0, fiber.NewError(fiber.StatusConflict, "errors.courseWordNotEligible")
	}
	// Only a word nothing in the pack can illustrate.
	if _, ok, err := s.noteForWord(word); err != nil {
		return unit, word, 0, err
	} else if ok {
		return unit, word, 0, fiber.NewError(fiber.StatusConflict, "errors.courseWordNotEligible")
	}

	lemmaID, ok, err := s.courseLemmaID(word)
	if err != nil {
		return unit, word, 0, err
	}
	if !ok {
		// Without a dictionary row the word cannot be counted in the deck.
		return unit, word, 0, fiber.NewError(fiber.StatusConflict, "errors.courseWordNotEligible")
	}
	return unit, word, lemmaID, nil
}

// courseLemmaID picks the pack lemma row for a course word: the requested part
// of speech first, then the most frequent.
func (s *Server) courseLemmaID(w course.Word) (int64, bool, error) {
	var id int64
	err := s.pack.DB().QueryRow(`
		SELECT l.id FROM lemma l WHERE l.lemma = ?
		 ORDER BY CASE WHEN l.upos = ? THEN 0 ELSE 1 END,
		          COALESCE(l.freq_rank, 1000000) LIMIT 1`, w.Lemma, w.UPOS).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return id, true, nil
}

// validateCourseSentence is the single gate for model text, used by propose
// AND accept. It returns the cleaned sentence and the byte span of the
// headword in it. The headword must appear as a token run, case-folded.
func validateCourseSentence(hu, headword string) (string, int, int, bool) {
	hu = strings.TrimSpace(stripTicks(hu))
	if hu == "" || strings.ContainsAny(hu, "\n\r") || len([]rune(hu)) > courseAIMaxRunes {
		return "", 0, 0, false
	}
	var words []morph.Token
	for _, t := range morph.Tokenize(hu) {
		if t.IsWord {
			words = append(words, t)
		}
	}
	if len(words) < 1 || len(words) > courseAIMaxTokensInSentence {
		return "", 0, 0, false
	}
	head := strings.Fields(headword)
	if len(head) == 0 {
		return "", 0, 0, false
	}
	for i := 0; i+len(head) <= len(words); i++ {
		match := true
		for j, h := range head {
			if !strings.EqualFold(words[i+j].Text, h) {
				match = false
				break
			}
		}
		if match {
			return hu, words[i].Start, words[i+len(head)-1].End, true
		}
	}
	return "", 0, 0, false
}

type courseAIFields struct {
	Sentences []AIExample `json:"sentences"`
}

// parseCourseProposals reads the model's answer leniently and keeps only the
// sentences the validator accepts, de-duplicated and capped.
func parseCourseProposals(text, headword string) []CourseProposal {
	body := stripFence(strings.TrimSpace(text))
	if i, j := strings.IndexByte(body, '{'), strings.LastIndexByte(body, '}'); i >= 0 && j > i {
		body = body[i : j+1]
	}
	var raw courseAIFields
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		if err := json.Unmarshal([]byte(strings.ReplaceAll(body, "`", "")), &raw); err != nil {
			return []CourseProposal{}
		}
	}
	out := []CourseProposal{}
	seen := map[string]bool{}
	for _, ex := range raw.Sentences {
		hu, _, _, ok := validateCourseSentence(ex.HU, headword)
		if !ok || seen[strings.ToLower(hu)] {
			continue
		}
		seen[strings.ToLower(hu)] = true
		out = append(out, CourseProposal{HU: hu, TR: strings.TrimSpace(stripTicks(ex.TR))})
		if len(out) == courseAIMaxProposals {
			break
		}
	}
	return out
}

func (s *Server) handleCourseWordPropose(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	_, word, _, err := s.unitWordForAI(c, u.ID)
	if err != nil {
		return err
	}
	client, err := llm.New(s.service("llm"), s.llmKey())
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, llmUnavailableKey(err))
	}
	lang := s.answerLang(c.Query("lang"))

	ctx, cancel := context.WithTimeout(c.Context(), courseAITimeout)
	defer cancel()
	text, err := client.Chat(ctx, courseAISystem(lang), "Headword: "+word.Lemma, courseAIMaxTokens)
	if err != nil {
		slog.WarnContext(c.Context(), "course ai propose failed",
			"user", u.ID, "word", word.Lemma, "model", client.Model(), "error", err)
		return fiber.NewError(fiber.StatusBadGateway, "errors.courseAIFailed")
	}
	out := CourseProposeResponse{
		Lemma: word.Lemma, Model: client.Model(), Lang: lang,
		Proposals: parseCourseProposals(text, word.Lemma),
	}
	slog.InfoContext(c.Context(), "course ai proposed",
		"user", u.ID, "word", word.Lemma, "model", out.Model,
		"prompt_version", courseAIPromptVersion, "kept", len(out.Proposals))
	return c.JSON(out)
}

func (s *Server) handleCourseWordAccept(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req CourseAcceptRequest
	if err := c.Bind().JSON(&req); err != nil {
		return badRequest("invalid request body")
	}
	unit, word, lemmaID, err := s.unitWordForAI(c, u.ID)
	if err != nil {
		return err
	}
	hu, start, end, ok := validateCourseSentence(req.HU, word.Lemma)
	if !ok {
		return badRequest("the sentence must be one short sentence containing %q", word.Lemma)
	}

	cfg := s.Config()
	store := srs.NewStore(s.db.DB)
	plan := srs.PlanFor(cfg.Review.Goal, directionWeights(cfg), s.audioAvailable())
	if len(plan.Directions) == 0 {
		return badRequest("no review directions are enabled")
	}

	tr := strings.TrimSpace(stripTicks(req.TR))
	if len([]rune(tr)) > courseAIMaxRunes*2 {
		tr = string([]rune(tr)[:courseAIMaxRunes*2])
	}
	note := srs.Note{
		UserID: u.ID, Kind: "sentence", HU: hu,
		TargetForm: hu[start:end], TargetStart: start, TargetEnd: end,
		TargetFeats: []string{},
		GlossRU:     word.GlossRU, GlossEN: word.GlossEN,
		LemmaID: &lemmaID, // no SentenceID: that is what marks it AI-written
		Notes:   tr,
		Source:  "course:" + unit.Code,
	}

	out := CourseAcceptResponse{Lemma: word.Lemma}
	if existing, err := store.FindDuplicate(u.ID, note.HU, note.TargetForm); err != nil {
		return err
	} else if existing != 0 {
		if _, err := store.Restore(u.ID, existing); err != nil {
			return err
		}
		out.Duplicate = true
	} else {
		if _, _, err := store.CreateNote(note, plan.Directions, time.Now()); err != nil {
			return err
		}
	}
	if _, err := store.MarkLemmaLearning(u.ID, lemmaID); err != nil {
		return err
	}
	if !out.Duplicate {
		if _, err := s.db.Exec(`
			UPDATE course_progress SET notes_made = notes_made + 1,
			       skipped = MAX(skipped - 1, 0)
			 WHERE user_id = ? AND unit_code = ?`, u.ID, unit.Code); err != nil {
			return err
		}
	}
	slog.InfoContext(c.Context(), "course ai sentence accepted",
		"user", u.ID, "unit", unit.Code, "word", word.Lemma, "duplicate", out.Duplicate)
	return c.JSON(out)
}
