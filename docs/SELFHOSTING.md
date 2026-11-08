# Self-hosting

One container, one volume. Optional services when you want speech or an LLM.

## Quick start

```bash
docker run -d --name tanulom \
  -p 8012:8012 \
  -v tanulom-data:/data \
  --restart unless-stopped \
  ghcr.io/antlko/learn-hungarian-ai:latest
```

Open **http://localhost:8012** and create your account. **Nothing is downloaded**: the binary
carries a starter language pack and installs it into `/data` on first boot, audio included. The
252 MB full pack is an upgrade you build yourself with `make langpack` — offered, never
required. See [ADR-010](../.claude/DECISIONS.md#adr-010).

> **Until the first `v*` tag is pushed there is no image at that address.** `docker-publish.yml`
> runs the CI gate against the tagged commit and then builds `amd64` and `arm64` into GHCR, so
> a failing test publishes nothing; before the first tag, build it yourself from a clone —
> `docker build -t tanulom .` — and use `tanulom` in place of the GHCR name above. The package
> also has to be made public once in the GitHub UI, or pulls from another machine are refused.

### The first run

The first page is the setup wizard: it creates the administrator account (password floor of
ten characters — a length, not a composition rule) and asks for the interface language and the
gloss language. Everything lives under the one volume: `tanulom.db` (accounts, progress, review
log), `config.yaml`, `langpack-hu-v1.sqlite` (the read-only pack; the starter edition is installed on
first boot and Settings says which edition you have) and `media/` (the ear trainer's clips).
Back up the volume and you have backed up everything; there is no separate export
([ADR-011](../.claude/DECISIONS.md#adr-011)).

## Docker Compose, with optional services

```bash
docker compose up -d                                   # core only
docker compose --profile mt up -d                      # + sentence translation
docker compose --profile speech up -d                  # + Piper TTS + Whisper ASR
docker compose --profile nlp --profile speech up -d    # + HuSpaCy
docker compose --profile llm up -d                     # + Ollama
```

| Profile | Adds | Rough footprint |
|---|---|---|
| *(none)* | SRS, reader, grammar, HVPT, listening, progress | ~40 MB image |
| `mt` | Hungarian → Russian/English sentence translation in the reader | ~600 MB model, 200–400 MB resident |
| `nlp` | context-aware analysis of arbitrary text | ~1.5 GB |
| `speech` | card audio, dictation checking, pronunciation scoring | ~3 GB |
| `llm` | local tutor and roleplay | ~5 GB+ depending on model |

**Set the profiles once.** Typing `--profile mt` on every command is a thing you will forget on
a redeploy, and forgetting it silently takes the translator away — the app then correctly
reports it as unreachable, which reads like a fault. Copy `.env.example` to `.env` beside the
compose file:

```bash
COMPOSE_PROFILES=mt,llm
```

and plain `docker compose up -d` brings it up with everything else.

### The translator, after that first command

Everything else is in **Settings → Services**, and you should not need to come back here:

- The row says which of four things is true — not configured, not answering, downloading the
  model, or ready — instead of one flat «no connection» for the last three. A failed download
  prints the service's own reason.
- **Скачать модель / Download the model** starts the fetch, or retries one that died. No
  container restart, no SSH.
- The provider `<Select>` is the on/off switch; turning it off hides every translate control in
  the app.

The model (~2.4 GB to fetch, ~600 MB converted) is baked into the `mt` image at build time, so
the container starts ready and a rebuild reuses the layer cache rather than fetching it again —
there is deliberately no volume on `/models` ([ADR-015](../.claude/DECISIONS.md#adr-015)). What
the app **cannot** do is start the container — it has no
shell and no Docker socket, and giving a web app the socket would hand it root on your host.
That is why the first command above is still a command. [ADR-015](../.claude/DECISIONS.md#adr-015)

Not using Docker? `make mt` creates the venv and runs the service on `:5003` natively.

**The core is complete without any of them.** Optional services are probed at runtime; features
that need an unreachable one are visibly disabled with a reason, never silently missing.

## Timezone

Set `TZ` (any IANA name, e.g. `Europe/Budapest`) on the `tanulom` service. "Today" — the daily
new-card budget, sibling burial, the streak day — rolls over at local midnight; without `TZ`
the container runs on UTC, so for a learner at UTC+2 the day flips at 02:00. The binary embeds
tzdata, so this works even though the distroless image ships no zoneinfo.

Instead of the `llm` profile you can point at any OpenAI-compatible endpoint in
Settings → Services. Keys are stored in SQLite, never in `config.yaml`.

## Sizing

| Deployment | CPU | RAM | Disk |
|---|---|---|---|
| Core only | 1 core | 256 MB | 1 GB + pack |
| + `nlp` | 2 cores | 2 GB | 3 GB |
| + `speech` | 4 cores | 4 GB | 8 GB |
| + `llm` (8B local) | 8 cores or a GPU | 16 GB | 20 GB |

The core runs comfortably on a Raspberry Pi or a NAS; images are built for `amd64` and `arm64`.

## Reverse proxy

Caddy:

```
tanulom.example.com {
    reverse_proxy localhost:8012
}
```

nginx:

```nginx
server {
    listen 443 ssl http2;
    server_name tanulom.example.com;
    # ssl_certificate ...;

    client_max_body_size 64M;          # EPUB uploads and audio recordings

    location / {
        proxy_pass http://127.0.0.1:8012;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Serve over HTTPS: the session cookie is marked `Secure` behind TLS, and **microphone access
requires a secure context** — the Speak trainer will not work over plain HTTP on a remote host.

## Moving one learner: Settings → Данные

**Settings has a Data panel, and it moves a PROFILE — not an install.** This is the path to use
when you are changing machines or moving off someone else's instance onto your own
([ADR-040](../.claude/DECISIONS.md#adr-040)).

- **Export** gives you a `.zip`: `profile.json` with every table that belongs to you — both FSRS
  state spaces, the review log, your texts and their analyses, your glosses, the daily ledger,
  your tutor chats — plus `media/` for audio you uploaded.
- **Import** replaces the profile of the account you are logged in as, in one transaction. Other
  accounts on the destination are untouched.
- It verifies itself. Every card's review log is replayed through the scheduler and compared to
  the state that arrived with it; a mismatch on the same CPU architecture fails and rolls back.
  Moving between an arm64 and an amd64 host produces a *warning* instead, because FSRS's interval
  fuzz redraws — everything is there and scheduling is unaffected.
- **API keys do not travel.** `settings` rows marked secret are left out of the file on purpose.
  Enter them again on the new install: Settings → Сервисы.
- The import blocks the app while it runs. SQLite here is single-writer by design, so a big
  profile is a few seconds of an unresponsive UI rather than a background job.

This is not an `.apkg` and never becomes one. That format carries one card with one interval, so
the morphological half of what the app knows cannot cross it — ADR-011's refusal stands.

## Backup and restore

**Backup is still a filesystem operation, and it is a different job from the export above:** the
export moves one learner, a backup preserves the install. The data directory is the whole thing: `tanulom.db` (every
user's notes, cards, review log, both state spaces, API keys), `config.yaml`, `media/` and the
language pack. Copy the directory and you have copied the install; put it back and it is back.

```bash
docker run --rm -v tanulom-data:/data -v "$PWD:/out" alpine \
  tar czf /out/tanulom-$(date +%F).tar.gz -C /data .
```

Stop the container first, or accept a WAL-consistent-but-not-quiesced snapshot. SQLite in WAL
mode tolerates the latter, but a clean stop is better — and a **restore must be done stopped**.
Swapping a SQLite file under a live WAL connection is how databases die; there is no code path
in the app that does it, and there should not be one at the shell either.

Two entries are reproducible and can be left out of an archive if size matters: the language
pack (the starter edition reinstalls itself from the binary on the next boot; the full one is
rebuilt with `make langpack`) and `media/hvpt/` (re-rendered by `make audio`).

## Updating

```bash
docker compose pull && docker compose up -d
```

Migrations run automatically inside `db.Open` on start. There is no migrate step — the
distroless image has no shell to run one.

**Language pack upgrades are separate.** A new pack is a file swap; the binary checks
`schema_version` at startup and refuses to start on a major mismatch with an actionable message.
`morph_feature` ids are stable across pack versions, so your learning state carries over.

## Multi-user

Roles are **admin** (full control) and **learner** (own data only). Learning data is per-user;
the language pack is shared. This is designed for a household or a small study group, not for
running a service — there is no billing, quota or tenant isolation beyond row-level user
scoping.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| "language pack not found" | **should be unreachable** — the binary installs its own pack on first boot | check the startup log for `starter language pack installed`, and that `/data` is writable by uid 65532 (`nonroot`). A pack-dependent route answering 503 on a fresh install means something broke, not that setup is incomplete |
| "pack schema version N required" | pack too old or too new for the binary | pull a matching pack or roll the image back |
| Capability dot hollow | service configured but unreachable | check the container is up and the endpoint is right; hover the dot for the error |
| Microphone unavailable | insecure context | serve over HTTPS |
| Reviews pile up | new-card limit too high | Settings → Review — the governor shows the projection |
| Slow first analysis of a long text | tokenising and looking up every token | one-time per text; results are persisted in `text_token` |
