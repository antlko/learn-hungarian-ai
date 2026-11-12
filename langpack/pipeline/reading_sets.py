"""Ready-made reading sets: a «Рассказ» and a «Диалог» for every course unit.

Both kinds are written, not picked (ADR-074 replaced the Tatoeba «Фразы»):

  * «Рассказ» (`track` core/theme, code = the unit code, `pipeline/stories.py`):
    one short connected text per unit, 8-14 sentences in 2-4 paragraphs.
  * «Диалог» (`track` dialogue, code `<unit>-dialogue`, `pipeline/dialogues.py`):
    one short written dialogue per unit.

Both are stored as `source = 'authored'` sentences, labelled «Написано ИИ,
носителем не проверено» everywhere they are shown (ADR-066, ADR-074) and feed
reading only: no drill, example or n-gram ever reads them.

This module owns the tables, the unit rows (code, band, ord, theme) both writers
share, and `validate`. The learner imports a set as an ordinary text with one
click and reads it in the reader. See docs/LANGPACK.md "Reading sets".

The band of a set is an estimate: core months 1-2 -> A1, 3-4 -> A2, 5-6 -> B1; a
theme is A2 (`lemma.cefr` is itself estimated from frequency rank, R6).
"""

from __future__ import annotations

import sqlite3

from . import course

BANDS = ("A1", "A2", "B1", "B2", "C1")
CONTENT_UPOS = ("NOUN", "VERB", "ADJ", "ADV", "NUM", "INTJ")

# Core months -> band. Themes are decided from their own words.
MONTH_BAND = {1: "A1", 2: "A1", 3: "A2", 4: "A2", 5: "B1", 6: "B1"}
# Theme vocabulary is mostly A2+, and on hu-v1 every theme was A2 under the old
# per-theme rule (>= 20% A1 words and a full A1 sentence pool), so it is fixed.
THEME_BAND = "A2"

SCHEMA = """
CREATE TABLE IF NOT EXISTS reading_set (
    code     TEXT PRIMARY KEY,
    level    TEXT NOT NULL,
    track    TEXT NOT NULL DEFAULT 'core',
    ord      INTEGER NOT NULL,
    title_ru TEXT NOT NULL, title_en TEXT NOT NULL,
    theme_ru TEXT NOT NULL DEFAULT '', theme_en TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS reading_set_sentence (
    set_code    TEXT NOT NULL REFERENCES reading_set(code),
    sentence_id INTEGER NOT NULL REFERENCES sentence(id),
    ord         INTEGER NOT NULL,
    para        INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (set_code, sentence_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rss_sentence ON reading_set_sentence(sentence_id);
"""


def _rank(band: str | None) -> int:
    return BANDS.index(band) if band in BANDS else 99


def ensure_schema(conn: sqlite3.Connection) -> None:
    """Create the tables, and add `para` to a pack built before it existed."""
    conn.executescript(SCHEMA)
    cols = {r[1] for r in conn.execute("PRAGMA table_info(reading_set_sentence)")}
    if "para" not in cols:
        conn.execute("ALTER TABLE reading_set_sentence ADD COLUMN para INTEGER NOT NULL DEFAULT 0")


def unit_band(conn: sqlite3.Connection, unit: dict) -> str:
    return MONTH_BAND.get(unit["month"], "A2") if unit["track"] == "core" else THEME_BAND


def unit_rows(conn: sqlite3.Connection) -> list[dict]:
    """One row per basic-tier unit the pack's course holds (ADR-057: MONTH_BAND
    stops at month 6). `ord` is even, so the unit's dialogue sorts at ord-1,
    right before the story."""
    rows = []
    for u in course.UNITS:
        if u.get("tier", "basic") != "basic":
            continue
        if not conn.execute("SELECT 1 FROM course_unit WHERE code = ?", (u["code"],)).fetchone():
            continue
        rows.append({
            "code": u["code"], "level": unit_band(conn, u), "track": u["track"],
            "ord": 2 * u["ord"],
            "title_ru": f"Рассказ: {u['title_ru']}", "title_en": f"Story: {u['title_en']}",
            "theme_ru": u.get("situation_ru", ""), "theme_en": u.get("situation_en", ""),
        })
    rows.sort(key=lambda r: r["ord"])
    return rows


def build(conn: sqlite3.Connection) -> dict[str, int]:
    """(Re)write the two tables in `conn`: each unit's «Диалог» (dialogues.build)
    and «Рассказ» (stories.build), from the authored text. A unit without a
    story yet is skipped with a warning; `validate(require_all=True)` refuses."""
    from . import dialogues, stories  # both import this module's constants

    ensure_schema(conn)
    rows = unit_rows(conn)
    conn.execute("DELETE FROM reading_set_sentence")
    conn.execute("DELETE FROM reading_set")
    # dialogues.build clears every authored sentence, so it runs before stories.
    d = dialogues.build(conn, rows)
    s = stories.build(conn, rows)
    return {"reading_set": d["sets"] + s["sets"],
            "reading_set_sentence": d["sentences"] + s["sentences"],
            "dialogue_sets": d["sets"], "dialogue_sentences": d["sentences"],
            "story_sets": s["sets"], "story_sentences": s["sentences"]}


def validate(conn: sqlite3.Connection, require_all: bool = False) -> list[str]:
    """Problems in the sets a pack carries; empty is good.

    Every reading set is authored and labelled (a «Рассказ» or a «Диалог»), holds
    8-14 sentences; `dialogues.problems` and `stories.problems` check the text.
    `require_all`: every unit must have a story (release builds set it).
    """
    from . import dialogues, stories

    problems: list[str] = []
    for code, level, track, tru, ten in conn.execute(
            "SELECT code, level, track, title_ru, title_en FROM reading_set"):
        n = conn.execute(
            "SELECT COUNT(*) FROM reading_set_sentence WHERE set_code = ?", (code,)).fetchone()[0]
        if not dialogues.MIN_LINES <= n <= dialogues.MAX_LINES:
            problems.append(
                f"{code}: {n} sentences, want {dialogues.MIN_LINES}-{dialogues.MAX_LINES}")
        if level not in BANDS:
            problems.append(f"{code}: unknown level {level!r}")
        want = (("Диалог:", "Dialogue:") if track == dialogues.TRACK
                else (stories.TITLE_PREFIX_RU + ":", stories.TITLE_PREFIX_EN + ":"))
        if not (tru.startswith(want[0]) and ten.startswith(want[1])):
            problems.append(f"{code}: title must start with {want[0]!r} / {want[1]!r}")
    for (sid,) in conn.execute(
        "SELECT x.sentence_id FROM reading_set_sentence x "
        "WHERE x.sentence_id NOT IN (SELECT id FROM sentence WHERE source = 'authored')"
    ):
        problems.append(f"sentence {sid}: missing or not authored, every reading set is authored and labelled")
    for sid, k in conn.execute(
        "SELECT sentence_id, COUNT(*) FROM reading_set_sentence GROUP BY 1 HAVING COUNT(*) > 1"
    ):
        problems.append(f"sentence {sid}: in {k} sets")
    problems.extend(dialogues.problems(conn))
    problems.extend(stories.problems(conn, require_all))
    return problems
