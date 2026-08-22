"""Tatoeba sentences and their translations.

Example sentences are the unit this app actually studies -- the source brief is
explicit that cards must be sentences, never bare words (§3), because a bare
lemma teaches nothing about the morphology that carries the meaning.

Licensing: sentence text is CC BY 2.0 FR. **Audio licences are per clip**, chosen
by the individual contributor, so audio is not imported here at all; the HVPT
rendering stage handles it with an explicit per-clip check.
"""

from __future__ import annotations

import bz2
from collections.abc import Iterator
from dataclasses import dataclass
from pathlib import Path


@dataclass
class Sentence:
    id: int
    text: str
    translations: dict[str, str]


def _open_maybe_bz2(path: Path):
    if path.suffix == ".bz2":
        return bz2.open(path, "rt", encoding="utf-8", errors="replace")
    return path.open("r", encoding="utf-8", errors="replace")


def load_sentences(path: Path) -> dict[int, str]:
    """`id \\t lang \\t text` -> {id: text}."""
    out: dict[int, str] = {}
    with _open_maybe_bz2(path) as fh:
        for line in fh:
            parts = line.rstrip("\n").split("\t")
            if len(parts) < 3:
                continue
            try:
                out[int(parts[0])] = parts[2]
            except ValueError:
                continue
    return out


def load_links(path: Path) -> dict[int, list[int]]:
    """`hu_id \\t other_id` -> {hu_id: [other_id, ...]}."""
    out: dict[int, list[int]] = {}
    with _open_maybe_bz2(path) as fh:
        for line in fh:
            parts = line.rstrip("\n").split("\t")
            if len(parts) < 2:
                continue
            try:
                a, b = int(parts[0]), int(parts[1])
            except ValueError:
                continue
            out.setdefault(a, []).append(b)
    return out


def build(
    hu_path: Path,
    link_paths: dict[str, tuple[Path, Path]],
    *,
    min_tokens: int = 3,
    max_tokens: int = 20,
) -> Iterator[Sentence]:
    """Join Hungarian sentences to their translations.

    Length bounds match the `n+1` scorer's target window (6-14 tokens) with slack
    either side: shorter sentences carry too little context to teach a suffix,
    longer ones stop being comprehensible input.
    """
    hu = load_sentences(hu_path)

    translations: dict[int, dict[str, str]] = {}
    for lang, (links_path, other_path) in link_paths.items():
        links = load_links(links_path)
        other = load_sentences(other_path)
        for hu_id, targets in links.items():
            for t in targets:
                if t in other:
                    translations.setdefault(hu_id, {}).setdefault(lang, other[t])
                    break

    for sid, text in hu.items():
        n = len(text.split())
        if n < min_tokens or n > max_tokens:
            continue
        yield Sentence(id=sid, text=text, translations=translations.get(sid, {}))


def difficulty(text: str, known_ranks: dict[str, int]) -> float:
    """A crude 0-1 difficulty from the rarest word in the sentence.

    Replaced by the real `n+1` scorer once a learner has state; this only orders
    the pack's own material before anyone has studied anything.
    """
    ranks = [known_ranks.get(w.strip(".,!?;:\"'»«").lower(), 20_000) for w in text.split()]
    if not ranks:
        return 1.0
    hardest = max(ranks)
    return min(1.0, hardest / 20_000)
