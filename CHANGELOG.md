# Changelog

All notable changes to tanulom. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
the project uses [semantic versioning](https://semver.org/). Every design decision behind an entry
is an ADR in [`.claude/DECISIONS.md`](.claude/DECISIONS.md), and every pedagogical claim traces to a
note in [`.claude/research/`](.claude/research/README.md).

## [1.5.2] — 2026-11-18 — Fix: flaky tests of the Piper download path broke CI

Tests only; the application behaves exactly as in 1.5.1 (same binary logic, same files, same
manifests, so installs from the `v1.5.2` tag fetch the same pinned bytes).

### Fixed
- **CI failure on `v1.5.1`**: `TestContextCancelAbortsAndLeavesNoPartial` (download cancellation)
  reported "the download never started" after 5 s on a loaded runner. Its "download started"
  signal was a non-blocking send on an unbuffered channel: when the HTTP handler ran before the
  test reached its receive, the signal was dropped for good. The channel is now buffered, the
  wait is 30 s, and if the download ends early the test now reports the error instead of a bare
  timeout. I could not reproduce the failure on a quiet or loaded development machine, so this is
  the most plausible cause, not a proven one; the new diagnostic will name any other cause.
- **Two more timing races found while stress-testing under CPU load** (12 busy loops, `-race`):
  `TestStopDuringTheDownloadCancelsWithoutAnErrorOrPartialFiles` announced a size that differed
  from the pinned size, so the loader sometimes rejected the file as corrupt before the test's
  Stop arrived (it now announces the real size and then stalls); and
  `TestWhileTheVoicesLoadTheCardKeepsShowingTheVoicesStep` checked the child's endpoint the instant
  the start was counted, a moment before it is recorded (it now waits for it).
- After the fixes, 60 stress runs of the Piper packages passed with no failure.

### Known, not changed
- `TestCrashRestartsWithBackoffThenFails` (the translator `mt` supervisor, present since v1.4.0)
  can fail when the machine is heavily overloaded; it did not fail on CI and is out of scope here.

## [1.5.1] — 2026-11-18 — Smaller images: Piper is downloaded once from the project repository

v1.5.0 carried the Piper runtime and the voices inside the app, which made the binary 326 MB and the
Docker image 555 MB. That was too big. They are now fetched once, on demand, from the project's own
repository ([ADR-081](.claude/DECISIONS.md), amends ADR-080). Reading, the voices and the audio bar are unchanged.

### Changed
- **Size:** the binary goes from 326 MB to **46 MB** and the `full` Docker image from 555 MB to **195 MB**;
  no Piper bytes are inside either.
- **Settings → Components → Speech:** «Установить» downloads the runtime and the three voices **once**,
  about **277 MB**, from the project's public repository on GitHub, unpacks them (≈ 430 MB on disk) and
  starts Piper (≈ 70 s end to end in our measurement). Every file is checked against hashes built into the
  app; a file that does not match is refused. Afterwards starting, stopping and booting use no network.
- **Buttons:** «Установить» (first time), «Выключить» / «Включить» (afterwards, offline), «Удалить»
  (removes the runtime, the voices and the download cache). A failed download keeps what it fetched and the
  next press resumes; «Остановить» during the install cancels it.
- **Operators:** new `--tts-base-url` / `TANULOM_TTS_BASE_URL` for a mirror with the same layout (https, or
  http to localhost); `--tts-runtime-dir` and `--voices-dir` now mean "use this local tree, do not download".
  This is the way to install on a machine that cannot reach GitHub.

### Removed
- The embedded runtime and voices: the `ttsbundle` build tag, `make tts-stage`, `TTS_BUNDLE` and
  `scripts/tts_stage.sh`. `make tts-smoke` now tests the real install through a local mirror with the network
  cut, then Enable offline, then Uninstall.

### Requirements and limits
- The first install needs GitHub (`raw.githubusercontent.com`) reachable, or a mirror, or a local tree.
  Speech works offline after that. The files live in the repository (not on PyPI), pinned by hash.
- Supported: macOS 14+ on Apple silicon, Linux amd64 (glibc 2.28+), Linux arm64, in the `full` image. Not
  supported: macOS Intel, Windows, the `slim` image (the browser voice is used). About 300 MB of RAM while on.
- Use an absolute `--data-dir`; a relative one breaks unpacking.
- A tagged release fetches from its own tag. A dev or untagged build fetches from `main` and may report a
  corrupt bundle if `main` has moved on.
- Licences: Piper and espeak-ng are GPL-3.0 and no longer inside the binary or image. The runtime the app
  downloads is the vendored object code in this repository, and the corresponding source stays next to it
  in `services/tts/vendor/source/`; keep it there. Anna and Berta are fine-tuned from `hifi_tts_92`, whose
  licence is still unconfirmed (`langpack/LICENSES.md`).
- **Not verified:** the Linux amd64 bundle was not executed on a real host (arm64 only); the
  sentence-by-sentence playback with real audio was not heard end to end in an automated browser, so
  listen to a text once.

### Upgrade notes
- From 1.5.0: nothing migrates. Open Settings → Components and press «Установить» (a 1.5.0 install had
  nothing to keep). The repository history carries the runtime and voices (≈ 560 MB), so a fresh clone is large.
- Docker: the optional compose `tts` profile now downloads on its first start into its own volume;
  installing from Settings inside the main container does the same and is simpler.

## [1.5.0] — 2026-11-18 — Hungarian voices in the reader, offline

*Superseded by 1.5.1: 1.5.0 shipped the runtime and voices embedded (326 MB binary, 555 MB image); 1.5.1 downloads them once instead, for size.*

The reader can now read any text aloud with natural Hungarian voices that run on your own
computer, and the separate Listen tab is folded into the reader. Piper and the three voices ship
**inside the app**: turning speech on never touches the network, so it keeps working even if the
upstream projects disappear ([ADR-079](.claude/DECISIONS.md), [ADR-080](.claude/DECISIONS.md)).

### Learner-visible
- **Listen to any text.** The reader has an audio bar: play the whole text sentence by sentence
  from the sentence you selected, pause, resume, stop, speed 0.5–1.5×, the current sentence is
  highlighted and scrolled into view. The next sentences are prepared ahead, so there is no gap.
- **Choose who reads to you.** The first time you press play, a dialog offers three voices:
  Anna and Berta (female), Imre (male), each with a sample line. The choice is remembered in the
  browser and can be changed from the audio bar or Settings → Speech.
- **Listening mode inside Reading.** The Listen tab is gone; «Аудирование» is a toggle in the reader
  (`/read/:id?mode=listen`) that keeps everything it had: the three passes (transcript hidden →
  shown → dictation with a diff), repeat/next shadowing by sentence, speed, the coverage warning,
  and recordings with an A/B loop. Uploading your own text with audio moved to the Library
  («Свой текст с аудио»). `/listen` redirects to the Library.
- **Listening hours still count.** All audible playback is reported to the same listening ledger
  that gates the curriculum, and reading time pauses while audio plays, so no second is counted twice.
- **Silent fallback.** If the local voices are off or fail, the reader falls back to your device's
  speech without an error banner. Review cards and practice still use device speech.

### Settings → Components
- **Speech (Piper) is an on/off switch, not an installer.** «Включить» checks the bundled files,
  unpacks them and loads the three voices into memory (about 300 MB while on); «Выключить» frees the
  memory and returns to device speech and survives a restart. «Удалить распакованную среду» frees
  disk space; the engine and voices stay inside the app and are unpacked again on the next «Включить».
- No download, no pip, no model fetch. On a platform without a bundled runtime the card says so
  calmly and the device voice is used.

### Under the hood
- **Vendored, hash-pinned runtime** in `services/tts/vendor/` (371 MB): a standalone Python 3.12 and
  seven wheels (piper-tts 1.8.0, onnxruntime 1.32.0, numpy and four small ones) for macOS arm64,
  Linux amd64 and Linux arm64, every file pinned in `MANIFEST.sha256`/`requirements.lock`.
  `make tts-vendor` is the only target that uses the network (run by a maintainer);
  `make tts-verify` re-hashes everything offline and is part of `make docs-check`.
- **Release builds embed it.** `make build` stages one platform and builds with `-tags ttsbundle`
  (`TTS_BUNDLE=0` opts out): the binary grows to about 326 MB on macOS arm64 and «Включить» works
  with no flags. Dev and test builds stay tag-less. The `full` Docker image does the same
  (about 555 MB, linux/arm64 measured); `slim` has no speech.
- **The three voice models are committed** to the repository (3 × 63 MB, no Git LFS).
- **Server API**: `GET /api/speech/voices`, `POST /api/speech/tts`, `GET /api/speech/audio/:file`
  (auth required; a content-addressed cache under `<data>/cache/tts`, pruned at 512 MiB);
  `ListenProgressRequest.pass` accepts 0 for playback outside listening mode.
- **Offline proof**: `make tts-smoke` synthesises all three voices with outbound network denied
  (macOS `sandbox-exec`, Linux `unshare -rn`); the same was verified with `docker run --network none`.
- Extraction is hardened (tar-slip, symlink and size limits), files are verified while copied and
  unpacked from the verified copy, and the runtime is swapped in atomically.

### Licences and obligations
- Piper and its bundled espeak-ng are **GPL-3.0-or-later**. They run as a separate process and are
  never linked into the Go binary. Their corresponding source (`piper_tts` sdist, `piper1-gpl`
  v1.8.0, espeak-ng at the pinned commit) is committed in `services/tts/vendor/source/` and shipped
  in the `full` image under `/usr/share/doc/tanulom/tts-source/`. If you redistribute a binary or
  image, ship that source with it.
- The voice datasets are CC0. **Anna and Berta are fine-tuned from `hifi_tts_92` (en_US), whose
  licence is not yet confirmed**; this is recorded as open in `langpack/LICENSES.md`.

### Requirements and limits
- Bundled speech runs on macOS 14+ on Apple silicon, Linux amd64 (glibc 2.28+) and Linux arm64.
  Not supported: macOS Intel, Windows, and the `slim` image (the device voice is used there).
- The Linux bundles are verified by hash and by an offline Docker run on arm64; the linux/amd64
  bundle was not executed on a real host. Audio playback was verified through the server, the
  Settings flow and code review; the sentence-by-sentence playback chain with real audio was not
  heard end-to-end in an automated browser (it has no audio clock), so listen to a text once.
- Piper is not shown to be thread-safe, so synthesis is serialised. Pronunciation scoring and
  speech recognition are still not built.

### Upgrade notes
- The repository grows by about 560 MB (371 MB vendored runtime + 190 MB voices); refreshing the
  vendored runtime adds that again, so do it rarely.
- No database migration. Existing listening state and logs are kept as they are.
- The «Слух» (Ear) screen and its minimal-pair training are unchanged; only the Listen tab moved.

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
