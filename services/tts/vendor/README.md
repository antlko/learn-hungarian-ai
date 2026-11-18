# Vendored offline Piper runtime (ADR-080)

Committed so that Piper keeps working with no network, forever, even if PyPI or GitHub disappear.
Nothing in this directory is fetched at install time; the app verifies it against `MANIFEST.sha256`,
unpacks the Python for the host platform and `pip install --no-index --require-hashes` the wheels
from the same directory.

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
`make tts-stage` (`scripts/tts_stage.sh`, also run by the Dockerfile `full` stage) copies the manifest, the lock, **one**
platform directory and the three voices into the gitignored `backend/internal/components/ttsbundle/data/`, which the
`-tags ttsbundle` build embeds (≈ 265 MB for darwin-arm64; release binary ≈ 311 MB). `source/` is never embedded; the
Docker `full` image carries it under `/usr/share/doc/tanulom/tts-source/`. The app verifies only the running
platform's files against `MANIFEST.sha256`.
