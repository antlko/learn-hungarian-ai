"""Vowel harmony tests.

Cases are chosen to be the ones a learner actually gets wrong, not just the ones
that exercise the branches: neutral vowels next to back ones, antiharmonic
stems, and last-vowel rounding.
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import pytest

from pipeline.harmony import (
    Harmony,
    classify,
    ends_in_vowel,
    lengthen_final_vowel,
    pick,
    vowels_of,
)


@pytest.mark.parametrize(
    "word,expected",
    [
        # Plain back
        ("ház", Harmony.BACK),
        ("asztal", Harmony.BACK),
        ("magyar", Harmony.BACK),
        # Plain front unrounded
        ("kert", Harmony.FRONT_UNROUNDED),
        ("kenyér", Harmony.FRONT_UNROUNDED),
        ("_", Harmony.FRONT_UNROUNDED),  # no vowels at all
        # Front rounded, decided by the LAST front vowel
        ("köd", Harmony.FRONT_ROUNDED),
        ("gyümölcs", Harmony.FRONT_ROUNDED),
        ("tükör", Harmony.FRONT_ROUNDED),
        # Rounded then unrounded: the last vowel wins, so this is unrounded
        ("könyvek", Harmony.FRONT_UNROUNDED),
        # Neutral vowels must NOT override a back vowel -- the classic error
        ("papír", Harmony.BACK),
        ("kocsi", Harmony.BACK),
        ("Balaton", Harmony.BACK),
        ("kávé", Harmony.BACK),
        # Antiharmonic: front-looking, back-behaving
        ("híd", Harmony.BACK),
        ("cél", Harmony.BACK),
        ("víz", Harmony.BACK),
        ("férj", Harmony.BACK),
    ],
)
def test_classify(word, expected):
    assert classify(word) is expected


def test_classify_is_case_insensitive():
    assert classify("BALATON") is classify("balaton")


def test_pick_two_form_suffix():
    # -ban/-ben has no separate rounded form: front rounded shares the front one.
    assert pick(Harmony.BACK, "ban", "ben") == "ban"
    assert pick(Harmony.FRONT_UNROUNDED, "ban", "ben") == "ben"
    assert pick(Harmony.FRONT_ROUNDED, "ban", "ben") == "ben"


def test_pick_three_form_suffix():
    assert pick(Harmony.BACK, "hoz", "hez", "höz") == "hoz"
    assert pick(Harmony.FRONT_UNROUNDED, "hoz", "hez", "höz") == "hez"
    assert pick(Harmony.FRONT_ROUNDED, "hoz", "hez", "höz") == "höz"


def test_real_suffix_selection():
    """The examples from the source brief §2.2, end to end."""
    cases = [
        ("ház", "ban", "hoz", "tól"),      # back
        ("kert", "ben", "hez", "től"),     # front unrounded
        ("köd", "ben", "höz", "től"),      # front rounded
    ]
    for stem, ine, all_, abl in cases:
        h = classify(stem)
        assert pick(h, "ban", "ben") == ine
        assert pick(h, "hoz", "hez", "höz") == all_
        assert pick(h, "tól", "től") == abl


@pytest.mark.parametrize("word,expected", [("alma", True), ("ház", False), ("", False)])
def test_ends_in_vowel(word, expected):
    assert ends_in_vowel(word) is expected


@pytest.mark.parametrize(
    "word,expected",
    [
        ("alma", "almá"),
        ("epe", "epé"),
        ("ház", "ház"),   # unaffected
        ("kocsi", "kocsi"),
        ("", ""),
    ],
)
def test_lengthen_final_vowel(word, expected):
    assert lengthen_final_vowel(word) == expected


def test_vowels_of_preserves_order():
    assert vowels_of("gyümölcs") == ["ü", "ö"]
    assert vowels_of("Balaton") == ["a", "a", "o"]


@pytest.mark.parametrize(
    "word,expected",
    [
        # The last non-transparent front vowel decides rounding. Adding the
        # plural changes which vowel is adjacent to the suffix, and therefore
        # changes the suffix -- a real alternation, not an edge case.
        ("könyv", Harmony.FRONT_ROUNDED),      # könyvhöz
        ("könyvek", Harmony.FRONT_UNROUNDED),  # könyvekhez
        ("tükör", Harmony.FRONT_ROUNDED),      # tükörhöz
        ("tükrei", Harmony.FRONT_UNROUNDED),   # tükreihez -- `i` is transparent, `e` decides
        ("ötven", Harmony.FRONT_UNROUNDED),    # ötvenhez
        ("öt", Harmony.FRONT_ROUNDED),         # öthöz
        ("ing", Harmony.FRONT_UNROUNDED),      # only transparent vowels
    ],
)
def test_rounding_is_decided_by_the_nearest_vowel(word, expected):
    assert classify(word) is expected


def test_plural_can_flip_the_suffix():
    """Regression: scanning backwards for *any* rounded vowel got this wrong."""
    assert pick(classify("könyv"), "hoz", "hez", "höz") == "höz"
    assert pick(classify("könyvek"), "hoz", "hez", "höz") == "hez"
