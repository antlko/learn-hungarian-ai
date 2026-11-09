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

### 1.1 Copy rules

Learner-facing text is cut to what the screen cannot do without (browser flow not exercised).

- **One helper line per block, at most**, and only where the block cannot be understood without
  it. Section subtitles, "what this means" paragraphs and caveats are not that line.
- **Everything longer goes behind `InfoTip`** (`components/common/InfoTip.vue`): a small grey «?»
  that opens a popover beside it on a wide screen and a card above the tab bar on a phone. Put it
  next to the label or title it explains. The page must read without it. **Placement:** the card
is teleported to `<body>` and `position: fixed` from the button's rect — below and left-aligned
to the button, clamped to stay inside the viewport with 12px margins (max width
`min(20rem, 100vw − 24px)`), flipped above when there is no room below; scrolling closes it; Esc
or an outside click closes it and focus returns to the button. Never render it inside a clipping
container.
- **Every sub-page has a way back.** `components/common/BackButton.vue` («← Назад», 44px) sits at
  the top of every screen that is not one of the five tabs — unit page, placement test (every
  step), reader, «Знаю» list, tutor — because the phone chrome has no browser back. It uses
  `router.back()` when the previous entry is its fallback route, else navigates to the fallback.
- **Labels are not hints.** A machine translation named with its model, an uncertain analysis,
  «not built», a refused session with its numbers: short, but always visible (§1.5).
- **Write like a person.** Short sentences, the word the learner would say, no stacked clauses,
  no officialese («в наборе», «по сроку», not «размер порции очереди»). Headings are nouns or verbs.
- **Glossary (one word per idea).** «слово» — never «лемма» (the pack's lemma is an internal
  term). «в колоде» — a word added to review; «выучено» — a word reliably remembered; no
  «в работе». Today, Progress and the plan say the same thing in the same words.
- **No month wording** anywhere a learner reads («Месяц 1», «месяц 3»): levels are named by
  size or CEFR band. `unit.month` is an internal ordering field.
- **No standalone state fillers** («впереди», «ещё нет», «не было», «ещё не сдавалась»). A thing
  not reached simply carries no label; an empty value is «—». A state word stays only when the
  learner can act on it, in plain speech: «пройдена», «не пройдена», «в процессе».
- **Registration and sign-in** carry title, fields and one button; a password rule is a placeholder.

---

## 2. Shell

**Page container, scroll, safe areas (one rule each).**
- `AppShell` wraps every routed screen in ONE container, `mx-auto w-full max-w-5xl` (1024px)
  inside the shell's `px-4 sm:px-6 lg:px-8`, so the outer edges are identical on every page
  ([ADR-059](../.claude/DECISIONS.md#adr-059)). `meta.width: 'wide'` (`max-w-6xl`) is the only
  exception, for the reader. **Dashboard pages (Today, Progress) use the full 5xl width; a view
  that is a single reading or form column puts `page-narrow` (`mx-auto w-full max-w-3xl`, defined
  once in `tailwind.css`) on its root** — Settings, Users, Library, Resources, Listen, Ear, Words,
  Known words, Placement, Review start, Grammar, Unit, Tutor. A view's root otherwise sets no
  `max-w-*`, `mx-auto`, `px-*` or `pt-*` of its own. The content is wrapped in `RouteFade`
  (§14.3): fade + 6px rise on a path change.
- `<main>` is the scroll container and survives navigation, so the router's window scrollBehavior
  never reaches it: the shell resets `main.scrollTop` to 0 on every PATH change (not query-only
  ones), restores it on Back/Forward, and `history.scrollRestoration = 'manual'` stops a reload
  restoring an offset into an unrendered page.
- The topbar is in the flex column (never overlays), solid, fixed `h-14` plus
  `env(safe-area-inset-top)`; `viewport-fit=cover` is set in `index.html`; BottomTabs pads
  `env(safe-area-inset-bottom)`.

```
┌──────────────┬──────────────────────────────────────────────────────┐
│  tanulom     │  A2.2 ≈71%  ·  🔥 17  ·  due 42  ·  account menu     │
├──────────────┼──────────────────────────────────────────────────────┤
│ ▸ Today      │                                                      │
│   Review  42 │                    ‹ route ›                         │
│   Read       │                                                      │
│   Grammar    │                                                      │
│   Tutor *    │            (* only when an LLM is configured)        │
│   Listen     │                                                      │
│   Ear        │                                                      │
│ ─────────    │                                                      │
│   Progress   │                                                      │
│   Accounts † │            († admin only)                            │
│   Settings   │                                                      │
└──────────────┴──────────────────────────────────────────────────────┘
```

Below `md` the left menu is replaced by bottom tabs (below). Library is reached from Read; Words
from the bottom-tab sheet and Progress.

**Capability dots** (now inside the account menu, not the bar) — filled = reachable, hollow = configured but unreachable, absent = off.
Hovering gives endpoint and latency. Features that need an unreachable service are visibly
disabled with a one-line reason, never silently missing.

**The shell, as built.** The sidebar orders by the daily loops — Today (which carries the course as a road — there is no Plan item), Review (with the
`dueNow` badge), Read, Grammar, Listen, Ear — and nothing for features that are not built (the earlier «скоро» group of Speak and Write was removed: a row that does nothing reads as breakage). It **collapses to a 56px
icon rail** (PanelLeft button at the foot, persisted per device in `tanulom.sidebar`, the badge
becoming a dot on the icon). Below `md` the rail is replaced by the bottom tabs (below); an earlier off-canvas drawer behind a
hamburger was dropped, and before that the fixed 224px column ate 60% of a phone screen.

**Top bar, level chip, account menu, bottom tabs (B4).** The right side of the bar is, in order:
the **level chip**, the streak flame (≥ sm only), the due chip (links to `/review`) and the
**account menu**. The chip shows the estimated label («A2.2») and «≈71% слов текста» with its
confidence qualifier («мало данных» / «ориентировочно»); on a phone only the label. Click/tap opens
a popover: «Уровень (оценка)», one mini bar per CEFR band with an **80% marker**, the method
sentence (`level.method.v1`, ALWAYS visible, never a tooltip) and a link to `/progress`. Data:
`GET /api/stats/level` through `stores/level.ts` — fetched once, then at most once a minute, when an
answer lands or the learner returns to Today; no polling. While loading it is a skeleton; on a
failed fetch the chip is absent (never an error, never `e.message`). It is a measurement that can
go down: no ring, no flame, no points (R14, ADR-047). The **account menu** holds language, theme,
the optional-service dots and sign-out; the dots left the bar.

**Dynamic navigation rules.** Order is fixed. Review's badge is `dueNow`. A small dot sits on the
row holding today's unmet obligation: cards due → Review; queue clear but the day's minutes not met
→ Read. The AI teacher row is listed only when an `llm` is configured (the capabilities list has
answered; before that it is listed, so nothing flickers). The disabled «скоро» rows (Speak, Write)
are gone. A new-reader-words badge on Read is NOT built: no cheap endpoint exposes it.

**Below `md` the bottom tabs replace the rail and the hamburger drawer**: Сегодня · Повторение (due
badge) · Чтение · Грамматика · Ещё. «Ещё» opens a sheet with the remaining routes. 44px targets,
`aria-current`, the bar adds `env(safe-area-inset-bottom)`. The shell exposes `--bottom-tabs-h`
(bar height incl. safe area on phones, `0px` from md up); `<main>` pads by it, and a screen's own
sticky bottom strip should sit at `bottom: var(--bottom-tabs-h)`.

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
| Today | the header, the next action, obligations | **the path** — the unit panel sits beside it from `lg` (§3.1) |
| Grammar | the lesson, then the drill | **the level's topics**, current one marked |

The last two are new, and both were the same failure: the orienting answer existed but was
somewhere else. The old Plan screen kept the whole syllabus BELOW the unit card, so "what am I doing now" and
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

> **Shell chips (topbar).** The level chip, the due chip and the streak chip are all `h-9` pills,
> `text-sm`. The level chip shows only «A1.1» and a caret; its popover (teleported, viewport-clamped,
> `w-80` on desktop, a full-width card above the tab bar on phones, Esc / outside click) holds the
> level, «Понимаете ≈N% слов обычного текста», the confidence chip, per-band bars, the method
> sentence and «Подробнее» → `/progress`. Under `md` the due chip is removed (the tab badge carries
> it); the streak chip (flame + number) shows on phones and is hidden from `lg` on Today, where the
> streak card exists. The brand mark sits left of the title on phones. *Written without exercising
> a browser.*

### 3.1 Today: the dashboard and the path (replaces the Plan screen)

**Layout.** From `lg` (1024px) a two-column grid, `minmax(0,1fr) 340px`, gap 24: left the path
(a ≈640px column centred in its cell), right a statistics column that is `sticky` ONLY when
`(min-width:1024px) and (min-height:900px)` — on a shorter laptop it scrolls with the page, because a
sticky column taller than the viewport strands its lower cards. Cards are compact (`p-4`, numerals
`text-2xl`). **Below `lg` there is no stats strip at all**: a phone shows the topbar, the banner and
the path (the streak is a chip in the topbar, the due count is the Повторение tab's badge). There
is no page title of its own on any width — the topbar says «Сегодня»; only a small tram strip
(`aria-hidden`) sits right-aligned on desktop. Components: `views/TodayView`,
`roadmap/TodayStats`, `roadmap/RoadmapPath`.

**Statistics column** (`TodayStats`; derived facts only, counts may go down, nothing is a reward,
no explanatory paragraphs — explanations sit behind `InfoTip`). Tinted `StatCard`s that enter with
the staggered `card-in`: **«К повторению»** (mist; due now, counted up; «новых: N»; a primary
«Повторить» to `/review/session`; with nothing due and nothing new it shows the thermal-dome
illustration and «На сегодня всё» instead — rest, not a reward); **«В колоде»** (sage; total and
four chips from `GET /api/stats/deck`: новое · учу · повторяю = young + relearning · закреплено =
mature + known; hidden until the endpoint answers); **«Серия»** (cream; the unchanged
obligations-met streak and today's minutes «12 из 90 мин» on a thin bar); **«Уровень»** (blush;
«A1.1», «≈N% слов текста» and an `InfoTip` with the method sentence; text on blush is ink); and a
white **«Минуты по дням»** `MiniBars` for the last 7 days (text labels, no goal line).

**The path and the soft gate (ADR-058/059).** `unitPhase.computeGate` runs client-side over
`plan.units`: core units in `ord` order, `share = settled / total` (0 for an empty unit), `passed =
share ≥ unlock.share` (server value, default 0.7). **Current** = the first core unit not passed
(this replaces `plan.current` for drawing). Units before it are passed; units after it are
**closed** unless the learner already opened them (`enrolled`) — then they are drawn open with
their ring. Themes are never closed (dimmed until enrolled). Closed is presentation only: the node
is soft grey with a lock (`aria-label` «закрыт»), still tappable, and the unit page says why (below);
the API refuses nothing and ADR-013 stands.

Nodes are round (56px, 72px for the current) in a gentle zigzag. A unit in work (enrolled, or with
words already in the deck) wears a **`ProgressRing`**: value = settled share, centre = «42%», colour
by share on the ember → blue → green ramp, fill 700ms on mount, a check once the share is reached.
The current, unopened unit is a play disc with the «Начать» bubble; once opened it is a ring with
«Продолжить»; only this node has the halo pulse. Titles sit under nodes (≤3 lines); no state
words are printed (they live in `aria-label`s). Tier headers stay thin dividers
(«Базовый · закреплено N из M» — the numerator is **settled**, the word is «закреплено», not
«выучено», because that is what the number now means) with a 40px decorative illustration at the
end. A checkpoint is a flag node after the last core unit of its month (the only padlock besides
the soft-grey closed ones is a failed checkpoint); it links to `/progress`. Theme units are NOT at
the bottom of the path: a segmented control directly under the banner, **«Курс | По темам»**
(`role=tablist`, ≥44px targets, arrow keys), swaps the path for the themed units — a grid (3 columns,
2 under 400px) under the header «По темам · закреплено N из M». The choice is remembered per
device (`localStorage` `tanulom.todayView`, try/catch) AND in the URL (`/?view=themes|course`; the URL
wins), so back from a unit page returns to the same view: a THEME unit's `BackButton` falls back to
`/?view=themes`, a core unit's to `/`. Enrolling a theme unit changes nothing on the course path; the
themes view shows its own header and no «Продолжить» bubble. A checkpoint flag also draws a quiet 1.5px
dashed divider (6/6, `border` at 60%, `aria-hidden`) across the column behind it. No month is ever named.

**Sticky banner.** When the previous unit has reached the share and the current one is not
opened: «Готовы к следующему разделу?» with a primary «Добавить слова» (opens the unit page,
whose action enrols) and a quiet «Позже» that hides it for the session (`sessionStorage`).
Otherwise the banner shows the current unit's title (one line, truncated) and a labelled
«Продолжить» / «Начать» button. It always refers to the CORE path's current unit, in both
views. Below `lg` the banner is `sticky top-0` inside a wrapper with an opaque `bg-background` and a
`::before` strip covering `<main>`'s top padding and side gutters, so no path content shows above or
around it; from `lg` it is an ordinary static block (no strip — it once painted over the title).

A node navigates to the unit's **page**, `/unit/:code` (`UnitView`, narrow column). Top to bottom:
`BackButton`, title, an 88px `ProgressRing` with «закреплено N из M» and «в колоде K», the goal,
a status chip only when informative, a **soft notice** when the unit is closed (cream tint, ink
text: «Сначала закрепите «<предыдущий>»: N% из 70%» with a secondary «Открыть всё равно» that
enrols exactly like the primary action — the sticky bar is replaced by it while closed), grammar
chips, the words as a plain list, and the **sticky bottom bar** with the unit's one action
(`bottom: var(--bottom-tabs-h)` above the phone tabs). `/?unit=<code>` redirects to the page; a
unit becoming fully learned still raises the celebration. On load the page does not scroll,
except once: if the current node is below the first screenful, `<main>` is scrolled (instant) to
centre it. Performance: plain node elements, rings only on started units, `content-visibility:
auto` per tier. `plan.handedOver` adds one small «Куда дальше» link.

> Status: type-check, lint, build and docs-check run; the browser flow (two-column layout,
> sticky column, ring animation, closed nodes, ready banner, unit notice) has NOT been exercised
> in a browser.

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
`u` undo · `t` set the word aside (§4.3) · `a` the tutor about this card (only with an LLM), in both phases and
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

**The session header is ONE slim row that never wraps** (amended 2026-11-09; browser NOT
exercised): `flex-nowrap`, the count `whitespace-nowrap` and tabular. On a phone
(<md): `[× Выйти]  …  3/15  [▢ Форма ?]  [Учитель]` — the exit link (to `/review`, no confirm —
answers are saved one by one), the count as «3/15», a SHORT card-type chip (icon + one word from
`review.dir.short.*`: Форма / Перевод / Слово / На слух / Письмо) whose «?» carries the long
description and «клик по слову — разбор», a «новая» chip when it applies, and the Teacher as an
icon. The pile name, the weak-mode note and «набор N из M» are NOT on the phone row (the set
summary says which set it is). On md+ the row keeps the fuller wording: «набор 2 из 5 · 7 из
15» (a practice round: «все слова · 7 из 20») and the long type name («форма в предложении»).
Practice adds a «Тренировка» chip; what that means is in the type chip's «?» («Не влияет на
расписание повторений.»). Nothing else sits above the progress strip. **«Учитель» is hidden
entirely while the LLM is not configured and answering** (`capabilities.available('llm')`): no
dead-end panel, no `a` key, no key-strip entry; what the optional tutor is lives in Settings →
Компоненты only. The chips keep the no-hit-target rule that put the direction there in the
first place: `пропуск` reads in Russian as the imperative "skip", and as a bordered pill it
was clicked as a skip button and did nothing.

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

**On md+ it opens in a right-hand drawer** (`BottomSheet side="right"`): fixed to the right edge below
the topbar, 24rem wide (max 90vw), 200ms slide (none under reduced motion), internal scroll, ✕ in its
own header row, `Esc` closes. No scrim: the card stays visible and usable, a click outside does not
close it (click another word to switch, the same word to close), and on windows >= 1100px it **reserves its
width** (24rem right padding on the review page, animated 200ms, none under reduced motion) so the card
re-centres in the remaining space; narrower windows overlay it. The Teacher uses the same single drawer — opening one replaces the other — and nothing
scrolls for either. Below md the Teacher is the bottom sheet and the word stays under «Подробнее».
*Written without exercising a browser.*

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
   desktop: [× Выйти]  набор 2 из 5 · 7 из 15   ФОРМА В ПРЕДЛОЖЕНИИ (?)  [Учитель]
   phone:   [× Выйти]              7/15   Форма (?)  [🎓]
   ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░░░░░░░░░░░░░░
   [повторяю] ▬▬▬▬ (?)
   ┌─────────────────────────────────────────────┐
   │  A ház___ lakom.                             │
   │  …                                           │
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
- **The stage line** under the progress strip is ONE line (amended 2026-11-09): a chip with the
  stage the scheduler reports — новая · учу · шаг 2 из 2 · повторяю · закреплено · заново — four
  tiny rungs beside it, and a «?» that opens the «Как это работает» sheet (`HowItWorks`, the one
  place that explains stages, the four buttons and a miss). It is a POSITION: it can go back
  down, it has no end, nothing on it accumulates ([ADR-039](../.claude/DECISIONS.md#adr-039)).
  The «эта карточка: верно N из M» tally, the outcome marks and the inline «что это значит?»
  panel left this screen (the history lives in the words browser); after the answer
  «интервал 3 дн → 8 дн» appears. The **chance of recalling it now** was taken off the card on
  2026-09-03: in learning steps the model computes it against minutes, so it read «100%» over
  five crosses. It still stands in the words browser. No bar, no denominator, no target
  ([R14], [R16 §3]).
- **The verdict takes the answer box's place.** On a typed card the box is the question's; once
  the answer is in, «вы: ala / нужно: aztán» renders exactly where the box was — the eye is
  already there — followed by what the card additionally graded, and only then the sentence as
  context, for the directions whose prompt did not already show it. The first layout printed
  the sentence and its crib again above the diff, below a divider, and on a long card the
  correct form was under the fold behind the key strip: every wrong answer meant scrolling to
  find out what was wrong. The column is anchored at the top while a card is up, so the reveal
  grows downward instead of re-centring the prompt.
- **The grades carry their intervals** — «Хорошо ~3 дн», from `Scheduler.Preview`, marked
  approximate because fuzz is seeded at answer time; absent in practice. The pre-answer line
  «ответ верный → вернётся через ~3 дн» under the box was removed (2026-11-09): the grade
  buttons say it after the answer. A typed miss still prints «слово вернётся примерно через ~N».
- **Between sets: a summary of measured facts** — answered, right, missed (listed with their
  answers, to read over), «закрепилось» (the words whose interval crossed 21 days in this set,
  named), minutes — with «Следующий набор» (`space`) and «На сегодня хватит» **above the
  figures**, so a 375×812 phone reaches both without scrolling; the explanation of the missed
  list is behind a «?». No praise line,
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

**Review on touch** (below `md`, or `(pointer: coarse)` on any width). The actions — «Проверить» /
«Подсказка», «Показать», the four grades, «Дальше» — live in a bar pinned above the tab bar
(`bottom: var(--bottom-tabs-h)`, 0px on md+), buttons at least 48px high with `gap-3` so a thumb
does not reach the neighbour; on a wider touch screen the bar pads for the home-indicator inset
itself (on <md the tab bar already covers it). No swipe gestures. `KeyBar` is hidden in both cases
(and, where it shows, sits above the tab bar; on the review screen it starts collapsed to a
small chip, per-device). The stage line is always visible; the word inspector column below `lg` is a «Подробнее»
disclosure that opens itself when a word is asked for. Typed answers are `lang="hu"`,
`autocapitalize/autocorrect=off`, `spellcheck=false`, `enterkeyhint="done"`; the phone keyboard's
Enter submits («Проверить»). **No accent strip by default**: answers are not case-sensitive and
diacritics are not required («Принимать ответ без диакритики», on by default), so a plain Latin
keyboard is enough. `AccentStrip` stays as a per-device opt-in (Settings → Повторение, «Показывать
кнопки с á é ő ű…», `lib/accentKeys.ts`, localStorage, default off): nine lower-case letters in
ONE horizontally scrolling row inside the action bar, no shift; it inserts at the caret without
blurring. **Keyboard:** the pinned bar is lifted by the keyboard's height
(`composables/useKeyboardInset.ts`, from `visualViewport`) so «Проверить» stays above it, the spacer
below the card grows by the same amount and the input is scrolled into view. On desktop the
primary button carries a tiny `↵` («Проверить ↵», «Показать ↵», «Дальше ↵»); the key strip stays
collapsed; phones show nothing about keys. Not yet checked in a browser at 375px.

**Bottom sheet** (`components/common/BottomSheet.vue`). Teleported to `<body>`, slides up with a
200ms transform/opacity transition (none under `prefers-reduced-motion`), `max-height: 75dvh`,
rounded top corners, grab handle, visible ✕, backdrop tap and Esc close it, `role="dialog"
aria-modal`, focus moves in and returns to the opener, Tab wraps inside. It sits above the tab bar
(`z-50` over the bar's `z-30`), pads for the home-indicator inset and, while the keyboard is up,
lifts by the keyboard's height. On <md the **Teacher** opens in it (the same `TutorAside`
conversation and thread as the desktop side panel, which stays on md+) and «Как это работает»
uses it too (a centred dialog on md+).

**«Как это работает»** (`components/review/HowItWorks.vue`) is a `BottomSheet` (<md) / dialog: the four
stages, what each grade schedules (the card's own preview times when opened from the «?» beside the
ladder, general text from the start screen), that one miss does not erase a word and there is no
«N правильных подряд» rule, that misses are listed and return later while hits are not re-asked in
the set, what «Знаю» does, and that the new-card budget is global. After a typed miss the verdict
carries one line saying when the word returns, from the preview already on the card.

### 4.5 The start screen — what to take, before taking it

```
┌─ Повторение (?) ───────────────────────────────────────────────────────────┐
│  ╭─ Сегодня ───────────────────────╮  ╭─ Тренировка (?) ───────────────╮    │
│  │  74 к повторению   72 новых слов │  │  [ Начать ]                    │    │
│  │  [          ▸ Начать   ⎵       ] │  │  Настроить ⌄                   │    │
│  │  ( Все 146 | План 69 | Тексты 77)│  ╰────────────────────────────────╯    │
│  │  Настроить ⌄                     │                                         │
│  ╰──────────────────────────────────╯                                         │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Lively pass (ADR-059; browser NOT exercised).** «Сегодня» is a mist tint card: the two
numbers (к повторению · новых) are large count-up numerals (the final value is read once by
screen readers), the pile segmented control carries soft-tinted counts, and the rest state draws
the thermal-dome (rest, not reward). «Тренировка» is a cream tint card with a 48px paprika-string
chip (decorative). The two cards enter with the `card-in` stagger.

In the session the card itself stays plain white — no art. Around it: the progress strip's done
part is `primary` on a soft mist track; a correct answer washes the card edge sage and draws the
check (240ms, `check-draw`); a wrong one washes it blush and nudges the answer box ±4px
(`answer-wrong`, transform only); «Дальше» enters with `route-in`; the stage chip is tinted
(новое = mist, учу = cream, повторяю = sage, закреплено = sage + check) with ink text. The set
summary shows a ProgressRing of верно/отвечено (label «12 из 15», a check when all were right),
four tinted tiles (отвечено mist, верно sage, не получилось blush, минут cream) and the missed
words in a blush card. No confetti, no points; reduced motion shows the final state.

Mobile-first: one column, the hero first. «Сегодня» shows the two numbers that matter
(к повторению · новых слов) of the selected pile and one large «Начать». Under it a segmented
control «Все · План · Тексты» (counts as badges when the pile is non-empty) chooses what the button
starts. Set size and order sit behind «Настроить» — remembered per device exactly as before
(`lib/sets.ts`), default 15 / by due date; «вперемешку» lives there. «Тренировка» is a quieter
card with its own «Начать»; its pile, length and «слабые слова» are behind its own «Настроить», and a
«?» says in a sentence that practice does not change the schedule. With nothing due and nothing new
the hero is a calm «На сегодня всё» with links to practice and to reading. From `md` up the two
cards sit side by side (3fr/2fr, max 4xl) instead of stretching a form. The «Как это работает»
dialog opens from the «?» by the title. Browser flow not exercised.

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
- **`space` starts the selected pile** (default «Все»), as it starts the day on Today. Today's own «Начать: Память» goes
  straight to the card, past this screen: that button exists so the learner does not have to
  choose. The sidebar row, the topbar due chip and the memory loop card come here; the library,
  the reader and the trash go straight to the card with their query.
- **The card still says what it is in.** The progress row names the pile («План», «Только
  слова из текста») and, in practice, the mode, the pile and the criterion really applied. The
  done screen says what the other piles hold in one sentence and links back here; its
  «Тренировка» starts a round over the pile just finished.

### 4.6 Answering by options (client; browser not yet exercised)

A cloze/produce card is answered by tapping one of `QueueItem.options` when the per-device
`answerMode` (`lib/answerMode.ts`; default `choose` on `(pointer: coarse)`, `type` otherwise) is
`choose` AND the server sent at least three options; otherwise the typed box appears, silently.
A tap submits at once as the ordinary answer with `via: 'choice'`. The verdict colours the
options in place (sage on the right one, blush plus the existing nudge on a wrong tap, the rest
at 50%), so the card does not move. The bar shows «Знаю» before the answer and «Дальше» after;
there is no «Подсказка» in this mode. Keys: `1`–`5` pick, `Enter`/`Space` move on. A toggle in
the header row and a row under «Настроить» on the start screen switch the mode. Choosing is
recognition and weaker than typing (PEDAGOGY §2.10); the start-screen tip says so in one line.

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

Title and a primary «Добавить текст» button. Two segments, «Мои тексты» and «Готовые подборки»
(own texts by default when there are any, otherwise the sets). Own texts are compact rows, not
tall cards: title, kind icon and word count, the known-words share, the level chip, a small bin,
and «Повторить слова: N» wherever the text has words in the deck (the per-text scope, ADR-022).
Past three texts a toolbar appears: title search, sort, level filter. The add form opens from the
header button only — a library whose front page is a data-entry form reads as a task, not a
shelf.

**The level chip always names its author.** `B1 · оценка по частотам` (the ingest heuristic:
pack frequency bands ≥90% of running words, grammar-topic ceiling at ≥3 hits, a sentence-length
nudge), `· оценка модели` (the `services.llm` second opinion), `· вы указали` (the learner —
never overwritten by a machine). A text the estimator cannot resolve carries no chip at all:
«уровень не определён» is a supported outcome, and the filter can select exactly those.
ADR-023; the labelling rule is ADR-014's.

**Ready-made sets («Готовые подборки»).** Shown only when `GET /api/library/sets` returns
something (older packs: the segment is not drawn; a failed fetch does the same and never blocks
the shelf). Compact rows grouped by level A1 · A2 · B1, the set at the learner's level first
(«подходит вам сейчас» is a hint, never a filter or a lock), each with title, «N предложений»,
and one small button — «Добавить» (`POST …/import`, then the reader) or «Открыть» when already
imported. The collapse to a few sets («Показать все подборки») stays. What the sets are —
real sentences written by people, adding creates a text and the cards are the learner's own act
of tapping words — is an `InfoTip` beside the heading. Browser flow not exercised.

**The reader on a phone.** A compact header: title, the two coverage figures as one line each over
a thin bar, with «что означают эти цифры» behind an `InfoTip`; the text runs edge to edge with a
generous line height. A selected word opens a **bottom sheet** above the tab bar (at most 60% of
the height, with a close button) and the text gets padding and scroll margin so the word stays
visible above it; «Знаю / Игнорировать / В карточки» are large buttons pinned to the sheet's bottom.
The key bar is hidden below `md` and on touch. From `md` the word detail is the side rail it was.
Browser flow not exercised.

### 5.2 Words — «что выучилось»

`/words`, linked from the lemma total on Прогресс («Все слова →» — a total nobody can inspect
is a claim). Every deck word as a row (a stacked card at 390 px, a three-column row from `md`):
dictionary form and gloss; the **stage** as three rungs (учится → повторяется → закреплено,
filled vs hollow plus the stage named in words, so colour is never the only channel);
«вспоминаем» (relearning) sits on the first rung in a neutral tone, not red — it is a step back,
not a failure; «знаю» stands outside the ladder. The ladder is a position, not a score: it can
go down and counts nothing. While a card is on learning steps the row says «шаг N из M». Beside
it a **memory bar** («память ≈N%», the strongest card's retrievability now, shown only once
something was reviewed), the next review in words (сегодня / завтра / через N дн.) and the
card's interval · reviews · misses (only non-zero fields). Stage chips (все · учатся · вспоминаем
· молодые · зрелые · знаю) send `?stage=`; they carry no counts because the API returns no
per-stage totals. A collapsible legend explains the stages (young = interval under 21 days,
Anki's convention). Stage, status, sort and search persist in the query string. Search goes
through pack case-variants (never `COLLATE NOCASE`) and matches glosses too; sorts include
«сначала шаткие», the browsing twin of practice's weak mode. Verified at 390 px: not yet.

**Lively pass (ADR-059; browser NOT exercised).** Four tinted tiles above the list — новое
(mist) · учу (cream) · повторяю (sage) · закреплено (sage) — count WORDS from
`GET /api/stats/deck` with a ≤600ms count-up; they are hidden when the endpoint does not answer.
The row stage chip uses the same tints as the review card's chip (ink text; закреплено carries a
check), rows enter with the `card-in` stagger (≤6 steps), and the empty state draws the paprika
string. «Знаю» (`/known`) gets a sage header tile with the count and a sprig; its rows stagger
the same way. Verified at 390 px: not yet.

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

**As redesigned (ADR-053).** The topic list is a road grouped by level, drawn with the Today
roadmap's node vocabulary (disc on a rail, shape + icon + word per state): «только правило» ·
«ещё не начато» · «тренируем по одному» · «вперемешку» · «освоено» (the padlock only for a failed
checkpoint). A topic page has two tabs, Урок | Упражнения; ADR-043's evidence picks the first one.
The exercises tab prints the gate in numbers from the server («точность ≈72%, нужно 80%; ответов
14 из минимум 20») and one sentence on why practice is one pattern at a time first. A topic with
no drills says so and points to the tutor and the reader. The side rail of sibling topics is gone;
previous/next links replace it.

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

Single column from 390 px; every chart has a text or table alternative and every interactive
element is keyboard-operable. Section order:

1. **Where you are** (`LevelPanel`, `GET /api/stats/level`, [ADR-047](../.claude/DECISIONS.md#adr-047)).
   The estimated level label, «≈N% слов обычного текста», the confidence qualifier and the fixed
   method sentence are ALWAYS on screen. Under them one bar per CEFR band (A1…C1) with a marker
   at the 80% target, «ваших N из M», and reached / not reached as icon + word. A band is a
   button: it opens `GET /api/stats/level/band/:band`, the highest-frequency words of that band
   the learner does not yet count, glossed through `lib/gloss.ts`. «Хочу выучить» calls the same
   `POST /api/notes` the reader uses (a duplicate says «уже есть»). Nothing here is stored or
   only rises; both figures can go down.
2. **Lemma curve** (`LemmaCurve`). Hover/touch crosshair with date and count, ← → / Home / End
   from the keyboard, range 30 дн · 90 дн · всё (client-side), dashed reference lines from the
   server's plan targets (labelled as reference points, not judgements), table alternative.
3. **Activity calendar** (`ActivityCalendar`). Week columns, last 26 weeks, shade = minutes
   against the learner's own daily target in four steps with a legend — not points, no streak
   art. A cell's accessible name carries its minutes; selecting a day shows minutes by loop
   (cards and exercises / reading / listening) and, if the obligation failed, WHICH half
   (queue or minutes). Arrow keys move by day and week (roving tabindex); a table alternative
   lists the last 60 recorded days. No due forecast and no raw card count: the screen speaks only
   `srs.Breakdowns` vocabulary.
4. **Grammar** — the locative grid (cells are buttons named by the feature label, never the raw
   id) and the feature heatmap by family. Choosing a cell or a feature row opens a detail panel:
   the pack's label and plain explanation (`stores/features.ts`), measured accuracy / answer count
   / typical time, the recall-now figure with a note that it is time-based, and links to the
   grammar topics from the mastery row's `topics` (`/grammar?topic=<code>`).
5. **Level checks** («Проверки уровня»). The next one only: titled by size («300 слов», «900 слов»,
   «2 000 слов» …), never by month; ONE progress line «36 из 300 слов» with a thin bar; no state
   word until it has a verdict. A self-attested criterion (e.g. «Могу прочитать вслух незнакомый
   абзац без остановок») is a plain sentence + checkbox with an InfoTip: the learner checks it,
   the app cannot hear, and the tick does not touch the streak. Button «Проверить». History uses
   shape + word: пройдена / не пройдена.
6. **Free practice** — only when there is some; schedules nothing (ADR-019). Day bars have
   screen-reader text, not title-only tooltips.

The grid, the heatmap rows and the feature panel are the product's argument for itself: no
tool that treats a wordform as an atom can render them.

**The heatmap measures ACCURACY, not retrievability, and says so when it has measured
nothing.** Retrievability is the probability of recalling something *right now*: it is ~1.0 for
anything reviewed recently, so a learner a few days in met nine families at 100%, read the
screen as broken, and was right to. The bar is the rolling accuracy, drawn only once the
server's `ready` flag — `srs.GateReady` against `drill.gateWindow` — says the figure rests on
evidence; below it a row states its answer count and «мало данных». Colour has two earned states
and one unearned: held, `weak` (the server's verdict against `drill.masteryThreshold`), and
unmeasured. Nothing here re-derives either threshold. The detail panel does print
recall-now, labelled as a time-based estimate, because it is a different question from the bar.

**Families are named, and open** — «Падежи», never `Case`. The «Требует внимания» line names weak
features by label. **The locative grid distinguishes "not measured" from "not met"**: dashed with
an answer count versus «не было».

**Loading** is skeletons per block; the level panel and the overview load independently, and
failures go through `describeError`. The calendar, grid and feature panel are lazy chunks. The
exam countdown lives on Today (§9.1), not here.

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
**Components** (admin only — see §10.1a) · **Services** (TTS/ASR/NLP/LLM provider, endpoint, key, test button, latency) ·
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

## 10.1a Settings → Компоненты — the pack, restart and honest sidecars

Admin only: `ComponentsPanel` is not mounted for a learner, and the API enforces the role
independently. It sits above Services and opens with one paragraph: the core is complete with
nothing installed, these are optional upgrades. Data comes from `GET /api/admin/components`
([ADR-049](../.claude/DECISIONS.md#adr-049)); one card per component, each with what it is, what
you lose without it, and a state chip that carries a shape and text (never colour alone):
не создан · не настроен · загружает N% · готов · нет связи · говорит устройство · ошибка.

- **Language pack** (`PackCard`). Says whether the starter or the full pack is loaded and what the
  full one adds (`lib/packFacts.ts` holds lemmas 5,725 → 35,311 and sentences 12,858 → 60,000; the
  starter figures were counted in the embedded pack, the full-pack ones are «по документации» —
  none is measured at runtime, and the label says which is which). «Установить полный словарь»
  starts `POST /api/admin/pack/install`, then polls the job every 1.5 s **only while it runs and
  the tab is visible** (timers cleared on unmount); a job already running is picked up on open.
  `ready` raises a banner «Готово. Нужен перезапуск» (focus moves to it) with «Перезапустить
  сейчас». A supervised server restarts itself and the card polls `/api/bootstrap` until it answers
  again after having gone quiet, then reloads; an unsupervised one says «сервер остановлен —
  запустите его заново вручную» and polls nothing. `failed` maps `errorKey` through
  `describeError` with a retry. **The button is never enabled-and-failing**: with no pinned
  checksum (the default) it is disabled, the card says the pack is not published, and shows the
  two config fields (`langpack.url`, `langpack.sha256`, saved through the ordinary settings
  update) and the `make langpack` fallback.
- **Install method tabs** (`ComponentMethods`, for `mt`, `nlp`, `llm`; browser flow NOT exercised, real
  install NOT exercised). Under the one-line description each card has a tablist
  «Встроенный · Docker Compose · Свой сервер» (arrow keys, Home/End, ≥44 px targets). The default tab
  is the one in use: built-in when `provider: managed`, else built-in when `managed.available`, else
  Docker Compose. **A tab is a view, not a setting** — only installing or «Сохранить» changes config.
  `llm` has no built-in tab (a multi-GB model server is not embedded); a server that sends no
  `managed` field gets the two other tabs only. All `managed` fields are optional to the client.
  - **Встроенный** (`ManagedTab`, `POST /api/admin/components/:id/{install,start,stop,uninstall}`).
    One honest line before pressing: «Скачает и запустит сервис прямо здесь: ≈диск, ≈память —
    оценка» (figures are the server's constants, not measurements). `absent` → «Установить»
    (disabled with the server's `unavailableKey` sentence and a pointer to the other tabs when
    `available` is false); `installing` → thin progress bar, step label (`component.step.*`) and a
    collapsible «Журнал» (monospace, selectable, follows the tail unless scrolled up); `running` →
    chip «работает · порт N · только внутри» with «Остановить» / «Удалить» (confirm dialog: files go,
    settings stay); `stopped`/`installed` → «Запустить»; `failed` → the `errorKey` sentence, the
    short `errorDetail`, the log opened, «Повторить». «Перезапуск сервера не нужен» is stated
    because the app starts the child itself. State is announced in an `aria-live` line (state and
    step, not the ticking percentage). The list is re-read every 2 s **only while a copy is
    installing/starting and the tab is visible** (plus an 8 s grace after a button press); when the
    work ends, settings are re-read because the server flips the provider itself.
  - **Docker Compose**: the command, with «Что такое profile?» and «А порты не заняты?» as InfoTips.
    The second is true of `docker-compose.yml` as shipped (the sidecars publish no host ports);
    it stops being true if the admin adds `ports:`.
  - **Свой сервер**: the address (and, for `llm`, the model with the server's list) moved here from
    the Services card; «Сохранить» uses the ordinary settings update, «Проверить» re-probes.
- **Services rows follow the tabs.** A `managed` row shows «встроенный · порт N» read-only on a sage
  tint (no input) and its status is the same polled built-in state; mt/nlp/llm rows otherwise show
  the address read-only with a pointer to the tab. `managed` is offered in the provider select only
  once something is installed. Switching away is allowed (the server stops the child).
- **Translator** reuses the Services card's live block (`MtBlock`: download bytes, retry); its
  state still lives in `SettingsView`, so the Services row and this card cannot disagree.
- **Morphology analyser, speech server, speech recognition** report `built: false` as «пока не
  сделано» in the repository's own words — no compose command, no «Проверить», no «на связи».
  The Services row for a component that is not built reads «пока не сделано» too (tts keeps
  «говорит устройство» where the device has a voice, [ADR-012](../.claude/DECISIONS.md#adr-012)).
- **Context analyser (nlp)** (`built: true`). Explains itself in one paragraph (picks the reading of
  an ambiguous word from its neighbours; without it everything works and disputed cases say
  «возможно») and keeps the unmeasured ≈2.4 GB RAM line. Not configured: the compose command plus
  a pointer to the endpoint field. Configured and not ready: «Скачать модель» calls
  `POST /api/admin/nlp/warmup`; while the sidecar reports `loading` the panel re-reads
  `/api/admin/components` every ~2 s **only while the tab is visible** (timers cleared on unmount)
  and an `aria-live` line reads «загружает модель N%». Unreachable shows `errors.nlpUnreachable`;
  a failed warmup shows its `describeError` sentence inline.
- **Reader label.** An analysis with `via: "context"` was chosen among the form's readings by the
  sidecar; the word inspector keeps the «возможно» note (certainty stays `uncertain`) and adds
  «По контексту (HuSpaCy)…» (`reader.viaContext`). Uncertainty is never hidden. `WordDetail`
  (stored tokens carry no `via`) is unchanged.
- **Language model**: configured/reachable/model; endpoint and model stay editable in Services.
- A component with a `composeProfile` that is built shows a copyable
  `docker compose --profile <x> up -d` and «Проверить», which re-asks every probe.

**Not exercised in a browser:** the nlp warmup/poll and context label, plus the progress, restart-and-reload and focus behaviour were
type-checked and built, not run end to end.

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

**Palette A «Kalocsa & Balaton» ([ADR-059](../.claude/DECISIONS.md#adr-059)) — current.**
Warm paper `#fbf7ef`, card `#ffffff`, ink `#1e2a32`, primary teal-blue `#1d5c6e`; four soft
tints `cream #fbefd9 · mist #dcebef · blush #f7dcd2 · sage #ddebdd` (card = fill + 1px edge one
step darker, no shadow); `success #236b43`, `warn #8a5f00`, `destructive #b3261e`.
`--paprika #b8402a` is **graphics only** (illustration strokes, the ring's low end) and is never
text. Ring ramp `--ring-start/mid/end` = `#b8402a → #2a6fb0 → #1b8a6b`. Dark (derived, own
steps): bg `#0c1116`, card `#13181e`, ink `#f4ebdd`, primary `#7cc4d4`, tints
`#2b2619 / #16303a / #34201c / #1b2e22`, paprika `#e8836a`, ramp `#e8836a → #5da2e6 → #4fcba5`.
Radius `--radius` is 0.875rem (sm 10 · md 12 · lg 14 · xl 18px).
**The numbers are produced by `scripts/validate_palette.py` (`make palette-check`, part of
`make docs-check`), which reads the tokens out of `tailwind.css` and checks every pair used.**
Minimum measured: light text 4.58 / graphics 3.17; dark text 4.67 / graphics 3.04. Selected
ratios (light / dark): ink on bg 13.72 / 16.05, on blush 11.26 / 12.98; muted-fg on card 6.99 /
9.70; primary on card 7.48 / 9.09; success on sage 5.22 / 7.56; warn on cream 4.96 / 7.77 (not
allowed on blush: use ink); paprika on blush 4.24 / 5.76; border on bg 3.17 / 3.23. Colour is
never the only channel: a ring carries a numeric label, a state word for screen readers and a
check at settled.

The tables below record the pre-A palette for the knowledge-state colours (`kn-*`, unchanged
and re-verified by the script against the new surfaces) and the method.

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

**Added by ADR-059:** `route-fade` (160ms fade + 6px rise on a path change), `card-in` as a
`--i`-staggered grid arrival (40ms × index, max 6), skeleton `shimmer` (translateX of a
pseudo-element, 1.4s), the current node's `halo` (2.4s, opacity/transform on a pseudo-element;
its end keyframe equals the static state so reduced motion lands cleanly), `bar-grow`
(`scaleY`, bottom origin), the check draw (240ms) and the count-up (≤600ms, `useCountUp`,
off under reduced motion; the final value is announced once). `--ease-standard` is
`cubic-bezier(0.2, 0, 0, 1)`.

**Second documented exception: the ring stroke.** `ProgressRing` animates `stroke-dashoffset`
over 700ms: an arc's length has no transform equivalent. It is learner-visible state change on
a rare event (a unit's share moved), not the answer loop. The reduced-motion rule flattens it.

Rules:

- **120–320ms, no exceptions** (the ring's 700ms above is the named exception). A learner answering 200 cards a day sees `answer-correct` 200
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

**Rings (ADR-059).** A progress ring is allowed in exactly one case: a **derived share with a
counted denominator and a numeric label** — a unit's settled words out of its total, «42%».
Never a streak ring, never an XP or «activity» ring, never a daily-goal ring, never anything
that only goes up. It may go down (a lapse unsettles a word). Colour follows §14.7's ring
ramp and is never the only channel.

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

**The ring ramp (ADR-059).** `ProgressRing` colours its stroke from a dedicated ramp —
`--ring-start` (ember/vermilion) at 0, `--ring-mid` (blue) near 0.35, `--ring-end` (teal-green)
from 0.7, mixed with `color-mix(in oklch)`. It is not the knowledge scale and not
`destructive`: an empty ring is "not started", not a failure; `destructive` stays for real
errors only.

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
`views/TodayView.vue` (off the refetched plan, because "done" is a fact about the review deck), a
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
| Vine of leaves and buds | `art/Vine.vue` | no longer placed (login and setup now use the paprika string, §14.10) |

**Rules.** `currentColor`, 1.5 px strokes, round caps, no animation, under 2 KB a motif,
`aria-hidden` — the sentence beside a drawing carries the meaning. The default tone is 70 % of
`--muted-foreground`; `accent` is 50 % of `--primary` and marks an arrival (the done screen,
the doors). Decorative contrast sits at 2–3:1 by design: a drawing that competes with the
text is the wrong drawing. A drawing never depicts a figure — no rings, flames or counters
(§14.5) — and never appears on a `Note`, on a card, on the Progress screen, or in the header
of a screen. `Illustration.vue` (`name`, `size` sm/md/lg, `tone`) is the one way to place one;
`EmptyState` takes `art` in place of `icon`.

**Two-tone decorative pieces (ADR-059).** Six more, line in `currentColor` (primary) plus a
fill from `--art-fill` (a 12–18% tint; no gradients, no text, under 2 KB each, 120×120 except
the tram strip 360×80; `scripts/check_art.py` enforces size and the bans):

| Piece | Component | Allowed place |
|---|---|---|
| Chain Bridge | `art/ArtChainBridge.vue` | Library empty |
| Parliament dome | `art/ArtParliamentDome.vue` | Grammar, nothing started / empty |
| Tram on the Danube | `art/ArtTram.vue` | Today header strip |
| Thermal-bath cupola | `art/ArtThermalDome.vue` | Today «nothing due» rest state |
| Paprika string | `art/ArtPaprikaString.vue` | Words empty |
| Balaton waves | `art/ArtBalaton.vue` | Listen / Ear empty |

(D3b also places: the Chain Bridge in the Library header and the Resources empty state, the
parliament dome in the Grammar header, the paprika string above the login and setup wordmark,
the balaton waves on the Ear start card, the tram on the 404 — all outside any `Note`, card
body or metric.)

Placed with `Illustration` (`chainBridge · parliament · tram · thermal · paprika · balaton`).
**Forbidden, unchanged:** on a graded card, a `Note`, the Progress screen's metrics, or as a
measure of anything. The one-colour embroidery pieces above keep their rules.

**What nobody caught** goes to the handler in `main.ts` (ADR-038): the stack to the console,
one toast in the learner's language through `lib/errors.ts`, the same sentence at most once
per two seconds, and a 401 that got that far clears the session and opens the login form with
the way back. Screens still catch their own failures and say them in place — the handler is
the floor, not the pattern.

### 14.10 The lively pass on the remaining screens (D3b, ADR-059)

Visual only; no behaviour, route or gate logic changed. Browser **not** exercised when written.

- **Progress** — a two-column dashboard from 1024px (level + curve left, measured tiles right;
  one column below). The level panel is a `mist` card: the level label, a confidence chip and a
  `ProgressRing` for *coverage* (≈N % of a text, numeric label counting up). The ring is hidden
  from assistive tech because the sentence beside it says the same in words. Per-band bars have
  tinted tracks and the 80 % marker. Tiles (`StatCard`): words in the deck, learned, solid in
  grammar, free practice — measured counts that can go down. The lemma curve has a soft area
  tint; the activity calendar steps from tint tokens; the locative grid uses sage (solid) and
  cream (needs work) with the percentage printed in every cell. Level checks: a ring for «N из
  M слов» (counted denominator) with the sentence as the accessible text, tinted history rows
  (sage passed, cream failed — no red). No illustrations here.
- **Grammar** — header is a `PageHero` with the parliament dome. Topic map discs and rows
  tinted by stage (not started mist, one-at-a-time light sage, mixed sage, rule-only cream,
  mastered primary + check; the stage word is always printed). Lesson/exercise tabs use a soft
  mist active state; the gate explainer is a cream card with an accuracy bar and the printed
  80 % marker. Options are ≥48 px; a correct option draws a check, a wrong one a cross, with
  sage / blush wash and ink text (no red text on tint).
- **Library / Reader** — `PageHero` with the Chain Bridge; tabs tinted; rows are soft cards;
  CEFR chips tinted by band (A1 sage, A2 mist, B1 cream, B2 blush; the level text is always
  inside). Reader: cream paper surface, unknown words get a mist wash plus a dotted underline,
  learning words a cream wash plus a solid underline (never colour alone); the word sheet has a
  mist header and the «Знаю» button is sage. No illustration on the reading surface.
- **Settings** — each section title carries a small tinted icon chip (`SectionChip`, decorative).
  Component cards are tinted by state (ready sage, installing mist with a ring and percentage,
  failed blush, not built cream) and always print the state word.
- **Login / Setup** — a cream-washed page; paprika string over the wordmark; the form card
  stays the single raised object.
- **Listen / Ear / Users / Resources / 404** — tinted option and list cards; balaton on the
  Listen empty state and the Ear start card; Resources groups carry tinted icon chips; the 404
  shows the tram strip.
- **Shell** — the active sidebar row is a mist pill with a primary icon; bottom tabs put a mist
  pill behind the active icon; the topbar due chip is cream, the streak sage, the level chip
  mist. Text on every tint is ink; paprika/warn colours are never used for text on a tint.
### 14.11 Brand mark

A flat, geometric mark: the Hungarian Parliament's central dome (lantern, spire, two flanking
pinnacles, arched roofline) in cream `#fbf7ef` on a rounded teal tile `#1d5c6e` (radius 22 %),
a paprika `#b8402a` pennant on the spire and two cream wave strokes for the Danube. Flat
colours, no gradients, no text; under 1.5 KB. Decorative chrome like §14.9: it never draws a
metric.

- Master: `web-ui/public/favicon.svg` (64×64). `icon-mask.svg` is the maskable variant (full
  bleed, glyph inside the 80 % safe zone); `icon-small.svg` is the simplified 16/32 px variant
  (no arches, one wave). `BrandMark.vue` carries the same geometry — change both together.
- Raster files (`favicon-32.png`, `favicon.ico`, `apple-touch-icon.png`, `icon-192/512.png`,
  `icon-maskable-512.png`) are generated and committed: run `scripts/make_icons.sh` (needs
  Google Chrome headless and python3 only).
- `manifest.webmanifest` + `<meta name="theme-color">` (light `#fbf7ef`, dark `#0a0e12`).
- Components: `components/brand/BrandMark.vue` (`size`, `tile`, `title`; `tile=false` is the
  glyph in `currentColor`) and `BrandWordmark.vue` (mark + «tanulom» in the display serif).
- Minimum size 16 px (use `icon-small.svg` below 32 px); clear space of one eighth of the tile
  on every side. The tile keeps its own colours, so it reads on light and dark pages alike.
- No tagline beside the mark: it truncated in the sidebar and added nothing.

