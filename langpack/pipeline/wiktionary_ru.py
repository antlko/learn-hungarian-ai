"""Russian glosses from the Russian Wiktionary, via wiktextract.

Why a second gloss source at all. The pack's `lemma_gloss` came from the English
Wiktionary only, so a learner whose interface is Russian read English definitions
of Hungarian words — two foreign languages to cross for one meaning. The app
labelled that honestly (a fallback gloss is marked as one) but labelling a gap is
not closing it.

What this closes, and what it does not. The Russian edition carries roughly
12 300 glossed Hungarian entries against the English edition's 45 900, and its
shape is lopsided: about 10 000 nouns and under 600 verbs. So this is not a
replacement, it is a first choice with English underneath — which is exactly the
right way round, because the words a beginner meets are the ones ruwiktionary
does cover. Where it does not, `gloss_fallback` in the UI already says so.

Run standalone against an existing pack (minutes, no full rebuild):

    python3 -m pipeline.wiktionary_ru --pack ../data/langpack-hu-v1.sqlite
"""

from __future__ import annotations

import argparse
import json
import re
import sqlite3
import sys
from collections import defaultdict
from pathlib import Path

from . import sources
from .wiktionary import POS_TO_UPOS

# Glosses that carry no meaning. ruwiktionary uses these placeholders for
# entries somebody created and nobody filled in; importing them would replace a
# usable English gloss with a dash.
_EMPTY = re.compile(r"^[\s\-–—?.]*$")

# A leading register/domain label is kept ("зоол. собака" tells the learner more
# than "собака"), but wiki leftovers are not.
_WIKI_JUNK = re.compile(r"\[\[|\]\]|\{\{|\}\}|''+")

# Beyond this a "gloss" is an encyclopaedia paragraph, and a card that shows one
# is a card nobody reads. The English side applies the same discipline.
MAX_GLOSS_CHARS = 120

# How many senses of one lemma reach the pack. The first is what the card shows;
# the rest are there for the word's detail view. A noun with eleven senses is a
# dictionary entry, not a flashcard.
MAX_SENSES = 3


def _clean(gloss: str) -> str | None:
    g = _WIKI_JUNK.sub("", gloss).strip().strip(";,")
    if not g or _EMPTY.match(g) or len(g) > MAX_GLOSS_CHARS:
        return None
    return g


def parse(path: Path) -> dict[tuple[str, str], list[str]]:
    """(lemma, upos) -> ordered, de-duplicated Russian glosses."""
    out: dict[tuple[str, str], list[str]] = defaultdict(list)
    skipped_pos: set[str] = set()
    with path.open(encoding="utf-8") as fh:
        for line in fh:
            try:
                raw = json.loads(line)
            except json.JSONDecodeError:
                continue
            word = (raw.get("word") or "").strip()
            if not word:
                continue
            pos = raw.get("pos", "")
            upos = POS_TO_UPOS.get(pos)
            if upos is None:
                skipped_pos.add(pos)
                continue
            key = (word, upos)
            for sense in raw.get("senses", []):
                for gloss in sense.get("glosses") or []:
                    g = _clean(gloss)
                    if g and g not in out[key]:
                        out[key].append(g)
    if skipped_pos:
        print(f"  ru: ignored parts of speech: {sorted(skipped_pos)}", file=sys.stderr)
    return {k: v[:MAX_SENSES] for k, v in out.items() if v}


def apply(conn: sqlite3.Connection, glosses: dict[tuple[str, str], list[str]]) -> dict[str, int]:
    """Insert Russian glosses beside the English ones. Returns counts.

    Matching is on (lemma, upos) first and lemma alone second. The relaxed match
    matters more than it looks: ruwiktionary tags `piros` as an adjective and the
    frequency-derived lemma may be a noun, and refusing that pair would drop a
    colour word to teach a tagging convention.
    """
    lemma_ids: dict[tuple[str, str], int] = {}
    by_word: dict[str, int] = {}
    for lid, lemma, upos in conn.execute("SELECT id, lemma, upos FROM lemma"):
        lemma_ids[(lemma, upos)] = lid
        by_word.setdefault(lemma, lid)

    rows: list[tuple[int, str, int, str]] = []
    seen: set[int] = set()
    exact = relaxed = 0
    for (lemma, upos), senses in glosses.items():
        lid = lemma_ids.get((lemma, upos))
        if lid is not None:
            exact += 1
        else:
            lid = by_word.get(lemma)
            if lid is None:
                continue
            relaxed += 1
        # One lemma can be reached twice (noun and adj entries for one word).
        # The first match wins rather than the senses being concatenated, so a
        # card never shows two unrelated definitions as if they were one word.
        if lid in seen:
            continue
        seen.add(lid)
        for i, g in enumerate(senses):
            rows.append((lid, "ru", i, g))

    conn.execute("DELETE FROM lemma_gloss WHERE lang = 'ru'")
    conn.executemany(
        "INSERT OR REPLACE INTO lemma_gloss (lemma_id, lang, sense_idx, gloss) VALUES (?,?,?,?)",
        rows,
    )
    return {
        "entries": len(glosses),
        "lemmas_matched": len(seen),
        "exact_pos": exact,
        "relaxed_pos": relaxed,
        "gloss_rows": len(rows),
    }


def apply_course_glosses(conn: sqlite3.Connection) -> dict[str, int]:
    """Promote the course's hand-written Russian glosses into `lemma_gloss`.

    These are better than anything a dump can give, for the words that matter
    most. They were written for this syllabus by someone who chose which of a
    word's senses a beginner needs — `kér` is "просить", not "просить, требовать,
    заказывать, ходатайствовать" — and they cover all 853 course words, where
    ruwiktionary reaches 481 of them.

    So they take precedence over the imported gloss rather than filling in
    behind it. A learner in month two is looking at the sense the unit teaches;
    showing them the dictionary's first sense instead would be a small lie told
    853 times.
    """
    lemma_ids: dict[tuple[str, str], int] = {}
    by_word: dict[str, int] = {}
    for lid, lemma, upos in conn.execute("SELECT id, lemma, upos FROM lemma"):
        lemma_ids[(lemma, upos)] = lid
        by_word.setdefault(lemma, lid)

    promoted = unmatched = 0
    done: set[int] = set()
    for lemma, upos, gloss in conn.execute(
        "SELECT lemma, upos, gloss_ru FROM course_word ORDER BY unit_code, ord"
    ).fetchall():
        if not gloss:
            continue
        lid = lemma_ids.get((lemma, upos)) or by_word.get(lemma)
        if lid is None:
            unmatched += 1
            continue
        # Two units can teach one lemma (a homograph, or a word that recurs in a
        # new sense). The earlier unit's gloss stands: that is the sense the
        # learner met first and the one their card already carries.
        if lid in done:
            continue
        done.add(lid)

        # sense_idx 0 is what a card shows, so the course gloss takes it and the
        # imported senses shift down behind it — nothing is lost, the word's
        # detail view still has the dictionary reading.
        #
        # Rewritten rather than UPDATE ... sense_idx + 1: that increments 0 into
        # an occupied 1 and SQLite rejects it mid-statement, because the unique
        # index is checked per row and not at the end.
        existing = [
            g
            for (g,) in conn.execute(
                "SELECT gloss FROM lemma_gloss WHERE lemma_id = ? AND lang = 'ru' "
                "ORDER BY sense_idx",
                (lid,),
            )
            if g != gloss
        ]
        conn.execute("DELETE FROM lemma_gloss WHERE lemma_id = ? AND lang = 'ru'", (lid,))
        conn.executemany(
            "INSERT INTO lemma_gloss (lemma_id, lang, sense_idx, gloss) VALUES (?, 'ru', ?, ?)",
            [(lid, i, g) for i, g in enumerate([gloss, *existing][:MAX_SENSES])],
        )
        promoted += 1
    return {"promoted": promoted, "unmatched": unmatched}


def record_gloss_lang(conn: sqlite3.Connection) -> list[str]:
    """Keep `pack_meta.gloss_langs` truthful about what is actually in there.

    The Go side reads this to decide whether "both" is a gloss setting it can
    offer, so a stale value here is a setting that silently does nothing.
    """
    present = [
        lang
        for (lang,) in conn.execute(
            "SELECT DISTINCT lang FROM lemma_gloss ORDER BY lang = 'en' DESC, lang"
        )
    ]
    conn.execute(
        "INSERT OR REPLACE INTO pack_meta (key, value) VALUES ('gloss_langs', ?)",
        (json.dumps(present),),
    )
    return present


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--pack", required=True, type=Path, help="the pack to add Russian glosses to")
    ap.add_argument("--force-fetch", action="store_true")
    args = ap.parse_args()

    if not args.pack.exists():
        print(f"no pack at {args.pack}", file=sys.stderr)
        return 1

    source = next(s for s in sources.SOURCES if s.key == "wiktionary_ru")
    path = sources.fetch(source, force=args.force_fetch)
    if path is None:
        print("  ! Russian Wiktionary unavailable — the pack keeps English glosses only")
        return 0

    glosses = parse(path)
    conn = sqlite3.connect(args.pack)
    try:
        stats = apply(conn, glosses)
        course = apply_course_glosses(conn)
        langs = record_gloss_lang(conn)
        conn.commit()
    finally:
        conn.close()

    print(
        f"  ru glosses: {stats['gloss_rows']} rows over {stats['lemmas_matched']} lemmas "
        f"({stats['exact_pos']} exact POS, {stats['relaxed_pos']} relaxed) "
        f"from {stats['entries']} entries"
    )
    print(
        f"  course glosses promoted to sense 0: {course['promoted']}"
        + (f" ({course['unmatched']} course words have no lemma row)" if course["unmatched"] else "")
    )
    print(f"  pack_meta.gloss_langs = {langs}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
