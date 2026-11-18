# Vendored Piper runtime: the project's own mirror (ADR-080, ADR-081)

Committed so that Piper does not depend on PyPI, python-build-standalone or any third party at install time.
The installed app **downloads these files once** from this repository (`raw.githubusercontent.com/<repo>/<ref>/services/tts/vendor/...`)
and verifies each one against `MANIFEST.sha256` / `requirements.lock`, which are compiled into the binary; then it
unpacks the Python for the host platform and runs `pip install --no-index --require-hashes` from the downloaded
copy. The repository is the mirror: **the files must be pushed to the ref before a tag can install them.**

## Contents
- `MANIFEST.sha256` — `sha256  size  relative-path` for every file here (except this README, the manifest and the lock).
- `requirements.lock` — `name==version --hash=sha256:...` for exactly the wheels; one line per package, listing every platform's hash.
- `darwin-arm64/`, `linux-amd64/` (glibc >= 2.28), `linux-arm64/` — each: a python-build-standalone
  `cpython-3.12.16+20261110-<triple>-install_only_stripped.tar.gz` and the wheels, byte-identical to upstream:
  piper-tts 1.8.0 (abi3), onnxruntime 1.32.0 (cp312), numpy 2.5.5 (cp312), protobuf 7.36.2, flatbuffers 25.12.19,
  packaging 26.3, pathvalidate 3.3.1. (sympy, coloredlogs, humanfriendly, mpmath are not needed.)
- `source/` — GPL-3.0 Corresponding Source for piper-tts and the bundled espeak-ng: the PyPI sdist
  `piper_tts-1.8.0.tar.gz` (Python only: no C sources, no espeak-ng), `piper1-gpl-v1.8.0.tar.gz` (the release tag:
  `espeakbridge.c`, CMake) and `espeak-ng-724808c.tar.gz` (the commit piper1-gpl v1.8.0 pins as `GIT_TAG 724808c`).

## How it was produced
`make tts-vendor` (maintainer only, the ONLY target that uses the network; scripts/tts_vendor.py) downloaded on
2026-11-18 from pypi.org / files.pythonhosted.org (sha256 checked against the PyPI JSON digests),
github.com/astral-sh/python-build-standalone release 20261110 (checked against its upstream `SHA256SUMS`), and
github.com/OHF-Voice/piper1-gpl + espeak-ng/espeak-ng (tag archives; no upstream digest, sha256 pinned in the script).
`make tts-verify` re-hashes everything offline and also checks `requirements.lock` against the wheels.

## How it ships
It does not ship inside the binary or the Docker image (no `ttsbundle` tag, no staging step). The app downloads one
platform directory (≈ 277 MB with the voices) at Install and deletes the download cache after a successful install.
`make sync-components` copies `MANIFEST.sha256` and `requirements.lock` (and `langpack/voices/MANIFEST.sha256`) into
`backend/internal/components/ttsfetch/data/`, which the binary embeds. `source/` is never downloaded: it is the GPL
Corresponding Source and **stays in this repository at the same tag** as the object code the app fetches. Do not remove it.
For an air-gapped host, copy this tree and `langpack/voices/` and use `--tts-runtime-dir` / `--voices-dir`, or serve it
behind `--tts-base-url`.
