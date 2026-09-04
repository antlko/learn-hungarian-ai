# R18 — Sets, in-session relearning, and progress the learner can see

**Confidence: Medium.** The relearning findings are solid laboratory results; the session-length
and goal-proximity findings come from adjacent fields (consumer behaviour, microlearning) and are
about *persistence*, not acquisition. Each claim below carries its own grade.

## The problem this note answers

A learner with 70 due cards is handed 70 cards. Nothing in the spacing literature says a sitting
must be that long, and everything in the motivation literature says a far-off finish line
depletes effort. The question is how to cut a day's work into pieces **without touching the
scheduler** ([ADR-003](../DECISIONS.md#adr-003)) and without building the point economy
[R14](14-motivation.md) and [R16 §3](16-debunked.md) rule out.

## Claims and evidence

### 1. Relearn to a criterion of one correct recall per session — **High**

Rawson & Dunlosky (2011, *JEP: General*, "How much is enough?") and the successive-relearning
programme that followed: practise each item to **one correct recall in every later session**
(three in the very first one), then relearn on a spaced schedule. Gains diminished after about
three relearning sessions. Karpicke & Roediger (2008, *Science*) showed the other half: items
**dropped from testing after their first correct recall** were remembered at ~33–36% a week
later, against ~80% for items kept in the retrieval cycle — repeated retrieval, not repeated
study, is what consolidates.

- Rawson & Dunlosky 2011 — https://www.semanticscholar.org/paper/ab9fe6e3290d62086cba93ed67cea421f75eba08
- Rawson & Dunlosky 2022, *Current Directions* — https://journals.sagepub.com/doi/10.1177/09637214221100484
- Karpicke & Roediger 2008 — https://pubmed.ncbi.nlm.nih.gov/18276894/

FSRS-6 already implements the spaced half (its learning steps are the relearning schedule). What
it does not define is the **session**: when a failed card returns *within the same sitting*, and
when a sitting is over. The criterion above answers both.

### 2. A near, visible goal sustains effort better than a far one — **Medium (persistence only)**

The goal-gradient hypothesis (Kivetz, Urminsky & Zheng 2006, *J. Marketing Research*): effort
rises as the goal approaches — in field experiments people rated more songs per visit and were
less likely to quit a session as the reward threshold neared, and engagement dipped right after
each reward before rising again. Endowed progress (Nunes & Drèze 2006, *J. Consumer Research*):
framing a task as *already under way* nearly doubled completion (34% vs 19%) for identical
real effort.

- Kivetz, Urminsky & Zheng 2006 — https://journals.sagepub.com/doi/abs/10.1509/jmkr.43.1.39
- Nunes & Drèze 2006 — https://academic.oup.com/jcr/article-abstract/32/4/504/1787425

Both are studies of *how long people keep going*, measured on consumers, not of what they
learn. They say nothing about acquisition and must not be cited as if they did. What they
support is narrow: a 15-card bar the learner can see the end of is more likely to be finished
than a 70-card one, and the finish of set 2 of 5 is a real event to mark rather than an
invented one.

### 3. Short, frequent sittings: positive but poorly measured — **Medium-Low**

Microlearning meta-analyses (2023–2025) report moderate-to-large effects on retention, with
sessions of 5–10 minutes several times a week as the usual recipe — and with heterogeneous
definitions (1–15 minutes), short follow-ups and weak primary studies. Treat as consistent
with sets, not as evidence for a specific size.

- Systematic review & meta-analysis, higher education — https://www.researchgate.net/publication/394265408
- Microlearning and soft skills (2025) — https://pmc.ncbi.nlm.nih.gov/articles/PMC12061706/

### 4. Rewards undermine; informational feedback does not — **High**

Deci, Koestner & Ryan (1999, *Psych. Bulletin*, 128 studies): expected tangible rewards —
engagement-, completion- and performance-contingent — undermine free-choice intrinsic
motivation (d = −0.40, −0.36, −0.28). Positive **informational** feedback — telling someone how
they did — does not, and enhances perceived competence. Hattie & Timperley (2007, 74
meta-analyses) and Wisniewski, Zierer & Hattie (2020, 435 studies, d = 0.48): feedback about the
task and about self-regulation is what moves learning; "praise, punishment and extrinsic rewards
were the least effective".

- Deci, Koestner & Ryan 1999 — https://www.semanticscholar.org/paper/8ad9801baea65b40fbbe6fc56e34b2b7be47d0ba
- Hattie & Timperley 2007 — https://journals.sagepub.com/doi/abs/10.3102/003465430298487
- Wisniewski, Zierer & Hattie 2020 — https://pmc.ncbi.nlm.nih.gov/articles/PMC6987456

This is the line between a progress display and a point economy: a number that **measures**
something (chance of recall, interval, answers right) is informational feedback; a number that
only ever goes up (XP, streak repairs, combos) is a reward. [R14]'s refusal stands; this note
says what is allowed on the other side of it.

### 5. Learners act on their judgments of learning, and those judgments can be calibrated — **Medium**

Judgments of learning (JOLs) drive study choices: when Metcalfe & Finn manipulated JOLs
independently of actual recall, study decisions followed the judgment, not the performance
(Metcalfe 2009, *Current Directions*). Calibration improves with feedback. Showing the model's
own recall estimate beside a card is feedback on the JOL — the learner sees "62%" on a word they
felt sure of.

- Metcalfe 2009, "Metacognitive judgments and control of study" — https://journals.sagepub.com/doi/abs/10.1111/j.1467-8721.2009.01628.x
- Finn, "Framing effects on metacognitive monitoring and control" — https://www.columbia.edu/cu/psychology/metcalfe/PDFs/FINN-Framing%20Effects%20on%20Metacognitive%20Monitoring%20and%20Control%20-%20813-21.pdf

The risk runs the other way too: a high figure can lower effort on an item ("it says 95%, I
know this"). Mitigation is structural — the figure is informational, the **scheduler** still
decides what is asked and when, and the answer is still typed or graded in full.

### 6. "Young" and "mature" are conventions, not findings — n/a

Anki calls a card *mature* at an interval of **21 days or more** and *young* below it. It is a
reporting convention that a decade of Anki users read fluently, not a memory threshold. We use
it because a learner who has used Anki already knows what it means, and we say so.

- Anki manual, Statistics — https://docs.ankiweb.net/stats.html

## What this changes in our code

1. **The day's queue is dealt in sets.** `stores/review.ts` cuts the served queue into sets of
   `setSize` cards — **default 15**, chosen per device on the review screen (10/15/20/30,
   `tanulom.review.setSize`), the way practice round length is ([ADR-024](../DECISIONS.md#adr-024)).
   The **server serves the same queue as before**: sets are a presentation of it, `Breakdowns`
   and the streak are untouched, and `dueNow` still has to reach zero for the day to count.
   [ADR-026](../DECISIONS.md#adr-026).
2. **A set is finished when every card in it has been answered once.** *(Amended 2026-09-03.)*
   The first version re-served a miss at the back of the set once its FSRS step had elapsed,
   and waited with a countdown when the set ran out first; the learner did not understand the
   countdown and asked for it to go. Now nothing answered in a set is re-served in it. The
   store fetches the served list again the moment a set finishes, so a miss whose step has
   elapsed is in the next set because the scheduler put it there, and one still maturing
   returns through «сегодня вернутся» — nothing is asked early, and the scheduler is still not
   consulted twice. The miss is *listed* with its answer on the set summary and on the done
   screen: within a sitting the pair is read over, not re-tested. A card answered right — even
   «Трудно» — is likewise not repeated in the set.
3. **Each card shows its own memory** (§4, §5): its stage (new / learning / young / mature /
   relearning, §6), its last outcomes as a row of marks, how many times it was right out of all
   attempts, and what each grade would schedule (`Scheduler.Preview`, labelled approximate
   because interval fuzz is seeded at answer time). After the answer: the interval it had
   against the one it has now. `srs.Memory`, attached in `internal/srs/memory.go`. The chance
   of recalling it now (`Scheduler.Retrievability`) is served and shown in the words browser,
   **not on the card** *(amended 2026-09-03)*: on a card in its learning steps it is computed
   against an interval of minutes and read «100%» over five crosses, and §5's calibration
   argument cannot survive that — a figure the learner reads as a false claim calibrates
   nothing. *(Amended 2026-09-04.)* The row is measured and was silent about what a learner
   should DO with it: «верно 0 из 9» over eight crosses reads as a score being lost, and the
   first learner asked outright whether the word's progress went «в плюс или минус» and what
   would finish it. «что это значит?» opens five sentences that answer exactly that — the
   stage's meaning, what a right answer and a miss schedule next, that a card never ends but
   returns ever more rarely, that the tally is the log and carries no «N in a row» threshold,
   and that «В корзину» is the one control that removes a word. §5's calibration argument is
   the reason this is *explanation* and not a completion bar: there is no denominator to draw,
   and drawing one would be the score §4 rules out. [ADR-039](../DECISIONS.md#adr-039).
4. **The set summary is informational feedback only** (§4): answered, right first time, came
   back, words that reached *mature* in this set, minutes. No praise line, no points, no sound
   louder than a two-note cue. The full stop still belongs to the empty queue.
5. **Today shows the day in sets** — «45 к повторению · 3 подхода по 15» — because a number the
   learner can see the end of is one they start (§2). It reads the same per-device size.
6. **Order stays oldest-first by default.** A card overdue by a month is decaying fastest and
   belongs in set one. «Вперемешку» is offered as an explicit, printed choice: the server shuffles
   with a **day-stable seed** so a reload mid-day does not reshuffle the remaining queue under the
   learner's hands, and `spreadSentences` still runs last so one sentence never repeats back to
   back. New cards keep curriculum order and still come after the due ones — reviews are a debt.

## Caveats

- No study compares 15-card and 70-card sittings on retention; the size is a usability decision
  informed by §2–§3, and it is a per-device dial rather than a setting so the learner can find
  their own.
- §2 and §3 are the weakest links and are cited only for *whether the learner keeps going*.
- §1's criterion — one correct recall per session — is met within a sitting only when the
  sitting continues past the set in which the miss happened: the one-minute step has usually
  elapsed by the next set boundary, and the refreshed list carries the card. A learner who
  stops after one set meets it at their next visit instead, which FSRS's step already
  schedules. This is a real weakening, accepted on 2026-09-03 on the learner's report that the
  in-set echo and its countdown were the reason to stop — a criterion nobody stays for is met
  by nobody.
- FSRS's learning steps are what make the between-set refresh sufficient; a scheduler without
  minute-scale steps would need a policy of its own here, which is exactly what [ADR-003]
  forbids writing.

## Related

[[R1 retrieval]](01-retrieval-practice.md) · [[R2 spacing]](02-spacing.md) · [[R3 FSRS]](03-fsrs.md) · [[R14 motivation]](14-motivation.md) · [[R16 debunked]](16-debunked.md) · [ADR-026](../DECISIONS.md#adr-026)
