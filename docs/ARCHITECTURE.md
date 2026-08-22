# Architecture

How the system behaves. Read this before changing anything structural.

**Read first:** [`.claude/DECISIONS.md`](../.claude/DECISIONS.md) for *why* the shape is what it
is. This document describes the *what*.

---

## 1. Shape

A **static Go binary** embeds the built Vue SPA and serves both the API and the UI on one port.
Optional sidecars add speech and NLP. Nothing in v1.0 requires a sidecar.

```
                    ┌──────────────────────────────────────────┐
   browser ────────►│  tanulom  (one static binary, distroless) │
                    │  ┌────────────────────────────────────┐  │
                    │  │ Fiber v3 · API + SPA catch-all      │  │
                    │  ├────────────────────────────────────┤  │
                    │  │ srs · morph · mining · drill · hvpt │  │
                    │  │ curriculum · stats · export · corpus│  │
                    │  └───────┬──────────────────┬─────────┘  │
                    │          ▼                  ▼            │
                    │   tanulom.db          langpack-hu-vN     │
                    │   (read-write)        (read-only, ATTACH)│
                    └──────────┬───────────────────────────────┘
                               │  optional, probed at runtime
              ┌────────────────┼─────────────────┬──────────────┐
              ▼                ▼                 ▼              ▼
        services/nlp     services/tts      services/asr     any OpenAI-
        HuSpaCy          Piper             faster-whisper   compatible LLM
```

---

## 2. The two state spaces — the core idea

Everything distinctive follows from refusing to treat a wordform as an atom.

```
  "házaimban"
       │
       ▼  internal/morph — indexed lookup in the language pack
  ┌────────────────────────────────────────────────┐
  │ lemma: ház                                     │
  │ feats: Case=Ine | Number[psor]=Plur |          │
  │        Person[psor]=1 | Number=Sing            │
  │ segmentation: ház + a + im + ban               │
  └───────┬──────────────────────────┬─────────────┘
          ▼                          ▼
   lemma_state[ház]          morph_state[Case=Ine]
   (FSRS-6 state)            morph_state[Number[psor]=Plur]
                             morph_state[Person[psor]=1]
                                   (FSRS-6 state each)
```

Hungarian UD exposes **22 case values** plus mood, verb form, definiteness, possessor marking,
voice, aspect and degree — roughly **120 teachable atoms** after merging allomorph variants.
Inventory in [R17](../.claude/research/17-hungarian-nlp-stack.md).

**What this unlocks:**

| Capability | Why it needs two spaces |
|---|---|
| Real `n+1` | `elmentem volna` has *zero* unknown lemmas and *two* unknown features. Lemma-only counting calls it easy; it isn't. |
| Per-suffix weakness map | "Your inessive is solid; your conditional is at 0.41 retrievability" is only expressible if features have their own state. |
| Targeted generation | LLM prompts carry the known-lemma set *and* the feature to drill. |
| Honest progress | You cannot fake B1 by grinding vocabulary while morphology rots. |

---

## 3. Data ownership

Two SQLite files in the data dir, plus one YAML.

| Store | Holds | Mutability |
|---|---|---|
| `config.yaml` | settings: review params, mining weights, drill thresholds, service endpoints | hand-editable; app writes atomically |
| `tanulom.db` | users, sessions, **secrets**, notes, cards, review logs, both state spaces, texts, drills, HVPT trials, recordings, checkpoints | read-write |
| `langpack-hu-vN.sqlite` | lemmas, wordforms, paradigms, features, sentences, minimal pairs, curriculum, n-grams | **read-only**, `ATTACH`ed |

**Invariants.**

1. **`config.yaml` is mutated only via `Server.updateConfig`** — clone → mutate → validate →
   atomic save (temp+rename) → swap under lock. Never touch `s.cfg` directly.
2. **Secrets never go in YAML.** API keys live in `tanulom.db`, are stripped from DTOs (only a
   `secrets: {key: true}` presence flag is exposed), and are preserved on `PUT` when omitted.
3. **The language pack is never written to.** Learner corrections to a bad analysis go in
   `morph_correction` in the user DB and are applied as an overlay at lookup time.
4. **`normalize()` fills zero fields only.** Changing a default affects new installs and unset
   fields — never silently rewrites a user's stored values.
5. **Migrations are embedded and run inside `db.Open`.** There is no migrate command and the
   distroless image has no shell to run one. Never edit an applied migration.

---

## 4. Package graph

Import direction matters; these are the edges that prevent cycles.

```
cmd/tanulom
    └── api ──┬── config ── state
              ├── db (goose migrations, embedded)
              ├── auth
              ├── lang ── (opens + ATTACHes the pack)
              ├── morph ── lang
              ├── srs ── db
              ├── mining ── morph, srs, stats
              ├── curriculum ── lang, db
              ├── drill ── curriculum, morph, srs, llm?
              ├── hvpt ── lang, srs, media
              ├── stats ── db, lang
              ├── export ── db, media
              ├── speech ── media          (optional providers)
              ├── llm ── corpus            (optional providers)
              ├── corpus ── lang
              ├── media
              └── web (//go:embed all:dist)
```

**Never:** `lang` or `morph` importing `srs` (they are language knowledge, not learner state).
**Never:** `srs` importing `drill` (the scheduler must not know about exercise types).
**Never:** `llm` importing anything that would let it write cards directly — generation goes
through `corpus` attestation and then through the normal note-creation path.

---

## 5. Key algorithms

### 5.1 `n+1` sentence scoring — `internal/mining/score.go`

```
known(lemma)   := status = 'known' OR retrievability(lemma_state) ≥ θr     θr default 0.80
known(feature) := retrievability(morph_state) ≥ θr
unknown(S)     := |lemmas ∉ known| + w_f · |features ∉ known|              w_f default 0.7

score(S) = a₁ · gauss(unknown(S); μ=1.5, σ=0.8)     // the n+1 sweet spot
         + a₂ · [elementCoverage(S) ≥ 0.90]          // comprehensibility floor  [R6]
         + a₃ · utility(new lemmas)                  // frequency-weighted payoff
         + a₄ · relevance(current curriculum topic)
         + a₅ · lengthFit(n; target 6–14)
         − a₆ · dupPenalty(S)
```

All weights in `config.yaml` under `mining:`. Tunable without a rebuild.

### 5.2 Dual-space review emission — `internal/srs/emit.go`

One cloze or produce answer emits **1 card review + N feature reviews**.

| Outcome | FSRS rating |
|---|---|
| correct, latency < learner's median | `Easy` (4) |
| correct, normal latency | `Good` (3) |
| correct but slow, or after a hint | `Hard` (2) |
| incorrect | `Again` (1) |

**Only features actually required by the item are graded** — derived from the blanked span's
`feats`, not from every token in the sentence. Grading unrequired features is what would turn
the heatmap into noise.

### 5.3 New-card governor — `internal/srs/forecast.go`

Simulate 90 days from the current stability distribution, new-card limit `L`, desired retention
`R`. If projected peak load exceeds `daily_minutes / median_seconds_per_review`, **refuse to
raise `L`** and render the curve. The learner may override — after seeing it.

This exists because the brief names review-avalanche as the single most common cause of quitting.

### 5.4 Corpus attestation — `internal/corpus/attest.go`

2–4-grams of any LLM-generated Hungarian are checked against the pack's `ngram` index.
Unattested spans are marked *«не подтверждено корпусом» / "not attested in corpus"* and
**cannot become a card without an explicit override**. See
[R10](../.claude/research/10-llm-tutors.md).

### 5.5 Morpheme-aware diff — `internal/morph/diff.go`

Segment the learner's answer and the target, align morphemes, classify the divergence:
`harmony | case | definiteness | possessor | vowel_length | agreement | spelling`. The **class**,
not the mere fact of being wrong, drives what gets drilled next.

---

## 6. Request flows worth knowing

**Answering a review card**
```
POST /api/review/answer
  → srs.Grade(card, rating)                    // FSRS-6 next state
  → morph.Diff(answer, target)                 // if typed
  → srs.EmitFeatureReviews(item.requiredFeats) // second state space
  → db tx: card + review_log + morph_state[]   // one transaction, all or nothing
  → drill.MaybeEnqueueFromError(errorClass)
  → respond with next card prefetched
```

**Ingesting a text**
```
POST /api/texts
  → mining.Extract(source)          // readability / srt / epub parse
  → morph.Tokenize + Lookup         // pack lookup, sidecar if available
  → persist text + text_token
  → stats.Coverage(text, user)      // lemma + element
  → mining.Score(sentences)         // n+1 rail
```

**Capability probing**
```
GET /api/capabilities
  → parallel HEAD/health to nlp, tts, asr, llm endpoints, 2s timeout
  → {name, configured, reachable, latency_ms, detail}
  → cached 30s; UI renders capability dots and degrades features
```

---

## 7. Where to make a change

| I want to… | Go here |
|---|---|
| Change how intervals are computed | You don't. `internal/srs/scheduler.go` wraps FSRS unmodified — see [ADR-003](../.claude/DECISIONS.md#adr-003) |
| Change what gets scheduled | `internal/srs/directions.go`, `emit.go` |
| Change how sentences are chosen | `internal/mining/score.go` + `mining:` in `config.yaml` |
| Add a grammar topic | `langpack/pipeline/curriculum.py`, then rebuild the pack |
| Change the blocked→interleaved gate | `internal/drill/schedule.go` + `drill:` in `config.yaml` |
| Add an exercise type | `internal/drill/generate.go` + a renderer in `web-ui/src/components/drill/` |
| Add a minimal pair | `langpack/pipeline/minimal_pairs.py` |
| Add an API endpoint | `internal/api/handlers_*.go`, DTO in `dto.go`, mirror in `web-ui/src/types/`, document in [`API.md`](API.md) |
| Change the schema | new `internal/db/migrations/NNNNN_name.sql`; never edit an applied one |
| Add a language | out of scope for v1 — the pack schema is language-agnostic but the curriculum and minimal pairs are not |

---

## 8. Frontend

Vue 3 `<script setup>`, Pinia, vue-router, Tailwind v4, shadcn-vue (Reka UI), `@lucide/vue`
(**not** `lucide-vue-next`), `vue-i18n`, `vite-plugin-pwa`.

- Stores: `auth`, `settings`, `ui`, `capabilities`, `review`, `texts`, `curriculum`, `hvpt`,
  `progress`.
- `src/types/` mirrors `backend/internal/api/dto.go` (camelCase). Keep them in sync when shapes
  change.
- Router guards gate setup / login / role — **UX only**. The API enforces roles independently.
- Offline: the review queue and its audio are precached, so Loop 1 works on the metro.

Screens, keyboard map and colour semantics: [`UX.md`](UX.md).
