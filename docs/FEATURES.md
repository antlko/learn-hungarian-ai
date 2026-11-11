# What tanulom does

A plain-language tour for a learner and for the person who hosts it. Each section says what you
see, and what is optional. For *why* it works this way, follow the links to
[PEDAGOGY.md](PEDAGOGY.md) and the decisions in [`.claude/DECISIONS.md`](../.claude/DECISIONS.md);
for how to run it, [SELFHOSTING.md](SELFHOSTING.md).

**The one idea.** Most apps treat a word form as one thing. Hungarian builds words from a stem plus
suffixes, so tanulom keeps two separate memories: one for **words** and one for **grammar facts**
(for example "-ban, in"). Answering one sentence can update both, and only the grammar the
question actually asked about.

**Nothing below needs a download or a second service** unless it says "optional". The app ships
with a starter dictionary inside it ([README](../README.md#do-i-need-the-big-dictionary-the-40-minute-thing)).

## Today

The home screen answers "where am I and what next?":

- **Your estimated level** (for example `A2.2`) and "about 71% of the words in ordinary text".
  The same chip sits in the top bar. It is an estimate computed from your deck, not an exam
  (see [Progress](#progress)).
- **One next action**: review the cards that are due, otherwise start the next unit, otherwise
  the day's minutes.
- **Today's obligations**: cards cleared and minutes done, and the streak.
- **Words from your reading**, kept apart from the course so mining a word never changes
  which unit is next.
- **The roadmap**: the 24 course units under their months, plus checkpoints. Each unit is
  *пройден* (done), *начат* (started), *следующий* (next) or *впереди* (ahead). Nothing is
  locked; the order is advice.

Optional: nothing.

## Review

Cards come in five directions per word: recognise, produce, listen, cloze (fill the blank in a
real sentence) and spell. A wrong suffix is highlighted, not the whole word. Scheduling is
FSRS-6, unmodified.

- **How a card works.** A new or forgotten card goes through short learning steps (about 1 and
  10 minutes), then graduates to days. The card tells you which stage it is in and what each
  answer would schedule. **There is no "N correct in a row" rule**, because FSRS has none: a
  miss simply brings the card back soon. The «Как это работает» dialog says this in the app
  ([ADR-051](../.claude/DECISIONS.md#adr-051)).
- **Sets.** The day's queue is dealt in sets (default 15, your choice, set on the start screen).
  The missed ones are listed with their answers at the end of a set, then fetched again.
- **Hint and reveal.** «Подсказка» shows the answer and counts as a hint. The answer is checked on
  the server; the page never knows it in advance.
- **Accent strip.** On a phone, a row of Hungarian letters (á é í ó ö ő ú ü ű) sits above the
  keyboard. Typing without accents can be accepted with a lower grade; you choose in Settings.
- **«Знаю».** Sets a word aside because you know it. Nothing is deleted; restoring it brings
  the card back overdue.
- **Practice mode.** Free practice (random or weakest words). Since ADR-064 a practice answer is
  a real review: it moves the word like any other (except words in «Знаю» and new words, which
  say «Не засчитано» and why); a counted answer can be undone. Before a word is due memory grows
  less, so practice takes more answers.
- **A progress bar for every word.** 0–100 (ADR-070), shown while reviewing, in the word list,
  in the word's inspector and as a words-by-progress chart on Progress. The stages are marks on
  the bar at 20 and 70; a right answer moves it up, a mistake moves it down. 100 means about
  half a year between showings, and the word still comes back — it is progress, not a score.
- **Mobile.** Large touch targets and the bottom tab bar (see [Mobile](#mobile)).

Optional: the tutor can be asked about a card (it never changes the schedule).

## Words

Every word in your deck as a list, with a **stage ladder**: *учу* (short steps),
*вспоминаем* (forgotten, back on short steps), *повторяется* (interval under 21 days),
*закреплено* (21 days or more) and *знаю* (set aside). Filter by stage, sort by weakest or by
due date. The estimate of memory is computed by the FSRS library, not invented.

## Grammar

- **A path of 33 topics**, drawn like the roadmap, grouped by level, with plain phase names:
  «только правило» (explained only), «ещё не начато», «тренируем по одному» (one pattern at a
  time), «вперемешку» (mixed), «освоено».
- **Lesson first, exercises second.** A first visit opens on the lesson; on a return visit the
  exercises lead.
- **The gate, in numbers.** Practice stays one pattern at a time until your recent accuracy
  reaches 80% (shown as "17 of 20 correct, need 80%"), then mixes topics, because mixing too early
  hurts beginners ([PEDAGOGY.md](PEDAGOGY.md), R5).
- **13 of the 33 topics have exercises.** The rest are explained only and say so; they do not lock
  anything after them. Where an error cannot be fixed by typing practice (vowel length), the app
  routes you to ear training instead.

Optional: the tutor, linked from every lesson.

## Reader and Library

- **Read** a pasted text, a URL, an `.srt` subtitle or an `.epub`. Every word is analysed and
  coloured by what you know. Click a word to see its analysis, add it as a card, or mark it known.
  Uncertain analyses and fallback glosses are labelled.
- **Ready-made reading sets.** Library offers, for each basic-tier course unit and theme (36 in
  the starter), a «Фразы» set of real sentences from the dictionary and a «Диалог» — one short
  conversation written by an AI, labelled «Написано ИИ, носителем не проверено» (every word is in
  the dictionary; no native speaker has read it). «Посмотреть» shows every line with its translation
  before anything is added. Importing is an explicit click; it creates a text, never cards.
- **Level estimate for texts.** Each text shows the share of its words you already know.
- **Optional, translator.** A machine-translated crib under a sentence, always labelled with the
  model, never used as a card answer.

## Listen and Ear

- **Listen**: a three-pass protocol (blind, with text, shadowing) with loop and speed control, and
  dictation. Your listening time counts toward the day.
- **Ear training**: minimal pairs (words that differ by one sound, such as `kerek / kérek`) with
  several voices, because varied voices are what makes this training work. A session with too few
  voices is refused with the numbers. No contrast is ever marked "done".

Optional: nothing. The voices are clips inside the app; spoken words use your browser's voices.

## Tutor

A grammar chat in your language, available under every lesson, on its own page and from a review
card. It uses the lesson and the dictionary as its sources, names the model on every answer,
flags any Hungarian word the dictionary does not carry, and **cannot create cards**. It needs a
model: any OpenAI-compatible endpoint; `qwen3:8b` is recommended and a 1.7B model is too small
([TUTOR.md](TUTOR.md)). The AI dictionary entry for a word the pack lacks works the same way.

Optional: yes, entirely. Without a model the tutor entry points are hidden or say why.

## Progress

- **Estimated level.** A CEFR-style label such as `A2.2` computed from (a) how much of each
  level's frequent vocabulary you know and (b) how far you have got in that level's grammar
  topics. A band counts as reached when you know at least 80% of its words and at least half its
  topics with exercises are at mixed practice or mastered, and no checkpoint is failed. A word counts as
  known only if you set it aside or its card has graduated and you probably still remember it, so
  **the level can go down** if you stop reviewing. It is shown with its method sentence and a
  "few data" or "estimate" qualifier, never as a score ([ADR-047](../.claude/DECISIONS.md#adr-047)).
  Treat it as an estimate that can be off by a sublevel or two.
- **Text coverage.** The share of ordinary running text made of words you know.
- **The lemma curve, an activity calendar** (minutes by day: cards, reading, listening), and a
  **suffix heatmap** of grammar facts by strength.
- **Checkpoints** at months 1, 3 and 6: pass or fail. A failed one holds the next level shut;
  nothing else is ever locked.

## Course

Sixty-four units in three finite tiers — **basic** (24 units, about 1,400 words), **intermediate**
(20 units, about 1,000) and **advanced** (20 units, about 700) — go at your own pace. Each is tied
to a real errand ("register at a GP"), the grammar it needs and about 15 to 60 words. A unit's
words are "in your deck" once you take it up; the course counts how many you have actually
**learned** (known, or graduated and still remembered) beside how many are in the deck. **Twelve optional theme units** (numbers, the clock, days and months, colours,
the body, weather, food, clothes, home, travel, moods and hobbies) sit beside the path and never decide
what is next. **The course ends** and hands you a catalogue of real Hungarian; it is not an
endless drip.

## Settings and admin

- **Components** (admin): what is installed and what is missing, each with what you lose without
  it. Install the **full dictionary** with a progress bar and checksum, then **restart** from the
  page (works when the container has a restart policy). The full dictionary can be installed this
  way only once a pack is published; none is yet. Sidecars (translator, analyser) cannot be
  started from the app by design: you run one command and the page shows its state and the
  model download progress.
- **Services**: endpoints for the model, translator and so on. API keys are stored in the
  database, never in `config.yaml`.
- **Accounts** (admin): several people can share an install, each with their own cards and progress.
- **Export / import**: a full dump of one profile (cards, review history, texts, notes, chats)
  as a zip. It is verified on import by replaying the history. Secrets are left out.
- **Review, target minutes, new cards per day, set size, accent tolerance, theme.**

## Mobile

Below tablet width the left menu becomes **bottom tabs** (Сегодня, Повторение, Чтение,
Грамматика, Ещё), the roadmap and review are single-column with touch-sized controls, and the
review card keeps the Hungarian letters strip above the keyboard. Offline use is **not** built.

## Languages

The interface is in Russian and English, complete in both. Word meanings are shown in your chosen
gloss language and fall back to English with a visible label where Russian is missing.

## What it deliberately does not do

- **No points, XP, levels you earn, leagues, leaderboards or gems.** The only motivation shown is
  measured competence: the streak (obligations met), the curve, the heatmap, checkpoints, the
  exam countdown, and the *estimated* level, which falls if your memory does. The reasoning is in
  [R14](../.claude/research/14-motivation.md).
- **No replacement for a teacher.** The "talk to a human" loop is one only you can tick.
- **No invented Hungarian as material.** Sentences are ones people wrote. Model output (tutor,
  dictionary entry, translation) is labelled, checked against the dictionary where possible, and
  stays on its screen.
- **No silent guesses.** Uncertain analyses, fallback glosses, machine translation and
  low-confidence results are labelled.
- **No cloud and no telemetry.**

## Known limits

- FSRS's efficiency figure is a simulation, not a trial on Hungarian learners.
- There is no per-user FSRS parameter optimisation in v1.0.
- Coverage thresholds (95%, 98% for text) were derived for English.
- The **level is an estimate**: bands come from word frequency, not an exam, and it may be off by
  a sublevel or two.
- Only **13 of 33** grammar topics have exercises.
- **Speech recognition is not built** (`services/asr`), nor synthesized card audio
  (`services/tts`); the browser's voices are used. Hungarian ASR in general has about 11% word error rate.
- The ear trainer has **three** synthetic voices where the research asks for five or more.
- Optional sidecars are labelled as untested where they are: the analyser's container has not
  been run in CI and its memory use is a published figure, not ours.
- No offline mode, no speaking practice, no writing corrector.
