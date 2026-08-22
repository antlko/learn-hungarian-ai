"""The Russian gloss import, and the precedence rule it exists to enforce."""

from __future__ import annotations

import json
import sqlite3
from pathlib import Path

import pytest

from pipeline import wiktionary_ru


def _pack(tmp_path: Path) -> sqlite3.Connection:
    conn = sqlite3.connect(":memory:")
    conn.executescript(
        """
        CREATE TABLE pack_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
        CREATE TABLE lemma (id INTEGER PRIMARY KEY, lemma TEXT, upos TEXT);
        CREATE TABLE lemma_gloss (
            lemma_id INTEGER, lang TEXT, sense_idx INTEGER, gloss TEXT,
            PRIMARY KEY (lemma_id, lang, sense_idx)
        );
        CREATE TABLE course_word (
            unit_code TEXT, ord INTEGER, lemma TEXT, upos TEXT, gloss_ru TEXT
        );
        """
    )
    return conn


def _jsonl(tmp_path: Path, entries: list[dict]) -> Path:
    p = tmp_path / "ru.jsonl"
    p.write_text("\n".join(json.dumps(e, ensure_ascii=False) for e in entries), encoding="utf-8")
    return p


def test_parse_keeps_meaning_and_drops_placeholders(tmp_path: Path):
    path = _jsonl(
        tmp_path,
        [
            {"word": "ház", "pos": "noun", "senses": [{"glosses": ["дом"]}]},
            # A placeholder somebody left behind. Importing it would REPLACE a
            # usable English gloss with a dash.
            {"word": "üres", "pos": "adj", "senses": [{"glosses": ["—"]}]},
            # Wiki leftovers are stripped; the meaning survives.
            {"word": "kert", "pos": "noun", "senses": [{"glosses": ["[[сад]]"]}]},
            # An encyclopaedia paragraph is not a flashcard.
            {"word": "izé", "pos": "noun", "senses": [{"glosses": ["к" * 200]}]},
            # An unmapped part of speech is skipped rather than guessed at.
            {"word": "xyz", "pos": "romanization", "senses": [{"glosses": ["нечто"]}]},
        ],
    )
    out = wiktionary_ru.parse(path)
    assert out[("ház", "NOUN")] == ["дом"]
    assert out[("kert", "NOUN")] == ["сад"]
    assert ("üres", "ADJ") not in out
    assert ("izé", "NOUN") not in out
    assert ("xyz", "X") not in out


def test_parse_caps_senses_and_dedups(tmp_path: Path):
    path = _jsonl(
        tmp_path,
        [
            {
                "word": "fej",
                "pos": "noun",
                "senses": [
                    {"glosses": ["голова"]},
                    {"glosses": ["голова"]},  # duplicate across senses
                    {"glosses": ["глава"]},
                    {"glosses": ["кочан"]},
                    {"glosses": ["начальник"]},
                ],
            }
        ],
    )
    out = wiktionary_ru.parse(path)
    assert out[("fej", "NOUN")] == ["голова", "глава", "кочан"]


def test_apply_matches_on_pos_then_relaxes(tmp_path: Path):
    conn = _pack(tmp_path)
    conn.executemany(
        "INSERT INTO lemma VALUES (?,?,?)",
        [(1, "ház", "NOUN"), (2, "piros", "NOUN")],  # the pack tagged piros a noun
    )
    stats = wiktionary_ru.apply(
        conn,
        {
            ("ház", "NOUN"): ["дом"],
            ("piros", "ADJ"): ["красный"],  # the dictionary tagged it an adjective
            ("nincs_ilyen", "NOUN"): ["ничего"],
        },
    )
    assert stats["exact_pos"] == 1
    assert stats["relaxed_pos"] == 1
    got = dict(conn.execute("SELECT lemma_id, gloss FROM lemma_gloss WHERE lang='ru'"))
    assert got == {1: "дом", 2: "красный"}


def test_apply_is_idempotent(tmp_path: Path):
    """Re-running against a pack must not stack duplicate senses."""
    conn = _pack(tmp_path)
    conn.execute("INSERT INTO lemma VALUES (1,'ház','NOUN')")
    for _ in range(3):
        wiktionary_ru.apply(conn, {("ház", "NOUN"): ["дом", "здание"]})
    rows = conn.execute(
        "SELECT sense_idx, gloss FROM lemma_gloss WHERE lang='ru' ORDER BY sense_idx"
    ).fetchall()
    assert rows == [(0, "дом"), (1, "здание")]


def test_english_glosses_are_never_touched(tmp_path: Path):
    conn = _pack(tmp_path)
    conn.execute("INSERT INTO lemma VALUES (1,'ház','NOUN')")
    conn.execute("INSERT INTO lemma_gloss VALUES (1,'en',0,'house')")
    wiktionary_ru.apply(conn, {("ház", "NOUN"): ["дом"]})
    assert conn.execute("SELECT gloss FROM lemma_gloss WHERE lang='en'").fetchone() == ("house",)


def test_course_gloss_wins_and_dictionary_senses_survive_behind_it(tmp_path: Path):
    """The rule this module exists for.

    `kér` in the dictionary is "просить, требовать, заказывать, ходатайствовать".
    The unit that teaches it means "просить". A learner in month two must see the
    sense the unit taught — but the others must still be reachable, because the
    word really does mean them.
    """
    conn = _pack(tmp_path)
    conn.execute("INSERT INTO lemma VALUES (1,'kér','VERB')")
    conn.execute("INSERT INTO course_word VALUES ('m1u2', 3, 'kér', 'VERB', 'просить')")
    wiktionary_ru.apply(conn, {("kér", "VERB"): ["требовать", "заказывать"]})

    stats = wiktionary_ru.apply_course_glosses(conn)
    assert stats["promoted"] == 1

    rows = conn.execute(
        "SELECT sense_idx, gloss FROM lemma_gloss WHERE lang='ru' ORDER BY sense_idx"
    ).fetchall()
    assert rows == [(0, "просить"), (1, "требовать"), (2, "заказывать")]


def test_a_course_gloss_that_repeats_a_dictionary_sense_is_not_duplicated(tmp_path: Path):
    conn = _pack(tmp_path)
    conn.execute("INSERT INTO lemma VALUES (1,'ház','NOUN')")
    conn.execute("INSERT INTO course_word VALUES ('m1u1', 1, 'ház', 'NOUN', 'дом')")
    wiktionary_ru.apply(conn, {("ház", "NOUN"): ["дом", "здание"]})
    wiktionary_ru.apply_course_glosses(conn)
    rows = conn.execute(
        "SELECT sense_idx, gloss FROM lemma_gloss WHERE lang='ru' ORDER BY sense_idx"
    ).fetchall()
    assert rows == [(0, "дом"), (1, "здание")]


def test_the_earlier_unit_wins_when_two_units_teach_one_lemma(tmp_path: Path):
    conn = _pack(tmp_path)
    conn.execute("INSERT INTO lemma VALUES (1,'ad','VERB')")
    conn.executemany(
        "INSERT INTO course_word VALUES (?,?,?,?,?)",
        [
            ("m1u1", 5, "ad", "VERB", "давать"),
            ("m4u2", 2, "ad", "VERB", "подавать (заявление)"),
        ],
    )
    wiktionary_ru.apply_course_glosses(conn)
    rows = conn.execute("SELECT gloss FROM lemma_gloss WHERE lang='ru'").fetchall()
    assert rows == [("давать",)]


def test_gloss_langs_reflects_what_is_actually_there(tmp_path: Path):
    conn = _pack(tmp_path)
    conn.execute("INSERT INTO lemma VALUES (1,'ház','NOUN')")
    conn.execute("INSERT INTO lemma_gloss VALUES (1,'en',0,'house')")
    assert wiktionary_ru.record_gloss_lang(conn) == ["en"]

    wiktionary_ru.apply(conn, {("ház", "NOUN"): ["дом"]})
    assert wiktionary_ru.record_gloss_lang(conn) == ["en", "ru"]
    stored = conn.execute("SELECT value FROM pack_meta WHERE key='gloss_langs'").fetchone()[0]
    assert json.loads(stored) == ["en", "ru"]


@pytest.mark.parametrize("bad", ["", "  ", "—", "-", "?", "..."])
def test_empty_placeholders_are_refused(bad: str):
    assert wiktionary_ru._clean(bad) is None
