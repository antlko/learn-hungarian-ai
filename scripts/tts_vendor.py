#!/usr/bin/env python3
"""Vendor / verify the offline Piper runtime under services/tts/vendor (ADR-080).

  tts_vendor.py vendor   NETWORK. Maintainer-run via `make tts-vendor`. Downloads only
                         from pypi.org / files.pythonhosted.org / github.com (astral-sh/python-build-standalone,
                         OHF-Voice/piper1-gpl, espeak-ng/espeak-ng), verifies the PyPI sha256
                         digests and the upstream SHA256SUMS, rewrites MANIFEST.sha256
                         and requirements.lock.
  tts_vendor.py voices   OFFLINE. Rewrites langpack/voices/MANIFEST.sha256 (sha256, size, name of the
                         three hu_HU-*-medium.onnx and .onnx.json). `vendor` calls it too.
  tts_vendor.py verify   OFFLINE. Re-hashes every vendored file against MANIFEST.sha256
                         and requirements.lock against the wheels. Exit 1 on any drift.

Standard library only. Output is deterministic: versions and filenames are pinned here.
"""
import hashlib
import json
import os
import re
import sys
import urllib.request

ROOT = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
VENDOR = os.path.join(ROOT, "services", "tts", "vendor")
VOICES = os.path.join(ROOT, "langpack", "voices")
VOICES_MANIFEST = os.path.join(VOICES, "MANIFEST.sha256")
PERSONAS = ("anna", "berta", "imre")

PBS_TAG = "20261110"
PBS_PY = "3.12.16"
PBS_BASE = "https://github.com/astral-sh/python-build-standalone/releases/download/" + PBS_TAG
PLATFORMS = {  # dir -> python-build-standalone triple
    "darwin-arm64": "aarch64-apple-darwin",
    "linux-amd64": "x86_64-unknown-linux-gnu",
    "linux-arm64": "aarch64-unknown-linux-gnu",
}
COMMON = {  # name -> (version, wheel filename), identical on every platform
    "flatbuffers": ("25.12.19", "flatbuffers-25.12.19-py2.py3-none-any.whl"),
    "packaging": ("26.3", "packaging-26.3-py3-none-any.whl"),
    "pathvalidate": ("3.3.1", "pathvalidate-3.3.1-py3-none-any.whl"),
}
WHEELS = {  # platform -> {name: (version, filename)}
    "darwin-arm64": {
        "numpy": ("2.5.5", "numpy-2.5.5-cp312-cp312-macosx_14_0_arm64.whl"),
        "onnxruntime": ("1.32.0", "onnxruntime-1.32.0-cp312-cp312-macosx_14_0_arm64.whl"),
        "piper-tts": ("1.8.0", "piper_tts-1.8.0-cp39-abi3-macosx_11_0_arm64.whl"),
        "protobuf": ("7.36.2", "protobuf-7.36.2-cp310-abi3-macosx_10_9_universal2.whl"),
    },
    "linux-amd64": {
        "numpy": ("2.5.5", "numpy-2.5.5-cp312-cp312-manylinux_2_27_x86_64.manylinux_2_28_x86_64.whl"),
        "onnxruntime": ("1.32.0", "onnxruntime-1.32.0-cp312-cp312-manylinux_2_28_x86_64.whl"),
        "piper-tts": ("1.8.0", "piper_tts-1.8.0-cp39-abi3-manylinux_2_17_x86_64.manylinux2014_x86_64.manylinux_2_28_x86_64.whl"),
        "protobuf": ("7.36.2", "protobuf-7.36.2-cp310-abi3-manylinux2014_x86_64.whl"),
    },
    "linux-arm64": {
        "numpy": ("2.5.5", "numpy-2.5.5-cp312-cp312-manylinux_2_27_aarch64.manylinux_2_28_aarch64.whl"),
        "onnxruntime": ("1.32.0", "onnxruntime-1.32.0-cp312-cp312-manylinux_2_28_aarch64.whl"),
        "piper-tts": ("1.8.0", "piper_tts-1.8.0-cp39-abi3-manylinux_2_17_aarch64.manylinux2014_aarch64.manylinux_2_28_aarch64.whl"),
        "protobuf": ("7.36.2", "protobuf-7.36.2-cp310-abi3-manylinux2014_aarch64.whl"),
    },
}
SDIST = ("piper-tts", "1.8.0", "piper_tts-1.8.0.tar.gz")
# GPL Corresponding Source: the PyPI sdist has NO C sources (espeakbridge.c, CMakeLists.txt) and no
# espeak-ng, so the piper1-gpl release tag and the espeak-ng commit it pins (GIT_TAG 724808c) are
# vendored too. GitHub publishes no digest for tag archives: these are pinned here (trust on first
# use, identical on two fetches) and from then on enforced by MANIFEST.sha256.
GIT_SOURCES = {
    "piper1-gpl-v1.8.0.tar.gz": (
        "https://github.com/OHF-Voice/piper1-gpl/archive/refs/tags/v1.8.0.tar.gz",
        "0a22987a6157f14e2fb7cf4aa702b547467fd0448bea9f233abd503c3ff4a0e1"),
    "espeak-ng-724808c.tar.gz": (
        "https://github.com/espeak-ng/espeak-ng/archive/724808c5a83f9ef95fdd0db886ba7ba537ff224a.tar.gz",
        "7d8ebd78201923a443245362a928999977d1721c3eeba596eafc6ea8db7e243c"),
}
ALLOWED = ("https://pypi.org/", "https://files.pythonhosted.org/",
           "https://github.com/astral-sh/python-build-standalone/",
           "https://github.com/OHF-Voice/piper1-gpl/", "https://github.com/espeak-ng/espeak-ng/")
MANIFEST = os.path.join(VENDOR, "MANIFEST.sha256")
LOCK = os.path.join(VENDOR, "requirements.lock")


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def fetch(url):
    if not url.startswith(ALLOWED):
        sys.exit("refusing non-allowlisted URL: " + url)
    req = urllib.request.Request(url, headers={"User-Agent": "tanulom-tts-vendor"})
    with urllib.request.urlopen(req, timeout=120) as r:  # follows redirects (github -> objects)
        return r.read()


def put(path, data, want_sha):
    got = hashlib.sha256(data).hexdigest()
    if got != want_sha:
        sys.exit("sha256 mismatch for %s: upstream %s, downloaded %s" % (path, want_sha, got))
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "wb") as f:
        f.write(data)


def pypi_files(name, version):
    meta = json.loads(fetch("https://pypi.org/pypi/%s/%s/json" % (name, version)))
    return {u["filename"]: u for u in meta["urls"]}


def vendor():
    sums = {}
    for line in fetch(PBS_BASE + "/SHA256SUMS").decode().splitlines():
        parts = line.split()
        if len(parts) == 2:
            sums[parts[1].lstrip("*")] = parts[0]
    cache = {}

    def pypi(name, version, filename):
        files = cache.setdefault((name, version), pypi_files(name, version))
        if filename not in files:
            sys.exit("PyPI has no %s for %s %s" % (filename, name, version))
        return files[filename]

    for plat, triple in PLATFORMS.items():
        d = os.path.join(VENDOR, plat)
        tar = "cpython-%s+%s-%s-install_only_stripped.tar.gz" % (PBS_PY, PBS_TAG, triple)
        if tar not in sums:
            sys.exit("SHA256SUMS lacks " + tar)
        print("python", plat, tar, flush=True)
        put(os.path.join(d, tar), fetch(PBS_BASE + "/" + tar.replace("+", "%2B")), sums[tar])
        for name, (ver, fn) in sorted({**COMMON, **WHEELS[plat]}.items()):
            u = pypi(name, ver, fn)
            print("wheel ", plat, fn, flush=True)
            put(os.path.join(d, fn), fetch(u["url"]), u["digests"]["sha256"])
    name, ver, fn = SDIST
    u = pypi(name, ver, fn)
    print("source", fn, flush=True)
    put(os.path.join(VENDOR, "source", fn), fetch(u["url"]), u["digests"]["sha256"])
    for fn, (url, want) in sorted(GIT_SOURCES.items()):
        print("source", fn, flush=True)
        put(os.path.join(VENDOR, "source", fn), fetch(url), want)
    write_lock()
    write_manifest()
    write_voices_manifest()


def all_files():
    out = []
    for dp, _, fns in os.walk(VENDOR):
        for fn in fns:
            rel = os.path.relpath(os.path.join(dp, fn), VENDOR)
            if rel not in ("MANIFEST.sha256", "README.md", "requirements.lock"):
                out.append(rel)
    return sorted(out)


def wheel_hashes():
    """name -> (version, sorted set of sha256) over every vendored wheel."""
    res = {}
    for plat in PLATFORMS:
        for name, (ver, fn) in {**COMMON, **WHEELS[plat]}.items():
            p = os.path.join(VENDOR, plat, fn)
            res.setdefault(name, (ver, set()))[1].add(sha256(p))
    return res


def write_lock():
    lines = ["# Generated by `make tts-vendor` (scripts/tts_vendor.py). Do not edit.",
             "# One entry per package; every platform's wheel hash is listed, pip takes the one it finds."]
    for name, (ver, hs) in sorted(wheel_hashes().items()):
        lines.append("%s==%s %s" % (name, ver, " ".join("--hash=sha256:" + h for h in sorted(hs))))
    with open(LOCK, "w") as f:
        f.write("\n".join(lines) + "\n")


def write_manifest():
    with open(MANIFEST, "w") as f:
        for rel in all_files():
            f.write("%s  %d  %s\n" % (sha256(os.path.join(VENDOR, rel)), os.path.getsize(os.path.join(VENDOR, rel)), rel))


def voice_files():
    out = []
    for p in PERSONAS:
        out += ["hu_HU-%s-medium.onnx" % p, "hu_HU-%s-medium.onnx.json" % p]
    return out


def write_voices_manifest():
    with open(VOICES_MANIFEST, "w") as f:
        for fn in voice_files():
            p = os.path.join(VOICES, fn)
            f.write("%s  %d  %s\n" % (sha256(p), os.path.getsize(p), fn))


def verify_voices(bad):
    if not os.path.exists(VOICES_MANIFEST):
        bad.append("langpack/voices/MANIFEST.sha256 missing; run `make tts-vendor`")
        return
    listed = {}
    for line in open(VOICES_MANIFEST).read().splitlines():
        h, size, fn = line.split("  ", 2)
        listed[fn] = (h, int(size))
    if sorted(listed) != sorted(voice_files()):
        bad.append("voices manifest lists the wrong files")
    for fn, (h, size) in sorted(listed.items()):
        p = os.path.join(VOICES, fn)
        if not os.path.exists(p):
            bad.append("missing: voices/" + fn)
        elif os.path.getsize(p) != size or sha256(p) != h:
            bad.append("voice differs from its manifest: " + fn)


def verify():
    bad = []
    if not os.path.exists(MANIFEST):
        sys.exit("tts-verify: MANIFEST.sha256 missing; run `make tts-vendor`")
    listed = {}
    for line in open(MANIFEST).read().splitlines():
        h, size, rel = line.split("  ", 2)
        listed[rel] = (h, int(size))
    for rel, (h, size) in sorted(listed.items()):
        p = os.path.join(VENDOR, rel)
        if not os.path.exists(p):
            bad.append("missing: " + rel)
        elif os.path.getsize(p) != size:
            bad.append("size differs: " + rel)
        elif sha256(p) != h:
            bad.append("sha256 differs: " + rel)
    for rel in all_files():
        if rel not in listed:
            bad.append("not in MANIFEST: " + rel)
    verify_voices(bad)
    # requirements.lock must equal the wheels' hashes
    if not bad:
        want = {}
        for line in open(LOCK).read().splitlines():
            m = re.match(r"^([A-Za-z0-9_.-]+)==(\S+)((?: --hash=sha256:[0-9a-f]{64})+)$", line)
            if m:
                want[m.group(1)] = (m.group(2), set(x.split(":")[1] for x in m.group(3).split()))
        if want != wheel_hashes():
            bad.append("requirements.lock does not match the vendored wheels")
    if bad:
        print("tts-verify FAILED:\n  " + "\n  ".join(bad), file=sys.stderr)
        sys.exit(1)
    total = sum(s for _, s in listed.values())
    print("tts-verify OK: %d files, %.1f MB" % (len(listed), total / 1e6))


if __name__ == "__main__":
    if len(sys.argv) != 2 or sys.argv[1] not in ("vendor", "verify", "voices"):
        sys.exit(__doc__)
    {"vendor": vendor, "verify": verify, "voices": write_voices_manifest}[sys.argv[1]]()
