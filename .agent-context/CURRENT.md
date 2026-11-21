# Grammar exercises, phase 2 (started 2026-11-21)

## Phase 1 (DONE, uncommitted): person/qword/sort kinds, anti-repeat, lesson examples, ADR-086, R25. Full gate PASSED, 3 browser QA rounds.

## Phase 2 request (user, RU): more material for conditional (13 items) / imperative (0); pair matching; PICTURES ("think how; if fun, research and build");
listening mentioned but DEFERRED (ADR-084 removed the ear trainer at the user's request -> ask the user before building).

## Decisions (adviser memo, accepted)
- Imperative has 0 because pack never tags Wiktionary "subjunctive" (langpack/pipeline/wiktionary.py TAG_TO_FEATURE) -> map to Mood=Imp; _as_form_of merges senses into ONE feats dict (must emit one row per sense or olvassuk loses indicative). Needs pack rebuild + starter .gz regen (feasibility check first: are raw sources local?).
- `person` bare mode (no sentence; cue "(én · látni)" + mood label; answer = attested wordform with Mood/Person/Number match; only when sentence candidates <30; skip cells with >1 attested form).
- `match` kind: one lemma, 3-5 cue->form pairs (pronoun->tense form; question word->case form), reuse sort Columns/Tiles/Parts (Kind.MultiPart()), one review per feature, tap max Good, no placement; MatchBoard.vue + ChoiceChip.vue extracted from SortBoard.
- Pictures: Noto Emoji SVG (Apache-2.0) for nouns + ~10 ORIGINAL line-drawn places; embed package backend/internal/pictures (go:embed catalog.json + svg/, served GET /api/pictures/:id.svg with CSP/ETag, sanitiser test, <=1.5MB); NOTICE + THIRD_PARTY_LICENSES; coverage script langpack/tools/picture_coverage.py; curated catalog <=150 (start ~60). `scene` grammar kind (object in/on/at place; Hol?/Hova?/Honnan? with Ine/Sup, Ill/Sbl, Ela/Del; answer = attested form of the place noun; picture never defines correctness; alt text; arrow not colour-only; slide animation off under reduced-motion). Picture cue on word cards (additive, gloss stays) later in MVP step 6.
- Reviewer MANDATORY for pipeline+drill and for SVG serving.
## Waves
W1 parallel: S0 pipeline (langpack/) | P4 pictures pkg | M3 backend drill match+bare person
W2: scene backend (after M3+P4) ; W3 frontend (MatchBoard, ScenePicture, DrillItem, card picture cue, credits) ; W4 docs ADR-087..089 + R26 ; then review + QA agent (browser) + full gate.
## Status
- [ ] S0  [ ] P4  [ ] M3  [ ] scene  [ ] frontend  [ ] docs  [ ] review  [ ] QA  [ ] gate
