"""
HuSpaCy sidecar: context-aware Hungarian analysis for the reader.

Role (ADR-054). The core analyses words from the language pack and never needs
this. When this is configured and reachable, the core sends a whole text ONCE
and uses the answer only to CHOOSE among readings the pack already attests for
a form the pack found ambiguous. Nothing returned here introduces an analysis.

Endpoints
    GET  /health   200 only when the model is ready, 503 otherwise. The body is
                   {"status": "downloading"|"loading"|"ready"|"error",
                    "progress": 0..1, "model", "version", "error"?}.
    POST /warmup   Start (or retry) the download + load. Same body as /health.
                   Idempotent. Nothing downloads at startup:
                   the model is a 127 MB opt-in fetch.
    POST /analyze  {"text": str, "sentences": bool?}
                -> {"model", "tokens": [{start,end,text,lemma,upos,feats,sent}]}
                   start/end are UTF-8 BYTE offsets (spaCy gives character
                   offsets; Go slices strings by byte). feats is "Case=Ine|
                   Number=Sing". 413 above NLP_MAX_CHARS characters.

The model wheel is installed with `pip --target` into NLP_MODEL_DIR and loaded
through sys.path, so it persists on the mounted volume.
"""

from __future__ import annotations

import logging
import os

# Small thread counts BEFORE anything imports numpy/thinc.
for _v in ("OMP_NUM_THREADS", "OPENBLAS_NUM_THREADS", "MKL_NUM_THREADS"):
    os.environ.setdefault(_v, "2")

import shutil
import subprocess
import sys
import threading
import urllib.request
from contextlib import asynccontextmanager
from pathlib import Path

import uvicorn
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
from pydantic import BaseModel

log = logging.getLogger("nlp")

MODEL = "hu_core_news_md"
VERSION = os.environ.get("NLP_MODEL_VERSION", "3.8.1")
WHEEL_URL = os.environ.get(
    "NLP_WHEEL_URL",
    f"https://huggingface.co/huspacy/{MODEL}/resolve/main/{MODEL}-{VERSION}-py3-none-any.whl",
)
# Size reported by the HuggingFace CDN for the 3.8.1 wheel; used only as the
# progress denominator when the server sends no Content-Length.
FALLBACK_TOTAL = 126_995_383
MODEL_DIR = Path(os.environ.get("NLP_MODEL_DIR", "/models"))
PKG_DIR = MODEL_DIR / "site"
PORT = int(os.environ.get("NLP_PORT", "5002"))
MAX_CHARS = int(os.environ.get("NLP_MAX_CHARS", "20000"))

_lock = threading.Lock()
_state = {"status": "error", "progress": 0.0, "error": "model not downloaded: POST /warmup"}
_nlp = None
_busy = False
# spaCy pipelines are not documented as thread-safe; one analysis at a time.
_infer_lock = threading.Lock()


def _set(**kw) -> None:
    with _lock:
        _state.update(kw)


def _report() -> dict:
    with _lock:
        s = dict(_state)
    body = {"status": s["status"], "progress": round(s["progress"], 4), "model": MODEL, "version": VERSION}
    if s["status"] == "error":
        body["error"] = s.get("error", "")
    return body


MARKER = PKG_DIR / ".installed"


def _installed() -> bool:
    """Complete = the marker, written only after a successful rename, names this version."""
    try:
        return (PKG_DIR / MODEL).is_dir() and MARKER.read_text().strip() == VERSION
    except OSError:
        return False


def _install_wheel() -> None:
    """Download and install into a temp dir, then rename: a crash leaves no half-install."""
    tmp = MODEL_DIR / "site.tmp"
    shutil.rmtree(tmp, ignore_errors=True)
    shutil.rmtree(PKG_DIR, ignore_errors=True)
    _set(status="downloading", progress=0.0, error="")
    part = MODEL_DIR / f"{MODEL}-{VERSION}.whl.part"
    final = MODEL_DIR / f"{MODEL}-{VERSION}-py3-none-any.whl"
    try:
        _download(part)
        part.replace(final)
        # --no-deps: spaCy is already installed; the wheel must not pull a
        # second copy into the volume.
        subprocess.run(
            [sys.executable, "-m", "pip", "install", "--no-deps", "--no-cache-dir",
             "--target", str(tmp), str(final)],
            check=True, capture_output=True, text=True,
        )
        tmp.rename(PKG_DIR)
        MARKER.write_text(VERSION)
    except BaseException:
        shutil.rmtree(tmp, ignore_errors=True)
        raise
    finally:
        part.unlink(missing_ok=True)
        final.unlink(missing_ok=True)


def _spacy_load():
    if str(PKG_DIR) not in sys.path:
        sys.path.insert(0, str(PKG_DIR))
    import importlib

    importlib.invalidate_caches()
    for m in [k for k in sys.modules if k == MODEL or k.startswith(MODEL + ".")]:
        del sys.modules[m]
    import spacy

    return spacy.load(MODEL)


def _download(dest: Path) -> None:
    req = urllib.request.Request(WHEEL_URL, headers={"User-Agent": "tanulom-nlp"})
    with urllib.request.urlopen(req, timeout=30) as resp, open(dest, "wb") as out:
        total = int(resp.headers.get("Content-Length") or 0) or FALLBACK_TOTAL
        done = 0
        while chunk := resp.read(1 << 20):
            out.write(chunk)
            done += len(chunk)
            # Download is 0..0.9 of the bar; install + load take the rest.
            _set(progress=min(0.9, 0.9 * done / total))


def _load() -> None:
    global _nlp, _busy
    try:
        MODEL_DIR.mkdir(parents=True, exist_ok=True)
        if not _installed():
            _install_wheel()
        _set(status="loading", progress=0.92)
        try:
            nlp = _spacy_load()
        except Exception:  # noqa: BLE001 - a corrupt install is repaired once
            log.exception("model failed to load; reinstalling once")
            _install_wheel()
            _set(status="loading", progress=0.92)
            nlp = _spacy_load()
        with _lock:
            _nlp = nlp
        _set(status="ready", progress=1.0, error="")
        log.info("model ready: %s %s", MODEL, VERSION)
    except subprocess.CalledProcessError as exc:
        log.error("pip install failed: %s", (exc.stderr or "")[-500:])
        _set(status="error", error="install failed: " + (exc.stderr or str(exc))[-300:])
    except Exception as exc:  # noqa: BLE001 - the reason must reach /health
        log.exception("model failed to load")
        _set(status="error", error=str(exc)[:300])
    finally:
        with _lock:
            _busy = False


def start_load() -> bool:
    global _busy
    with _lock:
        if _busy or _nlp is not None:
            return False
        _busy = True
    threading.Thread(target=_load, daemon=True).start()
    return True


@asynccontextmanager
async def lifespan(_: FastAPI):
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    # An already-installed model loads at boot (no network). A missing one is
    # fetched only on /warmup.
    if _installed():
        start_load()
    yield


app = FastAPI(title="tanulom-nlp", lifespan=lifespan)


MAX_BODY = 4 * MAX_CHARS + 4096  # worst case: 4 bytes per character, plus JSON framing


@app.middleware("http")
async def cap_body(request: Request, call_next):
    """Refuse an oversized body BEFORE pydantic parses it."""
    if request.method == "POST":
        raw = request.headers.get("content-length")
        if raw is not None:
            try:
                n = int(raw)
            except ValueError:
                return JSONResponse(status_code=400, content={"error": "bad content-length"})
            if n < 0:
                return JSONResponse(status_code=400, content={"error": "bad content-length"})
            if n > MAX_BODY:
                return JSONResponse(status_code=413, content={"error": "body too large"})
        else:
            # No length (chunked): read with a cap, then hand the bytes on.
            body = b""
            async for chunk in request.stream():
                body += chunk
                if len(body) > MAX_BODY:
                    return JSONResponse(status_code=413, content={"error": "body too large"})

            async def receive():
                return {"type": "http.request", "body": body, "more_body": False}

            request = Request(request.scope, receive)
    return await call_next(request)


@app.get("/livez")
def livez() -> JSONResponse:
    """Liveness: 200 as soon as the process serves, model or not."""
    return JSONResponse(content={"alive": True})


@app.get("/health")
def health() -> JSONResponse:
    body = _report()
    return JSONResponse(status_code=200 if body["status"] == "ready" else 503, content=body)


@app.post("/warmup")
def warmup() -> JSONResponse:
    start_load()
    return JSONResponse(content=_report())


class AnalyzeRequest(BaseModel):
    text: str
    sentences: bool = False


def byte_offsets(text: str) -> list[int]:
    """offsets[i] = UTF-8 byte offset of character i; offsets[len(text)] = total."""
    out = [0] * (len(text) + 1)
    b = 0
    for i, ch in enumerate(text):
        out[i] = b
        b += len(ch.encode("utf-8", "surrogatepass"))
    out[len(text)] = b
    return out


def run_analysis(nlp, text: str, sentences: bool) -> list[dict]:
    offs = byte_offsets(text)
    doc = nlp(text)
    sent_of: dict[int, int] = {}
    if sentences:
        for si, sent in enumerate(doc.sents):
            for tok in sent:
                sent_of[tok.i] = si
    tokens = []
    for tok in doc:
        if tok.is_space:
            continue
        start, end = tok.idx, tok.idx + len(tok.text)
        item = {
            "start": offs[start],
            "end": offs[end],
            "text": tok.text,
            "lemma": tok.lemma_,
            "upos": tok.pos_,
            "feats": str(tok.morph),
        }
        if sentences:
            item["sent"] = sent_of.get(tok.i, 0)
        tokens.append(item)
    return tokens


@app.post("/analyze")
def analyze(req: AnalyzeRequest, request: Request) -> JSONResponse:
    # Sync endpoint: FastAPI runs it in its threadpool, off the event loop.
    # (Oversized bodies are already refused by the middleware before parsing.)
    if len(req.text) > MAX_CHARS:
        return JSONResponse(status_code=413, content={"error": f"text over {MAX_CHARS} characters"})
    with _lock:
        nlp = _nlp
    if nlp is None:
        return JSONResponse(status_code=503, content={"error": "model " + _report()["status"]})
    with _infer_lock:
        tokens = run_analysis(nlp, req.text, req.sentences)
    return JSONResponse(content={"model": f"{MODEL}-{VERSION}", "tokens": tokens})


if __name__ == "__main__":
    uvicorn.run(app, host=os.environ.get("NLP_HOST", "0.0.0.0"), port=PORT)
