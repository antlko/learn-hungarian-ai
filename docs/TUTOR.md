# The grammar tutor

How the tutor is built, what it is allowed to do, what it reads, and how to change it. Read
this before touching `internal/tutor`, the prompt, or the reference corpus.

**Decision records:** [ADR-027](../.claude/DECISIONS.md#adr-027) (the grounded tutor),
[ADR-029](../.claude/DECISIONS.md#adr-029) (a chat; the lesson found per question),
[ADR-030](../.claude/DECISIONS.md#adr-030) (translation as explanation),
[ADR-031](../.claude/DECISIONS.md#adr-031) (stop, edit, the context meter, the screen),
[ADR-034](../.claude/DECISIONS.md#adr-034) (the tutor reads the card),
[ADR-046](../.claude/DECISIONS.md#adr-046) (asking never changes a word's schedule),
[ADR-042](../.claude/DECISIONS.md#adr-042) (a chat about one word, and the dictionary entry that
is NOT a tutor route),
[ADR-045](../.claude/DECISIONS.md#adr-045) (chats by tab, cleared by the tabful; the context
meter describes the answer on screen). **Evidence:**
[R19](../.claude/research/19-grounded-tutor.md), with [R4](../.claude/research/04-explicit-instruction.md)
for why explanation on demand is worth having and [R10](../.claude/research/10-llm-tutors.md)
for why it is not trusted.

---

## 1. What it is, in one paragraph

A chat with a tutor — its own page with a new-chat button and the list of old chats, a
compact panel under every grammar lesson, and a side panel on the review card in both phases
([ADR-034](../.claude/DECISIONS.md#adr-034)) with prepared questions about that card — «разбери
предложение», «почему не `házben`?» — composed by the server from the card and answered in a
fixed shape — where the learner asks about Hungarian in their own
language: "why `-ban` and not `-ben`?", "how does this compare with Russian?", "how do I say
«I came into this village»?". A language model **explains in the interface language**, writing
the Hungarian itself in Hungarian, with a lesson in front of it: the one the chat is pinned
to (the lesson panel pins its topic; the page offers a pin as an option), or the one the app
**finds for the question** — from the Hungarian words the learner quoted, from the topic's own
words — plus the pack's feature tables, the learner's own record on those features, and up to
three sections of a reference corpus. The answer streams, names the model that wrote it, names
the lesson it was built on («по уроку: Гармония гласных»), and lists any Hungarian word in it
that the dictionary does not carry. **Nothing it says becomes a card, a gloss or a grade.**

## 2. The line it does not cross

The tutor sits on the *explaining* side of the line [ADR-014](../.claude/DECISIONS.md#adr-014)
drew for machine translation. It may put Hungarian on the screen as an *example inside an
explanation*, labelled; it may not put Hungarian into anything the learner is graded against.
Concretely, there is no code path from `tutor_message` to `note`, `card`, `user_gloss`,
`review_log`, `morph_state` or `lemma_state`, and `handlers_tutor_test.go` counts those tables
before and after a question to keep it that way. The system prompt tells the model it cannot
create cards; the panel says so to the learner.

**Translation is explanation** ([ADR-030](../.claude/DECISIONS.md#adr-030)). Asked «как
сказать …» or «напиши на венгерском …», the tutor gives the whole sentence, its literal
rendering, and then takes it apart word by word — which suffix, which harmony class, why — and
compares it with the learner's own attempt when there is one. The prompt's language rule is
about the *explanation*, never the material: v2's «Отвечай ТОЛЬКО ПО-РУССКИ», first line and
last, made a 1.7B model hand «напиши на венгерском …» back as the Russian sentence unchanged.
A translated sentence is labelled like every other Hungarian the tutor writes, dictionary-checked
word by word (a backticked span up to 200 characters is split, §6), and still becomes nothing.

The corpus-attestation guardrail of [R10](../.claude/research/10-llm-tutors.md) is therefore
not on the tutor's path — that guardrail exists for generated *material*, and the tutor
generates none. What the tutor has instead is **labelling**: the model's name on every answer,
and the dictionary check on every backticked word (§6).

## 3. Architecture

```
AiTeacherView / GrammarView ── TutorConversation.vue ── api/tutor.ts (fetch + SSE reader)
                                       │
                         POST /api/tutor/:id   {question, lang}
                                       │
                 handlers_tutor.go ─────┼── grounding: the thread's pin, or
                                       │   tutor.InferTopic(question, formFeatures, lastGrounded, index)
                                       ├── tutor.Build(Input)  → []llm.Message
                                       │        ▲
                    ┌──────────────────┘        │
                    │   Input is gathered from:  │
                    │   · the pack: grammar_topic.explain_md_{ru,en}, morph_feature rows
                    │   · the learner: srs.Mastery (accuracy, observations, ready)
                    │                  review_log.answer on cloze cards (last wrong forms)
                    │   · the corpus: tutor.KB.Retrieve(topic, question, expansions)
                    │   · the thread: tutor.Store.Messages (last 6 turns)
                    │
                    └── llm.Client.Stream ──► event: delta … event: done {message}
                                                       │
                                     tutor.Backticked / tutor.Unattested (morph.Analyzer.Lookup)
                                                       │
                                  tutor.Store.Append → tutor_message (topic_code = the grounding)
```

Packages: `internal/tutor` (corpus, retrieval, inference, prompt, attestation, store — no HTTP,
no `curriculum`), `internal/llm` (the OpenAI-compatible client, with `Stream`),
`internal/api/handlers_tutor.go` (routes, the topic index, prompt input assembly, the event
stream). The corpus is `//go:embed`ded from
`internal/tutor/kb/`; a corpus that fails to parse leaves the tutor unavailable with the reason
`errors.tutorCorpus` and affects nothing else.

## 4. The prompt

The assembly order is the design ([R19 §3], "lost in the middle"): rules first, the question
last, the authoritative lesson before the reference dump, and the whole thing short enough for
an 8k-context model. Every block has a **character budget** and is cut on a UTF-8 boundary —
never dropped — so the model always sees at least the head of the lesson.

| # | block | source | budget (chars) |
|---|---|---|---|
| 1 | system rules, in the answer language | `internal/tutor/prompts/system_{en,ru}.md` | as written (~2,400 in Russian, ~2,000 in English) |
| 2 | «УРОК ИЗ ПРИЛОЖЕНИЯ» / "THE LESSON THE APP TEACHES" | `grammar_topic.explain_md_*` | 3,600 |
| 2b | «КАРТОЧКА» / "CARD" — the review card the question is about ([ADR-034]): sentence + translation with its label, word, dictionary form, glosses, graded features with labels and allomorphs, typed vs expected, the diff's class; before the answer the answer is withheld and the block says so first | `tutor.CardContext` (`card.go`), built by `handlers_tutor.go:tutorCard` from the card, the note and the queue's own decorators | 1,600 (the reference drops to 2,000 when present) |
| 3 | features: id · label · allomorphs by harmony class · one-line note | `morph_feature` | 1,200 |
| 4 | this learner: accuracy over N answers per feature (or "N answers, too few"), last 3 wrong cloze forms | `srs.Mastery`, `review_log.answer` | 700 |
| 5 | reference: up to 3 sections, each ≤ 1,500, titled with source | `tutor.KB.Retrieve` | 3,600 |
| 6 | the thread, last 6 turns, oldest first | `tutor_message` | 3,000 |
| 7 | the question — composed by the server for a prepared template — then, for a prepared question, its shape (numbered sections, «Без проверочного вопроса»), then "Explain in {language}; everything Hungarian in Hungarian, in Latin letters, in backticks. A request to translate … is answered with the whole Hungarian sentence, then a word-by-word breakdown." — the tail is what a 4k-context Ollama keeps when it truncates (§9), so the two rules that matter most are repeated here | the request; `tutor.Compose` / `tutor.Shape` | 1,000 characters |

`tutor.PromptVersion` is stored on every answer (`tutor_message.prompt_version`) so a bad answer
can be traced to the prompt that produced it. The prompt's rationale and failure modes are in
[`.claude/prompts/tutor-system.md`](../.claude/prompts/tutor-system.md); the embedded files are
the single copy the binary sends. Change both together.

Generation parameters: `temperature 0.3`, `max_tokens 1000`, a 150-second stream timeout. The
budget is sized for a translation's word-by-word breakdown ([ADR-030]), roughly twice eight
Russian sentences; the timeout is that budget at a laptop CPU's seven tokens a second. An
answer cut off by any of the timeout, the budget (`finish_reason: length`, surfaced by
`llm.Stream` as `llm.ErrTruncated`), a stop, or a lost connection is **kept and marked**
`interrupted` and delivered as `done`, with `cutReason` naming which of the four: half an
explanation is still one, and the panel says where it stopped and why («ответ остановлен»,
«закончился отведённый объём») rather than passing it off as complete. `error` is sent only when nothing
arrived ([ADR-031] — a cut was once shown as «ничего не изменилось» under a stored partial).

**What the model saw.** Every stream asks for `stream_options.include_usage`, and the server's
`prompt_tokens` / `completion_tokens` are stored on the answer; on a `local` provider
`llm.ContextLength` then reads `GET /api/ps` — Ollama's list of running models, with the
`context_length` each is running with — and stores that too. The three figures are the
**context meter** under the composer (§6). A prompt longer than the window is cut from the
HEAD (§9): the rules and the lesson go first, and nothing in the stream says so — the meter is
how it is noticed.

## 4a. Threads, and the lesson behind a question

**A thread is a conversation** (`tutor_thread`, migration `00016`; [ADR-029]): a title from its
first question (60 runes), an optional **pinned topic**, and its own life — opened by «Новый
чат» or by the first question under a lesson, listed most-recent-first, deleted. The lesson
panel opens the learner's latest thread pinned to its topic and creates one pinned to it on the
first question; «Открыть в AI-учителе» opens that same thread on the page. A thread keeps its
last 200 messages (`tutor.ThreadKeep`).

**The log follows the answer only while the learner is at the bottom.** Scrolling up to
re-read stops the follow; coming back down, or sending, resumes it. An auto-scroll on every
delta fought the wheel and made the whole log jerk under the hands.

**A prompt echo is cut.** A small model fed a structured prompt sometimes continues the
structure instead of answering it — `gemma3:4b` wrote a fabricated «## ЭТОТ УЧЕНИК» block and
then recited «## СПРАВОЧНИК». `tutor.EchoAt` finds the first of the prompt's own block
headings in the answer, the handler cuts there and stops the stream, and the answer is stored
`interrupted` with `cutReason: echo` — «модель начала повторять служебный текст». Only the
four headings are matched; a «# Примеры» the lesson itself contains is left alone.

**The review card's questions** ([ADR-034]). `POST /api/tutor/:id {card: {cardId, phase,
template?, typed?}}`: the handler loads the card and its note, decorates them the way the
queue does (the pack's lemma and glosses, the pack's sentence translation, the cached crib —
never the sidecar), resolves the graded string through `expectedFor`, classifies what was
typed with `morph.DiffForms`, and hands the model a CARD block (§4, 2b). With a `template`
(`tutor.Templates`: `breakdown`, `whyForm`, `whyNotMine`, `translateWord`, `spelling`,
`explainTask`; before the answer `hintForm`, `hintWord`, `hintRule`, `explainTask`) the server
writes the visible question and the answer's shape; `tutor.Allowed` refuses a template
outside its phase with 400. **Before the answer the answer is withheld** — the typed card's
form, the produce card's word and sentence, the receptive card's meaning, the audio card's
Hungarian — and the block says so in its first line. **Asking costs the card nothing**
([ADR-046]): the client records no hint, sends nothing about the tutor with the answer, and
«Подсказка» remains the only thing that caps a grade. The withholding above and `tutor.Allowed`
are the whole guardrail, and the panel now says so where it used to state a price. The grounding takes the card's `targetFeats` before the question's words. The day's
review chat (`kind = review`, one per WORD per local day, keyed on `tutor_thread.note_id`;
`GET /api/tutor/review?note=` finds it without creating it) collects the questions about that
word; the AI-teacher page lists it as «Повторение · `jöttél`», named from `note.target_form` at
creation. The next word opens an empty chat — a panel that showed the previous word's answer,
and carried it into the model's window on every later question, is what the note id fixes
([ADR-034], amended). Threads created before migration `00019` have no word and keep the date.

**A third kind, `word`** ([ADR-042], migration `00021`): the chat the word inspector opens, one
per surface FORM per local day, keyed on `tutor_thread.word` and found by
`GET /api/tutor/word/:form`. Keyed on the form rather than a note because the words worth asking
about are very often the ones the pack cannot resolve at all — there is no note and sometimes no
lemma to hang it on. It is the same conversation component, so the model label, the grounding
line, the unattested warning and the honesty notice come with it rather than in a second copy.

**The three kinds are separate tabs on the AI-teacher page** ([ADR-045]), with per-kind counts,
and each tab can be cleared in one confirmed action (`DELETE /api/tutor?kind=`). A review chat is
one per word per day, so a month of reviewing had buried every real conversation under
«Повторение · …» and deleting them one at a time was the only way out. An unrecognised `kind` is
refused rather than silently meaning "all".

**The dictionary entry is NOT a tutor route** ([ADR-042]). `handlers_ai_entry.go` is a separate
handler on purpose: the tutor mints nothing and `handlers_tutor_test.go` counts the tables to
prove it, so routing an entry — which *can* reach `user_gloss`, on an explicit save, as
`machine` — through the tutor would delete that guarantee. It reuses the tutor's attestation
helpers and nothing else.

**A chat's controls** ([ADR-031]): «Стоп» replaces «Спросить» while an answer streams
(`Escape` too) — `POST /api/tutor/:id/stop` cancels the flight's context on the server, and the
partial answer arrives as `done`, marked interrupted, through the same stream. The last
question carries «Изменить вопрос»: the composer takes its text, a line says the old answer and
everything after it will go, and sending it asks with `replace: <id>` — `Store.TruncateFrom`
deletes from that turn on and re-titles a thread whose first question it was. A thread is a
conversation, never a tree of them.

**Every question is grounded, and the grounding is chosen per question.** A pinned topic wins.
Otherwise `tutor.InferTopic` scores the question against every topic's vocabulary
(`tutor.TopicIndex`, built by the handler from the DAG and the pack for each question — thirty-three
topics, cheap, always in step with the pack):

| signal | weight |
|---|---|
| a feature the analyser read on a Hungarian word in the question, taught by this topic (`házban` → `Case=Ine`) | 5 |
| a suffix shape one of its features takes, written with a hyphen (`-ban`, `-ek`) | 4 |
| a word of its title, either language | 3 |
| a word of one of its feature labels, either language | 2 |
| a word of its lesson prose (capped at 3 per topic) | 1 |
| a title/label/suffix term only ONE topic carries | +1 |

Threshold 3. Below it the thread's **last grounding is sticky** — «а ещё примеры?» stays on the
lesson the previous answer used — and with nothing at all the prompt says so under the lesson
heading (§4, block 2) and the reference is retrieved by the question alone. Deterministic, like
retrieval, and for the same reason: "why did the tutor use this lesson" must be answerable.
Words in another script are not analysed; a learner who types «ем» is answered about `-em` by
rule 2 of the prompt, but the inference sees no suffix — pinning is the override, and the label
is the mitigation.

**The grounding is printed.** `tutor_message.topic_code` on an assistant turn is the lesson
the answer was built on; the panel prints «по уроку: …» under it, or «по справочнику, без урока».
The chat list shows a free thread's last grounding as its subtitle.

## 5. The reference corpus

`internal/tutor/kb/` holds two kinds of file, both markdown with a front-matter block
(`title`, `source`, `url`, `license`, optional `topics`):

- **`src-*.md`** — generated by `langpack/pipeline/tutor_kb.py` (`make tutor-kb`) from open
  sources: Wikipedia's five Hungarian grammar articles and fourteen Wiktionary appendices
  (CC BY-SA 4.0, wikitext converted to markdown), and the "Notes on Grammar" of the US Foreign
  Service Institute's *Hungarian Basic Course* (1962, public domain; OCR text with diacritics
  restored from the pack where exactly one attested form matches). Do not edit these by hand —
  regenerate them.
- **`ref-*.md`** — fourteen reference notes written for this app, one per topic group: sounds
  and harmony; articles, `van`/`nincs`, negation, pronouns and address; nouns and stems; the
  locative cases; the dative and possession; the present tense and definiteness; the past
  tense; infinitive, potential and conditional; the imperative; the instrumental, the remaining
  cases and postpositions; comparison, adverbs and numerals; preverbs and word order;
  participles, converb, causative and frequentative; register. Each ends with a "Where speakers
  of Russian go wrong" section. Edit these by hand; they are reviewed prose.

The licence ledger is [`internal/tutor/kb/LICENSES.md`](../backend/internal/tutor/kb/LICENSES.md).
An 1882 grammar on Wikisource was evaluated and excluded for its pre-reform orthography.

**Sections.** The loader splits every file at `##`/`###`/`####` headings; a section's title is
its heading chain («Hungarian verbs › Past tense»), its topics are the file's `topics:` plus every
`<!-- topics: a, b -->` comment in the chain above it and in itself, and sections under 60
characters are dropped. The corpus currently loads as ~360 sections from 21 sources.

**Retrieval** (`retrieve.go`) is keyword-and-topic scoring, deterministic and explainable:

- `+6` for a section tagged with the open topic;
- `+8` for each distinct **whole-word** match of a token with Hungarian letters (`házban`,
  `-ből`) — a form the learner quoted is the most specific thing a question can carry;
- `+3` per query token in the section title, and a saturating term-frequency score
  (`1.5 · tf/(tf+1.5)`) per token in the body;
- the topic's English title and its features' English labels are added as **expansion terms**
  at 0.6 weight, so «объясни инессив» reaches the English section on the inessive;
- stopwords in English, Russian and the commonest Hungarian function words are ignored.

The top three sections are returned; when the words matched elsewhere, the open topic's best
section is kept in play; when nothing matched at all, the topic's sections alone are returned.
No embeddings, no vector index: a second model to deploy inside a static binary
([ADR-001](../.claude/DECISIONS.md#adr-001)) for a corpus of a few hundred sections, and it would
make "why did the tutor cite this" unanswerable.

## 6. Honesty: what the learner is told

- **Who wrote it.** `tutor_message.model` travels with every assistant turn and the panel prints
  «ответ модели qwen3:8b» under it.
- **Which words the dictionary does not know.** The model is told to backtick every Hungarian
  word. `tutor.Backticked` collects them — a backticked *sentence* up to 200 characters is split
  into its words, so a translation ([ADR-030]) is checked word by word — `tutor.Unattested` looks each one up through
  `morph.Analyzer.Lookup` — attested readings (`certain`, `uncertain`, `corrected`) count, the
  suffix-fallback `derived` reading does not — and the panel prints «Нет в словаре: házben»
  in the `uncertain` hue. Bare suffix shapes (`ban`, `nak`) are skipped; they are not words.
- **What it cannot do.** A standing note on the panel: the explanation is the model's, the
  lesson is the app's, nothing here becomes a card.
- **What the model is, when it is small.** A tag that names a size under 7B (`qwen3:1.7b`)
  draws a line under the notice: «модель qwen3:1.7b — маленькая: 1.7 млрд параметров, судя по
  названию … учитель рассчитан на 7–8 млрд». The size is read off the tag
  (`modelParamsB`, `composables/useTutorThread.ts`) and the copy says so; a tag that names no
  size shows nothing ([ADR-030]).
- **How full the window was.** Under the composer, from the latest answer: a thin bar and
  «Окно модели 8 192 токенов: последний вопрос и ответ заняли 3 850, свободно ≈ 4 342». When
  prompt plus answer reach the window, the line becomes a warning that the head of the prompt
  — the rules and the lesson — may have been cut, and names the fix (8k in Ollama). A server
  that reports no window gets the bare figures and «размер окна модель не сообщает»; one that
  reports nothing at all gets no meter. Zero is never printed as a number ([ADR-031]).
- **Which lesson it used.** «по уроку: Гармония гласных» under every answer, or «по справочнику,
  без урока» — an inferred choice the learner can see is one they can correct by pinning
  ([ADR-029](../.claude/DECISIONS.md#adr-029)).
- **When it is off.** With `services.llm` unconfigured the panel says so and links to Settings;
  with a model unnamed it says that instead. No dead panel, no silent absence
  ([ADR-005](../.claude/DECISIONS.md#adr-005)). A thread that fails to load says so and offers a
  retry — it once rendered nothing, because the error was drawn inside the block that needs a
  thread to exist. And `unattested` on the `done` event is never `null`: an answer with no
  backticked word once shipped one, the panel's render threw on it, and the learner saw a
  frozen panel until a reload read the stored `[]` back (`tutor.Unattested`, pinned by
  `TestTutorInfersTheLessonForAFreeChat`).

## 7. The learner's language

The client sends its rendered locale (`lang: ru|en`); the server accepts those two and falls
back to `locale.ui`. The system rules, the block headings and the closing "Answer in …" line are
all in that language, and the pack's feature labels are read in it too — so «инессив» on the
card, in the lesson and in the answer are the same word.

**Known asymmetry.** The reference corpus is English. A Russian-locale learner gets a Russian
answer produced from the pack's Russian lesson plus English references. Small models handle
that unevenly; the lesson is placed first partly for this reason. A Russian reference corpus
would be the natural next addition — it drops into the same directory.

## 8. Endpoints

| Method | Path | Notes |
|---|---|---|
| `GET` | `/api/tutor` | the learner's chats, most recently active first: `[{id, title, topic?, titleRu?, titleEn?, lastTopic?, lastTopicRu?, lastTopicEn?, messages, lastRole, lastContent, updatedAt}]` |
| `POST` | `/api/tutor?lang=` | `{topic?, kind?, note?}` → **201** with the thread response below; 404 on an unknown topic. `kind: "review"` + `note` → that word's chat for the day, **200** when it exists, 201 when created ([ADR-034]) |
| `GET` | `/api/tutor/review?lang=&note=` | that word's chat for the local day with its messages, or `thread: null` — never created by asking |
| `GET` | `/api/tutor/new?lang=` | a chat not started yet: availability and sources, `thread: null` |
| `GET` | `/api/tutor/topic/:code?lang=` | the latest thread pinned to a lesson, or `thread: null` with `topic` set — what the lesson panel opens; 404 on an unknown topic |
| `GET` | `/api/tutor/:id?lang=` | `{available, reason?, model?, lang, thread: {id, title, topic?, titleRu?, titleEn?, createdAt, updatedAt} \| null, topic?, messages, sources}`. 404 `errors.tutorNoThread` for another learner's thread or none; `available: false` with `reason: errors.llmOff` when the service is off, `errors.llmNoModel` when it is on but names no model |
| `POST` | `/api/tutor/:id` | `{question?, lang?, replace?, card?: {cardId, phase, template?, typed?}}` → `text/event-stream`. Events: `asked {id, question}` (the stored question — composed by the server for a template), `delta {text}` …, then `done {message}` whenever any words arrived (`message.interrupted` when the answer was cut — budget, timeout, stop, lost connection); `error {error}` only when nothing arrived (`errors.tutorFailed`, or `errors.tutorStopped` after a stop). `replace` is the id of a question of the learner's own: it and everything after it are deleted before this one is asked; 404 `errors.tutorNoMessage` otherwise. 400 blank or > 1,000 bytes; 404 no such thread; 503 `errors.llmOff` / `errors.llmNoModel` / `errors.tutorCorpus`; 409 while the learner's previous question is still streaming |
| `POST` | `/api/tutor/:id/stop` | ends the learner's in-flight answer on this thread → 204, idempotent; the stream delivers the partial as `done` |
| `DELETE` | `/api/tutor/:id` | deletes the thread and its turns → 204 |

A `message` is `{id, role, content, lang, model, promptVersion, topic?, topicTitle?, unattested,
interrupted, cutReason?, promptTokens?, answerTokens?, contextTokens?, createdAt}`; `topic` is the grounding
(§4a); the three token figures are the context meter's (§4, §6), absent when not reported. The
thread keeps its last 200 messages (`tutor.ThreadKeep`). A question asked from a review card
carries `cardId`, `cardPhase` and `template`; a thread carries `kind` (`review` for a word's
review chat). 400 for a template outside its phase, 404 `errors.tutorNoCard` for a card that
is not the learner's or is in the trash; a question is at most 1,000 **characters**.

## 9. Running a model

Any OpenAI-compatible chat endpoint works. Locally, `docker compose --profile llm up -d` starts
Ollama; then **Settings → Services → Language model**: provider `local`, endpoint
`http://ollama:11434` (or `http://localhost:11434` outside compose), and a **model** — required,
because the request names it and Ollama refuses one without. Save the endpoint first: the row
then asks the server what it serves (`GET /v1/models`) and offers the names in the Model field,
reading «укажите модель» until one is chosen and «такой модели нет на сервере» for a name it did
not list. `qwen3:8b` after `docker exec tanulom-ollama ollama pull qwen3:8b` is the documented
choice. A hosted API takes provider `openai`, its endpoint, and the key in the same screen
(stored in SQLite, never in YAML).

Two settings on the Ollama side that the app cannot make for you. **Context length: 8k or
more.** §5's budgets fit an 8k window; Ollama ships at 4k and truncates a longer prompt silently
(measured: a 16,000-character system message reported back as 2,050 prompt tokens), which drops
the lesson the answer was supposed to be grounded in. The Ollama app has a slider for it;
`OLLAMA_CONTEXT_LENGTH=8192` does the same for the bare server. **Thinking is switched off by the
app**, not by you: a thinking model (qwen3 is Ollama's headline suggestion) spends its hidden
reasoning out of the same `max_tokens` as the answer — six hundred characters of English
deliberation before the first Russian word on `qwen3:1.7b` — so a `local` provider is sent
`reasoning_effort: none` (`internal/llm`). A hosted API keeps its own default.

What to expect on a laptop CPU with a 7–8B model: the first token in a few seconds, eight
Russian sentences in thirty to sixty. The stream keeps the panel honest about that. What to
expect from a 1–2B model: fluent, labelled, and frequently wrong — the label on every answer is
the whole defence, and the page says so: a tag that names a size under 7B draws a line under the
notice with the size and the 7–8B the tutor was built for (§6). **Measured, 2026-09-03:**
`qwen3:1.7b` asked *bare* — no app prompt at all — for «Я пришёл в эту деревню и зашёл в свой
дом» wrote `Ígép a városba és bejártam a házam` with thinking off and `Én bejutottam ezt a
városba és bejutottam a saját házba` with thinking on (3,500 characters of reasoning first);
neither is a sentence anyone says. With the app's prompt v3 it printed the lesson's example
table under «Разбор:» instead of translating. A model of that size cannot do this task, and no
prompt makes it; the good answer a learner may have seen elsewhere came from a bigger model.
`qwen3:8b` is the smallest model the breakdown was designed around.

## 10. Changing things

| I want to… | Go here |
|---|---|
| Change what the model is told | `internal/tutor/prompts/system_{en,ru}.md`, bump `PromptVersion` in `prompt.go` and `version` in `.claude/prompts/tutor-system.md`. The language rule names the *explanation* and never says `ONLY` — `tutor_test.go` pins that ([ADR-030]) |
| Change the answer budget or the timeout | `tutor.MaxAnswerTokens` (`prompt.go`) and `streamTimeout` (`handlers_tutor.go`) — they are sized together: the budget at a CPU's seven tokens a second |
| Change what counts as "the window was full" | `usage` in `composables/useTutorThread.ts` (`full`: prompt + answer within 16 of the window); the figures themselves come from `llm.Stream` and `llm.ContextLength` |
| Change the chat's layout | `TutorConversation.vue` (the log, the composer, the lines under it — shared with the lesson panel) and `AiTeacherView.vue` (the rail, the header row, the height) |
| Change when the page calls a model small | `modelParamsB` in `composables/useTutorThread.ts`; the threshold (`< 7`) in `TutorConversation.vue` |
| Change the budgets or the block order | `internal/tutor/prompt.go` (`Build`); `tutor_test.go` pins the order and the cut |
| Add reference material | a new `ref-*.md` with front matter and `<!-- topics -->` comments, or a source in `langpack/pipeline/tutor_kb.py` (+ `LICENSES.md`), then `make tutor-kb` |
| Change how sections are chosen | `internal/tutor/retrieve.go`; `TestRetrieveRanksTopicAndWords` says what must keep holding |
| Change what counts as attested | `wordExists` in `handlers_tutor.go` — it is the pack's `Lookup`, and should stay it |
| Change what the endpoint is asked for, or how a model is verified | `internal/llm` (`request`, `ListModels`) and `probeModels` in `handlers_capabilities.go`; the settings row's copy keys off `detail` |
| Give the tutor more of the learner's record | `tutorInput` in `handlers_tutor.go`; keep it under `learnerBudget` |
| Change how the lesson is found for a free chat | `internal/tutor/infer.go` (weights, threshold, stickiness) and `tutorIndex` / `formFeatures` in `handlers_tutor.go`; `infer_test.go` and `TestTutorInfersTheLessonForAFreeChat` say what must keep holding |
| Put the tutor on another screen | mount `TutorConversation.vue` with `threadId` (a chat) or `topic` (the latest chat pinned there, or a new one); `TutorPanel.vue` is the lesson's card around it; `components/review/TutorAside.vue` is the review card's — `kind="review"`, `chips` (the prepared questions the view chose) and `card` (the card the request names) — ADR-034 |
| Add or change a prepared question on the review card | `internal/tutor/card.go` (`Templates`, `Allowed`, `Compose`, `Shape`, the class advice) and `tutorChips` in `ReviewView.vue` (which chips a direction × phase × outcome shows); the chip labels are `review.tutor.chips.*` in both locales; `card_test.go` pins both languages |

## 11. What it is not

Not a conversation partner ([R10] wants voice for that, and it is Phase 9); not a writing
corrector (`correct-writing.md`, prompt-then-reveal, also Phase 9); not a substitute for Loop 4
([R16 §7](../.claude/research/16-debunked.md)). And not a source of study material: the
reader and the course make cards, and the tutor explains why the cards say what they say.
