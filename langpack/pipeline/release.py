"""Derive the PUBLISHABLE full pack from a full pack built on this machine.

`make langpack` builds data/langpack-hu-v1.sqlite with OPUS OpenSubtitles
sentences in it. That is fine for the learner's own disk, but the OpenSubtitles
licence does not cover redistributing them, so the pack that is UPLOADED and
installed from Settings (ADR-049) must carry Tatoeba sentences only — the same
thing `pipeline.build --subtitle-sentences 0` produces from the raw dumps.
Those dumps are many gigabytes; this module gets the same artefact from the
pack already on disk:

  * every sentence whose source is not 'tatoeba' is removed (an allow-list, as
    in starter.py: a future source stays out by default), with its tokens,
    translations and any reading-set membership;
  * sentence ids are NOT renumbered — a learner's notes reference them, and the
    installer's id-compatibility check compares against the starter's ids;
  * the n-gram attestation index is rebuilt from what is left (counts are what
    make it a guardrail; build.py and add_subtitles.py do the same);
  * pack_meta stats are recomputed so the manifest describes the file it ships.

    python -m pipeline.release --full ../data/langpack-hu-v1.sqlite --out ../dist
"""

from __future__ import annotations

import argparse
import gzip
import hashlib
import json
import shutil
import sqlite3
import sys
from collections import Counter
from pathlib import Path

from . import emit
from .add_subtitles import tokenise

PACK_NAME = "langpack-hu-v1.sqlite"
ALLOWED_SOURCE = "tatoeba"

# What backend/internal/lang expects to find (pack.go Counts + the readers).
REQUIRED_TABLES = (
    "pack_meta", "lemma", "lemma_gloss", "wordform", "paradigm", "morph_feature",
    "sentence", "sentence_trans", "sentence_token", "minimal_pair", "pair_audio",
    "grammar_topic", "course_unit", "course_word", "resource",
    "reading_set", "reading_set_sentence", "ngram",
)


def strip_sentences(conn: sqlite3.Connection) -> int:
    """Delete every non-allowed sentence and everything hanging off it."""
    drop = "(SELECT id FROM sentence WHERE source <> ?)"
    n = conn.execute("SELECT COUNT(*) FROM sentence WHERE source <> ?",
                     (ALLOWED_SOURCE,)).fetchone()[0]
    for table in ("sentence_token", "sentence_trans", "reading_set_sentence"):
        conn.execute(f"DELETE FROM {table} WHERE sentence_id IN {drop}", (ALLOWED_SOURCE,))
    conn.execute("DELETE FROM sentence WHERE source <> ?", (ALLOWED_SOURCE,))
    # A set left with no sentence is not a set.
    conn.execute("DELETE FROM reading_set WHERE code NOT IN "
                 "(SELECT set_code FROM reading_set_sentence)")
    return n


def rebuild_ngrams(conn: sqlite3.Connection) -> int:
    counter: Counter[tuple[str, int]] = Counter()
    for (text,) in conn.execute("SELECT hu FROM sentence"):
        toks = [t.lower() for t in tokenise(text)]
        for n in (2, 3):
            for i in range(len(toks) - n + 1):
                counter[(" ".join(toks[i:i + n]), n)] += 1
    conn.execute("DELETE FROM ngram")
    conn.executemany("INSERT INTO ngram (gram, n, count) VALUES (?,?,?)",
                     ((g, n, c) for (g, n), c in counter.items() if c >= 2))
    return sum(1 for c in counter.values() if c >= 2)


def _count(conn: sqlite3.Connection, table: str) -> int:
    return conn.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0]


def refresh_meta(conn: sqlite3.Connection) -> dict:
    """Recompute the sentence-derived stats; leave the rest as the build wrote them."""
    row = conn.execute("SELECT value FROM pack_meta WHERE key='stats'").fetchone()
    stats = json.loads(row[0]) if row else {}
    stats.update({
        "sentences": _count(conn, "sentence"),
        "sentence_trans": _count(conn, "sentence_trans"),
        "sentence_tokens": _count(conn, "sentence_token"),
        "ngrams": _count(conn, "ngram"),
    })
    conn.execute("INSERT OR REPLACE INTO pack_meta (key, value) VALUES ('stats', ?)",
                 (json.dumps(stats),))
    return stats


def check(conn: sqlite3.Connection) -> list[str]:
    """Return problems; empty means the pack is consistent."""
    problems: list[str] = []
    ic = conn.execute("PRAGMA integrity_check").fetchall()
    if ic != [("ok",)]:
        problems.append(f"integrity_check: {ic[:5]}")
    fk = conn.execute("PRAGMA foreign_key_check").fetchall()
    if fk:
        problems.append(f"foreign_key_check: {len(fk)} violations, first {fk[:3]}")
    have = {r[0] for r in conn.execute("SELECT name FROM sqlite_master WHERE type='table'")}
    for t in REQUIRED_TABLES:
        if t not in have:
            problems.append(f"missing table {t}")
    if problems:
        return problems
    srcs = [r[0] for r in conn.execute("SELECT DISTINCT source FROM sentence")]
    if srcs != [ALLOWED_SOURCE]:
        problems.append(f"sentence sources are {srcs}, want only ['{ALLOWED_SOURCE}']")
    for table, col in (("sentence_token", "sentence_id"), ("sentence_trans", "sentence_id"),
                       ("reading_set_sentence", "sentence_id")):
        n = conn.execute(
            f"SELECT COUNT(*) FROM {table} WHERE {col} NOT IN (SELECT id FROM sentence)"
        ).fetchone()[0]
        if n:
            problems.append(f"{n} dangling {table}.{col}")
    n = conn.execute("SELECT COUNT(*) FROM reading_set_sentence WHERE set_code NOT IN "
                     "(SELECT code FROM reading_set)").fetchone()[0]
    if n:
        problems.append(f"{n} reading_set_sentence rows name a missing set")
    return problems


def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def derive(full: Path, out_dir: Path) -> dict:
    out_dir.mkdir(parents=True, exist_ok=True)
    out = out_dir / PACK_NAME
    for p in (out, out.with_name(out.name + "-wal"), out.with_name(out.name + "-shm"),
              out.with_name(out.name + ".gz")):
        if p.exists():
            p.unlink()
    if full.resolve() == out.resolve():
        raise SystemExit("--out must not be the directory holding --full")
    shutil.copyfile(full, out)

    conn = sqlite3.connect(out)
    try:
        removed = strip_sentences(conn)
        rebuild_ngrams(conn)
        stats = refresh_meta(conn)
        conn.commit()
        conn.execute("PRAGMA journal_mode=DELETE")
        conn.execute("VACUUM")
        conn.execute("ANALYZE")
        conn.commit()
        problems = check(conn)
        if problems:
            raise SystemExit("release pack invalid:\n  " + "\n  ".join(problems))
        per_source = dict(conn.execute("SELECT source, COUNT(*) FROM sentence GROUP BY 1"))
        words = _count(conn, "wordform")
        lemmas = _count(conn, "lemma")
        sentences = _count(conn, "sentence")
        meta = {k: (json.loads(v) if k in ("stats", "sources") or k == "gloss_langs" else v)
                for k, v in conn.execute("SELECT key, value FROM pack_meta")}
    finally:
        conn.close()

    manifest = emit.write_manifest(out, meta)
    gz = out.with_suffix(out.suffix + ".gz")
    with open(out, "rb") as fin, gzip.GzipFile(gz, "wb", compresslevel=9, mtime=0) as fout:
        shutil.copyfileobj(fin, fout)
    return {
        "gz": gz, "gz_bytes": gz.stat().st_size, "gz_sha256": sha256(gz),
        "sqlite": out, "sqlite_bytes": out.stat().st_size, "manifest": manifest,
        "wordforms": words, "lemmas": lemmas, "sentences": sentences,
        "removed_sentences": removed, "per_source": per_source, "stats": stats,
    }


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Derive the publishable (Tatoeba-only) pack from a full pack")
    ap.add_argument("--full", type=Path, required=True)
    ap.add_argument("--out", type=Path, required=True, help="output directory")
    args = ap.parse_args()
    if not args.full.exists():
        print(f"no such pack: {args.full}", file=sys.stderr)
        return 1
    r = derive(args.full, args.out)
    print(f"removed non-tatoeba sentences: {r['removed_sentences']}")
    print(f"gz:       {r['gz']}")
    print(f"gz bytes: {r['gz_bytes']}")
    print(f"gz sha256: {r['gz_sha256']}")
    print(f"sqlite:   {r['sqlite']}  ({r['sqlite_bytes']} bytes)")
    print(f"lemmas={r['lemmas']} wordforms={r['wordforms']} sentences={r['sentences']}")
    print(f"sentences per source: {r['per_source']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
