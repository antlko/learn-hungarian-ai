# R13 — Shadowing

**Confidence: Medium.** A large systematic review, but the authors deliberately avoided
meta-analysis because the evidence is methodologically mixed.

## Claim

Shadowing — repeating speech aloud with a 1–2 word lag while continuing to listen — improves
**comprehensibility, intelligibility, accentedness, fluency and prosody**. Evidence for
**segmental** accuracy is inconclusive.

## Evidence

**"A Systematic Review of Research on the use of Shadowing for Second Language Pronunciation
Teaching" (2025)** — 44 studies after screening.

- Improves comprehensibility, intelligibility, accentedness.
- Improves **suprasegmental** control: fluency and prosody.
- **Segmental** pronunciation: **inconclusive**.
- The authors chose a systematic review over a meta-analysis *because* the evidence base is
  mixed-methods and heterogeneous — that choice is itself a signal about confidence.
- Learners consistently rate it interesting, enjoyable and effective.

Links:
- Systematic review 2025 — https://www.tandfonline.com/doi/full/10.1080/29984475.2025.2546827
- Listening effects — https://files.eric.ed.gov/fulltext/EJ1479870.pdf

## What this changes in our code

1. **Shadowing owns prosody; ASR owns segments.** This is a clean division of labour backed by
   two different literatures ([R8](08-capt-asr.md) found ASR large on segmentals, small on
   suprasegmentals; shadowing is the mirror image). The Speak and Listen views label which
   trainer targets which, so the learner is not confused about why two things look similar.
2. **The three-pass protocol is UI state, not advice.** `web-ui/src/views/ListenView.vue`
   tracks the pass explicitly:

   | Pass | Transcript | Task |
   |---|---|---|
   | 1 · Blind | hidden | listen only, mark confidence |
   | 2 · Text | shown, unknowns highlighted | listen and read, resolve gaps |
   | 3 · Shadow | shown, scrolling | repeat with 1–2 word lag, optionally recorded |

   You cannot skip to pass 3 without passing through 1 — the protocol from source plan §3
   Loop 2, made structural.
3. **A/B loop and 0.5–1.5× pitch-preserved playback** are required affordances, not extras:
   shadowing at full speed is impossible for a beginner, and pitch-shifted audio would corrupt
   the vowel quality the learner is trying to imitate. `web-ui/src/lib/audio.ts` uses
   `playbackRate` with `preservesPitch`.
4. **Shadowing recordings are stored but not auto-scored.** Given that segmental evidence is
   inconclusive and Hungarian ASR has WER ≈ 11% ([R8](08-capt-asr.md)), scoring a shadowing pass
   would manufacture false precision. Recordings go to the `recording` table for the learner's
   own comparison over time — and for their teacher, which is where the useful feedback is.

## Caveats

- Heterogeneous methods, many small studies, few controls.
- Most work is with Japanese EFL learners; transfer to a Russian L1 / Hungarian L2 pair is
  untested.
- The enjoyment finding is self-report and not evidence of learning.

## Related

[[R7 HVPT]](07-hvpt.md) · [[R8 CAPT/ASR]](08-capt-asr.md) · [[R6 coverage]](06-lexical-coverage.md)
