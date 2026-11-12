"""Authored stories (pipeline/stories.py): format, ids, frequency gate.

The shape checks need no pack; the gate runs on a tiny fixture pack built here.
"""

import sqlite3
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from pipeline import course, dialogues, stories

BASIC = [u for u in course.UNITS if u.get("tier", "basic") == "basic"]


def test_the_written_stories_are_well_formed():
    assert stories.data_problems() == []


def test_only_basic_units_have_stories():
    assert set(stories.STORIES) <= {u["code"] for u in BASIC}


def test_the_sample_story_obeys_the_limits():
    paras = stories.STORIES["m1-u1-koszones"]
    flat = stories.flatten(paras)
    assert stories.MIN_PARAS <= len(paras) <= stories.MAX_PARAS
    assert stories.MIN_SENTENCES <= len(flat) <= stories.MAX_SENTENCES
    for _p, hu, ru, en in flat:
        assert len(dialogues._WORD.findall(hu)) <= stories.MAX_TOKENS
        assert ru.strip() and en.strip()


def test_ids_stay_clear_of_the_dialogue_ids_of_every_unit():
    for u in BASIC:
        d = {dialogues.line_id(u["ord"], i) for i in range(dialogues.MAX_LINES)}
        s = {stories.sentence_id(u["ord"], i) for i in range(stories.MAX_SENTENCES)}
        assert not d & s
        assert min(s) > max(d)
    allids = {stories.sentence_id(u["ord"], i) for u in BASIC for i in range(stories.ID_OFFSET)}
    assert len(allids) == len(BASIC) * stories.ID_OFFSET
    allids |= {dialogues.line_id(u["ord"], i) for u in BASIC for i in range(stories.ID_OFFSET)}
    assert len(allids) == 2 * len(BASIC) * stories.ID_OFFSET


def test_a_story_with_bad_shape_is_reported(monkeypatch):
    unit = BASIC[0]["code"]
    one = [("Reggel van.", "Утро.", "Morning.")]
    monkeypatch.setitem(stories.STORIES, unit, [one * 9])  # 1 paragraph, repeats
    probs = stories.data_problems()
    assert any("paragraphs" in p for p in probs)
    assert any("repeated" in p for p in probs)
    long = " ".join(["Reggel"] * 11) + "."
    monkeypatch.setitem(stories.STORIES, unit, [[(long, "x", "x")] * 4, [(long, "", "x")] * 5])
    assert any("words, at most" in p for p in stories.data_problems())
    assert any("translation is missing" in p for p in stories.data_problems())


def test_thresholds_follow_the_month():
    by = {u["code"]: u for u in BASIC}
    assert stories.top_n(by["m1-u1-koszones"]) == 500
    assert stories.top_n(by["m3-u1-ugyintezes"]) == 1000
    assert stories.top_n(by["m5-u1-elromlott"]) == 2000
    assert stories.top_n(by["t-szamok"]) == 2000


# ── the frequency gate on a tiny pack ──────────────────────────────────────

@pytest.fixture
def lex():
    c = sqlite3.connect(":memory:")
    c.executescript("""
        CREATE TABLE lemma (id INTEGER PRIMARY KEY, lemma TEXT, upos TEXT, freq_rank INTEGER, cefr TEXT);
        CREATE TABLE wordform (form TEXT, lemma_id INTEGER, source TEXT);
    """)
    rows = [  # id, lemma, upos, rank, cefr
        (1, "ház", "NOUN", 100, "A1"), (2, "nagy", "ADJ", 50, "A1"),
        (3, "van", "VERB", 2, "A1"), (4, "kenyér", "NOUN", 4000, "A1"),
        (5, "Anna", "PROPN", None, None), (6, "zsilip", "NOUN", 9000, "A1"),
        (7, "ritka", "ADJ", None, "A1"),
    ]
    c.executemany("INSERT INTO lemma VALUES (?,?,?,?,?)", rows)
    c.executemany("INSERT INTO wordform VALUES (?,?,'wiktionary')",
                  [(lm, i) for i, lm, *_ in rows])
    return stories.FreqLexicon(c)


UNIT = {"code": "x", "ord": 0, "track": "core", "month": 1, "words": [("kenyér", "NOUN", "", "", "")]}


def test_the_share_counts_running_tokens_not_lemmas(lex):
    rep = stories.frequency_report(lex, ["Ház van. Ház nagy. Ház zsilip."], UNIT)
    assert (rep["top"], rep["counted"]) == (5, 6)
    assert rep["rarest"] == "zsilip" and rep["rarest_rank"] == 9000


def test_own_words_and_names_are_exempt(lex):
    rep = stories.frequency_report(lex, ["Anna kenyér ház van."], UNIT)
    assert (rep["top"], rep["counted"], rep["exempt"]) == (2, 2, 2)
    assert rep["share"] == 1.0


def test_an_unranked_lemma_misses_and_is_the_rarest(lex):
    rep = stories.frequency_report(lex, ["Ház ritka."], UNIT)
    assert rep["share"] == 0.5 and rep["rarest"] == "ritka"
    assert rep["rarest_rank"] >= stories.NO_RANK


def test_below_the_gate_is_a_problem_and_at_it_is_not(lex):
    rep, probs = stories.check_story(lex, UNIT, "A1", ["Ház van. Nagy ház van."])
    assert rep["share"] == 1.0 and probs == []
    rep, probs = stories.check_story(lex, UNIT, "A1", ["Zsilip van.", "Zsilip ház."])
    assert rep["share"] == 0.5
    assert any("frequency gate" in p for p in probs)
    # 17 of 20 = 0.85: exactly the minimum passes.
    lines = ["Ház ház ház ház ház ház ház ház ház ház ház ház ház ház ház ház ház zsilip zsilip zsilip."]
    rep, probs = stories.check_story(lex, UNIT, "A1", lines)
    assert rep["share"] == pytest.approx(0.85)
    assert not any("frequency gate" in p for p in probs)


def test_an_unattested_word_is_reported_by_the_shared_checker(lex):
    _rep, probs = stories.check_story(lex, UNIT, "A1", ["Ház xyzzy."])
    assert any("not an attested wordform" in p for p in probs)
