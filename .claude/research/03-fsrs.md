# R3 — FSRS as the scheduler

**Confidence: Medium.** The algorithm is well-engineered and widely deployed; the headline
efficiency number comes from simulation, not a controlled trial. Say so.

## Claim

FSRS (Free Spaced Repetition Scheduler) models memory with three variables — **Difficulty,
Stability, Retrievability** — and schedules each review to land at a target recall probability.
Users can expect roughly **20–30% fewer reviews** for the same retention compared with SM-2.

## Evidence and lineage

- Descends from the **DSR model** (Woźniak) via MaiMemo's **DHP model**.
- Peer-reviewed underpinnings: *"A Stochastic Shortest Path Algorithm for Optimizing Spaced
  Repetition Scheduling"* (ACM KDD) and *"Optimizing Spaced Repetition Schedule by Capturing
  the Dynamics of Memory"* (IEEE TKDE), Ye et al., MaiMemo.
- Built into Anki since 23.10 (a toggle in deck options, not silently default).
- Algorithm reference — https://github.com/open-spaced-repetition/fsrs4anki/wiki/The-Algorithm
- Optimal retention discussion — https://github.com/open-spaced-repetition/fsrs4anki/wiki/The-optimal-retention
- Awesome-FSRS (implementations, papers) — https://github.com/open-spaced-repetition/awesome-fsrs

### Desired retention

Permissible range 0.70–0.99. Practical consensus: **0.85–0.90**. Below ~0.80 items decay before
consolidating; above ~0.95 workload grows steeply for little gain. FSRS can optimise either for
minimum workload or for maximum knowledge per unit of work — these give different answers, and
the knowledge-per-work objective is the more useful one for a learner on a deadline.

## What this changes in our code

1. **We vendor, we do not invent.** `internal/srs/` wraps
   [`github.com/open-spaced-repetition/go-fsrs/v4`](https://github.com/open-spaced-repetition/go-fsrs),
   which implements **FSRS-6**. The source plan's rule "никакого своего SRS-алгоритма" is
   honoured in substance: we write no scheduling maths of our own.
2. **Default desired retention 0.90**, user-adjustable, clamped 0.80–0.95, with the workload
   consequence shown as a curve before the change is applied
   (`internal/srs/forecast.go` → Settings → Review).
3. **`review_log` stores the full FSRS log shape** (`rating, state, due, stability, difficulty,
   elapsed_days, last_elapsed_days, scheduled_days, review_time, duration_ms`) so that:
   - a future optimizer can consume it without a migration,
   - replaying the log reproduces stored state exactly (this is an actual property test),
   - `.apkg` and CSV export are lossless.
4. **We run FSRS over two state spaces, not one** — cards *and* morphological features. See
   `internal/srs/emit.go`. FSRS itself is unmodified; we simply instantiate it twice.

## The honest caveat, stated in the UI

> The "20–30% fewer reviews" figure comes from **simulation over historical review logs**, not
> from a controlled experiment with live learners. It is a strong engineering result and a weak
> scientific one. We use FSRS because it is the best available scheduler, not because that
> number is proven.

Additionally: **`go-fsrs`'s optimizer sub-package is `in_development`.** v1.0 therefore ships
**FSRS-6 default parameters** for everyone. Per-user parameter optimisation arrives in v1.3 via
an optional sidecar running the Python/Rust optimizer over exported logs. This limitation is
surfaced in Settings → Review, not buried in docs.

## Related

[[R1]](01-retrieval-practice.md) · [[R2]](02-spacing.md) · [[R12]](12-retrieval-direction.md)
