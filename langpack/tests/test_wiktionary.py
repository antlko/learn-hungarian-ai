"""Wiktionary import guards.

These are regression tests for defects that reached the shipped hu-v1 pack, so
they are written against the exact strings that caused them.
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import pytest

from pipeline import wiktionary
from pipeline.wiktionary import _is_plausible_form


@pytest.mark.parametrize(
    "form,expected",
    [
        # A real form, including one with a word-internal hyphen.
        ("házon", True),
        ("van-e", True),
        ("kávé-automata", True),
        # An EMPTY declension cell. Wiktionary writes it "-", the hyphen is a
        # legal form character, and the importer took it literally: hu-v1 has
        # "-" stored as the essive of several thousand nouns, where it wins any
        # shortest-form tie-break outright.
        ("-", False),
        ("--", False),
        ("—", False),
        ("", False),
        # Layout artefacts and unsplit cells.
        ("hu-infl-nom", False),
        ("no-table-tags", False),
        ("menve (menvén)", False),
        ("mehetve / mehetvén", False),
    ],
)
def test_is_plausible_form(form, expected):
    assert _is_plausible_form(form) is expected


def _page(word, *senses):
    return {"word": word, "lang_code": "hu", "pos": "verb", "senses": list(senses)}


def _sense(lemma, *tags):
    return {"tags": ["form-of", *tags], "glosses": [f"inflection of {lemma}"],
            "form_of": [{"word": lemma}]}


def test_subjunctive_tag_maps_to_imperative():
    fo = wiktionary._as_form_of(
        _page("menjek", _sense("megy", "first-person", "singular", "indefinite",
                               "present", "subjunctive")), "VERB")
    assert fo is not None
    assert fo.feats["Mood"] == "Imp"
    assert fo.feats["Person"] == "1" and fo.feats["Definite"] == "Ind"


def test_multi_sense_page_keeps_every_reading():
    # olvassuk is the 1pl definite indicative AND the 1pl definite imperative.
    fo = wiktionary._as_form_of(
        _page("olvassuk",
              _sense("olvas", "first-person", "plural", "definite", "present", "indicative"),
              _sense("olvas", "first-person", "plural", "definite", "present", "subjunctive")),
        "VERB")
    assert fo is not None
    moods = sorted(r["Mood"] for r in fo.readings)
    assert moods == ["Imp", "Ind"]
    assert all(r["Person"] == "1" and r["Number"] == "Plur" for r in fo.readings)


def test_single_sense_page_has_one_reading():
    fo = wiktionary._as_form_of(
        _page("házat", _sense("ház", "accusative", "singular")), "NOUN")
    assert fo is not None
    assert fo.readings == [{"Case": "Acc", "Number": "Sing"}]
    assert fo.feats == fo.readings[0]


def test_identical_senses_collapse():
    s = _sense("ház", "accusative", "singular")
    fo = wiktionary._as_form_of(_page("házat", s, dict(s)), "NOUN")
    assert len(fo.readings) == 1
