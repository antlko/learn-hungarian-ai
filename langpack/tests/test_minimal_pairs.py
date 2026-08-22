"""The HVPT inventory's structural contract.

A minimal pair that is not minimal — or whose members do not differ in the
contrast it claims — trains the wrong thing. These checks run at build time and
here, so a hand-edited pair cannot slip through.
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from pipeline.minimal_pairs import PAIRS, RATES, VOICES, MinimalPair


def test_pairs_are_distinct_words():
    for p in PAIRS:
        assert p.a != p.b, f"{p.a}: a pair of one word is not a pair"


def test_priorities_are_sane():
    assert all(1 <= p.priority <= 5 for p in PAIRS)
    # The highest-yield sets (vowel length) must actually come first.
    top = [p.contrast for p in PAIRS if p.priority == 1]
    assert any(c in ("a/á", "e/é", "o/ó", "ö/ő") for c in top)


def test_glosses_exist_in_both_locales():
    # Both locales, always (ADR-004). "(non-word)" is a legitimate gloss:
    # 2AFC identification may use a nonword member, but it must SAY so.
    for p in PAIRS:
        for g in (p.gloss_a_ru, p.gloss_b_ru, p.gloss_a_en, p.gloss_b_en):
            assert g.strip(), f"{p.a}/{p.b}: empty gloss"


def test_render_grid_is_declared():
    # The render stage's shape is part of the pack contract: 3 talkers, and
    # rate variants are rates, never extra talkers.
    assert len(VOICES) == 3
    assert 1.0 in RATES


def test_members_differ_in_the_claimed_contrast():
    """Weak but honest check: for X/Y contrasts over single letters, replacing
    X with Y in member a should relate it to member b (or the pair is length /
    gemination, where lengths differ)."""
    for p in PAIRS:
        if "/" not in p.contrast:
            continue
        x, y = p.contrast.split("/", 1)
        related = (
            p.a.replace(x, y) == p.b
            or p.b.replace(y, x) == p.a
            or len(p.a) != len(p.b)  # gemination and digraph contrasts
        )
        assert related, f"{p.a}/{p.b} does not embody {p.contrast}"
