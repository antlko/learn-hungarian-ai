# R12 — You get back the direction you trained

**Confidence: High.** Transfer-appropriate processing is a robust principle.

## Claim

Vocabulary knowledge is **direction-specific**. Practising recognition (L2→L1) builds
recognition. Practising production (L1→L2) builds production. Training one does not fully
deliver the other.

## Evidence

- **Nakata (2016)** — 60 Swahili–English pairs, 64 participants; retrieval direction manipulated
  in both treatment and post-test. **Productive recall formats outperform recognition.**
- **Webb (2008, 2009)** — receptive knowledge measured by L2→L1 translation, productive by
  L1→L2; learners perform better on whichever they trained.
- **Forward vs backward translation:** forward translation (L1→L2) is more effective for **form
  recall**; backward (L2→L1) for **meaning recall**.
- Retrieval-format research generally: **recall > recognition**, and the cost is time.

Links:
- Retrieval formats — https://www.researchgate.net/publication/294443124
- Receptive/productive retrieval practice — https://www.researchgate.net/publication/303939278
- Intentional vocabulary activities meta-analysis — https://www.academia.edu/84277734

## What this changes in our code

**This note is the reason a note spawns multiple cards.** `internal/srs/directions.go`:

| Direction | Cue → Response | Trains | Default |
|---|---|---|---|
| `recognize` | HU → RU/EN | meaning recall (receptive) | on |
| `produce` | RU/EN → HU (typed) | form recall (productive) | on |
| `listen` | audio → meaning | aural recognition | on when TTS available |
| `cloze` | sentence with target form blanked | contextual production + morphology | on |
| `spell` | audio → typed HU | orthography — **this is what actually trains `a/á`, `ö/ő`** | on |

Each direction carries **its own independent FSRS state**. A lemma can be solid receptively and
due productively, and the app will schedule exactly the direction that is weak.

Consequences elsewhere:

1. **The goal drives the weighting.** Settings → Review exposes a goal
   (`reading` / `conversation` / `exam`) that re-weights which directions are generated for new
   notes. An exam-focused learner gets all five; a reading-focused one can drop `spell`.
2. **`spell` is not optional for Hungarian.** Vowel length is phonemic and orthographic, and a
   learner who never types `ő` will never internalise it. It is on by default and the UI
   explains why. Cf. [R7](07-hvpt.md) — perception and orthography reinforce each other.
3. **`cloze` is the highest-value direction** for our purposes, because it is the one that emits
   **morph feature reviews** into the second state space (`internal/srs/emit.go`). Recognition
   of a bare word teaches nothing about suffixes.
4. **Cost is acknowledged.** Five directions per note means five times the review load. The
   new-card governor (`internal/srs/forecast.go`) counts *cards*, not notes, so the daily limit
   is honest about what the learner is signing up for.

## Caveats

- Most of this literature uses paired-associate learning of isolated words. Our unit is a
  sentence in context, which is a departure — though one the source plan explicitly requires
  ("Только предложения, не слова").

## Related

[[R1]](01-retrieval-practice.md) · [[R3 FSRS]](03-fsrs.md) · [[R7 HVPT]](07-hvpt.md)
