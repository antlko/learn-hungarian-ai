# Speech synthesis (Piper) — **not built**

There is no code in this directory, and no compose entry: the `speech` profile in
[`docker-compose.yml`](../../docker-compose.yml) is commented out so nothing points at a missing
build. Nothing in the app calls this service; the config row exists so the setting has a place
to live when it is built.

What you have instead: card audio and spoken words use the browser's own voices (the `device`
provider, [ADR-012](../../.claude/DECISIONS.md#adr-012)), and the ear trainer plays clips shipped
inside the binary. The ear trainer is not affected.

## If it is built

A small HTTP service on port `5000` with `GET /health` (non-2xx until ready) and its own endpoint
(`/synthesize`), using Piper, following the contract in
[`../README.md`](../README.md). The core would probe it through `GET /api/capabilities` and keep
working when it is absent ([ADR-001](../../.claude/DECISIONS.md#adr-001)).
