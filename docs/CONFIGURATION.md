# Configuration

Everything lives in one data directory, mounted at `/data` in Docker.

```
/data
├── config.yaml              # settings — hand-editable, rewritten atomically by the app
├── tanulom.db               # users, sessions, secrets, all learner data
├── langpack-hu-v1.sqlite    # the language pack (read-only; installed from the binary)
└── media/                   # HVPT clips installed on first boot; TTS cache and recordings
```

The schema is created and upgraded **automatically at start**. There is no migrate command, and
the distroless image has no shell to run one.

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `TANULOM_DATA_DIR` | `/data` | where config, database, pack and media live |
| `TANULOM_ADDR` | `:8012` | listen address (`host:port`) |
| `TANULOM_LOG_LEVEL` | `info` | `debug` · `info` · `warn` · `error` |
| `TZ` | UTC | timezone for "today": the daily new-card budget, sibling burial and the streak day roll at local midnight. Any IANA name (`Europe/Budapest`); the binary embeds tzdata so it works in the distroless image |

Timezone is env-only on purpose — it is a fact about where the server runs, not a learner
setting, and a `config.yaml` field would be a second place for the same answer.

Flags mirror these: `--data-dir`, `--addr`. Precedence: **flag > env > saved state > default**.
The resolved directory is persisted to `state.json` so a restart without the flag stays
consistent — the same pattern upmonitor uses.

## `config.yaml`

```yaml
# Settings only. The learning domain lives in SQLite; secrets never appear here.
server:
  addr: ":8012"

locale:
  ui: ru                 # ru | en
  gloss: ru              # ru | en | both
  glossFallback: true    # show the EN gloss when RU is missing, visibly labelled

review:
  desiredRetention: 0.90    # clamped 0.80–0.95            [R3]
  newPerDay: 20             # the governor may refuse to raise this
  maxReviewsPerDay: 0       # 0 = unlimited. A DAY cap, not a fetch cap: graduated-card
                            # answers already given today are subtracted, so a reload
                            # cannot hand out another full batch. Learning/relearning
                            # steps in flight are never trimmed. [ADR-021]
  dailyMinutes: 90          # feeds the governor's sustainability check
  goal: conversation        # reading | conversation | exam — weights card directions [R12]
  directions:               # per-direction enable + weight
    recognize: {enabled: true,  weight: 1.0}
    produce:   {enabled: true,  weight: 1.4}
    listen:    {enabled: true,  weight: 1.0}
    cloze:     {enabled: true,  weight: 1.6}
    spell:     {enabled: true,  weight: 1.2}   # trains a/á, ö/ő — on by default [R15]
  burySiblings: true

mining:                     # n+1 scoring weights — see ARCHITECTURE.md §5.1
  knownThreshold: 0.80      # θr: retrievability above which an element counts as known
  featureWeight: 0.70       # w_f
  targetUnknown: 1.5        # μ of the gaussian
  unknownSigma: 0.8         # σ
  minCoverage: 0.90         # element-coverage floor                    [R6]
  targetLength: [6, 14]
  weights: {sweetSpot: 1.0, coverage: 0.8, utility: 0.6, relevance: 0.5, length: 0.3, duplicate: 1.0}

drill:                      # the blocked → interleaved gate            [R5]
  gateThreshold: 0.80       # rolling accuracy required to start interleaving
  gateWindow: 20
  masteryThreshold: 0.90
  masteryWindow: 50
  hysteresis: 0.10          # fall back to blocked below gateThreshold − hysteresis
  definiteness:             # item distribution mirrors documented errors [R15]
    definiteObjectPct: 55
    demonstrativePct: 25
  harmonyLatencyMs: 2500    # harmony counts as automated below this median latency

hvpt:
  # 3, not the 5 [R7] evidences: three is every CC0 Hungarian voice that exists.
  # The gap is stated in the product, and the refusal below carries both numbers.
  # Raise it if you add human recordings — see docs/PEDAGOGY.md §2.7.
  minTalkers: 3             # a session below this is refused           [R7]
  trialsPerSession: 40
  rateJitter: 0.10

curriculum:
  focusUnlockListeningHours: 40   # word-order/focus stays locked until this [R15]
  examDate: ""                    # booked exam, YYYY-MM-DD. [R14]: the pre-booked exam is the
                                  # best defence against the month-4–7 trough; Today counts
                                  # down to it. Empty = none booked (no default on purpose)

services:                   # endpoints only — keys live in SQLite      [ADR-005]
  tts: {provider: device}   # DEFAULT — the learner's browser speaks it, no endpoint
  asr: {provider: local, endpoint: "http://asr:5001"}
  nlp: {provider: local, endpoint: "http://nlp:5002"}
  llm: {provider: off,   endpoint: "http://ollama:11434", model: "qwen3:8b"}
  mt:  {provider: off,   endpoint: "http://mt:5003"}
```

**The `llm` endpoint is the host and nothing more — no `/v1`.** `internal/llm` appends the
OpenAI paths itself (`{endpoint}/v1/chat/completions`, `{endpoint}/v1/models`), so an endpoint
written with the suffix already on it is requested as `/v1/v1/chat/completions` and the tutor
answers with a 404 under a services row that never went green.

`provider` is one of `off`, `local`, `openai` — and, **for `tts` only**, `device`.

**`llm` is what the grammar tutor and the text-level second opinion talk to.** Any
OpenAI-compatible chat endpoint: `provider: local` for Ollama, llama.cpp or vLLM on your own
machine (`endpoint: http://ollama:11434`, `model: qwen3:8b`), `provider: openai` for a hosted
API with its key stored in Settings. With it `off` the tutor panel says so and links here; the
rest of the app does not notice.

**`model` is required whenever the provider is on.** The request names it, every answer is
labelled with it, and Ollama and the hosted APIs refuse a request without one — so a blank
model is a client that can only fail, and it failed as «модель не ответила пригодным образом»
under a row that said «на связи». The row now verifies the model, not just the socket: a
reachable endpoint is asked `GET /v1/models`, the settings screen offers that list in the
**Model** field, and the row reads «укажите модель» while it is blank or «такой модели нет на
сервере» for a name the endpoint does not carry. The tutor panel and the level estimate answer
`errors.llmNoModel` in that state, with the dial named. Two things a local model needs that
this file cannot set: **a context of at least 8k** (the tutor's prompt is budgeted for one, and
Ollama's default 4k truncates it silently — the app's Ollama slider, or `OLLAMA_CONTEXT_LENGTH`),
and **no "thinking"** — a thinking model such as qwen3 spends the answer's token budget on hidden
reasoning first, so a `local` provider is sent `reasoning_effort: none`; a hosted API is left at
the vendor's default because a non-reasoning hosted model rejects the field. There is no `tutor:` block — the tutor's budgets and prompt
are code, versioned with it ([`TUTOR.md`](TUTOR.md)). The review screen's set size and order
are **per-device** choices stored in the browser, not here, for the reason
[ADR-026](../.claude/DECISIONS.md#adr-026) gives.

**`mt` takes `off` or `local` only.** It is sentence translation for the reader, served by
`services/mt` (NLLB-200-distilled-600M under CTranslate2), and it speaks its own `POST
/translate` rather than the OpenAI chat API — so `provider: openai` is rejected rather than
accepted and then failing at request time. It is a *translation* model, not a chat model, and is
deliberately a separate slot from `llm`: a 600M model dedicated to translation beats a 7–8B
general one at Hungarian→Russian, and Settings shows it as its own row. Everything it returns is
labelled as machine output with the model named, and none of it becomes a card
([ADR-014](../.claude/DECISIONS.md#adr-014)). Off by default; the reader is complete without it.

Once the container is up, the rest is **Settings → Services**: the row reports whether the model
is downloading, ready or failed (with the service's own reason), «Скачать модель» retries a
failed download, and the provider `<Select>` is the on/off switch. Neither the app nor this file
can start the container — see [ADR-015](../.claude/DECISIONS.md#adr-015). Put
`COMPOSE_PROFILES=mt` in `.env` so `docker compose up -d` includes it.

**`tts` defaults to `device`, not to `off`.** Device speech is the Web Speech API in the
learner's browser: no endpoint, nothing to deploy, and a Hungarian voice on every desktop OS.
It is the default because speech is load-bearing for a language you have to hear, and with TTS
off `srs.PlanFor` drops the `listen` and `spell` directions entirely while the reader has
nothing to play — a fresh install made no sound anywhere and showed four services reading
"off". `device` takes no endpoint and is rejected on any service other than `tts`.

The server cannot verify it, and does not pretend to: `/api/capabilities` reports `device` as
configured and **not** reachable, and the client says which voice it actually found. See
[ADR-012](../.claude/DECISIONS.md#adr-012).

Because `normalize()` only fills zero fields (rule 2 below), an install whose `config.yaml`
already says `tts: {provider: off}` — every install created before this change — keeps that
value. Switch it in **Settings → Services**. The 🔊 buttons in the reader, the drills and the
listening screen work regardless: they are pure client features and never ask the server.

There is no `langpack:` block. `autoDownload` and `releaseUrl` were removed with the download
step itself ([ADR-010](../.claude/DECISIONS.md#adr-010)): the binary carries the pack. A stale
`langpack:` block in an existing file is ignored, not an error.

### `review.accentTolerance` (default: true)

Accepts a typed answer that differs from the target only in diacritics — `viszlat` for
`viszlát` — because a plain English keyboard cannot produce the marks. Never a silent pass:
the answer rates **Hard** so it returns sooner, the diff underlines the dropped mark, the
response carries `acceptedWithAccents: true`, and in the grammar drills the error class still
routes the miss to the ear trainer. The `spell` review direction is exempt — it exists
precisely to train the diacritics being forgiven. Both defaults follow the `*bool` pattern
(`burySiblings`): the zero value must not be confusable with "off".

## Rules that prevent real bugs

1. **Mutated only via `Server.updateConfig`** — clone → mutate → validate → atomic save → swap
   under lock. Never assign to `s.cfg` directly. New fields must also be added to
   `config/clone.go`.
2. **`normalize()` fills zero fields only.** Changing a default affects **new installs and unset
   fields**; it never silently rewrites a user's stored value.
3. **Defaults live in exactly one place** — exported constants in `internal/config`, so API
   fallbacks cannot drift from what a fresh config gets.
4. **Secrets are not here.** API keys go in `tanulom.db`, are stripped from DTOs (presence
   exposed as a boolean), and are preserved on `PUT` when blank or omitted.
5. **Validation rejects, it does not clamp silently** — except for documented clamps
   (`desiredRetention`), which are reported back in the response so the UI can explain.
