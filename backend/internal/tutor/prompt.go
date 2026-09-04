package tutor

import (
	_ "embed"
	"fmt"
	"strings"
	"unicode/utf8"

	"tanulom/internal/llm"
)

// The prompt, and the budgets that keep it inside a small model's window.
//
// [R19 §3] Position decides whether a model uses a fact: rules first, the
// question last, the authoritative lesson before the reference dump, and the
// whole thing short. Every block below has a character budget and is CUT,
// never dropped, so the model always sees at least the head of the lesson.
// Characters rather than tokens because the binary carries no tokenizer; the
// budgets are set so that a Cyrillic-heavy prompt still fits an 8k context.
//
// The system texts are embedded from prompts/, versioned by PromptVersion and
// documented in .claude/prompts/tutor-system.md. Bump both on a semantic edit.

//go:embed prompts/system_en.md
var systemEN string

//go:embed prompts/system_ru.md
var systemRU string

// PromptVersion is recorded on every stored answer, so a bad answer can be
// traced to the prompt that produced it.
//
// v2 (ADR-029): Hungarian always in Latin orthography in backticks, never
// transliterated even when the learner typed Cyrillic; two or three whole-word
// examples with a translation in every explanation; the no-lesson case
// stated to the model instead of left blank.
//
// v3 (ADR-030): the language rule names the EXPLANATION, never the material —
// «Отвечай ТОЛЬКО ПО-РУССКИ», first line and last, made a 1.7B model answer
// «напиши на венгерском …» with the Russian sentence unchanged. A translation
// request has a shape (the sentence, its literal rendering, then word by
// word, and the learner's own attempt compared) and is the one answer allowed
// past eight sentences.
//
// v4 (ADR-034): a question asked from the review card carries the CARD —
// the sentence, the word, the glosses, the graded features, what was typed
// against what was wanted — as a block right after the lesson; a prepared
// question also carries the shape its answer must take, in the tail, and
// switches the check question off. Before the answer the block withholds the
// answer and rule 9 tells the model to explain the rule and not name it.
const PromptVersion = 4

// Budgets, in characters.
const (
	lessonBudget    = 3600
	featuresBudget  = 1200
	learnerBudget   = 700
	referenceBudget = 3600
	// referenceBudgetWithCard is the reference's share when a CARD block is
	// present (ADR-034): the card is the strongest grounding there is, and
	// the total must not grow past what an 8k window holds in Cyrillic.
	referenceBudgetWithCard = 2000
	// cardBudget holds a full block in Cyrillic — the labels alone are a
	// third of it — with the withheld line first so a cut can never drop it.
	cardBudget    = 1600
	perSectionCap = 1500
	historyBudget = 3000
	historyTurns  = 6
	// QuestionLimit bounds what the client may send, in CHARACTERS — the
	// composer's maxlength — and the handler refuses more. It was bytes once,
	// and a Russian question of eight hundred letters was refused as too long.
	QuestionLimit = 1000
	// MaxAnswerTokens bounds the answer. Eight sentences is under 400 tokens
	// in English and under 700 in Russian; a translation with its word-by-word
	// breakdown (ADR-030) is about twice that — the answer the first learner
	// brought as the standard ran to 1,900 characters, which Cyrillic-heavy
	// text puts near 900 tokens. The ceiling leaves room for that without
	// letting a runaway model fill the screen, and a cut is marked
	// (llm.ErrTruncated → Message.Interrupted) rather than passed off as
	// the whole answer.
	MaxAnswerTokens = 1000
	// Temperature: low, for explanation rather than invention.
	Temperature = 0.3
)

// Feature is one atom the open topic teaches, from the pack.
type Feature struct {
	ID         string
	Label      string
	Explain    string
	Allomorphs map[string]string
}

// LearnerFeature is what the learner's own record says about one feature.
type LearnerFeature struct {
	Label        string
	Accuracy     float64
	Observations int
	// Ready is the gate's own verdict on whether the figure rests on enough
	// answers to be worth stating (drill.gateWindow). Below it, the count is
	// stated instead of the percentage — "3 answers" is honest, "67%" is not.
	Ready bool
}

// Mistake is a wrong typed answer from the review log: what was typed, what
// was wanted.
type Mistake struct {
	Typed    string
	Expected string
}

// Input is everything the prompt is assembled from.
type Input struct {
	// Lang is the answer language, "ru" or "en".
	Lang       string
	TopicCode  string
	TopicTitle string
	// Lesson is the pack's explain_md for the topic, in Lang.
	Lesson    string
	Features  []Feature
	Learner   []LearnerFeature
	Mistakes  []Mistake
	Reference []Hit
	// History is the thread so far, oldest first; only the last historyTurns
	// are sent.
	History  []llm.Message
	Question string
	// Card is the review card the question is about, or nil (ADR-034).
	Card *CardContext
	// Shape is the answer's shape for a prepared question — numbered
	// sections and the line that switches the check question off — written
	// after the question, at the very end.
	Shape string
}

// labels are the block headings, in the answer language — a model follows a
// heading it can read better than one it cannot.
type labels struct {
	lesson, noLesson, features, learner, reference, answerIn, notMeasured, accuracy, mistakes, typedFor, allomorphs string
}

var labelsByLang = map[string]labels{
	"en": {
		lesson:      "THE LESSON THE APP TEACHES",
		noLesson:    "No lesson is open: the question is outside any course topic, or the app could not tell which one. Answer from the REFERENCE sections and the general rules; name which part of the grammar the question belongs to, and say plainly where you are not sure.",
		features:    "FEATURES (id · name · suffix variants by harmony class · note)",
		learner:     "THIS LEARNER",
		reference:   "REFERENCE",
		answerIn:    "Explain in English; everything Hungarian in Hungarian, in Latin letters, in backticks. A request to translate or to write something in Hungarian is answered with the whole Hungarian sentence, then a word-by-word breakdown.",
		notMeasured: "%s: %d answers so far, too few to judge",
		accuracy:    "%s: right %d%% of the time over %d answers",
		mistakes:    "Recent wrong answers on this topic:",
		typedFor:    "typed `%s` where `%s` was expected",
		allomorphs:  "back / front unrounded / front rounded",
	},
	"ru": {
		lesson:      "УРОК ИЗ ПРИЛОЖЕНИЯ",
		noLesson:    "Открытого урока нет: вопрос вне тем курса, или приложение не смогло понять, к какой теме он относится. Отвечай по разделам «СПРАВОЧНИК» и общим правилам; назови, к какому разделу грамматики относится вопрос, и прямо говори, где не уверен.",
		features:    "ПРИЗНАКИ (id · название · варианты суффикса по классу гармонии · пояснение)",
		learner:     "ЭТОТ УЧЕНИК",
		reference:   "СПРАВОЧНИК",
		answerIn:    "Объясняй по-русски; всё венгерское — по-венгерски, латиницей, в обратных кавычках. Просьба перевести или написать по-венгерски — это венгерское предложение целиком, а затем разбор по словам.",
		notMeasured: "%s: пока %d ответов, судить рано",
		accuracy:    "%s: верно в %d%% случаев за %d ответов",
		mistakes:    "Недавние ошибки по этой теме:",
		typedFor:    "написал `%s`, ожидалось `%s`",
		allomorphs:  "задние / передние неогублённые / передние огублённые",
	},
}

// EchoAt reports where an answer starts reciting the prompt's own block
// headings — «## ПРИЗНАКИ», «## СПРАВОЧНИК» and the rest — or -1. A small
// model fed a structured prompt sometimes continues the structure instead of
// answering it (gemma3:4b did, with a fabricated «## ЭТОТ УЧЕНИК» block);
// nothing after that point is an answer, and the scaffolding is the app's,
// not something a learner should read. The handler cuts there and says so
// (CutEcho). Only headings that cannot occur in a real answer are matched:
// a «# Примеры» from the lesson's own prose is left alone.
func EchoAt(answer string) int {
	at := -1
	headings := cardHeadings()
	for _, l := range labelsByLang {
		headings = append(headings, l.lesson, l.features, l.learner, l.reference)
	}
	for _, h := range headings {
		if i := strings.Index(answer, "## "+h); i >= 0 && (at < 0 || i < at) {
			at = i
		}
	}
	return at
}

// Build assembles the messages for one question.
func Build(in Input) []llm.Message {
	lang := in.Lang
	if lang != "ru" {
		lang = "en"
	}
	l := labelsByLang[lang]
	system := systemEN
	if lang == "ru" {
		system = systemRU
	}

	var b strings.Builder
	b.WriteString(strings.TrimSpace(system))
	b.WriteString("\n\n")

	// 2. The lesson — the app's authority, right after the rules. A free
	// chat whose question matched no topic (ADR-029) gets the fact stated
	// under the same heading, so the model is told rather than left to
	// guess whether something is missing.
	if strings.TrimSpace(in.Lesson) == "" && in.TopicCode == "" {
		b.WriteString("## " + l.lesson + "\n\n" + l.noLesson + "\n\n")
	} else {
		b.WriteString("## " + l.lesson + ": " + in.TopicTitle + "\n\n")
		b.WriteString(cut(strings.TrimSpace(in.Lesson), lessonBudget))
		b.WriteString("\n\n")
	}

	// 2b. The card the question is about (ADR-034), right after the lesson:
	// the facts the answer must rest on, and the strongest grounding there is.
	if in.Card != nil {
		b.WriteString(cut(in.Card.block(lang), cardBudget))
		b.WriteString("\n")
	}

	// 3. The features, as a compact table the model can quote from.
	if len(in.Features) > 0 {
		var f strings.Builder
		f.WriteString("## " + l.features + "\n")
		f.WriteString("(" + l.allomorphs + ")\n")
		for _, feat := range in.Features {
			f.WriteString("- " + feat.ID + " · " + feat.Label)
			if v := allomorphLine(feat.Allomorphs); v != "" {
				f.WriteString(" · " + v)
			}
			if feat.Explain != "" {
				f.WriteString(" · " + oneLine(feat.Explain))
			}
			f.WriteString("\n")
		}
		b.WriteString(cut(f.String(), featuresBudget))
		b.WriteString("\n")
	}

	// 4. The learner: measured state, in the gate's own terms.
	if len(in.Learner) > 0 || len(in.Mistakes) > 0 {
		var f strings.Builder
		f.WriteString("## " + l.learner + "\n")
		for _, lf := range in.Learner {
			if lf.Ready {
				fmt.Fprintf(&f, "- "+l.accuracy+"\n", lf.Label, int(lf.Accuracy*100+0.5), lf.Observations)
			} else {
				fmt.Fprintf(&f, "- "+l.notMeasured+"\n", lf.Label, lf.Observations)
			}
		}
		if len(in.Mistakes) > 0 {
			f.WriteString(l.mistakes + "\n")
			for _, m := range in.Mistakes {
				fmt.Fprintf(&f, "- "+l.typedFor+"\n", m.Typed, m.Expected)
			}
		}
		b.WriteString(cut(f.String(), learnerBudget))
		b.WriteString("\n")
	}

	// 5. The reference: the best few sections, each capped, the whole capped —
	// tighter when a card is present, so the total does not grow.
	if len(in.Reference) > 0 {
		refBudget := referenceBudget
		if in.Card != nil {
			refBudget = referenceBudgetWithCard
		}
		var f strings.Builder
		f.WriteString("## " + l.reference + "\n")
		for _, h := range in.Reference {
			f.WriteString("### " + h.Title + " (" + h.Source + ")\n")
			f.WriteString(cut(h.Body, perSectionCap))
			f.WriteString("\n\n")
		}
		b.WriteString(cut(f.String(), refBudget))
		b.WriteString("\n")
	}

	msgs := []llm.Message{{Role: "system", Content: strings.TrimSpace(b.String())}}

	// 6. The thread, most recent turns, oldest first, under budget.
	hist := in.History
	if len(hist) > historyTurns {
		hist = hist[len(hist)-historyTurns:]
	}
	used := 0
	kept := make([]llm.Message, 0, len(hist))
	for i := len(hist) - 1; i >= 0; i-- {
		m := hist[i]
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		content := cut(m.Content, historyBudget-used)
		if content == "" {
			break
		}
		used += len(content)
		kept = append([]llm.Message{{Role: m.Role, Content: content}}, kept...)
		if used >= historyBudget {
			break
		}
	}
	msgs = append(msgs, kept...)

	// 7. The question, then the language — of the explanation, never of the
	// material — and the shape of a translation answer (ADR-030), at the very
	// end where a small model reads most reliably. The tail is also what
	// survives when a 4k-context Ollama truncates the prompt from the head
	// (docs/TUTOR.md §9), so the two rules that matter most are stated here
	// as well as in the system text.
	// QuestionLimit is characters; the cut is in bytes, so allow four per rune.
	q := cut(strings.TrimSpace(in.Question), QuestionLimit*4)
	if shape := strings.TrimSpace(in.Shape); shape != "" {
		q += "\n\n" + shape
	}
	msgs = append(msgs, llm.Message{Role: "user", Content: q + "\n\n" + l.answerIn})
	return msgs
}

// cut trims s to at most n bytes on a UTF-8 boundary, preferring a line
// break near the end so a table is not sliced mid-row.
func cut(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	i := n
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	if nl := strings.LastIndex(s[:i], "\n"); nl > i*3/4 {
		i = nl
	}
	return strings.TrimRight(s[:i], " \n") + " …"
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	return cut(s, 160)
}

// allomorphLine renders {back, front_unrounded, front_rounded} as
// `-ban / -ben / -ben`, or nothing when the feature has no exponent.
func allomorphLine(a map[string]string) string {
	back, fu, fr := a["back"], a["front_unrounded"], a["front_rounded"]
	if back == "" && fu == "" && fr == "" {
		return ""
	}
	dash := func(s string) string {
		if s == "" {
			return "∅"
		}
		return "-" + s
	}
	return dash(back) + " / " + dash(fu) + " / " + dash(fr)
}
