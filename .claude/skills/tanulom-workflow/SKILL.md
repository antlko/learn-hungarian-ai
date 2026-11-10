---
name: tanulom-workflow
description: Project protocol for tanulom — how the orchestrator uses the researcher/adviser/developer/reviewer agents, keeps the main context small, and verifies UI changes in the built-in browser. Load at the start of any non-trivial task (feature, bug, refactor, doc change) in this repo.
---

# tanulom workflow

Goal: finish correctly while the **main conversation stays small**. The docs are ~700 KB; the main
context must never ingest them. Agents read, the orchestrator decides.

## Roles (`.claude/agents/`; these override the global ones of the same name)

| Agent | Model | Use for | Never |
| --- | --- | --- | --- |
| `researcher` | Haiku, read-only | Anything needing docs/ or .claude/ lookups, "where is X / what did we decide / how does Y work", code surveys, library docs | edit, decide |
| `adviser` | Opus, read-only | Ambiguous/cross-cutting decisions, multi-file plans, invariant conflicts | implement, research broadly |
| `developer` | Sonnet | Implement an approved contract + tests + doc section + validation + browser check | design, widen scope |
| `reviewer` | Sonnet, read-only | Independent verdict on the real diff, re-run checks | fix, trust self-reports |

**If these agents exist, use them.** Do the work inline only when it is a 1–3-file obvious fix, a pure
question you can answer from CLAUDE.md, or a one-line doc edit.

## Flow

1. **Triage.** Read only CLAUDE.md (already loaded). Pick the row in its topic map.
2. **Research (if the answer is not in CLAUDE.md).** One `researcher` call with a precise question and the
   doc/section hints from the topic map. Several independent questions → several researchers in one message.
   Ask for ≤40 lines with `path:line` evidence. You open a slice yourself only to confirm a key claim.
3. **Decide.** Simple → you. Ambiguous, touches an invariant, schema/API/grading, or non-trivial plan →
   `adviser` with: user requirements verbatim, constraints, decisions so far, researcher evidence (pass
   large evidence by file path), the question. It returns a memo; you own the decision and tell the user.
4. **Implement.** `developer` with a contract (below). One developer per set of files; never two on
   overlapping files; coupled changes stay in one contract. Independent module changes may run in parallel.
5. **Verify UI in the built-in browser (every UI change, no exceptions)** — see next section.
6. **Review.** `reviewer` is mandatory for: `internal/srs` / replay / FSRS wiring, drill grading and
   options, migrations or `lemma_state`/`morph_state`/`review_log` shape, API/DTO/schema changes, config
   or secrets, tutor/LLM/MT guardrails, roles/auth, profile export/import, langpack pipeline output, or any
   large cross-cutting diff, or when the user asks. Skippable (say so) for copy/CSS/docs-only/tiny
   low-risk edits. Give the reviewer requirements + acceptance criteria + `git diff` instructions only —
   not the developer's reasoning. On CHANGES REQUIRED: developer fixes, reviewer re-checks only those
   findings; after two loops decide or ask the user.
7. **Report** to the user: outcome, key decisions, checks actually run (PASSED/FAILED/NOT RUN with
   commands), remaining risks. No narration of handoffs.

## Developer contract (scale to the task; trivial = two lines)

Objective · Context (decisions made, relevant invariant names/ADR ids — not their text, file paths) ·
Constraints (restate: both locales, no whole-doc reads, `CGO_ENABLED=0`, no commits) · Scope (files owned /
must not touch) · Acceptance criteria (observable; include the browser check for UI) · Validation commands ·
Expected report shape. Subagents do not see this conversation — restate anything that matters.

## UI verification in the built-in browser

Required for any change under `web-ui/` or any backend change a screen displays. "Type-check passes"
is not verification.

1. Use a scratch data dir and fixture account, never `data/` (see memory `scratch-verification`).
   Start via `mcp__Claude_Browser__preview_start` with `{name: "tanulom"}` (`make dev`, Vite on :5173);
   point the Go side at the scratch dir.
2. Navigate to the changed screen; exercise the actual flow (type, click, keyboard, the error path).
3. `read_console_messages` and `read_network_requests` — no errors, no unexpected 4xx/5xx.
4. `read_page` for structure/text; `computer screenshot` as proof of the final state.
5. `resize_window`: `mobile` if layout changed; `colorScheme` light AND dark if colours/components changed;
   switch locale ru ↔ en if copy changed. Reset to `desktop` afterwards.
6. If the browser tools are unavailable to a subagent, it reports `NOT RUN` and the **orchestrator** does
   this step itself (or delegates to a `developer` that has the tools). Never mark UI work done unverified.

## Context hygiene

- Never `Read` a doc over ~15 KB whole; `grep -n '^## '`, then offset/limit. Prefer a `researcher`.
- Pass facts between agents as short summaries or file paths, not transcripts.
- Long multi-stage work: keep a compact `.agent-context/CURRENT.md` (request, requirements, decisions,
  status, open questions; rewrite, don't append; tell the user when first created). Re-read after compaction.
- Ask the user only what cannot be found by inspection and where a guess is costly; give a recommendation.
  Get approval before destructive, externally visible or irreversible actions.
