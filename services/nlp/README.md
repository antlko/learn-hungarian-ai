# HuSpaCy sidecar (`nlp` profile)

Context-aware disambiguation for the reader. For a form the language pack finds ambiguous
(`vár`: castle / waits) the core sends the whole text here once and lets this service **choose
among the readings the pack already attests**. It never adds an analysis — see
[ADR-054](../../.claude/DECISIONS.md#adr-054). With it stopped the app behaves exactly as before.

## Managed mode (no Docker)

**Settings → Components → «Встроенный» → Установить** creates a venv, installs `requirements.txt`,
runs this service on `127.0.0.1` on a free port (`NLP_HOST`, default `0.0.0.0`) and fetches the
127 MB model through `POST /warmup` itself — about 1.5 GB free disk needed (estimate). Needs
`python3` ≥ 3.10 with venv; see [SELFHOSTING.md](../../docs/SELFHOSTING.md) and
[ADR-061](../../.claude/DECISIONS.md#adr-061). The app runs the copy embedded in its binary:
after editing `app.py` run `make sync-components`.

## Not a small container

The model `hu_core_news_md` 3.8.1 is published at about **2.4 GB RAM** (huspacy.github.io, 2024) and
~4,500 tokens/s on CPU. We have **not measured** it in a container: plan for at least 2.5–3 GB free
and set a memory limit you can afford. The image itself is small (python:3.12-slim + spaCy 3.8.7);
the 127 MB model wheel is not baked in.

## Run

```bash
docker compose --profile nlp up -d --build     # starts WITHOUT the model
# then fetch it: Settings -> Components, or POST /api/admin/nlp/warmup (admin),
# or directly: curl -X POST http://nlp:5002/warmup (port not published by default)
curl http://localhost:5002/health              # {"status":"downloading","progress":0.4,...}
```

Then Settings → services → NLP: provider `local`, endpoint `http://nlp:5002`.
Standalone: `pip install -r requirements.txt && NLP_MODEL_DIR=./models python app.py`.

The wheel comes from `https://huggingface.co/huspacy/hu_core_news_md/resolve/main/hu_core_news_md-3.8.1-py3-none-any.whl`
(HEAD-checked 2026-11-08: 126,995,383 bytes) and is installed with `pip --target /models/site`, so it
survives container recreation. It requires spaCy `>=3.8.0,<3.9.0`.

## API

- `GET /livez` → always `200` (container healthcheck).
- `GET /health` → `200` only when ready, else `503`; body `{status: downloading|loading|ready|error, progress: 0..1, model, version, error?}`.
- `POST /warmup` → same body; idempotent.
- `POST /analyze {text, sentences?}` → `{model, tokens:[{start,end,text,lemma,upos,feats,sent}]}`.
  `start`/`end` are **UTF-8 byte offsets** (converted from spaCy's character offsets); `feats` is
  `Case=Ine|Number=Sing`. Over 20,000 characters → `413`. Runs in the threadpool, one analysis at a time.

Env: `NLP_PORT` (5002), `NLP_MODEL_DIR` (/models), `NLP_MAX_CHARS` (20000),
`OMP_NUM_THREADS` (2).

## Licence

The model is **CC BY-SA 4.0** (HuSpaCy, SzegedAI). It is downloaded by the user on their own
machine and is not redistributed in this repository or its images.

## Status

Written and syntax-checked only. **Not run:** Docker build, the model download, real inference.
