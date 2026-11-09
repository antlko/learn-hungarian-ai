# CURRENT — polish pass (started 2026-11-08)

## Request (owner, 16 points, condensed)
1 Install unclear; Makefile has ~40-min langpack build not mentioned in README — is it needed?
2 No mobile version of learning.
3 Hard to understand how it learns; bad that "no mistake in a row" is required.
4 Statistics awful: no interaction, no clear progress view.
5 What are MT (translator) and HuSpaCy (nlp)? UI says "not implemented". Consider smallest HuSpaCy model.
6 Admin settings: click install, progress bar, "relaunch container" message. Verify HuSpaCy works in a small container.
7 Plan screen: boring/complex; want Duolingo-like roadmap on main screen; reader-mined words must not hijack topics.
8 Grammar topics UI: same complaints.
9 Apply UX best practice to 7 and 8.
10 Top-right icons uninformative; on login show score + Hungarian level/sublevel.
11 Review app idea vs what is actually efficient for learning Hungarian.
12 Well optimised.
13 Left menu dynamic.
14 Words screen: "учу" only — show stage/progress.
15 Review and greatly expand standard learning material sets.
16 Update docs, human-readable feature list.
Deliver: finished result only. Use researcher/adviser/developer/reviewer. Keep main context.

## Hard constraints (CLAUDE.md)
No XP/levels/leagues/gems (a *measured* level estimate with stated method is competence signal — adviser to confirm wording), FSRS unmodified, both ru+en locales, docs updated with change, make test is the gate, no frontend test framework.

## Status
- [x] research done -> findings-install.md, findings-frontend.md, findings-learning.md
- [x] adviser plan -> PLAN-polish.md (batches B1..B10; READ IT after compaction)
- [x] B1 BE learner API — done, reviewed (approved w/ notes), notes fixed
- [x] B4 FE shell — done (type-check/lint/build/docs-check PASSED; visual NOT RUN). KeyBar must get `bottom: var(--bottom-tabs-h)` in B6.
- [ ] B5 FE Today roadmap (running, agent a901c046bacfebc41); B2 BE pack install (running, agent ae79437ffcf6aaafc; review mandatory)
- DONE (checks pass, visual NOT RUN): B5 Today roadmap, B6a review mobile, B6b words stage, B7 grammar (ADR-053), B2 backend (review running)
- RUNNING: B3a course vocab (agent a7bf270b07f2b1b50), B2 reviewer, B8 stats (a334ed80c4d03f1c2), B9a Settings Components UI (a6c4a24e3acddc273)
- UPDATE: B2 re-reviewed APPROVED WITH NOTES (low: marker sha not re-hashed at boot; fail() on rename deletes parked pack). B8 stats, B9a settings UI, B11 perf (entry 94→34KB) done. Running: B3a (aad9741ad0c4aa28c), B9b nlp (af92d32d85c6560ff). Flaky pre-existing test: TestTutorStopKeepsThePartialAnswer under CPU contention (unrelated).
- DONE since: B3a course (core 1399w/24u + 12 theme units 539w), B3b-1 grammar +5 topics (33 total, 13 drillable; ADR-055; fixed 2 langpack tests), B9b nlp sidecar (reviewed, fixes applied; ADR-054), B9c nlp UI, B11 perf
- RUNNING: B3b-2 reading sets BE+pipeline+starter (acc379bffe6588b20), B3b-3 library sets UI (a9f56321ef1f0ee7c)
- DONE: B3b-2 sets BE (36 sets/898 sentences, ADR-056), B3b-3 library UI. RUNNING: B10 docs dev (a06c45f3b890d0777), FE reviewer (a53ac8d9f046cd2d5), BE/pipeline reviewer (a8e23a9c2217e3418). After: apply review fixes, make test (race, 25m), browser check by orchestrator, final report.
- BROWSER TEST ROUND 1 (scratch server :8099 + vite :5199, scratch data dir under scratchpad/scratch1, test acct test@example.test; kill both when done; remove nothing in data/): FIXED by me: ComponentsPanel byId TDZ crash (Settings blank). Sent to developer a72ee99cedb5e5ff8: D1 trash.title key, D2 Today next action with new cards, D3 accent strip clip, D5 dup subtitle, D6 /grammar map landing, D7 band homonyms, D8 English RAM hints, D9 library sets collapse, D10 memory bar only young/mature, D11 gate dup text. After: rebuild SPA+binary, re-test, run make test, final summary. All reviews done (FE/BE/sets APPROVED WITH NOTES; fixes applied).
- ROUND 2 (owner feedback after using the app; owner WIPED data/ learner DB, added `tanulom reset-password` + make reset-password/reset-learner): too much explanatory text → InfoTip.vue ("?" popover, written by me) + human copy; roadmap w/o «Месяц N» → tiers Базовый/Средний/Продвинутый (course extended to 3 finite tiers: owner request overrides 'course ends at 24' invariant → new ADR); show words learned/in deck/in course; unit label «пройден» after mere enrolment was misleading → «выучен»/«учу N/M»; mobile unit sheet sticky CTA; Review start redesign; Library/Reader mobile. Running: U1 BE+content (a5eb2ef7b80f575e9), U2a Today FE (add6abc7bb5396a5a), U2b copy diet/review start/library (a7a2a2e3dce6db736). After: rebuild, browser-verify (scratch server, fresh scratch data dir — NEVER data/), make test, final report.
- ROUND 3 (owner screenshots): Today too cluttered → Duolingo-like path only; header overlap = main scroll position persisted across routes + auto-scroll on load; page containers differ per view → one container in AppShell; «Месяц 1»/«впереди»/«лемм»/checkpoint wording to purge; owner's data pack (data/langpack-hu-v1.sqlite, old full pack) only had Basic tier → I ran `make refresh-curriculum` on it (backup data/langpack-hu-v1.sqlite.pre-tiers.bak; fixed Makefile PYTHON absolute path bug). Running: A1 shell (a956e377790a25ac4? check ids: ae042c4e4259aabbe=A1, a956e377790a25ac4=A2, a65b6f75b93d8b20e=A3). After: rebuild+browser verify 375/768/1280, measure lag, make test.
- ROUND 4 (owner): (1) small fixes A6 (settings input alignment, НОВОЕ, keybar centering) agent a513f3f8466d81452; (2) SOFT GATE: next unit CLOSED until >=70% of previous unit's words "settled" (mature >=21d or known); animated progress ring red→blue→green + %; "ready for next?" prompt; override always possible (ADR-013 amended) — backend G1 agent aea21fc5ef159d660 (adds unit.settled, course.unlock{share,settledDays}); (3) LIVELY REDESIGN: dashboard like owner's reference (soft coloured cards, rings, illustrations, right stats column, path stays), colours/animations in all workflows; researcher ae9e27237691eb362 → then adviser design spec → developers. Constraint: no XP/points; illustrations chrome only (ADR-037 to be amended carefully: decorative on Today banner/empty/done, never as metric).
- (old) REMAINING: B10 docs (README install rewrite, docs/FEATURES.md, reconcile pack numbers 5,059 vs 5,132, services/README sweep, ADR numbering: 051/052 unassigned; B6 never wrote its ADR), final `make test GO_TEST_FLAGS="-race -count=1 -timeout 25m"`, orchestrator browser check 390px+desktop (preview_start), final reviewer pass over FE batches? (visual), final summary
- (old) TODO: B3b (grammar topics numbers/time + sentence sets + library sets API/UI), B9b (nlp sidecar + Go client + ingest hook), B10 docs/README/FEATURES, final: ADR number collisions (B7 used 053; ADR-051/052 unassigned → fix numbering/refs), make test, orchestrator browser check 390px + desktop, then report
- (old line) Then: B3 BE content (after B2) ∥ B6 FE (after B5) -> B7 -> B8 -> B9 -> B10; final visual check in browser at 390px + desktop by orchestrator
- [ ] review (mandatory: B1, B2, B9), final `make test GO_TEST_FLAGS="-race -count=1 -timeout 25m"`

## Decisions
- Adopted adviser plan as-is. Level = estimate+text coverage (measured, can go down). FSRS steps unchanged; copy explains. Nothing published/pushed/committed. nlp built opt-in, ~2.4GB RAM floor.
- Subagents have ~40-turn limits: keep batches small, resume via SendMessage.
