# Configuration

Everything lives in one data directory, mounted at `/data` in Docker.

```
/data
├── config.yaml              # settings — hand-editable, rewritten atomically by the app
├── tanulom.db               # users, sessions, secrets, all learner data
├── langpack-hu-v1.sqlite    # the language pack (read-only; downloaded or baked in)
├── media/                   # TTS cache, HVPT clips, learner recordings
└── backups/                 # automatic snapshots taken before an import
```

The schema is created and upgraded **automatically at start**. There is no migrate command, and
the distroless image has no shell to run one.

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `TANULOM_DATA_DIR` | `/data` | where config, database, pack and media live |
| `TANULOM_ADDR` | `:8080` | listen address (`host:port`) |
| `TANULOM_LOG_LEVEL` | `info` | `debug` · `info` · `warn` · `error` |

Flags mirror these: `--data-dir`, `--addr`. Precedence: **flag > env > saved state > default**.
The resolved directory is persisted to `state.json` so a restart without the flag stays
consistent — the same pattern upmonitor uses.

## `config.yaml`

```yaml
# Settings only. The learning domain lives in SQLite; secrets never appear here.
server:
  addr: ":8080"

locale:
  ui: ru                 # ru | en
  gloss: ru              # ru | en | both
  glossFallback: true    # show the EN gloss when RU is missing, visibly labelled

review:
  desiredRetention: 0.90    # clamped 0.80–0.95            [R3]
  newPerDay: 20             # the governor may refuse to raise this
  maxReviewsPerDay: 0       # 0 = unlimited
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
  minTalkers: 5             # a session below this is refused           [R7]
  trialsPerSession: 40
  rateJitter: 0.10

curriculum:
  focusUnlockListeningHours: 40   # word-order/focus stays locked until this [R15]

services:                   # endpoints only — keys live in SQLite      [ADR-005]
  tts: {provider: local, endpoint: "http://tts:5000",  voices: [anna, berta, imre]}
  asr: {provider: local, endpoint: "http://asr:5001"}
  nlp: {provider: local, endpoint: "http://nlp:5002"}
  llm: {provider: off,   endpoint: "http://ollama:11434/v1", model: "qwen3:8b"}

langpack:
  autoDownload: true
  releaseUrl: ""            # blank = the project's default GitHub Releases
```

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
