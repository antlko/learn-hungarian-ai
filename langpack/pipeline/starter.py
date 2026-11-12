"""Build the STARTER language pack: the one baked into the binary.

The full pack is a quarter of a gigabyte of Wiktionary and Tatoeba. That is the
right artefact to *download*, and the wrong one to ship inside an executable —
but an app that boots into "no language pack is installed" is not a product, it
is a build step wearing a UI. So the binary embeds a compact pack that is enough
to actually learn from on day one, and the full pack is an upgrade.

What "enough" means here, concretely:

  * every morph_feature, grammar_topic, minimal_pair and course unit — the
    curriculum is small and is the whole spine of the app;
  * the top-N lemmas by corpus frequency plus every lemma the course names, with
    their glosses, their attested wordforms and their display paradigm;
  * only sentences whose every word resolves inside that lemma set, so the
    reader and the drill generator never hand the learner a sentence the starter
    cannot explain;
  * the n-gram attestation index in full — it is tiny and it is a guardrail.

Everything omitted is omitted because its absence DEGRADES rather than breaks:
a rarer word looks up as unknown, which the UI already renders honestly.

    python -m pipeline.starter --full ../data/langpack-hu-v1.sqlite \
        --out ../backend/internal/lang/starter/langpack-hu-starter.sqlite
"""

from __future__ import annotations

import argparse
import gzip
import json
import re
import shutil
import sqlite3
from pathlib import Path

from . import reading_sets

# Tuned against the real pack: 4000 lemmas covers the frequency band the first
# six months of the course lives in, and keeps the gzipped artefact small enough
# that nobody thinks twice about it being in the repo.
DEFAULT_LEMMAS = 4000
DEFAULT_SENTENCES = 12000

TABLES_COPIED_WHOLE = (
    "pack_meta",
    "morph_feature",
    "grammar_topic",
    "minimal_pair",
    "pair_audio",
    "ngram",
)


def _keep_story_names(dst: sqlite3.Connection) -> None:
    """Keep the proper-noun lemmas the authored texts name (Kata, Dani, ...).

    The frequency band drops first names, but the stories and dialogues are
    checked against the STARTER's wordforms (`reading_sets.validate`), so a name
    the text uses must be attested there too."""
    words: set[str] = set()
    for (hu,) in dst.execute("SELECT hu FROM full.sentence WHERE source = 'authored'"):
        for tok in re.findall(r"[^\W\d_]+", hu):
            if tok[:1].isupper():
                words.add(tok)
    for tok in sorted(words):
        dst.execute(
            """
            INSERT INTO keep_lemma
            SELECT DISTINCT l.id FROM full.wordform w JOIN full.lemma l ON l.id = w.lemma_id
             WHERE w.form = ? AND l.upos = 'PROPN'
               AND l.id NOT IN (SELECT id FROM keep_lemma)
            """,
            (tok,),
        )


def build(full: Path, out: Path, *, lemmas: int, sentences: int) -> None:
    if out.exists():
        out.unlink()
    for suffix in ("-wal", "-shm"):
        p = out.with_name(out.name + suffix)
        if p.exists():
            p.unlink()

    src = sqlite3.connect(f"file:{full}?mode=ro", uri=True)
    schema = [
        row[0]
        for row in src.execute(
            "SELECT sql FROM sqlite_master WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%'"
        )
    ]

    dst = sqlite3.connect(out)
    dst.executescript("PRAGMA journal_mode=OFF; PRAGMA synchronous=OFF;")
    for stmt in schema:
        dst.execute(stmt)

    src.close()
    dst.execute("ATTACH DATABASE ? AS full", (str(full),))

    # course_unit may not exist in an older full pack; copy what is there.
    have = {
        r[0]
        for r in dst.execute("SELECT name FROM full.sqlite_master WHERE type='table'")
    }
    whole = [
        t
        for t in (*TABLES_COPIED_WHOLE, "course_unit", "course_word", "resource")
        if t in have
    ]

    for table in whole:
        dst.execute(f"INSERT INTO {table} SELECT * FROM full.{table}")

    # ── the lemma set ────────────────────────────────────────────────────
    # Frequency band first, then anything the course explicitly names: a course
    # word outside the band must still be learnable, or the course is a lie.
    dst.execute(
        """
        CREATE TEMP TABLE keep_lemma AS
        SELECT id FROM full.lemma
         WHERE freq_rank IS NOT NULL AND freq_rank <= ?
        """,
        (lemmas,),
    )
    if "course_word" in have:
        dst.execute(
            """
            INSERT INTO keep_lemma
            SELECT DISTINCT l.id FROM full.lemma l
              JOIN full.course_word cw ON cw.lemma = l.lemma
             WHERE l.id NOT IN (SELECT id FROM keep_lemma)
            """
        )
    # Course words come FIRST, and this is not a nicety. Enrolling a unit turns
    # each of its words into a cloze card built from an attested sentence, and a
    # word with no sentence in the pack is honestly skipped — so a starter that
    # picks its sentences purely by length quietly teaches a short version of
    # every unit.
    #
    # The subtlety is that a course word's only sentences often contain a rarer
    # word that the frequency band excluded, which would disqualify the whole
    # sentence under the "fully resolvable" rule below. Dropping the sentence
    # loses the course word; keeping it unresolvable breaks the rule. So we do
    # neither: take the sentences, then PULL THEIR LEMMAS IN. The lemma set
    # grows by the handful of words the syllabus implies, which is exactly the
    # set a learner of that syllabus will meet.
    if "course_word" in have:
        dst.execute(
            """
            CREATE TEMP TABLE course_sentence AS
            SELECT DISTINCT id FROM (
                SELECT s.id AS id,
                       ROW_NUMBER() OVER (PARTITION BY cw.lemma ORDER BY s.len_tokens) AS rn
                  FROM full.sentence s
                  JOIN full.sentence_token t ON t.sentence_id = s.id
                  JOIN full.lemma l          ON l.id = t.lemma_id
                  JOIN full.course_word cw   ON cw.lemma = l.lemma
                 WHERE s.len_tokens BETWEEN 3 AND 14
                   AND s.source = 'tatoeba'  -- a pack with OPUS rows must not crowd these out
                 GROUP BY s.id, cw.lemma
            )
            WHERE rn <= 3
            """
        )
        dst.execute("CREATE UNIQUE INDEX temp.idx_cs ON course_sentence(id)")
        dst.execute(
            """
            INSERT INTO keep_lemma
            SELECT DISTINCT t.lemma_id FROM full.sentence_token t
             WHERE t.sentence_id IN (SELECT id FROM course_sentence)
               AND t.lemma_id IS NOT NULL
               AND t.lemma_id NOT IN (SELECT id FROM keep_lemma)
            """
        )
        _keep_story_names(dst)
    else:
        dst.execute("CREATE TEMP TABLE course_sentence (id INTEGER)")

    dst.execute("CREATE TEMP TABLE keep_sentence AS SELECT id FROM course_sentence")
    # REDISTRIBUTION BOUNDARY. The starter pack is compiled into a binary we
    # publish; the full pack is built on the learner's own machine. Tatoeba is
    # CC BY 2.0 FR and travels freely. OPUS OpenSubtitles is free for research
    # and universally used, but it is not a licence to embed film dialogue in a
    # distributed executable, so it stays out of this artefact.
    #
    # `source = 'tatoeba'` rather than `source != 'opensubtitles'`: an allow-list
    # keeps a future source out by default, which is the safe direction for a
    # test nobody will remember to update.
    #
    # This is the CORPUS copy and stays Tatoeba-only. The authored stories and
    # dialogues (pipeline/stories.py, dialogues.py, source = 'authored', written for tanulom, CC BY-SA 4.0) are
    # not copied from the full pack: `reading_sets.build(dst)` below writes them
    # into the starter itself, after checking them against the starter's own
    # wordforms.
    dst.execute(
        """
        INSERT INTO keep_sentence
        SELECT s.id FROM full.sentence s
         WHERE s.id NOT IN (SELECT id FROM keep_sentence)
           AND s.source = 'tatoeba'
           AND NOT EXISTS (
                 SELECT 1 FROM full.sentence_token t
                  WHERE t.sentence_id = s.id
                    AND (t.lemma_id IS NULL OR t.lemma_id NOT IN (SELECT id FROM keep_lemma)))
         ORDER BY s.len_tokens
         LIMIT MAX(0, ? - (SELECT COUNT(*) FROM keep_sentence))
        """,
        (sentences,),
    )
    # The course-sentence set above is chosen before this filter, so drop any
    # non-Tatoeba sentence that arrived through it.
    dst.execute(
        "DELETE FROM keep_sentence WHERE id IN "
        "(SELECT id FROM full.sentence WHERE source <> 'tatoeba')"
    )
    dst.execute("CREATE INDEX temp.idx_keep ON keep_lemma(id)")

    dst.execute("INSERT INTO lemma SELECT * FROM full.lemma WHERE id IN (SELECT id FROM keep_lemma)")
    dst.execute(
        "INSERT INTO lemma_gloss SELECT * FROM full.lemma_gloss "
        "WHERE lemma_id IN (SELECT id FROM keep_lemma)"
    )
    dst.execute(
        "INSERT INTO wordform SELECT * FROM full.wordform "
        "WHERE lemma_id IN (SELECT id FROM keep_lemma)"
    )
    dst.execute(
        "INSERT INTO paradigm SELECT * FROM full.paradigm "
        "WHERE lemma_id IN (SELECT id FROM keep_lemma)"
    )

    # ── sentences the starter can fully explain ──────────────────────────
    # A sentence is kept only when EVERY word token resolves to a kept lemma.
    # Half-known sentences are exactly what the n+1 scorer is supposed to grade
    # down, and shipping them unresolvable would poison the coverage numbers.
    dst.execute("CREATE INDEX temp.idx_keep_s ON keep_sentence(id)")

    dst.execute("INSERT INTO sentence SELECT * FROM full.sentence WHERE id IN (SELECT id FROM keep_sentence)")
    dst.execute(
        "INSERT INTO sentence_token SELECT * FROM full.sentence_token "
        "WHERE sentence_id IN (SELECT id FROM keep_sentence)"
    )
    if "sentence_trans" in have:
        dst.execute(
            "INSERT INTO sentence_trans SELECT * FROM full.sentence_trans "
            "WHERE sentence_id IN (SELECT id FROM keep_sentence)"
        )

    # Reading sets are chosen from what the STARTER holds, not copied from the
    # full pack: a full-pack set can name a sentence this file does not carry.
    # Same stage, same rules, so the two editions' sets are each self-contained.
    if "course_unit" in have and "course_word" in have:
        reading_sets.build(dst)
        # require_all: a release may not ship a unit without its «Рассказ» (ADR-074).
        problems = reading_sets.validate(dst, require_all=True)
        if problems:
            raise SystemExit("reading sets invalid in the starter:\n  " + "\n  ".join(problems))

    # The starter says what it is, so the UI can offer the upgrade honestly.
    dst.execute(
        "INSERT OR REPLACE INTO pack_meta (key, value) VALUES ('edition', 'starter')"
    )
    # pack_meta is copied whole from the full pack, so gloss_langs arrives
    # describing the FULL pack's glosses. Recompute it from the rows that
    # actually made it in: the Go side offers the gloss-language setting off
    # this field, and a value the pack cannot back is a setting that silently
    # does nothing.
    langs = [
        lang
        for (lang,) in dst.execute(
            "SELECT DISTINCT lang FROM lemma_gloss ORDER BY lang = 'en' DESC, lang"
        )
    ]
    dst.execute(
        "INSERT OR REPLACE INTO pack_meta (key, value) VALUES ('gloss_langs', ?)",
        (json.dumps(langs),),
    )
    dst.commit()
    dst.execute("DETACH DATABASE full")
    dst.execute("VACUUM")
    dst.commit()

    have_out = {r[0] for r in dst.execute("SELECT name FROM sqlite_master WHERE type='table'")}
    counts = {
        t: dst.execute(f"SELECT COUNT(*) FROM {t}").fetchone()[0]
        for t in (
            "lemma", "wordform", "sentence", "sentence_token", "minimal_pair",
            "ngram", "course_unit", "course_word", "resource",
            "reading_set", "reading_set_sentence",
        )
        if t in have_out
    }
    dst.close()

    gz = out.with_suffix(out.suffix + ".gz")
    with open(out, "rb") as fin, gzip.open(gz, "wb", compresslevel=9) as fout:
        shutil.copyfileobj(fin, fout)
    out.unlink()

    print("  " + "  ".join(f"{k}={v}" for k, v in counts.items()))
    print(f"✓ {gz}  ({gz.stat().st_size / 1e6:.1f} MB gzipped)")


def main() -> int:
    ap = argparse.ArgumentParser(description="Build the embedded starter language pack")
    ap.add_argument("--full", type=Path, required=True)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--lemmas", type=int, default=DEFAULT_LEMMAS)
    ap.add_argument("--sentences", type=int, default=DEFAULT_SENTENCES)
    args = ap.parse_args()
    args.out.parent.mkdir(parents=True, exist_ok=True)
    build(args.full, args.out, lemmas=args.lemmas, sentences=args.sentences)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
