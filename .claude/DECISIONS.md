# Architecture decision record

Each entry: the decision, what was actually considered, why this won, and what it costs.
An ADR is never deleted — if a decision is reversed, a new ADR supersedes it and says so.

---

## ADR-001 — Go core with a single static binary; sidecars are optional

**Status:** accepted · 2026-08-22

**Context.** The learning engine needs SRS, a database and an HTTP API. The *speech and NLP*
parts (HuSpaCy, Piper, Whisper) live in the Python/ONNX ecosystem. The user named
[`antlko/upmonitor`](https://github.com/antlko/upmonitor) as the deployment reference: one
`docker run`, one volume, one tiny container.

**Options considered.**

| Option | Verdict |
|---|---|
| Everything in Python (FastAPI) | Rejected — single language and a natural fit for the NLP stack, but a 3–6 GB image, slow cold start, and it abandons the "tiny container" ergonomics that were the stated reference |
| Go core + Python NLP baked into the *same* image | Rejected — one image to deploy, but the base image becomes heavy for every user, including the majority who never enable speech |
| **Go core + optional sidecars** | **Accepted** |

**Decision.** A static Go binary (Fiber v3, `modernc.org/sqlite`, `CGO_ENABLED=0`, distroless)
embeds the Vue SPA and contains the entire learning engine. NLP, TTS and ASR ship as separate
containers behind docker-compose profiles.

**Consequences.**
- The core image stays small and starts instantly; `docker run` with no arguments is a complete
  product.
- Users who want speech opt in explicitly and knowingly pay the disk cost.
- **Cost:** more moving parts in the full configuration, and a capability-probing layer
  (`GET /api/capabilities`) plus graceful degradation in every feature that touches a sidecar.
- **Hard constraint this creates:** morphological analysis cannot call Python at runtime →
  [ADR-002](#adr-002).

---

## ADR-002 — Morphology is precomputed into a shipped language pack

**Status:** accepted · 2026-08-22 · *follows from [ADR-001](#adr-001)*

**Context.** The product thesis requires decomposing every wordform into lemma + morphological
features. The best Hungarian analyser (HuSpaCy `hu_core_news_lg` — lemma 97.6%, POS 96.7%) is
Python. [ADR-001](#adr-001) forbids Python in the core.

**Options considered.**

| Option | Verdict |
|---|---|
| Require the NLP sidecar for all analysis | Rejected — makes the core useless alone, contradicting [ADR-006](#adr-006) |
| Port a finite-state analyser (emMorph/hunmorph) to Go | Rejected — large effort, and still worse than HuSpaCy on disambiguation |
| **Precompute `wordform → (lemma, upos, feats)` at build time** | **Accepted** |

**Decision.** A Python pipeline (`langpack/`, build-time only) runs HuSpaCy over corpora and
expands Wiktionary paradigms into a `wordform` table shipped in a read-only SQLite file. The Go
binary does an **indexed lookup**, not analysis.

Fallback ladder:
1. exact `wordform` hit → done;
2. unknown form → suffix-stripper built from `morph_feature` allomorph tables;
3. ambiguous form → frequency prior, and the analysis is flagged `Uncertain` in the API and UI;
4. `nlp` sidecar running → HuSpaCy disambiguates in context (`vár` = castle vs. waits).

**Consequences.**
- Analysis is O(1) and works fully offline with zero dependencies.
- The pack is a versioned artefact, `ATTACH`ed read-only, so upgrading is a file swap that
  cannot corrupt user data.
- **Cost:** pack size (millions of rows), a build pipeline to maintain, and imperfect
  disambiguation without the sidecar.
- **Cost:** pack quality is the ceiling on product quality → the golden test against UD
  Hungarian is a release gate, not a nicety.

---

## ADR-003 — FSRS-6 via `go-fsrs/v4`, instantiated over two state spaces

**Status:** accepted · 2026-08-22 · see [R3](research/03-fsrs.md)

**Context.** The source plan's rule §7.1.3 is "никакого своего SRS-алгоритма". Correct. But the
morph state space has no representation in Anki, so scheduling cannot be delegated to it either.

**Decision.** Vendor
[`github.com/open-spaced-repetition/go-fsrs/v4`](https://github.com/open-spaced-repetition/go-fsrs)
(FSRS-6) unmodified. Write **no scheduling maths of our own**. Instantiate it twice: once over
cards, once over morphological features. Keep `.apkg` export first-class so the learner is never
locked in.

**Consequences.**
- We honour the rule in substance: no invented algorithm.
- A single cloze answer emits 1 card review + N feature reviews — the mechanism that makes the
  weakness heatmap possible.
- **Cost:** `go-fsrs`'s optimizer sub-package is `in_development`, so v1.0 ships FSRS-6 **default
  parameters** for everyone. Per-user optimisation is deferred to v1.3 via an optional sidecar.
  **This limitation is stated in the UI**, not only in docs.
- **Cost:** five card directions per note means 5× the review load; the new-card governor
  therefore counts *cards*, not notes.

---

## ADR-004 — RU + EN, both locales complete

**Status:** accepted · 2026-08-22 *(user decision)*

**Context.** The learner is a Russian speaker in Hungary. But HU→EN lexicographic data
(Wiktionary/kaikki: 72,781 forms, 49,653 noun senses) is far richer than HU→RU.

**Decision.** Both UI locales complete and switchable. Glosses carry both languages where
available; **where an RU gloss is missing the EN gloss is shown and visibly labelled as a
fallback** — never silently substituted.

**Consequences.**
- The app is publishable to a wider audience from day one.
- **Cost:** roughly 2× the copywriting, and a standing rule that a PR adding an `en` key without
  its `ru` counterpart fails review.
- **Cost:** the language pack carries two gloss sets.

---

## ADR-005 — Speech and LLM are pluggable; local is the documented default

**Status:** accepted · 2026-08-22 *(user decision)* · see [R10](research/10-llm-tutors.md)

**Context.** [R10](research/10-llm-tutors.md) finds **voice input is a significant positive
moderator** for chatbot-assisted L2 learning — so speech is core, not decorative. But local
models need real hardware, and cloud models need keys and leak text.

**Decision.** A provider interface per capability (`TTS`, `ASR`, `LLM`) with three settings each:
`local` / `openai-compatible` / `off`. Local is the documented default. Every capability is
probed at runtime and surfaced with status and latency in Settings → Services.

**Consequences.**
- Users choose their own privacy/quality trade-off.
- **Cost:** an abstraction layer and two implementations per capability.
- **Constraint:** because Hungarian ASR WER ≈ 11% and LLM Hungarian is materially weaker than
  its English, both paths are gated — ASR feedback behind a confidence threshold, LLM output
  behind corpus attestation. Neither is ever presented as ground truth.

---

## ADR-006 — v1.0 is the core with zero external dependencies

**Status:** accepted · 2026-08-22 *(user decision)*

**Decision.** v1.0 = SRS + Reader + Grammar + HVPT + Listen + Progress + deployment. Speech and
LLM land in v1.1. **Every phase ends shippable.**

**Consequences.**
- There is a usable product early, and stopping after any phase leaves something real.
- **Standing invariant:** the app must remain fully usable with every sidecar stopped. This is
  verified at each release, not assumed.

---

## ADR-007 — upmonitor's deployment shape, adopted deliberately

**Status:** accepted · 2026-08-22

**Decision.** Copy the patterns that make upmonitor pleasant to self-host:

- `//go:embed all:dist` — the SPA lives inside the binary;
- `CGO_ENABLED=0` + `modernc.org/sqlite` — a genuinely static, cross-compilable binary;
- goose migrations **embedded and run inside `db.Open`** — no migrate step, which matters
  because distroless has no shell to run one;
- three-stage Dockerfile with `--platform=$BUILDPLATFORM` cross-compilation;
- one config/data directory as a single volume;
- central Fiber `ErrorHandler` rendering `{"error": …}`;
- `config.yaml` for hand-editable settings, SQLite for history, volume and **secrets**;
- `CLAUDE.md` as an index with `docs/` holding the depth.

**Where we deliberately differ.**

| upmonitor | tanulom | Why |
|---|---|---|
| `config.yaml` is the source of truth for the domain (services) | `config.yaml` holds only settings; the domain lives in SQLite | Learning data is high-volume and relational; hand-editing it is not a use case |
| Single SQLite file | Two — read-only pack + read-write user data | Lets the pack be swapped and versioned independently ([ADR-002](#adr-002)) |
| No optional services | compose profiles for `nlp`/`speech`/`llm` | [ADR-001](#adr-001) |

**Consequences.** A user who has deployed upmonitor already knows how to deploy this.
