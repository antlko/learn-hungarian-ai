package api

import (
	"net/http"
	"strings"
	"testing"
)

// The AI dictionary entry (ADR-042).
//
// Two things must hold, and neither is obvious from reading the handler:
// a small model's sloppy JSON still produces something useful, and the
// attestation guardrail cannot be switched off by the model forgetting to
// backtick its Hungarian.

func TestAIEntryReadsSloppyJSONFromSmallModels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, answer string
		wantLemma    string
		wantMeanings int
		wantRaw      bool
	}{
		{
			name:         "clean",
			answer:       `{"lemma":"` + "`bejelentkezik`" + `","pos":"VERB","meanings":["войти","зарегистрироваться"],"morphemes":["` + "`bejelentkez`" + `","-ve"],"examples":[{"hu":"` + "`Ön nincs bejelentkezve.`" + `","tr":"Вы не вошли."}]}`,
			wantLemma:    "bejelentkezik",
			wantMeanings: 2,
		},
		{
			name:         "fenced",
			answer:       "```json\n{\"lemma\":\"ház\",\"pos\":\"NOUN\",\"meanings\":[\"дом\"]}\n```",
			wantLemma:    "ház",
			wantMeanings: 1,
		},
		{
			name:         "prefaced and followed by chatter",
			answer:       "Sure! Here is the entry:\n{\"lemma\":\"kert\",\"meanings\":[\"сад\"]}\nHope that helps!",
			wantLemma:    "kert",
			wantMeanings: 1,
		},
		{
			// The rule "wrap every Hungarian word in backticks" read as
			// "wrap the value", and gemma3:4b wrote `"feltehetek"` where the
			// JSON wanted a string. No parser takes that, and the learner got
			// the raw fenced JSON on the panel. The ticks are stripped from
			// every field anyway, so they are stripped from the body too.
			name: "backticks around the values",
			answer: "```json\n{\n \"lemma\": `\"feltehetek\"`,\n \"pos\": \"VERB\",\n" +
				" \"meanings\": [\"мочь предположить\", \"можно предполагать\"],\n" +
				" \"morphemes\": [\"fel-\", \"tehe-\"],\n" +
				" \"examples\": [{\"hu\": `\"Feltehetek egy kérdést?\"`, \"tr\": \"Можно задать вопрос?\"}]\n}\n```",
			wantLemma:    "feltehetek",
			wantMeanings: 2,
		},
		{
			// Not JSON at all. The words are kept and shown as prose rather
			// than thrown away: a correct answer in the wrong shape has still
			// helped the learner.
			name:    "prose",
			answer:  "bejelentkezve means «logged in», from bejelentkezik.",
			wantRaw: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseAIEntry(tc.answer)
			if tc.wantRaw {
				if got.Raw == "" {
					t.Fatalf("unparseable answer must come back as raw, got %+v", got)
				}
				// Prose is shown as prose: the fence is the wire format, not
				// something the learner has any reason to read.
				if strings.Contains(got.Raw, "```") {
					t.Errorf("raw kept its fence: %q", got.Raw)
				}
				return
			}
			if got.Raw != "" {
				t.Errorf("parseable answer fell back to raw: %q", got.Raw)
			}
			if got.Lemma != tc.wantLemma {
				t.Errorf("lemma = %q, want %q", got.Lemma, tc.wantLemma)
			}
			if len(got.Meanings) != tc.wantMeanings {
				t.Errorf("meanings = %v, want %d of them", got.Meanings, tc.wantMeanings)
			}
		})
	}
}

// The backticks are how the guardrail finds the Hungarian. A model that drops
// them must not thereby turn the check off — that is the one failure a learner
// could not possibly notice.
func TestAIEntryChecksHungarianEvenWithoutBackticks(t *testing.T) {
	t.Parallel()
	// No backticks anywhere: the prompt asks for them, the model ignored it.
	answer := `{"lemma":"xyzzyk","pos":"NOUN","meanings":["ерунда"],
	            "examples":[{"hu":"A xyzzyk nagy.","tr":"Ерунда большая."}]}`
	out := parseAIEntry(answer)
	out.Form = "xyzzyk" // the asked-about form is excluded on purpose
	words := aiEntryHungarian(out, answer)

	has := func(w string) bool {
		for _, v := range words {
			if v == w {
				return true
			}
		}
		return false
	}
	// The asked-about form is deliberately NOT listed — the learner already
	// knows the pack lacks it, and repeating that is noise in a warning.
	if has("xyzzyk") {
		t.Errorf("the asked-about form must not be listed as unattested; got %v", words)
	}
	if !has("nagy") {
		t.Errorf("an example's words must be checked even unbackticked; got %v", words)
	}
	// Punctuation is trimmed, so the lookup gets a word and not a token.
	for _, w := range words {
		if w == "nagy." {
			t.Error("trailing punctuation reached the dictionary lookup")
		}
	}
}

// The model saying "I do not know" is an honest dead end and is passed
// straight through rather than dressed up.
func TestAIEntryHonoursTheModelsOwnUnknown(t *testing.T) {
	t.Parallel()
	got := parseAIEntry(`{"unknown": true}`)
	if !got.Unknown {
		t.Fatalf("want Unknown, got %+v", got)
	}
	if got.Lemma != "" || len(got.Meanings) != 0 || got.Raw != "" {
		t.Errorf("an unknown entry must carry nothing else: %+v", got)
	}
}

// The caps in the prompt are advice; the parser enforces them, so a model that
// writes twelve meanings cannot fill the panel.
func TestAIEntryCapsWhatItAccepts(t *testing.T) {
	t.Parallel()
	got := parseAIEntry(`{"lemma":"ház","meanings":["a","b","c","d","e"],
	                      "morphemes":["h","á","z","x"],
	                      "examples":[{"hu":"Egy."},{"hu":"Kettő."},{"hu":"Három."}]}`)
	if len(got.Meanings) != 3 {
		t.Errorf("meanings = %d, want at most 3", len(got.Meanings))
	}
	if len(got.Morphemes) != 3 {
		t.Errorf("morphemes = %d, want at most 3", len(got.Morphemes))
	}
	if len(got.Examples) != 2 {
		t.Errorf("examples = %d, want at most 2", len(got.Examples))
	}
}

// With no model configured the inspector must not offer the button at all:
// offering it and then failing is the pattern ADR-001 exists to avoid.
func TestAIEntryStatusSaysWhyItIsUnavailable(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)

	var st aiEntryStatus
	if resp := c.json(http.MethodGet, "/api/words/ai-entry/status", nil, &st); resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if st.Available {
		t.Error("no model is configured, so the entry must not be offered")
	}
	if st.Reason != "errors.llmOff" {
		t.Errorf("reason = %q, want errors.llmOff", st.Reason)
	}

	// And asking anyway is refused with the same key, not with a blank 500.
	var body map[string]string
	if resp := c.json(http.MethodPost, "/api/words/valami/entry", nil, &body); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("asking with no model = %d, want 503", resp.StatusCode)
	} else if body["error"] != "errors.llmOff" {
		t.Errorf("error = %q, want errors.llmOff", body["error"])
	}
}

// The entry path may reach `user_gloss` — but only on an explicit save, and
// never by itself. Asking for an entry must create nothing.
func TestAskingForAnEntryStoresNothing(t *testing.T) {
	t.Parallel()
	srv, c := newPackedServer(t)
	fake, _ := fakeLLM(t, `{"lemma":"ház","pos":"NOUN","meanings":["дом"]}`)
	enableLLM(t, srv, fake.URL)

	count := func(table string) int {
		var n int
		if err := srv.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}
	before := map[string]int{}
	for _, tb := range []string{"note", "card", "user_gloss", "lemma_state", "morph_state"} {
		before[tb] = count(tb)
	}

	var out AIEntryResponse
	if resp := c.json(http.MethodPost, "/api/words/haz/entry", nil, &out); resp.StatusCode != http.StatusOK {
		t.Fatalf("entry: %d", resp.StatusCode)
	}
	if out.Lemma != "ház" || out.Model == "" {
		t.Errorf("entry = %+v; it must carry the model that wrote it", out)
	}

	for tb, was := range before {
		if now := count(tb); now != was {
			t.Errorf("asking for an entry wrote to %s (%d -> %d); it may only be "+
				"displayed until the learner saves it", tb, was, now)
		}
	}
}
