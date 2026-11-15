"""Unit metadata the reading-text tools share (ADR-075, ADR-076).

The reading texts no longer live in the language pack: langpack/texts/*.json is
validated by `pipeline/texts.py` and exported to backend/internal/readsets/catalog.json,
which the Go binary embeds. This module keeps what both the validator and the legacy
helper modules (stories.py, dialogues.py) need: the CEFR band of a unit, its rank,
and `unit_rows`. The `reading_set*` tables stay in schema.sql only so older binaries
and old packs keep opening; nothing writes them any more.

The band of a unit is an estimate: core months 1-2 -> A1, 3-4 -> A2, 5-6 -> B1; a
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

def _rank(band: str | None) -> int:
    return BANDS.index(band) if band in BANDS else 99


def has_text_columns(conn: sqlite3.Connection) -> bool:
    """True when `reading_set` carries `genre` (a pack from ADR-075 on)."""
    return "genre" in {r[1] for r in conn.execute("PRAGMA table_info(reading_set)")}


def legacy_filter(conn: sqlite3.Connection) -> str:
    """SQL `AND ...` keeping only legacy (pre-ADR-075) sets, '' on an old pack."""
    return "AND genre = ''" if has_text_columns(conn) else ""


def unit_band(conn: sqlite3.Connection, unit: dict) -> str:
    return MONTH_BAND.get(unit["month"], "A2") if unit["track"] == "core" else THEME_BAND


def unit_rows(conn: sqlite3.Connection) -> list[dict]:
    """One row per basic-tier unit the pack's course holds (ADR-057: MONTH_BAND
    stops at month 6). `ord` is unit ord * 10: a unit's sets sort at +1 (short),
    +2 (medium), +3 (long)."""
    rows = []
    for u in course.UNITS:
        if u.get("tier", "basic") != "basic":
            continue
        if not conn.execute("SELECT 1 FROM course_unit WHERE code = ?", (u["code"],)).fetchone():
            continue
        rows.append({
            "code": u["code"], "level": unit_band(conn, u), "track": u["track"],
            "ord": 10 * u["ord"],
            "title_ru": f"Рассказ: {u['title_ru']}", "title_en": f"Story: {u['title_en']}",
            "theme_ru": u.get("situation_ru", ""), "theme_en": u.get("situation_en", ""),
        })
    rows.sort(key=lambda r: r["ord"])
    return rows
