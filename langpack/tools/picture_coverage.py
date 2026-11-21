#!/usr/bin/env python3
"""Offline helper (not shipped): how many of the most frequent Hungarian nouns
could get a picture?  Reads the pack READ-ONLY and writes a candidate TSV for a
human to curate into backend/internal/pictures/catalog.json.

Matching is a rough lower bound: an English gloss (sense text before any
parenthesis) equal to a picture name.  Names come from the shipped catalogue
(altEn) plus an optional --names TSV of "name<TAB>codepoint-hex" lines (e.g.
built from Noto's emoji annotations).  No emoji keyword data is bundled.

  python3 langpack/tools/picture_coverage.py [--pack data/langpack-hu-v1.sqlite]
        [--top 300] [--names names.tsv] [--out candidates.tsv]
"""
import argparse
import json
import re
import sqlite3
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def norm(s):
    return re.sub(r"\s+", " ", re.sub(r"\(.*?\)", "", s)).strip().lower()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--pack", default=str(ROOT / "data/langpack-hu-v1.sqlite"))
    ap.add_argument("--top", type=int, default=300)
    ap.add_argument("--names")
    ap.add_argument("--out", default="picture_candidates.tsv")
    a = ap.parse_args()

    cat = json.loads((ROOT / "backend/internal/pictures/catalog.json").read_text("utf8"))
    have = {e["lemma"] for e in cat["entries"]}
    names = {}  # english name -> picture file stem
    for e in cat["entries"]:
        names[norm(e["altEn"])] = e["pic"].removesuffix(".svg")
    if a.names:
        for line in Path(a.names).read_text("utf8").splitlines():
            if "\t" in line:
                n, cp = line.split("\t")[:2]
                names.setdefault(norm(n), "emoji_u" + cp.strip())

    db = sqlite3.connect(f"file:{a.pack}?mode=ro", uri=True)
    rows = db.execute(
        "SELECT id, lemma, freq_rank FROM lemma WHERE upos='NOUN' AND freq_rank IS NOT NULL "
        "ORDER BY freq_rank LIMIT ?", (a.top,)).fetchall()
    hit = 0
    with open(a.out, "w", encoding="utf8") as out:
        out.write("rank\tlemma\tstatus\tgloss\tpicture\n")
        for lid, lemma, rank in rows:
            glosses = [g for (g,) in db.execute(
                "SELECT gloss FROM lemma_gloss WHERE lemma_id=? AND lang='en' ORDER BY sense_idx", (lid,))]
            pic, status = "", "none"
            if lemma in have:
                status = "in-catalog"
            else:
                for g in glosses:
                    if norm(g) in names:
                        pic, status = names[norm(g)], "candidate"
                        break
            hit += status != "none"
            out.write(f"{rank}\t{lemma}\t{status}\t{'; '.join(glosses[:2])}\t{pic}\n")
    print(f"top {len(rows)} nouns: {hit} with a picture or candidate = {100 * hit / max(len(rows), 1):.1f}% "
          f"(rough lower bound); wrote {a.out}", file=sys.stderr)


if __name__ == "__main__":
    main()
