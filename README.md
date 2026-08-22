<div align="center">

# tanulom

### Self-hosted Hungarian, built on the evidence — not on vibes.

A learning app that knows the difference between `tanulok` and `tanulom`, and tracks
**every suffix you've met** as a thing you can know, forget and relearn — separately from the
words that carry it.

One `docker run` and you are learning: the language pack is inside the binary, and a
**six-month guided course** tells you what to do until you no longer need telling.

[![CI](https://github.com/antlko/tanulom/actions/workflows/ci.yml/badge.svg)](https://github.com/antlko/tanulom/actions/workflows/ci.yml)
[![Stack](https://img.shields.io/badge/stack-Go%20%2B%20Vue%203-00ADD8?logo=go&logoColor=white)](#how-it-works)
[![License: MIT](https://img.shields.io/badge/code-MIT-blue.svg)](LICENSE)
[![Data: CC BY-SA 4.0](https://img.shields.io/badge/language%20pack-CC%20BY--SA%204.0-green.svg)](docs/LANGPACK.md)

</div>

---

## Why another language app

Anki, LingQ and Migaku all treat a **wordform as an atom**. For Hungarian that is a category
error:

> `házaimban` isn't one thing. It's `ház` + `-a-` + `-im` + `-ban` — one word plus three
> grammatical facts, each learned, forgotten and relearned on its own schedule.

tanulom runs **two independent [FSRS-6](https://github.com/open-spaced-repetition/go-fsrs)
state spaces**: one over lemmas, one over the 51 morphological features the pack teaches. That single decision is
what makes the rest possible:

- **Real `n+1` input.** `elmentem volna` has *zero* unknown words and *two* unknown features.
  Word-counting calls it easy. It isn't.
- **A weakness map for your suffixes.** *"Your inessive is solid. Your conditional sits at 0.41
  retrievability. Your definite conjugation collapses when the object is a demonstrative"* —
  which happens to be the second most common error real learners make.
- **Generation that actually fits you.** Prompts carry your real known-lemma set and the exact
  feature you're weak at.
- **Progress you can't fake.** Vocabulary and morphology are scored separately.

## Built on the research, including where it disagrees with us

Every feature traces to a citation in [`.claude/research/`](.claude/research/README.md) — 18
notes, each with the effect size, the caveats, and a *"what this changes in our code"* section.

Three places where we follow the evidence instead of the intuition:

| Intuition | What the evidence says | What we do |
|---|---|---|
| Interleave grammar from the start — desirable difficulty! | Interleaving is **g = 0.42** overall and **attenuated for rule-based language material**; blocked practice first is *required* below a competence floor ([Brunmair & Richter 2019](https://www.psychologie.uni-wuerzburg.de/fileadmin/06020400/2019/Brunmair_Richter_in_press__2019_META-ANALYSIS_OF_INTERLEAVED_LEARNING.pdf); [Hwang 2025](https://onlinelibrary.wiley.com/doi/10.1111/lang.12659)) | Blocked until 80% rolling accuracy, **then** interleaved — and the app tells you why |
| Ear training is a one-and-done fix | HVPT works (**g = 0.92**) but **decays at 1–2 months** ([Uchihara et al. 2025](https://www.cambridge.org/core/journals/studies-in-second-language-acquisition/article/high-variability-phonetic-training-hvpt-a-metaanalysis-of-l2-perceptual-training-studies/6ABB8C1F32D88D53EA8D05A4565E76F6)) | No contrast is ever "complete" — they come back on schedule, forever |
| Just tell the learner the right answer | Feedback that makes you **produce** the repair beats being handed it ([Li 2010](https://onlinelibrary.wiley.com/doi/abs/10.1111/j.1467-9922.2010.00561.x)) | Locate → hint → reveal, and the reveal is gated server-side until you've tried |

Full map: [`docs/PEDAGOGY.md`](docs/PEDAGOGY.md).

## What's in it

- 🗺️ **A plan** — twenty-four units over six months, each naming the errand it buys you ("I can
  register at a GP"), the grammar it leans on and the words to own. A unit is done when its
  words are actually in your deck — a fact, not a button you press. Then it **stops**, and hands
  you a curated catalogue of real Hungarian to read and listen to. The hand-off is the point:
  a course that never ends produces a learner who can only read material made for them.
- 🧠 **Review** — FSRS-6, five independently scheduled directions per note (recognise, produce,
  listen, cloze, spell), with a **morpheme-level diff** that highlights the wrong suffix rather
  than the whole word.
- 📖 **Read** — paste a URL, `.srt` or `.epub`; every token analysed, coloured by what you know,
  with live coverage and an `n+1` rail.
- 🎧 **Listen** — three-pass protocol (blind → text → shadow) as enforced state, A/B loop,
  pitch-preserved 0.5–1.5×, dictation with character diff.
- 👂 **Ear** — multi-talker minimal-pair training, because talker variability is the mechanism,
  not a garnish. A session with too few distinct voices is **refused with the numbers**.
- 📐 **Grammar** — explicit rule first, then drills, gated blocked→interleaved, ordered by what's
  actually hard about Hungarian (harmony → possessives → past → **definite conjugation** →
  conditional → … → word-order focus, last).
- 🗣️ **Speak** *(v1.1)* — record, get **explicit segmental feedback** naming the phoneme, and
  have the failed contrast queued for ear training automatically.
- ✍️ **Write** *(v1.1)* — correction that makes you find the error first.
- 📊 **Progress** — the lemma curve, the feature heatmap, and the 3×3 locative grid coloured by
  mastery.
- 💾 **Yours** — one self-contained data directory, on your machine. No cloud, no telemetry,
  no account with us. Back it up by copying it.

## Quick start

```bash
docker run -d --name tanulom \
  -p 8012:8012 \
  -v tanulom-data:/data \
  --restart unless-stopped \
  ghcr.io/antlko/tanulom:latest
```

> The image is published by the first tagged release. Until then, clone the repo and run
> `docker compose up -d` with `build: .` uncommented, or `make build` for the bare binary.

Open **http://localhost:8012**, create your account, and start. **There is nothing to
download**: the binary carries a starter language pack — the whole curriculum and the 24-unit
course, 5,132 words with their 502,480 forms, glosses and paradigms, 12,000 sentences it can
fully explain, and the ear trainer's 384 clips — and installs it on first boot. It is enough to work through the
first months for real. The full 252 MB pack is an upgrade that widens the vocabulary; you build
it yourself with `make langpack` if and when you want it.

Add other people from **Accounts** (admin only) if a household or a tutoring group shares one
install: everyone signs in separately and keeps their own cards, texts and progress.

**That's the whole product.** No sidecars needed. Want speech or a local LLM?

```bash
docker compose --profile speech --profile llm up -d
```

See [`docs/SELFHOSTING.md`](docs/SELFHOSTING.md) for reverse proxy, backup and sizing.

### Or from source, without Docker

```bash
git clone https://github.com/antlko/tanulom.git
cd tanulom
make deps    # Go modules, npm packages, the pipeline's Python requirements
make dev     # backend and Vite together — open http://localhost:5173
```

`make` on its own lists every target. `make build` produces the same single static binary the
image ships. Details in [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md).

## How it works

One static Go binary embeds the built Vue app and serves both the API and the UI.

- **Backend** — Go with [Fiber v3](https://gofiber.io), pure-Go SQLite
  ([`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite), no CGO) with
  [goose](https://github.com/pressly/goose) migrations applied on start,
  [FSRS-6](https://github.com/open-spaced-repetition/go-fsrs), bcrypt + cookie sessions,
  structured `slog`.
- **Frontend** — Vue 3 + TypeScript, Pinia, Tailwind v4, shadcn-vue, `vue-i18n` (RU + EN), PWA
  for offline review.
- **Language pack** — a versioned read-only SQLite file built offline from Wiktionary, Tatoeba
  and frequency data by a Python pipeline with its own vowel-harmony and inflection engines. The
  runtime does indexed lookups and needs **no Python at all**; a
  [HuSpaCy](https://github.com/huspacy/huspacy) sidecar is an optional accuracy upgrade, never a
  requirement. [`docs/LANGPACK.md`](docs/LANGPACK.md).

Tiny, fast, `amd64` + `arm64` — it runs on a NAS or a Raspberry Pi.

## Honest about its limits

Stated in the app, not buried here: FSRS's efficiency figure comes from simulation, not a trial;
per-user FSRS optimisation isn't in v1.0; the 95/98% coverage thresholds were derived for
English; Hungarian ASR has ~11% word error rate, so it will sometimes contradict you; and the
ear trainer ships **three** synthetic voices where the research asks for five or more, because
three is how many freely-licensed Hungarian voices exist — the Ear screen says so permanently
rather than counting playback rates as extra talkers.

Not every grammar topic has drills behind it. The ones that don't say *"we have not built
exercises for this"* and teach by explanation, deliberately **without** gating the topics that
come after them — a rule we have not built exercises for must not wall off the rest of the
curriculum. And there is no export: your data is a directory you own outright, which is a
stronger guarantee than a file format we promise to keep writing.

And the big one: **this app cannot replace a teacher.** A model has no social stakes and
understands your broken Hungarian far too well. The one loop the app refuses to tick off for you
is the one with a human in it.

## Documentation

[Architecture](docs/ARCHITECTURE.md) · [Pedagogy](docs/PEDAGOGY.md) ·
[Language pack](docs/LANGPACK.md) · [UX](docs/UX.md) · [API](docs/API.md) ·
[Configuration](docs/CONFIGURATION.md) · [Development](docs/DEVELOPMENT.md) ·
[Self-hosting](docs/SELFHOSTING.md) · [Research library](.claude/research/README.md) ·
[Decisions](.claude/DECISIONS.md)

## Licence

Code: [MIT](LICENSE). The language pack is derived from CC BY-SA sources (Wiktionary, Tatoeba,
Hungarian Webcorpus) and is released under **CC BY-SA 4.0** — see
[`langpack/LICENSES.md`](langpack/LICENSES.md) for the full attribution ledger.
