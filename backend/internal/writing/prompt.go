package writing

import (
	"fmt"
	"strings"
)

// PromptVersion is the version of the answer format ("TANULOM-CHECK n") the
// prompt asks for. It is stored with every check.
const PromptVersion = 1

// Params are the inputs of Build.
type Params struct {
	// Lang is the learner's locale: "ru" or anything else for English.
	Lang string
	// Level is the learner's CEFR level (empty means A2).
	Level string
	// TopicTitle and TopicBrief are in Lang; both empty means a free topic.
	TopicTitle, TopicBrief string
	// MinWords and MaxWords are the suggested length; omitted unless both > 0.
	MinWords, MaxWords int
	// Text is the learner's essay.
	Text string
}

// promptTemplate uses @@@ for a code fence so the source stays a raw string.
const promptTemplate = `You are an experienced teacher of Hungarian as a foreign language. Below is a text written by a learner. Review it the way a careful teacher marks a written assignment.

ABOUT THE LEARNER AND THE TASK
- The learner works at CEFR level {{LEVEL}}.
- Task: {{TOPIC}}
{{LENGTH}}- The learner's own language is {{EXPLAIN_LANG}}.

WHAT TO DO
1. Find every objective error: spelling and accent marks (vowel length), vowel harmony, case endings, possessive endings, definite versus indefinite conjugation, agreement, verb forms, preverbs, word order, articles, wrong word choice, missing or extra words, punctuation.
2. Correct each error with the smallest change that fixes it. Keep the learner's words and ideas. Do not rewrite correct sentences to make them more elegant. Something correct but unnatural may be reported with TYPE: style, at most three times.
3. For each error explain, for a learner at this level: why it is wrong, what the right form is, why it is right, and which grammar rule applies. Be detailed: this explanation is the point of the exercise.
4. Report every error in its own [ERROR] block, in the order the errors appear in the text.
5. Write HINT, WHY, RULE, SUMMARY, STRENGTH and VOCAB notes in {{EXPLAIN_LANG}}. Hungarian words, QUOTE, FIX, EXAMPLE sentences and the corrected text stay in Hungarian.

ANSWER FORMAT
A program reads your answer, so follow this format exactly. Put the whole answer inside one code block that starts with @@@text and ends with @@@. Write nothing before or after the code block. Field names stay in English capitals, each at the start of its own line, followed by a colon. A value may continue onto the next lines.

TANULOM-CHECK 1
LEVEL: <the CEFR level this text shows: A1, A2, B1, B2, C1 or C2>
SUMMARY: <two to four sentences: the overall impression and the most important thing to work on>
STRENGTH: <one thing done well; repeat this line for each, at most three>

[ERROR]
QUOTE: <the wrong words copied character for character from the learner's text, one to six words, as short as possible>
NTH: <which occurrence of QUOTE in the text this is; 1 for the first>
TYPE: <one of: spelling, vowel_length, harmony, case, possessive, definiteness, agreement, verb_form, preverb, word_order, article, word_choice, missing_word, extra_word, punctuation, style>
FIX: <the corrected words that replace QUOTE>
HINT: <a short question that points at the problem without giving the answer>
WHY: <why QUOTE is wrong here and why FIX is right>
RULE: <the name of the rule, then the rule itself in one or two sentences>
EXAMPLE: <one more correct Hungarian sentence that uses the same rule> — <its translation into {{EXPLAIN_LANG}}>

For a missing word, QUOTE is the word just before the gap and FIX is that word followed by the missing one. For an extra word, FIX is QUOTE without it.

[VOCAB]
<one line per note: a more natural or more precise word — why>

[CORRECTED]
<the whole text with all corrections applied and nothing else changed>

[END]

If the text has no errors, write no [ERROR] block and say so in SUMMARY.

AN EXAMPLE OF THE FORMAT (for a different, shorter text; explanations in Russian)
@@@text
TANULOM-CHECK 1
LEVEL: A1
SUMMARY: Текст понятный. Главное — окончания: направление «куда» и прямое дополнение.
STRENGTH: Правильный порядок слов с «tegnap» в начале.

[ERROR]
QUOTE: boltban mentem
NTH: 1
TYPE: case
FIX: boltba mentem
HINT: Глагол «menni» отвечает на вопрос «где?» или «куда?»?
WHY: «-ban» значит «в чём» (место). С глаголом движения нужно «куда» — «-ba».
RULE: Иллатив -ba/-be: движение внутрь. Инессив -ban/-ben: нахождение внутри.
EXAMPLE: A parkba megyek. — Я иду в парк.

[ERROR]
QUOTE: egy kenyér
NTH: 1
TYPE: case
FIX: egy kenyeret
HINT: Что ты купил? Какое окончание у прямого дополнения?
WHY: «kenyér» — прямое дополнение глагола «vettem», ему нужен аккузатив -t; в этом слове основа меняется: kenyér → kenyeret.
RULE: Аккузатив -t отмечает прямое дополнение; у части слов долгое é в основе сокращается.
EXAMPLE: Levelet írok. — Я пишу письмо.

[CORRECTED]
Tegnap a boltba mentem. Vettem egy kenyeret és tejet.

[END]
@@@

THE LEARNER'S TEXT
<<<
{{TEXT}}
>>>
`

// Build returns the full prompt for a teacher to check Params.Text. The
// instructions are English; only the explanation language follows Params.Lang.
func Build(p Params) string {
	lang := "English"
	if strings.EqualFold(strings.TrimSpace(p.Lang), "ru") {
		lang = "Russian"
	}
	level := strings.TrimSpace(p.Level)
	if level == "" {
		level = "A2"
	}
	topic := "free topic chosen by the learner."
	if title, brief := strings.TrimSpace(p.TopicTitle), strings.TrimSpace(p.TopicBrief); title != "" || brief != "" {
		switch {
		case title == "":
			topic = brief
		case brief == "":
			topic = title
		default:
			topic = title + ". " + brief
		}
	}
	length := ""
	if p.MinWords > 0 && p.MaxWords > 0 {
		length = fmt.Sprintf("- Suggested length: %d–%d words.\n", p.MinWords, p.MaxWords)
	}
	text := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(p.Text, "\r\n", "\n"), "\r", "\n"))
	// The text goes in last so a {{…}} inside the essay is never substituted.
	out := strings.NewReplacer(
		"{{LEVEL}}", level,
		"{{TOPIC}}", topic,
		"{{LENGTH}}", length,
		"{{EXPLAIN_LANG}}", lang,
		"@@@", "```",
	).Replace(promptTemplate)
	return strings.Replace(out, "{{TEXT}}", text, 1)
}
