# R7 — High Variability Phonetic Training

**Confidence: High for the effect. The source plan overstates its durability — corrected here.**

## Claim

Training L2 sound perception with **multiple talkers** in **varied phonetic contexts**, with
**immediate feedback**, substantially improves perceptual discrimination — and the *variability*
is the active ingredient, not merely the repetition.

## Evidence

**Uchihara, Karas & Thomson (2025), *Studies in Second Language Acquisition* 47(3):794–827** —
"High variability phonetic training (HVPT): A meta-analysis of L2 perceptual training studies"

- **79 studies.**
- **g = 0.92** pre/post — medium-to-large.
- Moderator analyses cover learner-related and methodological variables.

Related work:
- A separate synthesis of 15 studies found an immediate effect of **g = 0.62** [0.53, 0.71].
- Perception→production transfer has its own meta-analysis in *Applied Psycholinguistics*.
- Origins: Logan, Lively & Pisoni (1991) — the Japanese /r/–/l/ studies that started the field.

Links:
- Uchihara, Karas & Thomson 2025 — https://www.cambridge.org/core/journals/studies-in-second-language-acquisition/article/high-variability-phonetic-training-hvpt-a-metaanalysis-of-l2-perceptual-training-studies/6ABB8C1F32D88D53EA8D05A4565E76F6
- Perception–production meta-analysis — https://www.cambridge.org/core/journals/applied-psycholinguistics/article/does-perceptual-high-variability-phonetic-training-improve-l2-speech-production-a-metaanalysis-of-perceptionproduction-connection/E38D8F5CE65DC708137B0E95F97C6BC7
- Research-to-practice bridge — https://www.researchgate.net/publication/384827252

## ⚠️ Correction to the source plan

The source plan (§1.1) states HVPT has *"с подтверждённым долгосрочным удержанием"* — confirmed
long-term retention.

**This is not what the literature says.** Syntheses report that **HVPT effects diminish over
time, with smaller effect sizes at one- to two-month follow-ups, indicating limited long-term
retention.** The immediate effect is robust; durability is the weak point of the method.

## What this changes in our code

1. **A contrast is never "done".** `internal/hvpt/state.go` gives every contrast a full FSRS
   state and schedules **maintenance trials** indefinitely, exactly like a vocabulary card.
   There is no "completed" badge for a phonetic contrast, by design.
2. **Talker variability is measured and enforced, not assumed.** `hvpt_trial.talker_id` is
   recorded on every trial. `internal/hvpt/trial.go` will not build a session that draws from
   fewer than `min_talkers` (default 5) distinct voices for a given contrast.
3. **Talker sourcing** — the literature's optimum is 5–8 voices, and Piper provides only 3
   Hungarian voices (`anna`, `berta`, `imre`). We reach the floor by combining:
   - 3 Piper synthetic voices,
   - N human speakers from **Common Voice Hungarian** (CC0),
   - ±10% rate jitter and small pitch variation per rendition.

   **This shortfall is real and we do not hide it.** The Ear view shows how many distinct
   talkers your session actually used. See [R17](17-hungarian-nlp-stack.md) for sources.
4. **Immediate feedback on every trial** — 2AFC, correct/incorrect shown instantly, with the
   option to replay both members of the pair. No batched end-of-session scoring.
5. **Varied phonetic contexts, not isolated words.** The `minimal_pair` table carries a
   `context_sentence` where one exists, so `öt`/`őt` is trained in running speech as well as
   in isolation.
6. **Failed phonemes from the Speak trainer enqueue HVPT trials** for the matching contrast —
   see [R8](08-capt-asr.md).
7. **Both members of a pair are words** (2026-09-03, [ADR-033](../DECISIONS.md#adr-033)). The
   inventory shipped a dozen nonce partners («(нет слова)») and the first learner heard them as
   "words that don't exist" — which they were. The build refuses a member the pack's own
   dictionary does not carry; contrasts with no attested pair (`ny`/`n`, `ty`/`t`) are dropped
   rather than faked.

## Hungarian contrast inventory

Vowel length (the highest-yield set for a Russian L1 speaker, who has no phonemic length):
`a/á`, `e/é`, `o/ó`, `ö/ő`, `u/ú`, `ü/ű`, `i/í`.
Sibilants/affricates: `s`, `sz`, `zs`, `cs`, `c`.
Palatals: `gy`, `ny`, `ty`, `ly`~`j`.
Gemination: `hal`/`hall`, `megy`/`meggy`, `eset`/`esett`.
Lexical pairs from the source plan §6.4: `kor/kór`, `öt/őt`, `tör/tőr`, `hat/hát`, `bal/bál`,
`szél/szel`, `agy/ágy`.

## Related

[[R8 CAPT/ASR]](08-capt-asr.md) · [[R13 shadowing]](13-shadowing.md) · [[R17 Hungarian stack]](17-hungarian-nlp-stack.md)
