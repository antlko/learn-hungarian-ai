"""Build the language pack.

    python -m pipeline.build --out ../dist/langpack-hu-v1.sqlite

Stages: fetch -> lexicon -> forms -> sentences -> features -> curriculum ->
pairs -> ngrams -> emit. Each prints what it produced, because a pack that built
"successfully" with an empty table is the failure mode that matters.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from collections import Counter
from pathlib import Path

from . import curriculum, emit, features, frequency, minimal_pairs, sentences, sources
from .inflect import noun_forms, paradigm
from .wiktionary import Entry, FormOf
from .wiktionary import parse as parse_wiktionary

TOKEN_RE = re.compile(r"[A-Za-zÁÉÍÓÖŐÚÜŰáéíóöőúüű]+(?:-[A-Za-zÁÉÍÓÖŐÚÜŰáéíóöőúüű]+)*")


def tokenise(text: str) -> list[str]:
    return TOKEN_RE.findall(text)


def feats_to_str(feats: dict[str, str]) -> str:
    return "|".join(f"{k}={v}" for k, v in sorted(feats.items()))


def main() -> int:
    ap = argparse.ArgumentParser(description="Build the Hungarian language pack")
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--version", default="1")
    ap.add_argument("--record-checksums", action="store_true")
    ap.add_argument("--force-fetch", action="store_true")
    ap.add_argument(
        "--generate-forms-top",
        type=int,
        default=5000,
        help="generate inflected forms for the N most frequent noun lemmas "
             "(0 = none). Wiktionary already covers most nouns; generation fills "
             "gaps and supplies the low-confidence candidates the answer diff needs.",
    )
    args = ap.parse_args()

    print("→ fetching sources")
    paths = sources.fetch_all(force=args.force_fetch, record=args.record_checksums)

    stats: dict[str, object] = {}
    conn = emit.create(args.out)
    index_ddl = emit._split_indexes(conn)

    # ── features ─────────────────────────────────────────────────────────
    problems = features.validate()
    if problems:
        print("feature inventory is invalid:", *problems, sep="\n  ")
        return 1
    n = emit.bulk(
        conn, "morph_feature",
        ["id", "key", "value", "label_ru", "label_en", "allomorphs",
         "explain_ru", "explain_en", "teach_order", "prereq_ids", "grid"],
        (
            (f.id, f.key, f.value, f.label_ru, f.label_en, json.dumps(f.allomorphs, ensure_ascii=False),
             f.explain_ru, f.explain_en, f.teach_order, ",".join(f.prereq_ids), f.grid)
            for f in features.ALL_FEATURES
        ),
    )
    stats["features"] = n
    print(f"  features: {n}")

    # ── curriculum ───────────────────────────────────────────────────────
    problems = curriculum.validate(set(features.by_id()))
    if problems:
        print("curriculum is invalid:", *problems, sep="\n  ")
        return 1
    n = emit.bulk(
        conn, "grammar_topic",
        ["code", "level", "title_ru", "title_en", "explain_md_ru", "explain_md_en",
         "prereq_codes", "feature_ids", "teach_order"],
        (
            (t.code, t.level, t.title_ru, t.title_en, t.explain_ru, t.explain_en,
             ",".join(t.prereq_codes), ",".join(t.feature_ids), t.teach_order)
            for t in curriculum.TOPICS
        ),
    )
    stats["topics"] = n
    print(f"  topics: {n}")

    # ── minimal pairs ────────────────────────────────────────────────────
    problems = minimal_pairs.validate()
    if problems:
        print("minimal pairs are invalid:", *problems, sep="\n  ")
        return 1
    n = emit.bulk(
        conn, "minimal_pair",
        ["a", "b", "contrast", "priority", "gloss_a_ru", "gloss_b_ru",
         "gloss_a_en", "gloss_b_en", "context_a", "context_b"],
        (
            (p.a, p.b, p.contrast, p.priority, p.gloss_a_ru, p.gloss_b_ru,
             p.gloss_a_en, p.gloss_b_en, p.context_a, p.context_b)
            for p in minimal_pairs.PAIRS
        ),
    )
    stats["minimal_pairs"] = n
    stats["contrasts"] = len(minimal_pairs.contrasts())
    print(f"  minimal pairs: {n} across {stats['contrasts']} contrasts")

    # ── lexicon ──────────────────────────────────────────────────────────
    lemma_ids: dict[tuple[str, str], int] = {}
    gloss_langs: list[str] = []
    wik = paths.get("wiktionary")

    if wik and wik.exists():
        print("→ lexicon from Wiktionary (this takes a minute)")
        lemma_rows, gloss_rows, form_rows = [], [], []
        # Inflected-form pages are collected separately: they can only be
        # resolved once every headword has an id.
        pending_forms: list[FormOf] = []
        next_id = 1

        for x in parse_wiktionary(wik):
            if isinstance(x, FormOf):
                pending_forms.append(x)
                continue
            e: Entry = x
            key = (e.word, e.upos)
            if key in lemma_ids:
                continue
            lid = next_id
            next_id += 1
            lemma_ids[key] = lid
            lemma_rows.append((lid, e.word, e.upos, None, None, None, e.ipa))
            # The citation form is itself a wordform. Omitting it meant `van`,
            # `kell`, `a` and `az` — the most frequent tokens in the language —
            # had a lemma row but were unfindable by lookup, which is what put
            # token resolution at 48%.
            form_rows.append((e.word, lid, e.upos, "", "wiktionary", "high"))
            for i, g in enumerate(e.glosses_en[:5]):
                gloss_rows.append((lid, "en", i, g))
            for form, feats in e.forms:
                form_rows.append((form, lid, e.upos, feats_to_str(feats), "wiktionary", "high"))

        # Resolve inflected-form pages against the headword index.
        by_word: dict[str, int] = {}
        for (word, upos), lid in lemma_ids.items():
            by_word.setdefault(word, lid)
        resolved = 0
        for fo in pending_forms:
            lid = lemma_ids.get((fo.lemma, fo.upos)) or by_word.get(fo.lemma)
            if lid is None:
                continue  # target headword absent; the form teaches nothing alone
            form_rows.append(
                (fo.form, lid, fo.upos, feats_to_str(fo.feats), "wiktionary", "high"))
            resolved += 1
        stats["form_of_pages"] = len(pending_forms)
        stats["form_of_resolved"] = resolved

        stats["lemmas"] = emit.bulk(
            conn, "lemma", ["id", "lemma", "upos", "freq_rank", "freq_pm", "cefr", "ipa"], lemma_rows)
        stats["glosses"] = emit.bulk(
            conn, "lemma_gloss", ["lemma_id", "lang", "sense_idx", "gloss"], gloss_rows)
        stats["forms_wiktionary"] = emit.bulk(
            conn, "wordform", ["form", "lemma_id", "upos", "feats", "source", "confidence"], form_rows)
        gloss_langs.append("en")
        print(f"  lemmas: {stats['lemmas']}  glosses: {stats['glosses']}  "
              f"forms: {stats['forms_wiktionary']} "
              f"(incl. {resolved}/{len(pending_forms)} inflected-form pages)")
    else:
        print("  ! Wiktionary extract unavailable — building without lemmas/glosses")
        stats["lemmas"] = stats["glosses"] = stats["forms_wiktionary"] = 0

    conn.commit()

    # ── frequency ────────────────────────────────────────────────────────
    freq_path = paths.get("frequency")
    lemma_rank: dict[str, tuple[int, float]] = {}
    if freq_path and freq_path.exists():
        print("→ frequency ranks")
        form_counts = frequency.load(freq_path)
        form_to_lemmas: dict[str, set[str]] = {}
        for (form, lid) in conn.execute("SELECT form, lemma_id FROM wordform"):
            form_to_lemmas.setdefault(form.lower(), set())
        # Map both surface forms and lemmas so an uninflected lemma still ranks.
        for (form, lemma) in conn.execute(
            "SELECT w.form, l.lemma FROM wordform w JOIN lemma l ON l.id = w.lemma_id"
        ):
            form_to_lemmas.setdefault(form.lower(), set()).add(lemma)
        for (lemma,) in conn.execute("SELECT lemma FROM lemma"):
            form_to_lemmas.setdefault(lemma.lower(), set()).add(lemma)

        lemma_rank = frequency.rank_lemmas(form_counts, form_to_lemmas)
        conn.executemany(
            "UPDATE lemma SET freq_rank = ?, freq_pm = ?, cefr = ? WHERE lemma = ?",
            [
                (rank, pm, frequency.cefr_for_rank(rank), lemma)
                for lemma, (rank, pm) in lemma_rank.items()
            ],
        )
        conn.commit()
        stats["ranked_lemmas"] = len(lemma_rank)
        print(f"  ranked lemmas: {len(lemma_rank)}")

    # ── generated forms ──────────────────────────────────────────────────
    if args.generate_forms_top:
        print(f"→ generating forms for the top {args.generate_forms_top} noun lemmas")
        targets = conn.execute(
            "SELECT id, lemma FROM lemma WHERE upos = 'NOUN' AND freq_rank IS NOT NULL "
            "ORDER BY freq_rank LIMIT ?",
            (args.generate_forms_top,),
        ).fetchall()
        # A generated row that exactly duplicates an attested one adds nothing and
        # makes the lookup needlessly ambiguous. Skipping at insert time is O(n);
        # doing it afterwards in SQL is a correlated subquery over ~2M rows.
        target_ids = {lid for lid, _ in targets}
        attested: set[tuple[str, int, str]] = set()
        for form, lid, feats in conn.execute(
            "SELECT form, lemma_id, feats FROM wordform WHERE source = 'wiktionary'"
        ):
            if lid in target_ids:
                attested.add((form, lid, feats))

        gen_rows, para_rows = [], []
        skipped = 0
        for lid, lemma in targets:
            for f in noun_forms(lemma):
                if (f.form, lid, f.feats_key) in attested:
                    skipped += 1
                    continue
                gen_rows.append((f.form, lid, "NOUN", f.feats_key, "generated", f.confidence.value))
            for slot, form in paradigm(lemma).items():
                para_rows.append((lid, slot, form))
        stats["forms_generated"] = emit.bulk(
            conn, "wordform", ["form", "lemma_id", "upos", "feats", "source", "confidence"], gen_rows)
        stats["paradigm_cells"] = emit.bulk(
            conn, "paradigm", ["lemma_id", "slot", "form"], para_rows)
        conn.commit()
        print(f"  generated forms: {stats['forms_generated']}  "
              f"paradigm cells: {stats['paradigm_cells']}  "
              f"(skipped {skipped} duplicating attested rows)")

    # ── sentences ────────────────────────────────────────────────────────
    hu_path = paths.get("tatoeba_sentences")
    if hu_path and hu_path.exists():
        print("→ sentences")
        # A links file gives hu_id -> other_id; the text itself lives in the
        # other language's own export. Pairing links with hun_sentences (as an
        # earlier version did) silently produced zero translations.
        link_paths = {}
        for lang, links_key, text_key in (
            ("en", "tatoeba_links_eng", "tatoeba_eng"),
            ("ru", "tatoeba_links_rus", "tatoeba_rus"),
        ):
            lp, tp = paths.get(links_key), paths.get(text_key)
            if lp and lp.exists() and tp and tp.exists():
                link_paths[lang] = (lp, tp)
        if not link_paths:
            print("  ! no translation sources available; sentences will be untranslated")

        rank_by_form = {lemma.lower(): r for lemma, (r, _) in lemma_rank.items()}
        srows, trows, tokrows = [], [], []
        lemma_by_form: dict[str, int] = {}
        for form, lid in conn.execute(
            "SELECT form, lemma_id FROM wordform WHERE confidence != 'low'"
        ):
            lemma_by_form.setdefault(form.lower(), lid)

        count = 0
        for s in sentences.build(hu_path, link_paths):
            toks = tokenise(s.text)
            if not toks:
                continue
            srows.append((s.id, s.text, "tatoeba", "CC BY 2.0 FR", len(toks),
                          sentences.difficulty(s.text, rank_by_form)))
            for lang, text in s.translations.items():
                trows.append((s.id, lang, text))
            for i, tok in enumerate(toks):
                tokrows.append((s.id, i, tok, lemma_by_form.get(tok.lower()), ""))
            count += 1
            if count >= 60_000:
                break

        stats["sentences"] = emit.bulk(
            conn, "sentence", ["id", "hu", "source", "license", "len_tokens", "difficulty"], srows)
        stats["sentence_trans"] = emit.bulk(
            conn, "sentence_trans", ["sentence_id", "lang", "text"], trows)
        stats["sentence_tokens"] = emit.bulk(
            conn, "sentence_token", ["sentence_id", "idx", "form", "lemma_id", "feats"], tokrows)
        conn.commit()
        print(f"  sentences: {stats['sentences']}  tokens: {stats['sentence_tokens']}")

        # ── n-grams for the attestation guardrail ─────────────────────────
        print("→ n-gram attestation index")
        counter: Counter[tuple[str, int]] = Counter()
        for (text,) in conn.execute("SELECT hu FROM sentence"):
            toks = [t.lower() for t in tokenise(text)]
            for n_ in (2, 3):
                for i in range(len(toks) - n_ + 1):
                    counter[(" ".join(toks[i : i + n_]), n_)] += 1
        stats["ngrams"] = emit.bulk(
            conn, "ngram", ["gram", "n", "count"],
            ((g, n_, c) for (g, n_), c in counter.items() if c >= 2))
        conn.commit()
        print(f"  n-grams (count >= 2): {stats['ngrams']}")

    # ── finalise ─────────────────────────────────────────────────────────
    print("→ indexing and vacuuming")
    meta = {
        "version": args.version,
        "schema_version": str(emit.SCHEMA_VERSION),
        "gloss_langs": gloss_langs or ["en"],
        "stats": stats,
        "sources": [
            {"key": s.key, "licence": s.licence, "attribution": s.attribution,
             "present": bool(paths.get(s.key))}
            for s in sources.SOURCES
        ],
        "attribution": "See langpack/LICENSES.md. Pack data is CC BY-SA 4.0.",
    }
    emit.finalise(conn, index_ddl, meta)
    conn.close()

    manifest = emit.write_manifest(args.out, meta)
    size_mb = args.out.stat().st_size / (1 << 20)
    print(f"\n✓ {args.out}  ({size_mb:.1f} MB)")
    print(f"  manifest: {manifest.name}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
