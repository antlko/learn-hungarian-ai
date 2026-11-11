"""Authored dialogues: one short, labelled conversation per course unit.

The «Фразы» sets (reading_sets.py) are real Tatoeba sentences, which read as a
bag: a person wrote each line alone. A dialogue is the other thing a learner
wants to read first -- two people, one everyday situation, the phrases a child
hears all day -- and no corpus offers that at A1, so we write them.

PROVENANCE, stated everywhere the text is shown: «Написано ИИ, носителем не
проверено» (ADR-066, amended 2026-11-11). These lines are not corpus sentences:

  * stored in `sentence` with `source = 'authored'`, licence CC BY-SA 4.0 like the rest of
    the pack data, translations in
    `sentence_trans` (ru, en), and NO `sentence_token` rows, so no query that
    reads tokens can reach them;
  * every reader of `sentence` for drills, examples or practice filters
    `source <> 'authored'` (backend tests prove it), and the n-gram attestation
    index never counts them -- the model must not vouch for its own text;
  * they live only in `reading_set` rows with `track = 'dialogue'`, code
    `<unit>-dialogue`, ordered right before the unit's «Фразы» set.

What the build checks (a failure fails the pack build; `validate` -> `problems`):

  1. 8-14 lines per dialogue, at most MAX_TOKENS words per line, a Russian AND
     an English translation per line;
  2. every word of every line is an ATTESTED wordform of the pack -- a row of
     `wordform` whose source is not 'generated' (the table and the test the
     Go side's exact lookup / `Analyzer.IsAttested` use, not `paradigm`);
  3. every content lemma of a word is at or below the unit's band (same rule as
     reading_sets.py), unless the learner has been taught it: a word of this
     unit or of an earlier core unit (themes: their own words and the core
     units of months 1-4) -- the dialogue is about those, whatever frequency
     says. A form passes when SOME attested reading of it passes: a homograph
     is not held against the line.

An attested form proves the form EXISTS, not that the line is idiomatic: that
is exactly what the label is for. Lines are written with the unit's grammar and
no more (month 1: present tense; month 2: accusative and the plain cases...).
"""

from __future__ import annotations

import re
import sqlite3
import sys
from pathlib import Path

from . import course
from . import reading_sets as rs

TRACK = "dialogue"
SOURCE = "authored"
LICENSE = "CC BY-SA 4.0 (written for tanulom)"
# Far above Tatoeba's ids (< 4M) and the subtitle ids that follow them, so a
# rebuild never collides; stable per (unit, line) so a diff of the pack is small.
ID_BASE = 9_000_000
MIN_LINES, MAX_LINES = 8, 14
MAX_TOKENS = 10

TITLE_PREFIX_RU = "Диалог"
TITLE_PREFIX_EN = "Dialogue"

_WORD = re.compile(r"[^\W\d_]+(?:[-'][^\W\d_]+)*", re.UNICODE)
# Letters, spaces and ordinary punctuation. No digits (write the number), no
# speaker labels, no leading dash: the import adds the turn marker.
_ALLOWED = re.compile(r"^[^\W\d_](?:[^\W\d_]|[ \-',.!?;…«»„”\"–])*$", re.UNICODE)


def _load() -> dict[str, list[tuple[str, str, str]]]:
    from . import (dialogues_m1, dialogues_m2, dialogues_m3, dialogues_m4,
                   dialogues_m5, dialogues_m6, dialogues_themes)
    out: dict[str, list[tuple[str, str, str]]] = {}
    for mod in (dialogues_m1, dialogues_m2, dialogues_m3, dialogues_m4,
                dialogues_m5, dialogues_m6, dialogues_themes):
        for code, lines in mod.DIALOGUES.items():
            if code in out:
                raise ValueError(f"dialogue for {code} defined twice")
            out[code] = [tuple(x) for x in lines]
    return out


DIALOGUES = _load()


def set_code(unit_code: str) -> str:
    return f"{unit_code}-dialogue"


def line_id(unit_ord: int, i: int) -> int:
    return ID_BASE + unit_ord * 100 + i


class Lexicon:
    """Attested readings of a form, read once per form from the pack."""

    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn
        self._cache: dict[str, list[tuple[int, str, str, str | None]]] = {}

    def readings(self, token: str) -> list[tuple[int, str, str, str | None]]:
        """[(lemma_id, lemma, upos, cefr)] for every attested reading.

        A name only counts as one when the text spells it as a name: `barna`
        (brown) is not rescued by the first name `Barna`, so the lookup tries
        the token as written and lower-cased, never a capitalised guess, and
        drops proper-noun readings where a common word exists.
        """
        if token not in self._cache:
            variants = list(dict.fromkeys([token, token.lower()]))
            ph = ",".join("?" * len(variants))
            reads = list(self.conn.execute(
                "SELECT DISTINCT l.id, l.lemma, l.upos, l.cefr FROM wordform w "
                "JOIN lemma l ON l.id = w.lemma_id "
                f"WHERE w.form IN ({ph}) AND w.source <> 'generated'", variants))
            common = [r for r in reads if r[2] != "PROPN"]
            self._cache[token] = common or reads
        return self._cache[token]


def check_line(lex: Lexicon, hu: str, band: str, own_lemmas: set[str]) -> list[str]:
    """Why a line is not allowed in a dialogue of `band`; empty is fine."""
    out: list[str] = []
    if not _ALLOWED.match(hu):
        out.append("characters outside letters and punctuation (digits? a speaker label?)")
    tokens = _WORD.findall(hu)
    if len(tokens) > MAX_TOKENS:
        out.append(f"{len(tokens)} words, at most {MAX_TOKENS}")
    limit = rs._rank(band)
    for tok in tokens:
        # `van-e`: each part is looked up; the question particle `e` is a word.
        for piece in (tok.split("-") if "-" in tok else [tok]):
            reads = lex.readings(piece)
            if not reads:
                out.append(f"{piece!r}: not an attested wordform")
                continue
            ok = any(
                upos not in rs.CONTENT_UPOS or rs._rank(cefr) <= limit or lemma in own_lemmas
                for _lid, lemma, upos, cefr in reads)
            if not ok:
                lem = ", ".join(sorted({f"{lm}/{cf or '?'}" for _i, lm, _u, cf in reads}))
                out.append(f"{piece!r}: lemma above {band} ({lem})")
    return out


def _unit_words(unit: dict) -> set[str]:
    """Lemmas a learner at `unit` has been taught: its own words plus those of
    the core units before it (a theme: months 1-4, the A2 the themes sit at)."""
    def words(u: dict) -> set[str]:
        return {w[0] if isinstance(w, tuple) else w for w in u["words"]}

    out = words(unit)
    for u in _basic_units():
        if u["track"] != "core" or u is unit:
            continue
        if (u["ord"] < unit["ord"]) if unit["track"] == "core" else (u["month"] <= 4):
            out |= words(u)
    return out


def _basic_units() -> list[dict]:
    return [u for u in course.UNITS if u.get("tier", "basic") == "basic"]


def build(conn: sqlite3.Connection, rows: list[dict]) -> dict[str, int]:
    """Write every unit's dialogue after `reading_sets.build` wrote the «Фразы»
    sets (`rows`, whose `level` the dialogue shares). Idempotent."""
    conn.execute("DELETE FROM reading_set_sentence WHERE set_code IN "
                 "(SELECT code FROM reading_set WHERE track = ?)", (TRACK,))
    conn.execute("DELETE FROM reading_set WHERE track = ?", (TRACK,))
    conn.execute("DELETE FROM sentence_trans WHERE sentence_id IN "
                 "(SELECT id FROM sentence WHERE source = ?)", (SOURCE,))
    conn.execute("DELETE FROM sentence_token WHERE sentence_id IN "
                 "(SELECT id FROM sentence WHERE source = ?)", (SOURCE,))
    conn.execute("DELETE FROM sentence WHERE source = ?", (SOURCE,))

    by_code = {u["code"]: u for u in _basic_units()}
    sets = sentences = 0
    for row in rows:
        lines = DIALOGUES.get(row["code"])
        unit = by_code.get(row["code"])
        if not lines or unit is None:
            continue  # `problems` reports a unit without a dialogue
        code = set_code(row["code"])
        conn.execute(
            "INSERT INTO reading_set (code, level, track, ord, title_ru, title_en, theme_ru, theme_en) "
            "VALUES (?,?,?,?,?,?,?,?)",
            (code, row["level"], TRACK, row["ord"] - 1,
             f"{TITLE_PREFIX_RU}: {unit['title_ru']}", f"{TITLE_PREFIX_EN}: {unit['title_en']}",
             row["theme_ru"], row["theme_en"]))
        for i, (hu, ru, en) in enumerate(lines):
            sid = line_id(unit["ord"], i)
            conn.execute(
                "INSERT INTO sentence (id, hu, source, license, len_tokens, difficulty) "
                "VALUES (?,?,?,?,?,NULL)",
                (sid, hu, SOURCE, LICENSE, len(_WORD.findall(hu))))
            conn.executemany(
                "INSERT INTO sentence_trans (sentence_id, lang, text) VALUES (?,?,?)",
                [(sid, "ru", ru), (sid, "en", en)])
            conn.execute(
                "INSERT INTO reading_set_sentence (set_code, sentence_id, ord) VALUES (?,?,?)",
                (code, sid, i))
            sentences += 1
        sets += 1
    return {"sets": sets, "sentences": sentences}


def data_problems() -> list[str]:
    """Problems that need no pack: shape of the written text."""
    out: list[str] = []
    known = {u["code"] for u in _basic_units()}
    for code, lines in DIALOGUES.items():
        if code not in known:
            out.append(f"{code}: no such course unit")
        if not MIN_LINES <= len(lines) <= MAX_LINES:
            out.append(f"{code}: {len(lines)} lines, want {MIN_LINES}-{MAX_LINES}")
        seen: set[str] = set()
        for i, (hu, ru, en) in enumerate(lines):
            if not ru.strip() or not en.strip():
                out.append(f"{code}[{i}]: a translation is missing")
            if hu in seen:
                out.append(f"{code}[{i}]: a line repeated: {hu!r}")
            seen.add(hu)
    return out


def problems(conn: sqlite3.Connection) -> list[str]:
    """Everything wrong with the dialogues a pack carries; empty is good."""
    out = data_problems()
    have = {r[0] for r in conn.execute("SELECT code FROM reading_set")}
    units = {u["code"]: u for u in _basic_units()}
    for code in units:
        if code in have and set_code(code) not in have:
            out.append(f"{code}: has «Фразы» but no dialogue")
    lex = Lexicon(conn)
    for code, level in conn.execute(
            "SELECT code, level FROM reading_set WHERE track = ? ORDER BY ord", (TRACK,)):
        unit_code = code.removesuffix("-dialogue")
        own = _unit_words(units[unit_code]) if unit_code in units else set()
        lines = list(conn.execute(
            "SELECT s.id, s.hu, s.source, "
            "(SELECT text FROM sentence_trans WHERE sentence_id = s.id AND lang = 'ru'), "
            "(SELECT text FROM sentence_trans WHERE sentence_id = s.id AND lang = 'en') "
            "FROM reading_set_sentence x JOIN sentence s ON s.id = x.sentence_id "
            "WHERE x.set_code = ? ORDER BY x.ord", (code,)))
        for sid, hu, source, ru, en in lines:
            if source != SOURCE:
                out.append(f"{code}: sentence {sid} is {source!r}, a dialogue holds authored lines only")
            if not (ru or "").strip() or not (en or "").strip():
                out.append(f"{code}: sentence {sid} lacks a ru or en translation")
            for why in check_line(lex, hu, level, own):
                out.append(f"{code}: {hu!r}: {why}")
    (n,) = conn.execute(
        "SELECT COUNT(*) FROM sentence_token WHERE sentence_id IN "
        "(SELECT id FROM sentence WHERE source = ?)", (SOURCE,)).fetchone()
    if n:
        out.append(f"{n} sentence_token rows on authored sentences: they must have none")
    (n,) = conn.execute(
        "SELECT COUNT(*) FROM sentence WHERE source = ? AND id NOT IN "
        "(SELECT sentence_id FROM reading_set_sentence)", (SOURCE,)).fetchone()
    if n:
        out.append(f"{n} authored sentences belong to no dialogue set")
    return out


def main(argv: list[str]) -> int:
    """`python -m pipeline.dialogues PACK.sqlite`: check the written text against a
    pack WITHOUT writing to it (the author's loop)."""
    if len(argv) != 1:
        print("usage: python -m pipeline.dialogues PACK.sqlite", file=sys.stderr)
        return 2
    conn = sqlite3.connect(f"file:{Path(argv[0])}?mode=ro", uri=True)
    lex = Lexicon(conn)
    bands = _bands(conn)
    bad = 0
    for p in data_problems():
        print(p)
        bad += 1
    for u in _basic_units():
        lines = DIALOGUES.get(u["code"])
        if not lines:
            print(f"{u['code']}: no dialogue yet")
            continue
        for hu, _ru, _en in lines:
            for why in check_line(lex, hu, bands[u["code"]], _unit_words(u)):
                print(f"{u['code']}: {hu!r}: {why}")
                bad += 1
    print(f"{len(DIALOGUES)} dialogues, {bad} problems")
    return 1 if bad else 0


def _bands(conn: sqlite3.Connection) -> dict[str, str]:
    """The band of each unit's «Фразы» set, as the pack carries it (fall back
    to the month's band when the pack has no sets yet)."""
    out = {}
    have = {c: lv for c, lv in conn.execute(
        "SELECT code, level FROM reading_set WHERE track <> ?", (TRACK,))}
    for u in _basic_units():
        out[u["code"]] = have.get(
            u["code"], rs.MONTH_BAND.get(u["month"], "A2") if u["track"] == "core" else "A2")
    return out


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
