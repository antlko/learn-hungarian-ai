# R14 — Motivation, streaks and gamification

**Confidence: Medium-Low.** Consistent findings on affect, weak and contested findings on
learning outcomes.

## Claim

Gamification reliably improves **motivation, perceived autonomy and relatedness** — and has
**minimal impact on competence**. Streaks drive engagement and retention. None of this
demonstrates language acquisition.

## Evidence

- A 2024 meta-analysis: gamification enhances **intrinsic motivation, autonomy and
  relatedness**, but shows **minimal impact on competency**.
- Duolingo-user study (n = 247): gamification features influence motivation mainly through
  **competence and relatedness** channels (Self-Determination Theory framing).
- Digital game-based learning (2024, Chinese EFL): enhanced enjoyment and ideal-L2-self;
  pre-existing autonomy predicted enjoyment — i.e. partly a selection effect.
- Streak research (Oulu 2025): streaks boost motivation and attention toward goal completion.
- **The critical finding:** a systematic review of the Duolingo literature concluded we still
  know far too little about how effective it is for sustainable acquisition, and that the
  research is **often methodologically weak**.

Links:
- Duolingo literature review — https://www.tandfonline.com/doi/full/10.1080/09588221.2021.1933540
- Streaks & motivation (Oulu 2025) — https://oulurepo.oulu.fi/bitstream/handle/10024/54117/nbnfioulu-202502121605.pdf
- Gamification & motivation — https://www.frontiersin.org/journals/psychology/articles/10.3389/fpsyg.2024.1295709/full
- DGBL & Chinese EFL (2024) — https://www.nature.com/articles/s41599-024-04073-3

## What this changes in our code

The source plan is blunt about this (§1.2): gamified apps as a *primary* method are a
"гарантированный плато на A2". We take the motivational mechanics and refuse the substitution.

1. **The streak counts *completed obligations*, not activity.** A day counts when the learner
   clears their due reviews and meets their stated minutes — not when they tap something.
   `internal/stats/streak.go`. A streak you can keep by opening the app for 30 seconds is a
   streak that measures nothing.
2. **No XP, no leaderboards, no leagues, no gems, no lootboxes.** Competence is where the
   evidence is null, and extrinsic point economies are the part most likely to crowd out the
   real goal.
3. **We lean on the channels that do have support** — competence and relatedness:
   - **Competence:** the morph mastery heatmap and lemma curve show *real* capability growth,
     tied to measurable checkpoints. This is a competence signal with substance behind it.
   - **Relatedness:** Loop 4 (a human teacher or tandem partner) and the weekly real-world task.
     The app schedules and protects these rather than simulating them.
4. **Checkpoints replace levels.** M1/M3/M6/M12/M18 are *assessments* with pass/fail — recorded
   monologue, timed reading, dictation accuracy, ECL-format mock sections. A failed checkpoint
   blocks curriculum advance and opens a diagnostic. This is the source plan's "правило отката"
   made structural.
5. **The motivational trough is anticipated in the product.** The source plan warns of collapse
   at months 4–7 and names the best-known defence: a pre-booked, pre-paid exam. The app tracks
   an exam date as a first-class object and counts down to it.

## Caveats

- Much of this literature is short-term engagement measurement, not learning measurement.
- Self-report dominates.
- Selection effects are pervasive (people who like gamified apps use gamified apps).

## Related

[[R10 LLM tutors]](10-llm-tutors.md) · [[R16 debunked]](16-debunked.md) · [[00 source plan §1.2]](00-source-plan.md)
