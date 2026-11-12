"""Authored stories: one short connected text per basic-tier course unit.

A «Рассказ: X» / "Story: X" is a narrated mini-scene or a caregiver-style
monologue: here-and-now reference, repetition, short frames reused with one slot
changed (R21). It took over the code and track of the old Tatoeba «Фразы» set
(the bare unit code), so an earlier import maps onto it (ADR-074).

PROVENANCE, as for dialogues (dialogues.py, ADR-066): stored in `sentence` with
`source = 'authored'`, translations in `sentence_trans`, NO `sentence_token`
rows; labelled «Написано ИИ, носителем не проверено»; reading only -- no drill,
example or n-gram index ever reads it.

SOURCE FORMAT (stories_m1.py ... stories_m6.py, stories_themes.py):

    STORIES = {unit_code: [[(hu, ru, en), ...], [...]]}

a list of paragraphs, each a list of sentence tuples. 2-4 paragraphs, 8-14
sentences in all, at most MAX_TOKENS words a sentence, a ru and an en
translation each. Ids: ID_BASE + unit_ord * 100 + 50 + i (dialogues own +0..49).

WHAT THE BUILD CHECKS (`problems`): the dialogue rules (every word an attested,
non-'generated' wordform; every content lemma in the unit's band or taught
earlier; length), plus the FREQUENCY GATE: the share of running tokens whose
lemma has `lemma.freq_rank <= TOP_N[unit]` must reach MIN_TOP_SHARE. The unit's
own words and proper nouns are exempt (left out of the count): a text built
around `kenyér` is allowed `kenyér`, not three more rare nouns.

AUTHOR'S LOOP (read-only, writes nothing to the pack):

    python -m pipeline.stories PACK.sqlite --brief m1-u1-koszones   # the brief
    python -m pipeline.stories PACK.sqlite [--require-all]          # validate
"""

from __future__ import annotations

import argparse
import sqlite3
import sys
from pathlib import Path

from . import course, dialogues
from . import reading_sets as rs

TRACK_DIALOGUE = dialogues.TRACK
SOURCE = dialogues.SOURCE
LICENSE = dialogues.LICENSE
ID_BASE = dialogues.ID_BASE
ID_OFFSET = 50
MIN_SENTENCES, MAX_SENTENCES = 8, 14
MIN_PARAS, MAX_PARAS = 2, 4
MAX_TOKENS = dialogues.MAX_TOKENS

TITLE_PREFIX_RU = "Рассказ"
TITLE_PREFIX_EN = "Story"

# Frequency gate: running tokens whose lemma ranks within TOP_N. 500 for months
# 1-2, 1000 for 3-4, 2000 for 5-6 and the themes. A tuning, not a published figure.
TOP_N_BY_MONTH = {1: 500, 2: 500, 3: 1000, 4: 1000, 5: 2000, 6: 2000}
TOP_N_THEME = 2000
MIN_TOP_SHARE = 0.85
NO_RANK = 10 ** 9
BRIEF_LEMMAS = 400


def _load() -> dict[str, list[list[tuple[str, str, str]]]]:
    from . import (stories_m1, stories_m2, stories_m3, stories_m4, stories_m5,
                   stories_m6, stories_themes)
    out: dict[str, list[list[tuple[str, str, str]]]] = {}
    for mod in (stories_m1, stories_m2, stories_m3, stories_m4, stories_m5,
                stories_m6, stories_themes):
        for code, paras in mod.STORIES.items():
            if code in out:
                raise ValueError(f"story for {code} defined twice")
            out[code] = [[tuple(x) for x in para] for para in paras]
    return out


STORIES = _load()


def set_code(unit_code: str) -> str:
    return unit_code


def sentence_id(unit_ord: int, i: int) -> int:
    return ID_BASE + unit_ord * 100 + ID_OFFSET + i


def top_n(unit: dict) -> int:
    return TOP_N_BY_MONTH.get(unit["month"], TOP_N_THEME) if unit["track"] == "core" else TOP_N_THEME


def _basic_units() -> list[dict]:
    return dialogues._basic_units()


def flatten(paras: list[list[tuple[str, str, str]]]) -> list[tuple[int, str, str, str]]:
    """[(paragraph, hu, ru, en)] in reading order."""
    return [(p, hu, ru, en) for p, para in enumerate(paras) for hu, ru, en in para]


# ── the frequency gate ─────────────────────────────────────────────────────

class FreqLexicon(dialogues.Lexicon):
    """The dialogue lexicon plus lemma frequency ranks."""

    def __init__(self, conn: sqlite3.Connection) -> None:
        super().__init__(conn)
        self.rank = {i: (r if r is not None else NO_RANK)
                     for i, r in conn.execute("SELECT id, freq_rank FROM lemma")}


def frequency_report(lex: FreqLexicon, hu_lines: list[str], unit: dict) -> dict:
    """Share of running tokens in the top N, the rarest lemma, the tokens that
    miss. Exempt (not counted): the unit's own words and proper nouns."""
    own = {w[0] if isinstance(w, tuple) else w for w in unit["words"]}
    n = top_n(unit)
    counted = top = exempt = 0
    rarest = (None, 0)  # (lemma, rank)
    misses: dict[str, int] = {}
    for hu in hu_lines:
        for tok in dialogues._WORD.findall(hu):
            for piece in (tok.split("-") if "-" in tok else [tok]):
                reads = lex.readings(piece)
                if not reads:
                    continue  # check_line reports it
                if any(upos == "PROPN" or lemma in own for _i, lemma, upos, _c in reads):
                    exempt += 1
                    continue
                best = min(reads, key=lambda r: lex.rank.get(r[0], NO_RANK))
                rk = lex.rank.get(best[0], NO_RANK)
                counted += 1
                if rk <= n:
                    top += 1
                else:
                    misses[best[1]] = rk
                if rk > rarest[1]:
                    rarest = (best[1], rk)
    return {"n": n, "counted": counted, "top": top, "exempt": exempt,
            "share": (top / counted) if counted else 1.0,
            "rarest": rarest[0], "rarest_rank": rarest[1], "misses": misses}


def _fmt_report(code: str, rep: dict, problems: list[str]) -> str:
    rk = "unranked" if rep["rarest_rank"] >= NO_RANK else str(rep["rarest_rank"])
    s = (f"{code}: top-{rep['n']} share {rep['share']:.0%} ({rep['top']}/{rep['counted']}, {rep['exempt']} exempt), "
         f"rarest {rep['rarest'] or '-'} ({rk})")
    if rep["misses"]:
        s += "; beyond top-N: " + ", ".join(
            f"{lm}={'?' if r >= NO_RANK else r}" for lm, r in sorted(rep["misses"].items(),
                                                                   key=lambda x: -x[1])[:8])
    if problems:
        s += "\n    " + "\n    ".join(problems)
    return s


# ── writing the pack ───────────────────────────────────────────────────────

def build(conn: sqlite3.Connection, rows: list[dict]) -> dict[str, int]:
    """Write every unit's story (`rows` from `reading_sets.unit_rows`). Runs
    AFTER `dialogues.build`, which clears all authored sentences. A unit with
    no story yet is skipped with a printed warning."""
    by_code = {u["code"]: u for u in _basic_units()}
    sets = sentences = 0
    missing = []
    for row in rows:
        paras = STORIES.get(row["code"])
        unit = by_code.get(row["code"])
        if not paras or unit is None:
            missing.append(row["code"])
            continue
        conn.execute(
            "INSERT INTO reading_set (code, level, track, ord, title_ru, title_en, theme_ru, theme_en) "
            "VALUES (?,?,?,?,?,?,?,?)",
            (set_code(row["code"]), row["level"], row["track"], row["ord"],
             row["title_ru"], row["title_en"], row["theme_ru"], row["theme_en"]))
        for i, (para, hu, ru, en) in enumerate(flatten(paras)):
            sid = sentence_id(unit["ord"], i)
            conn.execute(
                "INSERT INTO sentence (id, hu, source, license, len_tokens, difficulty) "
                "VALUES (?,?,?,?,?,NULL)",
                (sid, hu, SOURCE, LICENSE, len(dialogues._WORD.findall(hu))))
            conn.executemany(
                "INSERT INTO sentence_trans (sentence_id, lang, text) VALUES (?,?,?)",
                [(sid, "ru", ru), (sid, "en", en)])
            conn.execute(
                "INSERT INTO reading_set_sentence (set_code, sentence_id, ord, para) VALUES (?,?,?,?)",
                (set_code(row["code"]), sid, i, para))
            sentences += 1
        sets += 1
    if missing:
        print(f"  warning: {len(missing)} units have no story yet: {', '.join(missing[:6])}"
              f"{' ...' if len(missing) > 6 else ''}", file=sys.stderr)
    return {"sets": sets, "sentences": sentences}


# ── validation ─────────────────────────────────────────────────────────────

def data_problems() -> list[str]:
    """Problems that need no pack: shape of the written text."""
    out: list[str] = []
    known = {u["code"]: u for u in _basic_units()}
    for code, paras in STORIES.items():
        if code not in known:
            out.append(f"{code}: no such course unit")
        if not MIN_PARAS <= len(paras) <= MAX_PARAS:
            out.append(f"{code}: {len(paras)} paragraphs, want {MIN_PARAS}-{MAX_PARAS}")
        flat = flatten(paras)
        if not MIN_SENTENCES <= len(flat) <= MAX_SENTENCES:
            out.append(f"{code}: {len(flat)} sentences, want {MIN_SENTENCES}-{MAX_SENTENCES}")
        if len(flat) > ID_OFFSET:
            out.append(f"{code}: too many sentences for the id range")
        seen: set[str] = set()
        for i, (para, hu, ru, en) in enumerate(flat):
            if not paras[para]:
                out.append(f"{code}: empty paragraph {para}")
            if not ru.strip() or not en.strip():
                out.append(f"{code}[{i}]: a translation is missing")
            nw = len(dialogues._WORD.findall(hu))
            if not 1 <= nw <= MAX_TOKENS:
                out.append(f"{code}[{i}]: {nw} words, at most {MAX_TOKENS}: {hu!r}")
            if hu in seen:
                out.append(f"{code}[{i}]: a sentence repeated: {hu!r}")
            seen.add(hu)
    return out


def check_story(lex: FreqLexicon, unit: dict, band: str,
                hu_lines: list[str]) -> tuple[dict, list[str]]:
    """(frequency report, problems) for one text of `unit`."""
    own = dialogues._unit_words(unit)
    out: list[str] = []
    for hu in hu_lines:
        for why in dialogues.check_line(lex, hu, band, own):
            out.append(f"{hu!r}: {why}")
    rep = frequency_report(lex, hu_lines, unit)
    if rep["share"] < MIN_TOP_SHARE:
        out.append(f"frequency gate: {rep['share']:.0%} of words in the top {rep['n']}, "
                   f"want at least {MIN_TOP_SHARE:.0%}")
    return rep, out


def problems(conn: sqlite3.Connection, require_all: bool = False) -> list[str]:
    """Everything wrong with the stories a pack carries; empty is good."""
    out = data_problems()
    units = {u["code"]: u for u in _basic_units()}
    have = {r[0]: r[1] for r in conn.execute(
        "SELECT code, level FROM reading_set WHERE track <> ?", (TRACK_DIALOGUE,))}
    if require_all:
        for code in units:
            if code not in STORIES:
                out.append(f"{code}: no story written")
            elif code not in have:
                out.append(f"{code}: the pack has no story set for it")
    lex = FreqLexicon(conn)
    # A pack from before the stories has no `para`; `reading_sets.build` adds it.
    para_col = "x.para" if "para" in {r[1] for r in conn.execute(
        "PRAGMA table_info(reading_set_sentence)")} else "0"
    for code, level in have.items():
        unit = units.get(code)
        if unit is None:
            out.append(f"{code}: a story set for no basic-tier unit")
            continue
        lines = list(conn.execute(
            f"SELECT s.id, s.hu, s.source, {para_col}, "
            "(SELECT text FROM sentence_trans WHERE sentence_id = s.id AND lang = 'ru'), "
            "(SELECT text FROM sentence_trans WHERE sentence_id = s.id AND lang = 'en') "
            "FROM reading_set_sentence x JOIN sentence s ON s.id = x.sentence_id "
            "WHERE x.set_code = ? ORDER BY x.ord", (code,)))
        paras = {p for _s, _h, _src, p, _r, _e in lines}
        if not MIN_PARAS <= len(paras) <= MAX_PARAS:
            out.append(f"{code}: {len(paras)} paragraphs, want {MIN_PARAS}-{MAX_PARAS}")
        for sid, _hu, source, _p, ru, en in lines:
            if source != SOURCE:
                out.append(f"{code}: sentence {sid} is {source!r}, a story holds authored lines only")
            if not (ru or "").strip() or not (en or "").strip():
                out.append(f"{code}: sentence {sid} lacks a ru or en translation")
        _rep, probs = check_story(lex, unit, level, [hu for _s, hu, *_ in lines])
        out.extend(f"{code}: {p}" for p in probs)
    return out


# ── the author's CLI ───────────────────────────────────────────────────────

def brief(conn: sqlite3.Connection, code: str) -> str:
    """What an author needs to write the unit's story; read-only."""
    unit = next((u for u in _basic_units() if u["code"] == code), None)
    if unit is None:
        raise SystemExit(f"{code}: not a basic-tier unit; known: "
                         + ", ".join(u["code"] for u in _basic_units()))
    band = rs.unit_band(conn, unit)
    n = top_n(unit)
    own = [w[0] if isinstance(w, tuple) else w for w in unit["words"]]
    taught = dialogues._unit_words(unit)
    L = [f"UNIT {code}: {unit['title_ru']} / {unit['title_en']}",
         f"CEFR band: {band}   frequency gate: >= {MIN_TOP_SHARE:.0%} of words within top {n}",
         f"Situation: {unit.get('situation_ru', '')}", "",
         "RULES: one connected text, 2-4 paragraphs, 8-14 sentences in all, at most "
         f"{MAX_TOKENS} words a sentence, no digits (write numbers as words), no speaker labels. "
         "Child-directed style: here-and-now, repetition, a short frame reused with one slot "
         "changed. Repeat each unit word at least twice. Two named people, one coherent scene. "
         "Every word an attested form of the pack. Format:",
         f'    STORIES = {{"{code}": [[(hu, ru, en), ...], [(hu, ru, en), ...]]}}',
         "", "UNIT WORDS (taught; exempt from the frequency gate):"]
    for w in unit["words"]:
        if isinstance(w, tuple):
            L.append(f"  {w[0]} ({w[1]}): {w[2]} / {w[3]}")
        else:
            L.append(f"  {w}")
    ok = []
    seen: set[str] = set()
    for lid, lemma, upos, cefr, rank in conn.execute(
            "SELECT id, lemma, upos, cefr, freq_rank FROM lemma "
            "WHERE freq_rank IS NOT NULL AND upos <> 'PROPN' ORDER BY freq_rank"):
        if lemma not in seen and (lemma in taught or rs._rank(cefr) <= rs._rank(band)):
            seen.add(lemma)
            ok.append((rank, lemma, upos, lemma in taught and lemma not in own))
    L += ["", f"ALLOWED LEMMAS by frequency rank (band {band} or taught earlier; top {BRIEF_LEMMAS}"
          " shown; * = taught earlier):"]
    L.append("  " + "  ".join(f"{lm}/{up}{'*' if t else ''}={rk}"
                              for rk, lm, up, t in ok[:BRIEF_LEMMAS]))
    d = dialogues.DIALOGUES.get(code)
    L += ["", "THE UNIT'S EXISTING DIALOGUE (match its register, do not copy it):"]
    L += [f"  {hu}  |  {ru}" for hu, ru, _en in (d or [])] or ["  (none yet)"]
    return "\n".join(L)


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(
        prog="python -m pipeline.stories",
        description="Validate the written stories (and report on the dialogues) against a "
                    "pack, read-only; --brief prints an author's brief for one unit.")
    ap.add_argument("pack", type=Path)
    ap.add_argument("--brief", metavar="UNIT")
    ap.add_argument("--require-all", action="store_true",
                    help="a missing story is a problem (what `make starter` enforces)")
    args = ap.parse_args(argv)
    conn = sqlite3.connect(f"file:{args.pack}?mode=ro", uri=True)
    if args.brief:
        print(brief(conn, args.brief))
        return 0
    lex = FreqLexicon(conn)
    bad = 0
    for p in data_problems():
        print(p)
        bad += 1
    print("== stories")
    for u in _basic_units():
        paras = STORIES.get(u["code"])
        if not paras:
            msg = f"{u['code']}: no story yet"
            print(msg)
            if args.require_all:
                bad += 1
            continue
        rep, probs = check_story(lex, u, rs.unit_band(conn, u),
                                 [hu for _p, hu, _r, _e in flatten(paras)])
        print(_fmt_report(u["code"], rep, probs))
        bad += len(probs)
    print("== dialogues (frequency report only, not a failure)")
    for u in _basic_units():
        lines = dialogues.DIALOGUES.get(u["code"])
        if not lines:
            continue
        rep = frequency_report(lex, [hu for hu, _r, _e in lines], u)
        flag = "  <-- below the gate" if rep["share"] < MIN_TOP_SHARE else ""
        print(_fmt_report(u["code"], rep, []) + flag)
    print(f"{len(STORIES)} stories, {bad} problems")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
