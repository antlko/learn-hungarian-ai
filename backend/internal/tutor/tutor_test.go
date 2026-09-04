package tutor

import (
	"strings"
	"testing"
	"testing/fstest"

	"tanulom/internal/llm"
)

// A three-file corpus: enough to pin the loader's front matter, the heading
// chain, the topic comments, and the retrieval ordering.
func testKB(t *testing.T) *KB {
	t.Helper()
	fsys := fstest.MapFS{
		"kb/src-a.md": &fstest.MapFile{Data: []byte(`---
title: Cases
source: Test grammar
url: https://example.test/cases
license: CC BY-SA 4.0
topics: locative-grid
---

# Cases

## The inessive
<!-- topics: locative-grid -->
The inessive suffix -ban/-ben means "in". Back-vowel stems take -ban: ` + "`házban`" + `.
Front-vowel stems take -ben: ` + "`kertben`" + `.

### A sub-point
Something short but long enough to survive the minimum length filter here.

## The dative
<!-- topics: dative -->
The dative -nak/-nek marks the indirect object and the possessor in a possessive construction.
`)},
		"kb/src-b.md": &fstest.MapFile{Data: []byte(`---
title: Verbs
source: Another grammar
license: public domain
---

# Verbs

## Definite conjugation
<!-- topics: definite-conjugation -->
The definite conjugation is used when the object is definite: ` + "`látom a házat`" + `.

## Tiny
x
`)},
		"kb/notes.txt": &fstest.MapFile{Data: []byte("ignored")},
	}
	kb, err := loadFrom(fsys, "kb")
	if err != nil {
		t.Fatal(err)
	}
	return kb
}

func TestLoaderSplitsOnHeadingsAndReadsTopics(t *testing.T) {
	kb := testKB(t)
	titles := map[string]Section{}
	for _, s := range kb.Sections() {
		titles[s.Title] = s
	}
	if len(kb.Sections()) != 4 {
		t.Fatalf("want 4 sections (the 'Tiny' one is dropped), got %d: %v", len(kb.Sections()), keys(titles))
	}
	ine, ok := titles["Cases › The inessive"]
	if !ok {
		t.Fatalf("heading chain missing: %v", keys(titles))
	}
	if ine.Source != "Test grammar" || ine.License != "CC BY-SA 4.0" || ine.URL != "https://example.test/cases" {
		t.Errorf("front matter not carried: %+v", ine)
	}
	if strings.Contains(ine.Body, "<!--") || !strings.Contains(ine.Body, "házban") {
		t.Errorf("body should be the text without the topic comment: %q", ine.Body)
	}
	sub := titles["Cases › The inessive › A sub-point"]
	if !hasTopic(sub.Topics, "locative-grid") {
		t.Errorf("a sub-section inherits its parent's topics, got %v", sub.Topics)
	}
	dat := titles["Cases › The dative"]
	if !hasTopic(dat.Topics, "dative") || !hasTopic(dat.Topics, "locative-grid") {
		t.Errorf("section topics = file default + own comment, got %v", dat.Topics)
	}
	if len(kb.Sources()) != 2 {
		t.Errorf("want two distinct sources, got %v", kb.Sources())
	}
}

func keys(m map[string]Section) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestRetrieveRanksTopicAndWords(t *testing.T) {
	kb := testKB(t)

	// A Hungarian form in the question outranks the open topic's own section.
	hits := kb.Retrieve(Query{Topic: "dative", Text: "why is it házban and not házben?"}, 2)
	if len(hits) == 0 || !strings.Contains(hits[0].Title, "inessive") {
		t.Fatalf("the section containing the asked form should lead: %+v", hits)
	}
	// ...but the open topic is still represented.
	found := false
	for _, h := range hits {
		if hasTopic(h.Topics, "dative") {
			found = true
		}
	}
	if !found {
		t.Errorf("the open topic's section must stay in play: %+v", hits)
	}

	// A question with no matching words falls back to the topic's sections.
	hits = kb.Retrieve(Query{Topic: "definite-conjugation", Text: "почему так сложно?"}, 3)
	if len(hits) != 1 || !hasTopic(hits[0].Topics, "definite-conjugation") {
		t.Errorf("a wordless question retrieves the topic alone: %+v", hits)
	}

	// Expansion terms reach the English corpus from a Russian question.
	hits = kb.Retrieve(Query{Topic: "", Text: "объясни инессив", Expand: []string{"inessive — in"}}, 3)
	if len(hits) == 0 || !strings.Contains(hits[0].Title, "inessive") {
		t.Errorf("expansion should retrieve the inessive section: %+v", hits)
	}

	// Nothing matches, no topic: nothing is returned, not the whole corpus.
	if hits := kb.Retrieve(Query{Text: "hello"}, 3); len(hits) != 0 {
		t.Errorf("an unrelated question retrieves nothing, got %+v", hits)
	}
}

func TestTokensKeepSuffixesAndDropStopwords(t *testing.T) {
	got := tokens("Why -ban and not -ben in `házban`?")
	want := []string{"-ban", "-ben", "házban"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("tokens = %v, want %v", got, want)
	}
}

// The prompt's order is the design ([R19 §3]): rules, the lesson, features,
// the learner, the reference, the thread, and the question with the language
// line last.
func TestBuildOrdersBlocksAndEndsWithTheLanguage(t *testing.T) {
	kb := testKB(t)
	in := Input{
		Lang: "ru", TopicCode: "locative-grid", TopicTitle: "Локативная система",
		Lesson: "# Правило\n\nДевять клеток.",
		Features: []Feature{{ID: "Case=Ine", Label: "инессив — в", Explain: "внутри чего-то",
			Allomorphs: map[string]string{"back": "ban", "front_unrounded": "ben", "front_rounded": "ben"}}},
		Learner:   []LearnerFeature{{Label: "инессив", Accuracy: 0.62, Observations: 25, Ready: true}, {Label: "иллатив", Observations: 3}},
		Mistakes:  []Mistake{{Typed: "házben", Expected: "házban"}},
		Reference: kb.Retrieve(Query{Topic: "locative-grid", Text: "házban"}, 2),
		History: []llm.Message{
			{Role: "user", Content: "первый вопрос"},
			{Role: "assistant", Content: "первый ответ"},
		},
		Question: "Почему -ban, а не -ben?",
	}
	msgs := Build(in)
	if len(msgs) != 4 || msgs[0].Role != "system" || msgs[3].Role != "user" {
		t.Fatalf("want system + 2 history + user, got %d messages", len(msgs))
	}
	sys := msgs[0].Content
	// The rules mention the block names, so the needles are the HEADINGS.
	order := []string{"Объясняй ПО-РУССКИ", "## УРОК ИЗ ПРИЛОЖЕНИЯ", "Девять клеток", "## ПРИЗНАКИ", "-ban / -ben / -ben",
		"## ЭТОТ УЧЕНИК", "62%", "пока 3 ответов", "házben", "## СПРАВОЧНИК", "inessive"}
	last := -1
	for _, needle := range order {
		i := strings.Index(sys, needle)
		if i < 0 {
			t.Fatalf("system prompt lacks %q:\n%s", needle, sys)
		}
		if i < last {
			t.Errorf("%q appears out of order", needle)
		}
		last = i
	}
	if !strings.HasSuffix(msgs[3].Content, "а затем разбор по словам.") {
		t.Errorf("the language line must be the very last thing: %q", msgs[3].Content)
	}
	if !strings.HasPrefix(msgs[3].Content, in.Question) {
		t.Errorf("the question leads the last message: %q", msgs[3].Content)
	}
	// English, for the other locale.
	in.Lang = "en"
	msgs = Build(in)
	if !strings.Contains(msgs[0].Content, "Explain in ENGLISH") || !strings.HasSuffix(msgs[len(msgs)-1].Content, "then a word-by-word breakdown.") {
		t.Errorf("English prompt not selected: %q … %q", msgs[0].Content[:40], msgs[len(msgs)-1].Content)
	}
}

// A free chat whose question matched no lesson (ADR-029) tells the model so
// under the lesson heading — never a blank block it might read as "nothing to
// respect" — and still carries the reference and the language line.
// The language rule names the EXPLANATION, never the material (ADR-030).
// «Отвечай ТОЛЬКО ПО-РУССКИ» — first line and last, the two positions a small
// model follows most reliably — made a 1.7B model answer «напиши на
// венгерском …» with the Russian sentence unchanged. Both prompts must say
// that Hungarian is written in Hungarian, must give a translation request its
// shape, and must never say ONLY again.
func TestPromptLanguageRuleNamesTheExplanationNotTheMaterial(t *testing.T) {
	for _, tc := range []struct {
		lang, only string
		needles    []string
	}{
		{"ru", "ТОЛЬКО", []string{"Объясняй ПО-РУССКИ", "пиши ПО-ВЕНГЕРСКИ", "перевести на венгерский", "ПО СЛОВАМ",
			"Русское предложение без венгерского перевода — не ответ"}},
		{"en", "ONLY", []string{"Explain in ENGLISH", "in HUNGARIAN", "translate into Hungarian", "WORD BY WORD",
			"An English sentence without the Hungarian is not an answer"}},
	} {
		msgs := Build(Input{Lang: tc.lang, Question: "q"})
		sys := msgs[0].Content
		if strings.Contains(sys, tc.only) {
			t.Errorf("%s: the language rule must not say %s — that is what echoed the Russian back:\n%s", tc.lang, tc.only, sys)
		}
		for _, n := range tc.needles {
			if !strings.Contains(sys, n) {
				t.Errorf("%s: prompt lacks %q", tc.lang, n)
			}
		}
		last := msgs[len(msgs)-1].Content
		if strings.Contains(last, tc.only) {
			t.Errorf("%s: the closing line must not say %s either: %q", tc.lang, tc.only, last)
		}
	}
}

func TestBuildStatesTheAbsenceOfALesson(t *testing.T) {
	kb := testKB(t)
	msgs := Build(Input{Lang: "ru", Question: "Как сказать «привет»?",
		Reference: kb.Retrieve(Query{Text: "házban"}, 1)})
	sys := msgs[0].Content
	if !strings.Contains(sys, "## УРОК ИЗ ПРИЛОЖЕНИЯ\n\nОткрытого урока нет") {
		t.Errorf("the no-lesson case must be stated:\n%s", sys)
	}
	if strings.Contains(sys, "## ПРИЗНАКИ") || strings.Contains(sys, "## ЭТОТ УЧЕНИК") {
		t.Errorf("no topic means no features and no learner block:\n%s", sys)
	}
	if !strings.Contains(sys, "## СПРАВОЧНИК") {
		t.Errorf("the reference still travels:\n%s", sys)
	}
	if !strings.HasSuffix(msgs[len(msgs)-1].Content, "а затем разбор по словам.") {
		t.Errorf("the language line still ends the prompt")
	}
	if !strings.Contains(Build(Input{Lang: "en", Question: "q"})[0].Content, "No lesson is open") {
		t.Errorf("English no-lesson text missing")
	}
}

func TestBuildCutsToBudgetAndKeepsTheHeadOfTheLesson(t *testing.T) {
	long := strings.Repeat("Это очень длинный урок. ", 1000)
	msgs := Build(Input{Lang: "ru", TopicTitle: "t", Lesson: long, Question: "q",
		History: []llm.Message{{Role: "user", Content: strings.Repeat("x", 10000)}}})
	sys := msgs[0].Content
	// The rules travel whole; the lesson is what gets cut.
	if len(sys) > len(systemRU)+lessonBudget+300 {
		t.Errorf("system prompt not cut: %d bytes", len(sys))
	}
	if !strings.Contains(sys, "Это очень длинный урок") || !strings.Contains(sys, " …") {
		t.Errorf("the lesson head must survive, with an ellipsis marking the cut")
	}
	if len(msgs[1].Content) > historyBudget+8 {
		t.Errorf("history not cut: %d bytes", len(msgs[1].Content))
	}
	// A cut must never split a UTF-8 sequence.
	for _, m := range msgs {
		if !isValidUTF8(m.Content) {
			t.Errorf("invalid UTF-8 after cut")
		}
	}
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '\uFFFD' {
			return false
		}
	}
	return true
}

func TestBackticksAndAttestation(t *testing.T) {
	answer := "Say `házban` (in the house), not `házben`. The phrase `a nagy ház` is fine; `-ban` is the suffix, `ben` too.\n```\nnot a word\n```\n and `Kérek egy kávét.`"
	words := Backticked(answer)
	want := []string{"házban", "házben", "a", "nagy", "ház", "ban", "ben", "Kérek", "egy", "kávét"}
	if strings.Join(words, " ") != strings.Join(want, " ") {
		t.Errorf("Backticked = %v, want %v", words, want)
	}
	known := map[string]bool{"házban": true, "a": true, "nagy": true, "ház": true, "kérek": true, "egy": true, "kávét": true}
	un := Unattested(words, func(w string) bool { return known[strings.ToLower(w)] })
	if strings.Join(un, " ") != "házben" {
		t.Errorf("Unattested = %v, want [házben]: bare suffixes are skipped, real forms checked", un)
	}
	// A whole translated sentence (ADR-030) is checked word by word — the old
	// 80-byte cap let every one of them through unchecked.
	sentence := "`Eljöttem ebbe a faluba, és bementem a házamba, ahol a családom már várt rám a vacsorával.`"
	if len(sentence) <= 82 {
		t.Fatalf("the fixture must be longer than the old cap, got %d bytes", len(sentence))
	}
	got := Backticked(sentence)
	if len(got) != 13 || got[0] != "Eljöttem" || got[12] != "vacsorával" {
		t.Errorf("a backticked sentence is split into its distinct words, got %v", got)
	}
	if got := Backticked("`" + strings.Repeat("a ", 120) + "`"); got != nil {
		t.Errorf("a span past 200 bytes is a code block, not words: %v", got)
	}
	// Never nil: `null` on the wire froze the panel on every backtick-free answer.
	if none := Unattested(Backticked("no backticks here"), func(string) bool { return true }); none == nil || len(none) != 0 {
		t.Errorf("Unattested with nothing to check = %#v, want an empty non-nil list", none)
	}
}
