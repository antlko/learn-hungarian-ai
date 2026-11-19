package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/db"
	"tanulom/internal/llm"
	"tanulom/internal/tutor"
	"tanulom/internal/writing"
)

// «Письмо» (Writing, ADR-085). The check marks a text; it mints nothing: no
// handler here touches a card, a note, a gloss or any scheduling table, and a
// test counts those tables around every endpoint.

const (
	writingBodyLimit  = 6000 // runes
	writingTitleLimit = 120  // runes
	writingRawLimit   = 64 << 10
	writingModelLimit = 80 // runes
	// The local check streams: a 7B model on a laptop CPU needs minutes for a
	// whole-text review. The token budget and the timeout are sized together
	// (3000 tokens at ~10 tok/s is five minutes).
	writingMaxTokens     = 3000
	writingTemperature   = 0.2
	writingStreamTimeout = 300 * time.Second
	writingProgressEvery = time.Second
)

// writingFlight is a learner's one in-flight local check.
type writingFlight struct {
	piece int64
	stop  context.CancelFunc
}

func writingErr(status int, key string) error { return fiber.NewError(status, key) }

func writingPieceID(c fiber.Ctx) (int64, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, writingErr(fiber.StatusNotFound, "errors.writingNotFound")
	}
	return id, nil
}

// ownedPiece loads the piece or answers 404 — another learner's id is not
// distinguishable from a missing one.
func (s *Server) ownedPiece(c fiber.Ctx) (db.User, db.WritingPiece, error) {
	u, err := mustUser(c)
	if err != nil {
		return u, db.WritingPiece{}, err
	}
	id, err := writingPieceID(c)
	if err != nil {
		return u, db.WritingPiece{}, err
	}
	p, err := s.db.WritingPiece(u.ID, id)
	if errors.Is(err, db.ErrNotFound) {
		return u, p, writingErr(fiber.StatusNotFound, "errors.writingNotFound")
	}
	return u, p, err
}

func countWords(s string) int {
	n := 0
	for _, f := range strings.Fields(s) {
		for _, r := range f {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				n++
				break
			}
		}
	}
	return n
}

func clipRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func (s *Server) writingLang(requested string) string { return s.answerLang(requested) }

func writingTopicDTO(t writing.Topic) WritingTopicDTO {
	return WritingTopicDTO{Code: t.Code, Level: t.Level, Genre: t.Genre, TitleRU: t.TitleRU, TitleEN: t.TitleEN,
		BriefRU: t.BriefRU, BriefEN: t.BriefEN, MinWords: t.MinWords, MaxWords: t.MaxWords}
}

func (s *Server) handleWritingTopics(c fiber.Ctx) error {
	topics, err := writing.Topics()
	if err != nil {
		return err
	}
	out := make([]WritingTopicDTO, 0, len(topics))
	for _, t := range topics {
		out = append(out, writingTopicDTO(t))
	}
	return c.JSON(WritingTopicsResponse{Topics: out})
}

func (s *Server) handleWritingList(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	rows, err := s.db.WritingPieces(u.ID)
	if err != nil {
		return err
	}
	out := make([]WritingListItemDTO, 0, len(rows))
	for _, r := range rows {
		it := WritingListItemDTO{ID: r.ID, Title: r.Title, TopicCode: r.TopicCode, Level: r.Level,
			Words: countWords(r.Body), UpdatedAt: r.UpdatedAt.Format(time.RFC3339), Checks: r.Checks}
		if r.LastCheck != nil {
			it.LastCheck = &WritingLastCheckDTO{Source: r.LastCheck.Source, Model: r.LastCheck.Model,
				CreatedAt: r.LastCheck.CreatedAt.Format(time.RFC3339)}
		}
		out = append(out, it)
	}
	return c.JSON(WritingListResponse{Pieces: out})
}

func (s *Server) handleWritingCreate(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req WritingCreateRequest
	if len(c.Body()) > 0 {
		if err := decode(c, &req); err != nil {
			return err
		}
	}
	level := strings.ToUpper(strings.TrimSpace(req.Level))
	title := strings.TrimSpace(req.Title)
	if req.TopicCode != "" {
		t, ok := writing.FindTopic(req.TopicCode)
		if !ok {
			return writingErr(fiber.StatusNotFound, "errors.writingNoTopic")
		}
		if level == "" {
			level = t.Level
		}
		if title == "" {
			title = t.TitleRU
			if s.Config().Locale.UI == "en" {
				title = t.TitleEN
			}
		}
	}
	if !validLevel(level) {
		level = "A2"
	}
	if utf8.RuneCountInString(title) > writingTitleLimit {
		return writingErr(fiber.StatusBadRequest, "errors.writingTooLong")
	}
	p, err := s.db.CreateWritingPiece(u.ID, req.TopicCode, title, level)
	if err != nil {
		return err
	}
	dto, err := s.pieceDTO(u.ID, p, true)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(dto)
}

func validLevel(l string) bool {
	for _, v := range writing.Levels {
		if v == l {
			return true
		}
	}
	return false
}

func (s *Server) handleWritingGet(c fiber.Ctx) error {
	u, p, err := s.ownedPiece(c)
	if err != nil {
		return err
	}
	dto, err := s.pieceDTO(u.ID, p, true)
	if err != nil {
		return err
	}
	return c.JSON(dto)
}

func (s *Server) handleWritingUpdate(c fiber.Ctx) error {
	u, p, err := s.ownedPiece(c)
	if err != nil {
		return err
	}
	var req WritingUpdateRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	title := strings.TrimSpace(req.Title)
	if utf8.RuneCountInString(req.Body) > writingBodyLimit || utf8.RuneCountInString(title) > writingTitleLimit {
		return writingErr(fiber.StatusBadRequest, "errors.writingTooLong")
	}
	p, err = s.db.UpdateWritingPiece(u.ID, p.ID, title, strings.ReplaceAll(req.Body, "\r\n", "\n"))
	if err != nil {
		return err
	}
	dto, err := s.pieceDTO(u.ID, p, false)
	if err != nil {
		return err
	}
	return c.JSON(dto)
}

func (s *Server) handleWritingDelete(c fiber.Ctx) error {
	u, p, err := s.ownedPiece(c)
	if err != nil {
		return err
	}
	if err := s.db.DeleteWritingPiece(u.ID, p.ID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// pieceDTO is the piece with its history. Summaries cost one Parse each; only
// when withLatest is set is the newest check built in full (anchoring,
// attestation). Autosave answers with withLatest=false and latest=null, so
// typing never pays for a check's markup.
func (s *Server) pieceDTO(userID int64, p db.WritingPiece, withLatest bool) (WritingPieceDTO, error) {
	checks, err := s.db.WritingChecks(userID, p.ID)
	if err != nil {
		return WritingPieceDTO{}, err
	}
	out := WritingPieceDTO{ID: p.ID, Title: p.Title, TopicCode: p.TopicCode, Level: p.Level, Body: p.Body,
		Words: countWords(p.Body), CreatedAt: p.CreatedAt.Format(time.RFC3339), UpdatedAt: p.UpdatedAt.Format(time.RFC3339),
		Checks: []WritingCheckSummaryDTO{}}
	for i, ck := range checks {
		res := writing.Parse(ck.Raw)
		out.Checks = append(out.Checks, WritingCheckSummaryDTO{ID: ck.ID, Source: ck.Source, Model: ck.Model,
			Lang: ck.Lang, CreatedAt: ck.CreatedAt.Format(time.RFC3339), Status: res.Status, Truncated: res.Truncated,
			Interrupted: ck.Interrupted, Level: res.Level, ErrorCount: len(res.Errors)})
		if i == 0 && withLatest {
			f := s.checkDTO(ck, s.wordExists())
			out.Latest = &f
		}
	}
	return out, nil
}

// checkDTO rebuilds a check from what is stored: parse, anchor, attest, link.
func (s *Server) checkDTO(ck db.WritingCheck, exists func(string) bool) WritingCheckDTO {
	res := writing.Parse(ck.Raw)
	segs, anchored := writing.Anchor(ck.CheckedText, res.Errors)
	out := WritingCheckDTO{
		ID: ck.ID, Source: ck.Source, Model: ck.Model, Lang: ck.Lang, CreatedAt: ck.CreatedAt.Format(time.RFC3339),
		Status: res.Status, Truncated: res.Truncated, Interrupted: ck.Interrupted, Dropped: res.Dropped,
		Level: res.Level, Summary: res.Summary, Strengths: nonNil(res.Strengths), Vocab: nonNil(res.Vocab),
		Corrected: res.Corrected, Raw: ck.Raw,
		Errors:   make([]WritingErrorDTO, 0, len(res.Errors)),
		Segments: make([]WritingSegmentDTO, 0, len(segs)),
	}
	for i, e := range res.Errors {
		d := WritingErrorDTO{N: e.N, Quote: e.Quote, Fix: e.Fix, Type: e.Type, Hint: e.Hint, Why: e.Why,
			Rule: e.Rule, Example: e.Example, Anchored: anchored[i],
			Unattested: tutor.Unattested(hungarianWords(e.Fix, exampleHungarian(e.Example)), exists)}
		// The lesson link only when the installed pack really has the topic.
		if code := writing.GrammarTopicHint(e.Type); code != "" && s.dag != nil {
			if t, ok := s.dag.Topic(code); ok {
				d.TopicCode = code
				d.TopicTitle = t.TitleRU
				if ck.Lang == "en" {
					d.TopicTitle = t.TitleEN
				}
			}
		}
		out.Errors = append(out.Errors, d)
	}
	for _, sg := range segs {
		out.Segments = append(out.Segments, WritingSegmentDTO{Text: sg.Text, Errors: nonNilInts(sg.Errors)})
	}
	return out
}

func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func nonNilInts(in []int) []int {
	if in == nil {
		return []int{}
	}
	return in
}

// exampleHungarian is the Hungarian half of "sentence — translation".
func exampleHungarian(ex string) string {
	for _, sep := range []string{" — ", " – ", " - ", " — "} {
		if i := strings.Index(ex, sep); i >= 0 {
			return ex[:i]
		}
	}
	return ex
}

// hungarianWords are the distinct Latin-script words of the texts, three
// letters or more (shorter ones are mostly particles and suffix fragments).
func hungarianWords(texts ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range texts {
		for _, w := range strings.FieldsFunc(t, func(r rune) bool { return !unicode.IsLetter(r) }) {
			lw := strings.ToLower(w)
			if utf8.RuneCountInString(w) < 3 || !latinOnly(w) || seen[lw] {
				continue
			}
			seen[lw] = true
			out = append(out, w)
		}
	}
	return out
}

func (s *Server) handleWritingPrompt(c fiber.Ctx) error {
	_, p, err := s.ownedPiece(c)
	if err != nil {
		return err
	}
	var req WritingPromptRequest
	if len(c.Body()) > 0 {
		if err := decode(c, &req); err != nil {
			return err
		}
	}
	text := strings.TrimSpace(p.Body)
	if text == "" {
		return writingErr(fiber.StatusBadRequest, "errors.writingEmpty")
	}
	return c.JSON(WritingPromptResponse{
		Prompt: writing.Build(s.promptParams(p, s.writingLang(req.Lang), text)), Text: text,
		PromptVersion: writing.PromptVersion,
	})
}

func (s *Server) promptParams(p db.WritingPiece, lang, text string) writing.Params {
	pr := writing.Params{Lang: lang, Level: p.Level, Text: text}
	if t, ok := writing.FindTopic(p.TopicCode); ok {
		pr.TopicTitle, pr.TopicBrief = t.TitleEN, t.BriefEN
		if lang == "ru" {
			pr.TopicTitle, pr.TopicBrief = t.TitleRU, t.BriefRU
		}
		pr.MinWords, pr.MaxWords = t.MinWords, t.MaxWords
	}
	return pr
}

func (s *Server) handleWritingExternal(c fiber.Ctx) error {
	u, p, err := s.ownedPiece(c)
	if err != nil {
		return err
	}
	var req WritingExternalRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if len(req.Raw) > writingRawLimit || utf8.RuneCountInString(req.Text) > writingBodyLimit {
		return writingErr(fiber.StatusRequestEntityTooLarge, "errors.writingTooLong")
	}
	if strings.TrimSpace(req.Raw) == "" || writing.Parse(req.Raw).Status == writing.StatusFailed {
		return writingErr(fiber.StatusUnprocessableEntity, "errors.writingUnparseable")
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		text = strings.TrimSpace(p.Body)
	}
	model := clipRunes(strings.TrimSpace(req.Model), writingModelLimit)
	if model == "" {
		model = "external"
	}
	ck, err := s.db.AddWritingCheck(u.ID, db.WritingCheck{PieceID: p.ID, Source: "external", Model: model,
		Lang: s.writingLang(req.Lang), PromptVersion: writing.PromptVersion, CheckedText: text, Raw: req.Raw})
	if err != nil {
		return s.writingStoreErr(err)
	}
	return c.Status(fiber.StatusCreated).JSON(s.checkDTO(ck, s.wordExists()))
}

func (s *Server) writingStoreErr(err error) error {
	switch {
	case errors.Is(err, db.ErrTooManyChecks):
		return writingErr(fiber.StatusConflict, "errors.writingTooManyChecks")
	case errors.Is(err, db.ErrNotFound):
		return writingErr(fiber.StatusNotFound, "errors.writingNotFound")
	}
	return err
}

func (s *Server) handleWritingCheckGet(c fiber.Ctx) error {
	u, p, err := s.ownedPiece(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Params("checkId"), 10, 64)
	if err != nil {
		return writingErr(fiber.StatusNotFound, "errors.writingNotFound")
	}
	ck, err := s.db.WritingCheck(u.ID, id)
	if err != nil || ck.PieceID != p.ID {
		if err == nil || errors.Is(err, db.ErrNotFound) {
			return writingErr(fiber.StatusNotFound, "errors.writingNotFound")
		}
		return err
	}
	return c.JSON(s.checkDTO(ck, s.wordExists()))
}

func (s *Server) handleWritingCheckDelete(c fiber.Ctx) error {
	u, p, err := s.ownedPiece(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Params("checkId"), 10, 64)
	if err != nil {
		return writingErr(fiber.StatusNotFound, "errors.writingNotFound")
	}
	if err := s.db.DeleteWritingCheck(u.ID, p.ID, id); errors.Is(err, db.ErrNotFound) {
		return writingErr(fiber.StatusNotFound, "errors.writingNotFound")
	} else if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

var reWritingSection = regexp.MustCompile(`(?im)^\W*\[\s*(ERROR|VOCAB|CORRECTED|END)\s*\d*\s*\]`)

// completeErrorBlocks counts [ERROR] blocks that a later section header has
// already closed: the one still being written is not counted.
func completeErrorBlocks(answer string) int {
	heads := reWritingSection.FindAllStringSubmatch(answer, -1)
	n := 0
	for i := 0; i+1 < len(heads); i++ {
		if strings.EqualFold(heads[i][1], "ERROR") {
			n++
		}
	}
	return n
}

// truncateBytes cuts s to at most n bytes on a rune boundary.
func truncateBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// handleWritingCheckLocal runs the check on the configured model, streamed as
// SSE: `progress {chars, errors}` about once a second, then `done {check}` or
// `error {error}`. The answer is stored by a goroutine the server waits for,
// whether or not the browser is still there; the learner paid minutes for it.
func (s *Server) handleWritingCheckLocal(c fiber.Ctx) error {
	u, p, err := s.ownedPiece(c)
	if err != nil {
		return err
	}
	var req WritingLocalRequest
	if len(c.Body()) > 0 {
		if err := decode(c, &req); err != nil {
			return err
		}
	}
	text := strings.TrimSpace(p.Body)
	if text == "" {
		return writingErr(fiber.StatusBadRequest, "errors.writingEmpty")
	}
	if checks, err := s.db.WritingChecks(u.ID, p.ID); err != nil {
		return err
	} else if len(checks) >= db.MaxChecksPerPiece {
		return writingErr(fiber.StatusConflict, "errors.writingTooManyChecks")
	}
	client, err := llm.New(s.service("llm"), s.llmKey())
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, llmUnavailableKey(err))
	}
	flightCtx, stop := context.WithCancel(s.bgCtx)
	if _, busy := s.writingFlights.LoadOrStore(u.ID, &writingFlight{piece: p.ID, stop: stop}); busy {
		stop()
		return writingErr(fiber.StatusConflict, "errors.writingBusy")
	}
	release := func() { stop(); s.writingFlights.Delete(u.ID) }

	lang := s.writingLang(req.Lang)
	prompt := writing.Build(s.promptParams(p, lang, text))
	model := clipRunes(client.Model(), writingModelLimit)
	userID, pieceID := u.ID, p.ID
	exists := s.wordExists()
	s.bg.Add(1)

	c.Set(fiber.HeaderContentType, "text/event-stream; charset=utf-8")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set("X-Accel-Buffering", "no")
	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer s.bg.Done()
		defer release()
		ctx, cancel := context.WithTimeout(flightCtx, writingStreamTimeout)
		defer cancel()

		clientGone := false
		send := func(event string, payload any) {
			if clientGone {
				return
			}
			data, _ := json.Marshal(payload)
			if _, err := w.WriteString("event: " + event + "\ndata: " + string(data) + "\n\n"); err != nil {
				clientGone = true
				return
			}
			if err := w.Flush(); err != nil {
				clientGone = true
			}
		}

		var answer strings.Builder
		var lastProgress time.Time
		_, streamErr := client.Stream(ctx, []llm.Message{{Role: "user", Content: prompt}},
			writingMaxTokens, writingTemperature, func(delta string) error {
				answer.WriteString(delta)
				if time.Since(lastProgress) >= writingProgressEvery {
					lastProgress = time.Now()
					send("progress", map[string]int{
						"chars": utf8.RuneCountInString(answer.String()), "errors": completeErrorBlocks(answer.String())})
				}
				// A departed browser does not stop the model: what it writes
				// is stored for the next visit.
				return nil
			})

		raw := answer.String()
		if strings.TrimSpace(raw) == "" {
			slog.Warn("writing check produced nothing", "user", userID, "piece", pieceID, "error", streamErr)
			send("error", map[string]string{"error": "errors.writingCheckFailed"})
			return
		}
		interrupted := streamErr != nil
		if len(raw) > writingRawLimit {
			raw, interrupted = truncateBytes(raw, writingRawLimit), true
		}
		ck, err := s.db.AddWritingCheck(userID, db.WritingCheck{PieceID: pieceID, Source: "local", Model: model,
			Lang: lang, PromptVersion: writing.PromptVersion, CheckedText: text, Raw: raw, Interrupted: interrupted})
		if err != nil {
			slog.Error("could not store writing check", "error", err)
			key := "errors.writingCheckFailed"
			if errors.Is(err, db.ErrTooManyChecks) {
				key = "errors.writingTooManyChecks"
			} else if errors.Is(err, db.ErrNotFound) {
				key = "errors.writingNotFound" // the piece was deleted meanwhile
			}
			send("error", map[string]string{"error": key})
			return
		}
		send("done", map[string]any{"check": s.checkDTO(ck, exists)})
	})
}

// handleWritingCheckStop ends the learner's in-flight local check. What the
// model wrote so far is kept, marked interrupted. Idempotent.
func (s *Server) handleWritingCheckStop(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if v, ok := s.writingFlights.Load(u.ID); ok {
		if f, ok := v.(*writingFlight); ok {
			f.stop()
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}
