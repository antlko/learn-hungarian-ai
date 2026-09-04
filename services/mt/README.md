# Sentence translation (NLLB-200)

Translates a Hungarian sentence into Russian or English, on your own machine, so that
"what does this line mean?" does not have to be answered by pasting your reading material into
someone else's chat window.

Enabled per install and **off by default**. The reader is complete without it: the translate
control is not drawn when the service is unreachable, and nothing else in the app depends on it.

## What it is

[NLLB-200-distilled-600M](https://huggingface.co/facebook/nllb-200-distilled-600M), converted to
[CTranslate2](https://github.com/OpenNMT/CTranslate2) with int8 weights. About 600 MB on disk,
200–400 MB resident, roughly 0.2–0.5 s per sentence on a modern CPU core.

**A dedicated translation model, not a chat model.** Hungarian is low-resource for general LLMs,
and a 7–8B general model translating hu→ru is materially worse than this 600M one, which does
nothing else. The app's `llm` slot is for chat models and is a different thing; `services.mt`
rejects `provider: openai` for that reason — this speaks its own `/translate`, not the chat API.

**It translates out of Hungarian only.** That is what lets it be this small.

## Licence — read this before you redistribute anything

The **model weights are CC-BY-NC-4.0: non-commercial use only.** Running it for your own learning
is exactly what that licence is for. It is also why the weights are **not** baked into the
tanulom binary or its published image, and why this profile is opt-in: everything the project
ships has to be redistributable, and these weights are not.

The code in this directory is under the project's own licence. CTranslate2 is MIT.

## Running it

Docker, alongside the app:

```
docker compose --profile mt up -d
```

The image is built with the model already converted, so the first `up` is slow (it downloads
~2.4 GB from HuggingFace during the build) and every later start is instant.

Or directly, which is what you want on macOS — CPU inference inside Docker there is markedly
slower than native:

```
pip install -r requirements.txt -r requirements-convert.txt
MT_MODEL_DIR=~/.cache/tanulom-mt python app.py
```

Or, from the repository root, `make mt` — it creates `services/mt/.venv`, installs both
requirement files and starts the service with `MT_MODEL_DIR` pointed at `~/.cache/tanulom-mt`.

## Endpoints

| Method | Path | |
|---|---|---|
| `GET` | `/health` | 200 once loaded, **503 while loading** — deliberately, so the app's capability probe does not call the service usable before it can answer. The body carries the difference the status code cannot: `status` (loading / ready / failed), `detail`, `model`, and `downloadedBytes` / `totalBytes`. |
| `POST` | `/warmup` | Start or retry the download and load. Idempotent — a load already running is reported, not started twice. This is what makes a failed download recoverable without restarting the container. |
| `POST` | `/translate` | `{"texts": [...], "target": "rus_Cyrl"}` → `{"texts": [...], "model": "..."}`. Same length, same order. |

**About the progress fields.** `totalBytes` is the HuggingFace file listing (one network call,
cached); `downloadedBytes` is the size of the cache's `blobs/` directory, real files only. The
`snapshots/` tree is symlinks to those same blobs and `Path.stat()` follows them, so counting
the whole directory reports 9.9 GB downloaded of a 2.5 GB model. Both fields are omitted rather
than zeroed when they cannot be measured, and the app then draws an indeterminate bar — a made-up
percentage would be worse than no percentage.

The first run downloads and converts the model into `MT_MODEL_DIR`; later runs load it in a
second or two. `torch` is needed only for that one conversion.

Then point the app at it: **Settings → Services → Translation**, provider `local`, endpoint
`http://localhost:5003` (or `http://mt:5003` under compose).

## Environment

| Variable | Default | Meaning |
|---|---|---|
| `MT_MODEL` | `facebook/nllb-200-distilled-600M` | HuggingFace model to convert |
| `MT_MODEL_DIR` | `/models/nllb-ct2` | where the converted model lives |
| `MT_QUANTIZATION` | `int8` | `int8`, `int8_float16`, `float16`, `float32` |
| `MT_DEVICE` | `cpu` | `cuda` if you have one |
| `MT_BEAM_SIZE` | `4` | lower is faster and flatter |
| `MT_PORT` | `5003` | listen port |

`MT_MODEL` will accept the 1.3B and 3.3B distilled models, which are better and proportionally
slower. Anything larger stops being worth it for single sentences.

## Contract

```
GET  /health
     200 {"status":"ready","model":"..."}          once loaded
     503 {"status":"loading"|"failed","detail":"…"} until then
```

`/health` answers **503 while the model is loading**, and that is load-bearing: the core's
capability probe counts any non-5xx as reachable, so a premature 200 would light the capability
dot green and hand the learner a button that times out.

```
POST /translate
     {"texts": ["Reggel hét órakor felkelek."], "target": "rus_Cyrl"}
  -> {"texts": ["Утром я встаю в семь часов."],  "model": "facebook/nllb-200-distilled-600M"}
```

Targets: `rus_Cyrl`, `eng_Latn`. The response has the **same length and order** as the request —
the Go client rejects a short batch rather than risk pairing one sentence with another's
translation.

No authentication: like the other sidecars, it expects a private network and is never published
to the host in the shipped `docker-compose.yml`.

## What the app does with the output

It labels it. Every line is shown as machine translation with the model named, it is never
turned into a card, a gloss or a graded answer, and it is cached per user in `translation_cache`
so the same sentence is not paid for twice. See
[ADR-014](../../.claude/DECISIONS.md#adr-014).
