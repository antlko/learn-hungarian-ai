"""Compact word-list syntax for the course's growth batch and its theme units.

The month files spell every word as a five-field tuple, which is the right shape
for forty hand-argued words and a poor one for several hundred. A line here is

    lemma|N|русский перевод|English gloss|optional note

`N V A D M` expand to NOUN VERB ADJ ADV NUM; any other tag (INTJ, PRON, DET,
ADP, CCONJ …) is written out in full. Blank lines and `#` comments are ignored.
The result is exactly the `(lemma, upos, gloss_ru, gloss_en, note)` tuples the
month files use, so nothing downstream knows the difference.
"""

from __future__ import annotations

_UPOS = {"N": "NOUN", "V": "VERB", "A": "ADJ", "D": "ADV", "M": "NUM"}


def parse(text: str) -> list[tuple[str, str, str, str, str]]:
    out = []
    for n, raw in enumerate(text.splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        parts = [p.strip() for p in line.split("|")]
        if len(parts) not in (4, 5):
            raise ValueError(f"word list line {n}: expected 4 or 5 fields: {raw!r}")
        lemma, upos, ru, en = parts[:4]
        note = parts[4] if len(parts) == 5 else ""
        out.append((lemma, _UPOS.get(upos, upos), ru, en, note))
    return out
