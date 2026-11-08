package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/curriculum"
	"tanulom/internal/llm"
	"tanulom/internal/morph"
	"tanulom/internal/srs"
	"tanulom/internal/tutor"
)

// The grammar tutor (ADR-027, ADR-029): the pack teaches, a grounded model
// explains, and nothing it says becomes a card.
//
// A thread is a conversation — new, listed, deleted — with an optional pinned
// topic. Every question is grounded in one lesson: the pin, or the lesson the
// app infers from the question (tutor.InferTopic), and the answer records which
// so the panel can print it. The answer streams as server-sent events because a
// local model on a CPU writes a sentence at a time. Every answer carries the
// model's name, and every backticked Hungarian word the pack does not know is
// listed with it — the label is the defence, the same rule ADR-014 set for
// machine translation.

// streamTimeout bounds one answer. Generous: a 7B model on a laptop CPU takes
// a minute for eight Russian sentences — some seven tokens a second — and a
// translation with its word-by-word breakdown (ADR-030) may use the whole
// tutor.MaxAnswerTokens budget, which at that rate is about 140 s. Cutting it
// off at thirty seconds would make the feature unusable on exactly the
// hardware it was built for.
const streamTimeout = 150 * time.Second

// TutorThreadDTO is one conversation's header.
type TutorThreadDTO struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	// Topic is the pinned lesson, blank for a free chat; TitleRU/EN name it.
	Topic string `json:"topic,omitempty"`
	// Kind is "review" for a review chat (ADR-034), else blank; NoteID is the
	// word it is about, and Title is that word.
	Kind      string `json:"kind,omitempty"`
	NoteID    int64  `json:"noteId,omitempty"`
	TitleRU   string `json:"titleRu,omitempty"`
	TitleEN   string `json:"titleEn,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// TutorThreadResponse is a thread with the tutor's availability.
type TutorThreadResponse struct {
	// Available is false when no model is configured; Reason says why in an
	// i18n key, so the panel can point at Settings rather than at a blank.
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	Model     string `json:"model,omitempty"`
	Lang      string `json:"lang"`
	// Thread is nil when none exists yet — a lesson panel opened before its
	// first question. Topic then carries the pin the first question will
	// create the thread with.
	Thread   *TutorThreadDTO    `json:"thread"`
	Topic    string             `json:"topic,omitempty"`
	Messages []tutor.Message    `json:"messages"`
	Sources  []tutor.SourceInfo `json:"sources"`
}

// TutorAskRequest is one question.
type TutorAskRequest struct {
	Question string `json:"question"`
	// Lang is the client's rendered locale, ru or en; blank means locale.ui.
	Lang string `json:"lang"`
	// Replace is the id of a question of the learner's own to edit: it and
	// everything after it are deleted before this one is asked (ADR-031).
	Replace int64 `json:"replace,omitempty"`
	// Card is the review card the question is about (ADR-034). With a
	// Template the server composes the question itself; without one the
	// learner's own Question is asked with the card as context.
	Card *TutorCardRequest `json:"card,omitempty"`
}

// TutorCardRequest names a review card and the phase it is in.
type TutorCardRequest struct {
	CardID int64 `json:"cardId"`
	// Phase is "question" (nothing answered yet — the answer is withheld from
	// the model and the client records a hint) or "answer".
	Phase string `json:"phase"`
	// Template is one of tutor.Templates, or blank for a free question.
	Template string `json:"template,omitempty"`
	// Typed is what the learner typed, for the templates that compare it.
	Typed string `json:"typed,omitempty"`
}

// errEcho ends the stream where the model began reciting the prompt's own
// headings (tutor.EchoAt); the text before it is kept and labelled CutEcho.
var errEcho = errors.New("tutor: the model echoed the prompt")

// tutorFlight is the learner's one in-flight answer: which thread it is on,
// and how to stop it. The stream's own context descends from stop's, so
// POST /api/tutor/:id/stop ends the model call and the stream then finishes
// the ordinary way — `done`, with the partial answer marked interrupted.
type tutorFlight struct {
	thread int64
	stop   context.CancelFunc
}

// TutorCreateRequest opens a thread, pinned to Topic when it is non-blank.
// Kind "review" asks for the review chat about Note instead — one per word
// per local day: found when it exists (200), created when it does not (201).
// ADR-034.
type TutorCreateRequest struct {
	Topic string `json:"topic"`
	Kind  string `json:"kind,omitempty"`
	Note  int64  `json:"note,omitempty"`
	// Word is the surface form a `word` chat is about — the one the learner
	// clicked in a text. Lowercased server-side by `glossKey`, the same
	// normalisation `user_gloss` uses, so «Ház» and «ház» are one chat.
	Word string `json:"word,omitempty"`
}

// TutorSummaryDTO is one row of the learner's chat list, most recently active
// first.
type TutorSummaryDTO struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	// Topic is the pin; LastTopic is the lesson the latest answer used, for a
	// free chat's subtitle. Both come with their names in both languages.
	// Kind is "review" for a review chat, whose Title is the word it is about
	// and NoteID the note that word came from.
	Topic       string `json:"topic,omitempty"`
	Kind        string `json:"kind,omitempty"`
	NoteID      int64  `json:"noteId,omitempty"`
	TitleRU     string `json:"titleRu,omitempty"`
	TitleEN     string `json:"titleEn,omitempty"`
	LastTopic   string `json:"lastTopic,omitempty"`
	LastTopicRU string `json:"lastTopicRu,omitempty"`
	LastTopicEN string `json:"lastTopicEn,omitempty"`
	Messages    int    `json:"messages"`
	LastRole    string `json:"lastRole"`
	LastContent string `json:"lastContent"`
	UpdatedAt   string `json:"updatedAt"`
}

// answerLang resolves the language an answer is written in.
func (s *Server) answerLang(requested string) string {
	switch requested {
	case "ru", "en":
		return requested
	}
	if s.Config().Locale.UI == "en" {
		return "en"
	}
	return "ru"
}

// topicTitles names a topic in both languages; blank for a code the current
// pack does not carry, which the client falls back from.
func (s *Server) topicTitles(code string) (ru, en string) {
	if code == "" || s.dag == nil {
		return "", ""
	}
	if t, ok := s.dag.Topic(code); ok {
		return t.TitleRU, t.TitleEN
	}
	return "", ""
}

func (s *Server) topicTitle(code, lang string) string {
	ru, en := s.topicTitles(code)
	if lang == "en" {
		return en
	}
	return ru
}

// knownTopic validates a topic code against the curriculum.
func (s *Server) knownTopic(code string) (curriculum.Topic, error) {
	if err := s.requireCurriculum(); err != nil {
		return curriculum.Topic{}, err
	}
	t, ok := s.dag.Topic(code)
	if !ok {
		return curriculum.Topic{}, fiber.NewError(fiber.StatusNotFound, "no such topic")
	}
	return t, nil
}

func (s *Server) threadDTO(th tutor.Thread) *TutorThreadDTO {
	ru, en := s.topicTitles(th.TopicCode)
	return &TutorThreadDTO{
		ID: th.ID, Title: th.Title, Topic: th.TopicCode, Kind: th.Kind, NoteID: th.NoteID,
		TitleRU: ru, TitleEN: en,
		CreatedAt: th.CreatedAt.Format(time.RFC3339), UpdatedAt: th.UpdatedAt.Format(time.RFC3339),
	}
}

// threadResponse assembles a thread for the client: availability, the
// header, the turns with their grounding named in the answer language.
func (s *Server) threadResponse(lang string, th *tutor.Thread, topic string, msgs []tutor.Message) TutorThreadResponse {
	out := TutorThreadResponse{Lang: lang, Topic: topic, Messages: []tutor.Message{}, Sources: []tutor.SourceInfo{}}
	if th != nil {
		out.Thread = s.threadDTO(*th)
	}
	for _, m := range msgs {
		m.TopicTitle = s.topicTitle(m.Topic, lang)
		out.Messages = append(out.Messages, m)
	}
	if s.tutorKB != nil {
		out.Sources = s.tutorKB.Sources()
	}
	client, err := llm.New(s.Config().Services.LLM, s.llmKey())
	switch {
	case errors.Is(err, llm.ErrOff):
		out.Reason = "errors.llmOff"
	case errors.Is(err, llm.ErrNoModel):
		out.Reason = "errors.llmNoModel"
	case s.tutorKB == nil:
		out.Reason = "errors.tutorCorpus"
	default:
		out.Available = true
		out.Model = client.Model()
	}
	return out
}

func threadID(c fiber.Ctx) (int64, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fiber.NewError(fiber.StatusNotFound, "errors.tutorNoThread")
	}
	return id, nil
}

// handleTutorList is the learner's chat list.
func (s *Server) handleTutorList(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	store := tutor.NewStore(s.db.DB)
	rows, err := store.Threads(u.ID)
	if err != nil {
		return err
	}
	// `?kind=` selects one tab's worth. Filtered here rather than in SQL
	// because `Threads` already carries the joined last-message summary that
	// the list renders, and a second query shape for it would be a second
	// thing to keep in step.
	//
	// The empty kind is the ordinary chat — free or topic-pinned — and it is
	// addressed as `chat`, because `?kind=` with an empty value is
	// indistinguishable from `?kind=` absent.
	want := c.Query("kind")
	out := make([]TutorSummaryDTO, 0, len(rows))
	for _, r := range rows {
		if want != "" && kindKey(r.Kind) != want {
			continue
		}
		dto := TutorSummaryDTO{
			ID: r.ID, Title: r.Title, Topic: r.TopicCode, Kind: r.Kind, NoteID: r.NoteID, LastTopic: r.LastTopic,
			Messages: r.Messages, LastRole: r.LastRole, LastContent: r.LastContent,
			UpdatedAt: r.UpdatedAt.Format(time.RFC3339),
		}
		dto.TitleRU, dto.TitleEN = s.topicTitles(r.TopicCode)
		dto.LastTopicRU, dto.LastTopicEN = s.topicTitles(r.LastTopic)
		out = append(out, dto)
	}

	counts, err := store.CountThreadsByKind(u.ID)
	if err != nil {
		return err
	}
	return c.JSON(TutorListResponse{Threads: out, Counts: counts})
}

// kindKey names a thread's tab. `tutor_thread.kind` is ” for an ordinary
// chat, and ” is not addressable in a query string.
func kindKey(kind string) string {
	if kind == "" {
		return "chat"
	}
	return kind
}

// TutorListResponse is the thread list plus how many of each kind exist.
//
// The counts are for ALL kinds regardless of the filter: a tab strip has to
// show the number on the tab you are not looking at, which is the only reason
// tabs beat one long list here.
type TutorListResponse struct {
	Threads []TutorSummaryDTO `json:"threads"`
	Counts  map[string]int    `json:"counts"`
}

// handleTutorDeleteAll clears every chat, or every chat of one kind.
//
// A review thread is created once per word per local day, so a month of
// reviewing leaves hundreds — and deleting them one at a time was the only
// option. The client confirms with the count first; the server does not
// second-guess a confirmed request, but it will not accept a kind it does not
// recognise, because a typo that silently deleted everything would be the
// worst possible reading of `?kind=revieww`.
func (s *Server) handleTutorDeleteAll(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	kind := c.Query("kind")
	switch kind {
	case "", "chat", "review", "word":
	default:
		return badRequest("unknown chat kind %q", kind)
	}
	// Two different empties meet here, so the mapping is spelled out: `kind`
	// absent means EVERY kind, while `kind=chat` means the one whose stored
	// value happens to be ''. Both arrive at DeleteThreads as "", and the
	// difference is which rows that "" is allowed to match — so `chat` cannot
	// go through the same path as "all".
	store := tutor.NewStore(s.db.DB)
	var n int64
	switch kind {
	case "":
		n, err = store.DeleteThreads(u.ID, "")
	case "chat":
		n, err = store.DeleteThreadsOfChatKind(u.ID)
	default:
		n, err = store.DeleteThreads(u.ID, kind)
	}
	if err != nil {
		return err
	}
	slog.InfoContext(c.Context(), "tutor threads deleted", "user", u.ID, "kind", kind, "count", n)
	return c.JSON(fiber.Map{"deleted": n})
}

// handleTutorCreate opens a new chat, pinned when a topic is given.
func (s *Server) handleTutorCreate(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req TutorCreateRequest
	if len(c.Body()) > 0 {
		if err := decode(c, &req); err != nil {
			return err
		}
	}
	if req.Topic != "" {
		if _, err := s.knownTopic(req.Topic); err != nil {
			return err
		}
	}
	store := tutor.NewStore(s.db.DB)
	lang := s.answerLang(c.Query("lang"))
	now := time.Now()
	// A review chat's name, read from the note below; an ordinary chat is
	// named by its first question, in the store.
	title := ""
	switch req.Kind {
	case "":
	case tutor.KindReview:
		// The review chat about one WORD, for the local day: found or created
		// (ADR-034). A pin makes no sense on it — the lesson is found per
		// card. Keyed on the note rather than the day, because the panel that
		// opened on the previous word's conversation was the whole complaint.
		if th, ok, err := store.LatestThreadOfKind(u.ID, tutor.KindReview, req.Note, startOfDay(now)); err != nil {
			return err
		} else if ok {
			msgs, err := store.Messages(th.ID)
			if err != nil {
				return err
			}
			return c.JSON(s.threadResponse(lang, &th, "", msgs))
		}
		// The word names the chat, so the list can tell one from another.
		// Read from the note here and never sent by the client: a title is
		// what the learner will look for the conversation under.
		if req.Note > 0 {
			if title, err = s.noteWord(u.ID, req.Note); err != nil {
				return err
			}
		}
		req.Topic = ""
	case tutor.KindWord:
		// The inspector's chat about one form, for the local day: found or
		// created, exactly like the review chat, but keyed on the word. No
		// pin — the lesson is inferred per question, as in a free chat.
		if req.Word == "" {
			return badRequest("a word is required for a word chat")
		}
		form, err := glossKey(req.Word)
		if err != nil {
			return err
		}
		req.Word = form
		if th, ok, err := store.LatestWordThread(u.ID, form, startOfDay(now)); err != nil {
			return err
		} else if ok {
			msgs, err := store.Messages(th.ID)
			if err != nil {
				return err
			}
			return c.JSON(s.threadResponse(lang, &th, "", msgs))
		}
		// The form names the chat, so the «Слова» tab can tell one from
		// another at a glance.
		title = form
		req.Topic = ""
	default:
		return badRequest("unknown thread kind %q", req.Kind)
	}
	th, err := store.CreateThread(u.ID, tutor.NewThread{
		Topic: req.Topic, Kind: req.Kind, Title: title, NoteID: req.Note, Word: req.Word,
	}, now)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(s.threadResponse(lang, &th, req.Topic, nil))
}

// handleTutorNew is a chat that has not been started: availability, the
// sources, no thread. The page shows it before the first question, and the
// first question creates the thread — an empty row for every «Новый чат»
// click would be a list of nothing.
// handleTutorReviewToday answers with the review chat about `?note=` for the
// local day when there is one, and `thread: null` when there is not — the
// review card's panel opens on it without creating an empty chat for a learner
// who asks nothing (ADR-034). Without `?note=` it answers with the day's
// newest review chat, which is what a client older than migration 00019 asks
// for.
func (s *Server) handleTutorReviewToday(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	note, err := strconv.ParseInt(c.Query("note", "0"), 10, 64)
	if err != nil || note < 0 {
		return badRequest("bad note id %q", c.Query("note"))
	}
	lang := s.answerLang(c.Query("lang"))
	store := tutor.NewStore(s.db.DB)
	th, ok, err := store.LatestThreadOfKind(u.ID, tutor.KindReview, note, startOfDay(time.Now()))
	if err != nil {
		return err
	}
	if !ok {
		return c.JSON(s.threadResponse(lang, nil, "", nil))
	}
	msgs, err := store.Messages(th.ID)
	if err != nil {
		return err
	}
	return c.JSON(s.threadResponse(lang, &th, "", msgs))
}

// handleTutorWordToday is today's chat about one form, from the inspector.
//
// The same contract as the review chat's — one per key per local day, found
// and not created here — but keyed on the FORM. An inspector chat exists
// precisely for the words a card does not cover, including the ones the pack
// cannot resolve at all, so there is no note id to hang it on. ADR-042.
func (s *Server) handleTutorWordToday(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	form, err := glossKey(c.Params("form"))
	if err != nil {
		return err
	}
	lang := s.answerLang(c.Query("lang"))
	store := tutor.NewStore(s.db.DB)
	th, ok, err := store.LatestWordThread(u.ID, form, startOfDay(time.Now()))
	if err != nil {
		return err
	}
	if !ok {
		return c.JSON(s.threadResponse(lang, nil, "", nil))
	}
	msgs, err := store.Messages(th.ID)
	if err != nil {
		return err
	}
	return c.JSON(s.threadResponse(lang, &th, "", msgs))
}

func (s *Server) handleTutorNew(c fiber.Ctx) error {
	if _, err := mustUser(c); err != nil {
		return err
	}
	return c.JSON(s.threadResponse(s.answerLang(c.Query("lang")), nil, "", nil))
}

// handleTutorForTopic is what the panel under a lesson opens: the learner's
// latest thread pinned to that topic, or no thread yet with the pin to use.
func (s *Server) handleTutorForTopic(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	code := c.Params("code")
	if _, err := s.knownTopic(code); err != nil {
		return err
	}
	store := tutor.NewStore(s.db.DB)
	th, ok, err := store.LatestThreadForTopic(u.ID, code)
	if err != nil {
		return err
	}
	lang := s.answerLang(c.Query("lang"))
	if !ok {
		return c.JSON(s.threadResponse(lang, nil, code, nil))
	}
	msgs, err := store.Messages(th.ID)
	if err != nil {
		return err
	}
	return c.JSON(s.threadResponse(lang, &th, code, msgs))
}

func (s *Server) handleTutorThread(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	id, err := threadID(c)
	if err != nil {
		return err
	}
	store := tutor.NewStore(s.db.DB)
	th, err := store.GetThread(u.ID, id)
	if errors.Is(err, tutor.ErrNoThread) {
		return fiber.NewError(fiber.StatusNotFound, "errors.tutorNoThread")
	}
	if err != nil {
		return err
	}
	msgs, err := store.Messages(th.ID)
	if err != nil {
		return err
	}
	return c.JSON(s.threadResponse(s.answerLang(c.Query("lang")), &th, th.TopicCode, msgs))
}

func (s *Server) handleTutorDelete(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	id, err := threadID(c)
	if err != nil {
		return err
	}
	err = tutor.NewStore(s.db.DB).DeleteThread(u.ID, id)
	if errors.Is(err, tutor.ErrNoThread) {
		return fiber.NewError(fiber.StatusNotFound, "errors.tutorNoThread")
	}
	if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// tutorEvent is one server-sent event on the answer stream.
type tutorEvent struct {
	// ID is the stored question's id, on the `asked` event that opens the
	// stream — what the client needs to offer an edit of it (ADR-031).
	// Question is its text: composed by the server for a prepared template
	// (ADR-034), so the client replaces its pending bubble.
	ID       int64  `json:"id,omitempty"`
	Question string `json:"question,omitempty"`
	// Text is a delta of the answer.
	Text string `json:"text,omitempty"`
	// Message is the stored assistant turn, on `done` and on `error`.
	Message *tutor.Message `json:"message,omitempty"`
	// Error is an i18n key.
	Error string `json:"error,omitempty"`
}

func (s *Server) handleTutorAsk(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	id, err := threadID(c)
	if err != nil {
		return err
	}
	var req TutorAskRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	question := strings.TrimSpace(req.Question)
	if question == "" && (req.Card == nil || req.Card.Template == "") {
		return badRequest("question is required")
	}
	// Characters, not bytes: the composer counts characters, and a Russian
	// question of eight hundred letters was once refused as too long.
	if n := utf8.RuneCountInString(question); n > tutor.QuestionLimit {
		return badRequest("question is too long (%d characters, limit %d)", n, tutor.QuestionLimit)
	}
	store := tutor.NewStore(s.db.DB)
	th, err := store.GetThread(u.ID, id)
	if errors.Is(err, tutor.ErrNoThread) {
		return fiber.NewError(fiber.StatusNotFound, "errors.tutorNoThread")
	}
	if err != nil {
		return err
	}
	if s.tutorKB == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "errors.tutorCorpus")
	}
	client, err := llm.New(s.Config().Services.LLM, s.llmKey())
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, llmUnavailableKey(err))
	}
	// One question in flight per learner. A second one would race the first
	// for the thread and double the load on a model that is already the
	// slowest thing in the building.
	flightCtx, stop := context.WithCancel(s.bgCtx)
	if _, busy := s.tutorFlights.LoadOrStore(u.ID, &tutorFlight{thread: th.ID, stop: stop}); busy {
		stop()
		return fiber.NewError(fiber.StatusConflict, "errors.tutorBusy")
	}
	release := func() { stop(); s.tutorFlights.Delete(u.ID) }

	// Editing a question: it and everything after it go first, under the
	// flight so nothing interleaves, and the history is read after.
	if req.Replace != 0 {
		if err := store.TruncateFrom(th.ID, req.Replace); err != nil {
			release()
			if errors.Is(err, tutor.ErrNoMessage) {
				return fiber.NewError(fiber.StatusNotFound, "errors.tutorNoMessage")
			}
			return err
		}
	}

	lang := s.answerLang(req.Lang)

	// The card the question is about (ADR-034): its facts for the prompt, the
	// composed question for a prepared template, its features for the
	// grounding. Refused before anything is stored.
	var card *tutor.CardContext
	var shape string
	var cardFeats []string
	if req.Card != nil {
		ctx, feats, err := s.tutorCard(u.ID, req.Card, lang)
		if err != nil {
			release()
			return err
		}
		card, cardFeats = ctx, feats
		if tpl := tutor.Template(req.Card.Template); tpl != "" {
			if !tutor.Allowed(tpl, ctx.Direction, ctx.Phase, ctx.Typed != "") {
				release()
				return badRequest("template %q is not offered on a %s card in the %s phase", tpl, ctx.Direction, ctx.Phase)
			}
			question = tutor.Compose(tpl, *ctx, lang)
			shape = tutor.Shape(tpl, *ctx, lang)
		}
	}

	history, err := store.Messages(th.ID)
	if err != nil {
		release()
		return err
	}

	// The grounding (ADR-029): the pin, or the lesson inferred from the
	// question with the thread's last grounding as the fallback. Recorded on
	// the answer and printed under it.
	topic := th.TopicCode
	if topic == "" && s.dag != nil {
		// The card's own graded features first: they name the lesson more
		// surely than any word in the question does.
		feats := append(append([]string{}, cardFeats...), s.formFeatures(question)...)
		topic = tutor.InferTopic(question, feats, lastGrounded(history), s.tutorIndex()).Topic
	}
	input, err := s.tutorInput(u.ID, topic, lang, question, history)
	if err != nil {
		release()
		return err
	}
	input.Card, input.Shape = card, shape
	msgs := tutor.Build(input)

	now := time.Now()
	// The question is stored BEFORE the model is asked, so a model that fails
	// does not also lose what the learner typed.
	asked := tutor.Message{Role: "user", Content: question, Lang: lang}
	if req.Card != nil {
		asked.CardID, asked.CardPhase, asked.Template = req.Card.CardID, req.Card.Phase, req.Card.Template
	}
	questionID, err := store.Append(th.ID, asked, now)
	if err != nil {
		release()
		return err
	}

	// What follows runs after this handler has returned, on the response
	// writer. Nothing from `c` may be touched in there; everything it needs is
	// captured above. The server's own context bounds it, so a shutdown does
	// not wait on a model, and the WaitGroup lets Close wait for the store
	// write at the end.
	exists := s.wordExists()
	model := client.Model()
	svc := s.Config().Services.LLM
	userID, threadID := u.ID, th.ID
	topicTitle := s.topicTitle(topic, lang)
	s.bg.Add(1)

	c.Set(fiber.HeaderContentType, "text/event-stream; charset=utf-8")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set("X-Accel-Buffering", "no")
	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer s.bg.Done()
		defer release()
		ctx, cancel := context.WithTimeout(flightCtx, streamTimeout)
		defer cancel()

		var answer strings.Builder
		clientGone := false
		send := func(event string, ev tutorEvent) {
			if clientGone {
				return
			}
			data, _ := json.Marshal(ev)
			if _, err := w.WriteString("event: " + event + "\ndata: " + string(data) + "\n\n"); err != nil {
				clientGone = true
				return
			}
			if err := w.Flush(); err != nil {
				clientGone = true
			}
		}

		// The question's id first: the client's bubble carries a temporary one
		// until now, and an edit names the stored one.
		send("asked", tutorEvent{ID: questionID, Question: question})
		echoed := false
		usage, streamErr := client.Stream(ctx, msgs, tutor.MaxAnswerTokens, tutor.Temperature, func(delta string) error {
			answer.WriteString(delta)
			// The model reciting the prompt's own headings: cut there, stop
			// asking for more. The client's draft shows the tail for a moment;
			// the stored message that replaces it on `done` does not.
			if at := tutor.EchoAt(answer.String()); at >= 0 {
				cut := strings.TrimSpace(answer.String()[:at])
				answer.Reset()
				answer.WriteString(cut)
				echoed = true
				return errEcho
			}
			send("delta", tutorEvent{Text: delta})
			if clientGone {
				return errors.New("client went away")
			}
			return nil
		})

		text := strings.TrimSpace(answer.String())
		// The learner pressed stop (or the server is shutting down): not a
		// failure of the model's, and the client that asked for it is not
		// shown an error for it.
		stopped := flightCtx.Err() != nil
		// Interrupted covers the timeout, a stop, a lost connection AND the
		// answer budget (llm.ErrTruncated): the text ends before the answer
		// did, and the panel says so rather than passing it off as complete.
		// None of them is a failure once words arrived (ADR-031).
		stored := tutor.Message{
			Role: "assistant", Content: text, Lang: lang, Model: model,
			PromptVersion: tutor.PromptVersion, Topic: topic, TopicTitle: topicTitle,
			Unattested:   tutor.Unattested(tutor.Backticked(text), exists),
			Interrupted:  streamErr != nil,
			CutReason:    cutReason(streamErr, stopped, echoed, ctx.Err()),
			PromptTokens: usage.PromptTokens, AnswerTokens: usage.CompletionTokens,
		}
		if streamErr != nil && !stopped && !echoed && !errors.Is(streamErr, llm.ErrTruncated) {
			slog.Warn("tutor stream ended early", "user", userID, "thread", threadID, "error", streamErr)
		}
		if text == "" && streamErr != nil {
			// Nothing to keep: a failed call that produced no words leaves the
			// thread with the question only, and the client shows the error.
			//
			// A distinct key from errors.llmFailed (the text-level second
			// opinion's key, handlers_level.go) on purpose: the two failures
			// read very differently — "nothing changed" here, "the estimate
			// stayed as it was" there — and sharing one string once left this
			// message overwriting that one's meaning.
			key := "errors.tutorFailed"
			if stopped {
				key = "errors.tutorStopped"
			}
			send("error", tutorEvent{Error: key})
			return
		}
		// The window the model ran with, for the context meter: Ollama says
		// while the model is loaded, which is now; anyone else says nothing,
		// and the meter says that. Off the flight's context on purpose — a
		// stop must not also lose the figures.
		psCtx, psCancel := context.WithTimeout(s.bgCtx, 3*time.Second)
		stored.ContextTokens = llm.ContextLength(psCtx, svc, model)
		psCancel()

		// Logged because the context meter is only as honest as these three
		// numbers, and when it looked frozen there was no way to tell WHICH of
		// them had stopped arriving: the provider's usage block, or Ollama's
		// /api/ps window. A zero here is now visible in the log rather than
		// inferred from a stale figure on screen. ADR-031.
		if stored.PromptTokens == 0 || stored.ContextTokens == 0 {
			slog.InfoContext(ctx, "tutor answer reported partial usage",
				"user", userID, "thread", threadID, "model", model,
				"prompt_tokens", stored.PromptTokens, "answer_tokens", stored.AnswerTokens,
				"context_tokens", stored.ContextTokens)
		}

		id, err := store.Append(threadID, stored, time.Now())
		if err != nil {
			slog.Error("could not store tutor answer", "error", err)
		}
		stored.ID, stored.CreatedAt = id, time.Now()
		send("done", tutorEvent{Message: &stored})
	})
}

// cutReason names why a stream ended early, for the label under the answer:
// the learner's stop first (it cancels the same context a timeout would), then
// the budget, then the timeout, and anything else is the connection — the
// model or the client went away.
func cutReason(streamErr error, stopped, echoed bool, ctxErr error) string {
	switch {
	case streamErr == nil:
		return ""
	case stopped:
		return tutor.CutStop
	case echoed:
		return tutor.CutEcho
	case errors.Is(streamErr, llm.ErrTruncated):
		return tutor.CutBudget
	case errors.Is(ctxErr, context.DeadlineExceeded):
		return tutor.CutTimeout
	default:
		return tutor.CutConnection
	}
}

// handleTutorStop ends the learner's in-flight answer (ADR-031). What the
// model wrote so far is kept and marked interrupted, exactly as a timeout
// would leave it, and the stream delivers it as `done`. Idempotent — the
// answer may have finished a moment ago — and scoped to the thread named, so
// a stale button on another chat stops nothing.
func (s *Server) handleTutorStop(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	id, err := threadID(c)
	if err != nil {
		return err
	}
	if _, err := tutor.NewStore(s.db.DB).GetThread(u.ID, id); errors.Is(err, tutor.ErrNoThread) {
		return fiber.NewError(fiber.StatusNotFound, "errors.tutorNoThread")
	} else if err != nil {
		return err
	}
	if v, ok := s.tutorFlights.Load(u.ID); ok {
		if f, ok := v.(*tutorFlight); ok && f.thread == id {
			f.stop()
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// llmUnavailableKey names why the llm client could not be built, as the i18n
// key the panel prints: off, or on but with no model to name in the request.
func llmUnavailableKey(err error) string {
	if errors.Is(err, llm.ErrNoModel) {
		return "errors.llmNoModel"
	}
	return "errors.llmOff"
}

// lastGrounded is the lesson the thread's latest answer was built on.
func lastGrounded(history []tutor.Message) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" && history[i].Topic != "" {
			return history[i].Topic
		}
	}
	return ""
}

// wordExists answers "does the dictionary carry this form" for attestation.
// Without a pack every word is unknown to us, which is not the same as
// unattested: the check is skipped rather than flagging everything.
func (s *Server) wordExists() func(string) bool {
	if s.analyzer == nil {
		return func(string) bool { return true }
	}
	return func(w string) bool {
		all, err := s.analyzer.Lookup(w)
		if err != nil {
			return true
		}
		for _, a := range all {
			if a.Certainty != morph.DerivedAnalysis && a.Certainty != morph.UnknownAnalysis {
				return true
			}
		}
		return false
	}
}

// formFeatures reads the Hungarian words in a question through the pack's
// analyser and returns the features they carry — the strongest signal
// inference has: `házban` says Case=Ine whatever the rest of the sentence says.
// Words in another script are skipped; derived and unknown readings do not
// count, for the same reason they do not attest.
func (s *Server) formFeatures(question string) []string {
	if s.analyzer == nil {
		return nil
	}
	const maxWords = 12
	seen := map[string]bool{}
	var out []string
	words := strings.FieldsFunc(question, func(r rune) bool { return !unicode.IsLetter(r) })
	looked := 0
	for _, w := range words {
		if looked >= maxWords {
			break
		}
		if len([]rune(w)) < 3 || !latinOnly(w) || seen[strings.ToLower(w)] {
			continue
		}
		seen[strings.ToLower(w)] = true
		looked++
		all, err := s.analyzer.Lookup(w)
		if err != nil {
			continue
		}
		for _, a := range all {
			if a.Certainty == morph.DerivedAnalysis || a.Certainty == morph.UnknownAnalysis {
				continue
			}
			for _, f := range a.FeatureID {
				if !seen["#"+f] {
					seen["#"+f] = true
					out = append(out, f)
				}
			}
		}
	}
	return out
}

func latinOnly(w string) bool {
	for _, r := range w {
		if !unicode.IsLetter(r) || unicode.Is(unicode.Cyrillic, r) {
			return false
		}
	}
	return true
}

// tutorIndex builds every topic's vocabulary for inference: its titles and
// feature labels in both languages, its features' suffix shapes, and its
// lesson prose. Cheap enough to build per question — twenty-eight topics,
// one pack query each — and therefore always in step with the pack.
func (s *Server) tutorIndex() []tutor.TopicIndex {
	if s.dag == nil {
		return nil
	}
	var out []tutor.TopicIndex
	for _, t := range s.dag.Topics() {
		var labels, allomorphs []string
		for _, id := range t.FeatureIDs {
			var labelRU, labelEN, allo string
			if err := s.pack.DB().QueryRow(`SELECT label_ru, label_en, allomorphs FROM morph_feature WHERE id = ?`, id).
				Scan(&labelRU, &labelEN, &allo); err != nil {
				continue
			}
			labels = append(labels, labelRU, labelEN)
			var shapes map[string]string
			if jsonUnmarshalString(allo, &shapes) == nil {
				for _, v := range shapes {
					allomorphs = append(allomorphs, v)
				}
			}
		}
		out = append(out, tutor.NewTopicIndex(t.Code, []string{t.TitleRU, t.TitleEN}, labels, allomorphs,
			t.ExplainRU+"\n"+t.ExplainEN, t.FeatureIDs))
	}
	return out
}

// tutorInput gathers everything the prompt is built from: the topic's lesson
// and features from the pack, the learner's own state on those features, and
// the corpus sections the question reaches. With no topic (a free chat whose
// question matched no lesson) only the reference and the thread travel, and
// the prompt says so.
func (s *Server) tutorInput(userID int64, code, lang, question string, history []tutor.Message) (tutor.Input, error) {
	in := tutor.Input{Lang: lang, TopicCode: code, Question: question}
	for _, m := range history {
		in.History = append(in.History, llm.Message{Role: m.Role, Content: m.Content})
	}
	if code == "" || s.dag == nil {
		in.Reference = s.tutorKB.Retrieve(tutor.Query{Text: question}, 3)
		return in, nil
	}
	topic, ok := s.dag.Topic(code)
	if !ok {
		in.TopicCode = ""
		in.Reference = s.tutorKB.Retrieve(tutor.Query{Text: question}, 3)
		return in, nil
	}
	if lang == "ru" {
		in.TopicTitle, in.Lesson = topic.TitleRU, topic.ExplainRU
	} else {
		in.TopicTitle, in.Lesson = topic.TitleEN, topic.ExplainEN
	}

	// The features, from the pack, in the answer language; their English
	// labels double as retrieval expansion into the English corpus.
	expand := []string{topic.TitleEN}
	labelByID := map[string]string{}
	for _, f := range s.featureRows(topic.FeatureIDs, lang) {
		in.Features = append(in.Features, f.feature)
		labelByID[f.feature.ID] = f.feature.Label
		expand = append(expand, f.labelEN)
	}

	// The learner's state on exactly those features — the gate's own figures.
	if mastery, err := s.mastery(userID); err == nil {
		for _, m := range mastery {
			label, ok := labelByID[m.FeatureID]
			if !ok {
				continue
			}
			in.Learner = append(in.Learner, tutor.LearnerFeature{
				Label: label, Accuracy: m.Accuracy, Observations: m.Observations, Ready: m.Ready,
			})
		}
	}
	if mistakes, err := s.recentTypedMistakes(userID, topic.FeatureIDs, 3); err == nil {
		in.Mistakes = mistakes
	}

	in.Reference = s.tutorKB.Retrieve(tutor.Query{Topic: code, Text: question, Expand: expand}, 3)
	return in, nil
}

// featureRow is one morph_feature in the answer language, with its English
// label kept for retrieval expansion.
type featureRow struct {
	feature tutor.Feature
	labelEN string
}

// featureRows reads the pack's rows for the given feature ids, in the answer
// language; ids the pack does not carry are skipped, never invented.
func (s *Server) featureRows(ids []string, lang string) []featureRow {
	if s.pack == nil {
		return nil
	}
	out := make([]featureRow, 0, len(ids))
	for _, id := range ids {
		var labelRU, labelEN, explainRU, explainEN, allo string
		err := s.pack.DB().QueryRow(`
			SELECT label_ru, label_en, explain_ru, explain_en, allomorphs
			  FROM morph_feature WHERE id = ?`, id).Scan(&labelRU, &labelEN, &explainRU, &explainEN, &allo)
		if err != nil {
			continue
		}
		f := tutor.Feature{ID: id, Label: labelEN, Explain: explainEN, Allomorphs: map[string]string{}}
		if lang == "ru" {
			f.Label, f.Explain = labelRU, explainRU
		}
		_ = jsonUnmarshalString(allo, &f.Allomorphs)
		out = append(out, featureRow{feature: f, labelEN: labelEN})
	}
	return out
}

// noteWord is the name a review chat carries in the chat list: the word the
// note is about (ADR-034). The target form, or the sentence cut short when a
// note has none — never blank, because a row named «Повторение · » tells the
// learner nothing about which of the day's words it was.
func (s *Server) noteWord(userID, noteID int64) (string, error) {
	note, err := s.engine().Store().Note(userID, noteID)
	if err != nil {
		if errors.Is(err, srs.ErrNotFound) {
			return "", fiber.NewError(fiber.StatusNotFound, "errors.tutorNoCard")
		}
		return "", err
	}
	if w := strings.TrimSpace(note.TargetForm); w != "" {
		return w, nil
	}
	return tutor.TitleFrom(note.HU), nil
}

// tutorCard builds the CARD block for a question asked from the review card
// (ADR-034): the card and its note, decorated the way the queue decorates
// them (the pack's lemma and glosses, the pack's sentence translation, the
// cached crib — never the sidecar), the graded string the review path grades
// against (expectedFor, ADR-017), and the diff's verdict on what was typed.
// In the question phase the answer is withheld: the fields that would give
// it away are cleared and the block says so.
//
// The second value is the card's graded feature ids, for the grounding.
func (s *Server) tutorCard(userID int64, req *TutorCardRequest, lang string) (*tutor.CardContext, []string, error) {
	phase := req.Phase
	if phase != "question" && phase != "answer" {
		return nil, nil, badRequest("card phase must be question or answer")
	}
	e := s.engine()
	card, err := e.Store().Card(userID, req.CardID)
	if err != nil {
		if errors.Is(err, srs.ErrNotFound) {
			return nil, nil, fiber.NewError(fiber.StatusNotFound, "errors.tutorNoCard")
		}
		return nil, nil, err
	}
	note, err := e.Store().Note(userID, card.NoteID)
	if err != nil {
		if errors.Is(err, srs.ErrNotFound) {
			return nil, nil, fiber.NewError(fiber.StatusNotFound, "errors.tutorNoCard")
		}
		return nil, nil, err
	}
	if note.Trashed {
		return nil, nil, fiber.NewError(fiber.StatusNotFound, "errors.tutorNoCard")
	}

	items := []srs.QueueItem{{
		CardID: card.ID, NoteID: note.ID, Direction: card.Direction,
		HU: note.HU, TargetForm: note.TargetForm, TargetStart: note.TargetStart, TargetEnd: note.TargetEnd,
		TargetFeats: note.TargetFeats, GlossRU: note.GlossRU, GlossEN: note.GlossEN,
		LemmaID: note.LemmaID, SentenceID: note.SentenceID,
	}}
	if err := s.enrichFromPack(items); err != nil {
		return nil, nil, err
	}
	if err := s.attachCribs(userID, items); err != nil {
		return nil, nil, err
	}
	it := items[0]

	direction := string(card.Direction)
	ctx := &tutor.CardContext{
		Direction: direction, Phase: phase, HU: it.HU,
		Form: it.TargetForm, Lemma: it.Lemma,
		Typed: strings.TrimSpace(req.Typed), Expected: it.Expected,
	}
	if ctx.Lemma == "" {
		ctx.Lemma = it.TargetForm
	}
	if lang == "ru" {
		ctx.Gloss, ctx.GlossOther = it.GlossRU, it.GlossEN
		ctx.Translation, ctx.TranslationFallback = it.SentenceRU, false
		if ctx.Translation == "" && it.SentenceEN != "" {
			ctx.Translation, ctx.TranslationFallback = it.SentenceEN, true
		}
	} else {
		ctx.Gloss, ctx.GlossOther = it.GlossEN, it.GlossRU
		ctx.Translation, ctx.TranslationFallback = it.SentenceEN, false
		if ctx.Translation == "" && it.SentenceRU != "" {
			ctx.Translation, ctx.TranslationFallback = it.SentenceRU, true
		}
	}
	if ctx.Gloss == "" {
		ctx.Gloss, ctx.GlossOther = ctx.GlossOther, ""
	}
	ctx.TranslationModel = it.SentenceModel
	for _, f := range s.featureRows(note.TargetFeats, lang) {
		ctx.Features = append(ctx.Features, f.feature)
	}

	// The verdict, the review path's own way: the diff classifies the miss
	// and names the feature it blamed, so the tutor can explain THAT.
	if phase == "answer" && ctx.Typed != "" && card.Direction.Productive() && ctx.Expected != "" {
		d := morph.DiffForms(ctx.Typed, ctx.Expected, s.analyzer)
		correct := d.Match
		if d.AccentOnly() && card.Direction != srs.Spell && s.Config().Review.AccentToleranceEnabled() {
			correct, ctx.AcceptedWithAccents = true, true
		}
		ctx.Correct = &correct
		ctx.Class = string(d.Class)
		if d.WrongFeature != "" {
			if rows := s.featureRows([]string{d.WrongFeature}, lang); len(rows) > 0 {
				ctx.WrongFeatureLabel = rows[0].feature.Label
			} else {
				ctx.WrongFeatureLabel = d.WrongFeature
			}
		}
	}

	// Before the answer, everything that IS the answer goes: the form and the
	// dictionary word on a typed card, the meaning on a receptive one, the
	// Hungarian itself on an audio card. The block says the rest is withheld.
	if phase == "question" {
		ctx.Withheld = true
		ctx.Expected, ctx.Typed, ctx.Correct, ctx.Class, ctx.WrongFeatureLabel = "", "", nil, "", ""
		switch card.Direction {
		case srs.Produce:
			// The card shows the meaning alone; the sentence carries the word.
			ctx.Form, ctx.Lemma, ctx.HU = "", "", ""
			ctx.Translation, ctx.TranslationModel, ctx.TranslationFallback = "", "", false
		case srs.Cloze:
			ctx.Form = ""
			if note.TargetEnd > note.TargetStart && note.TargetEnd <= len(note.HU) {
				ctx.HU = note.HU[:note.TargetStart] + "____" + note.HU[note.TargetEnd:]
			}
		case srs.Recognize:
			ctx.Gloss, ctx.GlossOther, ctx.Translation, ctx.TranslationModel = "", "", "", ""
			ctx.TranslationFallback = false
		default: // listen, spell: the sound is the question
			ctx.HU, ctx.Form, ctx.Lemma, ctx.Gloss, ctx.GlossOther = "", "", "", "", ""
			ctx.Translation, ctx.TranslationModel, ctx.TranslationFallback = "", "", false
			ctx.Features = nil
		}
	}
	return ctx, append([]string{}, note.TargetFeats...), nil
}

// recentTypedMistakes reads the last wrong typed answers on cloze cards whose
// blanked span carried one of the topic's features — the review log's
// `answer` column, kept for this since migration 00002.
func (s *Server) recentTypedMistakes(userID int64, featureIDs []string, limit int) ([]tutor.Mistake, error) {
	if len(featureIDs) == 0 {
		return nil, nil
	}
	want := map[string]bool{}
	for _, id := range featureIDs {
		want[id] = true
	}
	rows, err := s.db.Query(`
		SELECT r.answer, n.target_form, n.target_feats
		  FROM review_log r
		  JOIN card c ON c.id = r.card_id
		  JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND r.correct = 0 AND r.answer != '' AND c.direction = ?
		 ORDER BY r.id DESC LIMIT 60`, userID, string(srs.Cloze))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tutor.Mistake
	seen := map[string]bool{}
	for rows.Next() && len(out) < limit {
		var typed, expected, feats string
		if err := rows.Scan(&typed, &expected, &feats); err != nil {
			return nil, err
		}
		hit := false
		for _, f := range strings.Split(feats, "|") {
			if want[f] {
				hit = true
				break
			}
		}
		if !hit || typed == expected || seen[typed+"|"+expected] {
			continue
		}
		seen[typed+"|"+expected] = true
		out = append(out, tutor.Mistake{Typed: typed, Expected: expected})
	}
	return out, rows.Err()
}
