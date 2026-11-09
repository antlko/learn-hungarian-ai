# Findings: frontend UX (researcher, 2026-11-08; bundle sizes & 390px layout unverified)

## Shell / mobile
- Nav static: AppSidebar.vue:56-65 hardcodes 8 learner items (Today, Plan, Review w/ deck.dueNow badge, Read, Grammar, Tutor, Listen, Ear); :74-77 two disabled "coming soon" rows (Speak, Write); :84-88 footer Progress, Users (admin), Settings.
- Topbar (AppTopbar.vue:88-179): due chip, streak flame + popover, CapabilityDots (hidden <sm), locale, theme, sign-out. No level/score.
- <md: sidebar hidden (AppShell.vue:103-106), hamburger drawer (:141-148). No bottom tab bar. ReviewStartView has no responsive classes. KeyBar fixed bottom offset by md:left-(--rail) (KeyBar.vue:85) — keyboard chrome on touch. ReviewView min-h-[70vh], lg: rails (ReviewView.vue:836-847,1541).

## Today / Progress
- TodayView.vue:175-182 GET /api/stats/overview: minutes vs goal, 3 loop cards, activity chart, Loop-4 attest card.
- ProgressView.vue:42 overview + GET /api/stats/mastery: lemma curve static SVG (:296-325, no hover), practice bars with title tooltips, feature heatmap, locative grid (LocativeGrid.vue:94 raw feature id in title), checkpoints, ActivityChart (hover + table toggle).
- Backend handlers_progress.go:115-245 returns lemma counts, curve, streak, today, practice, days(<=400), Breakdown, checkpoints, nextCheckpoint, blockedLevels, examDate.
- No learner level. Pieces: checkpoint specs stats/checkpoint.go:34-45 (m1=300,m3=900,m6=2000,m12=3500,m18=6000, GatesLevel), text-level estimator mining/level.go:85-172 (90% lexical band share, grammar hits>=3), lemma.cefr (frequency-rank estimate).
- Chart targets hardcoded handlers_progress.go:172 {"A1":900,"A2":2000,"B1":3500,"B2":6000} — duplicates checkpoints (violates one-place defaults).

## Plan / course
- course_unit + course_word in pack (course/course.go:39-113); course_progress = enrolment only; done derived (course.go:213-226); current = first non-done (:217-220).
- Owner's "new word from text switches topics": reader words complete a unit -> current jumps; PlanView opens on plan.current (PlanView.vue:67-69).
- UI: SyllabusList (month-grouped buttons, 4 dot states, SyllabusList.vue:22-115), UnitDetail (large card, UnitDetail.vue:104-206), desktop 2-col sticky 320px rail (PlanView.vue:163-236).
- Roadmap path: reuse GET /api/course (handlers_course.go:103-130); frontend only. Caveat: inPlaySet swallows errors -> empty set (handlers_course.go:70-73); IN clause one var per lemma (:64-65) may hit SQLite var limit.

## Grammar
- GrammarView.vue:110-128 drillsLead (ADR-043); phases drill/schedule.go:36-72 (locked, explained, blocked, interleaved, mastered); PhaseBadge.vue; TopicMap.vue:26-109 groups by CEFR level, grid cards, lock icon; rail GrammarView.vue:559-598 repeats it. Confusing: phase names, gate explanation, lesson/drill toggle, "rule card only" cards.

## Words
- Words list shows badge "Учится" (ru.json:1114); "учу" is review-card stage label (memory.stage.learning ru.json:578). WordsView.vue:146-192: memory bar, next due, lapses, badge.
- handlers_words.go:27-42 WordRow lacks stage/stability/reps/stepsLeft; srs/memory.go:72-114 has them (only on review card today).

## Perf
- Router lazy (router/index.ts:13-63); i18n.ts:2-3 eagerly imports both locales; no manualChunks.
- handleWords (handlers_words.go:76-245) loads all lemma_state + cards, computes in Go, paginates in Go; card-query error path doesn't close rows (:100-106).
- recordInteraction (handlers_progress.go:26-34) runs full-deck Breakdown per answer (srs/breakdown.go:96-107).
- handleOverview ~10 queries.
