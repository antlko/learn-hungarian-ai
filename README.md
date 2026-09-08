<div align="center">

# tanulom

### Self-hosted Hungarian, built on the evidence — not on vibes.

A learning app that knows the difference between `tanulok` and `tanulom`, and tracks
**every suffix you've met** as a thing you can know, forget and relearn — separately from the
words that carry it.

One `docker run` and you are learning: the language pack is inside the binary, and a
**six-month guided course** tells you what to do until you no longer need telling.

[![CI](https://github.com/antlko/learn-hungarian-ai/actions/workflows/ci.yml/badge.svg)](https://github.com/antlko/learn-hungarian-ai/actions/workflows/ci.yml)
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

Every feature traces to a citation in [`.claude/research/`](.claude/research/README.md) — 20
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
  than the whole word. The day's queue is dealt in **sets of 15** (your size, your order), a
  miss comes back at the end of its set once its learning step has elapsed, and every card shows
  **its own memory** — chance of recall, stage, the last outcomes, what each grade would
  schedule. Measured figures, no points.
- 🎓 **A grammar tutor** — a chat in your language, under every lesson and on its own page,
  answered by a local or hosted model that has the lesson, the pack's feature tables and a
  corpus of open grammars in front of it. It translates into Hungarian on request — the
  sentence, then word by word — names the model on every answer, and can be stopped, edited
  and asked straight from a review card. Ask about anything Hungarian: the app finds the
  lesson for the question and says which one it used. It names the model, flags any Hungarian
  word the dictionary does not carry, and cannot make cards.
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

## What it looks like

Light theme, Russian interface; the English one is complete too (ADR-004).

| | |
|---|---|
| ![Today — the four loops of the day, the obligation bar and the streak](docs/screenshots/today.png) | ![A review card after the answer, with the tutor's breakdown beside it](docs/screenshots/review.png) |
| *Today: the day's four loops, one obligation bar, a streak that counts obligations met.* | *A card after the answer — the verdict, the sentence, and the tutor reading the card.* |
| ![The reader — two coverage numbers over a pasted text, one word opened](docs/screenshots/reader.png) | ![A grammar lesson — the topic is explained before it is drilled](docs/screenshots/grammar.png) |
| *The reader: two coverage numbers, a word opened for its analysis.* | *Grammar: the lesson first, the drill behind a button.* |
| ![The ear trainer — one talker, one minimal pair](docs/screenshots/ear.png) | |
| *The ear trainer: a real word, two candidates, three voices.* | |

## Quick start

```bash
docker run -d --name tanulom \
  -p 8012:8012 \
  -v tanulom-data:/data \
  --restart unless-stopped \
  ghcr.io/antlko/learn-hungarian-ai:latest
```

Open **http://localhost:8012**, create your account, and start. **There is nothing to
download**: the binary carries a starter language pack — the whole curriculum and the 24-unit
course, 5,059 words with their 517,837 forms, glosses and paradigms, 11,400 sentences it can
fully explain, and the ear trainer's 342 clips — and installs it on first boot. It is enough to
work through the first months for real. The full 252 MB pack is an upgrade that widens the
vocabulary; you build it yourself with `make langpack` if and when you want it.

Add other people from **Accounts** (admin only) if a household or a tutoring group shares one
install: everyone signs in separately and keeps their own cards, texts and progress.

**That is the whole product.** Every sidecar below is optional, and the app tells you on screen
which ones it cannot reach and what that costs you.

---

## Installing it properly

The long version: what to run, what it downloads, what listens where, and which parts you have
to fetch yourself. If you only want it working, do §1 and §2 and stop.

### 1. What you need first

| Path | Requires |
|---|---|
| **Docker** (recommended) | Docker Engine 20.10+ with the Compose v2 plugin (`docker compose`, not `docker-compose`). Docker Desktop on macOS/Windows includes both. |
| **From source** | Go **1.26+**, Node **22.18+ or 24.12+** (the `web-ui/package.json` `engines` floor), and a C-free build — nothing else. `CGO_ENABLED=0`, so no compiler toolchain. |
| **Rebuilding the language pack** *(optional, rare)* | Python 3.11, ~40 GB of scratch space, and hours. Almost nobody needs this. |

Disk: **~100 MB** for the app and its data. Each optional service is a lot more — see §5.
It runs on `amd64` and `arm64`, which means a NAS or a Raspberry Pi is a real target.

### 2. Deploying it

#### Option A — one container

The Quick start above. Nothing else is involved: the image is a single static binary plus an
empty `/data`, and the starter pack unpacks itself into the volume on first boot.

```bash
docker pull ghcr.io/antlko/learn-hungarian-ai:latest
docker run -d --name tanulom -p 8012:8012 -v tanulom-data:/data \
  --restart unless-stopped ghcr.io/antlko/learn-hungarian-ai:latest
```

> **The image exists only after a `v*` tag is pushed.** `.github/workflows/docker-publish.yml`
> first runs the whole CI gate against the tagged commit and only then builds `linux/amd64` and
> `linux/arm64`, pushing `latest`, `1.2.3`, `1.2`, `1`, `v1.2.3` and `sha-abc1234`. A red test
> means no image, rather than a broken `latest`. Before the first release, build it yourself —
> see Option C.
>
> **Check the package is public after the first tag.** A GHCR package can land private even
> when the repository is public; the `image.source` label in the `Dockerfile` is what links it
> back to the repo. Open *repo → Packages → `learn-hungarian-ai` → Package settings*, and if
> visibility says Private, change it to Public. While it is private, a `docker pull` from any
> machine that is not logged in fails with `denied`, which looks exactly like a typo in the
> image name.

#### Option B — Docker Compose (what you want if you will ever add a sidecar)

**The compose file is [`docker-compose.yml`](docker-compose.yml) in the repository root, and it
is the one you deploy.** There is no second file to generate and nothing to template. You need
exactly two files on the host:

```bash
mkdir tanulom && cd tanulom
curl -fsSLO https://raw.githubusercontent.com/antlko/learn-hungarian-ai/main/docker-compose.yml
curl -fsSL  -o .env https://raw.githubusercontent.com/antlko/learn-hungarian-ai/main/.env.example
docker compose up -d
```

(Or just `git clone` the repo and run `docker compose up -d` inside it — same thing.)

**`.env` selects which optional services come up.** It sits beside the compose file and Compose
reads it automatically. Copied from [`.env.example`](.env.example), it holds one line:

```bash
COMPOSE_PROFILES=mt,llm      # comma-separated (nlp, speech: not implemented — see §5)
```

Set it once rather than typing `--profile mt` on every command. Forgetting the flag on a
redeploy silently takes the translator away, and the app then correctly reports it unreachable —
which reads like a fault rather than a missing flag.

**The app's own settings are not in this file.** `config.yaml` is written into the data volume on
first boot and edited through **Settings** in the UI; the only knobs the compose file carries are
the ones Docker owns:

| Variable | Default | What it does |
|---|---|---|
| `TANULOM_DATA_DIR` | `/data` | Database, `config.yaml`, the language pack, `media/`. Set in the image; leave it. |
| `TANULOM_ADDR` | `:8012` | Listen address **inside** the container. Change the left-hand host port instead. |
| `TANULOM_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. JSON to stdout. |
| `TZ` | UTC | **Set this.** The daily new-card budget, sibling burial and the streak all roll over at *local* midnight. Unset, your day ends at UTC midnight. The binary embeds tzdata, so `Europe/Budapest` just works. |

Precedence for the address is `--addr` > `TANULOM_ADDR` > `config.yaml` > `:8012`.

#### Option C — build it yourself

Either because there is no published image yet, or because you would rather not pull one.

```bash
git clone https://github.com/antlko/learn-hungarian-ai.git
cd learn-hungarian-ai
docker build -t tanulom .          # ~2 min: builds the SPA, then the static binary
docker run -d --name tanulom -p 8012:8012 -v tanulom-data:/data tanulom
```

To make Compose build instead of pull, comment out `image:` in `docker-compose.yml` and
uncomment the `build: .` line directly under it.

#### Option D — no Docker at all

```bash
git clone https://github.com/antlko/learn-hungarian-ai.git
cd learn-hungarian-ai
make build            # SPA -> embedded -> one static binary at bin/tanulom
./bin/tanulom --data-dir ./data
```

`bin/tanulom` is self-contained: no runtime, no shared libraries, no Python. Copy it to a server
with its data directory and it runs. For development instead, `make dev` runs the Go API and
Vite together with hot reload — open **http://localhost:5173**, where `/api` and `/media` are
proxied to Go on `:8012`. `make deps` fetches Go modules, npm packages and the pipeline's
Python requirements in one go; `make` on its own lists every target, and
[`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) covers the rest.

### 3. Ports

**Exactly one port is published to the host: `8012`.** Everything else is compose-internal, on
the private network Compose creates, reachable by service name and not from outside the host.

| Port | Service | Published? | Reached as |
|---|---|---|---|
| **8012** | tanulom — API **and** UI from one binary | **yes**, `-p 8012:8012` | `http://localhost:8012` |
| 11434 | Ollama (`llm` profile) | no — `expose` only | `http://ollama:11434` |
| 5003 | translator (`mt` profile) | no | `http://mt:5003` |
| 5002 | HuSpaCy (`nlp`) — *not implemented* | no | `http://nlp:5002` |
| 5001 | Whisper ASR (`speech`) — *not implemented* | no | `http://asr:5001` |
| 5000 | Piper TTS (`speech`) — *not implemented* | no | `http://tts:5000` |
| 5173 | Vite dev server — `make dev` only | dev only | `http://localhost:5173` |

Changing the app's port means changing the **left** side only: `-p 9000:8012`, or
`'9000:8012'` in the compose file. The container's own `:8012` is what
`internal/config.DefaultAddr`, `DEV_ADDR`, `vite.config.ts` and the `Dockerfile` all agree on,
and they are meant to stay agreed.

Putting this on the open internet is a reverse-proxy job — TLS, and a real hostname.
[`docs/SELFHOSTING.md`](docs/SELFHOSTING.md) has the Caddy and nginx blocks, plus backup and
sizing.

### 4. What downloads, and when

| Thing | Size | Who fetches it |
|---|---|---|
| The app image | ~40 MB | `docker pull` / `docker build` |
| **Starter language pack + 342 ear-training clips** | ~16 MB, **already inside the binary** | **Nobody. Installed on first boot.** |
| Ollama runtime | ~1.5 GB image | Compose, on `--profile llm` |
| **An LLM for the tutor** | **~5 GB for `qwen3:8b`** | **You — see §5.1. Nothing pulls a model for you.** |
| Translator weights (NLLB-200-distilled-600M) | ~2.4 GB downloaded, ~600 MB kept | The `mt` image build, automatically |
| Full language pack | 252 MB output, tens of GB scratch, hours | You, `make langpack` — optional upgrade |
| Piper voice models | 180 MB (3 × 60 MB) | You, `make voices` — only to re-render ear clips |

The two that matter: **the app needs nothing**, and **the tutor's model is the one thing you
must pull by hand**.

### 5. The optional services

None of them are required. The app probes each one at runtime and disables the feature it
backs, naming the reason on screen. Configure every one of them in **Settings → Services** after
the container is up — endpoints live in `config.yaml`, and any API key lives in SQLite and never
in YAML.

#### 5.1 A local model for the grammar tutor (Ollama)

Two ways in. **Inside Compose** is the simpler one — the compose file already declares the
service, gives it a volume so pulled models survive a recreate, and sets the context length the
tutor needs:

```bash
# 1. bring Ollama up (already in COMPOSE_PROFILES if you copied .env.example)
docker compose --profile llm up -d

# 2. pull a model — THIS IS THE STEP NOBODY DOES FOR YOU. ~5 GB.
docker exec tanulom-ollama ollama pull qwen3:8b

# 3. check it answers
docker exec tanulom-ollama ollama list
```

Then **Settings → Services → Language model**:

| Field | Value |
|---|---|
| Provider | `local` |
| Endpoint | `http://ollama:11434` — **the host only, no `/v1`.** The client appends `/v1/chat/completions` itself; write the suffix in and it requests `/v1/v1/…` and 404s under a row that never goes green. |
| Model | `qwen3:8b` |

Save the **endpoint first**: the row then asks the server what it serves (`GET /v1/models`) and
offers those names in the Model field. A blank model is a client that can only fail — Ollama
refuses a request without one — so the field is required and verified against that list.

**Or run Ollama on the host instead** (the native macOS/Windows app, or `curl -fsSL
https://ollama.com/install.sh | sh` on Linux):

```bash
ollama pull qwen3:8b
OLLAMA_CONTEXT_LENGTH=8192 ollama serve
```

The endpoint is then `http://host.docker.internal:11434` from a container on Docker Desktop, or
`http://localhost:11434` if you are running tanulom from source. You must also make the host
Ollama listen beyond loopback (`OLLAMA_HOST=0.0.0.0`) for a container to reach it.

**Two things the app cannot set for you:**

- **Context length: 8k or more.** Ollama ships at **4k** and truncates a longer prompt
  *silently, from the head* — which is exactly where the tutor puts the lesson the answer was
  supposed to be grounded in. Measured: a 16,000-character system message reported back as 2,050
  prompt tokens. The compose file sets `OLLAMA_CONTEXT_LENGTH=8192`; running Ollama yourself,
  set it too (the desktop app has a slider).
- **Model size: 7–8B is the floor**, not a preference. Measured 2026-09-03: `qwen3:1.7b` asked
  to translate one ordinary Russian sentence produced two different non-sentences, and with the
  app's own prompt printed the lesson's example table instead of translating. A model that size
  cannot do this task and no prompt fixes it. The app labels every answer with the model that
  wrote it and draws a line under any tag below 7B.

Expect, on a laptop CPU with a 7–8B model: first token in a few seconds, eight sentences in
thirty to sixty. Thinking is switched **off** by the app, not by you — a reasoning model would
otherwise spend the answer's whole token budget on hidden deliberation.

A hosted API works too: provider `openai`, its endpoint, its key — the key is stored in SQLite,
stripped from every API response, and never written to `config.yaml`.
[`docs/TUTOR.md`](docs/TUTOR.md) is the full account of what the tutor reads and what it may not do.

#### 5.2 Sentence translation in the reader (`mt`)

Answers "what does this line actually say" on your own CPU instead of in someone else's chat
window. NLLB-200-distilled-600M, built from `services/mt` in this repository.

```bash
docker compose --profile mt up -d      # first build downloads and converts ~2.4 GB — slow once
```

Settings → Services → translation: provider `local`, endpoint `http://mt:5003`. Running from
source instead, `make mt` serves it on `http://localhost:5003`.

The weights are CC-BY-NC, which is why this is opt-in and not in the published image. Machine
translation here is **display-only and always labelled with the model that wrote it** — it never
fills a card's answer unless you explicitly press «Принять как своё».

#### 5.3 Not implemented yet: `speech` and `nlp`

`services/tts`, `services/asr` and `services/nlp` contain a README and nothing else. The
profiles are declared in the compose file so the intended topology is visible, but
**`docker compose --profile speech up -d` will fail** — there is no Dockerfile to build.

This costs you nothing today: **`tts` defaults to the `device` provider**, the Web Speech API in
your own browser, so audio works everywhere with no endpoint and nothing deployed. Ear training
uses the 342 clips shipped inside the binary.

### 6. First run

The first page is the setup wizard: it creates the administrator account (password floor of ten
characters — a length, not a composition rule) and asks for the interface language and the gloss
language. Everything then lives under the one volume:

```
/data
├── tanulom.db                  accounts, cards, review log, progress, secrets
├── config.yaml                 written on first boot; edited through Settings
├── langpack-hu-v1.sqlite       the read-only language pack (starter edition at first)
└── media/                      the ear trainer's clips
```

**Back up that volume and you have backed up everything.** There is no separate export, on
purpose: the data directory is yours outright, which is a stronger guarantee than a file format
we promise to keep writing ([ADR-011](.claude/DECISIONS.md#adr-011)).

```bash
docker run --rm -v tanulom-data:/data -v "$PWD:/out" alpine \
  tar czf /out/tanulom-backup.tar.gz -C /data .
```

### 7. Upgrading

```bash
docker compose pull && docker compose up -d      # or: docker pull … && docker rm -f … && docker run …
```

Migrations are embedded and run inside `db.Open` on start — there is no migrate command and
nothing to remember. Your data volume is untouched.

### 8. When it does not work

| Symptom | Cause |
|---|---|
| `docker pull` says `denied` / `not found` | No `v*` tag has been pushed yet, or the GHCR package is still private. See Option A. |
| Container exits: `mkdir /data/…: permission denied` | A `/data` bind mount owned by the wrong user. The image runs as `nonroot` (uid 65532); named volumes inherit the right owner, a host directory does not — `chown 65532:65532` it, or use a named volume. |
| Blank page after an update | A stale cached `index.html`. Hard-reload once. |
| `docker compose --profile speech up` fails to build | Expected — not implemented. §5.3. |
| Services row says «на связи» but the tutor answers nothing useful | The model is too small, or Ollama's context is still 4k. §5.1. |
| The tutor 404s | `/v1` written into the endpoint. §5.1. |
| Your day rolls over at the wrong hour | `TZ` is unset, so the container is on UTC. §2, Option B. |


## How it works

One static Go binary embeds the built Vue app and serves both the API and the UI.

- **Backend** — Go with [Fiber v3](https://gofiber.io), pure-Go SQLite
  ([`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite), no CGO) with
  [goose](https://github.com/pressly/goose) migrations applied on start,
  [FSRS-6](https://github.com/open-spaced-repetition/go-fsrs), bcrypt + cookie sessions,
  structured `slog`.
- **Frontend** — Vue 3 + TypeScript, Pinia, Tailwind v4, hand-rolled primitives in the shadcn
  idiom, `vue-i18n` (RU + EN). Offline review is planned, not shipped.
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
[Language pack](docs/LANGPACK.md) · [UX](docs/UX.md) · [API](docs/API.md) · [Tutor](docs/TUTOR.md) ·
[Configuration](docs/CONFIGURATION.md) · [Development](docs/DEVELOPMENT.md) ·
[Self-hosting](docs/SELFHOSTING.md) · [Research library](.claude/research/README.md) ·
[Decisions](.claude/DECISIONS.md) · [Changelog](CHANGELOG.md)

## Licence

Code: [MIT](LICENSE). The language pack is derived from CC BY-SA sources (Wiktionary, Tatoeba,
Hungarian Webcorpus) and is released under **CC BY-SA 4.0** — see
[`langpack/LICENSES.md`](langpack/LICENSES.md) for the full attribution ledger.
