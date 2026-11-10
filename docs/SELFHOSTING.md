# Self-hosting

One container, one volume. Optional components (translator, morphology analyser, tutor model) when you want them. For what each one is, see the [README](../README.md#optional-components); for what the app does, [FEATURES.md](FEATURES.md).

## Quick start

```bash
git clone https://github.com/antlko/learn-hungarian-ai.git
cd learn-hungarian-ai
docker build -t tanulom .
docker run -d --name tanulom -p 8012:8012 -v tanulom-data:/data \
  --restart unless-stopped -e TZ=Europe/Budapest tanulom
```

Open **http://localhost:8012** and create your account. **Nothing is downloaded**: the binary
carries a starter language pack and installs it into `/data` on first boot, audio included. The
full dictionary is an optional upgrade (below). See [ADR-010](../.claude/DECISIONS.md#adr-010).

> **Prebuilt image.** `docker-publish.yml` builds `amd64` and `arm64` into
> `ghcr.io/antlko/learn-hungarian-ai` when a `v*` tag is pushed (after the CI gate passes); `v1.0.0`
> is out, so `docker compose up -d` with the repository's `docker-compose.yml` pulls it. If another
> machine is refused, make the GHCR package public once in the GitHub UI. A minimal, paste-ready
> compose file is in the [README](../README.md#b-docker-compose--the-prebuilt-image-no-clone-needed);
> to build from the clone instead, comment out `image:` and uncomment `build: .`.
>
> **Two images.** `ghcr.io/antlko/learn-hungarian-ai:latest` (also `X.Y.Z`, `X.Y`, `X`, and the
> aliases `latest-full` / `X.Y.Z-full`) carries Python, so «Встроенные компоненты» in Settings can
> install the analyser or translator inside the same container. `:latest-slim` / `X.Y.Z-slim` is the
> distroless image: smaller, no Python, core app and sidecar services (`--profile nlp|mt|llm`) only.
> Both run as uid 65532 and share the `/data` layout, so switching tag keeps your data and account.
> Building yourself: `docker build .` builds the slim image, `docker build --target full -t
> tanulom:full .` the Python one. No image variant was built or run when this was written.
>
> Not using Docker: `make build && ./bin/tanulom --data-dir ./data` (Go 1.26+, Node 22.18+).

### The full dictionary (optional)

The starter has 5,725 lemmas and 12,858 sentences; the full pack 35,311 lemmas and about 60,000
sentences (counts: [LANGPACK.md](LANGPACK.md)). It only widens lookup. Two ways:

- **Settings → Components → Install the full dictionary** (admin). One click: the download
  address and checksum are pinned in the binary (nothing to type; `langpack.url` / `langpack.sha256`
  in `config.yaml` are only an override, e.g. a mirror). **The checksum is not pinned yet**, so
  until a release ships one the button says "not published". Needs free disk for the download plus the unpacked pack (about 260 MB each),
  then a restart (below).
- **`make langpack`** on a workstation (about 2 GB of OpenSubtitles and 580 MB of Wiktionary are
  downloaded; running time not measured). Put `data/langpack-hu-v1.sqlite` into the volume and
  restart. `make subtitles` (the "~40 min" in the Makefile) only adds sentences to an existing pack.

### Restart policy matters

Keep `--restart unless-stopped` (the Compose file already has `restart: unless-stopped`). The
published image sets `TANULOM_SUPERVISED=1`, which tells the app that something will bring it back,
and Settings only then offers **Restart**: the process receives SIGTERM, runs the same graceful
shutdown as `docker stop`, exits, and Docker starts it again. Without a policy the process stays
down; the button still stops it gracefully but says so ("start it again yourself"). A bare binary
(`make build && ./bin/tanulom`) does not set the variable. The app deliberately has no Docker
socket access, so it cannot restart *other* containers — see [ADR-049](../.claude/DECISIONS.md#adr-049).

### The first run

The first page is the setup wizard: it creates the administrator account (password floor of
ten characters — a length, not a composition rule) and asks for the interface language and the
gloss language. Everything lives under the one volume: `tanulom.db` (accounts, progress, review
log), `config.yaml`, `langpack-hu-v1.sqlite` (the read-only pack; the starter edition is installed on
first boot and Settings says which edition you have) and `media/` (the ear trainer's clips).
Back up the volume and you have backed up everything. A per-learner export also exists
(Settings → Data, below; [ADR-040](../.claude/DECISIONS.md#adr-040)).

## Docker Compose, with optional services

**`nlp` setup.** `docker compose --profile nlp up -d` builds `services/nlp` and starts without the model. Fetch it
(`hu_core_news_md` 3.8.1, CC BY-SA 4.0, ~127 MB) from Settings → Components or with the admin
endpoint `POST /api/admin/nlp/warmup`. Then
in Settings set the NLP service to `local`, endpoint `http://nlp:5002`. Progress shows on the
Components card. It is not a small container: give it a memory limit you can afford and expect
slower ingest of long texts. Docker build and the real model have not been run by the authors'
CI; treat first use as a trial.

```bash
docker compose up -d                                   # core only
docker compose --profile mt up -d                      # + sentence translation
docker compose --profile nlp up -d                     # + HuSpaCy (>= 2.5-3 GB RAM free)
docker compose --profile llm up -d                     # + Ollama
```

| Profile | Adds | Rough footprint |
|---|---|---|
| *(none)* | SRS, reader, grammar, HVPT, listening, progress | ~40 MB image |
| `mt` | Hungarian → Russian/English sentence translation in the reader | ~600 MB model, 200–400 MB resident |
| `nlp` | HuSpaCy: picks the in-context reading of ambiguous forms (`vár`) among the pack's own | **plan >= 2.4 GB RAM** (published figure, not measured in a container); small image, ~127 MB model fetched once into a volume |
| `speech` | *not built yet* — the browser speaks | n/a |
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

## Components installed from the app (Settings → Components → «Встроенный»)

The translator (`mt`) and the morphology analyser (`nlp`) are plain Python services. Besides
Docker Compose and your own server, the **binary can install and run them itself**: an
administrator presses **Install** and the app builds a Python venv under
`<data>/components/<id>/`, installs the pinned requirements, prepares the model and starts the
service as a **child process**. No restart of the app is needed and no Docker socket is involved.
Settings → Components shows the three ways as tabs; this is the first.
[ADR-061](../.claude/DECISIONS.md#adr-061).

**What you need.** `python3` ≥ 3.10 with `venv` and `pip` on the `PATH` of the app, outbound
network (PyPI, the PyTorch CPU index, HuggingFace), a writable data directory and free disk. If
any is missing the tab is disabled with the reason and points at the other two. Debian/Ubuntu
need `apt install python3-venv`.

**Where it runs.**

| Method | Where the Python comes from |
|---|---|
| Native binary (`make build`, `make dev`) | your system `python3` |
| `:latest` image (target `full`; `:latest-full` is an alias) | the image: Python + venv + CA certificates, same binary and uid 65532, same `/data` layout |
| `:latest-slim` image (distroless; plain `docker build .`) | **none** — it has no shell and no Python, and says so (`errors.componentNoPythonSlimImage`): change the tag to `:latest` |

**Disk and RAM** (documented estimates, not measurements — the screen says "estimate"):

| Component | Free disk needed | Installed | RAM while running |
|---|---|---|---|
| `mt` | ≈ 6 GB at the start (torch is installed for the one-time conversion, then removed) | ≈ 2.5 GB | ≈ 2–3 GB |
| `nlp` | ≈ 1.5 GB | ≈ 0.6 GB (+127 MB model) | ≈ 2.4 GB (published figure) |

The preflight refuses an install below the first column (`errors.diskFull`) before anything is written.

**Where the files are.** Everything is under `<data>/components/<id>/`: `venv/` (the Python
environment), `src/` (the service source, **restored byte for byte from the copy inside the binary
on every start** — nothing else is ever run), `models/`, `home/` (the service's `HOME`) and `logs/`
(`run.log`, rotated at 1 MiB). An install is built in `<id>.installing/` and renamed on success, so a
failed one leaves nothing. **Uninstall** in Settings deletes the directory and, if the service was
using it, switches the provider to off. Doing it by hand is `rm -rf <data>/components/<id>`
while the app is stopped. It lives in the data volume, so it is in your backups and counts towards
their size; it can be re-created, so exclude it if you prefer.

**Ports.** A managed child listens on `127.0.0.1` on an **ephemeral port** chosen at each start. It
is not published, not written to `config.yaml` (the provider is `managed`, the endpoint is resolved
at runtime), and cannot clash with anything on the host. Only `8012` is ever yours to publish.

**Lifecycle.** Started at boot when the provider is `managed` and the component is installed — in
the background, so a slow or broken child never delays the app, which is fully usable without it.
A crash is restarted with exponential backoff; after 5 crashes in 10 minutes it is marked failed
(Start tries again). **Stop** lasts until the app restarts. Shutting the app down stops the children
first (SIGTERM, SIGKILL after 5 s). Children get a minimal environment: no `TANULOM_*` and none of
the app's secrets. `pip` alone also sees `HTTP(S)_PROXY`/`SSL_CERT_FILE` if you set them.

**If the app is killed hard** (`kill -9`, OOM): on Linux the kernel stops the children with it; on macOS they may keep running until the next start, which finds and stops them (`<id>/run.pid`). A child that stays up but never answers `/health` for 5 minutes is stopped and shown as failed.

**Not covered.** The tutor's LLM (Ollama) is not embedded (multi-GB, GPU libraries): keep it external
or on the `llm` profile. `tts`/`asr` do not exist yet. No real install was run by the authors when
this was written; treat the first one as a trial and read the log tail the card shows.

### What `--profile` means

In `docker-compose.yml` a service with a `profiles:` list is **optional**: `docker compose up`
ignores it until you name the profile (`--profile mt`) or list it in `COMPOSE_PROFILES`. Nothing
more. It is why `docker compose up -d` starts only the app. In compose the services reach each other
by service name on the compose network, so their ports **need not be published** — and the default
file does not publish them, so no host port can clash unless you add a `ports:` line yourself.

## Timezone

Set `TZ` (any IANA name, e.g. `Europe/Budapest`) on the `tanulom` service. "Today" — the daily
new-card budget, sibling burial, the streak day — rolls over at local midnight; without `TZ`
the container runs on UTC, so for a learner at UTC+2 the day flips at 02:00. The binary embeds
tzdata, so this works even though the distroless image ships no zoneinfo.

Instead of the `llm` profile you can point at any OpenAI-compatible endpoint in
Settings → Services. Keys are stored in SQLite, never in `config.yaml`.

## Sizing

| Component | RAM | Disk | Source of the figure |
|---|---|---|---|
| Core (the app) | 256 MB is comfortable | ~40 MB image, ~20 MB starter pack + 1 MB audio in `/data`, plus your database | estimate; **not measured** under load |
| Full dictionary (after install) | no more than the core, plus the pack's page cache | 252 MB file (260 MB extra while installing) | LANGPACK.md |
| `mt` translator | 200–400 MB resident | ~2.4 GB downloaded to build, ~600 MB kept | services/mt README (model card), not measured by us |
| `nlp` HuSpaCy | **>= 2.4 GB** published for the md model (plan 2.5–3 GB) | small image + ~127 MB model in a volume | huspacy.github.io; **not measured in a container; not run in CI** |
| `llm` Ollama, 7–8B model | 8–16 GB (CPU) or a GPU | ~1.5 GB image + ~5 GB per model | rough; not measured |
| `speech` (`tts`, `asr`) | n/a | n/a | **not built** |

The core runs on a Raspberry Pi or a NAS; the image is built for `amd64` and `arm64`. Do not run
`nlp` and a local 8B model on a small box at the same time without a memory limit
(`mem_limit` in the compose file).

## The tutor's model (Ollama)

The tutor needs a chat model; everything else works without. Inside Compose:

```bash
docker compose --profile llm up -d
docker exec tanulom-ollama ollama pull qwen3:8b       # ~5 GB; nobody does this for you
```

Then Settings → Services → Language model: provider `local`, endpoint `http://ollama:11434`
(**host only, no `/v1`**), model `qwen3:8b`. Save the endpoint first; the model picker is filled
from what the server reports. For Ollama on the host, run `OLLAMA_CONTEXT_LENGTH=8192 ollama serve`
with `OLLAMA_HOST=0.0.0.0` and use `http://host.docker.internal:11434`.

- **Context 8k or more.** Ollama defaults to 4k and silently drops the head of a longer prompt,
  which is where the lesson sits. The compose file sets 8192.
- **7–8B is the floor.** Measured 2026-09-03: `qwen3:1.7b` cannot translate a simple sentence.
- A hosted OpenAI-compatible API also works (provider `openai`); its key goes in the database.
- Details: [TUTOR.md](TUTOR.md).

## Ports

Only `8012` is published to the host. `ollama` (11434), `mt` (5003) and `nlp` (5002) are
reachable only inside the compose network. Components installed from the app listen on an ephemeral
`127.0.0.1` port inside the app's own container or machine and need no port at all. `5173` is the Vite dev server (`make dev`). Change the
**left** side of `8012:8012` to move the app; the four places that agree on 8012 are listed in
[DEVELOPMENT.md](DEVELOPMENT.md).

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

**Installing the full language pack from Settings.** An admin can press install in Settings; the
app downloads the pinned pack URL (built into the binary; override with `langpack.url`, see
[CONFIGURATION.md](CONFIGURATION.md)) into `/data` as a
temporary file, checks the gzip against the pinned checksum (`langpack.sha256` overrides), unpacks and opens it, confirms that
every word id your notes, lemma states and imported texts point at still means the same word,
and only then parks it beside the live pack as `langpack-hu-v1.sqlite.new`. Any failure leaves the pack you have
untouched and removes the temporary files; a full disk is reported as such. It needs free space
for the download plus the unpacked pack (roughly 260 MB each for the full pack). The running app keeps
reading the old pack; **the restart applies the new one** — at boot, before the pack is opened, the
check is repeated against your data as it is then and the file is swapped in (if it no longer
matches it is discarded, the old pack stays and Settings says why). Settings → Компоненты then offers **Restart** (a supervised container comes back by itself and the page reloads; otherwise the screen says to start it again by hand). Until a pack is published and its
checksum pinned, the button answers that the pack "has not been published yet"; building your own
(`make langpack`, then drop the file in the volume and restart) always works.

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
| `docker pull ghcr.io/…` says `denied` / `not found` | no `v*` tag has been pushed, or the package is still private | build from source (Quick start) |
| Install button says the pack "has not been published" | the binary pins no checksum yet and `langpack.sha256` is empty | expected until a release ships one. Use `make langpack` and copy the file, or set `langpack.url` and `langpack.sha256` to a pack you host |
| After an update the course or lessons look old (for example only the Basic tier shows) | the app keeps using the full language pack you built earlier, and the course lives inside the pack | stop the server and run `make refresh` (seconds, no downloads; it rewrites the course, lessons and features in `data/langpack-hu-v1.sqlite` — copy the file first if you want a backup). A pack installed from Settings is replaced by installing the newer one |
| Forgot the password | there is no email reset on a self-hosted install | stop the server, then `make reset-password` (from a checkout; `EMAIL=…` / `PASSWORD=…` optional — otherwise it prints a generated one) or `tanulom reset-password --data-dir ./data`; in Docker: `docker run --rm -v tanulom-data:/data tanulom reset-password --data-dir /data`. Progress is kept; every session of that account is signed out |
| Want to start over | — | stop the server, then `make reset-learner` (asks for `YES`): deletes accounts, progress and settings in `./data` and keeps the language pack and audio |
| After install, "restart needed", and Restart does nothing visible | the container has no restart policy, so it stopped and stayed stopped (`supervised: false`) | `docker start tanulom`; use `--restart unless-stopped` next time |
| Installed pack rejected: "incompatible" | its word ids differ from the pack your cards point at (built from another dump) | build from the same source dump, or keep the starter |
| `nlp` is "downloading" forever / "error" | the model wheel could not be fetched (network, proxy, disk) | read the card's reason; `docker logs tanulom-nlp`; retry from Components; the volume `tanulom-nlp-models` must be writable |
| `nlp` container killed | out of memory | give it >= 3 GB or leave it off; the app is unchanged without it |
| `docker compose up` fails pulling `tanulom` | the image is refused from another machine, or the tag is wrong | the image is published (`:latest` = Python-capable, `:latest-slim` = distroless without Python); make the GHCR package public once, or build from the clone with `build: {context: ., target: full}` |
