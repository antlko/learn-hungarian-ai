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

> **Superseded in part by [ADR-011](#adr-011).** The `.apkg` clause is withdrawn: the format
> cannot carry the morph state space, so exporting one would hand the learner half the truth.
> The FSRS decision above stands unchanged.

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

---

## ADR-008 — Drill items are held server-side between serve and answer

**Status:** accepted · 2026-08-22

**Context.** A grammar item has an expected answer. The client has to render the item, and the
server has to grade it. Three ways to bridge that:

1. **Send the answer with the item.** Simplest, and wrong twice over: the answer is visible in
   devtools, so the exercise stops being retrieval practice; and the client would have to echo
   back which features the item graded, which means a client can write anything it likes into
   `morph_state`. The mastery heatmap is the one number in this app that has to be trustworthy.
2. **Sign the item** (HMAC over topic/kind/features/expected). Stateless, but it needs a key,
   a rotation story, and it still ships the answer to the client unless the answer is left out —
   at which point the server needs the item anyway.
3. **Keep the item.** The server mints an opaque id, holds the item, and the client posts back
   `{itemId, answer, durationMs}`.

**Decision.** (3), in memory, with a 2-hour TTL and a per-user cap.

**Why not a table.** These records are worthless the moment the answer arrives. Persisting them
buys surviving a restart — which costs the learner one keystroke — in exchange for a migration,
a cleanup job, and a class of stale-row bugs. An expired id returns **410 Gone** and the client
fetches a new batch.

**Consequences.**

- The expected answer never appears in any response until an attempt has been made.
- A client cannot name the features it graded, cannot replay an item, and cannot answer another
  user's item.
- Pending items are lost on restart. Accepted.
- This works because the process is single. A multi-process deployment would need (2) or a
  shared store; nothing in the v1 shape is heading there ([ADR-001](#adr-001)).

---

## ADR-009 — A prerequisite is met at `interleaved`, and an un-drillable topic is `explained`

**Status:** accepted · 2026-08-22

**Context.** [R5](research/05-interleaving.md) specifies the gate as
`locked if prerequisites unmet` without saying what "unmet" means. Two questions fall out of
that, and both can wedge a learner permanently.

**Decision 1 — prerequisites are met at `interleaved`, not at `mastered`.**

Requiring mastery of every ancestor would serialise a two-year curriculum into a crawl: the
locative grid would wait on 50 interleaved answers about the accusative. R5's floor is about
whether the learner can absorb the load of the topic *in front of them*, and clearing the
competence floor upstream is what establishes that. Mastery is a maintenance state, not a gate.

**Decision 2 — a topic with no generator resolves to `explained`, a fifth phase.**

Some topics have a real rule card and no exercises behind them: the alphabet, politeness
registers, preverb detachment, and — on the shipped hu-v1 pack — every topic whose forms
Wiktionary lists without features. The alternatives were both bad:

- treat them as `locked` → the alphabet permanently walls off the entire curriculum;
- treat them as `mastered` → the app claims a mastery nobody demonstrated.

`explained` says what is true: this topic teaches by its explanation ([R4](research/04-explicit-instruction.md)),
we have not built exercises for it, and it does not hold up anything after it. It is rendered as
such in the topic map, not hidden behind a neutral colour.

**Consequences.** Availability has to be *proven* rather than assumed — a topic can pass every
structural test and still generate nothing — so `Generator.Availability` builds one item and
caches the verdict. That cost is paid once per topic per process.

---

## ADR-010 — The pack ships inside the binary; there is no download step

**Status:** accepted · 2026-08-22 · **supersedes** the "download a pack on first run" plan in
Phase 2 and the `packPresent: false` empty state it implied

**Context.** Every language feature — analysis, drills, the reader, the course — reads the
language pack. Until this ADR the pack was a separate 252 MB download, so a fresh install booted
into a working app with all of it dead behind *"no language pack is installed — download one
from Settings → Data"*. Two things made that worse than it sounds.

First, the message was reachable through no fault of the user: `lang.Detect` matched on the
*filename*, so a truncated or zero-byte download counted as an installed pack, and the app then
reported one as present while every query against it failed. That is the failure that was
actually observed.

Second, "download one from Settings" is a promise about a host that does not exist. This is
self-hosted software with no release infrastructure behind it; there was no URL to fetch from.

**Options considered.**

| Option | Verdict |
|---|---|
| Keep the download, add a real release host | Rejected — it makes first run depend on a service the maintainer has to keep alive forever, and on the user's network. The app stops being a single binary |
| Ship the 252 MB pack inside the binary | Rejected — a 290 MB binary and a 290 MB image layer, paid by everyone, to carry a long tail most learners never reach |
| **Embed a compact starter pack, keep the full pack as an offline upgrade** | **Accepted** |

**Decision.** `internal/lang` embeds a gzipped starter pack (5 132 lemmas, 502 480 wordforms,
12 000 sentences, the whole 24-unit course, the minimal pairs and the resource catalogue —
15.6 MB compressed) plus the 384 HVPT Opus clips. `EnsurePack` extracts them on first boot,
**proves the result opens** before renaming it into place, and does nothing on later boots.
`Detect` now verifies each candidate opens and walks back to an older pack if the newest is
broken. `pack_meta.edition` records which edition is installed.

The starter's contents are not a frequency band alone: the course's sentences are selected
first and *their* lemmas are pulled in, because a course word's only attested sentence often
contains a rarer word the band excluded. Without that, the pack would ship a syllabus it could
not teach — 781 of 853 course words now get a sentence-backed card against a full-pack ceiling
of 783.

**Consequences.**
- There is no first-run empty state left to design, and no download endpoint to secure. The
  binary is 37 MB.
- The full pack degrades *rarity*, not function: an unknown word already renders honestly, so
  the upgrade is a genuine option rather than a hidden requirement. It is built with
  `make langpack` — there is no in-app upgrade path, and the UI says so rather than offering a
  button the app cannot honour.
- The starter is a build artefact under `internal/lang/starter/`. It must be rebuilt
  (`make starter`) whenever the pack pipeline changes, or the binary ships last week's course.

---

## ADR-011 — No export; lock-in is answered by the data directory

**Status:** accepted · 2026-08-22 · **supersedes** the `.apkg` export commitment in
[ADR-003](#adr-003) and [R16 §4](research/16-debunked.md)

**Context.** v1.0 shipped `internal/export`: `.apkg`, CSV, a full backup, and a staged restore
that swapped the SQLite file at next boot because doing it live destroys a WAL database. The
stated reason was anti-lock-in — "the learner is never locked in".

**Decision.** The package and its four endpoints are removed. The Settings → Data panel and its
locale keys go with them.

**Why this is not a retreat from the promise.** An `.apkg` never carried what this app knows.
Anki's model is one card with one interval; the thing being exported is *two* FSRS state spaces,
a per-feature weakness map and a review log that replays bit-for-bit. What fits through the
`.apkg` bottleneck is the lemma side flattened to a single interval — the morphological half,
which is the whole thesis, cannot cross. Offering it invites the learner to believe they have
their progress when they have a vocabulary list.

What they do have is the data directory: `tanulom.db` is an ordinary SQLite file with a
documented schema, and the review log in it is the source of truth (undo is already
delete-and-replay over it). A backup is `tar` of the volume; a restore is putting it back with
the container stopped. That is a stronger guarantee than a lossy interchange format, and it is
true without any code.

**Consequences.**
- The docs, the README and the product now say the app has no export. Nothing claims one.
- The staged-restore machinery is gone, and with it the only code path that could swap a SQLite
  file under a live connection.
- Anki users cannot import their existing decks *out* of tanulom, and this is a real cost. If it
  is ever paid, the honest shape is a full JSON dump of both state spaces plus the review log —
  not an `.apkg`.
