"""Pinned, checksummed source downloads.

Reproducibility over convenience: a source whose checksum has drifted is a build
failure, not a warning. A pack is a shipped artifact that learner state points at,
so "it built, but from slightly different data than last time" is not acceptable.

Checksums are recorded on first fetch (`--record-checksums`) and enforced
thereafter. Every source and its licence is listed in ../LICENSES.md.
"""

from __future__ import annotations

import hashlib
import json
import shutil
import subprocess
from dataclasses import dataclass
from pathlib import Path

CACHE = Path(__file__).resolve().parents[1] / "cache"
CHECKSUMS = Path(__file__).resolve().parents[1] / "checksums.json"


@dataclass(frozen=True)
class Source:
    key: str
    url: str
    filename: str
    licence: str
    attribution: str
    required: bool = True


SOURCES: list[Source] = [
    Source(
        key="frequency",
        url="https://raw.githubusercontent.com/hermitdave/FrequencyWords/master/content/2018/hu/hu_50k.txt",
        filename="hu_50k.txt",
        licence="CC BY-SA 4.0",
        attribution="Hermit Dave, FrequencyWords (OpenSubtitles via OPUS)",
    ),
    Source(
        key="tatoeba_sentences",
        url="https://downloads.tatoeba.org/exports/per_language/hun/hun_sentences.tsv.bz2",
        filename="hun_sentences.tsv.bz2",
        licence="CC BY 2.0 FR",
        attribution="Tatoeba Project contributors",
    ),
    Source(
        key="tatoeba_links_eng",
        url="https://downloads.tatoeba.org/exports/per_language/hun/hun-eng_links.tsv.bz2",
        filename="hun-eng_links.tsv.bz2",
        licence="CC BY 2.0 FR",
        attribution="Tatoeba Project contributors",
    ),
    Source(
        key="tatoeba_links_rus",
        url="https://downloads.tatoeba.org/exports/per_language/hun/hun-rus_links.tsv.bz2",
        filename="hun-rus_links.tsv.bz2",
        licence="CC BY 2.0 FR",
        attribution="Tatoeba Project contributors",
    ),
    # The links files give hu_id -> other_id only. Rendering a translation needs
    # the other language's sentence text, which lives in its own export.
    Source(
        key="tatoeba_eng",
        url="https://downloads.tatoeba.org/exports/per_language/eng/eng_sentences.tsv.bz2",
        filename="eng_sentences.tsv.bz2",
        licence="CC BY 2.0 FR",
        attribution="Tatoeba Project contributors",
        required=False,
    ),
    Source(
        key="tatoeba_rus",
        url="https://downloads.tatoeba.org/exports/per_language/rus/rus_sentences.tsv.bz2",
        filename="rus_sentences.tsv.bz2",
        licence="CC BY 2.0 FR",
        attribution="Tatoeba Project contributors",
        required=False,
    ),
    Source(
        key="wiktionary",
        url="https://kaikki.org/dictionary/Hungarian/kaikki.org-dictionary-Hungarian.jsonl",
        filename="kaikki-hu.jsonl",
        licence="CC BY-SA 3.0 / GFDL",
        attribution="Wiktionary contributors, English Wiktionary (via wiktextract)",
        # ~580 MB. The pack is buildable without it -- lemmas then carry no
        # glosses, which the manifest records so the gap is visible rather than
        # silently shipped.
        required=False,
    ),
]


def path_for(source: Source) -> Path:
    return CACHE / source.filename


def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def load_checksums() -> dict[str, str]:
    if CHECKSUMS.exists():
        return json.loads(CHECKSUMS.read_text())
    return {}


def save_checksums(sums: dict[str, str]) -> None:
    CHECKSUMS.write_text(json.dumps(sums, indent=2, sort_keys=True) + "\n")


def fetch(source: Source, *, force: bool = False) -> Path | None:
    """Download a source if absent. Returns None when an optional one is missing."""
    dest = path_for(source)
    CACHE.mkdir(parents=True, exist_ok=True)

    if dest.exists() and not force:
        return dest
    if shutil.which("curl") is None:
        raise RuntimeError("curl is required to fetch sources")

    print(f"  fetching {source.key} <- {source.url}")
    result = subprocess.run(
        ["curl", "-sL", "--retry", "3", "--fail", "-o", str(dest), source.url],
        check=False,
    )
    if result.returncode != 0:
        dest.unlink(missing_ok=True)
        if source.required:
            raise RuntimeError(f"failed to fetch required source {source.key}")
        print(f"  ! optional source {source.key} unavailable; continuing without it")
        return None
    return dest


def verify(source: Source, *, record: bool = False) -> None:
    """Enforce the recorded checksum, or record it on first fetch."""
    dest = path_for(source)
    if not dest.exists():
        if source.required:
            raise RuntimeError(f"required source {source.key} is missing")
        return

    sums = load_checksums()
    actual = sha256(dest)

    if record or source.key not in sums:
        sums[source.key] = actual
        save_checksums(sums)
        return

    if sums[source.key] != actual:
        raise RuntimeError(
            f"checksum mismatch for {source.key}\n"
            f"  expected {sums[source.key]}\n"
            f"  actual   {actual}\n"
            "The upstream data changed. Review the diff, then re-record with "
            "--record-checksums if the change is intended."
        )


def fetch_all(*, force: bool = False, record: bool = False) -> dict[str, Path | None]:
    out: dict[str, Path | None] = {}
    for source in SOURCES:
        out[source.key] = fetch(source, force=force)
        verify(source, record=record)
    return out
