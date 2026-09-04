package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"tanulom/internal/config"
	"tanulom/internal/tutor"
)

// The grammar tutor (ADR-027): grounded, streamed, labelled, and never a card.

// fakeLLM answers a streamed completion and records the prompt it was sent.
func fakeLLM(t *testing.T, answer string) (*httptest.Server, func() string) {
	t.Helper()
	var mu sync.Mutex
	var lastBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Stream   bool `json:"stream"`
			Messages []struct {
				Role, Content string
			} `json:"messages"`
		}
		raw := new(strings.Builder)
		dec := json.NewDecoder(r.Body)
		_ = dec.Decode(&req)
		for _, m := range req.Messages {
			raw.WriteString("[" + m.Role + "]\n" + m.Content + "\n")
		}
		mu.Lock()
		lastBody = raw.String()
		mu.Unlock()
		if !req.Stream {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":` + jsonString(answer) + `}}]}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		// Two deltas, then the terminator — the shape Ollama and OpenAI send.
		half := len(answer) / 2
		for half > 0 && half < len(answer) && answer[half]&0xC0 == 0x80 {
			half++
		}
		for _, part := range []string{answer[:half], answer[half:]} {
			_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":` + jsonString(part) + `}}]}` + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(srv.Close)
	return srv, func() string { mu.Lock(); defer mu.Unlock(); return lastBody }
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func enableLLM(t *testing.T, srv *Server, endpoint string) {
	t.Helper()
	if _, err := srv.updateConfig(func(cfg *config.Config) {
		cfg.Services.LLM.Provider = "openai"
		cfg.Services.LLM.Endpoint = endpoint
		cfg.Services.LLM.Model = "test-model"
	}); err != nil {
		t.Fatalf("configure llm: %v", err)
	}
}

// parseSSE splits an event stream into (event, data) pairs.
func parseSSE(body string) [][2]string {
	var out [][2]string
	for _, block := range strings.Split(body, "\n\n") {
		var ev, data string
		for _, line := range strings.Split(block, "\n") {
			if v, ok := strings.CutPrefix(line, "event: "); ok {
				ev = v
			}
			if v, ok := strings.CutPrefix(line, "data: "); ok {
				data = v
			}
		}
		if ev != "" {
			out = append(out, [2]string{ev, data})
		}
	}
	return out
}

// newThread opens a chat — pinned when topic is non-blank — and returns its id.
func newThread(t *testing.T, c *client, topic string) int64 {
	t.Helper()
	var out TutorThreadResponse
	if resp := c.json(http.MethodPost, "/api/tutor", TutorCreateRequest{Topic: topic}, &out); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create thread: %d", resp.StatusCode)
	}
	if out.Thread == nil || out.Thread.Topic != topic {
		t.Fatalf("created thread = %+v, want topic %q", out.Thread, topic)
	}
	return out.Thread.ID
}

// ask streams one question and returns the events.
func ask(t *testing.T, c *client, id int64, q, lang string) [][2]string {
	t.Helper()
	resp, body := c.do(http.MethodPost, fmt.Sprintf("/api/tutor/%d", id), TutorAskRequest{Question: q, Lang: lang})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ask %q: %d %s", q, resp.StatusCode, body)
	}
	return parseSSE(string(body))
}

func doneMessage(t *testing.T, events [][2]string) *tutor.Message {
	t.Helper()
	if len(events) == 0 || events[len(events)-1][0] != "done" {
		t.Fatalf("want a trailing done event, got %v", events)
	}
	var done tutorEvent
	if err := json.Unmarshal([]byte(events[len(events)-1][1]), &done); err != nil || done.Message == nil {
		t.Fatalf("done event: %v %s", err, events[len(events)-1][1])
	}
	return done.Message
}

func TestTutorIsOffUntilAModelIsConfigured(t *testing.T) {
	srv, c := newPackedServer(t)

	var thread TutorThreadResponse
	if resp := c.json(http.MethodGet, "/api/tutor/topic/locative-grid", nil, &thread); resp.StatusCode != http.StatusOK {
		t.Fatalf("thread: %d", resp.StatusCode)
	}
	if thread.Available || thread.Reason != "errors.llmOff" {
		t.Errorf("with no model the panel must say why: %+v", thread)
	}
	if thread.Thread != nil || thread.Topic != "locative-grid" {
		t.Errorf("no thread yet, but the pin to create one with: %+v", thread)
	}
	if len(thread.Sources) == 0 {
		t.Error("the corpus sources are listed even when the model is off")
	}
	id := newThread(t, c, "locative-grid")
	if resp := c.json(http.MethodPost, fmt.Sprintf("/api/tutor/%d", id),
		TutorAskRequest{Question: "why?"}, nil); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("asking with no model answered %d, want 503", resp.StatusCode)
	}
	if resp := c.json(http.MethodGet, "/api/tutor/topic/no-such-topic", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown topic answered %d, want 404", resp.StatusCode)
	}
	if resp := c.json(http.MethodPost, "/api/tutor", TutorCreateRequest{Topic: "no-such-topic"}, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("pinning an unknown topic answered %d, want 404", resp.StatusCode)
	}
	if resp := c.json(http.MethodGet, "/api/tutor/999", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown thread answered %d, want 404", resp.StatusCode)
	}
	// A new chat that has not been started yet asks the same question and
	// gets the same answer, with no thread and no pin.
	var fresh TutorThreadResponse
	if resp := c.json(http.MethodGet, "/api/tutor/new", nil, &fresh); resp.StatusCode != http.StatusOK {
		t.Fatalf("new: %d", resp.StatusCode)
	}
	if fresh.Available || fresh.Reason != "errors.llmOff" || fresh.Thread != nil || fresh.Topic != "" {
		t.Errorf("a fresh chat's availability: %+v", fresh)
	}

	// An endpoint with no model named is the other way to have no tutor, and
	// the panel must name THAT dial: this is the state a fresh Ollama lands in.
	if _, err := srv.updateConfig(func(cfg *config.Config) {
		cfg.Services.LLM = config.Service{Provider: config.ProviderLocal, Endpoint: "http://localhost:1"}
	}); err != nil {
		t.Fatal(err)
	}
	if resp := c.json(http.MethodGet, fmt.Sprintf("/api/tutor/%d", id), nil, &thread); resp.StatusCode != http.StatusOK {
		t.Fatalf("thread: %d", resp.StatusCode)
	}
	if thread.Available || thread.Reason != "errors.llmNoModel" {
		t.Errorf("with no model named the panel must say so: %+v", thread)
	}
	var body map[string]string
	if resp := c.json(http.MethodPost, fmt.Sprintf("/api/tutor/%d", id),
		TutorAskRequest{Question: "why?"}, &body); resp.StatusCode != http.StatusServiceUnavailable || body["error"] != "errors.llmNoModel" {
		t.Errorf("asking with no model named answered %d %v, want 503 errors.llmNoModel", resp.StatusCode, body)
	}
}

func TestTutorGroundsTheQuestionStreamsAndLabelsTheAnswer(t *testing.T) {
	srv, c := newPackedServer(t)
	fake, lastPrompt := fakeLLM(t, "Use `házban`, never `házben`. Also `xyzzy` is not a word.")
	enableLLM(t, srv, fake.URL)

	// Pretend the learner once typed the wrong form on a cloze for this topic.
	noteOn(t, c, map[string]any{"hu": "A házban van egy nagy könyv.", "targetForm": "házban"})
	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	for _, it := range queue.Items {
		if it.Direction == "cloze" {
			c.json(http.MethodPost, "/api/review/answer",
				AnswerRequest{CardID: it.CardID, Typed: "házben"}, nil)
		}
	}

	id := newThread(t, c, "locative-grid")
	resp, body := c.do(http.MethodPost, fmt.Sprintf("/api/tutor/%d", id),
		TutorAskRequest{Question: "Why -ban and not -ben in házban?", Lang: "en"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ask: %d %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("content type %q, want an event stream", ct)
	}
	events := parseSSE(string(body))
	if len(events) < 4 || events[0][0] != "asked" || events[1][0] != "delta" {
		t.Fatalf("want asked, deltas, then done, got %v", events)
	}
	msg := doneMessage(t, events)
	// The stored question's id opens the stream, so the client can offer an
	// edit of it without a reload (ADR-031).
	var asked tutorEvent
	if err := json.Unmarshal([]byte(events[0][1]), &asked); err != nil || asked.ID <= 0 {
		t.Errorf("asked event = %s (%v), want the question's id", events[0][1], err)
	}
	if msg.ID != asked.ID+1 {
		t.Errorf("the answer follows the question: asked %d, answer %d", asked.ID, msg.ID)
	}
	if msg.Model != "test-model" {
		t.Errorf("the answer must name the model, got %q", msg.Model)
	}
	if msg.Topic != "locative-grid" || msg.TopicTitle != "en locative-grid" {
		t.Errorf("a pinned thread grounds every answer in its topic, got %q / %q", msg.Topic, msg.TopicTitle)
	}
	if !strings.Contains(msg.Content, "házban") {
		t.Errorf("stored answer = %q", msg.Content)
	}
	// The test pack knows házban; házben and xyzzy it does not.
	if strings.Join(msg.Unattested, ",") != "házben,xyzzy" {
		t.Errorf("unattested = %v, want [házben xyzzy]", msg.Unattested)
	}
	if msg.Interrupted {
		t.Error("a completed stream is not interrupted")
	}

	// The prompt: rules first, the pack's lesson, the learner's mistake, the
	// question and the language line LAST.
	prompt := lastPrompt()
	for _, needle := range []string{"Explain in ENGLISH", "## THE LESSON THE APP TEACHES: en locative-grid", "Case=Ine",
		"typed `házben` where `házban` was expected", "Why -ban and not -ben"} {
		if !strings.Contains(prompt, needle) {
			t.Errorf("prompt lacks %q:\n%s", needle, prompt)
		}
	}
	if !strings.HasSuffix(strings.TrimSpace(prompt), "then a word-by-word breakdown.") {
		t.Errorf("the language line must end the prompt:\n%s", prompt)
	}

	// The thread persisted both turns, question first, titled by the question,
	// and the lesson panel's route finds it.
	var thread TutorThreadResponse
	c.json(http.MethodGet, fmt.Sprintf("/api/tutor/%d?lang=en", id), nil, &thread)
	if !thread.Available || thread.Model != "test-model" {
		t.Errorf("thread availability: %+v", thread)
	}
	if len(thread.Messages) != 2 || thread.Messages[0].Role != "user" || thread.Messages[1].Role != "assistant" {
		t.Fatalf("thread = %+v", thread.Messages)
	}
	if thread.Thread == nil || thread.Thread.Title != "Why -ban and not -ben in házban?" || thread.Thread.TitleEN != "en locative-grid" {
		t.Errorf("thread header = %+v", thread.Thread)
	}
	if thread.Messages[1].TopicTitle != "en locative-grid" {
		t.Errorf("the grounding is named on the stored turn too: %+v", thread.Messages[1])
	}
	var byTopic TutorThreadResponse
	c.json(http.MethodGet, "/api/tutor/topic/locative-grid?lang=en", nil, &byTopic)
	if byTopic.Thread == nil || byTopic.Thread.ID != id || len(byTopic.Messages) != 2 {
		t.Errorf("the lesson panel opens the latest thread pinned to its topic, got %+v", byTopic.Thread)
	}

	// Deleting removes it.
	if resp := c.json(http.MethodDelete, fmt.Sprintf("/api/tutor/%d", id), nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete: %d", resp.StatusCode)
	}
	if resp := c.json(http.MethodGet, fmt.Sprintf("/api/tutor/%d", id), nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("a deleted thread answers %d, want 404", resp.StatusCode)
	}
	c.json(http.MethodGet, "/api/tutor/topic/locative-grid", nil, &byTopic)
	if byTopic.Thread != nil {
		t.Errorf("the panel must not find a deleted thread: %+v", byTopic.Thread)
	}
}

// A free chat (ADR-029): the app finds the lesson per question — from the
// form the learner quoted, from the topic's own words — keeps it for a
// follow-up with no signal, switches when a new subject is named, and says
// so in the prompt when nothing matched. Every answer records its grounding.
func TestTutorInfersTheLessonForAFreeChat(t *testing.T) {
	srv, c := newPackedServer(t)
	fake, lastPrompt := fakeLLM(t, "Because.")
	enableLLM(t, srv, fake.URL)

	id := newThread(t, c, "")

	// 1. A quoted form: the analyser reads Case=Ine on `házban`.
	msg := doneMessage(t, ask(t, c, id, "Why házban and not házben?", "en"))
	if msg.Topic != "locative-grid" || msg.TopicTitle != "en locative-grid" {
		t.Errorf("házban → locative grid, got %q / %q", msg.Topic, msg.TopicTitle)
	}
	if p := lastPrompt(); !strings.Contains(p, "## THE LESSON THE APP TEACHES: en locative-grid") || !strings.Contains(p, "Case=Ine") {
		t.Errorf("the inferred lesson and its features travel:\n%s", p)
	}

	// 2. A follow-up with no signal stays on the same lesson.
	msg = doneMessage(t, ask(t, c, id, "and three more?", "en"))
	if msg.Topic != "locative-grid" {
		t.Errorf("a follow-up keeps the last grounding, got %q", msg.Topic)
	}

	// 3. A new subject named in the topic's own words switches.
	msg = doneMessage(t, ask(t, c, id, "how does the accusative work?", "en"))
	if msg.Topic != "accusative" {
		t.Errorf("'accusative' names its lesson, got %q", msg.Topic)
	}
	if p := lastPrompt(); !strings.Contains(p, "## THE LESSON THE APP TEACHES: en accusative") {
		t.Errorf("the switched lesson travels:\n%s", p)
	}

	// 4. Nothing to go on, on a fresh chat: the prompt says so, the answer
	// records no lesson, and the chat list shows the last grounding.
	fresh := newThread(t, c, "")
	events := ask(t, c, fresh, "hello there", "en")
	msg = doneMessage(t, events)
	if msg.Topic != "" || msg.TopicTitle != "" {
		t.Errorf("no signal must mean no grounding, got %q", msg.Topic)
	}
	// The answer "Because." has no backticked word. The done event must still
	// carry a LIST — `"unattested":null` is what froze the panel mid-answer on
	// every such answer until a reload.
	if raw := events[len(events)-1][1]; !strings.Contains(raw, `"unattested":[]`) {
		t.Errorf("done event must carry an empty list, not null: %s", raw)
	}
	if p := lastPrompt(); !strings.Contains(p, "No lesson is open") || strings.Contains(p, "## FEATURES") {
		t.Errorf("the no-lesson case is stated and no features are sent:\n%s", p)
	}

	var list []TutorSummaryDTO
	c.json(http.MethodGet, "/api/tutor", nil, &list)
	if len(list) != 2 || list[0].ID != fresh || list[1].ID != id {
		t.Fatalf("most recently active first, got %+v", list)
	}
	if list[1].LastTopic != "accusative" || list[1].LastTopicEN != "en accusative" || list[1].Topic != "" {
		t.Errorf("a free chat lists its last grounding, not a pin: %+v", list[1])
	}
	if list[1].Title != "Why házban and not házben?" || list[1].Messages != 6 {
		t.Errorf("titled by the first question, counting every turn: %+v", list[1])
	}
}

func TestTutorAnswersInTheConfiguredLanguageByDefault(t *testing.T) {
	srv, c := newPackedServer(t) // setup chose ru
	fake, lastPrompt := fakeLLM(t, "Ответ.")
	enableLLM(t, srv, fake.URL)

	id := newThread(t, c, "accusative")
	ask(t, c, id, "Почему -t?", "")
	prompt := lastPrompt()
	if !strings.Contains(prompt, "Объясняй ПО-РУССКИ") || !strings.HasSuffix(strings.TrimSpace(prompt), "а затем разбор по словам.") {
		t.Errorf("a blank lang must fall back to locale.ui (ru):\n%s", prompt)
	}
	if !strings.Contains(prompt, "## УРОК ИЗ ПРИЛОЖЕНИЯ: ru accusative") {
		t.Errorf("the pinned lesson, in the answer language:\n%s", prompt)
	}
}

func TestTutorRefusesBadQuestionsAndKeepsThemOffTheThread(t *testing.T) {
	srv, c := newPackedServer(t)
	fake, _ := fakeLLM(t, "x")
	enableLLM(t, srv, fake.URL)

	id := newThread(t, c, "accusative")
	path := fmt.Sprintf("/api/tutor/%d", id)
	if resp := c.json(http.MethodPost, path, TutorAskRequest{Question: "   "}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("blank question: %d", resp.StatusCode)
	}
	if resp := c.json(http.MethodPost, path,
		TutorAskRequest{Question: strings.Repeat("a", 1001)}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("over-long question: %d", resp.StatusCode)
	}
	var thread TutorThreadResponse
	c.json(http.MethodGet, path, nil, &thread)
	if len(thread.Messages) != 0 || thread.Thread.Title != "" {
		t.Errorf("a refused question must not be stored or title the thread: %+v", thread)
	}
}

// A model that does not speak the chat-completions protocol — exactly what
// happens if `services.llm` is pointed at the translation sidecar (`services/mt`,
// which answers only `/translate` and `/health`) — must fail with its OWN key,
// distinct from `errors.llmFailed` (the text-level second opinion's key). The
// two once shared one string, which meant a bad tutor answer silently changed
// what a failed level estimate said too.
func TestTutorFailureHasItsOwnErrorKeyDistinctFromLevelEstimate(t *testing.T) {
	srv, c := newPackedServer(t)
	notAChatAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r) // e.g. services/mt: no /v1/chat/completions route
	}))
	defer notAChatAPI.Close()
	enableLLM(t, srv, notAChatAPI.URL)

	id := newThread(t, c, "locative-grid")
	events := ask(t, c, id, "why?", "")
	last := events[len(events)-1]
	if last[0] != "error" {
		t.Fatalf("want a trailing error event, got %v", events)
	}
	var ev tutorEvent
	if err := json.Unmarshal([]byte(last[1]), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Error != "errors.tutorFailed" {
		t.Errorf("error = %q, want errors.tutorFailed (not errors.llmFailed, which the text-level feature owns)", ev.Error)
	}
	// The question stays, so the learner can retry without retyping.
	var thread TutorThreadResponse
	c.json(http.MethodGet, fmt.Sprintf("/api/tutor/%d", id), nil, &thread)
	if len(thread.Messages) != 1 || thread.Messages[0].Role != "user" {
		t.Errorf("a failed answer keeps the question: %+v", thread.Messages)
	}
}

// The chat list: the learner's threads, most recently active first, titled by
// their first question and naming their pin — never a place a card, gloss or
// state space reads.
func TestTutorListIsTheLearnersHistory(t *testing.T) {
	srv, c := newPackedServer(t)
	var empty []TutorSummaryDTO
	if resp := c.json(http.MethodGet, "/api/tutor", nil, &empty); resp.StatusCode != http.StatusOK {
		t.Fatalf("empty history: %d", resp.StatusCode)
	}
	if len(empty) != 0 {
		t.Fatalf("a fresh account has no tutor history, got %+v", empty)
	}

	fake, _ := fakeLLM(t, "A válasz itt van.")
	enableLLM(t, srv, fake.URL)
	ids := map[string]int64{}
	for _, topic := range []string{"locative-grid", "accusative"} {
		ids[topic] = newThread(t, c, topic)
		ask(t, c, ids[topic], "q about "+topic, "")
	}

	var list []TutorSummaryDTO
	if resp := c.json(http.MethodGet, "/api/tutor", nil, &list); resp.StatusCode != http.StatusOK {
		t.Fatalf("list: %d", resp.StatusCode)
	}
	if len(list) != 2 || list[0].Topic != "accusative" || list[1].Topic != "locative-grid" {
		t.Fatalf("want both threads, most recent first, got %+v", list)
	}
	for _, row := range list {
		if row.ID != ids[row.Topic] || row.Messages != 2 {
			t.Errorf("%s: row = %+v", row.Topic, row)
		}
		if row.LastRole != "assistant" || !strings.Contains(row.LastContent, "válasz") {
			t.Errorf("%s: last turn = %+v", row.Topic, row)
		}
		if row.TitleRU != "ru "+row.Topic || row.Title != "q about "+row.Topic {
			t.Errorf("%s: titles not resolved: %+v", row.Topic, row)
		}
	}
	// Another learner sees none of it, and cannot read or delete it.
	other := &client{t: t, app: srv}
	c.json(http.MethodPost, "/api/users", CreateUserRequest{Email: "b@example.com", Password: goodPassword, Role: "learner"}, nil)
	other.json(http.MethodPost, "/api/login", LoginRequest{Email: "b@example.com", Password: goodPassword}, nil)
	var theirs []TutorSummaryDTO
	other.json(http.MethodGet, "/api/tutor", nil, &theirs)
	if len(theirs) != 0 {
		t.Errorf("threads are per learner, got %+v", theirs)
	}
	if resp := other.json(http.MethodGet, fmt.Sprintf("/api/tutor/%d", ids["accusative"]), nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("another learner's thread answered %d, want 404", resp.StatusCode)
	}
	if resp := other.json(http.MethodDelete, fmt.Sprintf("/api/tutor/%d", ids["accusative"]), nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("deleting another learner's thread answered %d, want 404", resp.StatusCode)
	}
}

// The line the tutor does not cross (ADR-027): a question changes nothing
// the learner is graded against. Counted, not assumed.
func TestTutorCreatesNoCardsAndMovesNoState(t *testing.T) {
	srv, c := newPackedServer(t)
	fake, _ := fakeLLM(t, "Add `házban` to your cards!")
	enableLLM(t, srv, fake.URL)
	count := func(table string) int {
		var n int
		if err := c.app.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}
	before := [5]int{count("note"), count("card"), count("morph_state"), count("review_log"), count("user_gloss")}
	id := newThread(t, c, "")
	ask(t, c, id, "Please add házban as a card", "en")
	after := [5]int{count("note"), count("card"), count("morph_state"), count("review_log"), count("user_gloss")}
	if before != after {
		t.Errorf("the tutor touched graded state: before %v, after %v", before, after)
	}
}

// A stream written by hand: the exact chunks a server sends, plus /api/ps
// with a window, so the handler's reading of finish_reason, usage and the
// window can be pinned. `provider local`, because the window is only ever
// asked of a local server.
func fakeStreamer(t *testing.T, srv *Server, body string, window int) {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(body))
		case "/api/ps":
			_, _ = fmt.Fprintf(w, `{"models":[{"name":"test-model","model":"test-model","context_length":%d}]}`, window)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fake.Close)
	if _, err := srv.updateConfig(func(cfg *config.Config) {
		cfg.Services.LLM = config.Service{Provider: config.ProviderLocal, Endpoint: fake.URL, Model: "test-model"}
	}); err != nil {
		t.Fatal(err)
	}
}

// An answer that ran out of budget is an answer: `done`, marked interrupted,
// with the server's figures and the window on it (ADR-031). It was once sent
// as `error` with «ничего не изменилось» under a stored partial — false twice.
func TestTutorKeepsABudgetCutAnswerWithItsFigures(t *testing.T) {
	srv, c := newPackedServer(t)
	fakeStreamer(t, srv, `data: {"choices":[{"delta":{"content":"Use "},"finish_reason":null}]}`+"\n\n"+
		`data: {"choices":[{"delta":{"content":"`+"`házban`"+`"},"finish_reason":"length"}]}`+"\n\n"+
		`data: {"choices":[],"usage":{"prompt_tokens":3210,"completion_tokens":640,"total_tokens":3850}}`+"\n\n"+
		"data: [DONE]\n\n", 8192)
	id := newThread(t, c, "locative-grid")
	msg := doneMessage(t, ask(t, c, id, "Why?", "en"))
	if !msg.Interrupted || msg.CutReason != tutor.CutBudget || msg.Content != "Use `házban`" {
		t.Errorf("a budget cut is done + interrupted, with its reason, got %+v", msg)
	}
	if msg.PromptTokens != 3210 || msg.AnswerTokens != 640 || msg.ContextTokens != 8192 {
		t.Errorf("the figures travel with the answer: %d/%d/%d", msg.PromptTokens, msg.AnswerTokens, msg.ContextTokens)
	}
	var thread TutorThreadResponse
	c.json(http.MethodGet, fmt.Sprintf("/api/tutor/%d", id), nil, &thread)
	if last := thread.Messages[len(thread.Messages)-1]; last.ContextTokens != 8192 || last.PromptTokens != 3210 || !last.Interrupted {
		t.Errorf("stored with the figures: %+v", last)
	}
}

// Stop ends the answer where it is: the partial text is stored and delivered
// as `done`, marked interrupted; the flight is released so the next question
// goes through; stopping when nothing is in flight is a no-op.
func TestTutorStopKeepsThePartialAnswer(t *testing.T) {
	srv, c := newPackedServer(t)
	started := make(chan struct{}, 1)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"Elj"},"finish_reason":null}]}` + "\n\n"))
		w.(http.Flusher).Flush()
		started <- struct{}{}
		<-r.Context().Done() // the stop cancels the request; the model never finishes
	}))
	t.Cleanup(fake.Close)
	enableLLM(t, srv, fake.URL)
	id := newThread(t, c, "locative-grid")

	type result struct {
		events [][2]string
	}
	got := make(chan result, 1)
	go func() {
		resp, body := c.do(http.MethodPost, fmt.Sprintf("/api/tutor/%d", id), TutorAskRequest{Question: "Say it", Lang: "en"})
		if resp.StatusCode != http.StatusOK {
			t.Errorf("ask: %d %s", resp.StatusCode, body)
		}
		got <- result{parseSSE(string(body))}
	}()
	<-started
	if resp := c.json(http.MethodPost, fmt.Sprintf("/api/tutor/%d/stop", id), nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("stop: %d", resp.StatusCode)
	}
	msg := doneMessage(t, (<-got).events)
	if msg.Content != "Elj" || !msg.Interrupted || msg.CutReason != tutor.CutStop {
		t.Errorf("the partial answer is kept and marked as stopped, got %+v", msg)
	}
	// Idempotent, and the flight is free again.
	if resp := c.json(http.MethodPost, fmt.Sprintf("/api/tutor/%d/stop", id), nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("a second stop: %d, want 204", resp.StatusCode)
	}
	if resp := c.json(http.MethodPost, "/api/tutor/999/stop", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("stopping on another learner's thread: %d, want 404", resp.StatusCode)
	}
	var thread TutorThreadResponse
	c.json(http.MethodGet, fmt.Sprintf("/api/tutor/%d", id), nil, &thread)
	if len(thread.Messages) != 2 || !thread.Messages[1].Interrupted || thread.Messages[1].CutReason != tutor.CutStop {
		t.Errorf("question + partial answer stored, with the reason: %+v", thread.Messages)
	}
}

// Editing a question deletes it and everything after it, re-titles the
// thread when it was the first, and asks again with the history that is
// left — never a branch (ADR-031).
func TestTutorEditReplacesTheQuestionAndWhatFollowed(t *testing.T) {
	srv, c := newPackedServer(t)
	fake, lastPrompt := fakeLLM(t, "Answer.")
	enableLLM(t, srv, fake.URL)
	id := newThread(t, c, "")
	ask(t, c, id, "Why házban?", "en")
	ask(t, c, id, "And kertben?", "en")
	var thread TutorThreadResponse
	c.json(http.MethodGet, fmt.Sprintf("/api/tutor/%d", id), nil, &thread)
	if len(thread.Messages) != 4 || thread.Thread.Title != "Why házban?" {
		t.Fatalf("two exchanges first: %d messages, title %q", len(thread.Messages), thread.Thread.Title)
	}
	first, answer := thread.Messages[0].ID, thread.Messages[1].ID

	// Not a question of the learner's own: refused, nothing deleted.
	var body map[string]string
	if resp := c.json(http.MethodPost, fmt.Sprintf("/api/tutor/%d", id),
		TutorAskRequest{Question: "x", Replace: answer}, &body); resp.StatusCode != http.StatusNotFound || body["error"] != "errors.tutorNoMessage" {
		t.Errorf("replacing an answer: %d %v, want 404 errors.tutorNoMessage", resp.StatusCode, body)
	}
	if resp := c.json(http.MethodPost, fmt.Sprintf("/api/tutor/%d", id),
		TutorAskRequest{Question: "x", Replace: 999999}, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("replacing a message that is not there: %d, want 404", resp.StatusCode)
	}
	c.json(http.MethodGet, fmt.Sprintf("/api/tutor/%d", id), nil, &thread)
	if len(thread.Messages) != 4 {
		t.Fatalf("a refused edit deletes nothing, got %d messages", len(thread.Messages))
	}

	// The first question, edited: the thread starts over from it.
	_, resp := c.do(http.MethodPost, fmt.Sprintf("/api/tutor/%d", id),
		TutorAskRequest{Question: "Why házban and not házben?", Lang: "en", Replace: first})
	_ = resp
	c.json(http.MethodGet, fmt.Sprintf("/api/tutor/%d", id), nil, &thread)
	if len(thread.Messages) != 2 || thread.Messages[0].Content != "Why házban and not házben?" {
		t.Errorf("the edited question and its answer alone remain: %+v", thread.Messages)
	}
	if thread.Thread.Title != "Why házban and not házben?" {
		t.Errorf("the thread is re-titled by the new first question, got %q", thread.Thread.Title)
	}
	if p := lastPrompt(); strings.Contains(p, "And kertben?") || strings.Contains(p, "[user]\nWhy házban?\n") {
		t.Errorf("the model must not see the turns that were removed:\n%s", p)
	}
}

// cardOf finds the card of one direction on a note. Straight from the table:
// the served queue buries siblings, so a fresh note shows only one of its
// cards a day, and the tests need a particular one.
func cardOf(t *testing.T, srv *Server, noteID int64, direction string) int64 {
	t.Helper()
	var id int64
	if err := srv.db.DB.QueryRow(`SELECT id FROM card WHERE note_id = ? AND direction = ?`, noteID, direction).Scan(&id); err != nil {
		t.Fatalf("no %s card on note %d: %v", direction, noteID, err)
	}
	return id
}

// A prepared question from the review card (ADR-034): the server composes
// the question from the card and the pack, puts the CARD block in the prompt,
// grounds the lesson from the card's own features, and stores the question
// with the card it was about.
func TestTutorCardQuestionIsComposedGroundedAndStored(t *testing.T) {
	srv, c := newPackedServer(t)
	fake, lastPrompt := fakeLLM(t, "Because `házban`.")
	enableLLM(t, srv, fake.URL)
	created := noteOn(t, c, map[string]any{"hu": "A házban van egy nagy könyv.", "targetForm": "házban"})
	cloze := cardOf(t, srv, created.NoteID, "cloze")

	id := newThread(t, c, "")
	resp, body := c.do(http.MethodPost, fmt.Sprintf("/api/tutor/%d", id), TutorAskRequest{
		Lang: "en", Card: &TutorCardRequest{CardID: cloze, Phase: "answer", Template: "whyNotMine", Typed: "házben"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ask with a card: %d %s", resp.StatusCode, body)
	}
	events := parseSSE(string(body))
	var asked tutorEvent
	if err := json.Unmarshal([]byte(events[0][1]), &asked); err != nil || asked.Question == "" {
		t.Fatalf("the asked event carries the composed question: %s", events[0][1])
	}
	if !strings.Contains(asked.Question, "`házben`") || !strings.Contains(asked.Question, "`házban`") {
		t.Errorf("composed question = %q", asked.Question)
	}
	msg := doneMessage(t, events)
	if msg.Topic != "locative-grid" {
		t.Errorf("the lesson is found from the card's features, got %q", msg.Topic)
	}
	prompt := lastPrompt()
	for _, needle := range []string{"## CARD", "`A házban van egy nagy könyv.`", "the learner typed: `házben`", "expected: `házban`",
		"Case=Ine", "Answer shape:", "No check question"} {
		if !strings.Contains(prompt, needle) {
			t.Errorf("prompt lacks %q:\n%s", needle, prompt)
		}
	}
	var thread TutorThreadResponse
	c.json(http.MethodGet, fmt.Sprintf("/api/tutor/%d", id), nil, &thread)
	if q := thread.Messages[0]; q.CardID != cloze || q.CardPhase != "answer" || q.Template != "whyNotMine" || q.Content != asked.Question {
		t.Errorf("the stored question names its card: %+v", q)
	}
	if thread.Thread.Title != tutor.TitleFrom(asked.Question) {
		t.Errorf("an ordinary chat is still titled by its first question, got %q", thread.Thread.Title)
	}
}

// The question-phase hint: nothing that is the answer reaches the prompt.
func TestTutorHintWordCarriesNoWord(t *testing.T) {
	srv, c := newPackedServer(t)
	fake, lastPrompt := fakeLLM(t, "A noun, back harmony.")
	enableLLM(t, srv, fake.URL)
	created := noteOn(t, c, map[string]any{"hu": "A házban van egy nagy könyv.", "targetForm": "házban"})
	produce := cardOf(t, srv, created.NoteID, "produce")
	id := newThread(t, c, "")
	resp, body := c.do(http.MethodPost, fmt.Sprintf("/api/tutor/%d", id), TutorAskRequest{Lang: "en",
		Card: &TutorCardRequest{CardID: produce, Phase: "question", Template: "hintWord"}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("hintWord: %d %s", resp.StatusCode, body)
	}
	doneMessage(t, parseSSE(string(body)))
	prompt := lastPrompt()
	block := prompt[strings.Index(prompt, "## CARD"):]
	if i := strings.Index(block[3:], "\n## "); i >= 0 {
		block = block[:i+3]
	}
	// The test pack glosses `ház` as «ház (ru)», so the gloss line is allowed
	// to carry the letters; the form, the dictionary line and the sentence
	// that holds it are not.
	if strings.Contains(block, "házban") || strings.Contains(block, "- dictionary form:") || strings.Contains(block, "- sentence:") {
		t.Errorf("the produce card's word must not reach the model before the answer:\n%s", block)
	}
	if !strings.Contains(block, "WITHHELD") {
		t.Errorf("the block says the answer is withheld:\n%s", block)
	}
}

// Another learner's card, a card that is not there, a bad phase: refused
// before anything is stored, with the tutor's own key.
func TestTutorRefusesAnotherLearnersCard(t *testing.T) {
	srv, c := newPackedServer(t)
	fake, _ := fakeLLM(t, "x")
	enableLLM(t, srv, fake.URL)
	id := newThread(t, c, "")
	var body map[string]string
	if resp := c.json(http.MethodPost, fmt.Sprintf("/api/tutor/%d", id), TutorAskRequest{Lang: "en",
		Card: &TutorCardRequest{CardID: 999999, Phase: "answer", Template: "breakdown"}}, &body); resp.StatusCode != http.StatusNotFound || body["error"] != "errors.tutorNoCard" {
		t.Errorf("a card that is not the learner's: %d %v, want 404 errors.tutorNoCard", resp.StatusCode, body)
	}
	if resp := c.json(http.MethodPost, fmt.Sprintf("/api/tutor/%d", id), TutorAskRequest{Lang: "en", Question: "q",
		Card: &TutorCardRequest{CardID: 1, Phase: "later"}}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("a bad phase: %d, want 400", resp.StatusCode)
	}
	var thread TutorThreadResponse
	c.json(http.MethodGet, fmt.Sprintf("/api/tutor/%d", id), nil, &thread)
	if len(thread.Messages) != 0 {
		t.Errorf("a refused question stores nothing, got %d messages", len(thread.Messages))
	}
}

// A review chat is one per WORD per local day (ADR-034, amended): found when
// it exists, created when it does not, named by the word rather than by a
// question, and a different word opens a different chat — the panel that
// showed the previous word's conversation is the whole reason for the note id.
func TestTutorReviewThreadIsOnePerWord(t *testing.T) {
	srv, c := newPackedServer(t)
	fake, _ := fakeLLM(t, "Answer.")
	enableLLM(t, srv, fake.URL)
	house := noteOn(t, c, map[string]any{"hu": "A házban van egy nagy könyv.", "targetForm": "házban"})
	glad := noteOn(t, c, map[string]any{"hu": "Örülök, hogy látlak.", "targetForm": "Örülök"})

	var today TutorThreadResponse
	url := fmt.Sprintf("/api/tutor/review?note=%d", house.NoteID)
	if resp := c.json(http.MethodGet, url, nil, &today); resp.StatusCode != http.StatusOK || today.Thread != nil {
		t.Fatalf("before any question there is no chat for the word: %d %+v", resp.StatusCode, today.Thread)
	}
	var first, second TutorThreadResponse
	req := TutorCreateRequest{Kind: "review", Note: house.NoteID}
	if resp := c.json(http.MethodPost, "/api/tutor", req, &first); resp.StatusCode != http.StatusCreated {
		t.Fatalf("first chat about the word: %d", resp.StatusCode)
	}
	if resp := c.json(http.MethodPost, "/api/tutor", req, &second); resp.StatusCode != http.StatusOK {
		t.Fatalf("second chat about the word: %d, want 200 (found)", resp.StatusCode)
	}
	if first.Thread == nil || second.Thread == nil || first.Thread.ID != second.Thread.ID || first.Thread.Kind != "review" {
		t.Errorf("one chat per word per day: %+v vs %+v", first.Thread, second.Thread)
	}
	// The word names it, so the chat list can tell one from another.
	if first.Thread.Title != "házban" || first.Thread.NoteID != house.NoteID {
		t.Errorf("a review chat is named by its word: %+v", first.Thread)
	}

	// The next word is a different conversation, and the first word's is not
	// what its panel opens on.
	var other TutorThreadResponse
	if resp := c.json(http.MethodGet, fmt.Sprintf("/api/tutor/review?note=%d", glad.NoteID), nil, &other); resp.StatusCode != http.StatusOK {
		t.Fatalf("the next word's chat: %d", resp.StatusCode)
	}
	if other.Thread != nil {
		t.Fatalf("a new word opens an empty chat, not the previous word's: %+v", other.Thread)
	}
	c.json(http.MethodPost, "/api/tutor", TutorCreateRequest{Kind: "review", Note: glad.NoteID}, &other)
	if other.Thread == nil || other.Thread.ID == first.Thread.ID || other.Thread.Title != "Örülök" {
		t.Errorf("the next word gets its own chat: %+v", other.Thread)
	}

	ask(t, c, first.Thread.ID, "Why házban?", "en")
	c.json(http.MethodGet, url, nil, &today)
	if today.Thread == nil || today.Thread.ID != first.Thread.ID || len(today.Messages) != 2 {
		t.Errorf("the word's chat is found with its messages: %+v (%d messages)", today.Thread, len(today.Messages))
	}
	var thread TutorThreadResponse
	c.json(http.MethodGet, fmt.Sprintf("/api/tutor/%d", first.Thread.ID), nil, &thread)
	if thread.Thread.Title != "házban" {
		t.Errorf("a question never renames a review chat; got %q", thread.Thread.Title)
	}
	var list []TutorSummaryDTO
	c.json(http.MethodGet, "/api/tutor", nil, &list)
	if len(list) != 2 || list[0].Kind != "review" {
		t.Errorf("the list carries the kind: %+v", list)
	}
	if resp := c.json(http.MethodPost, "/api/tutor", TutorCreateRequest{Kind: "bogus"}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("an unknown kind: %d, want 400", resp.StatusCode)
	}
	if resp := c.json(http.MethodPost, "/api/tutor", TutorCreateRequest{Kind: "review", Note: 987654}, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("a chat about a note that is not the learner's: %d, want 404", resp.StatusCode)
	}
}

// The question limit is characters, as the composer counts them — a
// thousand Cyrillic letters are two thousand bytes and were once refused.
func TestTutorAcceptsAThousandCyrillicCharacters(t *testing.T) {
	srv, c := newPackedServer(t)
	fake, _ := fakeLLM(t, "Да.")
	enableLLM(t, srv, fake.URL)
	id := newThread(t, c, "")
	if resp, body := c.do(http.MethodPost, fmt.Sprintf("/api/tutor/%d", id), TutorAskRequest{Question: strings.Repeat("я", 1000), Lang: "ru"}); resp.StatusCode != http.StatusOK {
		t.Errorf("1000 characters: %d %s", resp.StatusCode, body)
	}
	if resp := c.json(http.MethodPost, fmt.Sprintf("/api/tutor/%d", id), TutorAskRequest{Question: strings.Repeat("я", 1001), Lang: "ru"}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("1001 characters: %d, want 400", resp.StatusCode)
	}
}
