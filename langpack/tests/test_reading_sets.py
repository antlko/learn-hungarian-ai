"""Reading sets (ADR-074): an authored «Рассказ» and a «Диалог» per unit.

The stories themselves are checked in test_stories.py, the dialogues in
test_dialogues.py. Here: the sets a build writes into a COPY of the starter (the
committed starter may predate the stories, so we rebuild rather than read it),
and `validate`. Skips, rather than passing vacuously, if the starter is absent.
"""

import gzip
import shutil
import sqlite3
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from pipeline import course, dialogues, reading_sets, stories

STARTER_GZ = (Path(__file__).resolve().parents[2]
              / "backend/internal/lang/starter/langpack-hu-starter.sqlite.gz")
BASIC = [u for u in course.UNITS if u.get("tier", "basic") == "basic"]


@pytest.fixture(scope="module")
def conn(tmp_path_factory):
    if not STARTER_GZ.exists():
        pytest.skip("starter pack not present")
    out = tmp_path_factory.mktemp("starter") / "starter.sqlite"
    with gzip.open(STARTER_GZ, "rb") as fin, open(out, "wb") as fout:
        shutil.copyfileobj(fin, fout)
    c = sqlite3.connect(out)
    reading_sets.build(c)
    c.commit()
    yield c
    c.close()


def _stories(conn):
    """The «Рассказ» sets: code -> (level, track). Dialogues are separate."""
    return {code: (level, track) for code, level, track in conn.execute(
        "SELECT code, level, track FROM reading_set WHERE track <> 'dialogue'")}


def test_a_story_exists_for_every_unit_that_has_text_and_a_dialogue_for_all(conn):
    assert set(_stories(conn)) == set(stories.STORIES)
    dlg = {c for (c,) in conn.execute("SELECT code FROM reading_set WHERE track = 'dialogue'")}
    assert dlg == {f"{u['code']}-dialogue" for u in BASIC}
    assert len(BASIC) == 36


def test_the_story_keeps_the_unit_code_and_track(conn):
    by_code = {u["code"]: u for u in BASIC}
    for code, (_level, track) in _stories(conn).items():
        assert track == by_code[code]["track"]


def test_core_levels_follow_the_month_and_themes_are_a2(conn):
    by_code = {u["code"]: u for u in BASIC}
    for code, (level, track) in _stories(conn).items():
        if track == "core":
            assert level == reading_sets.MONTH_BAND[by_code[code]["month"]]
        else:
            assert level == reading_sets.THEME_BAND


def test_titles_name_the_unit_in_both_languages(conn):
    by_code = {u["code"]: u for u in BASIC}
    for code, ru, en in conn.execute(
            "SELECT code, title_ru, title_en FROM reading_set WHERE track <> 'dialogue'"):
        assert ru == f"Рассказ: {by_code[code]['title_ru']}"
        assert en == f"Story: {by_code[code]['title_en']}"


def test_a_dialogue_sorts_right_before_its_story(conn):
    order = [c for (c,) in conn.execute("SELECT code FROM reading_set ORDER BY ord, code")]
    for code in _stories(conn):
        assert order.index(f"{code}-dialogue") + 1 == order.index(code), code


def test_validator_is_clean_and_names_missing_stories_only_when_asked(conn):
    assert reading_sets.validate(conn) == []
    missing = [p for p in reading_sets.validate(conn, require_all=True) if "no story" in p]
    assert len(missing) == len(BASIC) - len(stories.STORIES)


def test_every_reading_sentence_is_authored_and_none_has_tokens(conn):
    bad = conn.execute(
        "SELECT x.sentence_id FROM reading_set_sentence x "
        "LEFT JOIN sentence s ON s.id = x.sentence_id "
        "WHERE s.id IS NULL OR s.source <> 'authored'").fetchall()
    assert bad == []
    assert conn.execute(
        "SELECT COUNT(*) FROM sentence_token WHERE sentence_id IN "
        "(SELECT id FROM sentence WHERE source = 'authored')").fetchone() == (0,)


def test_a_sentence_is_in_at_most_one_set(conn):
    assert conn.execute(
        "SELECT sentence_id FROM reading_set_sentence GROUP BY 1 HAVING COUNT(*) > 1"
    ).fetchall() == []


def test_story_paragraphs_are_stored_in_order(conn):
    for code in _stories(conn):
        paras = [p for (p,) in conn.execute(
            "SELECT para FROM reading_set_sentence WHERE set_code = ? ORDER BY ord", (code,))]
        assert paras == sorted(paras) and paras[0] == 0, code
        assert stories.MIN_PARAS <= len(set(paras)) <= stories.MAX_PARAS, code


def test_no_set_is_outside_the_size_bounds(conn):
    for code, n in conn.execute(
            "SELECT set_code, COUNT(*) FROM reading_set_sentence GROUP BY 1"):
        assert dialogues.MIN_LINES <= n <= dialogues.MAX_LINES, code


def test_a_tatoeba_sentence_in_a_set_fails_validation(conn, tmp_path):
    copy = tmp_path / "p.sqlite"
    conn.commit()
    shutil.copy(conn.execute("PRAGMA database_list").fetchone()[2], copy)
    c = sqlite3.connect(copy)
    tato = c.execute("SELECT MIN(id) FROM sentence WHERE source = 'tatoeba'").fetchone()[0]
    sid = c.execute("SELECT MIN(sentence_id) FROM reading_set_sentence "
                    "WHERE set_code = 'm1-u1-koszones'").fetchone()[0]
    c.execute("UPDATE reading_set_sentence SET sentence_id = ? WHERE sentence_id = ?", (tato, sid))
    assert any("not authored" in p for p in reading_sets.validate(c))
    c.close()


def test_an_unlabelled_title_fails_validation(conn, tmp_path):
    copy = tmp_path / "p.sqlite"
    shutil.copy(conn.execute("PRAGMA database_list").fetchone()[2], copy)
    c = sqlite3.connect(copy)
    c.execute("UPDATE reading_set SET title_ru = 'Фразы: x' WHERE code = 'm1-u1-koszones'")
    assert any("title must start" in p for p in reading_sets.validate(c))
    c.close()


def test_build_adds_para_to_a_pack_that_lacks_it(tmp_path):
    c = sqlite3.connect(tmp_path / "old.sqlite")
    c.executescript("""
        CREATE TABLE reading_set_sentence (set_code TEXT NOT NULL, sentence_id INTEGER NOT NULL,
            ord INTEGER NOT NULL, PRIMARY KEY (set_code, sentence_id));
        INSERT INTO reading_set_sentence VALUES ('x', 1, 0);""")
    reading_sets.ensure_schema(c)
    assert c.execute("SELECT para FROM reading_set_sentence").fetchall() == [(0,)]
    reading_sets.ensure_schema(c)  # idempotent
    c.close()


def test_selection_is_deterministic(conn, tmp_path):
    conn.commit()
    q = "SELECT set_code, sentence_id, ord, para FROM reading_set_sentence ORDER BY 1,3"
    before = conn.execute(q).fetchall()
    copy = tmp_path / "p.sqlite"
    shutil.copy(conn.execute("PRAGMA database_list").fetchone()[2], copy)
    c = sqlite3.connect(copy)
    reading_sets.build(c)
    assert c.execute(q).fetchall() == before
    c.close()
