# R8 — ASR-based pronunciation training needs *explicit* feedback

**Confidence: Medium-High.** One focused meta-analysis, consistent moderators.

## Claim

Automatic speech recognition improves L2 pronunciation — but the size of the effect depends
almost entirely on **what kind of feedback** the system gives. Explicit corrective feedback
nearly doubles the benefit over indirect feedback such as plain dictation.

## Evidence

**Ngo, Chen & Lai (2023), *ReCALL*** — "The effectiveness of automatic speech recognition in
ESL/EFL pronunciation: A meta-analysis" · 15 studies, 38 effect sizes, 2008–2021.

| Condition | Effect |
|---|---|
| ASR overall | **g = 0.69** (medium) |
| ASR **with explicit corrective feedback** (8 studies) | **g = 0.86** (large) |
| ASR with indirect feedback (e.g. dictation) | moderate |
| **Segmental** pronunciation | **large** |
| **Suprasegmental** pronunciation | **small** |
| Medium-to-long treatment | higher gains; short treatment ≈ no differential effect |
| Practising with peers | large; **practising alone: small** |
| Adult, intermediate learners | largely effective |

- Ngo, Chen & Lai 2023 — https://www.cambridge.org/core/journals/recall/article/effectiveness-of-automatic-speech-recognition-in-eslefl-pronunciation-a-metaanalysis/A915444CF252B61D14961D2FE733822D
- CAPT systematic review — https://www.cambridge.org/core/journals/recall/article/computerassisted-pronunciation-training-a-systematic-review/71E786F7DFC99727837909FDED7A2320

## What this changes in our code

1. **Feedback names the phoneme.** `internal/speech/feedback.go` aligns the ASR transcript to
   the target, locates the divergence, and reports it as *"you produced `ö` where `ő` was
   required — the vowel is short"* — not "try again", and not a bare similarity score.
   This is the difference between g = 0.86 and g = 0.69.
2. **Every detected segmental error enqueues the matching HVPT contrast**
   ([R7](07-hvpt.md)). Perception failure is usually upstream of production failure, so the
   remedy is perceptual training, not more repetition.
3. **We are honest about suprasegmentals.** ASR scoring is applied to segments only. Prosody,
   rhythm and intonation are routed to **shadowing** ([R13](13-shadowing.md)), which is where
   the evidence for those actually lies. The Speak view labels which is which.
4. **The "practising alone is small" finding is load-bearing.** It is one of the reasons Loop 4
   (a human) is un-completable by the app and is surfaced as irreplaceable in the Today view.
   ASR is a supplement, and the UI says so.
5. **Treatment length matters** — the app tracks cumulative Speak minutes and does not present
   a pronunciation score as meaningful below a minimum practice volume
   (`speech.min_minutes_for_score`, default 60).

## Caveats

- k = 15 is small; the explicit-CF sub-analysis rests on 8 studies.
- All primary studies are ESL/EFL. **Hungarian ASR is materially worse than English ASR** —
  `Trendency/whisper-large-v3-hu` reports WER ≈ 11.3% on Common Voice. A ~1-in-9 word error
  rate means the system will sometimes "correct" a learner who was right.
  → `internal/speech/feedback.go` requires a **confidence threshold** before reporting an error,
  and every feedback card carries a "the recogniser can be wrong — trust your teacher over this"
  note. Overclaiming here would be worse than useless.

## Related

[[R7 HVPT]](07-hvpt.md) · [[R9 corrective feedback]](09-corrective-feedback.md) · [[R13 shadowing]](13-shadowing.md) · [[R17]](17-hungarian-nlp-stack.md)
