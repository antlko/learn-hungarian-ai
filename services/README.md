# Optional sidecar services

**`mt/` is built.** `tts/` and `asr/` are not: they contain a README and nothing
else, and their compose entries are commented out so no profile points at a missing build.

| Directory | Service | Profile | Status | Adds |
|---|---|---|---|---|
| [`mt/`](mt/) | **NLLB-200-distilled-600M** | `mt` | **built** | sentence translation in the reader |
| [`tts/`](tts/) | Piper — 3 Hungarian voices | `speech` | not built | card audio, ear-training talkers (the browser speaks meanwhile) |
| [`asr/`](asr/) | faster-whisper (Hungarian fine-tune) | `speech` | not built | dictation checking, pronunciation scoring |

## Managed mode: the binary can run `mt` itself

Besides Compose and your own server, **Settings → Components → «Встроенный»** installs and runs
`mt` as a supervised child process of the app: a venv under `<data>/components/<id>/`,
the pinned `requirements.txt`, the model, then `python app.py` on `127.0.0.1:<ephemeral port>`
([ADR-061](../.claude/DECISIONS.md#adr-061), [SELFHOSTING.md](../docs/SELFHOSTING.md)). It runs the
source **embedded in the binary**, not these files directly: after editing `services/<id>/` run
`make sync-components` (a Go test fails on drift). The only change the services needed is
`MT_HOST` (default `0.0.0.0`, so Compose is unchanged; managed mode sets `127.0.0.1`).
Needs `python3` ≥ 3.10 with venv; the default distroless image has none (use `tanulom:full`).

## The rule they exist to preserve

> **The core must remain fully usable with every sidecar stopped.**

This is verified at every release, not assumed. The core does its own
morphological analysis from the precomputed language pack
([ADR-002](../.claude/DECISIONS.md#adr-002)); these services make it *better*, never *possible*.
(The optional context-disambiguation sidecar that once re-picked among the pack's readings was
removed: [ADR-069](../.claude/DECISIONS.md#adr-069).)

Every optional capability is probed at runtime (`GET /api/capabilities`) and rendered as a
status dot in the top bar. A feature needing an unreachable service is **disabled with a visible
reason** — never silently missing.

## Contract

Each service exposes:

- `GET /health` → `200` when ready, and **non-2xx until then**. This is what the capability probe
  calls, and it counts any non-5xx as reachable — so a service that answers `200` while still
  loading its model lights the capability dot green and hands the learner a button that times out.
  The body carries the state (`mt`: `downloading|loading|ready|error` + `progress`).
- its own endpoint (`/synthesize`, `/transcribe`, `/analyze`, `/translate`).

No authentication: they are expected to sit on a private compose network and are never published
to the host in the shipped `docker-compose.yml`.
