"""
Hungarian → Russian / English sentence translation for the reader.

Why this exists. A learner reading a Hungarian text occasionally needs to know
what a sentence actually says. Without this service the answer is a trip to
somebody else's chat window, which costs them their reading flow [R11] and
hands their material to a third party. This service is the alternative: a small
translation model on their own machine.

Why a translation model and not a chat model. Hungarian is low-resource for
general LLMs, and an 8B general model translating hu→ru is materially worse
than NLLB-200-distilled-600M, which is dedicated to the task and fits in a few
hundred megabytes of int8 weights. The app has an `llm` slot for chat models;
this is deliberately not it, and the wire protocol below is not the OpenAI one.

What the app does with the output. It labels it. Every translated line reaches
the screen marked as machine output, naming the model — see ADR-014. Nothing
from here becomes a card, a gloss or a graded answer.

Endpoints
    GET  /health     200 once the model is loaded, 503 while it is not.
                     The Go core's capability probe calls this, and it treats
                     any non-5xx as "reachable" — so a service that answered
                     200 while still loading would have the app enable a button
                     that then times out.

                     The BODY carries the difference the status code cannot:
                     `status` is loading / ready / failed, `detail` is the
                     reason when it failed, and `downloadedBytes`/`totalBytes`
                     track the first-run download. Without that body the app
                     renders "downloading 2.4 GB" and "never deployed"
                     identically, as "no connection" — which is the single most
                     confusing thing this service used to do.
    POST /warmup     Start (or retry) the download and load. Returns the same
                     body as /health. Idempotent: a load already in flight is
                     reported, not started twice. This is what makes a failed
                     download retryable from Settings instead of by restarting
                     the container over SSH.
    POST /translate  {"texts": [...], "target": "rus_Cyrl"}
                  -> {"texts": [...], "model": "..."}
                     Same length, same order. The Go client rejects a short
                     batch, because a dropped element would silently pair one
                     sentence with another sentence's translation.
"""

from __future__ import annotations

import logging
import os
import threading
from contextlib import asynccontextmanager
from pathlib import Path

import ctranslate2
import uvicorn
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field
from transformers import AutoTokenizer

log = logging.getLogger("mt")

# NLLB's own name for Hungarian. Only the TARGET travels on the wire: this
# service translates out of Hungarian and nothing else, which is the whole
# reason it can be this small.
SOURCE_LANG = "hun_Latn"
SUPPORTED_TARGETS = {"rus_Cyrl", "eng_Latn"}

HF_MODEL = os.environ.get("MT_MODEL", "facebook/nllb-200-distilled-600M")
MODEL_DIR = Path(os.environ.get("MT_MODEL_DIR", "/models/nllb-ct2"))
# int8 is the point of the exercise: it takes the 600M-parameter model to a few
# hundred megabytes and a fraction of a second per sentence on a CPU, at a
# quality cost that does not show up in sentences this short.
QUANTIZATION = os.environ.get("MT_QUANTIZATION", "int8")
DEVICE = os.environ.get("MT_DEVICE", "cpu")
BEAM_SIZE = int(os.environ.get("MT_BEAM_SIZE", "4"))
PORT = int(os.environ.get("MT_PORT", "5003"))

_translator: ctranslate2.Translator | None = None
_tokenizer = None
_state = {"status": "loading", "detail": ""}
_lock = threading.Lock()
# Guards the loader itself, so two /warmup calls cannot both start converting a
# 2.4 GB model into the same directory.
_load_lock = threading.Lock()
_loading = False
# Cached once: the model's total size, which costs a network round trip to learn.
_total_bytes = 0


def _snapshot_bytes() -> tuple[int, int]:
    """Bytes on disk so far, and the total the model needs.

    Real numbers or none: an invented percentage on a download that takes ten
    minutes is worse than an honest spinner, and this project does not print
    confident figures it cannot support.

    The total comes from the HuggingFace file listing, which needs the network
    and is cached after the first success. The progress is the size of the
    snapshot directory the downloader is filling. Both are best-effort; either
    failing yields (0, 0) and the UI falls back to an indeterminate bar.
    """
    global _total_bytes
    try:
        from huggingface_hub import HfApi
        from huggingface_hub.constants import HF_HUB_CACHE
    except Exception:  # noqa: BLE001 - progress is optional, translation is not
        return 0, 0

    if _total_bytes == 0:
        try:
            info = HfApi().model_info(HF_MODEL, files_metadata=True)
            _total_bytes = sum(f.size or 0 for f in (info.siblings or []))
        except Exception:  # noqa: BLE001 - offline, rate-limited, whatever
            _total_bytes = 0

    # `models--facebook--nllb-200-distilled-600M` is the on-disk name for a repo.
    #
    # Only `blobs/`, and only real files. A HuggingFace repo directory holds the
    # blobs plus a `snapshots/` tree of SYMLINKS to them, and `Path.stat()`
    # follows symlinks — so walking the whole directory counts every byte twice
    # and reports 9.9 GB downloaded of a 2.5 GB model. `.incomplete` parts live
    # in `blobs/` too and should count: they are bytes that have arrived.
    blobs = Path(HF_HUB_CACHE) / ("models--" + HF_MODEL.replace("/", "--")) / "blobs"
    done = 0
    if blobs.is_dir():
        done = sum(
            f.stat().st_size for f in blobs.iterdir() if f.is_file() and not f.is_symlink()
        )
    # Never claim more than the whole. A total measured from the file listing
    # and a size measured from disk can disagree by a revision or a stray part,
    # and a progress bar past 100% is worse than no bar at all.
    if _total_bytes:
        done = min(done, _total_bytes)
    return done, _total_bytes


def _report() -> dict:
    """The state, as both /health and /warmup return it."""
    with _lock:
        state = dict(_state)
    body = {"status": state["status"], "detail": state["detail"], "model": HF_MODEL}
    if state["status"] == "loading":
        done, total = _snapshot_bytes()
        body["downloadedBytes"], body["totalBytes"] = done, total
    return body


def prepare_model() -> Path:
    """Convert the HuggingFace model to a CTranslate2 one, once.

    Done here rather than only in the Dockerfile so that `pip install -r
    requirements.txt && python app.py` works on a laptop — which is where this
    will mostly run, because CPU inference inside Docker on macOS is slow.
    The Dockerfile calls the same function at build time, so the container
    starts ready instead of downloading 2.4 GB on first boot.
    """
    if (MODEL_DIR / "model.bin").exists():
        return MODEL_DIR

    log.info("converting %s -> %s (%s); this downloads ~2.4 GB once", HF_MODEL, MODEL_DIR, QUANTIZATION)
    # Imported lazily: the converter needs torch, which is only required the
    # first time and is not worth holding in memory for the service's life.
    from ctranslate2.converters import TransformersConverter

    MODEL_DIR.parent.mkdir(parents=True, exist_ok=True)
    TransformersConverter(HF_MODEL).convert(str(MODEL_DIR), quantization=QUANTIZATION, force=True)
    # The tokenizer is not part of the CT2 output and is needed at runtime.
    AutoTokenizer.from_pretrained(HF_MODEL).save_pretrained(str(MODEL_DIR))
    return MODEL_DIR


def load() -> None:
    global _translator, _tokenizer, _loading
    try:
        path = prepare_model()
        translator = ctranslate2.Translator(str(path), device=DEVICE, compute_type=QUANTIZATION)
        tokenizer = AutoTokenizer.from_pretrained(str(path), src_lang=SOURCE_LANG)
        with _lock:
            _translator, _tokenizer = translator, tokenizer
            _state.update(status="ready", detail="")
        log.info("model ready: %s", HF_MODEL)
    except Exception as exc:  # noqa: BLE001 - the reason must reach /health
        log.exception("model failed to load")
        with _lock:
            _state.update(status="failed", detail=str(exc))
    finally:
        with _load_lock:
            _loading = False


def start_load() -> bool:
    """Kick off a load unless one is already running. True if this call started it."""
    global _loading
    with _load_lock:
        if _loading:
            return False
        _loading = True
    with _lock:
        _state.update(status="loading", detail="")
    threading.Thread(target=load, daemon=True).start()
    return True


@asynccontextmanager
async def lifespan(_: FastAPI):
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    # Loaded in a thread so /health answers immediately and honestly while the
    # model comes up, rather than the port refusing connections for two minutes.
    start_load()
    yield


app = FastAPI(title="tanulom-mt", lifespan=lifespan)


@app.get("/health")
def health() -> JSONResponse:
    body = _report()
    # 503 while loading is deliberate: the capability probe must not report a
    # service that cannot answer /translate yet as usable. The body says which
    # kind of not-ready this is.
    return JSONResponse(status_code=200 if body["status"] == "ready" else 503, content=body)


@app.post("/warmup")
def warmup() -> JSONResponse:
    """Start or retry the download and load.

    Answers 200 in every case that is not an error, including "already ready"
    and "already loading" — the caller is asking for the model to exist, and it
    either does, is on its way, or has a reason it does not.
    """
    with _lock:
        status = _state["status"]
    if status == "ready":
        return JSONResponse(content=_report())
    started = start_load()
    body = _report()
    body["started"] = started
    return JSONResponse(content=body)


class TranslateRequest(BaseModel):
    texts: list[str] = Field(default_factory=list)
    target: str = "rus_Cyrl"


@app.post("/translate")
def translate(req: TranslateRequest) -> JSONResponse:
    with _lock:
        translator, tokenizer, state = _translator, _tokenizer, dict(_state)
    if translator is None or tokenizer is None:
        return JSONResponse(status_code=503, content={"error": f"model {state['status']}: {state['detail']}"})
    if req.target not in SUPPORTED_TARGETS:
        return JSONResponse(
            status_code=400,
            content={"error": f"target {req.target!r} is not one of {sorted(SUPPORTED_TARGETS)}"},
        )
    if not req.texts:
        return JSONResponse(content={"texts": [], "model": HF_MODEL})

    # NLLB is fed token STRINGS, and the target language is forced as a decoder
    # prefix rather than hoped for: without it the model is free to answer in
    # whatever language it likes, which for a Hungarian source is often
    # Hungarian.
    sources = [tokenizer.convert_ids_to_tokens(tokenizer.encode(t)) for t in req.texts]
    results = translator.translate_batch(
        sources,
        target_prefix=[[req.target]] * len(sources),
        beam_size=BEAM_SIZE,
        max_batch_size=8,
    )
    out = []
    for res in results:
        # [1:] drops the language token we forced; decoding it would print
        # "rus_Cyrl" at the head of every translation.
        tokens = res.hypotheses[0][1:]
        out.append(tokenizer.decode(tokenizer.convert_tokens_to_ids(tokens)))

    return JSONResponse(content={"texts": out, "model": HF_MODEL})


if __name__ == "__main__":
    uvicorn.run(app, host=os.environ.get("MT_HOST", "0.0.0.0"), port=PORT)
