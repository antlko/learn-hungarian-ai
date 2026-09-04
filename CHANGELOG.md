# Changelog

All notable changes to tanulom. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
the project uses [semantic versioning](https://semver.org/). Every design decision behind an entry
is an ADR in [`.claude/DECISIONS.md`](.claude/DECISIONS.md), and every pedagogical claim traces to a
note in [`.claude/research/`](.claude/research/README.md).

## [1.0.0] — unreleased

The first release: a self-hosted Hungarian trainer that tracks **two memories** — the words a
learner knows and the grammatical features they command — and derives everything else from them.

### Learning core
- FSRS-6 scheduling over two independent state spaces, lemmas and morphological features; one
  answer grades one card and only the features the item required (ADR-003).
- The review queue: due and new counted the way the queue serves them, sets of a per-device
  size, sibling burial across the day, one sentence never twice in a row, a start screen
  where the pile, the set size and the order are chosen (ADR-016, 021, 026, 028).
- A card shows its own memory — retrievability, interval, stability, the tally, what each
  grade would schedule — measured or computed by the library, never a score.
- Undo by delete-and-replay over the review log, bit for bit, fuzz included (ADR-020).
- Free practice that schedules nothing: rounds of a chosen size, an explicit weak-words mode
  (ADR-019, ADR-024). The trash sets a word aside, practised but never scheduled (ADR-025).
- The interleaving gate: blocked practice until rolling accuracy reaches 80 %, then
  interleaved, with hysteresis — and the rule it applied is printed on the screen.

### Material
- A language pack built from Wiktionary, Tatoeba, OpenSubtitles and frequency data, shipped
  inside the binary as a **starter edition** and installed on first boot; the full pack is an
  upgrade you build yourself (ADR-002, ADR-010). Every drill answer is attested in it.
- The reader: paste anything, see two coverage numbers, mine `n+1` words into cards with the
  sentence they came from; per-text review scope (ADR-022).
- A guided 24-unit course that ends and says so, then a catalogue of real Hungarian.
- Twenty-eight grammar topics, each a lesson before a drill; thirteen produce drills, the
  rest say «explained» and mean it.
- The ear trainer: high-variability phonetic training over thirty real-word minimal pairs,
  three CC0 voices, never marked complete (ADR-033).

### The tutor and the translator
- A grammar tutor on a local model: grounded in the pack's lesson, the feature tables, the
  learner's own mastery figures and a CC BY-SA reference corpus; it explains and mints nothing
  (ADR-027, ADR-029). It translates into Hungarian on request, word by word (ADR-030); it has
  a chat's controls — stop, edit-and-resend, a context meter (ADR-031); and it reads the review
  card: prepared questions composed by the server, and a question before the answer counts as
  a hint (ADR-034).
- Sentence translation from a dedicated NLLB model, display-only and always labelled with the
  model that wrote it; a translation in the wrong language says so, and the translator is
  offered wherever the learner's language is missing (ADR-014, 018, 035).

### The app
- One static Go binary with the Vue SPA embedded, one data directory, SQLite, no runtime
  dependencies; optional sidecars for NLP, TTS/ASR, translation and the LLM, and the app is
  fully usable with all of them stopped (ADR-001, ADR-006).
- Speech from the learner's own device by default (ADR-012); accounts with roles; Russian and
  English interfaces, both complete, always (ADR-004).
- Two typefaces — Literata for the material and the headings, Inter for the instrument — and
  five folk-embroidery line drawings on the quiet screens (ADR-036, ADR-037).
- Every failure in one voice, a shell that does not flash, docs that describe what ships, and
  the checks CI runs runnable with `make test` (ADR-038).

### Honest limits, stated in the product
- Per-user FSRS optimisation is absent; the efficiency figure is a simulation.
- Coverage thresholds are English-derived; Hungarian ASR runs at ~11 % WER; three voices only.
- Offline review is planned, not shipped. There is no export: the data directory is yours
  (ADR-011).

[1.0.0]: https://github.com/antlko/learn-hungarian-ai/releases/tag/v1.0.0
