"""Rewrite the pure-Python tables of an existing pack, in seconds.

`pipeline.build` is a full rebuild: it fetches Wiktionary and Tatoeba, parses
several gigabytes and takes hours. That is the right price for the lexicon and
the sentence corpus, and an absurd one for a typo in a grammar lesson.

Four tables in the pack are computed from Python literals alone and depend on
nothing that has to be downloaded:

    morph_feature   the feature inventory, labels and explanations
    grammar_topic   the teaching DAG and every lesson
    minimal_pair    the ear trainer's contrasts
    course_unit     the guided course, and course_word with it

This command rewrites exactly those, in place, in an existing pack. Everything
else in the file is untouched.

    python -m pipeline.refresh --pack ../data/langpack-hu-v1.sqlite

**This writes to a pack, which the running app treats as read-only.** That is
fine and is the point: a pack is a build artefact, and this is a build step.
Stop the server first — SQLite will not stop you, and a reader holding the file
open across the rewrite is asking for a stale page cache.

Rebuilding the pack is only half the job. The binary embeds its own starter
pack, so after this run `make starter` and commit the result, or the shipped
build still carries last month's lessons. See ADR-010.
"""

from __future__ import annotations

import argparse
import json
import sqlite3
from pathlib import Path

from . import course, curriculum, features, minimal_pairs

# table -> (columns, row-builder). Order matters: course_word references
# course_unit, so units are written first.
REFRESHED = ("morph_feature", "grammar_topic", "minimal_pair", "course_unit", "course_word")


def _rows_morph_feature() -> list[tuple]:
    return [
        (f.id, f.key, f.value, f.label_ru, f.label_en,
         json.dumps(f.allomorphs, ensure_ascii=False),
         f.explain_ru, f.explain_en, f.teach_order, ",".join(f.prereq_ids), f.grid)
        for f in features.ALL_FEATURES
    ]


def _rows_grammar_topic() -> list[tuple]:
    return [
        (t.code, t.level, t.title_ru, t.title_en, t.explain_ru, t.explain_en,
         ",".join(t.prereq_codes), ",".join(t.feature_ids), t.teach_order)
        for t in curriculum.TOPICS
    ]


def _rows_minimal_pair() -> list[tuple]:
    return [
        (p.a, p.b, p.contrast, p.priority, p.gloss_a_ru, p.gloss_b_ru,
         p.gloss_a_en, p.gloss_b_en, p.context_a, p.context_b)
        for p in minimal_pairs.PAIRS
    ]


def _rows_course_unit() -> list[tuple]:
    return [
        (u["code"], u["month"], u["ord"], u["title_ru"], u["title_en"],
         u["goal_ru"], u["goal_en"], u["situation_ru"], u["situation_en"],
         u.get("topic_codes", ""))
        for u in course.UNITS
    ]


COLUMNS = {
    "morph_feature": ["id", "key", "value", "label_ru", "label_en", "allomorphs",
                      "explain_ru", "explain_en", "teach_order", "prereq_ids", "grid"],
    "grammar_topic": ["code", "level", "title_ru", "title_en", "explain_md_ru",
                      "explain_md_en", "prereq_codes", "feature_ids", "teach_order"],
    "minimal_pair": ["a", "b", "contrast", "priority", "gloss_a_ru", "gloss_b_ru",
                     "gloss_a_en", "gloss_b_en", "context_a", "context_b"],
    "course_unit": ["code", "month", "ord", "title_ru", "title_en", "goal_ru",
                    "goal_en", "situation_ru", "situation_en", "topic_codes"],
    "course_word": ["unit_code", "lemma", "upos", "gloss_ru", "gloss_en", "note", "ord"],
}

BUILDERS = {
    "morph_feature": _rows_morph_feature,
    "grammar_topic": _rows_grammar_topic,
    "minimal_pair": _rows_minimal_pair,
    "course_unit": _rows_course_unit,
    "course_word": course.words,
}


# Indexes introduced after the first packs were built. `IF NOT EXISTS`, so this
# is a no-op on a pack that already has them.
MISSING_INDEXES = (
    "CREATE INDEX IF NOT EXISTS idx_stoken_form ON sentence_token(form)",
)


def _pack_lemmas(pack: Path) -> set[str]:
    """Every lemma the pack can look up, for the course's own validation."""
    conn = sqlite3.connect(f"file:{pack}?mode=ro", uri=True)
    try:
        have = {r[0] for r in conn.execute(
            "SELECT name FROM sqlite_master WHERE type='table'")}
        if "lemma" not in have:
            return set()
        return {r[0] for r in conn.execute("SELECT lemma FROM lemma")}
    finally:
        conn.close()


def refresh(pack: Path) -> dict[str, int]:
    if not pack.exists():
        raise SystemExit(f"no such pack: {pack}")

    # Validate BEFORE touching the file. A pack left half-rewritten by a
    # curriculum error is worse than one carrying the old lessons.
    problems = features.validate()
    problems += curriculum.validate(set(features.by_id()))
    problems += minimal_pairs.validate()
    problems += minimal_pairs.unattested_in(pack)
    problems += course.validate(_pack_lemmas(pack), {t.code for t in curriculum.TOPICS})
    if problems:
        raise SystemExit("refusing to write: " + "\n  ".join([""] + problems))

    conn = sqlite3.connect(pack)
    written: dict[str, int] = {}
    try:
        with conn:  # one transaction: all four tables swap together or none do
            have = {
                r[0] for r in conn.execute(
                    "SELECT name FROM sqlite_master WHERE type='table'")
            }
            # The pairs' old ids, so their audio rows can follow them: the
            # rewrite assigns ids in list order, and a row left pointing at
            # id 5 would play the old pair 5's clips for whatever pair 5 is now.
            old_pairs = {
                r[0]: (r[1], r[2])
                for r in conn.execute("SELECT id, a, b FROM minimal_pair")
            } if "minimal_pair" in have else {}
            # course_word has a foreign key onto course_unit, so the deletes
            # run child-first and the inserts parent-first.
            for table in reversed(REFRESHED):
                if table in have:
                    conn.execute(f"DELETE FROM {table}")
            for table in REFRESHED:
                if table not in have:
                    continue
                cols = COLUMNS[table]
                rows = BUILDERS[table]()
                conn.executemany(
                    f"INSERT INTO {table} ({','.join(cols)}) "
                    f"VALUES ({','.join('?' * len(cols))})",
                    rows,
                )
                written[table] = len(rows)

            # Audio rows follow their pair to its new id; rows of a pair that
            # is gone are dropped. Through negative ids, so a swap of two pairs
            # cannot collide halfway.
            if "pair_audio" in have and old_pairs:
                new_ids = {
                    (a, b): i for i, a, b in conn.execute("SELECT id, a, b FROM minimal_pair")
                }
                conn.execute("UPDATE pair_audio SET pair_id = -pair_id")
                for old_id, key in old_pairs.items():
                    if key in new_ids:
                        conn.execute("UPDATE pair_audio SET pair_id = ? WHERE pair_id = ?",
                                     (new_ids[key], -old_id))
                dropped = conn.execute("DELETE FROM pair_audio WHERE pair_id < 0").rowcount
                written["pair_audio (dropped)"] = dropped

            # Indexes added to schema.sql after a pack was built are not in that
            # pack, and the app cannot create them: it opens the pack read-only.
            # Adding one here costs nothing and keeps an existing pack usable.
            for stmt in MISSING_INDEXES:
                conn.execute(stmt)
    finally:
        conn.close()
    return written


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Rewrite the curriculum tables of an existing language pack")
    ap.add_argument("--pack", type=Path, required=True)
    args = ap.parse_args()

    for table, n in refresh(args.pack).items():
        print(f"  {table}: {n}")
    print("\n  now run `make starter` and commit, or the binary still ships the old text")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
