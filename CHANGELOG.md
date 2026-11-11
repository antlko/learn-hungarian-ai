# Changelog

All notable changes to tanulom. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
the project uses [semantic versioning](https://semver.org/). Every design decision behind an entry
is an ADR in [`.claude/DECISIONS.md`](.claude/DECISIONS.md), and every pedagogical claim traces to a
note in [`.claude/research/`](.claude/research/README.md).

## [Unreleased] — polish pass

Answers to the owner's sixteen points: install clarity, mobile, how the app learns, statistics,
the roadmap and a plain list of features. Nothing committed or released.

- **Removed** the optional HuSpaCy context sidecar (`nlp`, ADR-069): no measured benefit, ~2.4 GB RAM. Uncertain words keep the frequency prior and the "uncertain" mark.

### Learner-visible
- **Estimated level and text coverage** (ADR-047): a chip in the top bar («A2.2», «≈71% слов текста»),
  a card on Today, and a section in Progress, always with its method sentence; it can go down.
- **The course grows to three finite tiers** (ADR-057): 24 basic + 20 intermediate + 20 advanced
  core units (about 3 100 words), picked at your own pace; "month N" is gone from the screen. A
  unit now shows how many words you have **learned**, not only how many are in the deck.
- **Today is a roadmap** (ADR-048): level, one next action, today's obligations, words from
  reading, and the course units with checkpoints. The Plan screen is gone; `/plan` redirects.
- **"Next" follows enrolment**: mining a word from a text no longer switches the next unit.
- **Phone layout**: bottom tabs, single-column roadmap, touch-sized review, a row of Hungarian
  letters above the keyboard.
- **«Как это работает»** explains learning steps and that there is no "N correct in a row" rule
  (ADR-051). The scheduler is unchanged.
- **Words shows a stage ladder** (учится, вспоминаем, повторяется, закреплено, знаю) with filters.
- **Grammar** is a path of topics with plain phase names, Lesson | Exercises tabs and the gate
  in numbers (ADR-053).
- **Statistics redesigned**: activity calendar, level and coverage, clearer lemma curve and heatmap.
- **Account menu** holds language, theme, service status and sign-out; the left menu is dynamic.
- **Settings → Components** (ADR-052): what each optional piece is and costs, state and progress.
- **Library reading sets** (ADR-056): 36 ready-made sets of real sentences, imported on a click.

### Content
- **Course**: core units grew to about 1,400 words; 12 optional theme units (ADR-050).
- **Grammar**: five new topics (questions, numerals, pronouns, postpositions, time and date);
  33 topics, 13 with exercises (ADR-055).
- **Starter dictionary** is now 5,725 lemmas, 587,898 forms, 12,858 sentences.

### Install and operations
- **Install the full dictionary from Settings** (ADR-049): checksum-pinned, id-compatibility
  checked, applied on restart. No pack is published yet, so the button says so.
- **In-app Restart** (needs a restart policy; `TANULOM_SUPERVISED`).
- **HuSpaCy sidecar** (`nlp` profile) built, opt-in, selects only among attested readings (ADR-054);
  not run in CI, memory figure is the published one.
- **Docs**: README rewritten around what works today (build from source), docs/FEATURES.md added,
  the 40-minute pack question answered, the pack numbers reconciled.

### Performance
- Locale files load lazily and vendor code is a separate chunk; entry bundle 94 KB to 34 KB.

## [Unreleased] — earlier

The first month of real use, answered. Fourteen reports from the learner using it daily.

### Added
- **Full profile export and import** (ADR-040, superseding ADR-011's "no export" in the shape
  ADR-011 itself called honest). A zip carrying every user-scoped table — both FSRS state
  spaces, the review log, texts and their analyses, glosses, the daily ledger, tutor chats —
  plus your own audio. Columns are *discovered*, so a migration cannot silently drop one; a
  coverage test fails when a new table is neither exported nor excluded with a reason; and the
  import verifies itself by replaying every card's log against the state it arrived with.
  Secrets are left out and both screens say so. Still never an `.apkg`.
- **The card says what «~10 мин» is a step towards** (ADR-039 extended). A four-rung ladder and
  «ещё один верный ответ — и слово уйдёт на дни», from the scheduler's own `RemainingSteps` and
  `PreviewNext` — not a «N correct» threshold, because FSRS has none.
- **An activity chart** (ADR-044): minutes per day split into cards, reading and listening, with
  the daily target as a reference line, a met/unmet status row, a table view and 30/90/365
  ranges. Mounted on Today and on Progress, which previously loaded the rows and discarded them.
- **Reading counts toward the day** (ADR-044). It was the one loop reporting nothing, so an hour
  with a text left the day unmet. Clamped, and counted only while the tab is visible and the
  learner is interacting.
- **The streak explains itself** (ADR-044): the week Mon→Sun, and both halves of today's
  obligation stated separately. No freeze, no repair — those are a point economy.
- **A dictionary entry from the model** for a word the pack lacks (ADR-042), labelled, checked
  word by word against the dictionary, and never material. Plus a chat about one word, one per
  form per day.
- **Tutor chat tabs and bulk delete** (ADR-045): «Чаты» / «Повторение» / «Слова» with counts, and
  a confirmed clear-this-tab. A review chat is one per word per day, so a month of reviewing had
  buried every real conversation.
- **A start screen for the ear trainer**: what it trains, why the voices change, and what it
  cannot promise — before anything plays.

### Changed
- **«В корзину» is now «Знаю»** (ADR-041), and it marks the word known as well as taking it out
  of review. Restore reverts both halves. The machinery is ADR-025's, unchanged.
- **Grammar leads with the exercise on a return visit** (ADR-043). The first visit still opens on
  the lesson — [R4] is not traded away — but a topic with answered drills no longer makes you
  scroll past the whole explanation. «Все темы» became a visible button instead of a ghost one.
- The review start screen's set count is computed over the list the session will actually serve,
  and "new" is split into today's and the backlog. «2 подх. по 30» beside «+119 новых» described
  a session that would not run.
- The plan no longer paints a month as started because the reader happened to add its words: the
  dot follows enrolment, and the row says «из них N уже были в колоде».
- **Asking the AI tutor no longer costs the card anything** (ADR-046, reversing the hint half of
  ADR-034). A question before the answer used to set `usedHint` and cap the grade at Hard, so
  reading an explanation shortened the word's interval. The guardrail that made the cap
  unnecessary was there all along — the model is not given the answer in that phase and the
  answer-naming templates are refused — and the panel now says that instead of stating a price.
  «Подсказка» still caps, because it shows the graded string.

### Fixed
- **A meaning kept from the AI entry vanished on the next click.** The review card's word
  inspector never read `user_gloss`; only the reader's rail did. Both now render it through one
  `SavedGloss` component, with a badge naming whose line it is («ваш словарь» /
  «машинное · <model>»), and the AI entry is not re-offered over a meaning already kept.
- The tutor's context meter no longer freezes. It read the last answer that *had* figures, so one
  answer without usage left it showing the first answer's numbers forever; it now describes the
  answer on screen or says it cannot (ADR-045).
- «Слух» no longer starts playing audio on arrival — and no longer leaves a learner answering a
  2AFC on silence when the browser refuses an un-gestured play.
- `review_log.id` collided on import into a database that already had review logs; rowid aliases
  are now discovered and re-assigned rather than carried.
- The week strip started on Sunday because Vue casts an absent Boolean prop to `false`.
- «Нажмите 🔊» rendered as a blank gap: the app bundles font subsets and loads nothing from the
  network, so emoji have no glyph.
- The model's dictionary entry rendered as raw fenced JSON when the model wrapped its values in
  backticks — ``"lemma": `"feltehetek"``, which no parser takes. The entry is decoded again with
  the ticks removed (they are stripped from every field anyway), the prompt says where they
  belong, and the prose fallback no longer shows the ``` fence.
- The prepared questions in the word panel («Дай ещё три примера…», «Разбери … по частям») did
  nothing when clicked: they are chips without a review card, and the handler returned early on
  exactly that. They are now sent as the questions their labels already are.
- A one-letter word was announced rather than read: the definite article `A` at the head of a
  sentence came out of the voice as «nagy A» — "capital A". One-letter utterances are spoken
  lower-case.

## [1.0.0] — unreleased

The first release: a self-hosted Hungarian trainer that tracks **two memories** — the words a
learner knows and the grammatical features they command — and derives everything else from them.

### Learning core
- FSRS-6 scheduling over two independent state spaces, lemmas and morphological features; one
  answer grades one card and only the features the item required (ADR-003).
- The review queue: due and new counted the way the queue serves them, sets of a per-device
  size, sibling burial across the day, one sentence never twice in a row, a start screen
  where the pile, the set size and the order are chosen (ADR-016, 021, 026, 028).
- A card shows its own memory — retrievability, interval, stability, the tally, what each
  grade would schedule — measured or computed by the library, never a score.
- Undo by delete-and-replay over the review log, bit for bit, fuzz included (ADR-020).
- Free practice that schedules nothing: rounds of a chosen size, an explicit weak-words mode
  (ADR-019, ADR-024). The trash sets a word aside, practised but never scheduled (ADR-025).
- The interleaving gate: blocked practice until rolling accuracy reaches 80 %, then
  interleaved, with hysteresis — and the rule it applied is printed on the screen.

### Material
- A language pack built from Wiktionary, Tatoeba, OpenSubtitles and frequency data, shipped
  inside the binary as a **starter edition** and installed on first boot; the full pack is an
  upgrade you build yourself (ADR-002, ADR-010). Every drill answer is attested in it.
- The reader: paste anything, see two coverage numbers, mine `n+1` words into cards with the
  sentence they came from; per-text review scope (ADR-022).
- A guided 24-unit course that ends and says so, then a catalogue of real Hungarian.
- Twenty-eight grammar topics, each a lesson before a drill; thirteen produce drills, the
  rest say «explained» and mean it.
- The ear trainer: high-variability phonetic training over thirty real-word minimal pairs,
  three CC0 voices, never marked complete (ADR-033).

### The tutor and the translator
- A grammar tutor on a local model: grounded in the pack's lesson, the feature tables, the
  learner's own mastery figures and a CC BY-SA reference corpus; it explains and mints nothing
  (ADR-027, ADR-029). It translates into Hungarian on request, word by word (ADR-030); it has
  a chat's controls — stop, edit-and-resend, a context meter (ADR-031); and it reads the review
  card: prepared questions composed by the server, and a question before the answer counts as
  a hint (ADR-034).
- Sentence translation from a dedicated NLLB model, display-only and always labelled with the
  model that wrote it; a translation in the wrong language says so, and the translator is
  offered wherever the learner's language is missing (ADR-014, 018, 035).

### The app
- One static Go binary with the Vue SPA embedded, one data directory, SQLite, no runtime
  dependencies; optional sidecars for NLP, TTS/ASR, translation and the LLM, and the app is
  fully usable with all of them stopped (ADR-001, ADR-006).
- Speech from the learner's own device by default (ADR-012); accounts with roles; Russian and
  English interfaces, both complete, always (ADR-004).
- Two typefaces — Literata for the material and the headings, Inter for the instrument — and
  five folk-embroidery line drawings on the quiet screens (ADR-036, ADR-037).
- Every failure in one voice, a shell that does not flash, docs that describe what ships, and
  the checks CI runs runnable with `make test` (ADR-038).

### Honest limits, stated in the product
- Per-user FSRS optimisation is absent; the efficiency figure is a simulation.
- Coverage thresholds are English-derived; Hungarian ASR runs at ~11 % WER; three voices only.
- Offline review is planned, not shipped. There is no export: the data directory is yours
  (ADR-011).

[1.0.0]: https://github.com/antlko/learn-hungarian-ai/releases/tag/v1.0.0
