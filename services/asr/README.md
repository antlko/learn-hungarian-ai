# Speech recognition (faster-whisper) — **not yet implemented**

Planned for **Phase 8**. See [`.claude/PLAN.md`](../../.claude/PLAN.md).

This directory is referenced by the `speech` profile in
[`docker-compose.yml`](../../docker-compose.yml). Until it is built, **that profile will not
start** — the profile is declared so the intended topology is visible, not because it works today.

`docker compose up -d` (no profile) is unaffected and is the whole product as of v1.0.

## What will live here

An HTTP service on port `5001` exposing `/health` plus its capability endpoint. The Go core
probes it via `GET /api/capabilities` and degrades gracefully when it is absent — see
[ADR-001](../../.claude/DECISIONS.md#adr-001) and
[`docs/ARCHITECTURE.md`](../../docs/ARCHITECTURE.md).
