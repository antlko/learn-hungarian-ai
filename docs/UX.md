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
7. **Dark first**, both themes complete, `prefers-color-scheme` honoured, explicit toggle wins.

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

One primary CTA routes to whatever is actually due. `space` from anywhere on this screen starts it.

---

## 4. Review

Chrome-free. The card owns the viewport.

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

**Keyboard:** `space` reveal · `1`–`4` rate · `e` edit note · `s` suspend · `u` undo ·
`*` mark leech · `Esc` exit. Answering is `Enter` for typed directions.

**Audio autoplays** on `listen` and `spell` directions. Never on `recognize` — it would give
the answer away.

---

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

- **Coverage meter** shows lemma and element coverage with the 95/98 lines drawn and labelled
  *«пороги выведены для английского, ориентировочно»* ([R6](../.claude/research/06-lexical-coverage.md)).
- **Word detail rail** shows the breakdown of *this exact form*, not just the dictionary entry.
- **`n+1` rail** surfaces sentences scoring in the sweet spot, with the unknown elements named.
- **Keyboard:** `j`/`k` word · `n`/`p` sentence · `a` add card · `K` mark known · `i` ignore ·
  `f` toggle rail · `/` search · `1`–`5` set knowledge level directly.
- An **"analysis is wrong"** link on the rail writes a `morph_correction` override.

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

- 2AFC, immediate green/red, both members replayable after the answer.
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

A material = uploaded audio + its transcript (required: passes 2–3 and the coverage pre-check
are built on it; it goes through the normal text ingest). The pass switcher is the learner's
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
Locks are visible and name the specific blocker (*"Сначала нужно: винительный падеж"*), because
a learner who can see what to finish can plan; one who just finds a topic greyed out cannot.
Fallbacks are shown as a count — dropping back is normal, and a silent demotion would read as
the app changing its mind at random.

**Six exercise shapes**, `allomorph | case | conjugation | cloze | transform | mcq`. Choice
items are answered with `1`–`4` or the mouse; `cloze` and `transform` are typed. **The expected
answer is not in the payload** — the server holds it until the attempt arrives, which is what
makes this retrieval rather than recognition. After the answer, an option that is not a real
Hungarian word is marked, because *"-házben is not a word"* is a better correction than a red
cross.

**Keyboard:** `1`–`4` choose · `Enter` check, then `Enter` again for the next item.

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

---

## 9. Progress — the signature screen

```
┌─ Прогресс ────────────────────────────────────────────────────────────────┐
│                                                                           │
│  Леммы  1 847                          B2 ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄  6000    │
│  ▁▂▃▄▅▆▇                               B1 ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄  3500        │
│                                        A2 ┄┄┄┄┄┄┄┄●  2000                │
│                                                                           │
│  ── Локативная система ──────────────  ── Признаки ──────────────────     │
│           где     куда    откуда        Case      ████████░░ 71%          │
│  внутри   ██ -ban ██ -ba  ▓▓ -ból       Mood      ███░░░░░░░ 32%          │
│  на пов.  ██ -n   ▓▓ -ra  ░░ -ról       Definite  ██████░░░░ 58%          │
│  рядом    ▓▓ -nál ░░ -hoz ░░ -tól       Poss.     ███████░░░ 66%          │
│                                          VerbForm ████░░░░░░ 41%          │
│  ▲ слабое место: -ról/-ről (делатив)                                      │
│    и условное наклонение (Mood=Cnd, retrievability 0.41)                  │
└───────────────────────────────────────────────────────────────────────────┘
```

The **locative grid** and the **feature heatmap** are what no other tool can render, because no
other tool tracks features separately. This screen is the product's argument for itself.

**As built.** The lemma curve is cumulative from `lemma_state.first_seen`, with the A1–B2
month-milestone targets as dashed reference lines labelled as reference points, not judgements.
The heatmap groups features by key (Case, Mood, …), weakest first, with a "слабое место" line
naming the two weakest well-evidenced features. The checkpoint panel shows the NEXT assessment
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
**Services** (TTS/ASR/NLP/LLM provider, endpoint, key, test button, latency) ·
**About** (version, pack version and edition) · **Sound**, which sits outside the settings
payload because it is a device preference, not an account one.

There is **no Data section and no Profile section**. Data went with the export
([ADR-011](../.claude/DECISIONS.md#adr-011)) — the pack version and edition it used to carry
are in About. Profile is a single control, "change my password", which lives on the
**Accounts** screen beside the account it changes; account management is that screen,
admin-only in the sidebar except for that one card.

The **new-card limit** control is where the governor lives: raising it past what the learner's
stated minutes can sustain renders the 90-day projection first, and requires an explicit
override.

---

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

## 13. Offline (PWA)

The review queue and its audio are precached, so Loop 1 works on the metro — which is exactly
where the brief expects it to happen. Answers queue locally and sync on reconnect; conflicts
resolve last-write-wins per card, since a single learner cannot genuinely conflict with
themselves. Reading, grammar and HVPT need the server in v1.0; offline support for them is a
v1.2 consideration.

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
beside each token block so a reader never has to open this file to check one.

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

Four sounds — `correct`, `wrong`, `milestone`, `sessionComplete` — **synthesised at runtime**
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
visible *real* capability growth (the lemma curve, the feature heatmap); and a milestone moment
when something genuinely real happens — a curriculum unit finished, a grammar topic reaching
`mastered`, a checkpoint passed.

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
