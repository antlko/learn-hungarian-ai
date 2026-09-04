package tutor

import (
	"fmt"
	"strings"
)

// The card the learner is looking at, as the tutor sees it (ADR-034).
//
// A question asked from the review card carries the CARD itself — the
// sentence, the word and its dictionary form, the glosses, the pack's
// translation with its label, the graded features with their labels and
// allomorphs, what was typed against what was wanted and how the diff
// classified the miss — and the server composes both the visible question
// and the answer's shape from it. The client never assembles these facts:
// the labels, the allomorphs and the diff class live in the pack and the
// analyser, and the client is forbidden to derive the expected answer
// (ADR-017).
//
// Before the answer the block WITHHOLDS the answer (Withheld) and the
// question is one of the hint templates, which ask for the rule and never the
// form. The handler strips the fields; this package only prints what it is
// given and says, in the model's language, that the rest is withheld.

// CardContext is what the prompt's «КАРТОЧКА» / "CARD" block is built from.
type CardContext struct {
	// Direction is the card's kind: recognize, produce, cloze, listen, spell.
	Direction string
	// Phase is "question" or "answer".
	Phase string
	// HU is the sentence; in the question phase of a cloze it is already
	// blanked (`A ____ lakom.`).
	HU string
	// Form is the word as it stands in the sentence; Lemma its dictionary
	// form; Gloss the meaning in the answer language, GlossOther in the other.
	Form, Lemma, Gloss, GlossOther string
	// Translation is the sentence's translation in the answer language, or
	// in the other one when TranslationFallback is set; TranslationModel
	// names the machine that wrote it, blank for a pack translation.
	Translation, TranslationModel string
	TranslationFallback           bool
	// Features are the form's graded features, with the pack's labels.
	Features []Feature
	// Typed and Expected are the learner's answer and the graded string;
	// Class is the diff's error class, WrongFeatureLabel the label of the
	// feature the diff blamed; Correct is the verdict when there was one.
	Typed, Expected          string
	Class, WrongFeatureLabel string
	Correct                  *bool
	AcceptedWithAccents      bool
	// Withheld says the answer was stripped from this block: the model is
	// told to explain the rule and never name the form or the word.
	Withheld bool
}

// Template is one prepared question the review card offers (ADR-034). The
// ids are the contract with the client; the client decides which to SHOW
// from the card's direction, phase and outcome, the server composes the text
// and refuses one that is not offered on that card in that phase.
type Template string

const (
	TplBreakdown     Template = "breakdown"
	TplWhyForm       Template = "whyForm"
	TplWhyNotMine    Template = "whyNotMine"
	TplTranslateWord Template = "translateWord"
	TplSpelling      Template = "spelling"
	TplExplainTask   Template = "explainTask"
	TplHintForm      Template = "hintForm"
	TplHintWord      Template = "hintWord"
	TplHintRule      Template = "hintRule"
)

// Templates lists every template, for tests and docs.
var Templates = []Template{TplBreakdown, TplWhyForm, TplWhyNotMine, TplTranslateWord,
	TplSpelling, TplExplainTask, TplHintForm, TplHintWord, TplHintRule}

// Allowed reports whether a template may be asked on a card of this
// direction in this phase. The hint templates exist only before the answer,
// one per direction whose card shows something to hint about; everything
// that names the form or the meaning exists only after it; whyNotMine needs
// something typed to compare.
func Allowed(tpl Template, direction, phase string, typed bool) bool {
	switch phase {
	case "question":
		switch tpl {
		case TplExplainTask:
			return true
		case TplHintForm:
			return direction == "recognize"
		case TplHintWord:
			return direction == "produce"
		case TplHintRule:
			return direction == "cloze"
		}
		return false
	case "answer":
		switch tpl {
		case TplBreakdown, TplWhyForm, TplTranslateWord, TplSpelling, TplExplainTask:
			return true
		case TplWhyNotMine:
			return typed && (direction == "produce" || direction == "cloze" || direction == "spell")
		}
		return false
	}
	return false
}

// cardText is the per-language copy of the block, the questions and the
// shapes. Two languages, always (ADR-004).
type cardText struct {
	heading, typeLabel, phaseQ, phaseA, sentence, packTr, machineTr, fallbackTr, word, lemma,
	meaning, features, typed, expected, correct, wrong, accents, errClass, wrongFeat, withheld string
	directions map[string]string
	classes    map[string]string
	noCheck    string
	compose    map[Template]func(c CardContext) string
	shape      map[Template]func(c CardContext) string
}

var cardTexts = map[string]cardText{
	"ru": {
		heading: "КАРТОЧКА", typeLabel: "тип карточки", phaseQ: "фаза: ВОПРОС — ученик ещё не ответил",
		phaseA: "фаза: ответ показан", sentence: "предложение", packTr: "перевод из пакета",
		machineTr: "машинный перевод, %s", fallbackTr: "(перевод есть только на другом языке)",
		word: "слово в предложении", lemma: "словарная форма", meaning: "значение",
		features: "признаки формы (id · название · варианты суффикса)",
		typed:    "ученик написал", expected: "нужно было", correct: "ответ верный", wrong: "ответ неверный",
		accents: "принято без диакритики", errClass: "класс ошибки", wrongFeat: "неверный признак",
		withheld: "ОТВЕТ СКРЫТ: ученик ещё отвечает. Объясняй правило и НЕ называй ни искомую форму, ни искомое слово, ни его перевод.",
		directions: map[string]string{
			"recognize": "узнавание — видит венгерское предложение, вспоминает, что значит слово",
			"produce":   "воспроизведение — видит значение, набирает венгерское слово в словарной форме",
			"cloze":     "форма в предложении — вставляет пропущенное слово в нужной форме",
			"listen":    "на слух — слышит слово, вспоминает, что оно значит",
			"spell":     "написание — слышит слово, набирает его со всеми долготами и умлаутами",
		},
		classes: map[string]string{
			"harmony": "гармония гласных", "vowel_length": "долгота гласного", "vowel_quality": "качество гласного (огублённость)",
			"case": "падеж", "definiteness": "определённость", "possessor": "притяжательность", "stem": "другое слово",
			"spelling": "опечатка", "unknown": "не классифицирована",
		},
		noCheck: "Отвечай ровно этими разделами, с номерами. Без проверочного вопроса в конце.",
	},
	"en": {
		heading: "CARD", typeLabel: "card type", phaseQ: "phase: QUESTION — the learner has not answered yet",
		phaseA: "phase: the answer is shown", sentence: "sentence", packTr: "pack translation",
		machineTr: "machine translation, %s", fallbackTr: "(the only translation is in the other language)",
		word: "word in the sentence", lemma: "dictionary form", meaning: "meaning",
		features: "features of the form (id · name · suffix variants)",
		typed:    "the learner typed", expected: "expected", correct: "answered correctly", wrong: "answered wrongly",
		accents: "accepted without diacritics", errClass: "error class", wrongFeat: "wrong feature",
		withheld: "THE ANSWER IS WITHHELD: the learner is still answering. Explain the rule and do NOT name the form, the word or its meaning.",
		directions: map[string]string{
			"recognize": "recognition — sees the Hungarian sentence, recalls what the word means",
			"produce":   "production — sees the meaning, types the Hungarian word in its dictionary form",
			"cloze":     "form in the sentence — fills the gap with the word in the right form",
			"listen":    "listening — hears the word, recalls what it means",
			"spell":     "spelling — hears the word, types it with every long vowel and umlaut",
		},
		classes: map[string]string{
			"harmony": "vowel harmony", "vowel_length": "vowel length", "vowel_quality": "vowel quality (rounding)",
			"case": "case", "definiteness": "definiteness", "possessor": "possession", "stem": "a different word",
			"spelling": "a typo", "unknown": "not classified",
		},
		noCheck: "Answer in exactly these numbered sections. No check question at the end.",
	},
}

func init() {
	ru := cardTexts["ru"]
	ru.compose = map[Template]func(CardContext) string{
		TplBreakdown: func(c CardContext) string {
			return fmt.Sprintf("Разбери предложение `%s` целиком: что оно значит, что значит каждое слово, и почему слово `%s` стоит в этой форме.", c.HU, or(c.Form, c.Lemma))
		},
		TplWhyForm: func(c CardContext) string {
			return fmt.Sprintf("Почему в предложении `%s` слово `%s` стоит в форме `%s`? Какие признаки она несёт и почему именно этот вариант суффикса?", c.HU, or(c.Lemma, c.Form), or(c.Form, c.Lemma))
		},
		TplWhyNotMine: func(c CardContext) string {
			return fmt.Sprintf("Я написал `%s`, а нужно было `%s` (карточка: %s). Что такое `%s`, почему здесь `%s`, и в чём именно разница?", c.Typed, c.Expected, cue(c), c.Typed, c.Expected)
		},
		TplTranslateWord: func(c CardContext) string {
			return fmt.Sprintf("Что значит `%s` и как оно употребляется? Дай два коротких предложения с переводом.", or(c.Lemma, c.Form))
		},
		TplSpelling: func(c CardContext) string {
			if c.Typed != "" {
				return fmt.Sprintf("Как пишется `%s` и почему — долготы, двойные согласные, ly/j? Я написал `%s`.", or(c.Expected, c.Form), c.Typed)
			}
			return fmt.Sprintf("Как пишется `%s` и почему — долготы, двойные согласные, ly/j?", or(c.Expected, c.Form))
		},
		TplExplainTask: func(c CardContext) string {
			return fmt.Sprintf("Что спрашивает карточка типа «%s» и как к ней подойти?", short(cardTexts["ru"].directions[c.Direction]))
		},
		TplHintForm: func(c CardContext) string {
			return fmt.Sprintf("Подскажи, из чего состоит слово `%s` в предложении `%s` — основа и суффиксы, часть речи, — не переводя его.", c.Form, c.HU)
		},
		TplHintWord: func(c CardContext) string {
			return fmt.Sprintf("Подскажи, не называя слова: какое венгерское слово значит «%s»? Часть речи, класс гармонии, первая буква, похожие слова.", c.Gloss)
		},
		TplHintRule: func(c CardContext) string {
			return fmt.Sprintf("В предложении `%s` пропущено слово `%s` («%s»). Какая форма нужна и почему — правило, без готовой формы?", c.HU, c.Lemma, c.Gloss)
		},
	}
	ru.shape = map[Template]func(CardContext) string{
		TplBreakdown: func(c CardContext) string {
			return "Форма ответа:\n1. Что значит предложение (буквально).\n2. По словам: слово — значение — суффикс или приставка и почему.\n3. Форма `" + or(c.Form, c.Lemma) + "`: какой признак, какой вариант суффикса, почему."
		},
		TplWhyForm: func(c CardContext) string {
			return "Форма ответа:\n1. Что это за слово.\n2. Какие признаки несёт форма `" + or(c.Form, c.Lemma) + "`.\n3. Почему именно этот вариант (класс гармонии, основа).\n4. Ещё два примера с тем же суффиксом, с переводом."
		},
		TplWhyNotMine: func(c CardContext) string {
			return "Форма ответа:\n1. Что такое `" + c.Typed + "` — есть ли такая форма и что она значила бы.\n2. Почему здесь `" + c.Expected + "`.\n3. В чём именно разница — назови механизм: " + classAdvice("ru", c) + "\n4. Два примера с переводом."
		},
		TplTranslateWord: func(c CardContext) string {
			return "Форма ответа:\n1. Значения (одно–три).\n2. Два коротких предложения с переводом.\n3. С каким падежом и в каких сочетаниях употребляется."
		},
		TplSpelling: func(c CardContext) string {
			return "Форма ответа:\n1. Слово и значение.\n2. Написание по слогам: долгие гласные, двойные согласные, ly/j, dz/dzs.\n3. Как это слышится и от чего отличать.\n4. Два похожих слова."
		},
		TplExplainTask: func(c CardContext) string {
			return "Форма ответа: один абзац — что спрашивает карточка этого типа и как к ней подойти."
		},
		TplHintForm: func(c CardContext) string {
			return "Форма ответа:\n1. Основа и суффиксы слова `" + c.Form + "`, часть речи.\n2. Какие признаки несёт форма.\nНЕ переводи слово и не называй его значение."
		},
		TplHintWord: func(c CardContext) string {
			return "Форма ответа: подсказка, не называя слова — часть речи, класс гармонии гласных, первая буква, два похожих по смыслу слова, которые НЕ являются ответом."
		},
		TplHintRule: func(c CardContext) string {
			return "Форма ответа:\n1. Роль слова в предложении.\n2. Какой признак нужен и почему.\n3. Как образовать форму — правило, без готовой формы."
		},
	}
	cardTexts["ru"] = ru

	en := cardTexts["en"]
	en.compose = map[Template]func(CardContext) string{
		TplBreakdown: func(c CardContext) string {
			return fmt.Sprintf("Break the sentence `%s` down: what it means, what each word means, and why `%s` is in this form.", c.HU, or(c.Form, c.Lemma))
		},
		TplWhyForm: func(c CardContext) string {
			return fmt.Sprintf("In the sentence `%s`, why is `%s` in the form `%s`? Which features does it carry, and why this suffix variant?", c.HU, or(c.Lemma, c.Form), or(c.Form, c.Lemma))
		},
		TplWhyNotMine: func(c CardContext) string {
			return fmt.Sprintf("I wrote `%s`, but `%s` was wanted (card: %s). What is `%s`, why `%s` here, and what exactly is the difference?", c.Typed, c.Expected, cue(c), c.Typed, c.Expected)
		},
		TplTranslateWord: func(c CardContext) string {
			return fmt.Sprintf("What does `%s` mean and how is it used? Give two short sentences with translations.", or(c.Lemma, c.Form))
		},
		TplSpelling: func(c CardContext) string {
			if c.Typed != "" {
				return fmt.Sprintf("How is `%s` spelled, and why — long vowels, double consonants, ly/j? I wrote `%s`.", or(c.Expected, c.Form), c.Typed)
			}
			return fmt.Sprintf("How is `%s` spelled, and why — long vowels, double consonants, ly/j?", or(c.Expected, c.Form))
		},
		TplExplainTask: func(c CardContext) string {
			return fmt.Sprintf("What does a \"%s\" card ask, and how should I approach it?", short(cardTexts["en"].directions[c.Direction]))
		},
		TplHintForm: func(c CardContext) string {
			return fmt.Sprintf("Give me a hint about what `%s` in `%s` is made of — stem and suffixes, part of speech — without translating it.", c.Form, c.HU)
		},
		TplHintWord: func(c CardContext) string {
			return fmt.Sprintf("A hint without naming the word: which Hungarian word means \"%s\"? Part of speech, harmony class, first letter, similar words.", c.Gloss)
		},
		TplHintRule: func(c CardContext) string {
			return fmt.Sprintf("In `%s` the word `%s` (\"%s\") is missing. Which form is needed, and why — the rule, without the finished form?", c.HU, c.Lemma, c.Gloss)
		},
	}
	en.shape = map[Template]func(CardContext) string{
		TplBreakdown: func(c CardContext) string {
			return "Answer shape:\n1. What the sentence means (literally).\n2. Word by word: word — meaning — suffix or preverb and why.\n3. The form `" + or(c.Form, c.Lemma) + "`: which feature, which suffix variant, why."
		},
		TplWhyForm: func(c CardContext) string {
			return "Answer shape:\n1. What the word is.\n2. Which features the form `" + or(c.Form, c.Lemma) + "` carries.\n3. Why this variant (harmony class, stem).\n4. Two more examples with the same suffix, translated."
		},
		TplWhyNotMine: func(c CardContext) string {
			return "Answer shape:\n1. What `" + c.Typed + "` is — whether such a form exists and what it would mean.\n2. Why `" + c.Expected + "` here.\n3. What exactly the difference is — name the mechanism: " + classAdvice("en", c) + "\n4. Two examples with translations."
		},
		TplTranslateWord: func(c CardContext) string {
			return "Answer shape:\n1. Meanings (one to three).\n2. Two short sentences with translations.\n3. Which case it takes and which collocations it appears in."
		},
		TplSpelling: func(c CardContext) string {
			return "Answer shape:\n1. The word and its meaning.\n2. Spelling by syllable: long vowels, double consonants, ly/j, dz/dzs.\n3. How it sounds and what to tell it apart from.\n4. Two similar-looking words."
		},
		TplExplainTask: func(c CardContext) string {
			return "Answer shape: one paragraph — what a card of this type asks and how to approach it."
		},
		TplHintForm: func(c CardContext) string {
			return "Answer shape:\n1. The stem and suffixes of `" + c.Form + "`, part of speech.\n2. Which features the form carries.\nDo NOT translate the word or name its meaning."
		},
		TplHintWord: func(c CardContext) string {
			return "Answer shape: a hint without naming the word — part of speech, vowel-harmony class, first letter, two words similar in meaning that are NOT the answer."
		},
		TplHintRule: func(c CardContext) string {
			return "Answer shape:\n1. The word's role in the sentence.\n2. Which feature is needed and why.\n3. How to build the form — the rule, without the finished form."
		},
	}
	cardTexts["en"] = en
}

// Compose writes the visible question for a template, in the answer language.
func Compose(tpl Template, c CardContext, lang string) string {
	t := cardTexts[langOf(lang)]
	f := t.compose[tpl]
	if f == nil {
		return ""
	}
	return f(c)
}

// Shape writes the answer's shape for a template — the numbered sections the
// model is told to use, and the line that switches the check question off.
func Shape(tpl Template, c CardContext, lang string) string {
	t := cardTexts[langOf(lang)]
	f := t.shape[tpl]
	if f == nil {
		return ""
	}
	return f(c) + "\n" + t.noCheck
}

// classAdvice is the instruction for section 3 of whyNotMine, by the diff's
// error class (internal/morph/diff.go).
func classAdvice(lang string, c CardContext) string {
	if lang == "ru" {
		switch c.Class {
		case "harmony":
			return "назови класс гармонии основы и три варианта суффикса."
		case "vowel_length":
			return "долгота меняет слово: `" + c.Typed + "` и `" + c.Expected + "` — разные слова? приведи минимальную пару."
		case "vowel_quality":
			return "огублённость ö/ü против o/u, сравни с русским и английским."
		case "case", "definiteness", "possessor":
			return "что выражает признак «" + or(c.WrongFeatureLabel, c.Class) + "», каким суффиксом, почему здесь он, а не тот, что в `" + c.Typed + "`."
		case "stem":
			return "это другое слово: что значит `" + c.Typed + "`, что значит `" + c.Expected + "`, два примера с каждым."
		case "spelling":
			return "это опечатка или правило написания?"
		}
		return "назови разницу, которую видишь."
	}
	switch c.Class {
	case "harmony":
		return "name the stem's harmony class and the three suffix variants."
	case "vowel_length":
		return "length changes the word: are `" + c.Typed + "` and `" + c.Expected + "` different words? give a minimal pair."
	case "vowel_quality":
		return "rounding, ö/ü against o/u; compare with English and Russian."
	case "case", "definiteness", "possessor":
		return "what the feature \"" + or(c.WrongFeatureLabel, c.Class) + "\" expresses, with which suffix, and why it and not the one in `" + c.Typed + "`."
	case "stem":
		return "it is a different word: what `" + c.Typed + "` means, what `" + c.Expected + "` means, two examples of each."
	case "spelling":
		return "is it a typo or a spelling rule?"
	}
	return "name the difference you see."
}

// block renders the prompt's CARD section in the answer language.
func (c CardContext) block(lang string) string {
	t := cardTexts[langOf(lang)]
	var b strings.Builder
	b.WriteString("## " + t.heading + "\n")
	if d := t.directions[c.Direction]; d != "" {
		b.WriteString("- " + t.typeLabel + ": " + d + "\n")
	} else {
		b.WriteString("- " + t.typeLabel + ": " + c.Direction + "\n")
	}
	if c.Phase == "question" {
		b.WriteString("- " + t.phaseQ + "\n")
	} else {
		b.WriteString("- " + t.phaseA + "\n")
	}
	// First, where a budget cut cannot reach it: the one line that changes
	// what the model may say.
	if c.Withheld {
		b.WriteString("- " + t.withheld + "\n")
	}
	if c.HU != "" {
		line := "- " + t.sentence + ": `" + c.HU + "`"
		if c.Translation != "" {
			line += " — " + c.Translation
			switch {
			case c.TranslationModel != "":
				line += " (" + fmt.Sprintf(t.machineTr, c.TranslationModel) + ")"
			case c.TranslationFallback:
				line += " " + t.fallbackTr
			default:
				line += " (" + t.packTr + ")"
			}
		}
		b.WriteString(line + "\n")
	}
	if c.Form != "" {
		b.WriteString("- " + t.word + ": `" + c.Form + "`\n")
	}
	if c.Lemma != "" {
		b.WriteString("- " + t.lemma + ": `" + c.Lemma + "`\n")
	}
	if c.Gloss != "" {
		line := "- " + t.meaning + ": " + c.Gloss
		if c.GlossOther != "" && c.GlossOther != c.Gloss {
			line += " / " + c.GlossOther
		}
		b.WriteString(line + "\n")
	}
	if len(c.Features) > 0 {
		b.WriteString("- " + t.features + ":\n")
		for _, f := range c.Features {
			line := "  - " + f.ID + " · " + f.Label
			if v := allomorphLine(f.Allomorphs); v != "" {
				line += " · " + v
			}
			if f.Explain != "" {
				line += " · " + oneLine(f.Explain)
			}
			b.WriteString(line + "\n")
		}
	}
	if c.Typed != "" {
		b.WriteString("- " + t.typed + ": `" + c.Typed + "`\n")
	}
	if c.Expected != "" && !c.Withheld {
		b.WriteString("- " + t.expected + ": `" + c.Expected + "`\n")
	}
	if c.Correct != nil {
		if *c.Correct {
			line := "- " + t.correct
			if c.AcceptedWithAccents {
				line += " (" + t.accents + ")"
			}
			b.WriteString(line + "\n")
		} else {
			line := "- " + t.wrong
			if name := t.classes[c.Class]; name != "" {
				line += "; " + t.errClass + ": " + name
			}
			if c.WrongFeatureLabel != "" {
				line += "; " + t.wrongFeat + ": " + c.WrongFeatureLabel
			}
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

// CardHeadings lists the block heading in every language, for EchoAt.
func cardHeadings() []string {
	out := make([]string, 0, len(cardTexts))
	for _, t := range cardTexts {
		out = append(out, t.heading)
	}
	return out
}

func langOf(lang string) string {
	if lang == "ru" {
		return "ru"
	}
	return "en"
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// cue is what the card showed as its prompt, for whyNotMine's parenthesis:
// the meaning on a produce card, the blanked sentence on a cloze.
func cue(c CardContext) string {
	if c.Direction == "cloze" && c.HU != "" {
		return "`" + c.HU + "`"
	}
	if c.Gloss != "" {
		return "«" + c.Gloss + "»"
	}
	return "`" + or(c.Lemma, c.Form) + "`"
}

// short cuts a direction description at its dash: «узнавание — видит …» → «узнавание».
func short(s string) string {
	if i := strings.Index(s, " — "); i >= 0 {
		return s[:i]
	}
	return s
}
