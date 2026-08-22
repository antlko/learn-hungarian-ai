# R15 — What is actually hard about Hungarian

**Confidence: Medium.** Learner-corpus data plus acquisition studies; less meta-analytic
backing than the memory literature, but directly on target.

## The reordering the source plan gets right

Popular articles lead with "18 cases". The source plan (§2.1) argues that is a **false alarm**,
and the argument holds:

- no grammatical gender;
- suffixes attach regularly, with few exceptions;
- one meaning ≈ one suffix;
- the locative system is a clean **3×3 grid** learnable in an evening.

### The 3×3 locative grid (rendered in the app's Progress view)

|  | **where** (static) | **whither** (to) | **whence** (from) |
|---|---|---|---|
| **interior** | `-ban/-ben` (Ine) | `-ba/-be` (Ill) | `-ból/-ből` (Ela) |
| **surface** | `-n/-on/-en/-ön` (Sup) | `-ra/-re` (Sbl) | `-ról/-ről` (Del) |
| **proximity** | `-nál/-nél` (Ade) | `-hoz/-hez/-höz` (All) | `-tól/-től` (Abl) |

For a Russian L1 speaker the *semantics* are already present (в дом / в доме / из дома); only
the encoding differs. This is the easy part.

## The three real difficulties, in order

### 1. Vowel harmony — a motor skill, not knowledge

Every suffix has 2–3 allomorphs selected by the stem's vowels:

- back (`a á o ó u ú`) → `-ban`, `-hoz`, `-tól`
- front unrounded (`e é i í`) → `-ben`, `-hez`, `-től`
- front rounded (`ö ő ü ű`) → `-ben`, `-höz`, `-től`

Understood in 20 minutes. **Automated in ~3 months.** The bottleneck is repetition volume, not
comprehension — which is why every card must be a sentence carrying a real suffix, from week one.

### 2. Definite vs indefinite conjugation — the killer

`Látok egy házat` vs `Látom a házat`. No analogue in Russian or English.

**Documented error distribution (HunLearner corpus, automatic error detection study):**

| Error pattern | Share |
|---|---|
| **Definite object taking indefinite conjugation** | **55%** |
| **Demonstrative pronouns triggering the wrong conjugation** | **25%** |

Acquisition research with untutored learners finds verbal morphology acquisition is systematic,
driven by **frequency, perceptual salience, morphophonological regularity and semantic
complexity**. Time in-country helps most for the hardest cases — where the object is implicit
and only recoverable from context.

- Acquisition of the definite conjugation by learners of different L1s — https://www.academia.edu/76899084
- Automatic error detection, HunLearner corpus — https://www.academia.edu/65821517

### 3. Preverbs and focus — acquired last

`megy` → `elmegy`, `bemegy`, `kimegy`, `felmegy`. Russian-like prefixes, which helps. But
Hungarian is **discourse-configurational**: word order encodes **focus**, not roles (cases
encode roles). The immediately-preverbal slot is the focus position, and the preverb detaches
when something else occupies it:

- `Elmegyek a boltba.` — neutral
- `A boltba megyek el.` — it's *to the shop* that I'm going
- `Én megyek el a boltba.` — it's *I* who am going

This is what separates "correct but odd" from "native". It requires listening volume first.

## What this changes in our code

1. **Curriculum ordering** (`langpack` `grammar_topic` DAG, `teach_order`): vowel harmony →
   politeness (`ön`/`tetszik`, month 1 — socially costly to get wrong) → possessives → past →
   **definite/indefinite (month 3)** → conditional → infinitive+person → imperative →
   preverb detachment & focus (last, ~B2). Cases are *not* front-loaded as a block.
2. **Vowel harmony is drilled as volume, not explanation.** After the rule card, the app
   generates high-repetition allomorph-selection items and treats the feature as mastered only
   on **latency**, not accuracy alone — motor skills are measured by speed.
   `internal/drill/harmony.go`.
3. **Definite-conjugation drills are weighted to the documented failure modes.** 55% of items
   target definite objects, 25% specifically use demonstrative-pronoun objects.
   `internal/drill/definiteness.go`. Weights are in `config.yaml` and cite this note.
4. **Definite/indefinite is gated blocked→interleaved** per [R5](05-interleaving.md) — this is
   exactly the topic the source plan wanted interleaved from day one, and exactly the topic the
   evidence says must not be.
5. **The 3×3 locative grid is a UI component**, not a list — `LocativeGrid.vue` renders it
   coloured by per-cell mastery. Teaching it as a table is the whole point.
6. **Focus/word-order topics are locked** until listening volume passes a threshold
   (`curriculum.focus_unlock_listening_hours`, default 40). The source plan says "не пытайтесь
   освоить на A2 — сначала нужен объём слышанного"; we enforce it.
7. **Stem alternations** (`ló→lovak`, `tó→tavak`, `fa→fát`) are a **closed list of ~200 words**
   — flagged in the language pack and taught as items, not rules.

## Caveats

- The error-distribution figures come from one learner corpus; the exact percentages should be
  treated as indicative and are configurable rather than hardcoded.
- Acquisition-order studies for Hungarian are few and often use small, L1-specific samples
  (notably Chinese L1), which may not transfer to a Russian L1 learner.

## Related

[[R5 interleaving]](05-interleaving.md) · [[R4 explicit instruction]](04-explicit-instruction.md) · [[R7 HVPT]](07-hvpt.md) · [[R6 coverage]](06-lexical-coverage.md)
