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
4. ~~`nlp` sidecar → HuSpaCy disambiguates in context~~ — removed by [ADR-069](#adr-069); the ladder is exact → prior → suffix.

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
- **Amended 2026-09-03 — a probe verifies the model, not just the socket.** "Surfaced with
  status and latency" was read as `GET /health`, and for `llm` that reported «на связи» over an
  endpoint that refused every request: the config named no model, the settings screen had no
  field for one, and the first learner to plug in an Ollama got a tutor that only said «модель
  не ответила». The `llm` slot now requires a model name (`llm.ErrNoModel`, reported to the
  panel as `errors.llmNoModel`), the probe asks a reachable endpoint `GET /v1/models` and says
  «укажите модель» / «такой модели нет на сервере» with the list offered in Settings, and a
  `local` provider is sent `reasoning_effort: none` because a thinking model spends the
  answer's token budget on hidden reasoning first. Status for a service is what has been
  verified, and nothing more — the same rule ADR-012 set for device speech.

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

---

## ADR-012 — Hungarian speech comes from the learner's device by default

**Status:** accepted · 2026-08-23 · **refines** [ADR-005](#adr-005)

**Context.** ADR-005 made speech pluggable with "local is the documented default", and
`services/tts/` in this repository is a README. The consequence, in the shipped product, was
that a fresh install made no sound at all. `Services.TTS.Provider` defaulted to `off`, so
`srs.PlanFor` dropped the `listen` and `spell` directions before a single card was created; the
reader had no 🔊 anywhere despite docs/UX.md §5 drawing one; the listening screen refused to
create a material without an uploaded audio file; and Settings showed four rows reading
"Выключено" with no way to change that short of deploying a service that does not exist yet.

A learner's first impression of the app was therefore an app with the sound switched off, and
the only documented remedy was to go and run a Python sidecar.

**Decision.** Add a fourth provider, `device`, valid for `tts` only, and make it the default.
It means the Web Speech API in the learner's browser. It has no endpoint. Every desktop OS
ships at least one Hungarian voice, and the API is available in every browser we target.

**The honesty problem, and how it is resolved.** The server cannot verify device speech — it is
not running in that browser and cannot know which voices are installed. So the two halves of
the answer are given by the two parties that can actually give them:

- `/api/capabilities` reports `tts` as `configured: true, reachable: false, detail: "device"`.
  It states the configuration, which it knows, and declines the reachability claim, which it
  does not.
- The client asks `speechSynthesis`, and the Settings row names the voice it found — or says
  the device has none, with the OS-specific instructions for installing one. Every 🔊 button
  renders either way and carries its reason when disabled, per ADR-005's own rule that a
  feature needing an unavailable service is disabled *visibly*.

**Consequences.**
- Audio exists throughout the app with nothing deployed: the reader (word and sentence, `s`),
  the review card (`listen`/`spell` autoplay, replay everywhere else), the drill's correct
  answer, and the listening screen reading a transcript aloud sentence by sentence.
- `POST /api/listen/material` no longer requires an audio file. A transcript alone is a
  material. The UI labels which materials are recordings and which are synthesised, and states
  plainly that a synthesiser is worse input than a speaker — flatter intonation, and vowel
  length, which is phonemic in Hungarian, comes through less clearly.
- `normalize()` fills zero fields only, so an existing `config.yaml` that says `off` stays off.
  That is the documented rule and it is not worked around; Settings is where it changes. The
  client-side 🔊 controls do not consult the setting at all, because no server is involved in
  them.
- A real TTS sidecar remains supported and remains better. `device` is the floor, not the
  ceiling.

---

## ADR-013 — Curriculum order is advice; only two things hard-lock

**Status:** accepted · 2026-08-25 · **supersedes** Decision 1 of [ADR-009](#adr-009)

**Context.** ADR-009 worried about exactly this failure mode — "both can wedge a learner
permanently" — and answered it by meeting a prerequisite at `interleaved` rather than at
`mastered`. That was not enough. A real learner arrived here:

```
vowel-harmony   blocked   accuracy 0.678 over 27 answers   (gate: 0.80)
```

`vowel-harmony` is the SECOND topic in the course, and the app's own lesson for it says the rule
"takes twenty minutes to understand and about three months to automate: it is a motor skill".
Every A1 topic sits behind it. So the learner could read every lesson and practise nothing —
not the plural, not the accusative, not the locative grid — until a three-month motor skill
crossed 80% rolling accuracy. They reported it as "I went through the grammar once, why is
everything after it blocked", which is the correct reading.

The gate was also self-undermining: a dip on one topic re-locked every topic downstream, taking
away access the learner had already earned and work already in progress.

**Decision.** An unmet prerequisite becomes **advice**. `drill.Decision` carries `Advice` and
`AdviceArgs` naming the topic worth finishing first; the phase is computed from the topic's own
counters and the topic is practised regardless.

Two gates still lock, and neither is an ordering preference:

- **A failed checkpoint** shuts the levels above it ([R14](research/14-motivation.md)). That is
  a statement about a whole level's evidence, and finishing the topic below would not open it.
- **The listening volume** word order needs ([R15](research/15-hungarian-difficulty.md)). "You
  cannot reach this by rules" is a claim about the material, not about sequence.

**This is what the course plan already said.** docs/API.md has always documented `ahead` as not
`locked`: *"Nothing stops a learner jumping to month five, and calling it locked would be a lie
about a restriction the app does not enforce. The order is advice."* The grammar DAG was
enforcing the opposite. One of the two was wrong, and it was the one that wedged people.

**What is NOT changed.** The blocked → interleaved gate inside a topic stands, hysteresis
included — that is [R5](research/05-interleaving.md) and CLAUDE.md's invariant, and it decides
how one topic's own items are MIXED. With this change it can no longer take anything away: a
fallback means "practise this one contrast at a time again", not "your curriculum shrank".

**Consequences.**
- Measured on the reporting learner's deck: locked topics went from 24 of 28 to **1 of 28**, and
  that one is the listening gate.
- The demotion COUNTER is gone from the topic map. A tally of past fallbacks names no action and
  reads as a scoreboard of failures; the number is still recorded in `topic_progress.fallbacks`.
- `gate.locked.prereq` became `gate.advice.prereq` in both locales, and reads as a suggestion.
- A learner can now walk into a topic they are not ready for. That is the point: the app says so
  in the badge and lets them decide, which is the same respect the plan screen already showed.

---

## ADR-014 — Sentence translation is a labelled crib from a dedicated MT model

**Status:** accepted · 2026-08-26 *(user decision)* · refines
[ADR-005](#adr-005) · see [R10](research/10-llm-tutors.md), [R11](research/11-extensive-reading.md)

**Context.** The reader answers "what is this word" thoroughly and "what does this sentence say"
not at all. In practice the learner answered it themselves by copying the line into a chat
assistant in another tab. That costs the reading loop — the thing [R11] says the whole screen
exists to protect — and hands their reading material to a third party.

The obvious home for it was the existing `llm` slot. That turns out to be the wrong tool.
Hungarian is low-resource for general models, and a 7–8B chat model translating hu→ru is
materially worse than **NLLB-200-distilled-600M**, a dedicated translation model that fits in
~600 MB of int8 weights and answers in a fraction of a second on a CPU. The good general answer
needs either a big local model or a cloud key; the good *translation* answer needs neither.

**Decision.** A fifth service slot, `services.mt`, served by `services/mt` — NLLB-200 under
CTranslate2, speaking its own `POST /translate` rather than the OpenAI chat API. `provider:
openai` is **rejected** for `mt`, because accepting it would advertise a protocol we do not
speak. Off by default; the reader is complete without it.

**Why this does not breach the LLM guardrail.** CLAUDE.md's rule is that no LLM-generated
**Hungarian** becomes a card without passing `internal/corpus/attest.go`. This generates Russian
and English, and it generates nothing that is stored as content: the failure mode here is not "a
sentence no Hungarian speaker has written" but "a translation that is quietly wrong". Attestation
cannot catch that, so the defence is **labelling**, and labelling is therefore not optional:

- Every row returned by `POST /api/translate` carries the `model` that produced it, and the UI
  renders the model beside the text on every view, not in a tooltip.
- A translation never becomes a note's gloss, a card's answer, or anything graded **on its own**.
  It can be KEPT — `user_gloss`, migration 00009, the learner's own dictionary for words the pack
  lacks — where it is stored with `source='machine'` and the model, and re-rendered with the
  label on every view. It reaches a CARD only after the learner presses «Принять как своё»,
  which re-saves it as `manual`. A card's back is what the `produce` direction grades you
  against, so the line on it has to be one a person read and accepted rather than one the app
  laundered on their behalf; the rail says that outright instead of silently withholding the
  card. *(Amended 2026-08-26, when the reader gained the dictionary.)*
- The cache (`translation_cache`, migration 00008) stores the model with the text, so the second
  view can be labelled as honestly as the first.

**Consequences.**
- The reader gets a **«Перевести предложение» button in the word rail**, beside the word's own
  translation, and it is **not drawn** when the capability is unreachable. It was a `t` keystroke
  at the bottom of the key strip and went a release without being found, so the key was removed
  and the control made visible; the result renders under the button that asked for it rather
  than below the article, where on a narrow window it was off screen entirely.
  *(Amended 2026-08-26.)*
- A fully cached request is answered with the sidecar stopped. The learner already paid for those
  sentences and nothing in serving them needs the model.
- **Licence:** NLLB-200's weights are CC-BY-NC-4.0. Running it for your own learning is squarely
  what that permits, and it is why the weights are not baked into the published image and why the
  profile is opt-in. Everything tanulom itself ships stays redistributable.
- **Cost:** a fifth capability dot, a Python sidecar we now maintain, and a ~2.4 GB first build.
- A batch is all-or-nothing. A short response from the sidecar is an error, not a partial result:
  silently dropping element three would pair sentence four's translation with sentence three.

---

## ADR-015 — The translation model is downloadable from Settings

**Status:** accepted · 2026-08-26 · **refines** [ADR-014](#adr-014), **does not reverse**
[ADR-010](#adr-010)

**Context.** [ADR-014](#adr-014) put sentence translation behind an opt-in sidecar. That was the
right call and it left a hole: **nothing about the sidecar was visible or steerable from the
app.** `/api/capabilities` probes every optional service the same way — `GET /health`, anything
under 500 is reachable — and for four of the five that is correct. For `mt` it is not, because
this service has a long and entirely legitimate not-ready state: on first boot it downloads and
converts 2.4 GB of weights and answers 503 for the several minutes that takes.

Flattened through the probe, three different situations reached Settings as the same grey
«нет связи»:

| What is actually happening | What the learner saw | What they should do |
|---|---|---|
| the container was never deployed | нет связи | bring it up, once |
| the model is downloading | нет связи | wait |
| the download failed on a full disk | нет связи | free space, retry |

The only way to tell them apart was to read container logs over SSH — on a self-hosted app whose
whole point is that you run it yourself and mostly forget about it. And a failed download had no
remedy short of restarting the container, because the loader ran once, at process start.

**Is this the download button ADR-010 refused?** No, and the difference is the reason that ADR
gave. ADR-010 rejected "download the pack from Settings" because *"'download one from Settings'
is a promise about a host that does not exist"* — there was no URL, no release infrastructure,
nobody operating a server to fetch from. For NLLB-200 that host **does** exist, is HuggingFace,
and is not one we operate or promise anything about. Meanwhile [ADR-014](#adr-014) forbids the
other distribution mode: the weights are CC-BY-NC-4.0 and must not ship in a published image.
Download-on-demand is the only mode that satisfies both constraints, and the button is honest
because the thing behind it is real.

**Decision.**

1. **The sidecar reports what it is doing.** `GET /health` keeps its 503-while-loading — the
   probe must not call a service usable before it can serve — but its BODY now carries
   `status` (loading / ready / failed), the failure `detail`, and `downloadedBytes` /
   `totalBytes`.
2. **`POST /warmup`** starts or retries the load, guarded so two calls cannot both convert. This
   is what makes a failed download recoverable without a container restart.
3. **`GET /api/mt/status`** (any authenticated user) passes that body through, and
   **`POST /api/mt/warmup`** (admin, matching `PUT /api/settings`) proxies the retry. `mt` gets
   its own routes rather than a special case inside the capability probe, which stays uniform.
4. **Settings draws four states**, each with the remedy that fits it, and the generic services
   row **defers to the detailed status** so one screen cannot hold two answers.

**Progress is real or absent.** `totalBytes` comes from the HuggingFace file listing and
`downloadedBytes` from the size of the cache's `blobs/` directory — real files only, because the
`snapshots/` tree is symlinks to the same blobs and `stat()` follows them, which reported
**9.9 GB downloaded of a 2.5 GB model** the first time it ran. Where either figure is
unavailable the fields are omitted and the UI draws an indeterminate bar with "about 2.4 GB,
once". An invented percentage is exactly the confident-wrong-number this project refuses
everywhere else.

**What this deliberately does NOT do: start the container.** The core is a distroless static
binary with no shell and no Docker socket. Mounting the socket to gain that power would hand a
web application root on the host, which is not a trade worth a button. So the boundary is stated
rather than hidden: **you bring the container up once — `docker compose --profile mt up -d`, or
`COMPOSE_PROFILES=mt` in `.env` so plain `up -d` includes it — and everything after that is in
the UI.** The "not answering" state prints that command instead of a shrug.

**Consequences.**
- Turning the translator off is a `<Select>` in Settings, and every translate control disappears
  app-wide because they all read one capability.
- The weights are converted at BUILD time and baked into the image, so the container starts
  ready and an image rebuild reuses the layer cache rather than re-fetching 2.4 GB. There is
  **no volume on `/models`**: one was carried here from an earlier design that downloaded at
  runtime, and against a build-time conversion it only duplicated 635 MB and shadowed the image
  — weights served from the volume, `model` on every response read from the new image's
  `MT_MODEL`, so a rebuilt model would have labelled its output with a name that did not produce
  it. Labelling is the whole defence for machine translation ([ADR-014](#adr-014)).
- `make mt` runs the service natively, venv and all, for people not using Docker.
- **Cost:** two more endpoints, a polling loop that runs only while loading, and a status shape
  that four screens' worth of copy has to stay honest about.

---

## ADR-016 — The review queue is separable by where its material came from

**Status:** accepted · 2026-08-27 *(user decision)*

**Context.** A learner's deck has two origins that already differ in the database:
`note.source` is `course:<unit-code>` for words enrolled from a course unit and `manual` for
words added from a text in the reader. Downstream, nothing read it. One queue served both, so
"finish today's course work" was impossible without also clearing every word ever clicked while
reading — on a real deck, 79 course cards behind 88 reader ones.

The two piles answer different questions. The course deck is a syllabus obligation the plan
screen reports on and the checkpoints count. The reader deck is the learner's own mining, chosen
for reasons the app does not know. Merging them is not neutral: it makes the syllabus
unfinishable by anyone who also reads.

**Decision.** `GET /api/review/queue?scope=all|course|reader`, defaulting to `all`. A segmented
control «Всё · План · Тексты» on the review screen. No schema change — `note.source` already
discriminates.

Three constraints keep this a **filter over one scheduler**, not a second scheduler:

- **One FSRS state space.** Scope selects rows; it never touches scheduling.
- **One daily new-card budget, shared.** `newPerDay` and `introducedToday` stay global while
  `due`/`new` become scope-relative. A per-scope cap was **rejected**: it would let a learner
  take 20 course cards *and* 20 reader cards a day while the setting said 20 — arriving at the
  review avalanche the cap exists to prevent, by using the feature exactly as intended. Pinned
  by `srs.TestTheDailyNewCapIsSharedAcrossScopes`. The visible consequence is stated in copy
  (`review.scopeHint`): after the day's new cards are spent in one pile, the other shows its
  backlog and «Сегодня введено: 20 из 20».
- **No stored preference.** The scope is a property of one request, echoed back in the response
  so the client never assumes its request won. It is not persisted, and it is not on
  `QueueItem` — a per-item copy of a per-request fact is how DTOs rot.

**`reader` is the COMPLEMENT of `course`, not `source = 'manual'`.** A third source added later —
an LLM tutor, an import — would otherwise be invisible under every tab while still being served
under `all`, and `all == course + reader` would stop holding with nothing failing.
`srs.TestAnUnfamiliarSourceCountsAsReader`.

**Each tab carries TWO numbers, due and new.** The reader pile on the deck that prompted this
sat at 0 due and 88 new. A tab reading «Тексты 0» over 88 waiting words is the screen lying, and
a single merged figure would either hide the backlog or invent one.

**Consequences.**
- `internal/srs/scope.go` is new; `Store.Queue` takes a `Scope`. An unknown value is a 400 rather
  than a silent widening to the whole deck — a typo that returns everything looks exactly like
  the feature working.
- Every scope is counted on every request, because the control must label the tab the learner is
  not standing on.
- **Unlegislated by the research notes.** No note covers queue partitioning; [R5] is about drill
  topics, not the SRS queue. This is a product decision about learner autonomy, recorded here
  rather than dressed up as a finding.

---

## ADR-017 — `produce` grades the dictionary form; the sentence is held by the note

**Status:** accepted · 2026-08-27 *(user decision)* · see
[R12](research/12-retrieval-direction.md), [R15](research/15-hungarian-difficulty.md)

**Context.** A `produce` card prompts with the target word's gloss and grades the typed answer
against `note.target_form` — the surface token that happened to occur in the corpus sentence the
note was built from. So a card printed «день; солнце», the dictionary entry for `nap`, and
demanded `napokra`: the sublative plural. Another printed «имя» and demanded `nevem`. Nothing on
either card named a case or a possessor, and the placeholder promised «одно слово по-венгерски»
while the server wanted a case form. On the deck that prompted this, 18 of 39 course notes and 14
of 37 reader notes carried an inflected target. The learner could only fail, and could not tell
why.

**Decision.** `produce` grades the **dictionary form**, resolved from `note.lemma_id` through the
pack. `cloze` and `spell` keep `note.target_form`.

This **restores** the documented contract rather than departing from it. [R12]'s direction table
already defines `produce` as *"RU/EN → HU (typed) — form recall"* and `cloze` as *"sentence with
target form blanked — contextual production + morphology"*. The cue is the lemma's gloss;
therefore the response is the lemma. The inflected form in context is the question `cloze`
exists to ask, and it is the only direction that grades morphology.

**The R15 exposure, argued rather than ignored.** [R15] says *"every card must be a sentence
carrying a real suffix, from week one"*. That guarantee is held at **note level, not card level**:
the note keeps its sentence, and its `cloze` card carries the suffix. This is only honest
because of the second half of this ADR — before it, a note whose target had no analysable
features got *no* cloze card, so the note genuinely carried no suffix anywhere. **The two halves
must ship together.**

**A cloze needs a SPAN, not features.** Features are what it can *additionally* grade. Requiring
them denied the sentence exercise to 23 of 37 words mined from a text — the exact words the
learner chose to keep — and to 21 course notes besides. A featureless cloze grades the card's own
FSRS state and emits **nothing** into the morphological state space
(`srs.RequiredFeatures` returns nil for empty feats, pinned by
`srs.TestNoTargetFeaturesGradesNothing` and `api.TestAFeaturelessClozeGradesNoFeatures`), so the
two state spaces stay exactly as separate as before.

**Consequences.**
- `expectedAnswer` in `handlers_review.go` is the one place that decides what a typed answer is
  graded against. It falls back to `target_form` whenever the lemma cannot be resolved — no
  `lemma_id`, no pack, a lemma the pack lacks. A card that grades the inflected form is wrong; a
  card that grades nothing is broken.
- **Existing notes are repaired, not migrated.** `Store.BackfillClozeCards` mints the missing
  cloze card on the review screen — 44 notes on the deck that prompted this. A SQL migration was
  **rejected**: migrations run inside `db.Open`, before any config is read, so one would mint
  cloze cards for a learner who turned the direction off.
- A backfilled cloze enters as `state = 0`, so it is subject to the daily cap and to sibling
  burying. The 44 surface over several days rather than at once. That is correct, and is not the
  backfill failing.
- **Known consequence, accepted:** two notes sharing a lemma now produce two `produce` cards with
  the same prompt and the same answer, and sibling burial is keyed on `note_id`. On the deck that
  prompted this, 4 lemmas of 72 carry two notes. The second card is a legitimate second exposure
  at a 5% collision rate; if reading pushes that up, burying gains a `(lemma_id, produce)` key.
- **A note with no meaning at all had an empty produce prompt.** Note creation now backfills
  `gloss_ru`/`gloss_en` from the pack's `lemma_gloss`, per field and never over a meaning the
  learner wrote, and the review queue fills any that are still empty. This is pack data compiled
  by people, not machine translation, so [ADR-014](#adr-014)'s labelling rule does not reach it.

---

## ADR-018 — A card may carry a machine crib, beside the Hungarian and labelled

**Status:** accepted · 2026-08-27 *(user decision)* · **amends**
[ADR-014](#adr-014)

**Context.** [ADR-014](#adr-014) says a translation *"never becomes a note's gloss, a card's
answer, or anything graded **on its own**"*, and CLAUDE.md quotes the first half of that
sentence without the qualifier. A word mined from the reader has `sentence_id = NULL` — its
sentence came from the learner's own text, and the pack has no human translation for it. So its
`cloze` card showed a Hungarian sentence with a blank and nothing saying what the sentence meant,
to a learner who had met it once at a third coverage.

**Decision.** A cloze card may show the sentence's machine translation as **prompt context**,
carrying the model's name beside the text.

The line ADR-014 draws is between the **back** of a card and **context beside the front**:

- **Forbidden, unchanged.** A machine translation as a note's gloss, as a card's answer, or as
  anything graded. The `produce` direction grades you against a card's back, so that line has to
  be one a person read and accepted — «Принять как своё», re-saved as `manual`.
- **Permitted, labelled.** A crib printed beside the Hungarian on a cloze, where the graded
  string is the **Hungarian target form** and the crib is what makes choosing it possible. It is
  read, never answered.

**The label is not optional and not a tooltip.** `sentenceModel` travels with every queue item
and is **empty for a pack translation** — those were written by people and crediting a model
would libel the translator and make the label meaningless where it matters.
`api.TestAPackTranslationCarriesNoModel`.

**Consequences.**
- **The sidecar is never on the review path.** `attachCribs` reads `translation_cache` and
  nothing else; the queue is byte-identical whether MT is running, off, or mid-download.
  `api.TestTheQueueNeverCallsTheTranslator`. Filling the cache is a bounded background top-up,
  one flight per learner, drained on shutdown.
- With no crib the card is Hungarian plus its gloss and reports no failure — the same behaviour
  as a cache miss, so there is one thing to reason about rather than two.
- **The crib shows at question time on `cloze`,** not on reveal. That direction asks *which
  form*, and a learner cannot choose a form for a sentence they cannot read. The blank is
  Hungarian and the crib is not, so nothing is handed over; the required features stay
  deliberately unnamed, as [ADR-014](#adr-014) and `docs/UX.md` require.
- The crib's language follows `locale.ui`, never `locale.gloss` — the latter is allowed to be
  `both`, which is not a translation target.

---

## ADR-019 — Free practice: retrieval that deliberately schedules nothing

**Status:** superseded by [ADR-064](#adr-064) (practice answers are reviews) · accepted · 2026-08-27 *(user decision)* · see
[R1](research/01-retrieval-practice.md), [R14](research/14-motivation.md),
[ADR-003](#adr-003)

**Context.** The review queue answers one question — *what do I owe today* — and when it is
empty, or when the daily new-card cap has been spent, the app has nothing for a learner who
simply wants to train. That is a real want and a defensible one: [R1] is unambiguous that extra
retrieval helps. A learner asked for "a trainer that is always open, with words from the plan or
from my texts, just to practise."

The obvious implementation is the wrong one. Feeding those answers to FSRS tells the scheduler
the learner recalled something at an interval they were never actually tested at, and the model
has no way to distinguish that from a real review — this is the cramming problem, and it
degrades the one thing the app is built around. Discounting the update instead ("count practice
as half a review") means inventing scheduling maths of our own, which [ADR-003](#adr-003)
forbids outright.

**Decision.** Free practice is a **separate mode**, reachable at any time, that draws at random
from a chosen pile and **schedules nothing**.

A practice answer:

- changes **no card state** — no due date, no stability, no difficulty, no reps;
- writes **nothing to `morph_state`** — that is an FSRS state space too, so writing to it *is*
  scheduling, and it is how a feature would get closer to mastered without being reviewed;
- writes **nothing to `review_log`**.

That last one is not a style choice. Undo is delete-and-replay over `review_log`, and replay
must reproduce stored FSRS state bit for bit (`internal/srs/replay.go`). A row that never fed
the scheduler cannot live in that log without breaking the property the whole undo mechanism
rests on. Practice gets its own table, `practice_log` (migration `00010`).

**What it does record**, because a mode that leaves no trace cannot be reviewed by the learner
either: volume **and accuracy**, reported on the Progress screen as its own line, explicitly
labelled as not affecting the schedule. Volume alone was rejected — a count that only ever goes
up is the shape of a point economy, which is exactly what [R14] and
[R16 §3](research/16-debunked.md) say does nothing for competence. Accuracy is the half with
something measured behind it.

**Minutes count; the streak does not move.** Practice is real work, so it accumulates into
`daily_log.active_ms` the way listening does. It deliberately does **not** write `due_left`:
the streak needs the real queue cleared *as well as* the minutes (`stats.Day.Met`), so however
long a practice session runs it can never stand in for the day's obligation. `RecordPractice`
therefore touches `active_ms` only — writing `due_left` would let a practice session refresh a
stale zero and hand out a streak nobody earned.

**Selection is random and unweighted.** Weighting toward lapsed or weak words would be more
efficient per minute, and was rejected: this is the mode a learner opens to train rather than to
be managed, and a hidden agenda about which words come up makes it something else. Siblings are
buried *within* a round for the usual reason, but never across the day — practice is always
available, and burying it against review answers would be the app rationing the one thing it
does not schedule.

**Consequences.**
- `GET /api/practice/queue?scope=` and `POST /api/practice/answer`, on their own routes so the
  distinction cannot blur. `PracticeAnswerResponse` carries no `nextDue` and no `gradedFeatures`
  rather than zeroed ones — a `0` interval renders as a schedule that does not exist.
- **The verdict is shared, not duplicated.** Both modes route through `gradeTyped` and
  `expectedAnswer`: they differ in what they RECORD, never in what counts as correct. A second
  copy would drift, and the first symptom would be a word the trainer marks right and the queue
  marks wrong.
- **The screen says so.** A standing notice while practising states that answers do not move the
  schedule and bring no word closer to «знаю». The honesty rules require it: this is the one
  screen where an identical-looking card means two different things.
- Practice draws from cards in **any** state, new ones included. That does not bypass the daily
  new-card governor, because nothing is introduced: the card stays `new` and will still be
  scheduled properly later. The governor exists to bound future review load, and practice
  creates none.
- **Cost:** a second answer path to keep in step with the first, and a mode a learner could use
  to avoid the queue. The second is accepted deliberately — the queue is an obligation, not a
  cage, and [R14] is clear that autonomy is what keeps people coming back.

## ADR-020 — Stored FSRS state is complete: `remaining_steps` survives the database

**Status:** accepted · 2026-08-28 · see [ADR-003](#adr-003)

**Context.** go-fsrs v4's `Card` carries `RemainingSteps` — the position inside the
learning/relearning steps (`{1, 10}` min and `{10}` min by default). Our `CardState` and its
`toFSRS`/`fromFSRS` mappings dropped it, and no table had a column for it. Every card, feature
and HVPT contrast therefore re-entered the scheduler with 0 remaining steps, and in
`learningState` that hits the `remaining <= 0 → graduate` branch: **«Снова» or «Трудно» on a
card still in Learning or Relearning graduated it a day or more out instead of repeating the
minutes-scale step.** Only Good→Good coincided with the correct schedule, which is why the
replay property test never fired — replay folded through the same lossy mapping and agreed
with the wrong answer.

**Decision.** Persist the field (migration `00011`: `card`, `morph_state`,
`hvpt_contrast_state`; `lemma_state` skipped — its FSRS block is never advanced), map it in
`toFSRS`/`fromFSRS`, and repair stored state with a **one-time, marker-gated replay** of every
log (`srs.ReconcileSteps`, `hvpt.ReconcileSteps`) at boot in `api.New` — not in the migration,
because replay needs `desiredRetention` and the gate window, and migrations run inside
`db.Open` before any config exists. Markers live in the new `app_meta` table (facts about the
database file, unlike per-user `settings`). On failure the marker is not set, the server boots
anyway, and the next start retries; replay is deterministic (fuzz is seeded from stored
millisecond timestamps, and `RemainingSteps` does not enter the seed), so a re-run is a no-op.

**Consequences.**
- The replay invariant reads: stored state *including* `remaining_steps` reproduces bit for
  bit. `replay_test.go` compares the field; `scheduler_steps_test.go` pins the storage
  round-trip on the exact reported paths.
- Reconciliation replays under the CURRENT desiredRetention/gate window — the same assumption
  `UndoLast` has always made.
- Visible effect of the repair: prematurely-graduated Learning cards replay back to Learning
  with a past due date, so the queue can grow once, correctly.
- **Amended 2026-09-08 — "deterministic" means on one CPU architecture.** go-fsrs draws its
  fuzz from a floating-point generator and Go fuses multiply-adds on arm64 but not on amd64, so
  the same log replays to a different fuzzed due date on each, with stability and difficulty a
  bit apart. The marker keeps this repair from running again after a `data/` move between
  architectures; an undo on the new host lands inside the fuzz range of the stored date.
  `TestReconcileRepairsLossyHistory` had pinned a fuzzed day — its lossy chain and the truth
  both graduated on a final Good and differed only by that coin — and failed on CI's amd64 while
  passing on the arm64 laptop. Its chain now ends on a lapse, a difference in state and lapse
  count that no fuzz can hide. ARCHITECTURE.md §5.7.

## ADR-021 — Review numbers speak one vocabulary: the Breakdown

**Status:** accepted · 2026-08-28 · **amends** [ADR-016](#adr-016)'s consequences

**Context.** Three surfaces described "the queue" in three coordinate systems. The tab strip
counted raw due CARDS before burial; Today counted an unscoped, unburied `DueCount`; the
session served one card per word. A learner with 26 due cards over 10 words was shown «26»,
served 10, told «на сегодня всё» — and a refresh then produced five more, because FSRS-6
learning steps (1/6/10 minutes) had matured and nothing on the wire could say so. Every number
was true in its own units and the learner was right to trust none of them. Separately:
`maxReviewsPerDay` was an SQL LIMIT per fetch, so a reload handed out a fresh batch; the
done-screen's `newCapped` mixed a scoped count with the global budget; and `due_left` — the
streak's "queue cleared" obligation — counted the very siblings burial had made unreachable,
making the streak **unwinnable** for any learner whose words carry more than one due card.

**Decision.** One computation, `srs.Breakdowns`, produces every learner-facing number, and
everything in it is **post-burial and day-granular**:

- `dueNow` — what a queue built now would serve: one card per word, graduated (state 2) cards
  counted for the whole local day, learning steps only once elapsed. This is «к повторению»
  everywhere: the tab strip, Today, and `daily_log.due_left` (via `ActionableDue`).
- `dueLaterToday` + `nextDueAt` — learning steps maturing before local midnight: the cards
  behind "refresh gave 5 more", now announced with a time instead of denied.
- `buriedToday` — due cards hidden until tomorrow (siblings of answered words, and extra
  directions of a word the session serves once). Counted where a screen can explain them,
  never inside `dueNow`.
- `all == course + reader` holds field by field; a text scope is a sub-slice of reader,
  appended after the fixed three, and the global new budget is untouched (ADR-016).

Serving a graduated card hours before its timestamp is queue POLICY, not scheduling maths:
go-fsrs computes elapsed time from `last_review` at UTC-calendar-day granularity, so the
library is indifferent (ADR-003 stands). Learning steps keep instant precision — the minutes
are the mechanism.

Also decided here: `maxReviewsPerDay` budgets **the day** (today's state-2 answers are
subtracted; steps in flight are never trimmed); the done-screen's new-material figures are
`newShownNotes / newDeferredNotes / newBacklogNotes` on one basis, so a sibling deferral can
no longer masquerade as "the daily limit is reached"; and **the streak rule**: `due_left`
stores `ActionableDue`, and `dueLaterToday` deliberately does not block the day — a 10-minute
step at 23:55 is not a debt.

**Consequences.**
- `srs.ScopeCounts` and `srs.DueCount` are deleted; `QueueCounts` drops the lying fields
  (`learning`, `newCapped`, `newNotes`, `newNotesCapped`). `queue_day_test.go` pins the served
  session to `dueNow`, the buried accounting, the day cap and the deferral/budget split;
  `TestClearingTheServedQueueMeetsTheDueObligation` pins the streak repair.
- Timezone matters now that "today" is load-bearing: the binary embeds `time/tzdata` and
  honours `TZ` (UTC otherwise) — env-only, documented in SELFHOSTING.
- Cost: `«к повторению»` is stable across a day rather than creeping upward as graduation
  hours pass, which means a card can be served hours "early" by the old clock. That is the
  Anki model, and it is the one a learner can predict.

## ADR-022 — Per-text review is a scope, and a note remembers its texts

**Status:** accepted · 2026-08-28 · **extends** [ADR-016](#adr-016)

**Context.** "Let me review the words from THIS article" was unanswerable: a reader-mined note
recorded no text id — `note.sentence_id` is a PACK sentence id, NULL for reader notes — and the
only trace was the verbatim `hu` string.

**Decision.** A link table, `note_origin(note_id, text_id, added_at)` (migration `00012`), and
a `text:<id>` scope. The client sends `textId` with `POST /api/notes`; the link is written on
the CREATE branch **and on the DUPLICATE branch** — the same sentence re-added from a second
text creates no new note (FindDuplicate keys on `(hu, target_form)`), and with a note *column*
instead of a link table, text B's claim on the shared word would silently vanish. Deleting a
text unlinks and never deletes notes.

`ParseScope` accepts `text:<n>` with a strictly-parsed, canonicalised id, which is what lets
`Scope.filter()` keep its documented no-bound-arguments contract by inlining the int64
(pinned by `TestParseScopeRejectsMalformedTextIds`). The text scope is a **sub-slice of the
reader pile, never a fourth one**: `all == course + reader` keeps holding, the UI draws it as
a chip above the three tabs rather than beside them, and the daily new-card budget stays global
(pinned by `TestTheDailyNewCapIsSharedWithTextScope`).

**Backfill is best-effort and says so.** Old notes are linked at boot (marker
`note_origin_backfilled`) by reconstructing each text's sentences with the CLIENT's join rule —
space-joined tokens, whitespace stripped before `.,!?;:…` — which deliberately differs from the
server's own sentence assembly around quotes and dashes. Exact matches link; misses stay
unlinked and simply do not appear under the text's pile.

## ADR-023 — Text level: heuristic first, LLM as labelled second opinion, manual always wins

**Status:** accepted · 2026-08-28 · labelling rule from [ADR-014](#adr-014)

**Context.** A growing library needs levels to sort by, and the learner should not have to
guess whether an article is A2 or B2. The pack already knows enough for an honest estimate:
`lemma.cefr` (frequency bands, `langpack/pipeline/frequency.py` — the ONE band table;
`drill/generate.go`'s `bandFor` is a different dial for drill vocabulary and is deliberately
not consulted), `grammar_topic.level` + `feature_ids`, and `text_token` already stores each
token's lemma and features.

**Decision.** Three writers, one pecking order, the source ALWAYS stored and shown
(`text.level`, `level_source`, `level_detail`, migration `00013`):

- **heuristic** — `mining.EstimateLevel`, computed at ingest and backfilled once on first open:
  lexical level = smallest band covering ≥90% of resolved running words; grammar level =
  highest topic level with ≥3 feature hits; verdict = the harder of the two, nudged one step by
  mean sentence length; **abstains** (`level=""`) when under half the text resolves — «уровень
  не определён» is a supported outcome, per the honesty rules.
- **llm** — `POST /api/texts/:id/level`: the first consumer of the `services.llm` slot
  (`internal/llm`, minimal OpenAI-compatible chat; prompt versioned in
  `.claude/prompts/estimate-level.md`). Strict-enum JSON verdict; garbage → 502 and the stored
  estimate is untouched; the model's name travels in `level_detail` because the label is the
  entire defence. No Hungarian is generated, so corpus attestation is not implicated.
- **manual** — `PUT /api/texts/:id/level`: the learner's word. Never overwritten by either
  machine writer.

**Consequences.** The library sorts and filters by level, each chip captioned with its source
(«по частотам» / «оценка модели» / «вы указали»). Unlike coverage, a level is a fact about the
text, not the learner — estimated once, not recomputed per open.

## ADR-024 — Practice rounds have a shape: size, and an explicit weak-words mode

**Status:** accepted · 2026-08-28 · **amends** [ADR-019](#adr-019)

**Context.** ADR-019 fixed practice as random and unweighted, rejecting selection bias as "a
hidden agenda about which words come up". The learner then asked for exactly two dials: round
length («рандом на 30 слов») and training what is about to fall out of memory.

**Decision.** `GET /api/practice/queue` gains `size` (clamped to 10–50, default 20 — constants
in `internal/config`, not YAML: round length is a whim, not a setting) and `mode=random|weak`.
**Weak mode does not contradict ADR-019's reasoning — it removes its trigger.** The rejected
thing was a HIDDEN agenda; weak mode is the learner's explicit choice with the criterion
printed on screen: lowest current retrievability first (`Scheduler.Retrievability`, the
library's own forgetting curve — no scheduling maths of ours, ADR-003), weakest card per word.
A deck with nothing reviewed has nothing to rank, so weak **falls back to random and the
response echoes the mode actually served** — the screen never claims a criterion it did not
apply. Random stays the default.

Everything ADR-019 forbids stays forbidden: no card state, no morph_state, no review_log, no
due_left. The ADR-019 pins run against `mode=weak` too.

## ADR-025 — The trash: a word can be set aside, and set-aside words are practised, never scheduled

**Status:** accepted · 2026-09-02 · **amends** [ADR-019](#adr-019)

**Context.** The queue had no way to hear "no". A card the learner judged unusable — a sentence
the miner cut badly, a gloss that says nothing, a construction three months above them — could
only be failed forever or left to sit at the front of every session. Both answers train the
learner to distrust the queue, and one of them poisons the second state space: repeatedly
failing a card the learner never had a chance at writes `Again` into `morph_state` for features
they were never really asked about.

Suspending was already expressible — `card.suspended` has been in the schema since migration
00002 — and nothing in the product ever set it. What was missing was not a column but a
decision about what setting a word aside MEANS.

**Decision.** A **note-level** trash, stored as `note.trashed_at` (migration 00014), with four
properties:

- **The word leaves, not the card.** The learner is setting aside a WORD; hiding only the
  direction on screen would hand the same word back through a different door ten minutes later.
  Deliberately not `card.suspended`: that stays a card-level mechanism, so a future per-direction
  suspend cannot make "restore" ambiguous, and a timestamp gives the trash screen an order.
- **Nothing is deleted.** The note, its cards, their FSRS state and every row in `review_log`
  survive. Restore therefore returns the card with the state it left with — a word set aside for
  a fortnight comes back OVERDUE, which is the truth about it. It also means undo
  (delete-and-replay over that log, ADR-006) keeps working across a trashing.
- **It leaves the SCHEDULER, not the app.** `scope=trash` is accepted by free practice and
  **refused by `/api/review/queue`**. Practice schedules nothing (ADR-019), so a set-aside word
  can still be looked at while nothing about it moves. A trash the scheduler drew from would be
  a pile cards come back FROM, which is the opposite of what the button promised. This is the
  one amendment to ADR-019: practice gains a pile that review does not have, and it is safe
  precisely because practice records nothing.
- **Re-adding restores.** `FindDuplicate` matches a trashed note, so without this the reader's
  «добавить» reported success on a word that stayed invisible forever, and a re-enrolled course
  unit could never restock itself. Pressing add is an unambiguous "I want this again".

`Scope.filter()` carries the not-trashed clause for all three normal scopes, so the exclusion
cannot be forgotten by a new query; the four places that build their own WHERE use the exported
`srs.NotTrashed`. `all == course + reader` still holds — all three exclude the trash, and the
tab strip's arithmetic is untouched. Trash and restore return fresh `Breakdown`s (ADR-021), so
the strip updates from the server rather than by subtracting one from a number it is holding.

**Consequences.** «В корзину» sits under the review card in both phases (`t`, dimmed while the
answer box has focus, since `t` is a letter), with an undo banner rather than a confirmation
dialog — the action is reversible, and a dialog taxes the common case to guard the rare one.
`/trash` lists the words with a restore per row and «тренировать отложенные», which opens a
practice round. Answering a card trashed since the queue was built is a 409, so a stale tab
cannot schedule it. The trash carries no due dates and no counts of debt: it must be a place a
word can be found, never a second deck the learner feels they owe something to.

**Rejected:** deleting the note (loses the log that undo and any future FSRS optimiser read, and
makes "restore" a re-add at state zero); resetting FSRS state on restore (invents a memory the
learner does not have); downgrading `lemma_state` on trash (setting a card aside is not
forgetting the word, and course-unit completion is derived from that table — ADR-007);
a fourth review tab (breaks `all == course + reader`, and puts an obligation-shaped number on
a pile that owes nothing).

## ADR-026 — Review is dealt in sets, and a card shows its own memory

**Status:** accepted · 2026-09-02 *(user decision)* · see
[R18](research/18-sessions-and-progress.md), [R14](research/14-motivation.md),
[ADR-003](#adr-003), [ADR-021](#adr-021), [ADR-024](#adr-024)

**Context.** The queue served the day's work as one list. A learner with 70 due cards saw
«1 / 70», and the honest observation was that it is exhausting: the finish line is out of
sight, and the only feedback along the way is the card verdict. The same learner asked to see,
while answering, how a word is doing — is this the first time, is it nearly known, does the
app think I will remember it. Two asks, one boundary: nothing may touch the scheduler
([ADR-003]), nothing may become a point economy ([R14], [R16 §3]), and every review number
must keep speaking the Breakdown vocabulary ([ADR-021]).

**Options considered.**

| Option | Verdict |
|---|---|
| Server-side `size` parameter, queue cut to N | Rejected — the whole queue is already fetched and bounded by the day cap; cutting it on the server adds API surface and a second place where "how much is left" is computed. Sets are presentation |
| Client-side sets of a per-device size | **Accepted** — the server serves exactly the queue it served before; the client deals it out. `Breakdowns`, `QueueCounts` and the streak are untouched |
| A stored `review.setSize` setting | Rejected — settings are admin-only and a set size is a whim, not policy; practice already made this call ([ADR-024]). Per device, `tanulom.review.setSize`, default 15 |
| Mix new cards into the sets | Rejected — reviews are a debt already incurred and stay first; new cards still follow. Reconsidered when a learner asks |
| Re-serve a failed card immediately | Rejected — a one-minute echo tests working memory. The card returns at the back of the set and only once the FSRS step it was given has **elapsed** |
| Retry within the set until *three* correct recalls | Rejected — [R18 §1]'s three-correct criterion is for the very first learning session; every later sitting is "one correct", and FSRS's steps already carry the first day |
| Random order by default | Rejected — the most overdue card is decaying fastest and belongs in set one. Random is an explicit, printed choice, day-seeded so a reload cannot reshuffle the remainder |

**Decision.**

- **Sets.** `stores/review.ts` deals the head of the served queue as a set of `setSize` cards.
  A set is finished when every card in it has been answered once *(amended 2026-09-03 —
  originally "answered correctly once", with misses re-served at the back of the set; see the
  amendment below)*. Nothing answered in a set is re-served in it; a miss returns through the
  server's queue once its step has elapsed, and the served list is fetched again at every set
  boundary so that it can. Between sets the screen shows a set summary and the next set starts
  on `space`; the auto-reload that tops the queue up when a learning step matures runs **only on
  the done screen**, never under a set in progress.
- **Order.** `GET /api/review/queue?order=due|random`. `due` is today's behaviour. `random`
  shuffles the due list after the day cap is applied (the cap trims from the latest-due end and
  needs due order), with a seed of `(userID, local day)`, then `spreadSentences` runs last as
  always. The selected new cards are shuffled the same way; which cards are selected does not
  change.
- **Memory on the card.** `srs.Memory` travels with every queue item: `stage` (new · learning ·
  young · mature · relearning — Anki's 21-day convention for young/mature, named as a
  convention), `retrievability` from `Scheduler.Retrievability`, `stabilityDays`,
  `intervalDays`, `reps`, `lapses`, `correct`/`total` for the card and for the word, the last
  eight outcomes, and `preview` — what each grade would schedule, from `Scheduler.Preview`,
  labelled approximate because interval fuzz is seeded at answer time. `AnswerResponse` carries
  the card's `memory` after the answer, so the widget can show «3 дн → 8 дн». Practice shows
  the same widget without `preview`: nothing there is scheduled. *(Amended 2026-09-03: the
  review card no longer prints `retrievability`; see below.)*
- **The set summary is measured only:** answered, right first time, came back, words that
  reached *mature* in this set, minutes. A two-note `setComplete` cue; the descending fifth
  stays reserved for the empty queue.
- **Today** prints the day in sets beside `dueNow`, from the same per-device size.

**Consequences.**
- `internal/srs/memory.go` and `queue.go` (`order`), pinned by `memory_test.go` and
  `queue_order_test.go`; `QueueItem` gains `memory`, `AnswerResponse` gains `memory`.
- The review screen gains a set bar, a memory widget, per-grade interval hints under the
  rating buttons, and a set-summary screen; `lib/sets.ts` holds the per-device size.
- **Cost:** one extra `GET /api/review/queue` per set boundary. A reload mid-set loses only the
  set summary's tallies; the cards themselves are wherever the server says they are.

**Amendment — 2026-09-03** *(user decision)*. Two parts of the first version were removed after a
day of use:

- **The in-set retry is gone.** A miss went to the back of the set and was re-served once its
  learning step had elapsed; when the set ran out first, the screen waited with a countdown. The
  learner's report: «доступны через пару секунд» was not understood, and a session that stops to
  count down is a session that ends — they would rather go to practice. The replacement keeps the
  one thing the mechanic was protecting — nothing is asked early — by making the SERVER the only
  way back: a set is always the head of the served list, and `stores/review.ts` fetches the list
  again the moment a set finishes, before the summary shows. A missed card whose step has elapsed
  is in the next set because the scheduler put it there; one still maturing shows under «сегодня
  вернутся» on the done screen, which now reads fresh counts instead of the sitting's opening
  snapshot. The miss is *listed* with its answer — on the set summary and again on the done
  screen (`MissedList.vue`) — which is the "other way to recall it" the learner asked for. A set
  is finished when every card in it has been answered once, right or wrong. The client keeps no
  retry list, no retry clock and no waiting state. [R18 §1]'s one-correct-recall criterion is
  now met within the sitting only when the sitting continues past the set of the miss; the note's
  caveats say so.
- **The chance of recall is off the card.** `MemoryMeter` printed «100% вероятность вспомнить
  сейчас» beside five crosses on a card in its learning steps — correct by the model, which
  computes it against an interval of minutes, and to a person a claim that they know a word they
  have just failed five times. A figure that is right and reads as wrong, on the screen where it
  is seen most, is worse than no figure. The stage badge, the marks, the tally and the interval
  change stay; `retrievability` is still served and still shown in the words browser, where every
  card has graduated and the number means what it says.

The options table's "re-serve a failed card immediately — rejected" still stands: the card is
not re-served *at all* inside the set now, and the scheduler's step is respected exactly as
before — by not asking, rather than by waiting.
- **Cost:** the set counter is a client number over a server list. It is derived from the
  served queue and never displayed as an obligation; «к повторению» stays `dueNow`.

## ADR-027 — A grammar tutor: the pack teaches, a grounded model explains, nothing it says becomes a card

**Status:** accepted · 2026-09-02 *(user decision)* · see
[R19](research/19-grounded-tutor.md), [R4](research/04-explicit-instruction.md),
[R10](research/10-llm-tutors.md), [ADR-005](#adr-005), [ADR-014](#adr-014)

**Context.** The grammar screen states the rule and drills it; it cannot answer a question.
"Why -ban here and not -ben?", "how is this different from Russian?", "give me three more
examples with a front-vowel stem" are the questions a teacher takes in ten seconds and the app
took nowhere. The `services.llm` slot has existed since Phase 1 with one consumer (a text-level
second opinion, [ADR-023]). The learner runs small local models, so the design has to assume a
7–8B model on a CPU and Hungarian, where every model is at its weakest ([R19 §4]).

**Options considered.**

| Option | Verdict |
|---|---|
| An open chat with a system prompt | Rejected — a weak model with no material in front of it invents suffixes, and a wrong Hungarian form presented confidently is worse than no answer |
| Embeddings + vector index over a corpus | Rejected — an embedding model is a second model to deploy, the corpus is a few hundred titled sections, and the binary is static ([ADR-001]). Keyword-and-topic scoring over titled sections is enough and testable |
| Reference corpus inside the language pack | Considered and not chosen for now — consistent with lessons living in the pack, but it would bump the pack schema, require `make starter` on every edit, and tie tutor improvements to pack releases. Embedded in the binary (`internal/tutor/kb`), with the pack's own lesson and feature tables read live. Revisit if a second language pack ever exists |
| Let the tutor create cards or drills | Rejected — [R10]'s attestation guardrail is not built, and a card is what the learner is graded against. The tutor explains; the reader and the course make cards |
| Non-streaming answer | Rejected — a local model takes seconds per sentence; an answer that appears all at once after twenty seconds reads as a hang |
| Answer in the lesson's language only | Rejected — the learner asked for the interface language; the client sends its rendered locale, the server accepts `ru`/`en`, falls back to `locale.ui` |

**Decision.** `internal/tutor` assembles one prompt per question from, in this order: the
system rules (in the answer language), the pack's lesson for the open topic, the topic's feature
labels and allomorph tables from the pack, the learner's own state on those features (accuracy,
observations), the best three sections of the embedded reference corpus by topic-and-keyword
score, the last six turns, and — last, per [R19 §3] — the question with the answer-language
reminder. The whole prompt is held under a fixed character budget so it fits an 8k-context
model. `internal/llm` gains a streaming call; the answer streams to the client as server-sent
events and is stored in `tutor_message` (migration `00015`) per user and topic.

**Honesty.** Every answer carries the model's name. The model is asked to put Hungarian
words in backticks; each such word is looked up in the pack's `wordform` table and the ones the
dictionary does not carry are listed under the answer, in the `uncertain` hue, as *not found in
the dictionary* — the same treatment an uncertain analysis gets. A standing note on the panel
says the explanation is the model's and the lesson above is the app's, and that nothing here
becomes a card. With `llm` off the panel says how to enable it and offers nothing else.

**The reference corpus** is CC BY-SA and public-domain material only, with its own
`LICENSES.md`: the pack's own lesson prose is the primary source at runtime; the embedded
files are Wikipedia's Hungarian grammar articles and Wiktionary's Hungarian appendices
(CC BY-SA 4.0, converted from wikitext with attribution), the grammar notes of the US Foreign
Service Institute's *Hungarian Basic Course* (1962, public domain), and reference sections
written for this app. An 1882 grammar on Wikisource was evaluated and excluded: its orthography
predates the reforms and would teach a weak model `cz`.

**Consequences.**
- `GET /api/tutor/:topic` (thread, availability, sources), `POST /api/tutor/:topic` (SSE
  stream), `DELETE /api/tutor/:topic`. One in-flight question per learner; a question is at most
  1,000 characters; a thread keeps its last 200 messages.
- `TutorPanel.vue` under the lesson on the grammar screen, with three suggested questions per
  topic so the first question is one click. Placed after the lesson and never before it — [R4]
  is the order of that screen.
- `.claude/prompts/tutor-system.md` documents the prompt, its versions and failure modes; the
  text itself is embedded from `internal/tutor/prompts/` so Go and the note cannot drift.
- **Cost:** a second LLM consumer to keep honest, an English-only reference corpus behind a
  Russian answer (stated in `docs/TUTOR.md`), and a feature that is only as good as the model
  the learner runs — which the panel says.

---

## ADR-028 — Review has a start screen; the choice of pile and shape leaves the card

**Status:** accepted · 2026-09-03 *(user decision)* · **amends** [ADR-016](#adr-016),
[ADR-024](#adr-024), [ADR-026](#adr-026) · see [UX §4](../docs/UX.md#4-review),
[R18](research/18-sessions-and-progress.md)

**Context.** The review screen is specified as *chrome-free — the card owns the viewport*
(UX §4), and across three decisions it grew a control strip above the card: three pile tabs with
two numbers each ([ADR-016]), «Тренировка ∞» with its own rows for pile, round length and the
weak-words mode ([ADR-019], [ADR-024]), and the set size and order chips ([ADR-026]). Each landed
there because that screen was the one that had the numbers. The learner's report, after a day
with all of it on screen: `Всё 74 +72 · План 37 +32 · Тексты 37 +40 · Тренировка ∞` does not
belong over a card that is answered two hundred times a day. Choosing what to take is a
different moment from taking it, and they asked for a separate screen where the choice is made.

**Options considered.**

| Option | Verdict |
|---|---|
| Keep the strip, fold it behind a disclosure | Rejected — hidden chrome on the card screen is still chrome on the card screen, and a control that is sometimes there is harder to learn than one that is always somewhere else |
| A picker dialog over the card | Rejected — a modal interrupts the loop it sits on; the choice is a page you go to, not an interruption you dismiss |
| A start screen at `/review`, the card at `/review/session` | **Accepted** |
| Persist the chosen pile on the server | Rejected — [ADR-016]'s "no stored preference" stands. The choice is a property of one request and travels in the URL, so a reload is honest and a link can carry it |

**Decision.**

- **`/review` is the start screen** (`ReviewStartView.vue`): the three piles, each with `dueNow`
  and `newNotes` from `Breakdowns` ([ADR-021]) and the day in sets ([ADR-026]); the set size and
  order chips; and a practice block with the round's pile, length and «слабые слова». Nothing on
  it starts a session by itself, and its numbers come from `GET /api/review/queue?scope=all` —
  the same response the card screen reads — so the two screens cannot disagree and no endpoint
  is added.
- **`/review/session` is the card** (`ReviewView.vue`). Its intent travels in the query —
  `?scope=all|course|reader|text:N` for review, `?practice=all|course|reader|trash|text:N` for a
  round — the pattern the trash's «тренировать отложенные» established. The screen starts a
  session on mount and on a query change, and nothing else may start one; the round's length
  and mode are the store's, set on the start screen.
- **The card screen still says what it is in.** The pile name — or «только слова из текста» —
  sits in the progress row; a practice round prints «Тренировка · слова из: План», the
  weak-words criterion when it was really applied, and the fallback echo when it was not.
  [ADR-024]'s printed criterion stays on the screen where the cards are.
- **Today's call to action goes straight to the session** over «Всё» — UX §3: a plan that makes
  you choose has not planned anything. The sidebar row, the topbar chip and the memory loop card
  go to the start screen. The library's, the reader's and the trash's buttons go straight to the
  session with their query, because on those screens the choice has already been made.
- **The done screen links back to the start screen**, and when another pile has work it says so
  in one sentence («В других стопках: к повторению 37, новых 40») from the `Breakdown`s already
  in the queue response — so emptying one pile still does not take a reload to discover the
  other, which is what the tabs on that screen were for. Its «Тренировка» button starts a round
  over the pile just finished, by navigating with `?practice=`.

**Consequences.**
- `ScopeTabs.vue` is deleted; `stores/review.ts` loses `setScope` and `ensureScopes` — the
  strip they served no longer exists, and the store starts nothing on its own.
- Changing the set size or order mid-sitting means going back to the start screen; the
  re-deal-on-change watchers went with the chips. Accepted on purpose: [ADR-026] called the size
  a whim chosen before a sitting, and [R18 §2] is about seeing the end of a sitting, whose length
  is decided before it begins — which is where the dial now lives.
- **Cost:** one more click from the sidebar to a card, and one queue fetch per visit to the start
  screen. Today's «Начать» and every deep link stay one click.
- Unlegislated by the research notes, as [ADR-016]'s split was. The principle it rests on is
  UX §4's own: the card owns the viewport.

---

## ADR-029 — The tutor is a chat, and the app finds the lesson for it

**Status:** accepted · 2026-09-03 *(user decision)* · see [R19](research/19-grounded-tutor.md),
[R4](research/04-explicit-instruction.md), [ADR-027](#adr-027)

**Context.** [ADR-027] shipped the tutor as one thread per learner per *curriculum topic*: a
question could only be asked from under a lesson, or from a page that made the learner pick a
topic first. The first learner's reaction, verbatim: «для меня это должен быть просто как
AI-учитель и всё» — a chat with a new-chat button and a history of old chats, the way every
chat product looks; a topic is something they *may* choose, not a gate they must pass. Two
more things surfaced in the same session: the model wrote Hungarian suffixes in Cyrillic
(«Ем — задний гласный») because the learner had typed them that way, and it explained without a
single example — both prompt failures, both invisible to the lesson-first design because the
lesson had always been the example.

**Options considered.**

| Option | Verdict |
|---|---|
| Keep topic-per-thread and improve the picker | Rejected — it makes the learner know the curriculum's names to ask a question. Which lesson a question is about is the app's job to find, not the learner's to declare |
| A free chat with no grounding | Rejected, still — [ADR-027]'s first row and [R19 §5]: a weak model with nothing in front of it invents suffixes. The retrieval and the lesson stay; only *who picks the lesson* changes |
| Ask the model which topic the question is about | Rejected — a second model call per question on a CPU that already takes ten seconds for the first, and an unexplainable verdict. Lexical and morphological scoring over the pack's own titles, feature labels, allomorph shapes and lesson vocabulary is deterministic, fast, and its verdict can be printed |
| One thread per learner, forever | Rejected — a chat that never starts over carries the wrong lesson into the next question; a new chat is how a learner says "different subject" |

**Decision.**
- **A thread is a conversation** (`tutor_thread`, migration `00016`), with a title from its
  first question, an optional **pinned topic**, a new-chat button, a list, and delete. The
  lesson panel pins its topic — a question asked under «Гармония гласных» stays about it — and
  opens the most recent thread pinned there; «Открыть в AI-учителе» opens that same thread.
- **Every question is grounded, and the grounding is chosen per question.** A pinned topic
  wins. Otherwise `tutor.InferTopic` scores the question against every curriculum topic: the
  pack's analyser reads each Hungarian word in it and the features it carries point at the
  topic that teaches them (`házban` → `Case=Ine` → the locative grid); the topic's titles and
  feature labels in both languages, its allomorph shapes (`-ban`, `-ek`) and its lesson
  vocabulary score the rest. Below the threshold, the thread's **last grounding is sticky** —
  «а ещё примеры?» stays on the lesson the previous answer used — and with nothing at all the
  prompt says so in words and the reference corpus is retrieved by the question alone.
- **The grounding is printed.** `tutor_message.topic_code` on an assistant turn is the lesson
  that answer was built on, and the panel prints «по уроку: Гармония гласных» under it, or
  «по справочнику, без урока» when there was none. A label the learner can disagree with is
  the whole honesty of an inferred choice ([ADR-014]'s rule again).
- **Prompt version 2.** Hungarian is always written in Latin Hungarian orthography in
  backticks, never transliterated, even when the learner typed «ем»; every explanation carries
  two or three whole-word examples with a translation; the no-lesson case is stated to the
  model rather than left blank. `.claude/prompts/tutor-system.md` v2.

**Consequences.**
- `GET /api/tutor` lists threads; `POST /api/tutor {topic?}` opens one; `GET/POST/DELETE
  /api/tutor/:id` read, ask, delete; `GET /api/tutor/topic/:code` resolves the lesson panel's
  thread. The per-topic routes are gone. Existing threads are migrated one-to-one into pinned
  threads, so nothing a learner asked is lost.
- The learner's first question no longer needs a topic; the picker on the AI-teacher page is an
  option on the new-chat button, not the gate to it.
- `tutor` still imports nothing from `curriculum`: the index it scores is built by the handler
  from the DAG and the pack, and the inference is a pure function with a test.
- **Cost:** one more table, and a heuristic that can pick the wrong lesson. The label is the
  mitigation, and pinning is the override.
- `nothing it says becomes a card` is untouched: `handlers_tutor_test.go` still counts the
  tables before and after a question.

---

## ADR-030 — The tutor translates into Hungarian on request: as an explanation, word by word, never as material

**Status:** accepted · 2026-09-03 *(user decision)* · see [R19](research/19-grounded-tutor.md),
[R4](research/04-explicit-instruction.md), [R10](research/10-llm-tutors.md), [ADR-014](#adr-014),
[ADR-027](#adr-027), [ADR-029](#adr-029)

**Context.** The first learner typed «напиши на венгерском "Я пришел в эту деревню и зашел в
свой дом"» into the AI-teacher chat and got the Russian sentence back, unchanged, labelled
«ответ модели qwen3:1.7b · по уроку: Локативная система». The same model, asked the same thing
outside the app, produced `Eljöttem ebbe a faluba, és bementem a házamba.` with a word-by-word
breakdown — and, when the learner offered their own attempt, a comparison explaining each
difference. That breakdown is what the learner called "a tutor"; the echo is what they called
«очень тупой учитель».

The echo was the prompt's doing. Prompt v2 opened with «Отвечай ТОЛЬКО ПО-РУССКИ» and closed
with «Отвечай по-русски.» — the first and last lines, exactly the two positions a small model
follows most reliably ([R19 §3]). Asked for Hungarian, the model obeyed the language rule over
the request. Nothing in the prompt said what a translation request should produce, and rule 4
(«три–восемь предложений») forbade the breakdown even if it had tried.

**Options considered.**

| Option | Verdict |
|---|---|
| Refuse translation requests: the tutor explains grammar, it is not a translator | Rejected — «как сказать …?» is the commonest question a learner has, and its answer *with the mechanism named per word* is explicit instruction at the moment of need ([R4]), which is the tutor's whole job. Refusing it sends the learner to a chat product with no lesson, no labels and no dictionary check |
| Route Russian→Hungarian through `services/mt` | Rejected — NLLB-200 can generate Hungarian, but [ADR-014] made MT display-only for "what does this sentence say", it labels a sentence, not a word, and it explains nothing. A translation without the breakdown is exactly the answer the learner is not asking for |
| Loosen the language rule only | Insufficient — a model that may write Hungarian still does not know that a translation answer has a *shape*. The shape is the pedagogy |
| **Reword the language rule to speak of the explanation, and give translation requests an explicit shape and budget** | **Chosen** |

**Decision.**
- **The language rule names the explanation, never the material.** «Объясняй по-русски. Всё
  венгерское — слово, форму, суффикс, целое предложение — пиши по-венгерски, латиницей». The
  closing line says the same. `ONLY` does not appear; `tutor_test.go` pins its absence.
- **A translation request has a shape** (prompt rule 4): the whole Hungarian sentence in
  backticks; its literal rendering; then **word by word** — what each word or phrase means,
  which suffix or preverb it carries and why that one (case, harmony class, definiteness,
  person, number); and, when the learner offered a version of their own, a comparison that
  explains every difference. A Russian sentence without the Hungarian is stated not to be an
  answer. This is the one answer allowed to run past eight sentences — up to fifteen lines.
- **The budget follows.** `MaxAnswerTokens` 700 → 1,000 and the stream timeout 90 → 150 s,
  so a breakdown on a laptop CPU is not cut by a limit sized for eight sentences. **An answer
  cut by the budget is marked**: `llm.Stream` reports `finish_reason: length` as
  `llm.ErrTruncated`, and the handler stores it `interrupted`, the same flag a timeout sets —
  an answer that stops mid-word without saying so reads as complete.
- **The dictionary check reaches a whole sentence.** `tutor.Backticked` split only spans up to
  80 characters; a translated sentence is longer. The cap is 200 — a backticked sentence is
  checked word by word, so a stem the model invented inside it is listed under the answer.
- **The page names a small model.** The panel prints «модель qwen3:1.7b — маленькая: 1,7 млрд
  параметров, судя по названию; учитель рассчитан на 7–8 млрд» when the tag carries a size
  under 7B. The size is read off the tag and the copy says so; a tag that names no size shows
  nothing. `docs/TUTOR.md` has said since v1 that a 1–2B model is "fluent, labelled, and
  frequently wrong"; the honesty rules say a known limitation is stated *in the product*.
- Prompt **version 3**; `.claude/prompts/tutor-system.md` v3.

**What does not change.** The line [ADR-027] drew: the translation is an explanation on the
screen it was written on. There is still no code path from `tutor_message` to `note`, `card`,
`user_gloss` or either state space, and `handlers_tutor_test.go` still counts those tables. The
tutor's Hungarian is labelled (the model's name) and dictionary-checked (every backticked word);
it is not corpus-attested, and that is why it becomes nothing.

**Consequences.**
- A learner can ask «как сказать …» and get the sentence and the reason for each form —
  or, with a 1–2B model, a labelled attempt at one, under a line that says what the model is.
- A translation answer costs up to 1,000 tokens: on a CPU, up to two minutes of streaming. The
  stream keeps it visible.
- **Cost:** a small model may still translate wrongly, fluently. The mitigations are the ones
  the tutor already had — the model's name on the answer, the unattested list, the lesson
  label — plus the size line. None of them makes a wrong sentence right; they make it visible.
- **Measured** (2026-09-03, `docs/TUTOR.md` §9): `qwen3:1.7b` cannot translate the learner's
  sentence with *no* prompt at all — `Ígép a városba és bejártam a házam` — thinking on or off;
  with prompt v3 it wrote Hungarian, as v2 had not, but printed the lesson's example table
  instead of a translation. The prompt was wrong and is fixed; the model is too small and the
  page now says so. The breakdown itself is verified only against the stand-in in tests until
  a 7–8B model is on the machine — the first learner has `qwen3:1.7b` only.

---

## ADR-031 — The chat has a chat's controls: stop, edit-and-resend, a context meter, and the whole screen

**Status:** accepted · 2026-09-03 *(user decision)* · see [ADR-029](#adr-029), [ADR-030](#adr-030),
[R19 §3](research/19-grounded-tutor.md)

**Context.** The first learner's second round of feedback on the AI-teacher page, after
installing `gemma3:4b`: an answer that ran out of budget was shown with the interrupted mark
AND «Модель не ответила пригодным образом. Ничего не изменилось» under it — a cut is not a
failure, and "nothing changed" was false, the partial answer was stored; there was no way to
stop an answer that was going wrong; no way to fix a question except to type it again under
the wrong answer; nothing said how much of the model's window the prompt took, on a machine
where Ollama's window is a slider the app cannot see and a prompt longer than it is cut from
the head silently (`docs/TUTOR.md` §9); and the chat itself was a small box under a large
page title, two notices and a footnote — «окно чата очень маленькое, смотри на дизайн Клода
или ChatGPT».

**Options considered.**

| Option | Verdict |
|---|---|
| Cancel by closing the fetch on the client | Rejected — the server notices only on its next write, the partial answer is stored a moment later, and the client has no id for it. `POST /api/tutor/:id/stop` cancels the flight's context on the server; the stream then ends the ordinary way, `done` with `interrupted`, and the client keeps the same code path |
| Edit as a branch, the way ChatGPT keeps both versions | Rejected — a thread is a conversation with one lesson found per question; a tree of them is a second UI for no pedagogical gain. Editing a question deletes it and everything after it and asks again; the UI offers it on the last question |
| Ask the learner for the window size in Settings | Rejected — a figure typed by hand drifts from the slider the moment it is moved. Ollama's `/api/ps` reports the running model's `context_length`; the OpenAI stream reports `usage` when asked (`stream_options.include_usage`). Both measured on the learner's own server. A server that reports neither gets "the model does not report its window", never a guess |
| Estimate tokens client-side | Rejected — the binary carries no tokenizer (ADR-027's budgets are in characters for that reason), and a wrong number here is the kind of precision that misleads |

**Decision.**
- **Any text that arrived is an answer.** The stream ends with `done` and the stored message
  whenever the model wrote anything: `interrupted` marks a timeout, a stop, a lost connection
  or a budget cut, and `cutReason` says which, so the label under the answer names what
  happened («ответ остановлен», «закончился отведённый объём») instead of guessing at two. `error` is sent only when nothing arrived — `errors.tutorFailed`, or
  `errors.tutorStopped` when the learner stopped it first.
- **Stop.** `POST /api/tutor/:id/stop` cancels the learner's in-flight answer (the flight map
  now holds a cancel function); idempotent, 204. The button replaces «Спросить» while an
  answer streams; `Escape` does the same.
- **Edit.** `POST /api/tutor/:id {question, replace: <id>}` deletes that question of the
  learner's own and everything after it (`Store.TruncateFrom`), re-titles a thread whose
  first question it was, and asks. 404 `errors.tutorNoMessage` for a turn that is not theirs
  or not a question. The UI offers it on the last question; the composer says what will go.
- **The context meter.** `llm.Stream` asks for usage and returns it; `llm.ContextLength` reads
  `/api/ps` on a `local` server after the answer; both are stored on the assistant turn
  (`prompt_tokens`, `answer_tokens`, `context_tokens`, migration `00017`) and printed under the
  composer as a bar and a sentence: «Окно модели 8 192 токенов: последний вопрос и ответ
  заняли 3 850, свободно ≈ 4 342». When prompt + answer reach the window, the line turns into a
  warning that the head of the prompt — the rules and the lesson — may have been cut, and
  names the fix. Zero from the server prints as "does not report", never as a number.
- **The layout is the chat.** No page title (the top bar already names the page); the
  conversation fills the viewport under the bar; the log scrolls, the composer sits at the
  bottom with the notice, the meter and the sources under it; messages sit in a reading
  column; an assistant turn is text, not a card; the openers live in the empty log.

**Consequences.**
- Two new request shapes, one new route, three columns; `handlers_tutor_test.go` pins the cut,
  the stop, the edit and the figures.
- The meter is only as good as the server: Ollama gives both figures, a hosted API gives
  usage without a window, an old llama.cpp may give neither. The copy says which.
- `nothing it says becomes a card` is untouched.

---

## ADR-032 — The tutor is one key away from the review card, after the answer

**Status:** accepted · 2026-09-03 *(user decision)* · **superseded in part by [ADR-034](#adr-034)**
(both phases; the server composes the question) · see [ADR-027](#adr-027), [ADR-029](#adr-029),
[ADR-031](#adr-031), [R4](research/04-explicit-instruction.md)

**Context.** The first learner, on the review card, after typing `tudom` where `ismer` was
wanted: «добавь кнопку AI, типа я мог бы что-то спросить у учителя прям тут, не выходя из
кассы». The moment a card is marked wrong is exactly the moment [R4]'s explicit instruction is
worth having, and the AI-teacher page was three clicks and a retyped question away from it.

**Options considered.**

| Option | Verdict |
|---|---|
| A link to the AI-teacher page | Rejected — it leaves the session, and the learner must describe the card again. The point is asking *about this card* |
| Auto-ask the tutor on every wrong answer | Rejected — a second model call per miss on a machine where the model is the slowest thing; and an answer nobody asked for is noise under a card. The question is written for the learner, and they send it |
| Offer the tutor in both phases | Rejected — before the answer, a model with the expected form in its prompt is a hint that bypasses grading. «Подсказка» is that, and it is recorded (`usedHint`); the tutor is not recorded and must not become a second door |
| A chat per card | Rejected — twenty threads a session. One chat per sitting: the first question creates it, the next card's question lands in it, and it is listed on the AI-teacher page like any other |

**Decision.**
- **«Спросить учителя», key `a`, in the ANSWER phase only**, beside «В корзину». It opens the
  tutor in the side column the word inspector uses (beside the card on a wide screen, under
  it on a narrow one). `Esc` closes the panel; an `Esc` the composer keeps (to stop an answer
  or back out of an edit) is not passed on to the card.
- **The composer opens with the question written**, in the learner's language, from the
  card: the direction, the cue, what they typed and what was wanted, the sentence and its
  translation, the Hungarian in backticks so the lesson is found from it (ADR-029). Three
  shapes: a wrong typed answer, a right one, and a card that was only shown. Filled, never
  sent by itself.
- **One chat per sitting.** `TutorAside.vue` keeps the thread id the first question created
  and hands it back on the next card; the link on the panel opens the same chat on the
  AI-teacher page.
- **The card's keys stay out of the composer.** A key pressed in the tutor's textarea is the
  composer's (`Enter` sends there, not «Проверить»); the review's handler returns on a
  textarea target.

**Consequences.**
- No server change: the aside mounts `TutorConversation` with `fresh` and a new `seed`
  prop; the thread is a free chat, grounded per question.
- Nothing the tutor says touches the card: the verdict is already on the screen, and there
  is still no code path from `tutor_message` to a state space.
- **Cost:** a wider review layout while the panel is open, and a question template that has
  to be kept honest as the card gains fields.

---

## ADR-033 — The ear trainer plays words: every minimal-pair member is in the dictionary

**Status:** accepted · 2026-09-03 *(user decision)* · see [R7](research/07-hvpt.md),
[ADR-010](#adr-010)

**Context.** The first learner on the ear trainer: «как будто таких окончаний слов или слов
нет». They were right. Twelve of the thirty-two pair members in `langpack/pipeline/minimal_pairs.py`
were non-words — `hus`, `tur`, `tuz`, `fold`, `sak`, `szák`, `csél`, `hadj`, `ana`, `tuk`,
`bűsz`, `szok` — glossed «(нет слова)» and presented in a 2AFC trial as the partner of a real
word. The honesty rules refuse an unattested form as a drill answer everywhere else in the
app; the ear trainer had been shipping them as stimuli, with a label a learner does not read
while listening.

**Options considered.**

| Option | Verdict |
|---|---|
| Keep non-words and label them louder | Rejected — a nonce form teaches a shape the language does not have, and the learner's ear cannot tell which member was the real one. Nonce stimuli are a laboratory convenience, not a course |
| Drop every contrast that has no real pair | Rejected as the first move — real pairs exist for most: `tok`/`tök`, `sál`/`szál`, `csók`/`sók`, `zsír`/`szír`, `cikk`/`csikk`, `hagy`/`had`, `huzat`/`húzat`, `hurok`/`húrok`, `szurok`/`szűrök`, `szál`/`száll`. Two contrasts (`ny`/`n`, `ty`/`t`) have no attested pair the dictionary carries and are dropped rather than faked |
| **Real words only, verified by the build against the pack's own `wordform` table** | **Chosen** |

**Decision.**
- **Every pair member is attested.** `minimal_pairs.unattested(conn)` runs in the full build
  after the lexicon and in `refresh` before it writes; either refuses the pack and names the
  form. `validate()` also refuses the «(нет слова)» marker itself. Thirty pairs across
  sixteen contrasts, all checked against the 2-million-form dictionary.
- **Clips are named by the word, not the pair id.** `refresh` reassigns ids in list order,
  and a clip named `5_a_anna_100` was kept — file present, render skipped — for whatever pair
  5 had become. `clip_name(word, voice, rate)` folds the diacritics for the file system and
  keeps `kor` and `kór` apart with a hash; a word shared by two pairs (`szír`) is one clip.
- **Audio rows follow their pair through a refresh.** Old ids are mapped to new by `(a, b)`
  through negative temporaries; rows of a pair that is gone are dropped. `render` skips only a
  clip whose row AND file exist.
- The starter pack and its `media.tar.gz` are rebuilt from a refreshed copy of the full pack
  and committed ([ADR-010]); the learner's own pack needs `make refresh audio` once.

**Consequences.**
- The trainer says nothing it did not before; it stops saying `bűsz`.
- **Cost:** two palatal contrasts fewer, and one more reason the pack build needs the
  dictionary before the pairs.

---

## ADR-034 — The tutor reads the card: prepared questions are composed by the server, and asking before the answer is a hint

**Status:** accepted · 2026-09-04 *(user decision)* · **the hint half is superseded by
[ADR-046](#adr-046)** — asking the tutor no longer caps a grade; everything else stands ·
see [R4](research/04-explicit-instruction.md),
[R9](research/09-corrective-feedback.md), [R10](research/10-llm-tutors.md),
[R19 §3](research/19-grounded-tutor.md), [ADR-017](#adr-017), [ADR-027](#adr-027),
[ADR-032](#adr-032) (superseded in part). 

**Context.** ADR-032 put the tutor one key away from the review card, after the answer, with
a question the CLIENT composed from the card and filled into the composer. The first learner
wanted more and different: the button top right, in both phases; **prepared questions** that
depend on what the card asks — «разбери предложение», «почему эта форма, а не та», «переведи
слово с примерами» — one click each, answered in a fixed shape; and their own questions too.
Two things the client-composed seed could not do: it did not know the facts (the feature
labels, the allomorphs, the pack's translation, the diff's class live on the server, and the
client may not derive the expected answer — ADR-017), and it offered nothing before the
answer, where [R9]'s *hint* step lives.

**Options considered.**

| Option | Verdict |
|---|---|
| Keep composing the question on the client | Rejected — the client has no labels, no allomorphs, no diff class, and no right to the expected answer; every prepared question would be a paraphrase of what the screen shows |
| Auto-ask on every wrong answer | Rejected — a second model call per miss, and an answer nobody asked for under every card |
| Offer the tutor before the answer as a free extra | Rejected — a model with the expected form in its prompt is a hint that bypasses grading; «Подсказка» is recorded, and this must be too |
| A server-side «was the tutor consulted» check on the grading path | Rejected — it would be the first read of a tutor table on the review path, and a tampered client can omit `usedHint` on «Подсказка» just as well |
| **A structured request; the server composes and withholds; the client records the hint** | **Chosen** |

**Decision.**
- **`POST /api/tutor/:id {card: {cardId, phase, template?, typed?}, question?, lang}`.** With a
  template, the server writes the visible question in the learner's language from the card
  and the pack (`tutor.Compose`); without one, the learner's own text is asked with the card
  as context. Either way a **«КАРТОЧКА» / "CARD" block** — the sentence with its translation
  and label, the word and its dictionary form, the glosses, the graded features with the
  pack's labels and allomorphs, what was typed against what was wanted and the diff's class —
  goes into the prompt right after the lesson, and the grounding takes the card's own
  `targetFeats` before any word of the question. Prompt version 4; the reference budget
  yields room to the card so the total does not grow.
- **Templates and phases.** Nine templates (`tutor.Templates`): `breakdown`, `whyForm`,
  `whyNotMine`, `translateWord`, `spelling`, `explainTask` after the answer; `hintForm`
  (recognize), `hintWord` (produce), `hintRule` (cloze) and `explainTask` before it.
  `tutor.Allowed` refuses a template outside its phase with 400 — that is what stops a client
  from asking «почему `házban`» before answering. The client chooses WHICH chips to show
  from the direction, phase and outcome (UX.md §4); the server decides what may be asked.
- **A prepared answer has a shape.** Each template appends numbered sections to the question
  («1. Что такое `X` 2. Почему здесь `Y` 3. В чём разница — назови механизм 4. Два примера»)
  and switches rule 7's check question off; `whyNotMine`'s section 3 follows the diff's
  error class (harmony → the stem's class and three suffix variants; `stem` → two different
  words; …). Free questions keep rule 7.
- **Before the answer, a question is a hint** *(reversed by [ADR-046](#adr-046), 2026-09-22 —
  the cap is gone; the withholding below is what remains)*. The client sets `usedHint` the moment a
  question about the card is sent in the question phase — the channel «Подсказка» uses —
  and `srs.DeriveRating` caps the grade at Hard. The card is NOT revealed: the learner still
  answers, and the answer is graded and logged. The panel says so before the first question;
  «Хорошо» and «Легко» carry «засчитается как «Трудно»» and show the Hard preview. The
  prompt's block **withholds the answer** in that phase — no expected form on a typed card, no
  dictionary word or sentence on a produce card, no meaning on a receptive one, nothing
  Hungarian on an audio card — says so first, where a budget cut cannot reach it, and rule 9
  tells the model to explain the rule and not name it.
- **One review chat per WORD per local day** *(amended 2026-09-04 — see below; originally one
  per day)*. `POST /api/tutor {kind: "review", note}` finds that word's chat or creates it;
  `GET /api/tutor/review?note=` finds it without creating; `tutor_thread.kind` /
  `tutor_thread.note_id` (migrations `00018`, `00019`) and `tutor_message.card_id /
  card_phase / template` record what was asked about which card — on the tutor's side only.
  A review chat is named by its word, written at creation from `note.target_form`, never by
  a question.
- **The question limit is characters.** The composer counted characters, the server bytes,
  and a Russian question of eight hundred letters was refused.

**Consequences.**
- `handlers_tutor_test.go` pins: the composed question and the CARD block, the grounding from
  `targetFeats`, the withholding on a produce card, the refusal of a template outside its
  phase, another learner's card, the day's chat, a thousand Cyrillic characters; and still
  counts the five tables before and after a question — nothing the tutor says becomes a card.
- **Cost:** a small model may name the form anyway in the question phase. It is a hint either
  way: the cap is applied when the question is sent, before any answer arrives. *(ADR-046
  removed the cap; the cost stands, unpriced and stated.)*
- ADR-032's «answer phase only» and its client-composed seed are superseded; its side column
  and one-chat-per-sitting stand.

**Amendment, 2026-09-04 — the chat is per WORD, not per day** *(user report)*. One chat a day
sounded like continuity and read as its opposite. The panel opened on whatever was asked
last, so a learner looking at `Örülök` was shown a paragraph about `jöttél`; and because a
thread is sent whole, every later question carried that answer into the model's window —
8 192 tokens, of which one breakdown is ~1 500. The learner's words were «на новом слове
учитель всё равно показывает старый чат, должен новый».

- `tutor_thread.note_id` (migration `00019`) keys the chat on the NOTE, still within the
  local day: the same word ten minutes later — its learning step — continues the
  conversation, and the next word opens an empty one. The day boundary stays, so a chat
  cannot grow across a week.
- The word **names** the chat. `title` on a review thread is written at creation from the
  note, and the rail reads «Повторение · `jöttél`» instead of a column of identical dates;
  the Hungarian half is a `lang="hu"` span (ADR-036), not an interpolation. Threads from
  before the migration have `note_id = 0` and no title, and keep the date.
- The client cannot open the wrong chat by accident: `ReviewView` keeps the id **with** the
  note it belongs to and reads it back only for that note, so no watcher ordering can leave
  the previous word's id in the panel's props. `useTutorThread` carries a generation counter
  for the same reason on the other side — an answer still streaming for the previous word
  finishes on the server and is stored in the chat it was asked in, and stops writing to the
  panel; without it the previous word's answer went on streaming into the next word's empty
  log, under a «Стоп» button belonging to another conversation.
- **The standing notice and the sources list leave the review panel.** «Объяснение
  принадлежит модели… Справочный материал» is a paragraph of policy that is read once and is
  chrome by the tenth card, in a column that has a sentence of room; it stays on the chat
  screen and under the grammar lesson. Nothing per-answer moves: the model's name, the lesson
  used, the unattested words, the small-model note and the context meter are on every answer
  as before, and those are what the honesty rules actually require.

---

## ADR-035 — A translation in the wrong language says so, and the translator is offered wherever one is missing

**Status:** accepted · 2026-09-04 *(user decision)* · see [ADR-014](#adr-014), [ADR-018](#adr-018)

**Context.** The pack translates ~19,600 of its sentences into English and ~3,700 into
Russian. A Russian-locale learner therefore met English under most cards — printed with no
label, as if it were the pack's Russian — and the crib's «Перевести предложение» was drawn
only when NO translation was in hand (`canAsk = !shown && caps.available('mt')`), so the case
that needed the translator most never offered it. The word inspector's example sentences did
the same with a third copy of the language-picking rule, and had no translate control at
all. The learner's report: «нет кнопки перевести вообще». And the crib kept a fetched
translation across cards — no key, no watcher — so one sentence's Russian could sit under the
next.

**Options considered.**

| Option | Verdict |
|---|---|
| Show only the locale's translation; blank otherwise | Rejected — an English line beats no line; it only has to say what it is |
| Fetch the Russian for every English-only card in the queue | Rejected — the sidecar is never on the review path ([ADR-018]); a fetch is the learner's press |
| Replace the human English with the machine Russian | Rejected — the human line is the one to trust; the machine's goes under it, labelled |
| **Label the fallback, offer the translator under it, key the crib to the sentence, one picker everywhere** | **Chosen** |

**Decision.**
- **One picker.** `lib/gloss.ts` `pickGloss` decides the language and reports the fallback;
  `pickGlossList` does it for a list, `glossLine` appends the short mark «(англ.)» for a
  one-line row. The review card, the inspector, the reader's examples, the trash and the
  words browser all go through it; no view keeps a `ru || en` of its own.
- **The crib says the language.** `SentenceCrib` takes `isFallback` and prints
  «(по-английски — русского перевода в пакете нет)» under a pack line in the other language.
  A pack line still carries no model — a person wrote it.
- **The translator is offered wherever the learner's language is missing**, including under
  an English pack line, when `services.mt` is reachable: «Перевести на русский». The machine
  line is printed BELOW the human one with the model beside it ([ADR-014]); both stay.
- **The crib is keyed to the card** and resets its fetch when the sentence changes.
- **The inspector uses the same component** (`size="inline"`) for its example sentences and
  the same picker for the entry's glosses.
- The question-phase rule is untouched: no sentence translation before the answer on
  `recognize`, `listen` and `spell` — it is the answer; `cloze` keeps its crib ([ADR-018]).

**Consequences.**
- No server change; `POST /api/translate` and its cache already do the work, and the queue
  still never calls the sidecar (`TestTheQueueNeverCallsTheTranslator`).
- **Cost:** a second line under many cards. It is the honest one.

---

## ADR-036 — Two faces: a serif for the material and the headings, Inter for the instrument

**Status:** accepted · 2026-09-04 *(user decision)* · see UX.md §14.2, [ADR-004](#adr-004)

**Context.** The first learner's verdict on the look: «очень шаблонный, сгенерённый нейронкой»
— one sans for everything, grey boxes, the icons everyone has. Asked what would help without
turning the app bright, they chose typography: a serif with character for the headings and
for the Hungarian text, which should be larger and warmer; the interface stays as it is.
Fraunces was the first candidate and was dropped when checked: its subsets are Latin,
Latin-Extended and Vietnamese — no Cyrillic — so every Russian heading would have fallen back
to Inter and only the English locale would have got the serif. Literata (Google Fonts, OFL,
variable, made for reading on screens) has Cyrillic and Latin-Extended, which is what a
Russian heading and a Hungarian sentence need.

**Options considered.**

| Option | Verdict |
|---|---|
| Fraunces for the Hungarian only, headings in Inter | Rejected by the user — the headings were the point |
| Two serifs, Fraunces for Hungarian and Literata for headings | Rejected — two serifs beside each other argue, and ~120 KB more |
| Load from Google Fonts | Rejected — the shell works with no network; fonts are bundled subsets |
| Consolidate ten views onto `PageHeader` first | Deferred — ten header layouts differ (actions, intros, stats); the typography goes through one utility instead, and the consolidation stays on the polish list |
| **Literata for both roles, self-hosted subsets, one lever per role** | **Chosen** |

**Decision.**
- **Tokens:** `--font-display` and `--font-hungarian` (both Literata Variable, serif fallbacks)
  beside `--font-sans` (Inter) and `--font-mono` in `assets/tailwind.css`.
- **Hungarian is marked, never classed.** `lang="hu"` on the element, and one base rule sets
  the face, `1.0625em` and optical sizing; a nested `lang="hu"` inherits; `input`/`textarea`
  keep Inter because their diff is set in mono beside them; utilities win, so `font-mono`
  wells and every `text-*` size stay as authored.
- **The page title is one utility**, `page-title` (display face, the 30px step, weight 500,
  no tracking), used by `PageHeader.vue` and every view's `h1`; the wordmarks and the
  reader's text title use `font-display`.
- **Self-hosted subsets only** (`assets/fonts.css`): Inter latin, latin-ext, cyrillic,
  cyrillic-ext; Literata latin, latin-ext, cyrillic. The bare `@fontsource-variable/inter`
  import pulled Greek and Vietnamese too.
- **The favicon** is the teal identity (`#00686b`, the light `--primary` as sRGB) with a
  paper-white serif «ő»; the old one was the abandoned blue.

**Consequences.**
- The palette and its contrast comments are untouched; weight 500 in the serif is checked
  in both palettes in the browser.
- **Cost:** a second family in the bundle. The subset trim on Inter pays for most of it.

## ADR-037 — Decorative line drawings: one colour, folk motifs, on empty screens and at the two doors

**Date:** 2026-09-04 · **Status:** accepted

**Context.** The first learner's verdict on the look was «очень шаблонный, сгенеренный
нейронкой». Half of that is answered by the typography (ADR-036); the other half is that the
app had no picture of its own anywhere — the only SVGs in the tree were the favicon and the
lemma curve. Empty states were a grey icon over a sentence, the done screen a green tick, the
404 a number. Every one of those is what a scaffold ships.

The constraint is UX.md §14.5: nothing that reads as a reward. And §14.1: the palette is
measured, and a second colour system on top of it is a second thing to keep in contrast.

| Option | Verdict |
|---|---|
| A stock illustration set (unDraw, Humaaans, Storyset) | Rejected — the style is exactly the template the learner recognised, and each set brings its own palette |
| Coloured blobs / gradients behind headings | Rejected — fights the measured palette; decoration on every screen is chrome, not character |
| A mascot | Rejected — a face that reacts to your answers is [R16 §3] gamification with a smile |
| Animated illustrations | Rejected — §14.3's motion budget is for feedback, not for décor |
| Nothing; typography alone | Considered — but the empty states still say «here is where a list would be» in the voice of a framework |
| **Five line drawings in the manner of Hungarian folk embroidery, one colour, decorative** | **Chosen** |

**Decision.**
- **Five motifs**, drawn as paths in `web-ui/src/components/art/`: a **tulip** (the plains'
  motif), a **pair of birds** over a sprig, a **pomegranate rosette** (the round motif that
  closes a Matyó border), a **sprig of wheat**, and a horizontal **vine** of leaves and buds
  for a hem. Kalocsa and Matyó are where the shapes come from; nothing is traced.
- **One colour, `currentColor`, 1.5 px, no fills.** The same file sits on either palette;
  the wrapper sets the tone — `muted` (70 % of `--muted-foreground`) by default, `accent`
  (50 % of `--primary`) where the drawing marks an arrival. Decorative contrast is 2–3:1 on
  purpose: it must not compete with the sentence beside it.
- **`aria-hidden`, `focusable="false"`, no animation, under 2 KB a motif.** The words next to
  a drawing carry the meaning; the accessibility tree is unchanged.
- **Where:** `Illustration.vue` (`name`, `size`, `tone`) and an `art` prop on `EmptyState`.
  Placed on the library's two empty states, the trash, the words list, the tutor's history,
  the review start screen when the whole deck rests, the review done screen, the 404, and
  under the wordmark at login and setup. **Not** on a `Note` (its colour is the icon's), not on
  a card, not on the Progress screen, and not in a header on every screen — a vine on every
  page is chrome again.
- **Never a metric.** No rings, no flames, no numbers, nothing that fills as you go — §14.5.
  A drawing changes with the screen, never with the learner's figures.

**Consequences.**
- Decorative only: nothing to localise, nothing to test for behaviour. The a11y check is that
  the tree is identical with the drawings present.
- Both palettes are looked at in the browser before a motif ships; the tone classes are the
  only place a value could drift.
- **Cost:** a few kilobytes of inline SVG in the bundle, measured at the build.
- Documented in UX.md §14.9; CLAUDE.md carries the one-line rule.

## ADR-038 — Ready for a stranger: one voice for failures, a shell that does not flash, docs that describe what ships

**Date:** 2026-09-04 · **Status:** accepted

**Context.** The release checklist for `v1.0.0`, read as someone who has never seen the code
would read it. What they would have met:

- **Failures in three dialects.** The tutor and translator answer with i18n keys, the older
  handlers with English prose (`invalid email or password`), and the client mints sentinels for
  what never reached a handler (`network-unreachable`, `request-failed-503`). Sixteen call
  sites turned those into text in four different idioms, and four of them printed the sentinel
  raw. Nothing caught what nobody caught: an unhandled rejection was a silent screen.
- **A dark install flashed light.** The theme was applied by the store after the bundle ran;
  `index.html` shipped `class="light"` with no `theme-color`, no description.
- **Docs describing a different product.** README and UX.md §13 described a PWA with offline
  review and sync that does not exist; ARCHITECTURE.md said the queue was precached;
  DEVELOPMENT.md's target table missed six targets and said the pack is built in CI.
- **Checks that ran only in CI.** Locale parity and the research-citation checks lived inline
  in `ci.yml`; `make test` ran without the race detector CI runs with. A learner could push a
  broken locale and learn it from a red badge.
- **A pipeline that found the wrong Python.** `PYTHON ?= python3`, a venv mentioned only in a
  comment, `requirements.txt` listing pytest alone; `make refresh` did not exist. The first
  learner's `make audio` reported `rendered 0` because the system interpreter has no Piper.
- **A constant in two places.** `MIN_PASSWORD = 10` in two views, mirroring
  `auth.MinPasswordLength` by hand.

**Decision.**
- **One function for every failure a learner sees:** `web-ui/src/lib/errors.ts`
  `describeError`. Keys are looked up; the prose the handlers write for a learner is mapped to
  a key by its exact wording (a rewording on the server surfaces as an English sentence, never
  a crash); sentinels go by status (`0` → unreachable, 401 → session ended, 403 → not allowed,
  429 → too many attempts, ≥ 500 → the server's log); a sentence never seen is shown as the
  server wrote it, because a validation message beats «что-то пошло не так». The server's
  wording is NOT changed: its tests match those strings, and the map is the cheaper side.
- **A global handler** (`main.ts`): `app.config.errorHandler` and `unhandledrejection` both
  log the stack and toast `describeError`, one toast per sentence per two seconds; a 401 that
  reaches it clears the session and sends the learner to login with the way back.
- **The shell before the app:** an inline script in `index.html` applies the stored theme (else
  the OS preference) and the stored locale to `<html>` before the bundle loads, and sets
  `theme-color` to the page background of that palette; `stores/ui.ts` keeps doing the same,
  idempotently, and follows the toggle. A `description` for the tab and for link previews.
- **Docs say what ships.** Offline is **planned** (UX.md §13, ARCHITECTURE.md, README);
  the pack is built on a workstation; every `make` target is in the table; the upgrade path for
  a learner's own pack is written down; the GHCR image is named as something a tag produces.
- **The checks run locally:** `web-ui/scripts/check-locales.mjs` inside `npm run lint`,
  `scripts/docs-check.sh` as `make docs-check` inside `make test`, and `ci.yml` calls those
  scripts rather than carrying a second copy. `GO_TEST_FLAGS` defaults to `-race -count=1`.
- **The pipeline owns its interpreter:** `make venv` (run by `make deps`) creates
  `langpack/.venv`, `PYTHON` finds it on its own, `make audio` checks for ffmpeg and Piper
  before rendering, `make refresh` exists, and `requirements.txt` lists what the targets import
  (`requirements-build.txt` adds HuSpaCy for the full pack).
- **Bootstrap carries the password floor** (`minPassword`); the two screens read it.

**Consequences.**
- The map in `errors.ts` is a list of the server's learner-facing sentences. A new handler
  message either gets a key on the server (preferred, as the tutor does) or a row here.
- `make test` needs a C toolchain for `-race`; `GO_TEST_FLAGS=-count=1` is the documented way
  round it.
- Offline stays a promise in the docs until its own ADR lands with the code.

---

## ADR-039 — The card says what it takes for a word to stop coming back

**Status:** amended by [ADR-070](#adr-070) (progress number) · accepted · 2026-09-04 *(user report)* · see [R14](research/14-motivation.md),
[R16 §3](research/16-debunked.md), [R18](research/18-session-shape.md),
[ADR-019](#adr-019), [ADR-021](#adr-021), [ADR-025](#adr-025), [ADR-026](#adr-026)

**Context.** The review card carries a memory row: a stage badge, the last outcomes as ticks
and crosses, and «эта карточка: верно 3 из 11». Every figure on it is measured (ADR-026), and
none of it answers the question the first learner actually asked — «не сильно понятно, что
надо сделать, чтоб слово закончить учить… а что делать, чтоб оно исчезло? Прогресс слова,
будет он в плюс или минус или как оно считается — всё не ясно.»

Read cold, «верно 0 из 9» over eight crosses looks exactly like a score being lost, and a
stage called «учу» looks like a level to leave. Neither is true: the tally is the review log,
the stage is FSRS's own state, and nothing on the row is a threshold. The app was being
honest and silent at the same time — which, for a learner deciding whether the last ten
minutes were progress, is the same as being unclear.

**Options considered.**

| Option | Verdict |
|---|---|
| A progress bar per word — «7 of 10 to learned» | Rejected — there is no such denominator. FSRS has no «N correct» threshold, and inventing one would be a score with a target, which is exactly what [R14] and [R16 §3] rule out |
| Say it once, in onboarding | Rejected — the question arrives on card 200, not card 1, and nobody re-reads a tour |
| Put it in the docs and the stage tooltip | Rejected — it IS in the stage tooltip, and the tooltip is why the learner asked. UX.md's own rule: a label that matters is not in a `title=` |
| **A disclosure on the memory row, closed by default, remembered per device** | **Chosen** |

**Decision.** «что это значит?» sits at the end of the memory row and opens five sentences,
in `MemoryMeter.vue`, all of them things the app can stand behind:

1. **The stage and what it means** — the badge's own `stageHint`, which until now only
   existed as a `title=`.
2. **What happens next**, from the scheduler's own preview: a right answer takes the card out
   of today's queue and brings it back in ~X; a miss brings it back today in ~Y and shortens
   what follows. The same figures the grade buttons carry, labelled approximate for the same
   reason (fuzz is seeded at answer time). Shown only while the card is still asking — after
   the answer the row beside it prints where the interval actually went — and replaced in
   free practice by the sentence that matters there: **this schedules nothing** (ADR-019).
   Never inferred from a missing preview, which would report a server hiccup to the learner
   as «this changes nothing».
3. **A card never ends.** It returns ever more rarely — minutes, days, weeks, months, years —
   and that is what «выучено» means here. There is no state after which the word disappears,
   and the app says so rather than letting the learner wait for one.
4. **The tally is a log, not points.** Nothing accumulates and nothing burns; the next
   interval is worked out from the whole history by the model, and there is no «N in a row».
   The figures are quoted only once there are any — a first sight of a card reads «впервые»,
   not «верно 0 из 0».
5. **The one control that does remove a word: «В корзину»** (`t`) — the whole word, every
   direction, and not a delete (ADR-025). This is the honest answer to «что делать, чтоб оно
   исчезло», and it was previously discoverable only as a key on the strip.

Closed by default, remembered in `localStorage` like the set size and the sound: a per-device
whim, never a setting, and the server never hears of it.

**Consequences.**
- The panel adds no number, no target and no bar. It is the same class of object as the rest
  of the row: measured facts and their plain-language meaning. [R14]'s line is where it was.
- `review.memory.help.*` in both locales; the copy names «В корзину» and the `t` key, so a
  change to either must change this text too.
- The stage hints now appear on screen, not only in a tooltip, and are written to be read
  that way.

---

## ADR-040 — Export is back, as a full dump of everything

**Status:** accepted · 2026-09-22 *(user request)* · **supersedes in part**
[ADR-011](#adr-011) · see [R16 §4](research/16-debunked.md), [ADR-003](#adr-003)

**Context.** ADR-011 removed `internal/export` and said the app has no export. Its reasoning
was sound and is unchanged: an `.apkg` carries one card with one interval, so the
morphological state space — the whole thesis — cannot cross it, and handing a learner a
vocabulary list while calling it their progress is worse than handing them nothing.

What ADR-011 also said, in the same breath, is what this builds: *"If it is ever paid, the
honest shape is a full JSON dump of both state spaces plus the review log — not an `.apkg`."*
The learner asked for exactly that, in order to self-host: «должен иметь возможность сделать
фулл экспорт своего профиля со всеми словами, опытом (базой)… статистика и вообще все все
должно экспортится тоже».

The data directory answer ADR-011 offered — `tar` the volume — is true and is not enough. It
moves an installation, not a profile: it cannot take one learner out of a multi-user database,
and it asks someone to stop a container and copy a file to change machines.

**Options considered.**

| Option | Verdict |
|---|---|
| Keep ADR-011: the data directory is the answer | Rejected — it moves an installation, not a profile, and cannot separate one learner from a shared database |
| `.apkg` again | Rejected, for ADR-011's reasons, unchanged |
| A hand-maintained JSON schema with an explicit column list per table | Rejected — a migration that adds a column and forgets this file makes exports silently incomplete, which is precisely the failure the format exists to avoid |
| **A full dump with columns DISCOVERED via `PRAGMA table_info`, import replaces** | **Chosen** |

**Decision.** `internal/profile` + `GET /api/profile/export` + `POST /api/profile/import`.

- **Columns are discovered, never listed.** A new column rides along the day it is added; an
  export written by an older binary still imports, its missing columns taking schema defaults.
- **A table manifest with a coverage test.** `TestManifestCoversEveryUserTable` fails when a
  migration adds a table that is neither exported nor excluded with a stated reason. That test
  is the only thing standing between "we added a table" and "exports quietly stopped being
  complete".
- **Surrogate ids are remapped, not preserved.** `note.id` is global; on the destination
  another learner may hold id 5. Every rowid alias is re-assigned and the four referenced
  tables carry an old→new map. `review_log.id` taught this: referenced by nothing, so it
  carried no remap, so it collided the moment the destination had review logs of its own.
- **Import REPLACES, in one transaction.** Merging two FSRS histories means deciding which
  `review_log` wins for a card on both sides, and there is no answer that does not invent a
  learner's past. The confirm names by count what will be erased.
- **Secrets do not travel.** `settings` rows with `secret = 1` are omitted, and both the export
  screen and the import report say so. "Secrets never leave the server" stands; a file is how
  they would.
- **The integrity check is the replay property.** Every card's log is replayed through
  `internal/srs/replay.go` and compared to the state that arrived with it. Same architecture
  and a mismatch is corruption: fail and roll back. A different `goarch` and a mismatch is
  expected — go-fsrs's fuzz is floating point and Go fuses multiply-adds on arm64 only — so it
  is a warning that names the reason. No interchange format could make this check.

**Consequences.**
- CLAUDE.md's honesty rule «The app has no export» is rewritten rather than deleted: what the
  app has is stated precisely, including what it leaves out.
- ADR-011's `.apkg` refusal is untouched. This is not that format and never becomes it.
- The import blocks the app while it runs (`db.Open` sets `SetMaxOpenConns(1)`), and the UI
  says so rather than appearing to hang.
- Round-trip verified across two installs: every card's stability, difficulty, reps, state and
  fuzzed due date identical, `morph_state` included, and the exported password working on the
  destination.

---

## ADR-041 — «В корзину» becomes «Знаю», and marks the word

**Status:** accepted · 2026-09-22 *(user request)* · **amends** [ADR-025](#adr-025)

**Context.** ADR-025 built a trash: a word set aside, with its note, cards, FSRS state and
every `review_log` row intact — which is what makes restore return the card overdue and keeps
undo working across it. The machinery is right. The name was not: the learner's actual use for
it is «слова должны помечаться как уже выученные», and «В корзину» says the opposite of that.

Worse, binning the cards was only half of what they meant. A word they already know is a fact
about the WORD: the reader went on highlighting it as unfamiliar in its n+1 scoring and the plan
went on counting it as missing, while the learner had just said they know it.

**Decision.** One button, «Знаю», over ADR-025's machinery unchanged. It additionally sets
`lemma_state.status = 'known'`; restore reverts it to `learning`, the state a word has from
being added at all — not `new`, which would claim it had never been seen.

**Consequences.**
- `POST /api/notes/:id/know` is the name the client uses; `/trash` and `/restore` are the same
  handlers under the names they shipped with, so a stale tab and the stored tests keep working.
  The `trash` SCOPE value stays on the wire — it is ADR-025's contract and renaming it would
  break every stored reference.
- The screen is «Знаю», not «Корзина», and its copy states the second half: the reader stops
  highlighting the word.
- A note whose lemma the pack cannot resolve is still set aside; it simply has no word-level
  fact to record, and failing the request over that would be saying less, not more.
- Everything ADR-025 guarantees is still guaranteed: not a delete, restore returns the card
  overdue, undo replays across it, `all == course + reader` still excludes it.

---

## ADR-042 — The model may write a dictionary entry, and may not write material

**Status:** accepted · 2026-09-22 *(user request)* · see
[R10](research/10-llm-tutors.md), [ADR-014](#adr-014), [ADR-027](#adr-027),
[ADR-035](#adr-035)

**Context.** The pack is finite and the reader is not. A learner who clicked `bejelentkezve` in
a pasted text got «В языковом пакете этого нет.» and nothing else — no meaning, no breakdown, no
way on, on precisely the word they stopped for. They asked for the model to fill it: «должна
быть возможность добавлять слова в словарь с помощью нейронки… должно добавлять такое же
описание как на nincs скриншоте».

The tension is real. [R10] is why every guardrail in this app exists: Hungarian is where models
are weakest, and a confident wrong entry is worse than a dead end because the learner has no way
to check it.

**Options considered.**

| Option | Verdict |
|---|---|
| Let the tutor do it | Rejected — the tutor mints nothing, and a test counts `note`/`card`/`user_gloss` and both state spaces before and after a question. Routing this through it would delete that guarantee |
| Generate the entry AND the example sentences as material | Rejected — a card's Hungarian must be attested, `internal/corpus` does not exist, and this is exactly the rule Phase 9 exists to enforce |
| Free-text prose from the model, rendered as-is | Rejected — unparseable output cannot be checked word by word, and the check is the whole defence |
| **A separate handler, JSON in, displayed and labelled, saved only in two explicit steps** | **Chosen** |

**Decision.** `POST /api/words/:form/entry` in `handlers_ai_entry.go` — deliberately NOT a
tutor route. Prompt: `.claude/prompts/word-entry.md`.

- **Offered only where the pack has nothing.** The pack's own entry is richer than anything a
  model should be trusted to produce; this fills a dead end rather than competing.
- **Every Hungarian token is looked up**, through the same `tutor.Backticked` /
  `tutor.Unattested` / `wordExists` path the tutor's answers use, and the misses are listed
  under the entry. Tokens the model forgot to backtick are checked too — otherwise a model that
  drops the backticks silently turns the guardrail off, which is the one failure a learner could
  not possibly notice. The asked-about form is excluded: the learner knows the pack lacks it.
- **Two explicit steps to a card.** Saving stores `user_gloss` as `source='machine'` with the
  model; only «Принять как своё» re-saves it as `manual`, and only a `manual` gloss is a card's
  back. ADR-014 and ADR-035 unchanged.
- **The examples are illustration and say so.** Not attested, never notes, and the panel states
  it where they are rather than in a doc.
- **The model's own "I don't know" is passed through.** An honest dead end beats an invention.

**Consequences.**
- A second, smaller thing rides with it: a chat about one WORD, one per form per local day
  (`tutor_thread.word`, migration 00021), so «ещё примеры» is answerable in the moment. It uses
  the existing conversation component, so the model label, the grounding line and the honesty
  notice come for free rather than in a second copy.
- `handlers_tutor_test.go`'s mints-nothing test still passes, because this is not that path.
- A small model's sloppy JSON is read leniently — fences stripped, outermost object taken — and
  what still cannot be read comes back as prose, labelled, rather than being discarded.

---

## ADR-043 — Grammar lands on the lesson the first time and on the exercise after that

**Status:** accepted · 2026-09-22 *(user report)* · see [R4](research/04-explicit-instruction.md),
[R5](research/05-interleaving.md)

**Context.** The learner's report: «Упражнения в грамматике прям вообще внизу, аи репетитор
вверху, еще выше описание грамматики. Ну бред же. Должно быть что то типо как повторение слов.
Типо главная таска.»

They are right about the page. The exercise is the task and it was the third card down, reached
by scrolling past the whole lesson and the tutor panel.

They are also asking for something [R4] forbids. «Grammar teaches before it tests» is the
strongest finding in the research set, and a screen that leads with the exercise contradicts it.

**Decision.** The landing state is conditional, not flipped.

- First visit to a topic → the **lesson**, expanded, no exercise on screen. [R4] holds exactly.
- Once the learner has pressed «К упражнениям» for that topic, or the topic has ANSWERED
  drills → the **exercise** leads and the lesson is one disclosure away.

**The trap this avoids.** The obvious signal — "phase is past `explained`" — is wrong.
`drill/schedule.go` says *"a newly opened topic always starts blocked"*: `blocked` is the
DEFAULT for anything drillable ([R5]'s initial-blocked-practice finding) and says nothing about
whether the learner has read anything. Using it would have put an exercise in front of every
first-time topic — the [R4] violation the whole arrangement exists to avoid. The real evidence
is `topic_progress.attempts > 0`, a lifetime counter the EMAs deliberately do not forget, plus a
per-device record of the button press to cover the gap before the first answer.

**Consequences.**
- «Все темы» moves out of the lesson's `CardHeader`, where it was a ghost button and the only
  route back to the topic map in the whole SPA, into the page furniture as an outline button.
- The tutor becomes a side column toggled by a button, as the review card's is — not a step
  between reading and practising that everyone walked through.
- The `?topic=` / `?why=1` URL contract is unchanged and covered.
- docs/PEDAGOGY.md records this as the seventh documented divergence-that-is-not: the finding is
  honoured, the layout around it changed.

---

## ADR-044 — Reading counts toward the day, and the day says where it went

**Status:** accepted · 2026-09-22 *(user report)* · see [R14](research/14-motivation.md),
[ADR-021](#adr-021)

**Context.** «это время также должно учитывать чтение, слушание, и так далее.» The reader was
the one loop that reported NOTHING: an hour with a text moved no counter, so «21 / 90 мин» sat
still. That is not a display bug — `daily_log.active_ms` is one half of what the streak requires
(`stats.Day.Met`), so reading was literally not counted as study.

And the figure it feeds is a sum, which is the one thing a learner cannot act on: «21 минута»
does not say whether the reading counted.

**Decision.** Reading reports time, on the same terms listening does, and the day is drawn split
by what it was spent on.

- `POST /api/reading/progress`, clamped to 60 s per report — the same bound listening has.
- A second counting **only** while the tab is visible AND the learner interacted within the idle
  window. A wall clock would be dishonest: a tab left open overnight is not ten hours of
  Hungarian, and [R14] is explicit that a streak kept that way measures nothing.
- `daily_log.read_s` beside `listen_s` (migration 00020), because the total alone cannot say
  where an hour went.
- **The streak's definition is unchanged.** Queue cleared AND minutes met, both halves, as
  before. What changed is that the minutes now include work that was always work.

**Consequences.**
- Both halves exist on purpose: the client is honest about what it reports, and the server's
  clamp means it does not have to be trusted. A stuck timer mints a minute, not a night.
- The activity chart draws three measured series — cards, reading, listening. `due_left` is NOT
  a line on it: cards-remaining and minutes-spent are different scales, and one axis for both
  invites the comparison that is meaningless. The queue half of the obligation is a status row.
- Where a day failed, the tooltip and the streak popover name WHICH half failed. They fail
  independently, and "not met" on a day whose queue was clear pointed at the wrong thing.
- The chart's three series use their own validated categorical steps, not the knowledge-state
  colours: `primary` measures chroma 0.072 and reads grey as a fill, and `kn-known` is doing
  status duty here. Light and dark each pass the categorical checks against their own surface.

---

## ADR-045 — Chats are cleared by the tabful, and the context meter describes the answer on screen

**Status:** accepted · 2026-09-22 *(user report)* · **amends** [ADR-031](#adr-031) ·
see [ADR-034](#adr-034)

**Context.** Two reports about the tutor page.

«у эай учителя должна быть возможность также удалять чаты… Также группы повторения там должны
быть отдельной вкладкой чтоб не миксовать.» A review chat is one per word per local day
(ADR-034), so a month of reviewing leaves hundreds of «Повторение · jelenteni» rows on top of
every real conversation, and `DELETE /api/tutor/:id` was the only way out — one at a time.

«Окно модели 8 192 токенов… Показывается только первый раз, потом уже не увеличивается и ничего
не меняется если я начну чатиться.» The meter walked backwards through the thread to the last
answer that carried figures, on the reasoning that a stopped stream reports none and an old
reading beats «заняли 0». The cost was worse than the problem: once any answer came back without
usage, the panel kept showing the FIRST answer's numbers and looked frozen — and a learner
watching a figure sit still through three more questions cannot tell stale from stuck.

**Decision.**

- **Tabs, by kind.** «Чаты» / «Повторение» / «Слова», with per-kind counts, because a tab has to
  carry the number on the tab you are not looking at. `chat` is the wire name for the ordinary
  kind, whose stored value is `''` and which a query string cannot carry.
- **`DELETE /api/tutor?kind=`** clears one tab, or everything when the kind is absent. Confirmed
  by count first. `DeleteThreads(user, "")` (every kind) and `DeleteThreadsOfChatKind` (the `''`
  rows) are separate methods on purpose: confusing them would wipe a learner's whole tutor
  history while meaning to clear one tab. An unrecognised kind is refused rather than silently
  meaning "all".
- **The meter reads the LATEST answer only.** When that answer reported nothing it says so —
  «Расход на последний ответ модель не сообщила» — and the bar is absent. A stale reading is
  indistinguishable from a stuck one, so it is never shown.
- **The server logs partial usage**, naming which of the three figures went missing: the
  provider's usage block, or Ollama's `/api/ps` window. When the meter looked frozen there was
  no way to tell.

**Consequences.**
- `GET /api/tutor` returns `{threads, counts}` rather than a bare array. The stored tests
  asserted the old shape and were updated with it.
- The window-full warning no longer fires on a zeroed reading, which would have claimed the
  prompt was cut when nothing was known about it.
- ADR-031's meter stands; what changed is that it now describes the answer it is under.

---

## ADR-046 — Asking the tutor never changes a word's schedule

**Status:** accepted · 2026-09-22 *(user decision)* · **supersedes in part**
[ADR-034](#adr-034) (a question before the answer is a hint) · see
[R4](research/04-explicit-instruction.md), [R9](research/09-corrective-feedback.md),
[R14](research/14-motivation.md), [ADR-017](#adr-017), [ADR-042](#adr-042)

**Context.** ADR-034 priced the tutor: a question sent before the answer set `usedHint`, the
same channel «Подсказка» uses, so `srs.DeriveRating` capped the grade at Hard and the word came
back sooner than the learner's own answer warranted. The learner's verdict: «аи учитель не
должен мешать прогрессу слова никогда! типо удлинять его запоминание!»

They are right, and the reasoning ADR-034 gave does not survive the panel being used. The cap
was defensive: a model with the expected form in its prompt would be a hint that bypasses
grading. But the prompt **withholds the answer** in that phase and `tutor.Allowed` refuses the
templates that would name it — that is the guardrail, and it is a real one. What the cap
actually taxed was reading an explanation before answering, which is [R4]'s explicit
instruction and [R9]'s prompted retrieval: the two things the panel exists for. A price on
asking teaches the learner not to ask, and a grade that was lowered by something other than
their answer is the kind of invented number [R14] rules out everywhere else in this app.

**Options considered.**

| Option | Verdict |
|---|---|
| Keep the cap | Rejected — it taxes the behaviour the panel exists to make cheap, and the learner asked for it to stop |
| Cap only the answer-naming chips | Rejected — those chips do not exist in the question phase: `tutor.Allowed` already refuses them. There is nothing left to price |
| Record the question and show it on the card without capping | Rejected — a mark that changes nothing is decoration, and one more thing on a card that already carries five labels |
| **No channel at all from the tutor to the graded answer** | **Chosen** |

**Decision.**
- **The tutor does not reach the answer.** `consultTutor` and the `asked` event that fed it are
  **deleted** — from the store, from `ReviewView`, from `TutorAside` and from
  `TutorConversation`. `usedHint` is set by «Подсказка» and by nothing else, so a later change
  cannot quietly reconnect a wire that is not there.
- **The guardrail is unchanged and is now what the panel says.** Before the answer the server
  still withholds it — the typed card's form, the produce card's word and sentence, the
  receptive card's meaning, the audio card's Hungarian — and `tutor.Allowed` still refuses
  `whyForm`, `breakdown`, `translateWord` and `spelling` in that phase with 400.
- **The copy states the new fact**, where the old one stated the price: «До ответа учитель не
  получает правильный ответ и не называет его. На расписание слова вопрос не влияет: карточка
  всё равно спросит вас, и оценка будет ваша.»
- **«Подсказка» is untouched.** It reveals the graded string, so it caps at Hard, and «Хорошо» /
  «Легко» still say «засчитается как «Трудно»».

**Consequences.**
- One fewer reason not to ask, on the screen where [R4] says asking is worth the most.
- **Cost:** a small model may name the form anyway, and that answer is now unpriced. It was
  already only a cap rather than a refusal, a learner with a grammar beside the keyboard has
  always been able to do the same, and the honest position is that this app grades what you
  typed and does not police how you arrived at it.
- The review path still reads no tutor table, and `handlers_tutor_test.go` still counts the five
  tables before and after a question. The separation ADR-034 built survives; what is gone is the
  one wire that ran the other way.

---

## ADR-047 — The estimated level and text coverage are competence signals

**Status:** accepted · 2026-11-08 *(polish pass; owner-facing, reversible in code)* · see
[R6](research/06-lexical-coverage.md), [R11](research/11-extensive-reading.md),
[R14](research/14-motivation.md), [R16 §3](research/16-debunked.md)

**Context.** The app had no answer to "what level am I?" or "how much of real text do I
understand?" — only a lemma count and checkpoints. The motivational surface was limited to
measured competence signals (CLAUDE.md), and an invented "score" would be exactly what R14 and
R16 §3 rule out.

**Options considered.**

| Option | Verdict |
|---|---|
| A points/score number that rises with activity | Rejected — extrinsic, cumulative, measures effort not competence [R14] |
| Stored level that only advances | Rejected — a number that cannot go down is the score R16 §3 describes |
| A CEFR label from an in-app test | Rejected — we cannot verify an exam; claiming one is an honesty failure |
| **Per-request estimate from the deck, with its method printed** | **Chosen** |

**Decision.**
- `stats.EstimateLevel` (pure) computes a label («A2.2»), per-band shares and a **text coverage**
  (Σ`freq_pm` of counted lemmas ÷ Σ`freq_pm` of ranked pack lemmas). A lemma counts when
  `known`, or graduated (state Review) with retrievability ≥ `mining.knownThreshold`. Learning-
  step words do not count, so both figures can go down.
- A band is reached at share ≥ 0.80 AND half its drillable topics interleaved/mastered AND not
  blocked by a failed checkpoint. Sublevel splits the working band's progress in thirds; vocab
  done but grammar missing stays at `.3`. Confidence is `low` under 100 counted lemmas, else
  `estimate` — never "high".
- Every surface shows the fixed method sentence (`level.method.v1`): estimate from vocabulary and
  grammar; bands come from word frequency, not an exam; may be off by a sublevel or two. Never
  named "score" or "points". Computed per request, not stored; band sizes cached per boot.
- It joins the permitted motivational surface (CLAUDE.md).

**Consequences.**
- A learner can see their level fall after a lapse — by design, and the copy says so.
- **Cost:** the CEFR bands are a frequency estimate and the 0.80 share is our threshold, not a
  literature figure; Hungarian lemma coverage overstates comprehension [R6]. Hence "estimate".
- Per-request cost: curriculum resolution (read-only — the endpoint never writes `topic_progress`)
  plus one pass over the learner's Review-state cards and chunked pack lookups; the counted set
  is cached per user for 30 s and dropped when a card is answered.
- Suspended cards (`c.suspended = 0` filter) and trashed ones do not make a lemma count; only a
  `known` declaration or an active graduated card does.

---

## ADR-048 — "Next" follows enrolment, and a roadmap replaces the Plan screen

**Status:** accepted · 2026-11-08 *(polish pass)* · supersedes in part the `current`-unit rule of
the course screen · see [ADR-013](#adr-013), [R11](research/11-extensive-reading.md)

**Context.** `course.Resolve` called the first non-done unit `current`. Words added from the
reader complete units by being in the deck, so mining a word could finish unit 2 and make "next"
jump away from unit 1, which the learner had started. The owner saw "a new word from a text
switches topics".

**Decision.**
- `plan.current` (JSON name kept for compatibility; learner-facing «следующий») = the first unit
  that is enrolled and not done; else the first unit neither enrolled nor done. `done` stays
  derived from the deck (no stored completion flag); reader words still make a unit done.
- `inPlaySet` returns its errors and chunks its `IN` clause — it used to turn a pack error into an
  empty set, which reset every unit.
- **Decision recorded now, UI later:** a roadmap on Today replaces the Plan screen (unit nodes by
  month, checkpoint milestones, states пройден · начат · следующий · впереди; a unit finished
  mostly through reading says so; reader words get their own card, never nodes; nothing locks per
  [ADR-013](#adr-013); `/plan` redirects to `/`). No frontend change ships in this ADR's batch.

**Consequences.**
- Nothing about the order of units becomes a lock; it is still advice.
- A learner who enrols on a late unit is "next" there until they finish it, then returns to the
  first untouched unit.

## ADR-049 — The full pack can be installed from Settings, from a pinned checksum, and activates on restart

**Status:** accepted · 2026-11-08 *(polish pass)* · amends [ADR-010](#adr-010) (which refused a
download step) · see [ADR-002](#adr-002)

**Context.** ADR-010 put a starter pack in the binary so the app is never a build step wearing a UI;
the full pack stayed "build it yourself with `make langpack`". Owners asked to upgrade without a
toolchain. The refusal in ADR-010 was of a download the app *depends on*; an opt-in upgrade whose
absence changes nothing is a different thing, provided it cannot corrupt what exists.

**Decision.**
- An admin can install the full pack from Settings. `internal/lang.Installer` runs one job at a time:
  stream `langpack.url` to `<data>/langpack-hu-v1.sqlite.part`, SHA-256 over the **compressed**
  stream against `langpack.sha256`, gunzip (capped at 2 GiB) to a second temp file, open read-only,
  check schema and tables, run the id-compatibility check, then **park** it atomically as
  `langpack-hu-v1.sqlite.new` (+ a small `.new.json` marker: sha256, time). The live file is never
  replaced inside a running process: `database/sql` opens new pool connections by path, so a rename
  would make the running app silently read the new pack.
- **No pinned checksum, no download**: empty `sha256` (the default until a pack is published) answers
  `errors.packNotPublished` before any request. The URL is config, https only (http for loopback),
  admin-edited through `updateConfig`, and never taken from an install request.
- **Id-compatibility is the safety net**: every `lemma_id` (`lemma_state`, `note`, `text_token`) and
  `sentence_id` (`note`) in the learner database must map to the same `(lemma, upos)` / Hungarian text
  in the new pack as in the loaded one; otherwise `errors.packIncompatible` and nothing changes.
  `lemma.id` is sequential in Wiktionary-dump order, so a pack built from another dump would
  silently re-point every note. A test walks the learner schema so a new pointer column cannot be
  missed.
- **Activation is a restart, applied at boot.** `lang.ApplyPending` runs in `Server.loadPack` before
  anything opens the pack: it deletes stale `.part`/`.unpacked` files, re-opens the parked pack,
  re-runs the id check against the *current* live pack and the user DB as they are then (notes may
  have been added since the install), and renames it over the live file; if the check fails the
  parked file is deleted, the live pack stays, and `GET /api/admin/pack/install` reports
  `failed` + `errors.packIncompatible` for that process. `POST /api/admin/restart` sends the process SIGTERM (the existing graceful
  shutdown). The image sets `TANULOM_SUPERVISED=1` and is meant to run under a restart policy; when
  unset the endpoint still stops gracefully and returns `supervised:false` so the UI says "start it
  again yourself".
- The published pack is built with `--subtitle-sentences 0` (`make langpack-release`) because the
  OpenSubtitles licence does not cover redistribution.

**Ids, verified by construction.** `starter.py` copies `lemma`, `sentence`, `wordform` etc. with
`INSERT ... SELECT * FROM full.<table> WHERE id IN (keep)`, so every starter lemma/sentence id is
the full pack's id: the starter is a strict subset. A full pack built from the same Wiktionary
dump therefore agrees with the starter; one built from a different dump may not, and the id check
(at install and again at boot) refuses it. No automated test builds a tiny full→starter pair
(starter.py needs the curated course/curriculum tables); that is not covered.

**Accepted as-is.** Redirects to private address ranges are allowed: the URL is admin-config and the
body must match the pinned sha256. Logged URLs are credential-redacted.

**Rejected.** Mounting the Docker socket to restart/start sidecars (host-root for a web app, in a
distroless image); hot-swapping the pack (every cached analyser, DAG and inventory hangs off it);
trusting a checksum served beside the file (a compromised host serves both).

**Consequences.** A failed or interrupted install never changes the app: temp files are removed, the
live pack is untouched, a full disk is its own message. Until a release asset exists and its digest
is pinned, the button honestly says the pack has not been published.

**Amendment — the pin lives in the binary (2026-11-10).** Typing a URL and a SHA into Settings
before one click was a chore the app can do itself.
- *Committed pack (changed 2026-11-10; was a data release).* The pack file is committed at
  `langpack/release/langpack-hu-v1.sqlite.gz` (about 62 MB, no Git LFS, so raw URLs work) and served from
  `https://raw.githubusercontent.com/antlko/learn-hungarian-ai/main/langpack/release/langpack-hu-v1.sqlite.gz`;
  the SHA-256 pinned in the binary is what makes a moved `main` safe. Cost: the blob is in history for
  good, so a new pack version means another ~60 MB commit — change the pack rarely. Release assets
  were dropped because publishing one needs a separate manual upload.
  `releases/latest/download/…` was **rejected**: `latest` follows the newest *app* release, so cutting
  `v1.0.1` (which carries no pack asset) would 404 every install. A pack version is a different
  thing from an app version.
- *Binary-pinned default, resolved at point of use.* `config.DefaultLangPackURL`,
  `DefaultLangPackSHA256` (pinned: the Tatoeba-only pack, sha256 584c4ca9…c4acd, 65,535,311 bytes) and `DefaultLangPackSizeBytes` (button label only). **Effective pin**
  (`config.EffectiveLangPack`): checksum = stored `langpack.sha256` if non-empty, else the const; URL =
  the const if the stored URL is empty or is the old `releases/latest/download/…` default, else the
  stored one (a stored value equal to the current default also counts as unset). `normalize()` does NOT fill `langpack.url`, empty validates and means "the binary's pin", so nothing is ever written to YAML, so
  existing volumes need no edit and a new release's pin takes effect without touching them. A stored
  value is an explicit override (a mirror, a self-built pack) and wins.
- *Still refuses unpinned.* Empty effective checksum → `errors.packNotPublished`, no request made.
  `GET /api/admin/pack/install` adds `published` (effective checksum non-empty), `sizeBytes`, `url`,
  so the UI can offer one button when published and the old explanation when not.

## ADR-050 — The course grows; themed side units are optional and never decide the path

**Context.** The guided course had 24 units and 853 words. Month files carried 30-40 hand-argued
words a unit, and a learner who finished them still could not name the colours, the body, the
months or the weather: vocabulary that belongs to a *family*, not to one errand. Padding the
errand units with those words would blur what each unit buys; adding a twenty-fifth unit would
turn the course into the endless drip [R11] warns against.

**Decision.**
1. The twenty-four core units stay twenty-four (`course.CORE_UNITS`; `validate` refuses any
   other count) and **grow to about 1 400 words** (15-60 per unit, the existing validator
   bounds). The growth lives in `langpack/pipeline/course_growth.py`, appended after each
   unit's original words, chosen from high-frequency lemmas ([R6](research/06-lexical-coverage.md))
   that fit the unit's errand.
2. **Twelve themed side units** (numbers, the clock, days and months, colours, the body, weather,
   food, clothes, home, travel, moods, hobbies; ~45 words each) live in `course_themes.py` with
   `course_unit.track = 'theme'` (default `'core'`). They sort after month 6 (`ord` 701-712,
   `month` 6) and are *optional, finite and ordered*.
3. **Only the core path decides anything.** `Syllabus.Resolve` skips theme units when choosing
   `current` and excludes them from `total`, `completed` and `handedOver`; they still get their own
   derived `done`/`ahead` status and `enrolled` flag. Completion stays derived from the deck, so
   no stored flag exists to disagree with it.
4. A word is taught in exactly one unit, core winning over theme (`_dedup`; a test fails if a new
   word would be dropped silently). Every word must exist in the pack with the stated part of
   speech, have hand-written ru + en glosses, and **at least 90% of words must have a real
   sentence in the starter pack** (`course.unclozeable`, `tests/test_course.py`). No sentence is
   written for the course: Hungarian sentences stay ones a person wrote.

**Rejected.** More core units (the course would no longer end); a stored `completed` flag on
theme units (ADR-048's rule); letting theme enrolment move `current` (a learner who took the
colours would be told month 2 was "next" no longer); machine-generated glosses or sentences
(unattested material is labelled, not shipped as the course).

**Consequences.**
- `/api/course` units carry `track` (`core` | `theme`); `plan.total` is still 24.
- A pack built before this column reads every unit as core (`Load` falls back); `make refresh`
  adds the column to an older pack (`ALTER TABLE`) because the app opens packs read-only.
- Course glosses are promoted over the dictionary's at full build (`apply_course_glosses`), not by
  `refresh`; after `make refresh` the new words' course glosses are the ones on cards, the
  dictionary view still shows the imported first sense.
- The starter grows by about 2 MB gzipped.

## ADR-051 — A miss on a new card keeps FSRS's learning steps; the app explains them instead

**Status:** accepted · 2026-11-08 *(polish pass)* · see [ADR-003](#adr-003), [ADR-020](#adr-020),
[ADR-039](#adr-039), [R14](research/14-motivation.md)

**Context.** A learner answering «Снова» on a new card sees it come back in about a minute, and
again in ten, and read it as the app demanding "no mistake in a row" before a word may leave. That
feels punishing and made the owner doubt the method. FSRS has no "N correct in a row" rule: the
`{1, 10}` minute steps are go-fsrs's learning steps, after which the card graduates to a
stability-based interval of days.

**Options considered.**

| Option | Verdict |
|---|---|
| Shorten or drop learning steps, or graduate a new card on its first correct answer | Rejected — changes scheduling maths ([ADR-003](#adr-003)) and, worse, changes how stored state replays |
| Hide the steps from the card | Rejected — the card then answers "why is this back?" with silence |
| **Keep the steps unchanged; explain them** | **Chosen** |

**Decision.** FSRS parameters and learning steps are unchanged. Undo is delete-and-replay over
`review_log`, and profile import verifies by replaying every card's log against its stored state;
both depend on the scheduler reproducing history bit for bit ([ADR-020](#adr-020)). Any change to the
steps would make every stored log replay to a different state. The interface explains instead: the
«Как это работает» dialog on the review start screen and card says what the steps are, that a
graduated card is scheduled in days, that there is no threshold of consecutive correct answers,
and that a miss is listed with its answer and returns after its step. The card's step ladder
([ADR-039](#adr-039)) and the missed list ([ADR-026](#adr-026)) already show the position; the
dialog names it.

**Consequences.**
- No migration, no replay change, nothing to reconcile at boot.
- A learner who still finds a one-minute return too harsh has no setting for it; if the first
  real learners need one, that is a new ADR with its own replay story, not a tweak here.
- The explanation is copy, so it is pinned only by the locale parity check, not by a test.

## ADR-052 — Settings → Components: one place for the pack and the sidecars, with no Docker access

**Status:** accepted · 2026-11-08 *(polish pass)* · builds on [ADR-049](#adr-049) and
[ADR-015](#adr-015); the nlp half is [ADR-054](#adr-054)

**Context.** The owner could not tell what the translator (MT) and the analyser (HuSpaCy) were,
whether the Makefile's long pack build mattered, or why the settings rows said "not
implemented". Services were a list of endpoint fields in Settings → Services, which says nothing
about what is missing or what it costs.

**Options considered.**

| Option | Verdict |
|---|---|
| Mount the Docker socket so the app can start and stop sidecars | Rejected — a web app with the socket is root on the host, in a distroless image |
| Leave Services as endpoint fields and explain in the README only | Rejected — the reason to turn a component on must be on the screen where it is turned on |
| **An admin Components screen: what it is, what you lose without it, state, progress, the command** | **Chosen** |

**Decision.** `GET /api/admin/components` returns one entry per component (`pack`, `mt`, `nlp`,
`tts`, `asr`, `llm`) with a state (`off`, `device`, `unreachable`, `ready`, plus `loading`,
`downloading` and progress where the sidecar reports it), a documented RAM hint (never presented as
measured), the compose profile and whether this repository builds it at all. The screen
(`ComponentsPanel`) shows each with its cost in words and a state chip that carries text, not
colour alone. The app can do what needs no host access: install the full pack
([ADR-049](#adr-049)), ask a running sidecar to fetch its model (`mt` and `nlp` warm-up), and
restart itself (`POST /api/admin/restart`). **Starting a container stays the operator's command**,
printed on the card. A sidecar that does not exist (`tts`, `asr`) is labelled «не создан», never
shown as "coming soon" with an enabled switch.

**Consequences.**
- Everything here is admin-only and enforced by the API, not only by the route guard.
- Restart works only under a restart policy; the response says `supervised: false` otherwise and
  the screen says to start the container by hand.
- The full-pack figures shown on the card live in `lib/packFacts.ts` and are labelled «по
  документации»; they can lag the pack (the starter was rebuilt with more sentences after they were
  written). Counting them at runtime from the loaded pack would remove the drift.

## ADR-053 — Grammar topics use the roadmap node vocabulary; plain phase names; gate numbers shown

**Context.** The Grammar screen read as arbitrary and opaque: a grid of cards, a duplicated rail
on the topic page, and phase names («blocked», «interleaved») that are the gate's vocabulary, not
a learner's.

**Decision.** Topics are drawn as the Today roadmap draws units — a disc on a rail, one per topic,
grouped by CEFR level, each state with its own shape, icon and word. Phase names are plain:
«только правило» · «ещё не начато» · «тренируем по одному» · «вперемешку» · «освоено». «Ещё не
начато» is display-only (`lib/grammarStage.ts`: `blocked` with zero attempts) — the server's phase
and the gate are unchanged. The topic page has two tabs, Урок | Упражнения; ADR-043's evidence
(`topic_progress.attempts > 0` or the per-device «К упражнениям» press, never the phase) still
chooses which opens first. The exercises tab prints the gate in numbers taken from the server's
`decision` and `gate` (accuracy ≈72%, need 80%, answers 14 of at least 20) with one sentence on
why practice is one pattern at a time first ([R5]); no threshold is restated in the client.

**Consequences.**
- `locked` is shown only for the server's `locked` phase; nothing else is disabled (ADR-013).
- A topic without drills says so calmly and links to the tutor and the reader's texts.
- Accuracy appears as counted answers («верно 17 из 20»), never a bare percentage.
- No backend change: `decision.{current,target,observations,window}` and `gate.*` already existed.

## ADR-054 — The HuSpaCy sidecar is opt-in and only selects among attested analyses (SUPERSEDED by ADR-069)

**Status:** accepted · 2026-11 · *implements step 4 of [ADR-002](#adr-002)'s ladder*

**Context.** ADR-002 left "an `nlp` sidecar disambiguates in context" as a sentence. The pack
already holds several candidate readings for an ambiguous form (`vár` NOUN/VERB); the core picks
the more frequent and flags it `uncertain`. HuSpaCy `hu_core_news_md` 3.8.1 (CC BY-SA 4.0, ~127 MB,
published ~2.4 GB RAM, unmeasured in a container) can read the sentence.

**Decision.** `services/nlp` is an opt-in profile; the model is not baked in but fetched on
`POST /warmup` into a volume. The core calls `/analyze` **once per text ingest** (never per
token), only when the text contains an `uncertain` word, with a total timeout, and passes the
result to `morph.ApplyContextHints`, which may only **re-choose among `Lookup(form)`'s own
candidates** by (lemma, upos) at the overlapping byte span. A hint without a matching candidate
changes nothing; `certain`, `derived`, `unknown`, `corrected` words are never touched. The choice
is exposed as `analysis.via = "context"` (Certainty stays `uncertain`); the UI string is
`reader.viaContext`. Any sidecar error returns today's result untouched.

**Rejected.** Letting the sidecar's lemma/feats stand when the pack has no such reading (a
second source of truth, unattested output becoming cards); per-token calls (latency); making the
sidecar the default (RAM, and the core must be complete without it).

**Consequences.**
- Forms the pack does not know are still `derived`/`unknown`: the sidecar does not analyse them
  for the learner (a deliberate narrowing of the earlier README promise).
- `via` is not persisted in `text_token`; it is present on `/api/analyze` and at ingest (the
  stored lemma is the chosen one).
- Model licence CC BY-SA 4.0 is noted in `services/nlp/README.md`.

---

## ADR-055 — The curriculum grows: questions, numerals, pronouns, postpositions, time and date

**Status:** accepted · 2026-11-08 · **extends** [ADR-050](#adr-050) (the course grows) to the grammar spine

**Context.** The 28-topic DAG taught case, tense and mood but had no home for four things a
learner needs in the first weeks: how to ask and say no, how to count, who `ő` and `ez` are, and
how to say "under the table" and "at half past three". They were scattered as asides (the
singular after a numeral lives in `plural`, the accusative of pronouns in `accusative`) or absent
(question words, `nem`/`sem`, postpositions, the clock, the date). The tutor and the course
(whose theme units already name numbers, the clock, days and months) had nothing in the grammar
to point at. A feature audit also showed `Person=1/2/3` — the verb's person, graded in every
conjugated cloze — taught by no topic (`tests/test_features.py` was red).

**Decision.** Five topics, each with a ≥ 800-character lesson in both locales, every Hungarian
form looked up in the pack before it was allowed to stay:

| code | level | order | prerequisites | features |
|---|---|---|---|---|
| `questions-negation` | A1 | 55 | `present-indefinite` | — |
| `numerals` | A1 | 65 | `plural` | — |
| `pronouns-demonstratives` | A1 | 75 | `accusative` | `Person=1`, `Person=2`, `Person=3` |
| `postpositions` | A2 | 115 | `locative-grid`, `possessive` | — |
| `time-and-date` | A2 | 125 | `numerals`, `postpositions` | — |

1. **All five are `explained`.** None carries a feature with a suffix exponent, and
   `Generator.KindsFor` derives generators from features, so they resolve to
   `drill.unavailable.noGenerator` with no code change. Thirteen of thirty-three topics now
   produce drills (was thirteen of twenty-eight; measured on the full pack).
2. **No new feature ids.** `NumType=*` would be a heatmap cell nothing could ever grade; the
   ordinal's `-adik/-edik` would additionally have made the allomorph generator offer a drill on
   an unattested form. The lessons carry the rule instead. `Person=*` is placed on the pronoun
   topic because it is where the learner first meets the persons as a set.
3. **Adjectives are not a topic.** Attributive invariance and predicative agreement already sit in
   `plural` and `comparison`; a topic that restated them would not be a gap filled.
4. **Prerequisites are advice, not a lock** ([ADR-013](#adr-013)); only the DAG's order changes.

**Also fixed here.** The ear-trainer pair `szurok`/`szűrök` was tagged `u/ű` but differs in two
vowels (`u→ű` and `o→ö`) — the flaw [ADR-033](#adr-033) retired `orul`/`örül` for, and
`tests/test_minimal_pairs.py` caught it. It is **removed, not replaced**: `u/ű` stays trained by
`fut`/`fűt` and `tud`/`tűd`, and a replacement without rendered clips would be a trial with no
stimulus. 29 pairs, the same 18 contrasts; its 12 audio rows are dropped.

**Rejected.** A `Number` or `NumType` feature for numerals (above); generating drills for the new
topics from `Skill=*` markers (offers what cannot be built); a sixth topic for adjectives (3).

**Consequences.**
- `hu-v1` and the starter carry 33 topics; counts in CLAUDE.md, `docs/LANGPACK.md`,
  `docs/TUTOR.md` follow. The starter grows by about 9 KB gzipped.
- A learner's stored `topic_progress` is unaffected: rows are keyed by topic code and new topics
  start where every undrillable topic starts, in `explained`.

## ADR-056 — Reading sets are existing pack sentences, imported by an explicit click

**Status:** accepted · 2026-11-08 · builds on [ADR-050](#adr-050) (course units) and [ADR-023](#adr-023) (text level)

**Context.** The reader asks the learner to bring a text; a beginner has none at the right level,
and the gap between the course's words and "read the news" is where extensive reading stalls
([R11](research/11-extensive-reading.md), [R6](research/06-lexical-coverage.md)). The pack already
holds thousands of human-written, translated, tagged sentences.

**Decision.** (1) The pipeline stage `reading_sets` picks 20-30 existing Tatoeba sentences per
course unit (36 sets) under a CEFR band from `lemma.cefr`, each sentence in at most one set; no
text is generated, so nothing needs `internal/corpus` attestation. (2) The API lists them and
imports one on an explicit POST: the body is the set's sentences, one per paragraph, ingested by
the same path as a paste (`Server.ingestText`) with `source_url = "set:<code>"`; idempotent per
learner. (3) Importing creates no note and no card — mining stays the learner's act.

**Alternatives rejected.** Generating a graded passage with the LLM (breaks "a sentence in the
pack is one a person wrote"); a `set_import` table (the text row already records it); copying the
full pack's sets into the starter (can name sentences the starter lacks — it recomputes instead).

**Consequences.** Level is an estimate from frequency. Starter sentences are 3-4 tokens, so its sets
read as lists of short lines; a full pack gives longer ones. A pack older than this ADR serves an
empty list until `make refresh-curriculum`. The text model has no per-sentence translation slot, so
the translation is not carried into the imported text.

## ADR-057 — Course grows to three finite tiers; learned counts are derived

**Status:** accepted · 2026-11-09 · extends [ADR-050](#adr-050) · relies on [ADR-047](#adr-047),
[ADR-048](#adr-048) · [R11](research/11-extensive-reading.md), [R6](research/06-lexical-coverage.md)

**Context.** The owner asked that the roadmap stop saying «Месяц 1…6» (the learner picks their own
pace), that the six-month list be called the BASIC level with an intermediate and an advanced level
behind it, and that the main screen show how many words are learned and how many remain. They also
reported that a unit right after enrolment read «пройден»: `done` is derived from "every word is in
the deck" (ADR-048 and the CLAUDE.md invariant), which is true the moment a unit is enrolled, so the
label claimed something about the learner that had not happened.

**Decision.**
1. **Three finite tiers.** `course_unit.tier` is `basic` (the existing 24 units, months 1-6),
   `intermediate` (20 new core units, months 7-12, `ord` 701-720) or `advanced` (20, months 13-18,
   `ord` 801-820); default `basic` for a pack that predates the column; theme units are `basic` and
   sort last (`ord` 1001-1012). The pipeline validator now demands **exactly 24 + 20 + 20** core
   units (`course.TIER_UNITS`), consecutive in `ord`. The course still ENDS: after the last unit of
   the advanced tier `handedOver` is true. A fourth tier is the endless drip [R11] rejects.
2. **`month` becomes internal.** It orders units and places checkpoints (`m1/m3/m6/m12/m18`, already
   in `stats.Specs`, now all reachable); the UI shows the tier, never a month number.
3. **Resolution runs over all core units in order.** `current`, `total`, `completed`, `handedOver`
   are computed over basic → intermediate → advanced; theme units still never count. Next follows
   enrolment, order is advice ([ADR-013](#adr-013), ADR-048) — a tier does not lock the next.
4. **`learned` is derived, with the level estimate's own rule.** `UnitState.Learned` counts a unit's
   words that are *counted lemmas*: `known`, or a graduated Review-state card whose retrievability
   is at least `mining.knownThreshold`, suspended and trashed excluded — the function and 30-second
   cache `/api/stats/level` already uses (`Server.countedLemmas`), not a second copy of the rule.
   `GET /api/course` also returns `totals` `{words, inDeck, learned, perTier}` over the core words.
   `status` keeps its meaning (done = in the deck); the screen shows learned/total and does not call
   `done` «пройден». Nothing is stored; the number can go down as memories decay.
5. **Content rules unchanged.** Every word exists in the pack with the stated part of speech; ru + en
   glosses are hand-written (a word with an uncertain gloss was left out); no Hungarian sentence is
   written for the course; a word is in one unit across all tiers; at least 90% of words per tier
   have a Tatoeba sentence in the starter. Reading sets (ADR-056) are built for the basic tier and
   the themes only.

**Rejected.** Extending the basic tier to 64 units (blurs the "six months" milestone the checkpoints
are placed on); an unbounded tier list (the endless drip); a stored `learned`/`completed` counter
(ADR-048: the plan could then disagree with the deck); a second implementation of "counted" for the
course (the two screens would drift); renaming `done` on the wire (a breaking change for no gain —
the label is the UI's to fix).

**Consequences.**
- Words: basic 1 399, intermediate 1 012, advanced 695 (3 106 core) + 539 theme; clozeable share in the
  starter 96.8% / 100% / 100%. The advanced tier is the smallest because unused frequency-list
  candidates with an attested sentence ran out of themes they could honestly fill; it can grow later
  by adding lines, not units.
- The starter grows ~4.2 MB gzipped (19.9 → 24.1 MB): 1 264 lemmas, 2 558 sentences.
- `/api/course` units carry `tier` and `learned`; the response carries `totals`.


## ADR-058 — Soft gate by settled share

**Status:** accepted · 2026-11-09 · relies on [ADR-013](#adr-013) (order is advice),
[ADR-057](#adr-057) (derived `learned`), `srs.MatureDays`

**Context.** The owner asked that the NEXT unit on the Today path look closed until the learner has
"settled" most of the previous one: «запомнились и ушли на повтор через месяц-два». Without it a
learner enrols unit after unit while the earlier words are still on their learning steps.

**Decision.**
1. A word is **settled** when its lemma is `known`, or its strongest non-suspended, non-trashed
   card is `mature` — Review state with an interval of at least `srs.MatureDays` (21 days, Anki's
   convention, the Words list's split). `UnitState.settled` counts a unit's settled words that are
   in the deck; `totals.settled` / `perTier.*.settled` sum the core units. Same card scan and 30 s
   per-user cache as `learned` (`Server.countedAndSettled`); no FSRS maths repeated.
2. **`course.UnlockShare = 0.70`.** `GET /api/course` returns `unlock: {share, settledDays}` so the
   client hard-codes neither.
3. **It is a drawing, not a lock.** The path draws the unit after one with `settled < 70%` of its
   words as CLOSED, with a notice; opening it anyway is always one tap. Nothing is stored (no
   `unlocked` flag), no endpoint refuses enrolling or practising it, `plan.current` and
   `handedOver` are unchanged.

**Why not a hard lock.** ADR-013: locking on order wedged a learner behind `vowel-harmony` with all
of A1 shut behind it. A gate on "remembered for 21 days" is slower still, and a learner who knows
Hungarian already, or who learns from the reader, must never wait on a clock.

**Rejected.** A stored or server-enforced lock (the wedge); gating on retrievability or on the
learning stage `young` (a word seen yesterday is not settled); a client-side constant (drifts).

**Consequences.** Pace: a 60-word unit introduced over three days takes weeks to settle, so the
closed look will be the normal state for a keen learner — the one-tap override and the theme units
and reader words exist for exactly that. `settled` can go down (a lapse returns a card to
relearning). The figure is a subset of `wordsInPlay`, so a unit not yet enrolled shows 0 unless its
words came from elsewhere.

## ADR-059 — Lively redesign: palette A, derived progress rings, two-tone illustrations, soft unit gate presentation

**Status:** accepted · 2026-11-09 · amends [ADR-037](#adr-037) (drawings), UX.md §14.1/3/5/7/9 and §2;
presents [ADR-058](#adr-058); leaves [ADR-013](#adr-013) and the no-XP rule intact.

**Context.** The owner found the app grey and dry and asked for «more lively and colourful»:
soft coloured cards, progress rings, small illustrations, an activity bar chart, a statistics
column, animation in every workflow — and that the next unit look CLOSED until ≥70% of the
previous unit's words are settled, with a ring going red → blue → green and a prompt at ≥70%.

**Decision.**
1. **Palette A «Kalocsa & Balaton»**, light and a derived dark, four tints (cream, mist, blush,
   sage), `--paprika` for graphics only, a dedicated ring ramp (ember → blue → teal-green), radius
   0.875rem. Every pair used is checked by `scripts/validate_palette.py` (`make palette-check`,
   inside `make docs-check`); old token names are kept.
2. **Rings, narrowly.** `ProgressRing` exists for a *derived share with a counted denominator and a
   numeric label* — a unit's settled words. No streak/XP/activity ring. Colour is never the only
   channel (percent text, state word for screen readers, a check at settled). Its stroke animates
   `stroke-dashoffset` — a named exception to "transform and opacity only".
3. **Two-tone decorative illustrations** (six Budapest/Balaton pieces) in the listed empty/rest
   places; never on a graded card, a Note, or beside a metric.
4. **Soft gate presentation.** A unit after one with <70% settled is drawn closed (grey, lock),
   with a notice and a one-tap «open anyway». Nothing refuses; ADR-013/058 stand. At ≥70% on the
   current unit a friendly prompt offers the next section.
5. **Layout.** Shell container `max-w-5xl`; `.page-narrow` for single-column pages; route fade,
   staggered arrival, shimmer, halo and count-up, all off under `prefers-reduced-motion`.

**Why.** Delight without a points economy: every coloured thing here carries a measured fact or
is plainly decoration. [R14]/[R16 §3] stay binding — competence signals only.

**Consequences.** Contrast is now tested, not asserted (a script reads the shipped tokens).
`accent` stays the light mist surface; paprika is a separate token so existing hover tints do not
turn red. Wider shell means every single-column page must opt into `.page-narrow`. A ring can go
down; the copy must not call it progress-that-only-rises. New illustrations add ~4 KB to the
bundle. Rejected: confetti, level badges, a ring for streaks/time, a hard lock.

## ADR-060 — Answer options for review cards

**Status:** accepted · 2026-11-09 · relies on [ADR-017](#adr-017), [ADR-019](#adr-019) ·
[R12](research/12-retrieval-direction.md), [R1](research/01-retrieval-practice.md)

**Context.** Typing Hungarian with accents is awkward on a phone; the owner asked for
Duolingo-style «pick the right one of up to 5» on `cloze` and `produce`, including words mined
from reading.

**Decision.** The queue ships `QueueItem.Options` (3-5 shuffled strings including `expected`),
built in `internal/api/options.go` from attested forms: same-lemma forms for cloze (via
`morph.FormsOf`), same-POS neighbour lemmas without a shared gloss for produce. Seeded by
(card, day). Fewer than three buildable options means none, and the client types. A tap is sent as
the ordinary `typed` answer with `via: "choice"`; the existing `gradeTyped` grades it. `via` is
validated, never stored, ignores `usedHint`, and cannot yield Easy (no median duration is passed on
this path, so a correct answer is Good). Reader and custom notes use the same builder.

**Why.** Recognition is weaker than recall (R12), so typing stays the default and the client offers
options on touch. No new grading path means practice, review and replay cannot drift.

**Consequences.** Options reveal nothing new (`expected` is already on the wire). Distractors can
be thin for rare lemmas, which then get none. One extra pack query per option-bearing card per
queue build. Rejected: a rating bonus for choice, storing `via` in `review_log`, a distinct endpoint.

---

## ADR-061 — Managed components: embedded Python sidecars supervised by the binary

**Status:** accepted · 2026-11-09 · extends [ADR-052](#adr-052) (Components screen) and
[ADR-054](#adr-054) (HuSpaCy opt-in), amends [ADR-015](#adr-015)'s "the app cannot start the sidecar"

**Context.** ADR-015/052 drew the honest line at "you bring the container up once": the release
image is distroless (no shell, no Python) and the app has no Docker socket. The owner's next
question was whether pressing «Install» in Settings could do the whole thing inside the one
container, with no restart and no host-port bookkeeping. The `mt` and `nlp` services are plain
Python/uvicorn apps configured by env, so the binary can run them *itself* wherever Python exists
— a native run, or a `full` image variant.

**Decision.** New package `internal/components`. It embeds the two service sources (`go:embed`;
`services/<id>/` stays the truth and `make sync-components` + a drift test keep the copy honest) and,
on an admin's click, builds `<data>/components/<id>.installing/` (venv, pinned pip install, model
preparation), renames it into place and runs `venv/bin/python src/app.py` as a supervised child on
`127.0.0.1:<ephemeral port>`. New config provider `managed` (mt/nlp only) with no stored endpoint:
`api.resolveService` is the one resolver the clients, the probe and the Components rows share.
Children get an allowlisted environment, their own process group, a ring-buffer + rotating log,
restart with backoff (5 crashes / 10 min → `failed`) and SIGTERM→SIGKILL on stop/shutdown. Boot
autostart is a goroutine; shutdown stops children before the DB closes. Routes
`POST /api/admin/components/:id/{install,start,stop,uninstall}` (admin, single-flight, 202/409/404),
and `managed{…}` on the existing list. Install sets `provider=managed` via `updateConfig`; Uninstall
resets it; leaving `managed` in Settings stops the child.

**Why.** It removes the only manual step without giving the app a Docker socket (which would be
root on the host), keeps one container and one volume, and keeps the core independent: nothing
changes with the components absent.

**Rejected.** Talking to the Docker socket (root-equivalent); baking Python and the models into the
default image (size, and the distroless guarantee); downloading and executing code named by a request
or by config (an admin session would become remote code execution — only embedded sources run);
a stored endpoint (an ephemeral port would go stale and could clash); embedding the LLM (multi-GB,
GPU libraries).

**Consequences.**
- Needs `python3` ≥ 3.10 + venv + network; the default distroless image reports it unavailable and
  the `full` image variant (`--target full`) carries Python.
- A venv is renamed after creation; its scripts' shebangs point at the old path, which is why the
  service is always started as `venv/bin/python app.py` and pip is never run after the rename.
- Install time, sizes and RAM are documented estimates; **no real install had been run when this
  was written** — only the fake-runner tests and the `/bin/sh` runner test exist.
- `Stop` lasts until the app restarts (autostart follows the provider, which stays `managed`).
- torch (mt, conversion only) is installed per OS: macOS/other from PyPI (`torch==2.5.1`, CPU/MPS);
  Linux the wheel alone from the PyTorch CPU index with `--no-deps`, its dependencies from PyPI —
  never resolved against the PyTorch-only index (first real install failed there building
  `typing_extensions` from sdist). All installs use `--only-binary=:all:`, so a missing wheel is a
  clear error. pip's cache lives in `<id>/cache/pip`, kept across a failed attempt and freed on success.
- **Trust.** An admin who presses Install makes the server run `pip install` of PINNED, public
  packages (PyPI; the PyTorch CPU index only for torch itself on Linux) and then run code from
  them as the app's own OS user. That is the same trust as installing the app. What bounds it:
  only embedded sources and constants reach a command line (no URL, path or argument from a
  request); wheels only (`--only-binary`), so no package build scripts run; children get an
  allowlisted environment (no secrets), loopback-only listening, their own process group, and
  never the database. It is NOT a sandbox: a compromised pinned package would run with the app's
  file access. Hash-pinned requirements would tighten this and are not done.
- **Orphans.** A child is killed with the app on every graceful exit. On a hard kill: Linux sets
  `Pdeathsig=SIGKILL`; macOS has no equivalent, so a SIGKILLed server leaves its child running until
  the next start, when `reapStale` reads `<id>/run.pid` and kills the group if the pid's command
  line still names this component's venv. Installs are manager-wide single-flight; a child that
  stays up but never answers `/health` for 5 minutes is killed and marked `failed`
  (`errors.componentStartFailed`) rather than restarted.
- Not implemented on Windows (the manager reports unavailable).
- ADR-015's statement that the app cannot start the translator is true only of the default image
  and of Compose; with Python available it can, and says so.
- **Image tags (2026-11-10).** `:latest` / `X.Y.Z` / `X.Y` / `X` are now the Python-capable image
  (Dockerfile target `full`), so a fresh install can add the analyser or translator from Settings
  without changing the tag; `:latest-full` / `X.Y.Z-full` stay published as aliases so snippets in
  the wild keep working. The distroless target (`slim`, no Python; the last stage, so a contributor's
  plain `docker build .` still builds it) is `:latest-slim` / `X.Y.Z-slim`. Both run as uid 65532 on
  the same `/data` layout, so switching tag keeps data and account. The slim image sets
  `TANULOM_IMAGE=slim`; when the Python probe fails there the install answers
  `errors.componentNoPythonSlimImage` («замените тег на :latest») instead of `componentNoPython`.
- **`component run` — the same image as a second compose service (2026-11-10).** `tanulom component run
  <mt|nlp> [--port N] [--host 0.0.0.0|127.0.0.1]` (handled in `cmd/tanulom` before the server's flag parsing; the
  no-argument server and every existing flag are untouched) does the same preflight and the SAME install
  code as the button (`Manager.EnsureInstalled`, synchronous, progress on stdout), skips it when the
  `.installed` marker exists, then runs the embedded service as the process's foreground child
  (`Manager.RunForeground`): host default `0.0.0.0` (a compose sidecar must be reachable by service name;
  the managed mode still binds `127.0.0.1`, and `childEnvAt` is the one place the host is chosen), the
  service's own default port (mt 5003, nlp 5002), the same allowlisted env (no `TANULOM_*`, no secret) and
  the same `MT_MODEL`. SIGTERM/SIGINT are forwarded to the child's process group (SIGKILL after 5 s) and
  the exit code is the child's; there is no in-process restart loop — the container's restart policy is
  the supervisor, so a crash is visible to it. For nlp it also runs the `/warmup` path after `/health`
  answers, so the first start downloads the model. The inputs are an id, an IP literal and a port in
  1024-65535; no URL, path or argument reaches a command line, so ADR-061's rule stands. A bad argument
  exits 2, a missing Python (on `:latest-slim`: «use the :latest image») exits 1.
  **Why a same-image second service rather than a sidecar image:** no extra image to build and publish in
  CI, no second Dockerfile to keep in step with `services/`, and the sources run are the ones embedded in
  the binary (drift-tested). The NLLB weights are CC-BY-NC: they are downloaded by the user's own
  container from HuggingFace at first start, never baked into or redistributed with anything we publish
  (the reason ADR-015's baked image was a poor fit for a published tag). The sidecar has no auth, so the
  docs say `expose:`, never `ports:`, and give it its own volume — never the app's `/data`, which holds
  the SQLite database.

## ADR-062 — The word panel is one draggable, modeless bottom sheet on phones; the review drawer closes on advance

**Context.** Learner report, 2026-11-10: on a phone the review word inspector opened inline under «Подробнее», below the fold; the reader's phone word panel was a hand-rolled fixed box with no handle, animation or drag. On desktop the review drawer (Teacher or word) stayed open when the next card came.

**Decision.** `BottomSheet` gains below-md dragging (handle + header; down closes, up expands to ~92dvh), a `modeless` mode (no scrim, no trap, page behind live, 55dvh default) and a pinned `footer` slot. Review shows the word and the Teacher in the same single sheet at every width (word modeless on a phone, Teacher modal); the inline «Подробнее» block is removed. The reader shows `WordDetail` in a modeless sheet below md (its action row in the footer), the sticky rail from md up. Advancing to another card closes both word and Teacher, keyed on `store.index` + `cardId` (a learning-step card returns with the same `cardId`; the question→answer flip changes neither). The tutor thread stays keyed by note.

**Consequences.** One sheet primitive, one behaviour. Amends ADR-034's "tutor beside the card": the Teacher no longer survives to the next card. Nothing in scheduling, the tutor's data path or the API changes.

## ADR-063 — An audio card speaks the word it asks about; «Знаю» keeps away from «Дальше» on a phone

**Context.** Learner report, 2026-11-10 (phone): on a `spell` card the whole sentence was read out and the card showed nothing, so there was no way to tell which word to type; and «Знаю» sat at the foot of the page directly above the pinned «Дальше», where a slip sets a word aside.

**Decision.** `listen` and `spell` speak `targetForm` (autoplay and the «Слово» button; `spell` slowed to 0.85); the sentence is a second button, «В предложении», context on request. The card states the task in a line of text during the question. The Hungarian stays withheld until the answer, as before. On compact layouts «Знаю» moves to the right end of the stage line above the card; the options-question bar and the desktop placement are unchanged.

**Consequences.** Client-only: no grading, scheduling or DTO change (`spell` was already graded against `targetForm`). An isolated word from device TTS can sound flatter than in its sentence — the sentence button is the remedy. Homophone-free guarantee is not claimed: a word that sounds like another is still disambiguated only by the sentence button.


## ADR-065 — A review sitting survives navigation, not reload

**Context.** Learner report, 2026-11-11: leaving Review for another screen and coming back dealt a new set, so a half-finished sitting was lost. ADR-026 made sets client-side; ADR-028 gave Review a start screen.

**Decision.** The sitting stays in the Pinia store (`stores/review.ts`). Returning to Review resumes it when mode, scope and the local day all match what the store holds; otherwise the start screen shows as before. The start screen gets an explicit «Начать заново» that discards the held sitting and deals a fresh set. Nothing is written to `sessionStorage`/`localStorage`.

**Why not persist across reload.** A stored position into a list refetched from the server would be a client-side retry list: the queue has moved on (answers were logged, burial and the daily budget changed), so index N no longer means the same card. A reload therefore starts from the server's queue, which is the only authority on what is due.

**Consequences.** Amends ADR-026 ("refresh after each set" still holds) and ADR-028 (start screen is skipped on resume). Client-only: no API, scheduling or DTO change. A sitting from yesterday is never resumed (local-day check). Answers already given were logged at the time, so nothing is lost by discarding.

## ADR-066 — Reading sets: curated «Фразы», authored «Диалог», preview before import

**Status:** accepted 2026-11-11, amended 2026-11-11 *(user decision: the original rejection of model-written dialogues is reversed)*.

**Context.** Learner report, 2026-11-11: the shortest-first sets of ADR-056 read as a pile of unrelated fragments, many with names, rare words, morbid or nonsensical lines, and the learner could not see a set before importing it. What a learner wants to read first is what Hungarians really say: short exchanges, the phrases a child hears. No corpus offers that at A1, and Tatoeba lines picked one by one never form a conversation.

**Decision.**
- **«Фразы: X» / "Phrases: X"** stay **Tatoeba pack sentences only**. Scoring adds a conversational bonus (questions, first/second person), a proper-noun penalty and the frequency rank of the rarest content word. Order is a greedy lemma chain. `langpack/pipeline/reading_sets_exclude.txt` removes lines a human rejected (off-topic, morbid, nonsense, broken translation; 345 ids after reading all 36 sets), and a candidate whose content lemmas equal an earlier pick's is skipped (the same frame). `validate` fails on an excluded id.
- **«Диалог: X» / "Dialogue: X"**: one authored dialogue per course unit (36), 8-14 turns, at most 10 words a line, vocabulary and grammar at the unit's level, a Russian and an English translation per line (`pipeline/dialogues*.py`). Stored as `sentence.source = 'authored'` (ids from 9 000 000, no `sentence_token` rows) in `reading_set` rows with `track = 'dialogue'`, `ord` right before the unit's «Фразы». Build-time validation: every word is an attested `wordform`, every content lemma is within the unit's band or taught by that unit.
- **Provenance is stated everywhere**: «Написано ИИ, носителем не проверено» / "Written by AI, not checked by a native speaker" on the card and in the preview; the imported text is recorded as `kind: generated`. Attestation proves a form exists, not that a line is idiomatic; the label is what keeps that honest. The lines were read as a strict Hungarian teacher would, and simpler constructions replaced doubtful ones; still no native review.
- **`authored` feeds reading sets only.** Drills, examples, practice, course sentence search and listening read real corpus sentences: each query filters `source <> 'authored'` (backend tests prove it); the n-gram attestation index never counts authored lines, so the model cannot vouch for its own text; the starter gets its dialogues from `reading_sets.build`, never from the full pack.
- `GET /api/library/sets/:code` returns the set for preview (`{…, authored, sentences:[{hu, ru?, en?}]}`, 404 for an unknown code); the list DTO gains `authored`. The Library's «Посмотреть» opens it in a sheet with the translation in the learner's gloss language (a fallback language is labelled), footer «Добавить» / «Открыть». Import stays an explicit click. `forYou` is derived at request time, never stored.

**Rejected.** (1) Keeping the sets Tatoeba-only: it cannot produce a conversation. (2) Mixing dialogue lines into «Фразы»: a set would be half real and half AI, and the label could not be honest. (3) Letting authored lines serve drills or examples: those need real, attested Hungarian (ADR-042, "no LLM Hungarian becomes a card").

**Consequences.** Amends ADR-056; supersedes the 2026-11-11 first text of this ADR ("model-written dialogues rejected") and narrows INVARIANTS "Reading sets are pack sentences": the one AI-written Hungarian in the pack is a labelled dialogue that feeds nothing but reading. No LLM Hungarian enters a card. DTO, `docs/API.md`, `docs/LANGPACK.md`, UX §5.1 updated. Open: a native speaker's read of the 387 lines; the pack release lists authored lines as CC BY-SA 4.0 (`langpack/LICENSES.md`).

## ADR-067 — Machine translation is fetched when a crib is shown, never by the queue

**Context.** ADR-035 offered the translator wherever one was missing, as a button; ADR-014 made the translation a labelled crib. On a card whose crib is on screen but empty, the learner had to tap to see what the app could already supply.

**Decision.** `SentenceCrib` fetches the translation by itself, and only where the crib already renders: on cloze before the answer, on the other directions after it. An automatic failure is silent (no error banner); the manual button remains. The model label is unchanged (ADR-014). `WordInspector` batches its translation into one call. The reader's sentence translation stays one tap. There is no setting.

**Consequences.** Amends ADR-035 and ADR-014's "nothing until asked": the queue and scheduler still only read the cache and never trigger MT, and with the sidecar stopped the card behaves as before. Display-only: no grade, state or DTO change.

## ADR-068 — Services can be paused from the account menu

**Context.** A learner on a small machine wanted TTS or the LLM off for a session without deleting provider, endpoint or model from Settings.

**Decision.** `Service.Paused` is a flag that keeps provider, endpoint and model. `POST /api/admin/services/:name/paused` (admin only) sets it. Paused means treated as off everywhere and the managed child process (ADR-061) is stopped; it survives restart and a settings `PUT`. Non-admins see read-only status. Short names in the menu: tts «Озвучка» / "Voice", asr «Речь в текст» / "Speech to text", llm «ИИ-учитель» / "AI tutor", mt «Перевод» / "Translation".

**Consequences.** The app is already fully usable with every sidecar off, so pausing is the same path as "not configured". The flag lives in `config.yaml` via `Server.updateConfig` and `config/clone.go`; no secret is involved. Resuming restarts the managed child.

## ADR-069 — Remove the context-disambiguation sidecar

**Context.** ADR-054 added an optional HuSpaCy (`hu_core_news_md`) sidecar that, at text ingest and live `/api/analyze`, re-picked among the pack's own candidate readings for `uncertain` words (`Via="context"`, ladder step 4 of ADR-002). Nobody measured a benefit; it costs about 2.4 GB of RAM, a managed-component installer path, a compose profile, an admin warm-up endpoint and a Settings card.

**Decision.** Remove it entirely: `services/nlp`, `internal/nlp`, `morph.ApplyContextHints`, `AnalyzedToken.Via`, `services.nlp` in config, the `nlp` capability, the managed component and its warm-up machinery, `POST /api/admin/nlp/warmup`, the Settings card and the «по контексту» label. The lookup ladder is exact → prior → suffix. An uncertain word keeps the frequency prior and its "uncertain" mark, exactly as with the sidecar stopped before.

**Rejected.** An LLM as a substitute: slow per word, optional itself, and its choices are not attested in the pack. Keeping the code dormant: dead paths rot, and git history restores it (the last commit that has it is the one before this change).

**Consequences.** Supersedes ADR-054; amends ADR-002 step 4 (gone) and ADR-061 (`mt` is the only managed component). Capabilities list four services (tts, asr, llm, mt). An old `config.yaml` with a `services.nlp` block still loads (unknown keys are ignored) and the block is dropped on the next write; a stored `nlpKey` secret is simply never read. "The app is fully usable with every sidecar stopped" is unchanged.

## ADR-064 — Practice answers are reviews

**Status:** accepted · 2026-11-11 *(user decision)* · supersedes [ADR-019](#adr-019) and the "schedules nothing" part of [ADR-024](#adr-024) · see [R20](research/20-practice-and-progress.md), [ADR-003](#adr-003)

**Context.** ADR-019 kept practice out of FSRS for fear of the cramming problem and rejected a discounted update as scheduling maths of our own. The learner, who trains daily, reports the cost: a word answered correctly five times in «Тренировка» still sits at the same stage next to its review, so the trainer feels like it does nothing and the stage chip lies about what they know.

**Decision.** A practice answer is a real review. `POST /api/practice/answer` grades exactly as before, then calls the same `Engine.Answer` the review queue calls: one card review plus feature reviews only for the features the item required (`emit.go` untouched), one `review_log` row. It still writes `practice_log` with the SAME instant, so the practice volume stats are unchanged. It counts toward `maxReviewsPerDay`, the interaction ledger (streak, `due_left`) is refreshed through the review path, and sibling burial sees the answer. Undo deletes the `practice_log` row with the review it reverts (same card, same instant). The response carries `counted`, rating, `nextDue`, `intervalDays`, `gradedFeatures` and `memory`.

**Why this is sound.** go-fsrs v4 (FSRS-6) uses real elapsed time. An early review gains stability through `nextRecallStability` with the real recall chance (small when early, so the gain is small). A same-day answer uses `shortTermStability`, floored at the current S for Hard/Good/Easy. The library already discounts what ADR-019 feared; we add no maths and no weights.

**Exceptions (practice_log only, as before).** (1) Words in «Знаю» (trashed note, `scope=trash`): nothing about them moves ([R14] / trash rule). (2) NEW cards: a first review would bypass the global daily new budget. The response says `counted:false` with `reason` `known` or `new`, so the screen can say why nothing moved.

**Rejected.** "Only every second correct answer counts": drops real recall events, keeps every miss, biases the log toward failure, and needs a hidden counter. "Record only if due": makes practice a no-op exactly when it is most useful. A half-weight review: our own maths.

**Accepted FSRS behaviours.** Practice can advance learning steps without the 10-minute spacing. A same-day Good on a review card moves its due date to now + interval.

**Consequences.** `/api/practice/*` is no longer free of state; INVARIANTS.md is rewritten. Practice cards still carry no grade preview in the queue (the learner chooses the order, not the scheduler). `replay` stays exact: the log is the only source. Weak mode (ADR-024) is unchanged.

## ADR-070 — A progress number per word, mapped from FSRS stage and stability

**Status:** accepted · 2026-11-11 *(user decision)* · amends [ADR-039](#adr-039) and the ADR-026 amendment · see [R20](research/20-practice-and-progress.md)

**Context.** The statuses (new, learning, young, mature, relearning) are coarse, and with practice moving them (ADR-064) the learner wants to see a word rise on a right answer and fall on a mistake. ADR-039 refused any "N correct" threshold because FSRS has none.

**Decision.** Every card carries `Memory.progress`, an integer 0–100, derived by `srs.progressOf` from the stored state alone; the statuses are marks on it. Bands come from `StageOf`, so the number always agrees with the stage chip:

| Stage | Progress |
|---|---|
| new | 0 |
| learning, relearning | 0–19: `floor(20·(done+1)/(total+1))`, done = steps total − steps left (default steps: 6, then 13) |
| young (< 21 d) | 20–69: `20 + floor(50·min(S/21, 1))` |
| mature (≥ 21 d) | 70–100: `70 + floor(30·clamp(ln(S/21)/ln(180/21), 0, 1))` |

S is FSRS stability in days. 180 days (`srs.ProgressFullDays`) is a declared display horizon, a convention like the 21-day split, not a memory threshold and not a goal: 100 means "stable for half a year", not "learned". A word's number is its best card's; the inspector lists each card.

**Behaviour.** It rises on correct answers and falls on a lapse (relearning is below 20). It never counts up on its own: time passing changes nothing, only answers do. Display-only: never called a score, points or level; no XP-like surface (see the motivation invariants).

**Known edges, documented not patched.** Interval fuzz can flip a card between mature and young at the 21-day boundary, so a correct answer may show a small drop (−1; up to −9 in the property test, because the mature band is by interval and the fuzzed interval can exceed S) and the stage chip flips with it. A same-day answer on a settled card shows +0 by FSRS's own arithmetic. The property test logs these flips.

**Consequences.** `WordRow.progress`, `DeckStages.progress` (deciles of words with a reviewed card), `memory` on the lemma detail. Deciles count words by memory state, not reviews, so "every review number comes from `srs.Breakdowns`" still holds.

## ADR-071 — «Знаю» covers the word, not one note

**Status:** accepted · 2026-11-11 *(user report)* · amends [ADR-041](#adr-041)

**Context.** «Знаю» set `trashed_at` on the one note under the cursor and the lemma `known`. A lemma owns one note per sentence, text and course unit, so the same word came back the same day through another note (the queue never looks at lemma status). Enrolling a unit also matched on `(hu, target_form)` only: it could mint a fresh note for a known lemma and its duplicate path could un-trash a «Знаю» note.

**Decision.** The link is `note.lemma_id` (a pack lemma id; nullable). `Store.TrashWord` sets aside every live note of that lemma for the user in one transaction with one `trashed_at` instant and marks the lemma `known`; `Store.RestoreWord` restores the notes carrying the tapped note's instant and sets the lemma `learning`. A note with no lemma moves alone. Notes trashed at another time stay where they are. Enrolment skips a word whose lemma status is `known`, reporting it in `skipped` with `known: true`, and never restores a note for it. The review client drops every queued card of the lemma.

**Consequences.** `trashed`/`TrashCount` and `GET /api/trash` count and list words (a lemma and an instant), not notes; `cards` sums the directions of every note moved. Nothing is deleted, so undo and replay are unaffected. Manual add from the reader (`POST /api/notes`) still restores a trashed duplicate: that is the learner's explicit act.

**Same change: display case.** A cloze blank at the head of a sentence carries the sentence's capital, not the word's. Options (`internal/api/options.go`) and `expectedFor` for a cloze show the form in its lemma's case, lowercase unless the dictionary form is capitalised; grading was already case-insensitive. A tapped option is spoken (UX.md Review).


## ADR-072 — Course words take any attested pack sentence; unit phrases are pack sentences; AI gap-filling waits for attestation

**Status:** accepted · 2026-11-11 *(user report: `sziasztok` was left without a card)* · extends [ADR-042](#adr-042), [ADR-066](#adr-066)

**Context.** Greetings and interjections live in one- and two-word sentences, and enrolment's three-token floor left such words «без примера». The tempting fix is to let the model write the missing sentence. That collides with the invariant that no LLM Hungarian becomes a card without `internal/corpus/attest.go`, with [ADR-042](#adr-042) (the model may write a dictionary entry, never material) and [ADR-066](#adr-066) (the one labelled AI-written Hungarian lives in «Диалог» sets, `source='authored'`, filtered out everywhere else). MT stays display-only and labelled ([ADR-014](#adr-014)).

**Decision.** (1) Enrolment gains a last-resort pass with `len_tokens >= 1` — still `source <> 'authored'`, same attestation — which runs only when the 3+ token passes found nothing. When the analyser files the token under another lemma, a token spelled like the course headword still gives the cloze span, with EMPTY target features (a cloze needs a span, not features; another lemma's features are never graded); a lemma-id reading stays preferred. (2) `GET /api/course/units/:code/phrases` shows up to four short pack sentences with human translations — display-only, no note. (3) `UnitState.noExample` names enrolled-unit words the pack still cannot illustrate, derived per request. (4) AI gap-filling for those words is NOT built: it waits for attestation. The user may later override this explicitly; that would be a new ADR, with the output labelled and kept out of `note` until attested.

**Consequences.** A one-word greeting can become a card when the pack has such a sentence; a pack with none (the embedded starter has no `sziasztok` sentence) still reports the word, honestly, in `skipped` / `noExample`. `QueueItem.unit` (from `note.source = 'course:<code>'`) is display-only. Nothing here touches scheduling or the two state spaces.
