# Optional sidecar services

**Only `mt/` exists so far.** The rest arrive in Phases 8–9; see
[`.claude/PLAN.md`](../.claude/PLAN.md).

| Directory | Service | Profile | Phase | Adds |
|---|---|---|---|---|
| [`mt/`](mt/) | **NLLB-200-distilled-600M** | `mt` | **built** | sentence translation in the reader |
| [`tts/`](tts/) | Piper — 3 Hungarian voices | `speech` | 8 | card audio, ear-training talkers |
| [`asr/`](asr/) | faster-whisper (Hungarian fine-tune) | `speech` | 8 | dictation checking, pronunciation scoring |
| [`nlp/`](nlp/) | HuSpaCy `hu_core_news_lg` | `nlp` | 9 | context-aware disambiguation, unknown forms |

## The rule they exist to preserve

> **The core must remain fully usable with every sidecar stopped.**

This is verified at every release, not assumed. The core does its own
morphological analysis from the precomputed language pack
([ADR-002](../.claude/DECISIONS.md#adr-002)); these services make it *better*, never *possible*.

Every optional capability is probed at runtime (`GET /api/capabilities`) and rendered as a
status dot in the top bar. A feature needing an unreachable service is **disabled with a visible
reason** — never silently missing.

## Contract

Each service exposes:

- `GET /health` → `200` when ready, and **non-2xx until then**. This is what the capability probe
  calls, and it counts any non-5xx as reachable — so a service that answers `200` while still
  loading its model lights the capability dot green and hands the learner a button that times out.
- its own endpoint (`/synthesize`, `/transcribe`, `/analyze`, `/translate`).

No authentication: they are expected to sit on a private compose network and are never published
to the host in the shipped `docker-compose.yml`.
