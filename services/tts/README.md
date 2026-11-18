# Speech synthesis (Piper) — managed sidecar

A small HTTP service on port `5000` that speaks Hungarian with three [Piper](https://github.com/rhasspy/piper)
voices (`hu_HU-anna`, `hu_HU-berta`, `hu_HU-imre`, medium). It is the managed component `tts`:
the binary downloads the Python runtime and the voices once from the project repository (hash-pinned, [ADR-081](../../.claude/DECISIONS.md#adr-081)), unpacks them and runs Piper as a child process (`tanulom component run tts`,
[ADR-061](../../.claude/DECISIONS.md#adr-061)), or it runs as the `tts` compose profile (the `full` image). The app
works without it; the `device` provider (the browser's own voices,
[ADR-012](../../.claude/DECISIONS.md#adr-012)) stays the default.

## Contract

Follows [`../README.md`](../README.md):

- `GET /health` — non-2xx until the voices are loaded.
- `GET /voices` — the voices found in the voices directory.
- `POST /synthesize` — text and voice in, audio out.

Environment: `TTS_VOICES_DIR` — directory holding `hu_HU-{anna,berta,imre}-medium.onnx` and the
matching `.onnx.json`. The core passes it from `TANULOM_VOICES_DIR` (fallback `<data>/voices`).

## Voice models

The model files live in [`../../langpack/voices/`](../../langpack/voices) and are **committed to the
repository by the owner**; the app downloads them from there once at Install (hash-checked). About 190 MB of `.onnx` can be committed
plainly or with Git LFS. Licences: [`../../langpack/LICENSES.md`](../../langpack/LICENSES.md). The Piper
engine and espeak-ng are GPL-3.0; the runtime is vendored under [`vendor/`](vendor/README.md) (downloaded from this repository, not PyPI,
[ADR-081](../../.claude/DECISIONS.md#adr-081)) and only ever runs as a separate process.

## Run natively

`make tts` serves on `http://localhost:5000` from the local vendored tree (no download): it is
`tanulom component run tts --host 127.0.0.1 --port 5000 --data-dir .tts-data --voices-dir langpack/voices
--tts-runtime-dir services/tts/vendor` (the first start unpacks ≈ 430 MB into the gitignored `.tts-data/`).
`make tts-smoke` is the end-to-end proof (install through a local http mirror with the network cut, three voices synthesise, Enable offline, Uninstall).

## Docker

There is no separate image: the `tts` compose profile runs the `full` tanulom image as
`component run tts`, which downloads the runtime and voices (≈ 277 MB) on its first start from the project repository
into its volume. Nothing Piper is baked into the image (≈ 195 MB); the GPL source is in this repository at
`services/tts/vendor/source/`. The distroless `slim` image has no libc and no speech.
