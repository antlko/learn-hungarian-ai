package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/llm"
	"tanulom/internal/tutor"
)

// A dictionary entry for a word the language pack does not have.
//
// The pack is finite and the reader is not. A learner who clicks
// `bejelentkezve` in a pasted text used to get «В языковом пакете этого нет.»
// and nothing else — no meaning, no breakdown, no way on, on exactly the word
// they stopped for.
//
// This is NOT the tutor, and it is deliberately a separate handler. The tutor
// mints nothing — `handlers_tutor_test.go` counts `note`, `card`, `user_gloss`
// and both state spaces before and after a question, and that test must keep
// passing. This path CAN reach `user_gloss`, but only the way a machine
// translation does: `source='machine'` with the model recorded, on an explicit
// save, and a card's back only after «Принять как своё» re-saves it as
// `manual` (ADR-014, ADR-035). Keeping the two apart in code is what keeps
// that distinction true rather than merely intended.
//
// Prompt: `.claude/prompts/word-entry.md`, version 1. ADR-042.

// aiEntryPromptVersion travels with nothing yet — the entry is not stored —
// but it is recorded in the log line, so a bad entry can be traced to the
// template that produced it.
const aiEntryPromptVersion = 2

// aiEntryTimeout bounds one entry. Shorter than the tutor's stream budget: an
// entry is a few dozen tokens of JSON, and a learner waiting on a word popover
// has a much lower tolerance than one who asked a question.
const aiEntryTimeout = 45 * time.Second

// aiEntryMaxTokens is generous for the schema and mean enough that a model
// which starts writing an essay is cut rather than indulged.
const aiEntryMaxTokens = 400

// AIExample is one illustration sentence. Never material: it is not attested
// (`internal/corpus` does not exist), it never becomes a note, and the panel
// says so.
type AIExample struct {
	HU string `json:"hu"`
	TR string `json:"tr"`
}

// AIEntryResponse is a model's dictionary entry, labelled.
type AIEntryResponse struct {
	// Form is the word as asked for, normalised the way `user_gloss` keys it.
	Form string `json:"form"`
	// Model is which model wrote this. It travels with the entry and is
	// printed beside it, never in a tooltip — ADR-014's labelling rule.
	Model string `json:"model"`

	// Unknown is the model's own admission. An honest dead end beats a
	// confident invention, so it is passed straight through.
	Unknown bool `json:"unknown,omitempty"`

	Lemma     string      `json:"lemma,omitempty"`
	POS       string      `json:"pos,omitempty"`
	Meanings  []string    `json:"meanings,omitempty"`
	Morphemes []string    `json:"morphemes,omitempty"`
	Examples  []AIExample `json:"examples,omitempty"`

	// Raw is the model's text when it could not be read as the schema. Shown
	// as prose rather than discarded: a small model that writes a correct
	// answer in the wrong shape has still helped, and hiding it would be the
	// app deciding the learner may not see what it received.
	Raw string `json:"raw,omitempty"`

	// Unattested lists the Hungarian in this entry that the pack's dictionary
	// has never seen. The same check the tutor's answers get, and the one
	// thing a weak model gets wrong most often.
	Unattested []string `json:"unattested"`
}

func (s *Server) handleAIEntry(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	form, err := glossKey(c.Params("form"))
	if err != nil {
		return err
	}
	if err := s.requirePack(); err != nil {
		return err
	}

	cfg := s.Config()
	client, err := llm.New(cfg.Services.LLM, s.llmKey())
	if err != nil {
		// The same key the tutor uses, so one failure has one sentence
		// wherever it appears. lib/errors.ts turns it into a language.
		return fiber.NewError(fiber.StatusServiceUnavailable, llmUnavailableKey(err))
	}

	lang := s.answerLang(c.Query("lang"))
	system := aiEntrySystem(lang)

	ctx, cancel := context.WithTimeout(c.Context(), aiEntryTimeout)
	defer cancel()

	text, err := client.Chat(ctx, system, form, aiEntryMaxTokens)
	if err != nil {
		slog.WarnContext(c.Context(), "ai entry failed",
			"user", u.ID, "form", form, "model", client.Model(), "error", err)
		return fiber.NewError(fiber.StatusBadGateway, "errors.llmFailed")
	}

	out := parseAIEntry(text)
	out.Form = form
	out.Model = client.Model()
	// Form is set BEFORE the attestation pass, which reads it to exclude the
	// asked-about word from the warning.

	// The guardrail. Every Hungarian token in the entry is looked up, and the
	// misses are listed — including tokens the model forgot to backtick,
	// because a model that drops the backticks would otherwise silently turn
	// the check off. See the prompt's failure-mode table.
	exists := s.wordExists()
	out.Unattested = tutor.Unattested(aiEntryHungarian(out, text), exists)
	if out.Unattested == nil {
		out.Unattested = []string{}
	}

	slog.InfoContext(c.Context(), "ai entry produced",
		"user", u.ID, "form", form, "model", out.Model,
		"prompt_version", aiEntryPromptVersion,
		"parsed", out.Raw == "", "unknown", out.Unknown,
		"unattested", len(out.Unattested))

	return c.JSON(out)
}

// aiEntrySystem is the prompt, filled for one language.
//
// Kept in step with `.claude/prompts/word-entry.md` by hand, as the tutor's is
// with its own files; the template there carries the rationale and the failure
// modes, which do not belong in a Go string.
func aiEntrySystem(lang string) string {
	langName := "Russian"
	if lang != "ru" {
		langName = "English"
	}
	return `You are a Hungarian-to-` + langName + ` dictionary. Answer ONLY with a JSON object.

Schema:
{
  "lemma":    "the dictionary form of the word",
  "pos":      "one of: NOUN VERB ADJ ADV NUM PRON PROPN DET CONJ ADP PART INTJ",
  "meanings": ["short gloss", "another sense"],
  "morphemes":["stem", "-suffix"],
  "examples": [{"hu": "a short Hungarian sentence using the word",
                "tr": "its translation into ` + langName + `"}]
}

Rules:
- Wrap every Hungarian word you write in backticks, INSIDE the quotes: "lemma": "` + "`ház`" + `".
  Never put a backtick around a quoted value — that is not JSON and it cannot be read.
- At most 3 meanings, at most 3 morphemes, at most 2 examples.
- Meanings are in ` + langName + `, short, no sentences, no grammar labels.
- "morphemes" splits the FORM you were given, not the lemma: stem first, then each suffix
  written with its leading hyphen.
- If you do not know the word, return {"unknown": true} and nothing else.
- No prose, no markdown fences, no commentary. JSON only.`
}

// aiEntryFields is the schema as the model is asked to write it.
type aiEntryFields struct {
	Unknown   bool        `json:"unknown"`
	Lemma     string      `json:"lemma"`
	POS       string      `json:"pos"`
	Meanings  []string    `json:"meanings"`
	Morphemes []string    `json:"morphemes"`
	Examples  []AIExample `json:"examples"`
}

// parseAIEntry reads the model's answer, leniently.
//
// Small models fence their JSON, preface it, or append a remark. None of that
// is worth failing over, and none of it is worth guessing about either: what
// cannot be read as the schema comes back as `raw` and is shown as prose.
func parseAIEntry(text string) AIEntryResponse {
	shown := stripFence(strings.TrimSpace(text))

	// Take the outermost object, so a preface or a trailing remark is dropped
	// rather than taking the whole answer down with it.
	body := shown
	if i, j := strings.IndexByte(body, '{'), strings.LastIndexByte(body, '}'); i >= 0 && j > i {
		body = body[i : j+1]
	}

	raw, ok := decodeAIEntry(body)
	if !ok {
		// A backtick is not JSON syntax, and the prompt asks for backticks.
		// A model that wraps the whole VALUE instead of the word inside it —
		// "lemma": `"feltehetek"` — writes something no parser will take, and
		// the learner got the raw JSON on the panel, fence and all. The ticks
		// are decoration stripped from every field below anyway, so a second
		// pass without them loses nothing and recovers the entry.
		raw, ok = decodeAIEntry(strings.ReplaceAll(body, "`", ""))
	}
	if !ok {
		// Fence-stripped: shown as prose, so the ``` belongs to nobody.
		return AIEntryResponse{Raw: shown}
	}
	if raw.Unknown {
		return AIEntryResponse{Unknown: true}
	}
	out := AIEntryResponse{
		Lemma:     strings.TrimSpace(stripTicks(raw.Lemma)),
		POS:       strings.ToUpper(strings.TrimSpace(raw.POS)),
		Meanings:  cleanList(raw.Meanings, 3),
		Morphemes: cleanList(raw.Morphemes, 3),
	}
	for _, ex := range raw.Examples {
		hu := strings.TrimSpace(stripTicks(ex.HU))
		if hu == "" {
			continue
		}
		out.Examples = append(out.Examples, AIExample{HU: hu, TR: strings.TrimSpace(stripTicks(ex.TR))})
		if len(out.Examples) == 2 {
			break
		}
	}
	// Nothing usable came back in the shape, so keep the words themselves.
	if out.Lemma == "" && len(out.Meanings) == 0 && len(out.Examples) == 0 {
		return AIEntryResponse{Raw: shown}
	}
	return out
}

// decodeAIEntry reads the schema, reporting whether it could.
func decodeAIEntry(body string) (aiEntryFields, bool) {
	var raw aiEntryFields
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return aiEntryFields{}, false
	}
	return raw, true
}

// stripFence removes a leading ```json (or bare ```) fence and its closer.
//
// Applied before anything else reads the answer, including the prose fallback:
// a learner who is shown the model's text as prose should not be shown the
// fence it arrived in.
func stripFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if nl := strings.IndexByte(s, '\n'); nl >= 0 {
		s = s[nl+1:]
	}
	if end := strings.LastIndex(s, "```"); end >= 0 {
		s = s[:end]
	}
	return strings.TrimSpace(s)
}

// stripTicks removes the backticks the prompt asks for. They are how the
// attestation check finds the Hungarian; the learner should not read them.
func stripTicks(s string) string { return strings.ReplaceAll(s, "`", "") }

func cleanList(in []string, max int) []string {
	var out []string
	for _, v := range in {
		v = strings.TrimSpace(stripTicks(v))
		if v == "" {
			continue
		}
		out = append(out, v)
		if len(out) == max {
			break
		}
	}
	return out
}

// aiEntryHungarian collects every token worth looking up.
//
// The backticked spans first, which is what the prompt asks for and what the
// tutor relies on. Then the lemma and the example sentences REGARDLESS of
// whether they were backticked — because a model that drops the backticks
// would otherwise turn the guardrail off silently, which is the one failure
// mode a learner could not possibly notice.
func aiEntryHungarian(out AIEntryResponse, rawAnswer string) []string {
	seen := map[string]bool{}
	// The form the learner asked about is never listed. They are looking at
	// this panel BECAUSE the pack does not have it — repeating that back as a
	// warning about the model's honesty is noise, and noise in a warning is
	// how a warning stops being read.
	if out.Form != "" {
		seen[strings.ToLower(out.Form)] = true
	}
	var words []string
	add := func(w string) {
		w = strings.Trim(strings.TrimSpace(w), ".,!?;:—–\"'()«»")
		if w == "" || seen[strings.ToLower(w)] {
			return
		}
		seen[strings.ToLower(w)] = true
		words = append(words, w)
	}
	for _, w := range tutor.Backticked(rawAnswer) {
		add(w)
	}
	add(out.Lemma)
	for _, ex := range out.Examples {
		for _, w := range strings.Fields(ex.HU) {
			add(w)
		}
	}
	return words
}

// aiEntryStatus is what the inspector needs before it offers the button: is
// there a model at all, and which one. Without it the button would be offered
// and then fail, which is the pattern ADR-001 exists to avoid.
type aiEntryStatus struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	Model     string `json:"model,omitempty"`
}

func (s *Server) handleAIEntryStatus(c fiber.Ctx) error {
	if _, err := mustUser(c); err != nil {
		return err
	}
	client, err := llm.New(s.Config().Services.LLM, s.llmKey())
	if err != nil {
		return c.JSON(aiEntryStatus{Reason: llmUnavailableKey(err)})
	}
	return c.JSON(aiEntryStatus{Available: true, Model: client.Model()})
}
