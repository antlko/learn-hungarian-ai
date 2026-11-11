# CURRENT — 2026-11-11 batch (8 requests + 3 follow-ups)

## Status: implemented, reviewed, verified; NOT committed

## What landed (ADR)
- 064 practice answers are real reviews (except «Знаю» and new cards); random practice deals seen cards first.
- 065 review sitting survives navigation (in memory, same account/day/mode/scope); «Продолжить / Начать заново».
- 066 reading sets: curated «Фразы» (Tatoeba, exclusion list, chain order) + labelled AI-written «Диалог» sets
  (`source='authored'`, reading sets only); preview GET /api/library/sets/:code.
- 067 MT fetched when a crib is shown, never by the queue.
- 068 services: short names, `Service.Paused`, POST /api/admin/services/:name/paused, switch in account menu.
- 069 NLP sidecar removed.
- 070 word progress 0–100 (display-only): WordProgress bar in review, Words list, inspector, Progress chart.
- 071 «Знаю» covers every note of the lemma; enrolment skips known lemmas.
- Also: «учу» copy, light theme default, options/expected in lemma case, tapped option is spoken,
  Words rows open the inspector. Research note R20.

## Checks
- `make test GO_TEST_FLAGS="-race -count=1 -timeout 25m"` PASSED (before the last frontend-only fix);
  type-check, lint, docs-check PASSED after it. Reviewer APPROVED both rounds.
- Browser (scratch data): theme, resume, practice counted/undo, bar, words, inspector, chart, service switch,
  library preview, lowercase options, option speech, «Знаю». ru/en, light/dark, 375 px.
- NOT verified live: auto MT (no MT sidecar on the scratch instance).

## Open
- Owner's full pack in `data/` still has old sets until rebuilt (not touched).
- «ИМЯ» capitals from the owner's screenshot not reproduced.
- Cloze distractors for A1 interjections include vulgar words (picsába, basszus, francba) — pre-existing.
- Known small: reinstalling managed MT clears a pause; young/mature flip near 21 d can show up to −9.
