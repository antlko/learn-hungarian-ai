"""HTTP-layer tests for app.py with a stub synthesizer. Stdlib only:
    python3 -I -m unittest services/tts/test_app.py
"""

import http.client
import io
import json
import sys
import tempfile
import threading
import time
import unittest
import wave
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import app  # noqa: E402


def tiny_wav() -> bytes:
    buf = io.BytesIO()
    with wave.open(buf, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(22050)
        w.writeframes(b"\x00\x00" * 2205)
    return buf.getvalue()


class Base(unittest.TestCase):
    present = ("anna", "berta")
    delay = 0.0

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        d = Path(self.tmp.name)
        for vid in self.present:
            (d / f"{app.VOICES[vid]}.onnx").write_bytes(b"model-" + vid.encode())
            (d / f"{app.VOICES[vid]}.onnx.json").write_text(json.dumps({"audio": {"sample_rate": 22050}}))
        self.calls = []
        self.active = 0
        self.peak = 0
        self.mu = threading.Lock()

        def synth(voice, text, scale):
            with self.mu:
                self.active += 1
                self.peak = max(self.peak, self.active)
                self.calls.append((voice, text, scale))
            time.sleep(self.delay)
            with self.mu:
                self.active -= 1
            return tiny_wav()

        self.srv, self.state = app.make_server("127.0.0.1", 0, d, synth)
        self.state.ready.set()
        self.port = self.srv.server_address[1]
        self.t = threading.Thread(target=self.srv.serve_forever, daemon=True)
        self.t.start()

    def tearDown(self):
        self.srv.shutdown()
        self.srv.server_close()
        self.tmp.cleanup()

    def req(self, method, path, body=None, headers=None):
        c = http.client.HTTPConnection("127.0.0.1", self.port, timeout=10)
        data = body if isinstance(body, (bytes, type(None))) else json.dumps(body).encode()
        c.request(method, path, body=data, headers=headers or {})
        r = c.getresponse()
        out = (r.status, r.getheader("Content-Type"), r.read())
        c.close()
        return out

    def synth(self, **kw):
        body = {"text": "Szia", "voice": "anna", "lengthScale": 1.0}
        body.update(kw)
        return self.req("POST", "/synthesize", body)


class TestHTTP(Base):
    def test_health_gating(self):
        s, _, b = self.req("GET", "/health")
        self.assertEqual(s, 200)
        self.assertEqual(json.loads(b)["voices"], ["anna", "berta"])
        self.state.ready.clear()
        s, _, b = self.req("GET", "/health")
        self.assertEqual((s, json.loads(b)["status"]), (503, "loading"))
        self.state.error = "boom"
        s, _, b = self.req("GET", "/health")
        self.assertEqual((s, json.loads(b)["status"], json.loads(b)["error"]), (503, "failed", "boom"))

    def test_voices_shape(self):
        s, ct, b = self.req("GET", "/voices")
        self.assertEqual((s, ct), (200, "application/json"))
        vs = json.loads(b)
        self.assertEqual([v["id"] for v in vs], ["anna", "berta"])
        for v in vs:
            self.assertEqual(set(v), {"id", "sampleRate", "modelHash"})
            self.assertEqual(v["sampleRate"], 22050)
            self.assertEqual(len(v["modelHash"]), 16)
        self.assertNotEqual(vs[0]["modelHash"], vs[1]["modelHash"])

    def test_synthesize_ok(self):
        s, ct, b = self.synth(text="  Szia világ.  ", lengthScale=1.5)
        self.assertEqual((s, ct), (200, "audio/wav"))
        self.assertEqual(b[:4], b"RIFF")
        self.assertEqual(self.calls, [("anna", "Szia világ.", 1.5)])

    def test_default_length_scale(self):
        s, _, _ = self.req("POST", "/synthesize", {"text": "x", "voice": "imre"})
        self.assertEqual(s, 404)  # persona without files
        s, _, _ = self.req("POST", "/synthesize", {"text": "x", "voice": "anna"})
        self.assertEqual((s, self.calls[-1][2]), (200, 1.0))

    def test_bad_input_400(self):
        bad = [
            dict(text=""), dict(text="   "), dict(text=5), dict(text="a" * 1001),
            dict(text="é" * 501),  # 1002 bytes, 501 chars
            dict(voice="nobody"), dict(voice=None), dict(lengthScale=0.49), dict(lengthScale=3.01),
            dict(lengthScale="1"), dict(lengthScale=True), dict(lengthScale=None),
        ]
        for kw in bad:
            with self.subTest(kw=kw):
                self.assertEqual(self.synth(**kw)[0], 400)
        for raw in (b"not json", b"[1]", b"\xff\xfe", b"NaN"):
            with self.subTest(raw=raw):
                self.assertEqual(self.req("POST", "/synthesize", raw)[0], 400)
        self.assertEqual(self.req("POST", "/synthesize", b'{"text":"a","voice":"anna","lengthScale":NaN}')[0], 400)
        self.assertEqual(self.calls, [])

    def test_bounds_accepted(self):
        for ls in (0.5, 3.0, 2):
            self.assertEqual(self.synth(lengthScale=ls)[0], 200)
        self.assertEqual(self.synth(text="a" * 1000)[0], 200)
        self.assertEqual(self.synth(text="é" * 500)[0], 200)

    def test_413(self):
        s, _, _ = self.req("POST", "/synthesize", b"x" * (16 * 1024 + 1))
        self.assertEqual(s, 413)
        self.assertEqual(self.synth()[0], 200)  # server still fine

    def test_411_without_length(self):
        c = http.client.HTTPConnection("127.0.0.1", self.port, timeout=10)
        c.putrequest("POST", "/synthesize")
        c.endheaders()
        self.assertEqual(c.getresponse().status, 411)
        c.close()

    def test_404_405(self):
        self.assertEqual(self.req("GET", "/nope")[0], 404)
        self.assertEqual(self.req("POST", "/health", b"{}")[0], 405)
        self.assertEqual(self.req("GET", "/synthesize")[0], 405)
        self.assertEqual(self.req("PUT", "/voices", b"{}")[0], 405)
        self.assertEqual(self.req("DELETE", "/synthesize")[0], 405)

    def test_synth_failure_500(self):
        def boom(*_):
            raise RuntimeError("no model")

        self.state.synth = boom
        s, _, b = self.synth()
        self.assertEqual((s, json.loads(b)["error"]), (500, "no model"))


class TestHardening(Base):
    def raw(self, data: bytes) -> bytes:
        import socket
        s = socket.create_connection(("127.0.0.1", self.port), timeout=10)
        s.sendall(data)
        out = b""
        try:
            while True:
                chunk = s.recv(4096)
                if not chunk:
                    break
                out += chunk
                if b"\r\n\r\n" in out and b"Content-Length" in out:
                    break
        except OSError:
            pass
        s.close()
        return out

    def test_content_length_must_be_ascii_digits(self):
        for cl in ("\u00b2", "\u0663", "1 2", "-1", "+5"):
            out = self.raw(("POST /synthesize HTTP/1.1\r\nHost: x\r\nContent-Length: %s\r\n\r\n" % cl).encode("utf-8"))
            self.assertTrue(out.startswith(b"HTTP/1.1 411") or out.startswith(b"HTTP/1.1 400"), (cl, out[:40]))
        self.assertEqual(self.synth()[0], 200)

    def test_absurdly_long_content_length_is_refused_not_a_crash(self):
        out = self.raw(b"POST /synthesize HTTP/1.1\r\nHost: x\r\nContent-Length: " + b"9" * 5000 + b"\r\n\r\n")
        self.assertTrue(out.startswith(b"HTTP/1.1 411"), out[:40])
        self.assertEqual(self.synth()[0], 200)

    def test_huge_integer_length_scale_is_400(self):
        body = b'{"text":"Szia","voice":"anna","lengthScale":' + b"9" * 400 + b"}"
        self.assertEqual(self.req("POST", "/synthesize", body, {"Content-Type": "application/json"})[0], 400)
        self.assertEqual(self.synth()[0], 200)

    def test_deeply_nested_json_is_400(self):
        body = b"[" * 8000 + b"]" * 8000
        self.assertEqual(self.req("POST", "/synthesize", body)[0], 400)
        self.assertEqual(self.synth()[0], 200)

    def test_invalid_utf8_and_bad_json_are_400(self):
        self.assertEqual(self.req("POST", "/synthesize", b"\xff\xfe{")[0], 400)
        self.assertEqual(self.req("POST", "/synthesize", b"{not json")[0], 400)
        self.assertEqual(self.req("POST", "/synthesize", b'{"text":"\\ud800","voice":"anna"}')[0], 400)

    def test_handler_has_a_connection_timeout(self):
        self.assertEqual(app.CONN_TIMEOUT, 15.0)
        self.assertEqual(self.srv.RequestHandlerClass.timeout, app.CONN_TIMEOUT)


class TestSlowClient(unittest.TestCase):
    def test_a_stalled_body_releases_the_thread(self):
        import socket
        old = app.CONN_TIMEOUT
        app.CONN_TIMEOUT = 0.5
        try:
            with tempfile.TemporaryDirectory() as d:
                srv, state = app.make_server("127.0.0.1", 0, Path(d), lambda *a: tiny_wav())
                state.ready.set()
                threading.Thread(target=srv.serve_forever, daemon=True).start()
                s = socket.create_connection(("127.0.0.1", srv.server_address[1]), timeout=10)
                s.sendall(b"POST /synthesize HTTP/1.1\r\nHost: x\r\nContent-Length: 100\r\n\r\nabc")
                t0 = time.time()
                data = b""
                while True:
                    chunk = s.recv(4096)
                    if not chunk:
                        break
                    data += chunk
                self.assertLess(time.time() - t0, 5)
                self.assertIn(b"408", data.split(b"\r\n")[0])
                s.close()
                srv.shutdown()
                srv.server_close()
        finally:
            app.CONN_TIMEOUT = old


class TestConcurrent(Base):
    delay = 0.05

    def test_concurrent_requests(self):
        res = []

        def go(i):
            res.append(self.synth(text=f"mondat {i}", voice="anna" if i % 2 else "berta")[0])

        ts = [threading.Thread(target=go, args=(i,)) for i in range(12)]
        for t in ts:
            t.start()
        for t in ts:
            t.join()
        self.assertEqual(res, [200] * 12)
        self.assertEqual(len(self.calls), 12)
        self.assertGreater(self.peak, 1)  # the HTTP layer itself is concurrent
        self.assertEqual(self.req("GET", "/health")[0], 200)  # and /health is not blocked


if __name__ == "__main__":
    unittest.main()
