package api

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/config"
	"tanulom/internal/db"
	"tanulom/internal/writing"
)

// «Письмо» (ADR-085): pieces, checks, and the promise that none of it makes a card.

const writingAnswer = "TANULOM-CHECK 1\nLEVEL: A1\nSUMMARY: Jó.\nSTRENGTH: Rendben.\n\n" +
	"[ERROR]\nQUOTE: boltban mentem\nNTH: 1\nTYPE: case\nFIX: boltba mentem\nHINT: Hova?\nWHY: -ba.\nRULE: Illative.\n" +
	"EXAMPLE: A parkba megyek. — Я иду в парк.\n\n" +
	"[ERROR]\nQUOTE: xyzzyq\nNTH: 1\nTYPE: spelling\nFIX: xyzzyq\nHINT: ?\nWHY: ?\nRULE: ?\nEXAMPLE: ?\n\n" +
	"[CORRECTED]\nTegnap a boltba mentem.\n\n[END]\n"

const writingEssay = "Tegnap boltban mentem. Vettem kenyeret."

func newPiece(t *testing.T, c *client, body string) WritingPieceDTO {
	t.Helper()
	var p WritingPieceDTO
	if resp := c.json(http.MethodPost, "/api/writing", WritingCreateRequest{}, &p); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d", resp.StatusCode)
	}
	if resp := c.json(http.MethodPut, fmt.Sprintf("/api/writing/%d", p.ID), WritingUpdateRequest{Title: "T", Body: body}, &p); resp.StatusCode != http.StatusOK {
		t.Fatalf("update: %d", resp.StatusCode)
	}
	return p
}

func TestWritingCRUDOwnershipAndLimits(t *testing.T) {
	t.Parallel()
	srv, c := newPackedServer(t)
	addUser(t, c, "bela@example.com", goodPassword, "learner")
	other := signIn(t, srv, "bela@example.com", goodPassword)

	var topics WritingTopicsResponse
	if resp := c.json(http.MethodGet, "/api/writing/topics", nil, &topics); resp.StatusCode != 200 || len(topics.Topics) < 20 {
		t.Fatalf("topics: %d, %d", resp.StatusCode, len(topics.Topics))
	}
	first := topics.Topics[0]
	var p WritingPieceDTO
	if resp := c.json(http.MethodPost, "/api/writing", WritingCreateRequest{TopicCode: first.Code}, &p); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d", resp.StatusCode)
	}
	if p.Title != first.TitleRU || p.Level != first.Level || p.Checks == nil || p.Latest != nil {
		t.Errorf("created = %+v", p)
	}
	if resp := c.json(http.MethodPost, "/api/writing", WritingCreateRequest{TopicCode: "nope"}, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown topic: %d", resp.StatusCode)
	}
	url := fmt.Sprintf("/api/writing/%d", p.ID)

	// Autosave survives a reload.
	c.json(http.MethodPut, url, WritingUpdateRequest{Title: "Cím", Body: "Szia világ"}, nil)
	var got WritingPieceDTO
	c.json(http.MethodGet, url, nil, &got)
	if got.Body != "Szia világ" || got.Title != "Cím" || got.Words != 2 {
		t.Errorf("reload = %+v", got)
	}
	var list WritingListResponse
	c.json(http.MethodGet, "/api/writing", nil, &list)
	if len(list.Pieces) != 1 || list.Pieces[0].Words != 2 || list.Pieces[0].LastCheck != nil {
		t.Errorf("list = %+v", list)
	}

	// Limits.
	if resp := c.json(http.MethodPut, url, WritingUpdateRequest{Body: strings.Repeat("á", 6001)}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("long body: %d", resp.StatusCode)
	}
	if resp := c.json(http.MethodPut, url, WritingUpdateRequest{Title: strings.Repeat("c", 121)}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("long title: %d", resp.StatusCode)
	}

	// Another learner sees none of it: 404 on every route.
	for _, r := range [][2]string{
		{http.MethodGet, url}, {http.MethodPut, url}, {http.MethodDelete, url},
		{http.MethodPost, url + "/prompt"}, {http.MethodPost, url + "/checks/external"},
		{http.MethodPost, url + "/checks/local"}, {http.MethodDelete, url + "/checks/1"}, {http.MethodGet, url + "/checks/1"},
	} {
		if resp, _ := other.do(r[0], r[1], map[string]string{"raw": writingAnswer}); resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s %s by another learner: %d, want 404", r[0], r[1], resp.StatusCode)
		}
	}
	var theirs WritingListResponse
	other.json(http.MethodGet, "/api/writing", nil, &theirs)
	if len(theirs.Pieces) != 0 {
		t.Errorf("another learner lists %d pieces", len(theirs.Pieces))
	}
	if resp, _ := (&client{t: t, app: srv}).do(http.MethodGet, "/api/writing", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous: %d", resp.StatusCode)
	}

	if resp := c.json(http.MethodDelete, url, nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete: %d", resp.StatusCode)
	}
	if resp := c.json(http.MethodGet, url, nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("after delete: %d", resp.StatusCode)
	}
}

func TestWritingPromptStoresNothingAndCarriesTheEssay(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	p := newPiece(t, c, writingEssay)
	var pr WritingPromptResponse
	if resp := c.json(http.MethodPost, fmt.Sprintf("/api/writing/%d/prompt", p.ID), WritingPromptRequest{Lang: "ru"}, &pr); resp.StatusCode != 200 {
		t.Fatalf("prompt: %d", resp.StatusCode)
	}
	if pr.Text != writingEssay || pr.PromptVersion != writing.PromptVersion ||
		!strings.Contains(pr.Prompt, "<<<\n"+writingEssay+"\n>>>") || !strings.Contains(pr.Prompt, "Russian") {
		t.Errorf("prompt response = %q", pr.Prompt)
	}
	var got WritingPieceDTO
	c.json(http.MethodGet, fmt.Sprintf("/api/writing/%d", p.ID), nil, &got)
	if len(got.Checks) != 0 {
		t.Error("building a prompt stored a check")
	}
	empty := newPiece(t, c, "  ")
	if resp := c.json(http.MethodPost, fmt.Sprintf("/api/writing/%d/prompt", empty.ID), WritingPromptRequest{}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty text prompt: %d", resp.StatusCode)
	}
}

func TestWritingExternalParsesStoresAndRefusesNonsense(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	p := newPiece(t, c, writingEssay)
	url := fmt.Sprintf("/api/writing/%d/checks/external", p.ID)

	var body map[string]string
	if resp := c.json(http.MethodPost, url, WritingExternalRequest{Lang: "ru", Text: writingEssay, Raw: "Sorry, I cannot help with that."}, &body); resp.StatusCode != http.StatusUnprocessableEntity || body["error"] != "errors.writingUnparseable" {
		t.Errorf("nonsense: %d %v", resp.StatusCode, body)
	}
	var none WritingPieceDTO
	c.json(http.MethodGet, fmt.Sprintf("/api/writing/%d", p.ID), nil, &none)
	if len(none.Checks) != 0 {
		t.Fatal("a refused paste was stored")
	}

	// A whole chat message: chatter, a fence, bold keys.
	raw := "Here is your review!\n```text\n" + strings.ReplaceAll(writingAnswer, "QUOTE:", "**QUOTE:**") + "```\nGood luck."
	var ck WritingCheckDTO
	if resp := c.json(http.MethodPost, url, WritingExternalRequest{Lang: "ru", Text: writingEssay, Raw: raw, Model: "ChatGPT"}, &ck); resp.StatusCode != http.StatusCreated {
		t.Fatalf("external: %d", resp.StatusCode)
	}
	if ck.Source != "external" || ck.Model != "ChatGPT" || ck.Status != writing.StatusOK || len(ck.Errors) != 2 {
		t.Fatalf("check = %+v", ck)
	}
	e1, e2 := ck.Errors[0], ck.Errors[1]
	if !e1.Anchored || e1.Quote != "boltban mentem" || e1.Type != "case" {
		t.Errorf("error 1 = %+v", e1)
	}
	if len(e2.Unattested) != 1 || e2.Unattested[0] != "xyzzyq" {
		t.Errorf("a word the dictionary lacks is flagged: %v", e2.Unattested)
	}
	if e2.Anchored {
		t.Errorf("a quote not in the text must not anchor: %+v", e2)
	}
	if ck.Corrected != "Tegnap a boltba mentem." || ck.Level != "A1" {
		t.Errorf("corrected/level = %q %q", ck.Corrected, ck.Level)
	}
	var joined strings.Builder
	covered := false
	for _, sg := range ck.Segments {
		joined.WriteString(sg.Text)
		if len(sg.Errors) == 1 && sg.Errors[0] == 0 && sg.Text == "boltban mentem" {
			covered = true
		}
	}
	if joined.String() != writingEssay || !covered {
		t.Errorf("segments = %+v", ck.Segments)
	}
	// Default label, and the history on the piece.
	var ck2 WritingCheckDTO
	c.json(http.MethodPost, url, WritingExternalRequest{Raw: writingAnswer}, &ck2)
	if ck2.Model != "external" || ck2.CreatedAt == "" {
		t.Errorf("default label = %q", ck2.Model)
	}
	var got WritingPieceDTO
	c.json(http.MethodGet, fmt.Sprintf("/api/writing/%d", p.ID), nil, &got)
	if len(got.Checks) != 2 || got.Latest == nil || got.Latest.ID != ck2.ID || got.Checks[0].ErrorCount != 2 {
		t.Errorf("history = %+v", got.Checks)
	}
	var full WritingCheckDTO
	if resp := c.json(http.MethodGet, fmt.Sprintf("/api/writing/%d/checks/%d", p.ID, ck.ID), nil, &full); resp.StatusCode != 200 || full.Raw != raw {
		t.Errorf("one check: %d", resp.StatusCode)
	}
	if resp := c.json(http.MethodDelete, fmt.Sprintf("/api/writing/%d/checks/%d", p.ID, ck.ID), nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete check: %d", resp.StatusCode)
	}
}

func TestWritingTopicLinkOnlyWhenThePackHasTheLesson(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	p := newPiece(t, c, writingEssay)
	raw := "TANULOM-CHECK 1\n[ERROR]\nQUOTE: boltban mentem\nTYPE: case\nFIX: boltba mentem\n\n" +
		"[ERROR]\nQUOTE: kenyeret\nTYPE: harmony\nFIX: kenyeret\n\n[END]\n"
	var ck WritingCheckDTO
	c.json(http.MethodPost, fmt.Sprintf("/api/writing/%d/checks/external", p.ID), WritingExternalRequest{Raw: raw, Lang: "en"}, &ck)
	if len(ck.Errors) != 2 {
		t.Fatalf("errors = %+v", ck.Errors)
	}
	// "case" maps to locative-grid, which the test pack has; "harmony" maps to
	// vowel-harmony, which it does not.
	if ck.Errors[0].TopicCode != "locative-grid" || ck.Errors[0].TopicTitle == "" {
		t.Errorf("case link = %+v", ck.Errors[0])
	}
	if ck.Errors[1].TopicCode != "" {
		t.Errorf("a lesson the pack lacks was linked: %+v", ck.Errors[1])
	}
}

func TestWritingCheckCapIs30(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	p := newPiece(t, c, writingEssay)
	url := fmt.Sprintf("/api/writing/%d/checks/external", p.ID)
	for i := 0; i < db.MaxChecksPerPiece; i++ {
		if resp := c.json(http.MethodPost, url, WritingExternalRequest{Raw: writingAnswer}, nil); resp.StatusCode != http.StatusCreated {
			t.Fatalf("check %d: %d", i, resp.StatusCode)
		}
	}
	var body map[string]string
	if resp := c.json(http.MethodPost, url, WritingExternalRequest{Raw: writingAnswer}, &body); resp.StatusCode != http.StatusConflict || body["error"] != "errors.writingTooManyChecks" {
		t.Errorf("31st: %d %v", resp.StatusCode, body)
	}
	if resp := c.json(http.MethodPost, url, WritingExternalRequest{Raw: strings.Repeat("x", 64<<10+1)}, nil); resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("huge paste: %d", resp.StatusCode)
	}
}

func writingDone(t *testing.T, events [][2]string) WritingCheckDTO {
	t.Helper()
	if len(events) == 0 || events[len(events)-1][0] != "done" {
		t.Fatalf("want a trailing done, got %v", events)
	}
	var d struct {
		Check WritingCheckDTO `json:"check"`
	}
	if err := jsonUnmarshalString(events[len(events)-1][1], &d); err != nil {
		t.Fatal(err)
	}
	return d.Check
}

func TestWritingLocalCheckStreamsAndStores(t *testing.T) {
	t.Parallel()
	srv, c := newPackedServer(t)
	fake, lastPrompt := fakeLLM(t, writingAnswer)
	enableLLM(t, srv, fake.URL)
	p := newPiece(t, c, writingEssay)

	resp, body := c.do(http.MethodPost, fmt.Sprintf("/api/writing/%d/checks/local", p.ID), WritingLocalRequest{Lang: "en"})
	if resp.StatusCode != 200 {
		t.Fatalf("local: %d %s", resp.StatusCode, body)
	}
	events := parseSSE(string(body))
	if events[0][0] != "progress" {
		t.Errorf("first event = %v, want progress", events[0])
	}
	ck := writingDone(t, events)
	if ck.Source != "local" || ck.Model != "test-model" || ck.Interrupted || ck.Status != writing.StatusOK || len(ck.Errors) != 2 || ck.Lang != "en" {
		t.Errorf("check = %+v", ck)
	}
	if pr := lastPrompt(); !strings.Contains(pr, "<<<\n"+writingEssay+"\n>>>") || !strings.Contains(pr, "English") {
		t.Errorf("the model was not sent the prompt: %q", pr)
	}
	var got WritingPieceDTO
	c.json(http.MethodGet, fmt.Sprintf("/api/writing/%d", p.ID), nil, &got)
	if len(got.Checks) != 1 || got.Checks[0].Source != "local" {
		t.Errorf("stored = %+v", got.Checks)
	}
}

func TestWritingLocalCheckKeepsATruncatedAnswerAsInterrupted(t *testing.T) {
	t.Parallel()
	srv, c := newPackedServer(t)
	cut := "TANULOM-CHECK 1\\nSUMMARY: Jó.\\n\\n[ERROR]\\nQUOTE: boltban mentem\\nTYPE: case\\nFIX: boltba mentem\\nWHY: -ba\\n\\n[ERROR]\\nQUOTE: Vet"
	fakeStreamer(t, srv, `data: {"choices":[{"delta":{"content":"`+cut+`"},"finish_reason":"length"}]}`+"\n\ndata: [DONE]\n\n", 0)
	p := newPiece(t, c, writingEssay)
	_, body := c.do(http.MethodPost, fmt.Sprintf("/api/writing/%d/checks/local", p.ID), WritingLocalRequest{})
	ck := writingDone(t, parseSSE(string(body)))
	if !ck.Interrupted || ck.Status != writing.StatusPartial || !ck.Truncated || len(ck.Errors) != 1 {
		t.Errorf("truncated check = interrupted %v status %s truncated %v errors %d", ck.Interrupted, ck.Status, ck.Truncated, len(ck.Errors))
	}
}

func TestWritingLocalCheckWithNoAnswerStoresNothing(t *testing.T) {
	t.Parallel()
	srv, c := newPackedServer(t)
	fakeStreamer(t, srv, "data: [DONE]\n\n", 0)
	p := newPiece(t, c, writingEssay)
	_, body := c.do(http.MethodPost, fmt.Sprintf("/api/writing/%d/checks/local", p.ID), WritingLocalRequest{})
	ev := parseSSE(string(body))
	if len(ev) == 0 || ev[len(ev)-1][0] != "error" || !strings.Contains(ev[len(ev)-1][1], "errors.writingCheckFailed") {
		t.Fatalf("events = %v", ev)
	}
	var got WritingPieceDTO
	c.json(http.MethodGet, fmt.Sprintf("/api/writing/%d", p.ID), nil, &got)
	if len(got.Checks) != 0 {
		t.Error("an empty answer was stored")
	}
}

func TestWritingLocalCheckIsOffWithoutAModel(t *testing.T) {
	t.Parallel()
	srv, c := newPackedServer(t)
	p := newPiece(t, c, writingEssay)
	url := fmt.Sprintf("/api/writing/%d/checks/local", p.ID)
	var body map[string]string
	if resp := c.json(http.MethodPost, url, WritingLocalRequest{}, &body); resp.StatusCode != http.StatusServiceUnavailable || body["error"] != "errors.llmOff" {
		t.Errorf("llm off: %d %v", resp.StatusCode, body)
	}
	if _, err := srv.updateConfig(func(cfg *config.Config) {
		cfg.Services.LLM = config.Service{Provider: config.ProviderLocal, Endpoint: "http://localhost:1"}
	}); err != nil {
		t.Fatal(err)
	}
	if resp := c.json(http.MethodPost, url, WritingLocalRequest{}, &body); resp.StatusCode != http.StatusServiceUnavailable || body["error"] != "errors.llmNoModel" {
		t.Errorf("no model: %d %v", resp.StatusCode, body)
	}
}

// gatedLLM answers its first chunk at once and the rest only after release.
func gatedLLM(t *testing.T, first, rest string) (url string, started chan struct{}, release func()) {
	t.Helper()
	started = make(chan struct{}, 4)
	gate := make(chan struct{})
	var once sync.Once
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":` + jsonString(first) + `}}]}` + "\n\n"))
		w.(http.Flusher).Flush()
		started <- struct{}{}
		select {
		case <-gate:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":` + jsonString(rest) + `}}]}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(fake.Close)
	return fake.URL, started, func() { once.Do(func() { close(gate) }) }
}

func TestWritingSingleFlightAndStop(t *testing.T) {
	t.Parallel()
	srv, c := newPackedServer(t)
	url, started, release := gatedLLM(t, writingAnswer[:60], writingAnswer[60:])
	t.Cleanup(release)
	enableLLM(t, srv, url)
	p := newPiece(t, c, writingEssay)
	path := fmt.Sprintf("/api/writing/%d/checks/local", p.ID)

	var wg sync.WaitGroup
	var body []byte
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, body = c.do(http.MethodPost, path, WritingLocalRequest{})
	}()
	<-started
	var busy map[string]string
	if resp := c.json(http.MethodPost, path, WritingLocalRequest{}, &busy); resp.StatusCode != http.StatusConflict || busy["error"] != "errors.writingBusy" {
		t.Errorf("second check: %d %v", resp.StatusCode, busy)
	}
	if resp := c.json(http.MethodPost, "/api/writing/checks/stop", nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("stop: %d", resp.StatusCode)
	}
	wg.Wait()
	ck := writingDone(t, parseSSE(string(body)))
	if !ck.Interrupted || ck.Raw != writingAnswer[:60] {
		t.Errorf("stopped check = interrupted %v raw %q", ck.Interrupted, ck.Raw)
	}
	// The flight was released: stopping again is a no-op and a new check runs.
	if resp := c.json(http.MethodPost, "/api/writing/checks/stop", nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("idempotent stop: %d", resp.StatusCode)
	}
}

// The browser leaving does not lose the answer: it is stored when the model
// finishes.
func TestWritingAnswerIsStoredWhenTheClientLeaves(t *testing.T) {
	t.Parallel()
	srv, c := newPackedServer(t)
	url, started, release := gatedLLM(t, writingAnswer[:60], writingAnswer[60:])
	t.Cleanup(release)
	enableLLM(t, srv, url)
	p := newPiece(t, c, writingEssay)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.App().Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	payload := `{"lang":"ru"}`
	fmt.Fprintf(conn, "POST /api/writing/%d/checks/local HTTP/1.1\r\nHost: x\r\nCookie: %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s",
		p.ID, c.cookie, len(payload), payload)
	rd := bufio.NewReader(conn)
	_ = conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	sawProgress := false
	for !sawProgress {
		line, err := rd.ReadString('\n')
		if err != nil {
			t.Fatalf("before the first progress event: %v", err)
		}
		sawProgress = strings.HasPrefix(line, "event: progress")
	}
	<-started
	conn.Close() // the learner closed the tab
	time.Sleep(100 * time.Millisecond)
	release()

	deadline := time.Now().Add(20 * time.Second)
	for {
		var got WritingPieceDTO
		c.json(http.MethodGet, fmt.Sprintf("/api/writing/%d", p.ID), nil, &got)
		if len(got.Checks) == 1 {
			if got.Latest == nil || got.Latest.Raw != writingAnswer || got.Latest.Interrupted {
				t.Errorf("stored after the client left = %+v", got.Latest)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the answer was lost when the client left")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestWritingProgressCountsClosedErrorBlocks(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]int{
		"": 0, "TANULOM-CHECK 1\n[ERROR]\nQUOTE: a": 0,
		"[ERROR]\nQUOTE: a\n\n[ERROR]\nQUOTE: b":              1,
		"[ERROR]\nQUOTE: a\n\n[ERROR]\nQUOTE: b\n\n[VOCAB]\n": 2,
	} {
		if got := completeErrorBlocks(in); got != want {
			t.Errorf("completeErrorBlocks(%q) = %d, want %d", in, got, want)
		}
	}
}

// The check marks; it mints nothing. Every writing endpoint leaves the
// scheduling and card tables as it found them.
func TestWritingMintsNothing(t *testing.T) {
	t.Parallel()
	srv, c := newPackedServer(t)
	fake, _ := fakeLLM(t, writingAnswer)
	enableLLM(t, srv, fake.URL)
	tables := []string{"note", "card", "user_gloss", "lemma_state", "morph_state", "review_log",
		"feature_review", "practice_log", "daily_log", "topic_progress", "drill_attempt", "note_origin", "tutor_thread", "tutor_message"}
	// Give the tables something to lose: one learned word.
	count := func() map[string]int {
		out := map[string]int{}
		for _, tb := range tables {
			var n int
			if err := srv.db.QueryRow(`SELECT COUNT(*) FROM ` + tb).Scan(&n); err != nil {
				t.Fatalf("%s: %v", tb, err)
			}
			out[tb] = n
		}
		return out
	}
	before := count()

	p := newPiece(t, c, writingEssay)
	base := fmt.Sprintf("/api/writing/%d", p.ID)
	c.do(http.MethodGet, "/api/writing/topics", nil)
	c.do(http.MethodGet, "/api/writing", nil)
	c.do(http.MethodPost, base+"/prompt", WritingPromptRequest{Lang: "ru"})
	_, extBody := c.do(http.MethodPost, base+"/checks/external", WritingExternalRequest{Raw: writingAnswer})
	if !strings.Contains(string(extBody), `"anchored"`) {
		t.Fatalf("external check: %s", extBody)
	}
	c.do(http.MethodPost, base+"/checks/local", WritingLocalRequest{})
	c.do(http.MethodPost, "/api/writing/checks/stop", nil)
	c.do(http.MethodGet, base, nil)
	c.do(http.MethodDelete, base, nil)

	after := count()
	for _, tb := range tables {
		if before[tb] != after[tb] {
			t.Errorf("%s: %d rows before, %d after — the writing check must mint nothing", tb, before[tb], after[tb])
		}
	}
}

// Autosave must not pay for check markup: PUT answers with summaries and no
// full latest check; GET still builds latest.
func TestWritingAutosaveDoesNotBuildChecks(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	p := newPiece(t, c, writingEssay)
	c.json(http.MethodPost, fmt.Sprintf("/api/writing/%d/checks/external", p.ID), WritingExternalRequest{Raw: writingAnswer}, nil)
	var put, get WritingPieceDTO
	c.json(http.MethodPut, fmt.Sprintf("/api/writing/%d", p.ID), WritingUpdateRequest{Title: "T", Body: writingEssay + " x"}, &put)
	c.json(http.MethodGet, fmt.Sprintf("/api/writing/%d", p.ID), nil, &get)
	if put.Latest != nil || len(put.Checks) != 1 || put.Checks[0].ErrorCount != 2 || put.Checks[0].Status != writing.StatusOK {
		t.Errorf("PUT = latest %v checks %+v", put.Latest, put.Checks)
	}
	if get.Latest == nil || len(get.Latest.Segments) == 0 {
		t.Error("GET must still carry the newest check in full")
	}
}
