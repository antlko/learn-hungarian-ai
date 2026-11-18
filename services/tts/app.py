"""
Hungarian text-to-speech for the reader, on the learner's own machine (Piper).

Why this exists. The browser's speechSynthesis is the floor and stays the floor
(ADR-012), but its Hungarian voices are absent on many machines and uneven on the
rest. This service renders a sentence with one of three local Piper voices
(hu_HU anna / berta / imre, medium) so "listen to the whole text" sounds the
same everywhere. No cloud API is involved and nothing here is trained or
downloaded: the voice files are committed to the repository and found in
TTS_VOICES_DIR (ADR-079).

What the app does with the output. The Go core caches the WAV on disk and serves
it to the browser. Nothing from here becomes a card, a gloss or a graded answer.

Standard library only for serving (http.server); piper is imported lazily, on the
first voice load, so the HTTP layer runs under unittest with a stub synthesizer
(test_app.py). Environment: TTS_VOICES_DIR, TTS_HOST (default 0.0.0.0 for the standalone
container; the managed child always gets 127.0.0.1 from the Go manager),
TTS_PORT (5000), TTS_PRELOAD (`all`: load every voice at start, /health is 503
`loading` until done; default: lazy, on first use).

Endpoints
    GET  /health      200 when the service is ready and at least one voice file is
                      present, 503 otherwise (`loading`, `failed`). The body says
                      why (`status`, `detail`, `error`, `voices`).
    GET  /voices      [{"id": "anna", "sampleRate": 22050, "modelHash": "..."}]
                      Only voices whose .onnx and .onnx.json are both on disk.
                      `modelHash` changes when the model file does, so the Go
                      cache never serves audio rendered by a replaced model.
    POST /synthesize  {"text": "...", "voice": "anna", "lengthScale": 1.0}
                   -> audio/wav (16-bit mono). 400 bad input (text 1..1000 UTF-8 bytes,
                      lengthScale 0.5..3.0), 404 voice without model files,
                      413 body over 16 KB. Unknown path 404, wrong method 405.

Rendering a single word badly is a Piper property (it was trained on sentences);
the Go side therefore sends a word as "<word>." with a larger lengthScale. This
service renders exactly what it is given.
"""

from __future__ import annotations

import atexit
import hashlib
import io
import json
import logging
import math
import os
import shutil
import signal
import sys
import tempfile
import threading
import wave
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Callable

log = logging.getLogger("tts")

MAX_BODY = 16 * 1024
MAX_TEXT_BYTES = 1000
LENGTH_MIN, LENGTH_MAX = 0.5, 3.0

# Persona id -> file stem. Fixed: the app names exactly these three.
VOICES = {
    "anna": "hu_HU-anna-medium",
    "berta": "hu_HU-berta-medium",
    "imre": "hu_HU-imre-medium",
}

# synth(voice_id, text, length_scale) -> WAV bytes. Production: PiperSynth.
Synth = Callable[[str, str, float], bytes]


def _paths(voices_dir: Path, voice: str) -> tuple[Path, Path]:
    stem = VOICES[voice]
    return voices_dir / f"{stem}.onnx", voices_dir / f"{stem}.onnx.json"


def present(voices_dir: Path) -> list[str]:
    """Persona ids whose .onnx and .onnx.json are both on disk."""
    out = []
    for vid in VOICES:
        model, cfg = _paths(voices_dir, vid)
        if model.is_file() and cfg.is_file():
            out.append(vid)
    return out


_hashes: dict[Path, tuple[tuple[int, int], str]] = {}


def _model_hash(model: Path) -> str:
    """sha256 of the model file (first 16 hex), cached by size and mtime."""
    st = model.stat()
    key = (st.st_size, st.st_mtime_ns)
    cached = _hashes.get(model)
    if cached and cached[0] == key:
        return cached[1]
    h = hashlib.sha256()
    with model.open("rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    digest = h.hexdigest()[:16]
    _hashes[model] = (key, digest)
    return digest


def voice_list(voices_dir: Path) -> list[dict]:
    out = []
    for vid in present(voices_dir):
        try:
            model, cfg = _paths(voices_dir, vid)
            rate = int(json.loads(cfg.read_text(encoding="utf-8"))["audio"]["sample_rate"])
            out.append({"id": vid, "sampleRate": rate, "modelHash": _model_hash(model)})
        except Exception:  # noqa: BLE001 - a broken file is "not present", not a 500
            log.exception("voice %s unreadable", vid)
    return out


class PiperSynth:
    """Production synthesizer. piper is imported on first use, never at import."""

    def __init__(self, voices_dir: Path):
        self.voices_dir = voices_dir
        self._loaded: dict[str, object] = {}
        self._espeak_dir: str | None = None
        # One synthesis at a time: a sentence is ~0.2 s, and serialising keeps
        # the peak memory of onnxruntime/espeak-ng state bounded and makes
        # first-use voice loading race-free.
        self._lock = threading.Lock()

    def _espeak_data_dir(self) -> str:
        """The espeak-ng data directory, at a path short enough for espeak-ng.

        espeak-ng keeps its data path in a fixed ~160-byte buffer. A venv under a
        long data directory exceeds it, and the failure is a message about a
        missing `phontab` far from the cause. A short symlink in the temp dir
        avoids it.
        """
        if self._espeak_dir is not None:
            return self._espeak_dir
        from piper.phonemize_espeak import ESPEAK_DATA_DIR

        real = str(ESPEAK_DATA_DIR)
        if len(real) < 140:
            self._espeak_dir = real
            return real
        base = tempfile.mkdtemp(prefix="tts-")
        atexit.register(shutil.rmtree, base, ignore_errors=True)
        link = os.path.join(base, "espeak-ng-data")
        os.symlink(real, link)
        self._espeak_dir = link
        return link

    def _voice(self, voice: str):  # call with the lock held: a voice is never loaded twice
        v = self._loaded.get(voice)
        if v is None:
            from piper import PiperVoice

            model, cfg = _paths(self.voices_dir, voice)
            v = PiperVoice.load(model, config_path=cfg, espeak_data_dir=self._espeak_data_dir())
            self._loaded[voice] = v
            log.info("loaded voice %s", voice)
        return v

    def load(self, voice: str) -> None:
        with self._lock:
            self._voice(voice)

    def __call__(self, voice: str, text: str, length_scale: float) -> bytes:
        from piper import SynthesisConfig

        with self._lock:
            v = self._voice(voice)
            buf = io.BytesIO()
            with wave.open(buf, "wb") as w:
                v.synthesize_wav(text, w, syn_config=SynthesisConfig(length_scale=length_scale))
            return buf.getvalue()


class _Bad(Exception):
    def __init__(self, status: int, msg: str):
        super().__init__(msg)
        self.status, self.msg = status, msg


class State:
    """What /health reports. `ready` is set once preload (if any) is finished."""

    def __init__(self, voices_dir: Path, synth: Synth):
        self.voices_dir = voices_dir
        self.synth = synth
        self.ready = threading.Event()
        self.error = ""


CONN_TIMEOUT = 15.0  # seconds a connection may stall on a read


def _validate(raw: bytes, state: State) -> tuple[str, str, float]:
    try:
        req = json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, ValueError, RecursionError):  # deep nesting, bad bytes, bad JSON
        raise _Bad(400, "body must be a JSON object") from None
    if not isinstance(req, dict):
        raise _Bad(400, "body must be a JSON object")
    text, voice, scale = req.get("text"), req.get("voice"), req.get("lengthScale", 1.0)
    if not isinstance(text, str):
        raise _Bad(400, "text must be a string")
    text = text.strip()
    try:
        n = len(text.encode("utf-8"))
    except UnicodeEncodeError:  # lone surrogates
        raise _Bad(400, "text is not valid UTF-8") from None
    if not 1 <= n <= MAX_TEXT_BYTES:
        raise _Bad(400, f"text must be 1..{MAX_TEXT_BYTES} bytes")
    try:
        scale_ok = not isinstance(scale, bool) and isinstance(scale, (int, float)) \
            and math.isfinite(scale) and LENGTH_MIN <= scale <= LENGTH_MAX
    except OverflowError:  # an integer too large for a float
        scale_ok = False
    if not scale_ok:
        raise _Bad(400, f"lengthScale must be a number within {LENGTH_MIN}..{LENGTH_MAX}")
    if not isinstance(voice, str) or voice not in VOICES:
        raise _Bad(400, "unknown voice")
    if voice not in present(state.voices_dir):
        raise _Bad(404, f"voice {voice} has no model file")
    return text, voice, float(scale)


def make_handler(state: State):
    class Handler(BaseHTTPRequestHandler):
        server_version = "tanulom-tts"
        timeout = CONN_TIMEOUT  # per-connection socket timeout: a slow client cannot hold a thread
        protocol_version = "HTTP/1.1"

        def log_message(self, fmt, *args):  # stderr, through logging
            log.info("%s %s", self.address_string(), fmt % args)

        def _send(self, status: int, body: bytes, ctype: str) -> None:
            self.send_response(status)
            self.send_header("Content-Type", ctype)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def _json(self, status: int, obj) -> None:
            self._send(status, json.dumps(obj).encode("utf-8"), "application/json")

        def _route(self, method: str) -> None:
            path = self.path.split("?", 1)[0]
            known = {"/health": "GET", "/voices": "GET", "/synthesize": "POST"}
            if path not in known:
                self._drain()
                return self._json(404, {"error": "not found"})
            if known[path] != method:
                self._drain()
                self.send_response(405)
                self.send_header("Allow", known[path])
                body = json.dumps({"error": "method not allowed"}).encode()
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
                return
            if path == "/health":
                return self._health()
            if path == "/voices":
                return self._json(200, voice_list(state.voices_dir))
            self._synthesize()

        def _drain(self) -> None:
            # Small unread bodies would corrupt the next keep-alive request.
            cl = self.headers.get("Content-Length") or "0"
            try:
                n = int(cl) if cl.isascii() and cl.isdigit() else 0
            except ValueError:
                n = 0
            try:
                if 0 < n <= MAX_BODY:
                    self.rfile.read(n)
                elif n:
                    self.close_connection = True
            except OSError:  # timed out while draining
                self.close_connection = True

        def _health(self) -> None:
            have = present(state.voices_dir)
            if state.error:
                return self._json(503, {"status": "failed", "detail": state.error, "error": state.error, "voices": have})
            if not state.ready.is_set():
                return self._json(503, {"status": "loading", "detail": "loading voices", "error": "", "voices": have})
            if not have:
                msg = "no hu_HU-*-medium voice files found"
                return self._json(503, {"status": "failed", "detail": msg, "error": msg, "voices": []})
            self._json(200, {"status": "ready", "detail": "", "error": "", "voices": have})

        def _synthesize(self) -> None:
            cl = self.headers.get("Content-Length")
            try:
                # ASCII digits only: str.isdigit() also accepts e.g. superscripts and Arabic-Indic digits.
                if cl is None or not (cl.isascii() and cl.isdigit()):
                    raise ValueError
                n = int(cl)
            except ValueError:  # includes the interpreter's int-string length limit
                self.close_connection = True
                return self._json(411, {"error": "Content-Length required"})
            if n > MAX_BODY:
                self.close_connection = True  # the body is never read
                return self._json(413, {"error": f"body over {MAX_BODY} bytes"})
            try:
                raw = self.rfile.read(n)
            except OSError:  # socket timeout: the client stalled mid-body
                self.close_connection = True
                return self._json(408, {"error": "request body timed out"})
            if len(raw) != n:
                self.close_connection = True
                return self._json(400, {"error": "body shorter than Content-Length"})
            try:
                text, voice, scale = _validate(raw, state)
            except _Bad as b:
                return self._json(b.status, {"error": b.msg})
            try:
                wav = state.synth(voice, text, scale)
            except Exception as exc:  # noqa: BLE001 - the reason must reach the Go log
                log.exception("synthesis failed")
                return self._json(500, {"error": str(exc)})
            self._send(200, wav, "audio/wav")

        def do_GET(self):
            self._route("GET")

        def do_POST(self):
            self._route("POST")

        def do_PUT(self):
            self._route("PUT")

        def do_DELETE(self):
            self._route("DELETE")

        def do_PATCH(self):
            self._route("PATCH")

        def do_HEAD(self):
            self._route("HEAD")

    return Handler


class Server(ThreadingHTTPServer):
    daemon_threads = True
    allow_reuse_address = True
    request_queue_size = 64  # the default backlog of 5 drops bursts of parallel requests


def make_server(host: str, port: int, voices_dir: Path, synth: Synth) -> tuple[Server, State]:
    state = State(voices_dir, synth)
    return Server((host, port), make_handler(state)), state


def main() -> int:
    logging.basicConfig(level=logging.INFO, stream=sys.stderr, format="%(asctime)s %(levelname)s %(message)s")
    voices_dir = Path(os.environ.get("TTS_VOICES_DIR", "/voices"))
    host = os.environ.get("TTS_HOST", "0.0.0.0")
    port = int(os.environ.get("TTS_PORT", "5000"))
    preload = os.environ.get("TTS_PRELOAD", "").strip().lower() == "all"

    synth = PiperSynth(voices_dir)
    srv, state = make_server(host, port, voices_dir, synth)

    def stop(*_):
        # shutdown() blocks until serve_forever returns, so never call it on that thread.
        threading.Thread(target=srv.shutdown, daemon=True).start()

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)

    def warm() -> None:
        try:
            if preload:
                for vid in present(voices_dir):
                    synth.load(vid)
        except Exception as exc:  # noqa: BLE001
            log.exception("preload failed")
            state.error = f"preload failed: {exc}"
        state.ready.set()

    if preload:
        threading.Thread(target=warm, daemon=True).start()
    else:
        state.ready.set()
    log.info("listening on %s:%d (preload=%s)", host, port, "all" if preload else "off")
    srv.serve_forever()
    srv.server_close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
