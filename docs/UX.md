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

Also here: review forecast, checkpoint history, daily-minutes adherence, and the honest
statement that the lemma targets come from the brief and are estimates.

---

## 10. Settings

Sections: **Profile** · **Review** (desired retention, new-card limit with the governor curve,
direction weighting by goal) · **Services** (TTS/ASR/NLP/LLM provider, endpoint, key, test
button, latency) · **Language** (UI locale, gloss language, fallback behaviour) ·
**Data** (pack version, sources and licences, export/import, danger zone) · **About**.

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
