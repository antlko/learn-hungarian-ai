# Self-hosting

One container, one volume. Optional services when you want speech or an LLM.

## Quick start

```bash
docker run -d --name tanulom \
  -p 8080:8080 \
  -v tanulom-data:/data \
  --restart unless-stopped \
  ghcr.io/<owner>/tanulom:latest
```

Open **http://localhost:8080** and create your account. The setup wizard offers to download the
Hungarian language pack (~a few hundred MB) on first run.

## Docker Compose, with optional services

```bash
docker compose up -d                                   # core only
docker compose --profile speech up -d                  # + Piper TTS + Whisper ASR
docker compose --profile nlp --profile speech up -d    # + HuSpaCy
docker compose --profile llm up -d                     # + Ollama
```

| Profile | Adds | Rough footprint |
|---|---|---|
| *(none)* | SRS, reader, grammar, HVPT, listening, progress | ~40 MB image |
| `nlp` | context-aware analysis of arbitrary text | ~1.5 GB |
| `speech` | card audio, dictation checking, pronunciation scoring | ~3 GB |
| `llm` | local tutor and roleplay | ~5 GB+ depending on model |

**The core is complete without any of them.** Optional services are probed at runtime; features
that need an unreachable one are visibly disabled with a reason, never silently missing.

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
    reverse_proxy localhost:8080
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
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Serve over HTTPS: the session cookie is marked `Secure` behind TLS, and **microphone access
requires a secure context** — the Speak trainer will not work over plain HTTP on a remote host.

## Backup and restore

**Settings → Data → Export** downloads a zip containing `config.yaml`, `tanulom.db` and `media/`.
The language pack is excluded because it is reproducible and large.

**Import** validates the archive, snapshots your current state into `backups/`, then applies —
so a restore is always recoverable.

For scripted backups, the volume is self-contained:

```bash
docker run --rm -v tanulom-data:/data -v "$PWD:/out" alpine \
  tar czf /out/tanulom-$(date +%F).tar.gz -C /data .
```

Stop the container first, or accept a WAL-consistent-but-not-quiesced snapshot. SQLite in WAL
mode tolerates the latter, but a clean stop is better.

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
| "language pack not found" | first run, no download | Settings → Data → Download, or mount the file into `/data` |
| "pack schema version N required" | pack too old or too new for the binary | pull a matching pack or roll the image back |
| Capability dot hollow | service configured but unreachable | check the container is up and the endpoint is right; hover the dot for the error |
| Microphone unavailable | insecure context | serve over HTTPS |
| Reviews pile up | new-card limit too high | Settings → Review — the governor shows the projection |
| Slow first analysis of a long text | tokenising and looking up every token | one-time per text; results are persisted in `text_token` |
