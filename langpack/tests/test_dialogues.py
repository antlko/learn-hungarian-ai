"""Authored dialogues (pipeline/dialogues.py): written text, attested forms,
in-band words, kept out of everything that is not reading.

The data checks need no pack. The pack checks read the STARTER (the artefact a
fresh install gets) and skip, rather than pass vacuously, if it is absent.
"""

import gzip
import shutil
import sqlite3
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from pipeline import course, dialogues, reading_sets

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
    c = sqlite3.connect(f"file:{out}?mode=ro", uri=True)
    yield c
    c.close()


def test_every_basic_unit_has_exactly_one_dialogue():
    assert {u["code"] for u in BASIC} == set(dialogues.DIALOGUES)
    assert len(BASIC) == 36


def test_the_written_text_is_well_formed():
    assert dialogues.data_problems() == []


def test_lines_are_short_with_both_translations_and_no_speaker_labels():
    for code, lines in dialogues.DIALOGUES.items():
        assert dialogues.MIN_LINES <= len(lines) <= dialogues.MAX_LINES, code
        for hu, ru, en in lines:
            assert 1 <= len(dialogues._WORD.findall(hu)) <= dialogues.MAX_TOKENS, (code, hu)
            assert ru.strip() and en.strip(), (code, hu)
            # The reader would treat «A:» or a leading dash as part of the
            # sentence: the import adds the turn marker, the text has none.
            assert not hu.lstrip().startswith(("-", "–", "—")), (code, hu)
            assert ":" not in hu.split(" ")[0], (code, hu)
            assert not any(ch.isdigit() for ch in hu), (code, hu)


def test_a_dialogue_has_two_speakers_turn_taking_not_a_monologue():
    # Turns alternate by construction; a dialogue with no question or no
    # answer would not read as one.
    for code, lines in dialogues.DIALOGUES.items():
        assert any(hu.rstrip().endswith("?") for hu, _, _ in lines), code


def test_the_starter_dialogues_are_all_attested_and_in_band(conn):
    assert dialogues.problems(conn) == []


def test_dialogue_sets_are_authored_only_with_no_tokens(conn):
    rows = conn.execute(
        "SELECT s.source, COUNT(*) FROM reading_set_sentence x "
        "JOIN reading_set r ON r.code = x.set_code JOIN sentence s ON s.id = x.sentence_id "
        "WHERE r.track = 'dialogue' GROUP BY 1").fetchall()
    assert [r[0] for r in rows] == ["authored"]
    assert conn.execute(
        "SELECT COUNT(*) FROM sentence_token WHERE sentence_id IN "
        "(SELECT id FROM sentence WHERE source = 'authored')").fetchone() == (0,)
    # Authored sentences exist ONLY as dialogue lines.
    assert conn.execute(
        "SELECT COUNT(*) FROM sentence WHERE source = 'authored' AND id NOT IN "
        "(SELECT sentence_id FROM reading_set_sentence)").fetchone() == (0,)


def test_a_dialogue_shares_the_level_of_its_unit(conn):
    for code, level in conn.execute("SELECT code, level FROM reading_set WHERE track <> 'dialogue'"):
        assert conn.execute(
            "SELECT level FROM reading_set WHERE code = ?", (f"{code}-dialogue",)
        ).fetchone() == (level,), code


def test_dialogue_ids_stay_clear_of_corpus_ids(conn):
    top = conn.execute(
        "SELECT MAX(id) FROM sentence WHERE source <> 'authored'").fetchone()[0]
    low = conn.execute(
        "SELECT MIN(id) FROM sentence WHERE source = 'authored'").fetchone()[0]
    assert top < dialogues.ID_BASE <= low


# ── the checker itself ─────────────────────────────────────────────────────

def test_an_unattested_form_is_reported(conn):
    lex = dialogues.Lexicon(conn)
    bad = dialogues.check_line(lex, "Xyzzyzett a ház.", "A1", set())
    assert any("not an attested wordform" in b for b in bad)


def test_a_word_above_the_band_is_reported_unless_the_unit_teaches_it(conn):
    lex = dialogues.Lexicon(conn)
    # `bizottság` (committee) is B-band; month-1 dialogues are A1.
    assert any("above A1" in b for b in dialogues.check_line(lex, "Ez a bizottság.", "A1", set()))
    assert dialogues.check_line(lex, "Ez a bizottság.", "A1", {"bizottság"}) == []


def test_a_name_does_not_rescue_a_common_word(conn):
    lex = dialogues.Lexicon(conn)
    # `barna` is the colour (B1 in the pack) and also a first name (`Barna`).
    assert any("above A2" in b for b in dialogues.check_line(lex, "Ez barna.", "A2", set()))


def test_a_label_a_digit_or_too_many_words_is_reported(conn):
    lex = dialogues.Lexicon(conn)
    assert dialogues.check_line(lex, "Anna: Szia!", "A1", set())
    assert dialogues.check_line(lex, "Szia 5!", "A1", set())
    long = " ".join(["Szia"] * (dialogues.MAX_TOKENS + 1)) + "!"
    assert any("words, at most" in b for b in dialogues.check_line(lex, long, "A1", set()))


def test_a_pack_with_a_bad_dialogue_fails_validation(conn, tmp_path):
    copy = tmp_path / "p.sqlite"
    shutil.copy(conn.execute("PRAGMA database_list").fetchone()[2], copy)
    c = sqlite3.connect(copy)
    sid = c.execute("SELECT MIN(id) FROM sentence WHERE source = 'authored'").fetchone()[0]
    c.execute("UPDATE sentence SET hu = 'Xyzzyzett vagyok.' WHERE id = ?", (sid,))
    problems = reading_sets.validate(c)
    c.close()
    assert any("not an attested wordform" in p for p in problems)


def test_build_is_idempotent(conn, tmp_path):
    copy = tmp_path / "p.sqlite"
    shutil.copy(conn.execute("PRAGMA database_list").fetchone()[2], copy)
    c = sqlite3.connect(copy)
    reading_sets.build(c)  # the committed starter may predate the stories
    q = ("SELECT set_code, sentence_id, ord FROM reading_set_sentence ORDER BY 1,3",
         "SELECT id, hu, source FROM sentence WHERE source = 'authored' ORDER BY id",
         "SELECT sentence_id, lang, text FROM sentence_trans WHERE sentence_id >= ? ORDER BY 1,2")
    before = [c.execute(q[0]).fetchall(), c.execute(q[1]).fetchall(),
              c.execute(q[2], (dialogues.ID_BASE,)).fetchall()]
    reading_sets.build(c)
    after = [c.execute(q[0]).fetchall(), c.execute(q[1]).fetchall(),
             c.execute(q[2], (dialogues.ID_BASE,)).fetchall()]
    c.close()
    assert before == after
