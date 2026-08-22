"""Wiktionary import guards.

These are regression tests for defects that reached the shipped hu-v1 pack, so
they are written against the exact strings that caused them.
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import pytest

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
