# UX specification

Screen by screen. Design principles first, because they explain the individual decisions.

---

## 1. Principles

1. **The study plan is the product; the app is furniture.** The home screen shows what to do
   today, not what the app can do. No feature grid, no "explore" tab.
2. **Keyboard first, everywhere.** Review, reading and drilling are high-repetition activities.
   Anything requiring a mouse in a loop you do 200 times a day is a design failure.
3. **Never interrupt a flow with a modal.** Adding a card while reading is one keystroke and a
   toast. Nothing that stops reading is allowed in the reading path
   ([R11](../.claude/research/11-extensive-reading.md)).
4. **Show the mechanism.** When the app schedules, gates or rejects something, it says why, in
   one sentence, in plain language. A learner who does not trust the system will override it.
5. **Uncertainty is visible.** Fallback glosses, uncertain morphological analyses, unattested
   LLM output and low-confidence ASR are all labelled. Never a confident-looking wrong answer.
6. **Austerity where it is a drill.** The Ear trainer is two buttons on a blank field. It is
   not an experience; it is 10 minutes of forced-choice repetition.
7. **Light first**, both themes complete, `prefers-color-scheme` honoured, explicit toggle wins.
   A fresh install opens on paper (`:root` carries the light palette; `.dark` is the override) —
   flipped from dark-first when the first learner read the dark default as unfinished rather
   than as a choice. Nothing in a component names a colour, so the flip lives entirely in
   `index.html`, `stores/ui.ts` and the token file.

---

## 2. Shell

```
┌──────────────┬──────────────────────────────────────────────────────┐
│  tanulom     │  due 42  ·  🔥 17  ·  ●TTS ●ASR ○NLP ●LLM   ru|en 🌙 │
├──────────────┼──────────────────────────────────────────────────────┤
│ ▸ Today      │                                                      │
│   Review  42 │                    ‹ route ›                         │
│   Read       │                                                      │
│   Listen     │                                                      │
│   Speak      │                                                      │
│   Ear        │                                                      │
│   Grammar    │                                                      │
│   Write      │                                                      │
│   Library    │                                                      │
│ ─────────    │                                                      │
│   Progress   │                                                      │
│   Settings   │                                                      │
└──────────────┴──────────────────────────────────────────────────────┘
```

**Capability dots** — filled = reachable, hollow = configured but unreachable, absent = off.
Hovering gives endpoint and latency. Features that need an unreachable service are visibly
disabled with a one-line reason, never silently missing.

**The shell, as built.** The sidebar orders by the daily loops — Today, Plan, Review (with the
`dueNow` badge), Read, Grammar, Listen, Ear — with the two unbuilt sections («Говорение»,
«Письмо») at the BOTTOM under a visible «скоро» heading: a dead row between two working ones
reads as breakage, and a `title`-only tooltip is invisible on touch. It **collapses to a 56px
icon rail** (PanelLeft button at the foot, persisted per device in `tanulom.sidebar`, the badge
becoming a dot on the icon; the «скоро» group is hidden entirely there — an icon that does
nothing and cannot explain itself is worse than absence). Below `md` the rail is replaced by an
off-canvas drawer behind a topbar hamburger; before that, the fixed 224px column simply ate 60%
of a phone screen.

**The rail is collapsed by default** — a nav one mouse-move away does not need to hold 224px of
every screen — and **opens under the pointer**, folding back ~120 ms after it leaves. The collapse
arrow therefore pins rather than merely closes: pinned open it never moves, pinned closed it is a
56px rail that reads in full whenever you actually look at it. Three details are load-bearing:

- **It overlays; it does not push.** The wrapper keeps the pinned width and the expanded rail
  floats over the page. Widening a rail that is part of the flex row would reflow everything on
  every hover — the reading measure changing mid-sentence, text jumping under the cursor.
  `--rail` stays the PINNED width for the same reason: the key bar and the milestone strip centre
  themselves on it, and a hover must not slide them. Measured: `main`'s left edge and the key
  bar's are unchanged at 56px whether the rail is folded or open.
- **The hover target is the rail, not a fixed-width wrapper.** It shipped on the wrapper once,
  and the wrapper is 56px while the open rail is 224px — so `pointerleave` fired the instant the
  cursor moved onto the panel it had just opened. It folded, the cursor came back, it opened
  again. `pointerleave` counts descendants, so the rail with the collapse arrow inside it is the
  one element that covers everywhere the pointer can legitimately be.
- **Only where hover exists** (`(hover: hover)`), because `pointerenter` fires on a tap and a
  touch tablet wide enough to show the rail would otherwise pop the nav open on a single tap.
- **No open delay, a close delay.** Opening has to feel immediate; folding waits so a diagonal
  sweep past the rail does not flicker it. Keyboard focus counts as being there, or tabbing into
  a collapsed rail would land on an unlabelled icon.

- **The icons sit on the rail's centreline.** A row's padding is asymmetric when expanded
  (`pl-4` sets the text's left margin, `pr-3` keeps the badge off the edge) and symmetric
  (`px-3`) when collapsed. Carrying the asymmetry into the rail put every icon 4px right of
  centre — the nav's `px-2` leaves a 40px row, and a 16px icon after `pl-4` centres at 24
  against the row's own 20. Padding is in the transition, so the correction rides the expand
  rather than snapping at one end of it.
- **Nothing inside pops.** The wordmark cross-fades between `t` and the full name, the «скоро»
  group and the due badge fade rather than being `v-if`-ed in and out. A hard swap snapped the
  header from one glyph to two lines the instant the hover registered, which read as the whole
  panel juddering even while its width was interpolating perfectly.

The width transition is the documented §14.3 exception (150 ms, `cubic-bezier(0.2, 0, 0, 1)`),
and the global `prefers-reduced-motion` rule flattens it like everything else — no per-component
check. **That means a learner whose OS has "reduce motion" on gets an instant rail, on purpose**,
and no amount of tuning the duration will change that for them. The topbar carries the page's name on the left and, per the sketch above,
the due chip and the streak flame — both from the deck store, which reads the same `Breakdown`
as the review tabs, throttled and refreshed on navigation so the chrome can never contradict
the screen beside it.

**The right rail** is the shell's answer to "where am I in this". Four screens use it and the
shape is the same everywhere: `flex flex-col gap-6 lg:flex-row lg:items-start`, a
`min-w-0 flex-1` main column, and a `lg:w-80 lg:sticky lg:top-2` aside that follows the scroll
and stacks underneath below `lg`.

| Screen | Main column | Rail |
|---|---|---|
| Read | the text | the word's breakdown, its meaning, its translation |
| Review | the card | the word breakdown (`WordInspector`) |
| Plan | the unit you are on | **«Весь план»** — the 24 units by month |
| Grammar | the lesson, then the drill | **the level's topics**, current one marked |

The last two are new, and both were the same failure: the orienting answer existed but was
somewhere else. Plan kept the whole syllabus BELOW the unit card, so "what am I doing now" and
"where does it sit" were a scroll apart although they only mean anything together. Grammar had
no list at all while a topic was open — «Все темы» was the only orientation, and it was a button
that threw the topic away to show a map, so asking what was around this cost you the thing you
were reading.

**The key strip** (`components/common/KeyBar.vue`) is fixed to the bottom of the viewport on the
two screens that are keyboard loops — the reader and the review card — and draws the keys AS
KEYS.

```
        ┌───────────────────────────────────────────────────────────┐
        │ ← → по словам  ↵ в карточки  ⎵ знаю  ⌫ игнорировать   ✕   │
        │ s озвучить  t перевести                                   │
        └───────────────────────────────────────────────────────────┘
```

Principle 1.2 says keyboard-first everywhere, and that was true of the code and not of the
screen: each of those loops ended in one grey `text-xs` sentence below the fold. Sentence
translation sat on `t` for a release and was reported as a missing feature. A shortcut nobody
can see is a shortcut nobody uses.

- **Built from the handler, not from a locale sentence**, so a key that is added, moved or gated
  cannot go on being advertised. `t` is drawn only where a translator is reachable — and is no
  longer `preventDefault`ed on installs without one.
- **It reflects state.** The review strip's first cap says «показать» or «дальше» by phase and
  `↵ проверить` on typed directions; `1`–`4` and `Esc` are drawn dim rather than removed, so the
  row does not reflow under the hands between cards.
- **Not a control surface.** `role="group"`, no tab stops, caps `aria-hidden` with the readable
  label beside them — a screen reader gets "next word", not "arrow right arrow left". It
  documents the keyboard; it is not a second way to press these keys.
- **Dismissible and remembered per device**, because a permanent hint becomes furniture you stop
  seeing. It collapses to a `⌃ Keys` pill, never to nothing.
- It **wraps** rather than scrolling: `overflow-x-auto` hid the last item behind the edge with
  nothing saying it was there, and «перевести» read as «перевест».
- `<main>` carries the bottom padding for it (`AppShell.vue`), so the last line of a text is
  never behind the strip.

---

## 3. Today — the screen that decides whether the app works

The four loops from the brief, as the home screen.

```
┌─ Сегодня ─────────────────────────── чт, 22 авг ── 3 дня до чекпойнта M1 ─┐
│                                                                           │
│  ╭─ 1 · Память ────────╮ ╭─ 2 · Вход ─────────╮                          │
│  │      ◕  18/25 мин   │ │      ○  0/30 мин   │                          │
│  │   42 к повторению   │ │   Sziszi ep.14     │                          │
│  │   12 новых          │ │   покрытие 91%     │                          │
│  ╰─────────────────────╯ ╰────────────────────╯                          │
│  ╭─ 3 · Разбор+выход ──╮ ╭─ 4 · Человек ──────╮                          │
│  │      ○  0/30 мин    │ │   вт 19:00 · Anna  │                          │
│  │   Грамматика:       │ │   ☐ отметить       │                          │
│  │   опред. спряжение  │ │   вручную          │                          │
│  ╰─────────────────────╯ ╰────────────────────╯                          │
│                                                                           │
│  ▸ Начать: Память                                    [space]              │
│                                                                           │
│  Задача недели: заказать кофе только по-венгерски          ☐              │
│  Нагрузка (30 дней) ▁▂▃▅▆▅▄▃▂▂▃▄▅▄▃▂▁▁▂▃▄▃▂▂▁▁▂▂▁▁                       │
└───────────────────────────────────────────────────────────────────────────┘
```

**Loop 4 has no completion button the app can press.** A checkbox the learner ticks manually,
with a line explaining why: *"Приложение не может это засчитать за вас. Живой собеседник —
единственная незаменимая часть."* ([R16](../.claude/research/16-debunked.md) §7.)

One primary CTA routes to whatever is actually due. `space` from anywhere on this screen starts
it. **As built:** queue first — reviews are a debt already incurred — then input, then study.
The button was specified here from the beginning and was missing from the screen, which left the
home page as four cards of statistics and made the learner decide what to open. Deciding that is
what the study plan is for. It goes straight to the card at `/review/session` over the whole
deck, past the review start screen (§4.5), because it exists so the learner does not have to
choose; the memory loop card goes to the start screen, where the choice is
([ADR-028](../.claude/DECISIONS.md#adr-028)).

**The queue figures here are the review screen's own** (`overview.queue`, the whole-deck
Breakdown — ADR-021): «к повторению» is what the queue will serve now, with «вернутся сегодня:
N» named separately when learning steps are still maturing, and the memory loop card carries
`+новые` in words. This screen printing a raw card count over a burial-collapsed session is
exactly how the app confused its first learner. The 30-day load strip from the sketch above is
built: minutes as bar height, obligation-met days in green, straight off `daily_log`. First
paint holds the layout with skeletons; errors carry a retry.

---

## 4. Review

Chrome-free. The card owns the viewport. What to review — which pile, in what shape, or a
practice round — is chosen on the **start screen** at `/review` (§4.5); the card lives at
`/review/session` and takes its intent from the query.

```
                    ┌─────────────────────────────────┐
                    │  A ház___ lakom.        🔊      │
                    │                                 │
                    │  Я живу в этом доме.            │
                    │                                 │
                    │        [ ваш ответ ]            │
                    └─────────────────────────────────┘
                              12 / 54
```

After answering:

```
                    ┌─────────────────────────────────┐
                    │  A házban lakom.        🔊      │
                    │       ▔▔▔                       │
                    │  вы: ház·ben   →   ház·ban      │
                    │      ─────         ─────        │
                    │  гармония гласных: á — задний    │
                    │  ряд, поэтому -ban, не -ben      │
                    │                                 │
                    │  проверено:  Case=Ine ✓          │
                    │              гармония ✗          │
                    ├─────────────────────────────────┤
                    │  1 Снова  2 Трудно  3 Хорошо  4 Легко │
                    └─────────────────────────────────┘
```

The **morpheme diff** is the point: the wrong suffix is highlighted, not the whole word
([`internal/morph/diff.go`](../backend/internal/morph/)). The **feature chips** show which
`morph_state` entries this answer graded — making the second state space visible rather than
mysterious.

**Keyboard:** `space` reveal (`Enter` on typed directions) · `1`–`4` rate · `h` hint ·
`u` undo · `t` set the word aside (§4.3) · `a` the tutor about this card, in both phases and
free ([ADR-046](../.claude/DECISIONS.md#adr-046)) · `Esc` close the side panel (the word breakdown,
then the tutor) · `space` on a set summary starts the next set (§4.4). Drawn by the key strip
(§2), which is where `u` and `Esc` reach a learner for the first time: both have been implemented
since the screen was built and were advertised in neither of the two hint sentences the strip
replaces. `t` is drawn dim while the answer box has focus, because it is a letter and goes into
it — the same guard `h` and `u` sit behind, and the button under the card always works.

**«Учитель», top right, in both phases** ([ADR-034](../.claude/DECISIONS.md#adr-034)). A
ghost button beside the direction chips, key `a`. The tutor opens in the side column the word
inspector uses, with **prepared questions about this card** as chips above the composer — one
click sends a question the SERVER composes from the card and the pack, and the answer comes
back in a fixed shape (numbered sections, no check question). Which chips, by direction ×
phase × outcome:

| card | before the answer | after: wrong | after: right |
|---|---|---|---|
| узнавание | «Из чего состоит `X`? (подсказка)», «Что спрашивает эта карточка?» | — | «Разбери предложение целиком», «Почему именно `X`?», «Переведи `X` и дай два примера» |
| воспроизведение · форма в предложении | «Подскажи, не называя слова» / «Какая форма нужна и почему?», «Что спрашивает…» | «Почему не `typed`?», «Почему именно `X`?», «Разбери предложение», «Переведи…» | «Почему именно `X`?», «Разбери…», «Переведи…» (+ «Как пишется…» when accepted without diacritics) |
| написание | «Что спрашивает…» | «Как пишется `X` и почему», «Разбери…», «Переведи…» | «Как пишется…», «Разбери…» |
| на слух | «Что спрашивает…» | «Разбери…», «Почему именно `X`?», «Переведи…» | same |

**Before the answer, asking costs the card nothing** ([ADR-046](../.claude/DECISIONS.md#adr-046)).
Until 2026-09-22 a question in the question phase set `usedHint` and capped the grade at Hard,
like «Подсказка»; it no longer does, and no signal from the panel reaches the answer at all.
What defends the grade is unchanged and is what the panel now says: the server withholds the
answer from the model in that phase and the hint chips ask for the rule, never the form —
«До ответа учитель не получает правильный ответ и не называет его. На расписание слова вопрос
не влияет: карточка всё равно спросит вас, и оценка будет ваша.» «Подсказка», which does show
the graded string, still caps at Hard and still says so on «Хорошо» / «Легко». Free questions are typed in the same composer and
travel with the card as context. **One chat per word** («Повторение · `jöttél`» on the
AI-teacher page); the panel's link opens it there. The same word later the same day — its
learning step — continues that conversation; the next word opens an empty panel. The standing
honesty paragraph and the sources list are NOT drawn here: they are the chat screen's, and
what defends an answer travels with the answer — the model's name, the lesson it used, the
words the dictionary does not carry, the small-model note and the context meter ([ADR-034],
amended). Keys pressed in the composer are the composer's; `Esc` closes the panel unless the
composer kept it to stop an answer.

**The card header names what it is asking.** It used to read `УЗНАВАНИЕ НОВАЯ` — two words
uppercased by CSS, run together with nothing saying that the first is the direction and the
second means you have not seen this card before. It read as one unparseable label. Now: the
words «тип карточки», then two visually distinct chips, then a sentence under the progress bar
saying what this direction asks of you («узнавание — видите венгерское слово, вспоминаете, что
оно значит»), for all five directions. The chips keep the no-hit-target rule that put the
direction there in the first place — `пропуск` reads in Russian as the imperative "skip", and as
a bordered pill it was clicked as a skip button and did nothing.

**Audio autoplays** on `listen` and `spell` directions. Never on `recognize` — it would give
the answer away. Those two directions also withhold the Hungarian entirely until the answer:
the sound is the question, and printing the sentence turns a dictation into a copying exercise.
Speech comes from the device by default ([ADR-012](../.claude/DECISIONS.md#adr-012)); a device
with no Hungarian voice gets the reason on the card rather than a silent one.

**The direction indicator is a LABEL, not a control.** It used to be a bordered pill in the
top-right corner where a control belongs, and the Russian name of the `cloze` direction was
«пропуск» — which reads as the imperative "skip". It was clicked as a skip button and did
nothing, which is the worst outcome an affordance can have. It is now unboxed text, and the
direction is named «форма в предложении».

**A cloze names the word it wants.** `Jó ____, Timmy!` accepts reggelt, napot, estét and
éjszakát — four real greetings — so a card showing only the gap is asking the learner to guess
which one was meant. Cloze is the only direction that writes to the morphological state space,
so that guess landed in the grammar heatmap as a case failure. The card therefore carries the
target's DICTIONARY form and its meaning (*«Поставьте в нужную форму: éjszaka — ночь»*), and
deliberately not the required features: naming the case would hand over the answer the card
exists to ask for. The word is given; the form is the exercise.

**A typed card says how much to type.** `produce` prompts with the target word's gloss, and one
Hungarian word often glosses to a Russian phrase — `viszlát` is «пока, до встречи». Learners
typed the phrase and were marked wrong for answering the question they were asked, so the
placeholder names the scope («одно слово по-венгерски») rather than adding a line of chrome.

**`produce` asks for the DICTIONARY form; `cloze` asks for the form in the sentence.** They are
different questions and a note carries both. `produce` used to be graded against whatever
inflected token the note's sentence happened to contain, so a card printed «день; солнце» — the
entry for `nap` — and demanded `napokra`, a sublative plural nothing on the card had named, while
the placeholder promised one word. The sentence and its suffix are not lost: they are the cloze
card's question, on the direction that grades morphology.
[ADR-017](../.claude/DECISIONS.md#adr-017).

**The queue is two piles, and the learner picks.** «Всё · План · Тексты» on the start screen
(§4.5) — until 2026-09-03 a segmented control above the progress row — draws course material and
words mined from reading as separate stacks, because "finish today's course work" was otherwise
impossible without also clearing every word ever clicked in a text. **Each pile carries two
numbers, due and new.** The reader pile really can sit at 0 due and 88 new, and one merged figure
would either hide that backlog or invent one. The daily new-card budget is shared across both,
and the copy says so: switching piles buys no extra new material. The card names its pile in the
progress row, and the done screen says what the other piles hold and links back to the start
screen, so emptying one pile does not need a reload to discover the other.
[ADR-016](../.claude/DECISIONS.md#adr-016), [ADR-028](../.claude/DECISIONS.md#adr-028).

**A cloze over the learner's own sentence says what the sentence means.** A word added from the
reader has no pack sentence behind it, so there is no human translation to show — the crib comes
from the MT sidecar and is printed **beside the Hungarian, with the model named beneath it**,
never in a tooltip. It appears at question time on `cloze` alone: that direction asks *which
form*, and a learner cannot choose one for a sentence they cannot read. The blank is Hungarian
and the crib is not, so nothing is handed over, and the required features stay unnamed as ever.
With the translator off the card is Hungarian plus its gloss and says nothing about a failure.
[ADR-018](../.claude/DECISIONS.md#adr-018).

**The cloze blank is split by the server.** `note.target_start/target_end` are byte offsets into
UTF-8 and do not cross the wire: the client used to slice `hu` with them as JavaScript string
indices, and one accented letter before the target put the gap inside the following word.

**Every word on the card is clickable, and the card says what the sentence means.** The card
used to be an inert string with the TARGET WORD's dictionary gloss floating under it —
`Jó hírt hoztunk.` above «хороший; добрый», which is the entry for `jó` and reads as a
mistranslation of the sentence. Three things fixed that, and all three were already in the pack:

- the target word is **underlined** in the sentence, so the gloss has a visible referent, and
  the gloss line names the word it belongs to;
- the **whole sentence's translation** is shown (`sentence_trans`, 19,641 EN / 3,746 RU rows the
  review screen had never read), on reveal rather than on the question;
- clicking any word opens the **inspector** (§4.1).

### 4.1 The word inspector

A panel beside the card — the same one the reader's rail is — answering three questions in the
order a learner asks them: what does it mean (the dictionary entry), what SHAPE is it in (the
morpheme breakdown of this exact form, every feature named from the pack's label table), and
where would I meet it (attested sentences with their translations, `GET /api/lemma/:id/examples`).

The middle one is the reason it exists. `hoztunk` is not a word to memorise, it is `hoz` + past
+ 1pl + indefinite, and a learner who can see that decomposes the next one themselves. Nothing
in the panel is generated: an unresolvable form says so, and a word with no attested sentence
says that too rather than being given an invented one.

`Esc` closes it. It is available in the reader too, where the rail gained the same examples.

### 4.2 The end of a session, and the daily new-card limit

**Every number on this screen speaks the Breakdown vocabulary** (ADR-021): post-burial,
day-granular, one coordinate system shared with the tab strip and Today. The first learner's
session was «26 к повторению» over ten served items, «на сегодня всё», and five more cards on
refresh — three numbers, three coordinate systems, all technically true.

**Three endings, not two, and the third counts down.** Clearing what is answerable NOW is not
the end of the day when learning steps are still maturing: «Пока всё» carries «Сегодня
вернутся ещё: N — через M:SS», a live countdown to `nextDueAt`, and the queue **reloads
itself** when the step lands (and on tab focus) — the session summary accumulating across the
reload, because it is the same sitting. «Возвращайтесь завтра» is said only when nothing
returns today. Under it, the hidden cards are named instead of silently counted: «отложено до
завтра: K — другие стороны уже отвеченных слов», and a new word deferred because its sibling
was answered today is reported as a DEFERRAL, never as the daily limit's doing — `pacedOut` is
true only when the budget is genuinely spent with words genuinely behind it.

**The session summary** is four measured figures — answered, accuracy, new words started,
minutes — a competence signal in the [R14] sense, not a celebration.

Two different endings, and they used to print as one. "Очередь пуста. Возвращайтесь завтра"
directly above "новых карточек в очереди: 59" is a screen contradicting itself — the queue is
not empty, the day's ration of NEW material is spent. Clearing the day's work and running into
the cap now read differently, and the cap states its arithmetic:

- **in words first, cards second.** A note is a word; a card is one direction of it and a note
  spawns three to five. "59 new cards" said to someone who had taken up 27 words was accurate
  and unusable — there was no way to get from one number to the other. The screen leads with
  words and explains the multiplier.
- **the pace, and what it implies.** Started today, out of the limit; how many days the backlog
  takes at that rate. A limit without its arithmetic reads as a fault.
- **where the dial is.** A link to Settings. Saying "come back tomorrow" while hiding the
  control makes a setting look like a verdict.

**The limit budgets what reaches the screen.** It used to select `newPerDay` cards and *then*
bury siblings — and new cards come back ordered by note, so a note's three directions sit
together: a limit of 20 selected 20 cards, collapsed them to about seven notes and showed seven.
The learner met a third of the new material their own setting promised, every day, with nothing
saying why. Buried first, then trimmed. `internal/srs/queue.go`, pinned by
`queue_newlimit_test.go`.

**The reveal always states the answer.** «Подсказка» sets the card to its answered state without
submitting, so on a typed card there was no diff and no server verdict — and therefore nothing on
screen saying what should have been typed. It was worse once `produce` began grading the
dictionary form, because the sentence printed below carries the *inflected* one, so the answer
appeared nowhere at all. The reveal now prints it plainly, suppressed only when the diff is
already showing both lines or when the learner just typed it correctly.

**The crib sits with the sentence it translates.** On `produce` the Hungarian appears only in the
reveal, while its translation was rendering up beside the gloss — two halves of one sentence a
whole card apart, with nothing saying they were the same sentence. And it is printed **once**:
on the directions whose reveal carries the sentence, the crib above the answer box stays off in
the answer phase, or the same machine translation stood on the card twice. Where no translation exists,
the same slot offers to fetch one: a pack translates 19,641 of its sentences and not all, and a
sentence the learner brought from their own reading has none until the sidecar has seen it, so
"no translation" has to be an offer rather than a dead end. Nothing is fetched until asked — the
queue itself still only ever reads the cache.

**«Тренировка» is a mode beside the three piles, not a fourth one.** It draws at random from
whichever pile you point it at, ignores due dates entirely, and is reachable from the start screen
(§4.5) at any time and from the done screen the moment the queue is empty or its daily ration
spent — which is when a learner asks for it. It shows `∞` rather than a count, because there is
no obligation behind it to count. The round's shape is chosen on the start screen: the pile,
**length (10/20/30)** and the **«слабые слова» mode**, which leads with the lowest-retrievability
words, weakest card per word. The criterion is printed — an explicit learner choice, not the
hidden agenda ADR-019 rejected — and the card's progress row repeats it as it was really applied:
on a deck with nothing reviewed it falls back to random and says so (ADR-024). The done screen's
«Тренировка» starts a round over the pile just reviewed, a text filter included, so the words
just mined from an article can be drilled on the spot.

**The per-text filter is a link, not a pile.** It arrives from the library card's or the reader's
«Повторить слова: N» button as `/review/session?scope=text:<id>` — straight to the card, because
on those screens the choice is already made — and the card's progress row reads «Только слова из
текста». A sub-slice of «Тексты» (ADR-022), so the three-pile arithmetic keeps adding up; the
start screen does not list texts, the library does.

Answering there **schedules nothing**: no due date moves, no feature is graded, no word gets
closer to «знаю». That is stated on screen, in a standing notice, for the whole session — this is
the one screen where an identical-looking card means two different things, and inferring the
difference from a due date that never moves is not something to ask of a learner. Practice
volume and accuracy get their own line on Прогресс, under the lemma curve and never folded into
the review count, with the same caveat repeated. Minutes count toward the daily target because
they are real work; the streak still needs the real queue cleared.
[ADR-019](../.claude/DECISIONS.md#adr-019).

**Burial lasts the DAY, not the page load.** Answering one direction of a word takes that word's
other directions off the table until tomorrow. Without it, a refresh served the same word again a
second later — with the answer still on screen in the learner's memory, so it tested nothing —
and because a reload also tops the session back up to the daily budget, the queue never shrank.
It read as the app having lost every answer. The card actually answered does still return, ten
minutes on: that is FSRS's learning step, not a sibling. `buryAnsweredSiblings`, pinned by
`queue_bury_today_test.go`, and gated on `review.burySiblings` like the rest of burial.

---

### 4.4 Sets — the day dealt out in bites, and a card that shows its own memory

```
   План · набор 2 из 5 · отвечено: 7 из 15                    тип карточки  ФОРМА В ПРЕДЛОЖЕНИИ
   ▓▓▓▓�▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░░░░░░░░░░░░░░
   [закрепляю]  ✓✓✗✓✓  эта карточка: верно 4 из 5
   ┌─────────────────────────────────────────────┐
   │  A ház___ lakom.                             │
   │  …                                           │
   │  ответ верный → вернётся через ~3 дн         │
   └─────────────────────────────────────────────┘
   [ Снова  ~1 мин ] [ Трудно  ~10 мин ] [ Хорошо  ~3 дн ] [ Легко  ~8 дн ]
```

**Seventy due cards were served as «1 / 70».** The first learner's word for it was
*exhausting*, and the spacing literature has nothing to say in defence of a seventy-card
sitting — what it does say ([R18](../.claude/research/18-sessions-and-progress.md)) is that a
sitting relearns each item to **one correct recall**, that a **near, visible finish line** is
one people reach, and that **measured progress is feedback** where points are rewards. So the
queue the server serves — unchanged, every number on it unchanged
([ADR-021](../.claude/DECISIONS.md#adr-021)) — is dealt out by the client in **sets**
([ADR-026](../.claude/DECISIONS.md#adr-026)).

- **Size and order are per-device chips on the start screen** (§4.5, ADR-028 — until
  2026-09-03 they sat above the bar), like practice round length (ADR-024): 10 / 15 / 20 / 30,
  default 15; «сначала просроченные» (the default — the most overdue card is decaying fastest)
  or «вперемешку», a random order that is **stable for the day** so a reload cannot reshuffle
  the remainder. Chosen before the sitting; changing one mid-sitting means going back to the
  start screen. Practice rounds are one set and read neither.
- **The bar is the set's**, not the day's: «набор 2 из 5 · отвечено: 7 из 15». A set is
  finished when every card in it has been answered once. **Nothing answered in a set comes back
  in it.** A miss returns through the server's queue once the learning step FSRS gave it has
  elapsed: the store fetches the served list again the moment a set finishes, so a card missed
  a few minutes ago is in the next set because the scheduler put it there, and one still
  maturing is under «сегодня вернутся ещё» on the done screen. The first version re-served the
  miss at the back of the set and waited with a countdown when the set ran out first; the
  learner did not understand «вернутся через 0:41» and asked for it to go
  ([ADR-026](../.claude/DECISIONS.md#adr-026), amended 2026-09-03). What a miss gets instead is
  a line to read: the set summary and the done screen **list the missed words with their
  answers**.
- **The memory row** under the direction hint says what this card knows about itself: the
  stage as a badge (новая · учу · молодая · закрепилась · заново — Anki's young/mature at 21
  days, named as a convention in the tooltip), the last eight outcomes as marks, and «эта
  карточка: 4 из 5». After the answer the badge and the marks move to the new state and
  «интервал 3 дн → 8 дн» appears. Every figure is measured or computed by the library; nothing
  on the row goes up on its own, which is the whole of the difference between this and a score
  ([R14], [R16 §3]). The **chance of recalling it now** used to sit on this row and was taken
  off (2026-09-03): on a card in its learning steps the model computes it against an interval
  of minutes, so it read «100%» over five crosses — right by the model, and to the learner a
  claim that they know a word they have just missed five times. It still stands in the words
  browser, where every card has graduated and the number means what it says.
- **«что это значит?» — what it takes for the word to stop coming back**
  ([ADR-039](../.claude/DECISIONS.md#adr-039)). The row was measured, honest and silent about
  the one question the first learner asked: «не сильно понятно, что надо сделать, чтоб слово
  закончить учить… а что делать, чтоб оно исчезло?» Read cold, «верно 0 из 9» over eight
  crosses looks like a score being lost. A disclosure at the end of the row — closed by
  default, remembered per device — answers it in five sentences: the stage and what it means
  (the hint that until now lived only in a `title=`); what a right answer and a miss schedule
  next, from the same preview the grade buttons use, while the card is still asking — in free
  practice, that **nothing here is scheduled** instead; that a card never ends but returns ever
  more rarely, which is what «выучено» means; that the tally is the review log and not points,
  with no «N in a row» threshold behind it; and that the one control which does remove a word
  is **«В корзину»** (`t`) — the whole word, and not a delete (§4.7). No bar, no denominator
  and no target: there is no such number, and inventing one is the thing [R14] and [R16 §3]
  rule out.
- **The verdict takes the answer box's place.** On a typed card the box is the question's; once
  the answer is in, «вы: ala / нужно: aztán» renders exactly where the box was — the eye is
  already there — followed by what the card additionally graded, and only then the sentence as
  context, for the directions whose prompt did not already show it. The first layout printed
  the sentence and its crib again above the diff, below a divider, and on a long card the
  correct form was under the fold behind the key strip: every wrong answer meant scrolling to
  find out what was wrong. The column is anchored at the top while a card is up, so the reveal
  grows downward instead of re-centring the prompt.
- **The grades carry their intervals** — «Хорошо ~3 дн», from `Scheduler.Preview`, marked
  approximate because fuzz is seeded at answer time — and a typed card says «ответ верный →
  вернётся через ~3 дн» under its box. Neither appears in practice, where nothing is scheduled.
- **Between sets: a summary of measured facts** — answered, right, missed (listed with their
  answers, to read over), «закрепилось» (the words whose interval crossed 21 days in this set,
  named), minutes — with «Следующий набор» on `space` and «На сегодня хватит». No praise line,
  no streak theatre; a two-note `setComplete` cue rather than the full stop, which still
  belongs to the empty queue. The learning-step auto-reload does **not** fire on this screen:
  the list was already fetched again when the set finished, which is how a matured miss is in
  the next set.
- **Today prints the day in sets** — «45 к повторению · 3 подхода по 15» — from the same
  per-device size.

**The crib says what language it is in, and offers the other** ([ADR-035](../.claude/DECISIONS.md#adr-035)).
The pack translates most sentences into English and few into Russian; a Russian-locale
learner used to get the English line unlabelled and no way to ask for Russian, because the
translate button was drawn only when there was no line at all. Now an other-language pack line
carries «(по-английски — русского перевода в пакете нет)», and «Перевести на русский» sits
under it whenever the translator is reachable; the machine's line is printed below the human
one with its model beside it, and both stay. The crib is keyed to the card, so a fetched
translation cannot survive onto the next sentence. The word breakdown (§4.1) shows its example
sentences through the same component, and every list row that shows a gloss in the other
language marks it «(англ.)».

**The stage chip speaks the learner's language, not Anki's.** «молодая» — a literal *young*,
Anki's term for a card under 21 days — read as a mistranslation to the first learner, and it
was one in spirit: nothing on the screen said what a card is young *at*. The chips are now a
sentence in three tenses — «новая · учу · закрепляю · закрепилась · заново» (*new · learning ·
settling · settled · relearning*) — and the tally under the marks says «верно 2 из 2» rather
than a bare «2 из 2». The tooltip still names the 21-day convention, once. The chip describes
the **card** — карточка, feminine — which is why it is «новая» and not «новое», and why the
`isNew` chip in the header (`review.newCard`) is the same word; the tooltip says «эту карточку»
so the noun is on the screen.

**The grade previews are read from the instant they were computed.** They are made when the
queue is served; measured against the clock instead, the 10-minute learning step read «~7 мин»
on a card reached three minutes later, the 6-minute one «~3 мин», and the learner asked,
reasonably, whether the maths was right. The maths was the library's and right; the display
was ours and drifting. `preview.at` travels with the figures, and an interval is `due − at`.
The gap the learner saw — «Хорошо ~10 мин» beside «Легко ~12 дн» — is real and by design:
on the learning steps *Good* is the next step, minutes away, and *Easy* graduates the card
straight to days from the model's initial stability for an easy answer. The tooltip says so.

**The key strip sits under the card, not under the viewport.** Fixed strips (`KeyBar`, the
milestone toast) centred on the window and so sat 112px right of the card they belonged to,
half a sidebar off. The shell exposes the rail's width as `--rail`, and the strips start
after it.

### 4.5 The start screen — what to take, before taking it

```
┌─ Повторение ──────────────────────────────────────────────────────────────┐
│  Что взять сейчас: стопку по расписанию или круг тренировки.              │
│                                                                           │
│  ╭─ По расписанию ─────────────────────────────────────────────────────╮  │
│  │  Всё      к повторению: 74 · 5 подх. по 15 · +72 новых  [▸ Начать ⎵]│  │
│  │  План     к повторению: 37 · 3 подх. по 15 · +32 новых  [ Начать ]  │  │
│  │  Тексты   к повторению: 37 · 3 подх. по 15 · +40 новых  [ Начать ]  │  │
│  │  ─────────────────────────────────────────────────────────────────  │  │
│  │  в наборе: 10 [15] 20 30   порядок: [сначала просроченные] вперемешку│  │
│  ╰─────────────────────────────────────────────────────────────────────╯  │
│                                                                           │
│  ╭─ Тренировка ∞ ──────────────────────────────────────────────────────╮  │
│  │  Свободная тренировка: случайные слова, в любой момент.             │  │
│  │  На расписание не влияет.                                           │  │
│  │  слова из: [Всё] План Тексты   слов за круг: 10 [20] 30  Слабые слова│  │
│  │  [ Начать круг ]                                                    │  │
│  ╰─────────────────────────────────────────────────────────────────────╯  │
└───────────────────────────────────────────────────────────────────────────┘
```

**The choice left the card on 2026-09-03.** Three decisions had each put a control strip above
the card — the pile tabs (ADR-016), «Тренировка ∞» with its rows (ADR-019, ADR-024), the set
chips (ADR-026) — and the learner's report was that
`Всё 74 +72 · План 37 +32 · Тексты 37 +40 · Тренировка ∞` does not belong over a card answered
two hundred times a day. Choosing what to take is a different moment from taking it
([ADR-028](../.claude/DECISIONS.md#adr-028)).

- **`/review` is the start screen; `/review/session` is the card.** Every «Начать» is a link
  carrying the choice in the query — `?scope=course`, `?practice=reader` — and the card starts
  what its URL asks for, on arrival and on a query change. Nothing else starts a session, so a
  reload is honest and any screen can make the same choice with a link.
- **The numbers are the card screen's own.** `GET /api/review/queue?scope=all` returns every
  pile's `Breakdown` (ADR-021) — due now, new in words, learning steps returning today — and
  the day in sets from the per-device size. Two numbers per pile, in Today's two colours, never
  one.
- **The raised object is the obligation.** The scheduled piles sit on the one raised card;
  practice is quiet beneath it — always there, owing nothing — with its pile, length and
  «слабые слова» chips, and the trash deliberately not among the piles (§4.3, ADR-025).
- **`space` starts «Всё»**, as it starts the day on Today. Today's own «Начать: Память» goes
  straight to the card, past this screen: that button exists so the learner does not have to
  choose. The sidebar row, the topbar due chip and the memory loop card come here; the library,
  the reader and the trash go straight to the card with their query.
- **The card still says what it is in.** The progress row names the pile («План», «Только
  слова из текста») and, in practice, the mode, the pile and the criterion really applied. The
  done screen says what the other piles hold in one sentence and links back here; its
  «Тренировка» starts a round over the pile just finished.

### 4.3 «Знаю» — taking a word out because you know it

The queue had no way to hear "no". A card the learner judged unusable — a sentence the miner cut
badly, a gloss that says nothing, a construction three months above them — could only be failed
forever or left to sit at the front of every session. Both answers teach the learner to distrust
the queue, and one of them poisons the heatmap: failing a card nobody could have answered writes
«снова» into `morph_state` for features the card never really asked about.

**A quiet control, below the ratings and away from them.** It is the rare action on a screen
whose four grading buttons are pressed two hundred times a day, and it must never be within a
slip of them. Offered in BOTH phases: a card can be judged unusable from the question alone, and
making the learner reveal an answer they do not want first would be the app insisting on an
interaction in order to accept a refusal.

**No confirmation dialog — an undo instead.** Trashing is reversible, and a dialog taxes the
common case to guard the rare one. What it owes instead is a way back that is visible without
going and finding a screen, so the next card carries a banner: «"lassan" отложено — из очереди
убрали», with «Вернуть». Restoring from there puts the word back in the DECK, not into the
running session — the queue was built before it left — and the banner says which, rather than
leaving the learner to wonder why the card did not reappear. It survives exactly one card: long
enough to change your mind, short enough not to become chrome.

**The counts update from the server, not from arithmetic here.** Trash and restore return fresh
`Breakdown`s, so the tab strip and the text chip stay in the one vocabulary every review number
speaks (ADR-021).

**`/trash`** lists the set-aside words — dictionary form, meaning, the sentence they came out of,
whether they came from the course or from reading, and when — with «Вернуть» per row and
«Тренировать отложенные», which opens a practice round over them. Reached from the banner and
from «Слова», never the sidebar: it is not part of any daily loop.

Two things it must be at once, and they pull against each other. It has to be a place a word can
be found again, or «убрать» is a delete wearing a softer word. And it must not become a second
deck the learner feels they owe something to — so it carries no due dates, no counts of debt and
no progress. The one line of copy at the top says exactly what a trashed word does and does not
do, because a learner who thinks this is a delete will not use it. See
[ADR-025](../.claude/DECISIONS.md#adr-025).

## 5. Read

```
┌─ Telex · Négy új metrókocsi… ──────────── покрытие: лемм 94% · элем. 87% ──┐
│                                                     ▔▔▔▔▔95──98▔▔▔        │
│  A BKV ma reggel bejelentette, hogy négy új      │ ┌──────────────────┐   │
│  metrókocsit állítanak forgalomba a hármas       │ │ bejelentette     │   │
│  metró vonalán. A járműveket a gyártó a múlt     │ │ ▸ be·jelent·ette │   │
│  héten szállította le, és a próbaüzem …          │ │                  │   │
│                                                   │ │ be- перфектив.  │   │
│  ██ известно  ██ учится  ██ новое  ░░ игнор      │ │    приставка     │   │
│                                                   │ │ jelent  сообщать │   │
│  ── n+1 (3) ──────────────────────────────────    │ │ -ette   past,    │   │
│  · A járműveket a múlt héten szállították le.     │ │    Def=Def, 3Sg  │   │
│    новое: szállít · Number=Plur+Def               │ │                  │   │
│                                                   │ │ 🔊 [парадигма]   │   │
└───────────────────────────────────────────────────┴─┴──────────────────┴───┘
```

- **The author's line breaks survive.** The reader renders the text as paragraphs, splitting on
  a blank line and keeping single newlines as line breaks. It used to emit one space between any
  two non-adjacent tokens, which flattened a pasted article into an unbroken wall. The gap is
  read out of the body by BYTE offset and decoded back, never guessed — offsets are UTF-8 bytes
  and JavaScript counts UTF-16.
- **The coverage panel says what it counts, in words.** It used to head both figures with
  «покрытие» — a term out of the lexical-coverage literature and exactly the jargon this project
  bans from learner-facing copy — print bare percentages with no denominator, stack the two in
  one shared track so they could not be told apart, and bury the third quantity at the tail of a
  caveat. `по леммам 14% · по элементам 14%` answered none of "14% of what", "why two", "what
  moves either".

  Now two named bars with counts — «Знакомых слов 111 из 785 · 14%» and «…из них с разобранной
  грамматикой 110 из 785 · 14%» — «не разобрано слов: 21» as its own line, and a disclosure
  carrying the mechanism. **The second figure needed explaining most**, because it is not
  intuitive: a word counts only when every feature on it is at `mining.knownThreshold`
  retrievability, and `morph_state` is written by exactly two things — cloze cards
  (`srs.Direction.GradesFeatures`) and the grammar drills. Reading, `recognize` and `produce` do
  not touch it, so a learner with hundreds of words in play and no drills behind them sees a low
  second figure that no amount of reading moves, and there was no way to work that out from the
  screen. It can also drift down on its own as retrievability decays. The 95/98 marks stay,
  still labelled as English-derived ([R6](../.claude/research/06-lexical-coverage.md)), but under
  the disclosure — they were crowding out the two sentences that answer the actual question. The
  computation in `internal/mining/coverage.go` is unchanged.
- **A word the pack does not have gets two ways out, and no dead buttons.** `aludtál` and
  `Kinyitom` resolve to no lemma, and the rail went on drawing «Знаю», «Игнорировать» and
  «В карточки» as live controls: the first two dropped the click on the floor (`setStatus` needs
  a `lemmaId`), and the third built a card blank on its back. The status buttons are now
  disabled with the reason — the rule `SpeakButton` already followed — and the rail offers
  **«Перевести слово»** through the local translator and **«Своё значение»**, a meaning you type,
  which needs no service.

  Both render **in the same shape as the pack's own «Значение»** — heading, numbered line —
  because to a learner they answer the same question, and a differently-shaped panel implied a
  lesser kind of answer. Both can be **kept**: `user_gloss` (migration 00009) is the learner's
  own dictionary, an overlay beside the read-only pack in the same way `morph_correction` is.

  **A saved machine line stays a machine line.** `source` is stored and re-rendered on every
  view, and such an entry does not reach a card: the back of a card is what the `produce`
  direction grades you against, so it has to carry something a person read and accepted.
  «Принять как своё» is that acceptance and re-saves the entry as `manual`. The rail says so
  rather than silently withholding the card.

  **Every word card reads it** — the reader's rail AND the review card's word inspector, through
  one component (`SavedGloss.vue`). The inspector once did not, so a meaning kept from the AI
  entry vanished the next time the same word was clicked. A badge beside «Значение» says whose
  line it is: «ваш словарь» (manual) or «машинное · <model>» (machine). While a meaning is kept,
  the AI entry's «Спросить» is not offered again — it would sit over the answer as if there
  were none.
- **The sentence's translation lives in the rail, under the button that asked for it**, not
  below the article where it used to render — a long way from where the learner clicked, and off
  screen entirely on a narrow window. The `t` key is **gone**: a keystroke at the bottom of the
  key strip went a release without being found, and one visible button beats a shortcut plus a
  line of documentation about it.
- **Word detail rail** shows the breakdown of *this exact form*, not just the dictionary entry.
- **`n+1` rail** surfaces sentences scoring in the sweet spot, with the unknown elements named.
- **Keyboard:** `←`/`→` word (`j`/`k` alias) · `Enter` add card · `Space` mark known ·
  `Backspace`/`Delete` ignore · `s` speak the sentence. Drawn by the key strip (§2).
  `K`/`i`/`a` remain as aliases; translation is a button in the rail, not a key — see above.
  The three triage keys **advance**, so a page is walked end to end from the home row; a
  modifier is never claimed (⌘← stays Back), auto-repeat moves but never triages, and with
  nothing selected `Space` still scrolls the page.
- **Audio is here, and it needs no server.** 🔊 on the selected word, on every `n+1` candidate,
  and on the whole sentence under `s` — the device speaks it
  ([ADR-012](../.claude/DECISIONS.md#adr-012)). **Read along** speaks each word as the selection
  lands on it: a device preference in `localStorage`, off until switched on, debounced so a held
  arrow key does not stutter the first syllable of every word it passes. **A one-letter word is
  spoken lower-case**: every engine announces a capital letter rather than reading it, so the
  definite article `A` at the head of a sentence came out as «nagy A» — "capital A"
  (`lib/speech.ts`).
- **Sentence translation, where the learner has deployed one.** `t` renders the selected
  sentence into the UI language via `services/mt`, and the panel names the model that wrote it —
  in the panel, not in a tooltip, because it is the only text on the screen that no person and no
  corpus stands behind ([ADR-014](../.claude/DECISIONS.md#adr-014)). Without the service the
  control is **not drawn at all** and `t` is not advertised in the key hint: the reader has to
  read as complete on the default install, which is the one where this is off. Walking the cursor
  out of the translated sentence drops the panel rather than captioning the next line with the
  previous line's meaning.
- An **"analysis is wrong"** link on the rail writes a `morph_correction` override.

### 5.1 The library

The shelf, built to stay usable as it grows: a two-column grid of cards — title, kind icon
(paste/page/subtitles, 🎧 when audio is attached), date, word count, the two coverage bars with
their reader names, and «Повторить слова: N» wherever the text has words in the deck (the
per-text scope, ADR-022). Past three texts a toolbar appears: title search, sort
(newest/level/coverage/title), level filter. The add form lives behind a header button and
opens itself only on an empty shelf — a library whose front page is a data-entry form reads as
a task, not a shelf.

**The level chip always names its author.** `B1 · оценка по частотам` (the ingest heuristic:
pack frequency bands ≥90% of running words, grammar-topic ceiling at ≥3 hits, a sentence-length
nudge), `· оценка модели` (the `services.llm` second opinion), `· вы указали` (the learner —
never overwritten by a machine). A text the estimator cannot resolve carries no chip at all:
«уровень не определён» is a supported outcome, and the filter can select exactly those.
ADR-023; the labelling rule is ADR-014's.

### 5.2 Words — «что выучилось»

`/words`, linked from the lemma total on Прогресс («Все слова →» — a total nobody can inspect
is a claim). Every deck word as a row: dictionary form, gloss, status badge, a **memory bar**
(the strongest card's retrievability right now, coloured by band — shown only once something
was reviewed, because a bar over nothing would be an invented measurement), next review date,
lapse count. Search goes through pack case-variants (never `COLLATE NOCASE`) and matches
glosses too; sorts include «сначала шаткие», which is the browsing twin of practice's weak
mode.

---

## 6. Listen

The three-pass protocol is enforced state, not advice ([R13](../.claude/research/13-shadowing.md)).

```
┌─ Hungarian with Sziszi · ep.14 ───────────────────────────────────────────┐
│  ① Вслепую  ─────  ② С текстом  ─────  ③ Shadowing                        │
│     ▲ вы здесь                                                            │
│                                                                           │
│  ▶ ──────●───────────────────────────────  4:12 / 11:38   0.8×  A⟷B      │
│                                                                           │
│  Транскрипт скрыт. Слушайте. Отметьте, где потеряли нить.                 │
│                                        [ потерял нить ]  [ дальше → ]     │
└───────────────────────────────────────────────────────────────────────────┘
```

Pass 2 reveals the transcript with unknown elements highlighted. Pass 3 scrolls it for shadowing
and optionally records. **Pre-check:** if element coverage of the transcript is below ~90%, a
banner warns the material is too hard and offers something easier — the brief's own rule.

**Dictation** is a sub-mode with character-level diff, which is where `a/á` and `ö/ő` errors
surface most sharply.

Playback uses `preservesPitch` so vowel quality is not corrupted at reduced speed.

---

## 7. Ear (HVPT)

Deliberately austere. This is a drill.

```
                         контраст:  ö / ő

                              🔊  (imre)

                    ┌──────────┐    ┌──────────┐
                    │   tör    │    │   tőr    │
                    └──────────┘    └──────────┘
                         ←               →

              23/40      точность 78%      дикторов: 6
```

- 2AFC, immediate green/red, both members replayable after the answer. **Both members are
  words** ([ADR-033](../.claude/DECISIONS.md#adr-033)): the inventory once paired a real word
  with a nonce one — `csak`/`sak`, `tuz`/`tűz` — and «как будто таких слов нет» was the
  learner's exact reading of it. The build now refuses a member the dictionary does not carry.
- **Talker badge** and a **distinct-talker count** for the session — because talker variability
  is the mechanism and we make it auditable ([R7](../.claude/research/07-hvpt.md)).
- **No completion badge, ever.** Contrasts return on an FSRS schedule indefinitely.
- **Keyboard:** `←`/`→` choose · `r` replay · `space` next.

**As built.** Below `hvpt.minTalkers` distinct voices the session is a REFUSAL SCREEN carrying
the numbers ("в этом паке 3 голоса, нужно 5") and the honest trade-off of lowering the bar in
settings — never a silently degraded session. The session plan rotates voices so consecutive
trials never share a talker. The session-done screen shows the numbers and then explicitly says
no contrast is ever finished; the contrast map uses dashes for never-trained contrasts, not
zeros. The played member never reaches the client until after the choice.

### 7.1 Listen — the three passes as UI state

A material = a transcript, and **optionally** an uploaded audio file. The transcript is
required — passes 2–3 and the coverage pre-check are built on it, and it goes through the
normal text ingest. The audio file is not: requiring one meant the loop could not start until
the learner had found a recording *and* a matching transcript, which nobody does, so on a fresh
install this screen was an empty list above an upload form. Without a file the learner's own
browser reads the transcript aloud, sentence by sentence, with the current sentence marked in
pass 2 ([ADR-012](../.claude/DECISIONS.md#adr-012)). The list labels which materials are
recordings and which are synthesised, and the player states plainly that a synthesiser is worse
input than a speaker. The pass switcher is the learner's
own claim, persisted server-side; the app never advances it. Pass 1 hides the transcript —
that IS the pass. Pass 3 is per-sentence dictation with a character diff, which is where missed
`á`/`ö` diacritics light up red. A/B loop, 0.5–1.5× with `preservesPitch`. Element coverage
below ~90% shows the honest warning before playback. Heard seconds are reported as they play
(clamped to 60 s per report — hours accumulate, they cannot be claimed) and feed both the
word-order listening gate and the daily ledger.

---

## 8. Grammar

```
┌─ Определённое и неопределённое спряжение ────────── A2 · месяц 3 ─────────┐
│                                                                           │
│  ⓘ Сейчас: блочная практика                                      78%      │
│    Мы намеренно не перемешиваем темы, пока точность ниже 80%.             │
│    Смешивание на раннем этапе мешает закрепить правило.  [почему?]        │
│    ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓░░░░  78% из 80% (окно 20)                          │
│                                                                           │
│  Látok egy házat.   ←  неопределённый объект                              │
│  Látom a házat.     ←  определённый объект                                │
│                                                                           │
│  ── упражнение 14/20 ────────────────────────────────────────────────     │
│  ___ azt a filmet tegnap.                                                 │
│      [ Láttam ]    [ Láttam ]  ← варианты минимально различны             │
└───────────────────────────────────────────────────────────────────────────┘
```

The **phase badge is load-bearing UX**: a learner who does not know why the app is holding back
interleaving will assume it is broken. `[почему?]` links to a short explanation citing
[R5](../.claude/research/05-interleaving.md).

**The lesson leads, and the exercises are entered deliberately.** Opening a topic raises the
LESSON and puts no exercise on screen at all; a button moves to the drills, which then take the
raised surface while the lesson stays above as a reference. This is [R4] applied rather than
cited: the screen used to raise the drill card and put it first, so opening a topic asked
`név + Case=Sup — какой вариант?` of someone who had not been told what a superessive was.

The lesson is assembled from three things the pack already carried and the app was not showing:
`grammar_topic.explain_md_*` as prose, `morph_feature.label_*` + `explain_*` for what each
feature IS, and `morph_feature.allomorphs` laid out by harmony class — which is literally what
the allomorph drill then asks about. `TopicLesson.vue`.

**Feature ids are never shown alone.** `Case=Sbl` is an identity, not a label. Every surface
that names a feature — the drill prompt, the graded-feature chips after an answer, the reader's
`n+1` rail and word detail — resolves it through the pack's own label table
(`stores/features.ts`), in the UI locale, with the raw id kept small beside it where two screens
need to be connected. A feature the pack does not know still renders as its id: honest and
debuggable.

**As built.** The badge always carries three things: the phase, the reason for it in plain
language, and a bar showing the distance to the next transition. The reason is an i18n key with
arguments, computed by the same pure function that set the phase — so the explanation on screen
is provably the rule the server applied, not a paraphrase. While the estimate is thin the bar
says so (*"ответов пока мало, судить рано"*) rather than implying a verdict; once past the
window it switches from *"3 из 20 ответов"* to *"скользящее среднее по последним 20"*, because
"28 из 20" reads as a bug.

Five phases, not four. **`explained`** is for a topic with a real rule card and no exercises
behind it — the alphabet, politeness, preverb detachment. It says so plainly (*"только
объяснение — упражнений пока нет"*) and does not gate what comes after it.

**Topic map.** `TopicMap.vue` renders the DAG grouped by CEFR level, in the pack's teach order.
The order is shown as a recommendation (*"Лучше сначала: винительный падеж"*) and every topic is
open: prerequisites stopped being locks in [ADR-013](../.claude/DECISIONS.md#adr-013), because
gating all of A1 behind 80% accuracy on a three-month motor skill is a wall, not a floor. The
two hard locks that remain — a failed checkpoint and the listening volume — still show a lock
icon and name themselves.

The demotion COUNT is deliberately not shown. Dropping back is normal, the phase badge announces
it when it happens, and a running tally of past fallbacks names no action a learner can take.

**Six exercise shapes**, `allomorph | case | conjugation | cloze | transform | mcq`. Choice
items are answered with `1`–`4` or the mouse; `cloze` and `transform` are typed. **The expected
answer is not in the payload** — the server holds it until the attempt arrives, which is what
makes this retrieval rather than recognition. After the answer, an option that is not a real
Hungarian word is marked, because *"-házben is not a word"* is a better correction than a red
cross.

**Keyboard:** `1`–`4` choose · `Enter` check, then `Enter` again for the next item.

### 8.2 The tutor — a question about the rule, answered in your language

```
┌─ Спросить репетитора ─────────────────────────────────────────────────────┐
│  Объяснение принадлежит модели; урок выше — приложению. Ничто из          │
│  сказанного здесь не становится карточкой.                                │
│  ┌────────────────────────────────────────────────────────────────────┐   │
│  │                       вы: Почему házban, а не házben?              │   │
│  │  Здесь работает гармония гласных. В `ház` единственный гласный —   │   │
│  │  `á`, задний, поэтому инессив берёт `-ban`: `házban` …             │   │
│  │  ответ модели qwen3:8b                                             │   │
│  │  Нет в словаре: házben — модель могла их выдумать.                 │   │
│  └────────────────────────────────────────────────────────────────────┘   │
│  [ Почему именно эта форма? ] [ Ещё три примера… ] [ Чем это отличается…] │
│  ┌ Спросите об этой теме…                                    ┐ [Спросить] │
└───────────────────────────────────────────────────────────────────────────┘
```

**Placed after the lesson, never before it.** [R4] is the order of this screen; the tutor
does not reorder it, and a question is asked about a rule that has been read. The panel is a
`quiet` card between the lesson and the drills ([ADR-027](../.claude/DECISIONS.md#adr-027),
[R19](../.claude/research/19-grounded-tutor.md)).

- **The answer streams.** A local model on a CPU writes a sentence at a time; an answer that
  landed whole after twenty seconds read as a hang. The thread scrolls inside the panel — the
  lesson above stays where the learner left it.
- **Every answer names the model** («ответ модели qwen3:8b») — the label is the defence
  ([ADR-014](../.claude/DECISIONS.md#adr-014)) — and every backticked Hungarian word the pack's
  dictionary does not carry is listed under it in the `uncertain` hue: «Нет в словаре: házben».
  An answer cut off — by the answer budget («закончился отведённый объём»), the timeout, a
  stop («ответ остановлен»), a lost connection, or the model reciting the prompt's own headings
  («модель начала повторять служебный текст», `tutor.EchoAt`) — is kept and marked with its
  reason, and it is an answer, never an error: `error` only means nothing arrived
  ([ADR-031](../.claude/DECISIONS.md#adr-031)).
- **A standing note** under the composer says whose words these are and that nothing here
  becomes a card; the sources sit behind «Справочный материал» beside it.
- **Four openers** in the empty log so the first question is one click; a composer after that
  (`Enter` sends, `Shift+Enter` breaks the line, `Escape` stops or backs out of an edit) that
  grows with its text. The panel is the learner's latest chat pinned to this topic — it
  survives a reload, and «Открыть в AI-учителе» opens the same chat on its page; «Удалить чат»
  removes it.
- **«Стоп»** replaces «Спросить» while an answer streams; what arrived is kept and marked.
  **«Изменить вопрос»** on the last question puts its text back in the composer, says the old
  answer and everything after it will go, and asks again — a chat, not a tree of them.
- **The context meter** under the composer: a thin bar and «Окно модели 8 192 токенов:
  последний вопрос и ответ заняли 3 850, свободно ≈ 4 342», from the server's own figures
  (usage on the stream, the window from Ollama's `/api/ps`). Prompt + answer reaching the
  window turns it into a warning — the head of the prompt, the rules and the lesson, was cut
  without a word — and names the fix. A server that reports no window gets the bare figures
  and «размер окна модель не сообщает»; nothing is ever printed as 0.
- **With no model configured** the panel says so and links to Settings — never a dead panel.
  **The panel froze mid-answer, blank until a reload, on every answer with no backticked
  word** — the commonest kind from a small model: the `done` event carried
  `"unattested": null`, the render threw on `.length`, and the stored copy read back after a
  reload as `[]` was why reloading "fixed" it. The list is never null now, the panel guards it
  anyway, and a thread that fails to load says so and offers a retry instead of rendering
  nothing (the error used to live inside the block that needs a thread to exist).
- The footnote names the reference material the tutor reads: the pack's lesson, its feature
  tables, and the open grammars in `internal/tutor/kb` ([`TUTOR.md`](TUTOR.md)).

### 8.3 The AI teacher — a chat, with the lesson found for it

```
┌ AI-учитель ──────────────────────────────────────────────────────────────────┐
│ [ + Новый чат ]        │ Почему házban, а не házben?          [🗑 Удалить чат]│
│ ТЕМА НОВОГО ЧАТА       │ тема определяется по каждому вопросу                 │
│ [Любая — определять…▾] │ ──────────────────────────────────────────────────── │
│ ЧАТЫ                   │                       ( Почему házban, а не házben? )│
│ ▸ Почему házban, а…  🗑│                                     ✎ Изменить вопрос│
│   по уроку: Локативн…  │ В `ház` гласный задний, поэтому инессив берёт `-ban`:│
│ ▸ Гармония гласных     │ `házban` — «в доме», `kertben` — «в саду» …          │
│   тема: Гармония глас… │ ответ модели qwen3:8b · по уроку: Локативная система │
│                        │                                                      │
│                        │ ┌ Спросите о венгерском…                 ┐ [Спросить]│
│                        │ ▓▓▓▓▓▓▓▓▓░░░░░░░░░░░                                 │
│                        │ Окно модели 8 192 токенов: последний вопрос и ответ  │
│                        │ заняли 3 850, свободно ≈ 4 342.                      │
│                        │ Объяснение принадлежит модели… Справочный материал   │
└──────────────────────────────────────────────────────────────────────────────┘
```

**The screen is the chat** ([ADR-031](../.claude/DECISIONS.md#adr-031)). No page title — the
top bar already says «AI-учитель» — and the conversation takes the viewport under it: the log
scrolls inside, the composer sits at the bottom, and under the composer, where every chat
product puts its "may be wrong" line, sit the honesty notice, the context meter and the
sources. Messages sit in a reading column; a question is a bubble, an answer is text. The
first learner's words: «окно чата очень маленькое, смотри на дизайн Клода или ChatGPT» — the
chat had been a box under a 3xl title, two notices and a footnote.

The page is a chat the way every chat looks ([ADR-029](../.claude/DECISIONS.md#adr-029)) — the
first learner's words for what it should be were «просто как AI-учитель и всё», with a new-chat
button and old chats to come back to, like the Ollama app they had just installed. What makes
it this app's chat and not a generic one:

- **A topic is an option on the new chat, never the gate to it.** Left on «любая», the server
  finds the lesson for each question — from the Hungarian words the learner quoted, from the
  topic's own words — and the answer says which: «по уроку: Локативная система» under it, or
  «по справочнику, без урока». The lesson panel pins its own topic, and its «Открыть в
  AI-учителе» lands on that same chat here.
- **The list is chats, titled by their first question**, most recent first, with the pin or the
  last lesson used as a subtitle and a delete control on each row. A review chat is titled by
  the WORD it is about — «Повторение · `jöttél`», the Hungarian half in its own face — with a
  repeat icon and «вопросы об этом слове» under it ([ADR-034](../.claude/DECISIONS.md#adr-034)).
  A new chat is not stored
  until its first question, so «Новый чат» pressed and abandoned leaves nothing behind.
- **The URL is the chat**: `?thread=ID` reopens it; `?topic=code` from a lesson resolves to the
  latest chat pinned there or to a new chat pinned to it.
- Everything the panel prints, this page prints: the honesty line, the model's name, the
  grounding, the unattested words, the sources footnote.

### 8.1 Placement

```
┌─ Определение уровня ──────────────────────────────────────────────────────┐
│  Гармония гласных · 2/12   ▓▓▓░░░░░░░░░░░░░░░░░░░░░░░                     │
│                                                                           │
│  vendég + Case=Dat — какой вариант?                                       │
│      [ vendégnak ]    [ vendégnek ]                                       │
└───────────────────────────────────────────────────────────────────────────┘
```

A short adaptive ladder up the DAG — three items per topic, stopping after two consecutive
failed topics. The verdict is shown **before** it is applied, per topic, with the number of
answers behind each decision, and the screen states plainly that nothing is marked mastered:
three items cannot show that.

### 8.1 Knowing where you are

**The step has a denominator.** `grammar.teachOrder` was «шаг {n}» — a bare ordinal — so
"A1 · шаг 20" left a learner unable to tell 20-of-28 from 20-of-40. It now counts within the
LEVEL, which is the unit the rail shows and the one a learner thinks in: «A1 · шаг 2 из 10».

**The rail lists the level's topics** with each one's `PhaseBadge`, the current row marked
`aria-current`, and a click moving between them without closing anything.

**The URL is the source of truth.** `start()` had always READ `?topic=` and nothing ever wrote
it, so the address bar described a topic you were no longer on, a reload put you somewhere else,
a shared link was a lie, and Back did not undo «Все темы» because nothing had been pushed. Now:
changing MODE — map to topic, topic to map — **pushes**, so Back returns you; moving BETWEEN
topics in the rail **replaces**, so browsing ten does not cost ten Back presses to leave; and a
`watch` on `route.query.topic` drives the screen, because pushes nothing listens to are worse
than no pushes at all.

---

## 9. Progress — the signature screen

```
┌─ Прогресс ────────────────────────────────────────────────────────────────┐
│                                                                           │
│  Леммы  1 847                          B2 ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄  6000    │
│  ▁▂▃▄▅▆▇                               B1 ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄  3500        │
│                                        A2 ┄┄┄┄┄┄┄┄●  2000                │
│                                                                           │
│  ── Локативная система ─────────────  ── Грамматические признаки ─────    │
│           где     куда    откуда       надёжно·4  шатко·2  мало данных·9  │
│  внутри   -ban    -ba     -ból                                            │
│            93%     88%     71%         ▸ Падежи        ответов: 61   71%  │
│  на пов.  -n      -ra     -ról           ████████████████░░░░░░░░         │
│            91%     64%     отв: 4      ▸ Наклонение    ответов: 24   92%  │
│  рядом    -nál    -hoz    -tól           ██████████████████████░░         │
│            отв: 2  не было  не было    ▸ Число         ответов: 4         │
│                                          мало данных                      │
│  Цвет — доля верных ответов.                                              │
│  Пунктир — ответов пока мало.          ▲ Требует внимания:                │
│  Серое — форма не встречалась.           делатив — с чего 64%,            │
│                                          условное наклонение 71%          │
└───────────────────────────────────────────────────────────────────────────┘
```

The **locative grid** and the **feature heatmap** are what no other tool can render, because no
other tool tracks features separately. This screen is the product's argument for itself.

**As built.** The lemma curve is cumulative from `lemma_state.first_seen`, with the A1–B2
month-milestone targets as dashed reference lines labelled as reference points, not judgements.

**The heatmap measures ACCURACY, not retrievability, and says so when it has measured
nothing.** Retrievability is the probability of recalling something *right now*: it is ~1.0 for
anything reviewed recently, so a learner a few days in met nine families at 100%, read the
screen as broken, and was right to. It also tracks the calendar rather than the learner — a
week away would have looked like the grammar collapsing, though nothing was forgotten and no
answer was given. The bar is the rolling accuracy, drawn only once the server's `ready` flag
— `srs.GateReady` against `drill.gateWindow`, the same line the interleaving gate draws — says
the figure rests on evidence; below it a row states its answer count and the words
«мало данных» rather than inventing a percentage. Colour has two earned states and one
unearned: held, `weak` (the server's verdict against `drill.masteryThreshold`), and unmeasured.
Nothing on this screen re-derives either threshold: `(*Server).mastery` reads both from config
once, so the screen and the scheduler cannot quote different numbers.

**Families are named, and open.** «Падежи», «Определённость объекта» — never `Case` or
`Number[psor]` — and each expands to the individual features under it, by the label the pack
ships in both locales. A feature id survives only in a `title`, for debugging. The
«Требует внимания» line names weak features the same way; it previously printed
`Mood=Ind, Number=Sing` beneath a column of 100% bars, so the card contradicted itself in two
adjacent elements.

**The locative grid distinguishes "not measured" from "not met".** Both used to render as an
em dash, so seven untouched cells read as breakage. A met-but-unmeasured cell is dashed and
carries its answer count; a never-met cell says «не было» in words.

**A criterion not yet reached is not a failure.** It shows an empty circle and a progress bar,
not the ✗ the history list uses for a genuinely failed checkpoint. The checkpoint panel shows
the NEXT assessment
with live evidence before it is taken — the measured criterion labelled "измерено приложением",
the human-verified ones labelled "ваше собственное подтверждение — приложение проверить это не
может". A failure prints the three-failure-mode diagnostic with the numbers that triggered each
finding, and locks curriculum levels above the checkpoint until a retake passes.

### 9.1 Today — the ledger made visible

The header carries the exam countdown ([R14](../.claude/research/14-motivation.md): the
pre-booked exam is the defence against the month-4–7 trough) and the streak flame. The
obligation bar states the rule it applies: minutes against the stated target, queue cleared or
not. The four loop cards show live counts from today's ledger. Loop 4 has an attestation button
("сегодня было") that the streak deliberately ignores — the card says why the app cannot count
this loop for you.

Also here: review forecast, checkpoint history, daily-minutes adherence, and the honest
statement that the lemma targets come from the brief and are estimates.

---

## 10. Settings

Sections, in order: **Review** (desired retention, new-card limit with the governor curve,
direction weighting by goal) · **Grammar drills** (the competence gate, exposed on purpose) ·
**Curriculum** (the exam date) · **Language** (UI locale, gloss language, fallback behaviour) ·
**Ear trainer** (the talker floor and trials per session — the control the HVPT refusal screen
has always told people to come here and change, and which did not exist) ·
**Services** (TTS/ASR/NLP/LLM provider, endpoint, key, test button, latency) ·
**About** (version, pack version and edition) · **Speech** and **Sound**, which sit outside the
settings payload because they are device preferences, not account ones.

**Save is admin-gated on every card, not some of them.** `PUT /api/settings` is admin-only, but
only HVPT and Services said so; Review, Curriculum and Language drew an enabled button that
answered with a bare 403. All five are now disabled for a learner, with one line at the foot of
the screen saying why — gating without a reason reads as a broken control.

**The `tts` row no longer contradicts the Speech card.** With `tts.provider: off` it said
«не настроено» while the card a few hundred pixels below named a working voice, because the two
are about different things: the row is a TTS *service*, and Hungarian speech comes from the
browser's own synthesiser ([ADR-012](../.claude/DECISIONS.md#adr-012)). Where the device has a
voice, the row now says «говорит устройство» — one screen, one answer.

**The `llm` row verifies the model, not just the socket, and the row grew a Model field.**
A fresh Ollama lands with the endpoint filled and the model blank, and Ollama refuses a request
that names no model — so the tutor failed with «модель не ответила пригодным образом» while this
row said «на связи», and nothing on the screen pointed at the cause. The probe now asks a
reachable language-model endpoint what it serves (`GET /v1/models`); the row reads «укажите
модель» while the field is blank and «такой модели нет на сервере» for a name the endpoint did
not list, and the field offers the list as a datalist — type-ahead, because not every server
answers the route and a name the row cannot verify is still allowed. The tutor panel says the
same thing (`errors.llmNoModel`) with its link to this screen.

**Services** gained a **translator block**, because the generic row cannot describe this one.
Every optional service is probed the same way — `GET /health`, under 500 is reachable — and for
`mt` that flattens three different situations into one grey «нет связи»: never deployed, model
downloading, download died. It has a long and legitimate not-ready state (2.4 GB of weights, and
503 for the minutes that takes). The block draws four states with the remedy that fits each: the
one command that deploys it, a real progress bar, the service's **own** failure text, or
"ready". «Скачать модель» retries a dead download without a container restart, and the row above
it defers to this status so one screen cannot hold two answers. It stops at starting the
container — the core has no shell and no Docker socket, and that boundary is printed rather than
papered over. [ADR-015](../.claude/DECISIONS.md#adr-015)

**Speech** names the voice, offers the picker whenever the device has **any** Hungarian voice —
the old `> 1` guard meant a stock macOS, which ships exactly one, never drew the control at all
— and answers the complaint the card actually gets. "It sounds robotic" is almost never the
browser: every desktop ships a small *compact* voice per language and hides the good one behind
a download, and `Tünde` and `Tünde (Enhanced)` then sit side by side in the list with the
compact one usually first, so picking by list order picked the worst voice installed.
`pickVoice` now prefers a quality build over list order, the card prints the OS-specific path to
install one, and until there is one the app reads at 0.9× — a compact voice smears the joins
between units, and the joins are where Hungarian vowel length lives.

**Speech** also reports what this machine can actually do: the voice in use, or the reason there is
none with the OS-specific fix. Which voices exist is a fact about the device, so the answer
cannot live in an account — and the server, which cannot see the browser, does not claim it
([ADR-012](../.claude/DECISIONS.md#adr-012)).

There is **no Data section and no Profile section**. Data went with the export
([ADR-011](../.claude/DECISIONS.md#adr-011)) — the pack version and edition it used to carry
are in About. Profile is a single control, "change my password", which lives on the
**Accounts** screen beside the account it changes; account management is that screen,
admin-only in the sidebar except for that one card.

The **new-card limit** control is where the governor lives: raising it past what the learner's
stated minutes can sustain renders the 90-day projection first, and requires an explicit
override.

---

## 10.1 Settings → Данные — moving a profile

[ADR-040](../.claude/DECISIONS.md#adr-040). One card, outside the admin-gated block: a profile
belongs to the learner, so every account has this and it survives a settings load failure.

**Export** is a plain link, not a fetch — the browser's own download machinery handles an
unbounded zip better than anything we would write around it. Beside it, the counts of what the
profile contains, in the learner's units: «слов: 412 · карточек: 1236 · ответов: 5104».

**Import** is the most destructive thing a learner can ask for, so the confirm names by count
what it will erase and says there is no undo. It also says the app will stop responding while it
runs, because SQLite here is single-writer and a big profile is a few seconds of a frozen UI —
an unexplained freeze reads as a crash.

The **report** afterwards states the rows, the media files, and the integrity check: «Журнал
ответов проверен: 20 карточек переиграны заново и сошлись с сохранённым состоянием.» And that
the API keys did not travel, which is the one thing the learner has to act on.

## 10.2 Activity by day — where the hours went

[ADR-044](../.claude/DECISIONS.md#adr-044). Replaces the flat 30-bar strip on Today, which drew
`active_ms` as one undifferentiated height and so could not answer the only question anyone has
about it.

**The axis is a calendar, not a list of rows.** `daily_log` only has rows for days something
happened on, so mapping the response straight to bars drew N bars for N rows and spread them
evenly across a month: a fortnight of study rendered as a solid block against an empty left
margin while the axis still claimed a full month. The chart silently closed up the very gaps it
existed to show. Every day in the window now gets a slot, and a day with nothing on it gets a
2px mark on the baseline — an absent bar is indistinguishable from a bar too short to see, and
the tooltip says «занятий не было» rather than three zeroes.

**Opens on 7 days**, switchable to 30 and 90 without a round trip: the server sends 90 rows by
default and the chart slices them, because refetching to change a window made the switch feel
like a page load. A day with no row reports neither half of the obligation as met — a missing
row is not a cleared queue, and «очередь пуста» about a day nobody opened the app would be
inventing a fact.

Three **stacked** series, all minutes, all measured: cards (reviews, free practice, drills),
reading, listening. The dashed line is the learner's own `dailyMinutes` — on the same axis
because it IS minutes, a threshold and not a second measure.

**There is no second y-axis and `due_left` is not a line on it.** Cards-remaining and
minutes-spent are different scales, and one axis for both invites exactly the comparison that
means nothing. The queue half of the day's obligation is the thin status row under the bars:
met / not met, a state.

Three things keep it a record rather than a score: nothing accumulates across the window, the
only target is the one the learner set, and where a day failed the tooltip names **which half**
failed — they fail independently, and «не выполнена» on a day whose queue was clear pointed at
the wrong thing.

The series use their own validated categorical steps rather than the knowledge-state colours.
`primary` measures chroma 0.072 and reads grey as a fill; `kn-known` is doing status duty here
for "the day counted". Light and dark are each stepped and validated against their own surface —
see the comment in `assets/tailwind.css`, which carries the validator command.

## 10.3 The streak, openable

[ADR-044](../.claude/DECISIONS.md#adr-044). The flame was a number with a tooltip, and a learner
could not tell what a day required, why today had not counted, or how the week had gone.

It opens on: the week Mon→Sun as seven cells (filled where the day counted, ringed on today,
empty for days that have not happened); the count; and **both halves of today's obligation,
separately** — «Очередь: осталось 21» / «Минуты: 1 из 90».

**No freeze and no repair.** Duolingo's week strip is a legible display of a rule and is worth
borrowing; its freeze is a purchasable forgiveness item, which is the point economy
[R14](../.claude/research/14-motivation.md) rules out. A missed day is missed.

The five dots beside it are **not** progress — they are optional-service status, and they now
carry a label saying so, because the first learner read them as a week.

## 11. Colour semantics

Used consistently everywhere; defined once as Tailwind tokens.

| Meaning | Token | Used for |
|---|---|---|
| known / correct | `--kn-known` | known words, correct answers, mastered features |
| learning | `--kn-learning` | in-progress words and features |
| new | `--kn-new` | unseen words, locked topics |
| ignored | `--kn-ignored` | proper nouns, deliberately skipped |
| uncertain | `--kn-uncertain` | uncertain morphological analysis, fallback gloss, unattested LLM output |
| error | `--kn-error` | wrong answers, failed checkpoints |

**Colour is never the only channel.** Every state also carries a shape, an icon or text —
required for colour-blind users and for the ~8% of men with red/green deficiency who would
otherwise find the reader unusable.

---

## 12. Accessibility

- Every interactive element is keyboard-reachable with a visible focus ring. Tailwind v4 resets
  buttons to `cursor: default`; reuse `buttonVariants` rather than hand-rolling `<button>`.
- `prefers-reduced-motion` disables all transitions and the progress-ring animations.
- Audio is never the sole carrier of a correctness signal — HVPT feedback is colour **and**
  icon **and** text.
- Contrast targets WCAG AA in both themes; the knowledge palette is verified against
  deuteranopia and protanopia simulation.
- All copy flows through `vue-i18n`. **A PR adding an `en` key without its `ru` counterpart
  fails review** ([ADR-004](../.claude/DECISIONS.md#adr-004)).
- Screen-reader labels on the heatmap cells state feature name and mastery percentage, because
  a colour grid is otherwise opaque.

---

## 13. Offline (planned)

**Not shipped in v1.0.** There is no service worker, nothing is precached, and every screen
needs the server; the README and ARCHITECTURE.md said otherwise for a while and no longer do
(ADR-038). What the brief asks for — the review queue and its audio on the metro, answers
queued locally and synced on reconnect, last-write-wins per card since one learner cannot
conflict with themselves — is the next phase after the release, with its own ADR when it
lands. Reading, grammar and the ear trainer would stay online in that phase too.

---

## 14. Design system

Six primitives — palette, type, motion, sound, elevation, and the panel the app says "no" in —
plus the boundary around the thing people expect from a learning app and will not find here.
Everything below lives in
[`web-ui/src/assets/tailwind.css`](../web-ui/src/assets/tailwind.css),
[`lib/motion.ts`](../web-ui/src/lib/motion.ts), [`lib/sound.ts`](../web-ui/src/lib/sound.ts),
[`components/ui/card/index.ts`](../web-ui/src/components/ui/card/index.ts),
[`components/common/Note.vue`](../web-ui/src/components/common/Note.vue)
and [`components/common/Milestone.vue`](../web-ui/src/components/common/Milestone.vue).

### 14.1 Palette

**How the numbers were obtained.** Each `oklch()` triple is converted to linear sRGB through
the Oklab matrices, gamma-encoded to sRGB, clamped as a browser would clamp it, then run
through WCAG 2.1 relative luminance and `(L₁+0.05)/(L₂+0.05)`. Ratios below are therefore
measurements of what is actually rendered, not of an out-of-gamut intention. **Text and any
colour carrying state ≥ 4.5:1. Boundaries ≥ 3:1.** The same figures are repeated in a comment
beside each token block so a reader never has to open this file to check one. Since the
light-first flip (§1.7), `:root` in `tailwind.css` carries the LIGHT values and `.dark` the
override — the measurements themselves are unchanged, only which block is the default.

Dark — against `--background` #0a0e12 / `--card` #13181e / `--muted` #21272e:

| Token | vs background | vs card | vs muted |
|---|---|---|---|
| `foreground` | 17.80 | 16.42 | — |
| `muted-foreground` | 9.05 | 8.35 | 7.02 |
| `primary` | 10.16 | 9.38 | — |
| `primary-foreground` on `primary` | 9.98 | | |
| `destructive` | — | 5.27 | — |
| `border` | 3.31 | 3.05 | — |
| `input` | 4.00 | 3.70 | — |
| `ring` | 10.16 | — | — |
| `kn-known` | 9.28 | 8.56 | 7.20 |
| `kn-learning` | 10.25 | 9.46 | 7.95 |
| `kn-new` | 7.79 | 7.19 | 6.05 |
| `kn-ignored` | 6.25 | 5.76 | 4.85 |
| `kn-uncertain` | 8.00 | 7.38 | 6.20 |
| `kn-error` | 6.67 | 6.15 | 5.18 |

Light — against `--background` #fbfaf7 / `--card` #ffffff / `--muted` #f1f0ec:

| Token | vs background | vs card | vs muted |
|---|---|---|---|
| `foreground` | 16.58 | 17.31 | — |
| `muted-foreground` | 6.53 | 6.82 | 5.98 |
| `primary` | 6.29 | 6.56 | — |
| `primary-foreground` on `primary` | 6.38 | | |
| `destructive` | — | 6.15 | — |
| `border` | 3.04 | 3.17 | — |
| `input` | 3.78 | 3.94 | — |
| `ring` | 6.29 | — | — |
| `kn-known` | 5.28 | 5.51 | 4.83 |
| `kn-learning` | 5.16 | 5.39 | 4.73 |
| `kn-new` | 5.39 | 5.62 | 4.94 |
| `kn-ignored` | 4.95 | 5.16 | 4.53 |
| `kn-uncertain` | 5.73 | 5.98 | 5.25 |
| `kn-error` | 5.86 | 6.12 | 5.37 |

**What was actually wrong.** The audit found four failures, not a general dimness:
`border` and `input` measured **1.31–1.53:1** in both themes — that, not the greys, is why the
UI read as flat; light `kn-learning` measured **3.65:1** and light `kn-known` **4.44:1** as
text; `kn-ignored` measured **3.59–4.08:1**. `muted-foreground` already passed (7.63 dark,
6.45 light) and was not the problem it looked like.

**Three rules the values now obey.**

1. **Borders are opaque.** They were `oklch(1 0 0 / 12%)`. An alpha border's contrast depends
   on whatever is behind it, so it cannot be stated once and verified — and it measured
   1.33:1. Opaque values can be checked against every surface we ship.
2. **Two border tiers.** `--border` is the ≥ 3:1 tier and is the only one allowed to bound a
   control (outline button, input, tab). `--border-subtle` (1.50 dark / 1.46 light) is for
   decorative rules *inside* a surface, where 3:1 turns the screen into a wireframe. WCAG
   1.4.11 asks 3:1 of what identifies a component, not of every line drawn; splitting the
   tiers makes that judgement reviewable instead of implicit. A control is never identified by
   `--border-subtle` alone. This is the one added token name; nothing was renamed.
3. **De-emphasis comes from chroma, never from contrast.** `ignored` is the quietest state
   because it is grey (C = 0.02), not because it is dim. Anything a learner must read stays at
   4.5:1 whatever it means.

**Identity — a cold mineral teal, oklch hue 198.** The knowledge scale already owns green
(known, 150), amber/ochre (learning, 75–80), blue (new, 258), violet (uncertain, 305) and red
(error, 25). The old primary sat at hue 250 — the same colour as *new*, so a primary button and
an unseen word were the same signal. The accent has to occupy a hue none of the six states use;
the widest unclaimed arc is 150→258 and 198 sits near its midpoint, ≥ 48° from either
neighbour. It is Hungarian the way the thermal-bath and Zsolnay glazes are Hungarian — a
material, not a flag. No flags, no paprika. Surfaces carry a trace of chroma with it: ink-blue
in dark (C ≈ 0.012–0.03 at hue 255), warm paper in light (C ≈ 0.004–0.006 at hue 95), both far
below where they read as "a colour", so the greys look chosen rather than defaulted.

Two honest consequences. In light theme *learning* is **ochre, not amber** — amber cannot reach
4.5:1 on paper at any chroma worth having, so the hue survives and the lightness does not. And
chromas are capped at the sRGB gamut boundary (light `primary` 0.08, `kn-known` 0.139,
`kn-learning` 0.112), because a value outside the gamut is silently clamped by the browser and
the ratio in the table would then describe something we never shipped.

**Colour-blindness — the palette is not safe on its own, and cannot be.** Simulated with
Machado 2009 at severity 1.0: under deuteranopia `known`/`error` land 0.04 apart in Oklab, and
under protanopia `known`/`learning` land 0.05 apart — i.e. indistinguishable. Six semantic
hues cannot survive dichromacy. This is the measurement behind §11's rule: **colour is never
the only channel** is a hard requirement, not a nicety.

### 14.2 Type

12 · 14 · 16 · 18 · 20 · 24 · 30 · 36 px, mapped onto `text-xs` … `text-4xl`.

Two ratios on purpose. The body range (12→20) steps at ≈ 1.125, because one card here routinely
stacks label, value, unit and caveat and four levels of hierarchy must fit between 12 and 20px
without any two reading as the same size. The display range (24→36) steps at ≈ 1.2–1.25,
because a heading has to win instantly rather than gracefully. Every line-height is a multiple
of 2px so baselines still align when two columns use different steps. Tracking is negative at
display sizes to answer Inter's optical looseness when scaled up, and slightly **positive at
12px** so that ő and ű — which Hungarian uses constantly — keep their double acutes clear of
the following glyph.

**Two faces** ([ADR-036](../.claude/DECISIONS.md#adr-036)). **Inter** is the instrument —
buttons, labels, figures, everything the learner operates. **Literata** (OFL, variable, made
for reading on screens; Latin-Extended and Cyrillic, so a Russian heading and a Hungarian
sentence share one face) is the *material* and the *headings*: the sentence on the card, the
text in the reader, the page titles. The first learner's word for one sans over everything was
«шаблонный», and the fix they asked for was character where the language is. Fraunces was
tried first and has no Cyrillic. Both faces are bundled subsets (`assets/fonts.css`): Inter
latin, latin-ext, cyrillic, cyrillic-ext; Literata latin, latin-ext, cyrillic — nothing loads
from the network.

Hungarian is **marked, never classed**: `lang="hu"` on the element, and one base rule in
`tailwind.css` sets the serif at `1.0625em` with optical sizing; a nested `lang="hu"` inherits
rather than compounding, and the answer box stays in Inter because its diff is set in mono
beside it. The page title is one utility, `page-title` — the display face at the 30px step,
weight 500, no tracking (the serif carries its own contrast; the semibold and the negative
tracking were Inter's) — used by `PageHeader.vue` and every view's `h1`; the wordmarks and the
reader's text title use `font-display`. The favicon is a paper-white serif «ő» on the teal.

`--font-mono` is a system stack (no bytes added to the binary) and is the face for morpheme
splits, character diffs and paradigm tables: `ház·ban` above `ház·ben` only makes its point
when the columns line up.

### 14.3 Motion

Seven movements, defined as `--animate-*` theme tokens, so they are ordinary Tailwind
utilities: `animate-answer-correct` (180ms) · `animate-answer-wrong` (300ms, a damped ±4px
shake) · `animate-card-in` (160ms) · `animate-card-out` (120ms) · `animate-tick-up` (240ms, a
value counting up) · `animate-phase-change` (260ms, the interleaving badge) ·
`animate-milestone` (320ms). Durations are mirrored in `lib/motion.ts` for the few things that
must be timed in script.

Rules:

- **120–320ms, no exceptions.** A learner answering 200 cards a day sees `answer-correct` 200
  times; anything longer is a tax collected 200 times.
- **Transform and opacity only.** Both are compositor-only, so an animation can never contend
  with the keystroke that started it. Never animate width, height, top or left.
  **One documented exception: chrome.** The sidebar's collapse transitions `width` (200ms, with
  the labels fading). The rule above exists for the answer loop, where an animation runs two
  hundred times a day against a keystroke; collapsing the rail is a rare, learner-initiated
  layout change, and moving the whole shell by transform would detach it from the document flow
  it exists to reshape. The global reduced-motion rule still flattens it. Content never gets
  this exemption.
- **Motion never delays input.** The next card is interactive on the frame it mounts; the
  animation decorates a state that has already changed. Anything overlaying content sets
  `pointer-events: none`.
- **Small amplitudes** — 4px of shake, 6px of rise, 1% of scale. An instrument acknowledging an
  answer, not an app congratulating you. The wrong-answer shake is deliberately gentle: the
  learner is wrong roughly a fifth of the time by design, and a violent animation on a fifth of
  answers teaches avoidance.
- **`prefers-reduced-motion` is honoured by one global rule** at the bottom of `tailwind.css`,
  not by per-component discipline — a guarantee that depends on every future component
  remembering is not a guarantee. Every animation is decorative: each one accompanies
  information that is also present in text, colour or an icon.

### 14.4 Sound

Five sounds — `correct`, `wrong`, `milestone`, `sessionComplete`, `setComplete` — **synthesised at runtime**
from oscillators and gain envelopes in `lib/sound.ts`. No audio files ship: nothing to licence,
nothing to download, nothing to embed in the single static binary, and no CDN for a self-hosted
install to depend on.

[R1] The testing effect grows when feedback follows the retrieval attempt
([01-retrieval-practice.md](../.claude/research/01-retrieval-practice.md)). That is what these
are — feedback, not reward. Nothing is scored and nothing is unlocked by hearing one.

Design, all of it a consequence of repetition:

- **Under 250ms**, so the sound is over before the next card is read.
- **Sine and triangle only.** Sawtooth and square carry odd harmonics that turn abrasive by the
  fiftieth repetition.
- **Quiet — peak 0.07 of full scale — and `wrong` is quieter than `correct`, not louder.**
- **One consonant set** (D5/A5/D6, plus G4→E♭4 for `wrong`), so two sounds landing close
  together are a chord and not a clash. `sessionComplete` is a descending fifth: a full stop.
  `setComplete` is D5 rising to A5, quieter and shorter — a comma, for a set finished with more
  to follow ([R18]).
- The `AudioContext` is created lazily on the **first user gesture** (browsers block it
  otherwise) and `resume()`d whenever it is found suspended.
- A module-level flag persisted in `localStorage` (`tanulom.sound`), **default on**, surfaced
  in the UI store as `ui.sound` and toggled in Settings. Turning it on plays the tone, which
  doubles as the gesture that opens the audio context.
- `prefers-reduced-motion` is **not** consulted. Sound is a separate channel and gets a
  separate preference.

### 14.5 The gamification boundary

[R14](../.claude/research/14-motivation.md) is specific: gamification reliably improves
motivation, autonomy and relatedness, and has **minimal impact on competence**. So the app takes
the motivational mechanics and refuses the substitution
([R16](../.claude/research/16-debunked.md) §3).

**Built:** immediate feedback on every retrieval (the sounds, the colour, the morpheme diff);
visible *real* capability growth (the lemma curve, the feature heatmap, and — per card — the
memory row of §4.4: recall chance, stage, history, the interval a grade would give, every figure
computed by the scheduler's own library); a set summary of measured facts between sets; and a
milestone moment when something genuinely real happens — a curriculum unit finished, a grammar
topic reaching `mastered`, a checkpoint passed. [R18 §4] draws the line: a number that
*measures* something is informational feedback; a number that only ever goes up is a reward.

**Deliberately not built, and this absence is a decision, not an oversight:** XP, points,
scores, levels, ranks, tiers, leagues, leaderboards, gems, coins or any invented currency,
streak freezes and repairs, lootboxes, combo multipliers, daily-login celebrations. Each is an
extrinsic economy — the part most likely to crowd out the real goal, and the part with no
competence evidence behind it. The list is repeated at the top of `Milestone.vue` so the next
person to open that file reads the refusal before they read the code.

`Milestone.vue` is a transient overlay driven by `stores/milestone.ts`. It takes a title, an
optional subtitle and a **required** `fact` — the measured statement behind the moment
("юнит 3 из 24 — 32 слова в колоде"). Required in the type, because a milestone with nothing
checkable behind it is a reward. If you cannot state the number, you do not have a milestone.
It never blocks input (container and panel are `pointer-events: none`; only the dismiss button
takes pointers, and focus is never stolen), it self-dismisses, it announces itself with
`role="status"` / `aria-live="polite"` rather than interrupting, and it honours both the
reduced-motion rule and the sound setting. One milestone at a time — a second replaces the
first rather than queueing, because a stream of celebrations is what makes each one cheap. The
streak is not celebrated here: it lives on Today, quietly, counting obligations met.

### 14.6 Elevation

**Hierarchy is carried by the boundary, not by the fill.** The obvious way to raise a surface is
to lighten it; measured, that is the wrong move. `border` against `--popover` — one step up from
`--card` in dark — is **2.84**, and against `--muted` **2.57 / 2.78**. All are under the 3:1 the
boundary tier promises, so every outline button, input and tab on a lightened surface quietly
stops meeting WCAG 1.4.11. Lightening the fill buys about 0.03 of perceived elevation and costs
0.2 of contrast on every control standing on it.

So every tier keeps the `--card` fill and each ratio already measured for `card` (foreground
16.42 / 17.31, `border` 3.05 / 3.17, dark / light) holds everywhere. Four tiers, as
`cardVariants` in `components/ui/card/index.ts`:

| tier | boundary | measured | use |
| --- | --- | --- | --- |
| `raised` | `border` + `shadow-raised` | 3.05 / 3.17 | **one** primary object per screen |
| `default` | `border` | 3.05 / 3.17 | anything you can click |
| `quiet` | `border-subtle` | 1.50 / 1.46 | anything you only read |
| `inset` | `--background` fill | `border` on it 3.31 / 3.04 | a well cut into the plane |

The rule that makes this a hierarchy rather than a palette: **a card you can click keeps the
3:1 boundary** — it is a component and 1.4.11 applies to it — **a card you only read drops to
the decorative tier**. Before this, every card was bounded at 3:1, which is exactly why the app
read as one flat plane: thirty objects all shouting at the same volume say nothing.

`--elevation-raised` is defined per theme: near-black and wide in dark, because on a
0.16-lightness ground a shadow tuned for paper is invisible; tinted with the ink-blue of
`foreground` in light, because a neutral-black shadow greys warm paper.

`stagger-in` deals `card-in` down a list at 25ms intervals, stopping at the eighth child so a
24-unit syllabus does not take two seconds to become legible. It is not an eighth movement — it
is the one that already exists, applied in sequence.

### 14.7 The calm panel

The product is full of statements it is a design decision to make: a topic with no drill
generator, an ear session refused below the talker floor, a coverage warning, words the pack has
no attested sentence for, a placement test that will not call three items mastery. Rendered in
`uncertain` or `error` from edge to edge, every one of them looked like a crash — and an app
that reports its own integrity as a fault teaches you not to trust either.

`components/common/Note.vue` gives them one treatment, and **confines colour to the icon**:

- `info` — neutral. An empty state, a hint, something not started yet.
- `limit` — a stated limitation or refusal. The `uncertain` hue on the icon only; the same hue
  the app uses everywhere for "we will not claim this", and nowhere for "broken".
- `error` — something actually failed. The only variant that gets `destructive`, which is what
  keeps `destructive` meaning anything.

Body text stays at `muted-foreground`, an optional title at `foreground`. `role="alert"` only
for `error`: a refusal is information, not an interruption.

### 14.8 Where feedback is wired

`composables/useAnswerFeedback.ts` is the single implementation of "a verdict just landed": it
plays `correct`/`wrong` and exposes `flashClass` + `flashKey` for the two answer animations (the
key is what makes an identical second verdict replay — CSS will not restart an animation on an
element that did not change). **The sound never delays the UI**: state is set first, nothing is
awaited, and `play()` swallows its own failures.

- Review — `views/ReviewView.vue`, on the answer response; `sessionComplete` when the queue
  empties, and only if the session had cards in it.
- Grammar drills and Placement — `components/grammar/DrillItem.vue`, so both screens answer
  identically through the one component that owns the item; `sessionComplete` on a finished
  batch.
- Ear — `views/EarView.vue`, on the trial response; `sessionComplete` at the end of a session.

The three milestones are fired where the fact behind them becomes provable: a unit in
`views/PlanView.vue` (off the refetched plan, because "done" is a fact about the review deck), a
topic in `views/GrammarView.vue` (on the phase transition, carrying the gate's own reason
string), a checkpoint in `views/ProgressView.vue` (carrying the measured lemma criterion, never
the self-attested half). `Milestone.vue` is mounted once in `AppShell.vue`.

### 14.9 Illustrations

The app draws five things, and only five ([ADR-037](../.claude/DECISIONS.md#adr-037)): the
line motifs of Hungarian folk embroidery, in one colour, without fills.

| Motif | Component | Where it sits |
|---|---|---|
| Tulip on its stem | `art/Tulip.vue` | tutor history when empty; review start when the whole deck rests |
| Pair of birds over a sprig | `art/BirdPair.vue` | the library's two empty states (no texts; a filter that found nothing) |
| Pomegranate rosette | `art/Rosette.vue` | the review done screen (accent tone); the 404 |
| Sprig of wheat | `art/Sprig.vue` | the trash and the words list when empty |
| Vine of leaves and buds | `art/Vine.vue` | under the wordmark at login and setup (accent tone) |

**Rules.** `currentColor`, 1.5 px strokes, round caps, no animation, under 2 KB a motif,
`aria-hidden` — the sentence beside a drawing carries the meaning. The default tone is 70 % of
`--muted-foreground`; `accent` is 50 % of `--primary` and marks an arrival (the done screen,
the doors). Decorative contrast sits at 2–3:1 by design: a drawing that competes with the
text is the wrong drawing. A drawing never depicts a figure — no rings, flames or counters
(§14.5) — and never appears on a `Note`, on a card, on the Progress screen, or in the header
of a screen. `Illustration.vue` (`name`, `size` sm/md/lg, `tone`) is the one way to place one;
`EmptyState` takes `art` in place of `icon`.

**What nobody caught** goes to the handler in `main.ts` (ADR-038): the stack to the console,
one toast in the learner's language through `lib/errors.ts`, the same sentence at most once
per two seconds, and a 401 that got that far clears the session and opens the login form with
the way back. Screens still catch their own failures and say them in place — the handler is
the floor, not the pattern.
