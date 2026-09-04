package tutor

import (
	"strings"
	"testing"

	"tanulom/internal/llm"
)

func sampleCard(phase string) CardContext {
	correct := false
	c := CardContext{
		Direction: "cloze", Phase: phase, HU: "A házban lakom.",
		Form: "házban", Lemma: "ház", Gloss: "дом", GlossOther: "house",
		Translation: "I live in the house.", TranslationFallback: true,
		Features: []Feature{{ID: "Case=Ine", Label: "инессив — в", Explain: "внутри",
			Allomorphs: map[string]string{"back": "ban", "front_unrounded": "ben", "front_rounded": "ben"}}},
		Typed: "házben", Expected: "házban", Class: "harmony", WrongFeatureLabel: "инессив — в", Correct: &correct,
	}
	if phase == "question" {
		c.Withheld = true
		c.Form, c.Expected, c.Typed, c.Correct, c.Class = "", "", "", nil, ""
		c.HU = "A ____ lakom."
	}
	return c
}

// The CARD block sits right after the lesson and before the features; the
// shape of a prepared answer is the last thing before the language line
// (ADR-034, [R19 §3] — the ends of the prompt are what a small model reads).
func TestBuildPlacesTheCardAfterTheLessonAndTheShapeLast(t *testing.T) {
	card := sampleCard("answer")
	msgs := Build(Input{
		Lang: "ru", TopicCode: "locative-grid", TopicTitle: "Локативная система",
		Lesson:   "# Правило\n\nДевять клеток.",
		Features: []Feature{{ID: "Case=Ine", Label: "инессив"}},
		Question: Compose(TplWhyNotMine, card, "ru"), Card: &card, Shape: Shape(TplWhyNotMine, card, "ru"),
	})
	sys := msgs[0].Content
	order := []string{"## УРОК ИЗ ПРИЛОЖЕНИЯ", "Девять клеток", "## КАРТОЧКА", "`A házban lakom.`", "ученик написал: `házben`",
		"нужно было: `házban`", "гармония гласных", "## ПРИЗНАКИ"}
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
	tail := msgs[len(msgs)-1].Content
	if !strings.Contains(tail, "Я написал `házben`") || !strings.Contains(tail, "Форма ответа:") ||
		!strings.Contains(tail, "Без проверочного вопроса") || !strings.HasSuffix(tail, "а затем разбор по словам.") {
		t.Errorf("the tail is question, shape, then the language line:\n%s", tail)
	}
	if !strings.Contains(tail, "класс гармонии основы") {
		t.Errorf("whyNotMine's section 3 follows the error class:\n%s", tail)
	}
}

// Before the answer the block says the answer is withheld and carries none of
// it: no expected form, no typed form, no verdict; and the model is told, in
// the block itself, not to name it.
func TestBuildWithholdsTheAnswerInTheQuestionPhase(t *testing.T) {
	card := sampleCard("question")
	msgs := Build(Input{Lang: "ru", TopicTitle: "t", Lesson: "x", Question: Compose(TplHintRule, card, "ru"), Card: &card,
		Shape: Shape(TplHintRule, card, "ru")})
	sys := msgs[0].Content
	block := sys[strings.Index(sys, "## КАРТОЧКА"):]
	if i := strings.Index(block[3:], "\n## "); i >= 0 {
		block = block[:i+3]
	}
	if strings.Contains(block, "házban") || strings.Contains(block, "нужно было") || strings.Contains(block, "ответ неверный") {
		t.Errorf("the withheld block must carry nothing that is the answer:\n%s", block)
	}
	if !strings.Contains(block, "ОТВЕТ СКРЫТ") || !strings.Contains(block, "A ____ lakom.") || !strings.Contains(block, "`ház`") {
		t.Errorf("the withheld block says so, shows the blanked sentence and the dictionary word:\n%s", block)
	}
	if !strings.Contains(msgs[len(msgs)-1].Content, "без готовой формы") {
		t.Errorf("hintRule asks for the rule, not the form:\n%s", msgs[len(msgs)-1].Content)
	}
}

// A card takes its room from the reference, not on top of it: the system
// prompt with a card is no longer than without one at full budgets.
func TestBuildWithACardStaysUnderTheOldTotal(t *testing.T) {
	long := strings.Repeat("Это очень длинный урок. ", 300)
	// Three full-size sections, as a real retrieval returns them; the test
	// corpus's own sections are too short to reach the budget.
	body := strings.Repeat("The inessive case marks location inside something. ", 40)
	ref := []Hit{{Section: Section{Title: "a", Source: "s", Body: body}}, {Section: Section{Title: "b", Source: "s", Body: body}},
		{Section: Section{Title: "c", Source: "s", Body: body}}}
	without := Build(Input{Lang: "ru", TopicCode: "locative-grid", TopicTitle: "t", Lesson: long, Reference: ref, Question: "q"})
	card := sampleCard("answer")
	with := Build(Input{Lang: "ru", TopicCode: "locative-grid", TopicTitle: "t", Lesson: long, Reference: ref, Question: "q", Card: &card})
	if len(with[0].Content) > len(without[0].Content) {
		t.Errorf("with a card the system prompt grew: %d > %d bytes", len(with[0].Content), len(without[0].Content))
	}
}

// Every template composes a question and a shape in both languages, and the
// visible question names the card's Hungarian in backticks so the lesson can
// be found from it (ADR-029) and the dictionary check can see it.
func TestCardTemplatesAreWrittenInBothLanguages(t *testing.T) {
	for _, lang := range []string{"ru", "en"} {
		for _, tpl := range Templates {
			phase := "answer"
			if tpl == TplHintForm || tpl == TplHintWord || tpl == TplHintRule {
				phase = "question"
			}
			card := sampleCard(phase)
			q, shape := Compose(tpl, card, lang), Shape(tpl, card, lang)
			if q == "" || shape == "" {
				t.Errorf("%s/%s: question %q, shape %q", lang, tpl, q, shape)
			}
			if tpl != TplExplainTask && tpl != TplHintWord && !strings.Contains(q, "`") {
				t.Errorf("%s/%s: the question names no Hungarian: %q", lang, tpl, q)
			}
			want := "Без проверочного вопроса"
			if lang == "en" {
				want = "No check question"
			}
			if !strings.Contains(shape, want) {
				t.Errorf("%s/%s: the shape must switch the check question off: %q", lang, tpl, shape)
			}
		}
	}
	// The block itself, both languages, mentions the withheld line only when withheld.
	if b := sampleCard("answer").block("en"); !strings.Contains(b, "## CARD") || strings.Contains(b, "WITHHELD") {
		t.Errorf("answer-phase block:\n%s", b)
	}
	if b := sampleCard("question").block("en"); !strings.Contains(b, "WITHHELD") {
		t.Errorf("question-phase block:\n%s", b)
	}
}

// The hint templates exist only before the answer, one per direction that
// shows something to hint about; the ones that name the answer only after it.
func TestAllowedTemplatesFollowThePhase(t *testing.T) {
	cases := []struct {
		tpl        Template
		dir, phase string
		typed, ok  bool
	}{
		{TplHintForm, "recognize", "question", false, true},
		{TplHintForm, "produce", "question", false, false},
		{TplHintWord, "produce", "question", false, true},
		{TplHintRule, "cloze", "question", false, true},
		{TplHintRule, "cloze", "answer", false, false},
		{TplExplainTask, "listen", "question", false, true},
		{TplWhyForm, "cloze", "question", false, false},
		{TplWhyForm, "recognize", "answer", false, true},
		{TplWhyNotMine, "cloze", "answer", true, true},
		{TplWhyNotMine, "cloze", "answer", false, false},
		{TplWhyNotMine, "recognize", "answer", true, false},
		{TplBreakdown, "spell", "answer", false, true},
		{Template("bogus"), "cloze", "answer", true, false},
	}
	for _, c := range cases {
		if got := Allowed(c.tpl, c.dir, c.phase, c.typed); got != c.ok {
			t.Errorf("Allowed(%s, %s, %s, typed=%v) = %v, want %v", c.tpl, c.dir, c.phase, c.typed, got, c.ok)
		}
	}
}

// The model reciting the CARD heading is an echo like any other.
func TestEchoAtCutsTheCardHeading(t *testing.T) {
	echoed := "Ответ.\n\n## КАРТОЧКА\n- тип: …"
	if at := EchoAt(echoed); at != strings.Index(echoed, "## КАРТОЧКА") {
		t.Errorf("EchoAt = %d, want the card heading at %d", at, strings.Index(echoed, "## КАРТОЧКА"))
	}
	if at := EchoAt("Answer.\n## CARD\n"); at < 0 {
		t.Errorf("EchoAt missed the English card heading")
	}
	_ = llm.Message{}
}
