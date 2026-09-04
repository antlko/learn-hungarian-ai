"""Add conversational sentences to an existing pack, without rebuilding it.

`make langpack` is hours: Wiktionary, Tatoeba, HuSpaCy over every sentence. The
sentence stage of that build does NOT use HuSpaCy — look at `build.py`, it links
each token through the pack's own `wordform` table — so adding sentences to a
pack that already has a lexicon is minutes of work, not hours, and there is no
reason to pay the full price for it.

    make subtitles      # python -m pipeline.add_subtitles --pack ...

What it does:

  1. reads the frequency ranks and the form -> lemma index out of the pack;
  2. runs the two-pass selection in `subtitles.py` over the OPUS archives, or
     reads the harvested pool from `cache/subtitle-pool.jsonl` when it is there
     — the scan is 47 minutes and the choice is arithmetic, so retuning the
     selection must not cost the scan again;
  3. inserts the survivors as `source='opensubtitles'` rows, with their
     translations and their tokens;
  4. rebuilds the n-gram attestation index, which now has more to attest.

Idempotent: every `opensubtitles` row is deleted first, so running it twice
gives the same pack rather than two copies of everything. Tatoeba rows are never
touched.

**Stop the server first.** The app treats a pack as read-only; this is a build
step writing to one.

Afterwards `make starter` — the starter pack deliberately excludes these
sentences (see `starter.py`), but its lemma selection reads the full pack and
should be regenerated from the same artefact it will ship beside.
"""

from __future__ import annotations

import argparse
import json
import re
import sqlite3
import sys
import time
from collections import Counter
from pathlib import Path

from . import sentences as sentences_mod
from . import subtitles

SOURCE = "opensubtitles"
LICENCE = "OPUS OpenSubtitles (Lison & Tiedemann 2016); see langpack/LICENSES.md"

_WORD = re.compile(r"[^\W\d_]+(?:[-'][^\W\d_]+)*", re.UNICODE)


def tokenise(text: str) -> list[str]:
    return _WORD.findall(text)


def _frequency_ranks(conn: sqlite3.Connection) -> dict[str, int]:
    """Form -> rank, from the pack itself.

    The selection scores a sentence by how much of it a learner is likely to
    know, and the pack's own frequency ranks are the measure it already uses
    everywhere else. Reading them from the pack rather than from the raw
    frequency file keeps the two consistent when one is rebuilt.
    """
    ranks: dict[str, int] = {}
    for form, rank in conn.execute(
        """SELECT w.form, l.freq_rank FROM wordform w JOIN lemma l ON l.id = w.lemma_id
            WHERE l.freq_rank IS NOT NULL"""
    ):
        f = form.lower()
        cur = ranks.get(f)
        if cur is None or rank < cur:
            ranks[f] = rank
    return ranks


def _lemma_by_form(conn: sqlite3.Connection) -> dict[str, int]:
    out: dict[str, int] = {}
    for form, lid in conn.execute(
        "SELECT form, lemma_id FROM wordform WHERE confidence != 'low'"
    ):
        out.setdefault(form.lower(), lid)
    return out


def run(pack: Path, archives: dict[str, Path], limit: int, cache: Path | None = None) -> dict:
    if not pack.exists():
        raise SystemExit(f"no such pack: {pack}")
    missing = {k: p for k, p in archives.items() if not p.exists()}
    if missing:
        raise SystemExit(
            "missing OPUS archives: "
            + ", ".join(str(p) for p in missing.values())
            + "\n  fetch them with: python -m pipeline.sources --only opensubtitles_ru,opensubtitles_en"
        )

    conn = sqlite3.connect(pack)
    # WAL for the write — this inserts half a million rows and rebuilds an index
    # — but the pack MUST end as a single file. It is copied, gzipped into the
    # binary and shipped; a `-wal` sidecar left beside it is content that goes
    # missing the first time someone moves the `.sqlite` on its own. Restored to
    # `delete` in the `finally` below, which checkpoints and removes them.
    conn.execute("PRAGMA journal_mode=WAL")
    try:
        ranks = _frequency_ranks(conn)
        lemma_by_form = _lemma_by_form(conn)
        print(f"  pack: {len(ranks)} ranked forms, {len(lemma_by_form)} form->lemma entries",
              flush=True)

        t0 = time.time()
        picked, stats = subtitles.select(archives, ranks, limit, cache=cache)
        stats["select_seconds"] = round(time.time() - t0)
        print(f"  selection: {json.dumps(stats)}", flush=True)

        with conn:
            # Idempotent: this command owns every `opensubtitles` row and no
            # other. Re-running must replace them, not duplicate them.
            old = [r[0] for r in conn.execute(
                "SELECT id FROM sentence WHERE source = ?", (SOURCE,))]
            if old:
                conn.execute("DELETE FROM sentence_token WHERE sentence_id IN "
                             "(SELECT id FROM sentence WHERE source = ?)", (SOURCE,))
                conn.execute("DELETE FROM sentence_trans WHERE sentence_id IN "
                             "(SELECT id FROM sentence WHERE source = ?)", (SOURCE,))
                conn.execute("DELETE FROM sentence WHERE source = ?", (SOURCE,))
                print(f"  replaced {len(old)} rows from a previous run", flush=True)

            next_id = (conn.execute("SELECT COALESCE(MAX(id), 0) FROM sentence")
                       .fetchone()[0]) + 1
            have = {r[0] for r in conn.execute("SELECT hu FROM sentence")}

            srows, trows, tokrows = [], [], []
            for sel in picked:
                if sel.hu in have:
                    continue
                have.add(sel.hu)
                toks = tokenise(sel.hu)
                if not toks:
                    continue
                srows.append((next_id, sel.hu, SOURCE, LICENCE, len(toks),
                              sentences_mod.difficulty(sel.hu, ranks)))
                for lang, text in (("ru", sel.ru), ("en", sel.en)):
                    if text:
                        trows.append((next_id, lang, text))
                for i, tok in enumerate(toks):
                    tokrows.append((next_id, i, tok, lemma_by_form.get(tok.lower()), ""))
                next_id += 1

            conn.executemany(
                "INSERT INTO sentence (id, hu, source, license, len_tokens, difficulty) "
                "VALUES (?,?,?,?,?,?)", srows)
            conn.executemany(
                "INSERT INTO sentence_trans (sentence_id, lang, text) VALUES (?,?,?)", trows)
            conn.executemany(
                "INSERT INTO sentence_token (sentence_id, idx, form, lemma_id, feats) "
                "VALUES (?,?,?,?,?)", tokrows)

            stats["inserted_sentences"] = len(srows)
            stats["inserted_translations"] = len(trows)
            stats["inserted_tokens"] = len(tokrows)

            # The attestation index covers the whole corpus, so it is rebuilt
            # rather than appended to: the counts are what make it a guardrail,
            # and a stale count is a wrong one.
            print("  rebuilding the n-gram attestation index", flush=True)
            counter: Counter[tuple[str, int]] = Counter()
            for (text,) in conn.execute("SELECT hu FROM sentence"):
                t = [x.lower() for x in tokenise(text)]
                for n in (2, 3):
                    for i in range(len(t) - n + 1):
                        counter[(" ".join(t[i:i + n]), n)] += 1
            conn.execute("DELETE FROM ngram")
            conn.executemany(
                "INSERT INTO ngram (gram, n, count) VALUES (?,?,?)",
                ((g, n, c) for (g, n), c in counter.items() if c >= 2))
            stats["ngrams"] = sum(1 for c in counter.values() if c >= 2)
    finally:
        # Fold the WAL back into the file and leave no sidecars behind.
        try:
            conn.execute("PRAGMA wal_checkpoint(TRUNCATE)")
            conn.execute("PRAGMA journal_mode=DELETE")
        finally:
            conn.close()
    return stats


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Add conversational sentences from OPUS OpenSubtitles to an existing pack")
    ap.add_argument("--pack", type=Path, required=True)
    ap.add_argument("--cache", type=Path, default=Path(__file__).resolve().parents[1] / "cache")
    ap.add_argument("--limit", type=int, default=120_000)
    args = ap.parse_args()

    archives = {
        "ru": args.cache / "OpenSubtitles-hu-ru.txt.zip",
        "en": args.cache / "OpenSubtitles-en-hu.txt.zip",
    }
    stats = run(args.pack, archives, args.limit, cache=args.cache / "subtitle-pool.jsonl")
    print("\n  " + json.dumps(stats, indent=2).replace("\n", "\n  "))
    print("\n  now run `make starter` and commit\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
