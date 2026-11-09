"""Reading sets: the ready-made «Живые примеры» the starter ships.

Read the STARTER (the artefact a fresh install gets). Skips, rather than passing
vacuously, if the file is absent.
"""

import gzip
import shutil
import sqlite3
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from pipeline import course, reading_sets

STARTER_GZ = (Path(__file__).resolve().parents[2]
              / "backend/internal/lang/starter/langpack-hu-starter.sqlite.gz")


@pytest.fixture(scope="module")
def starter_path(tmp_path_factory):
    if not STARTER_GZ.exists():
        pytest.skip("starter pack not present")
    out = tmp_path_factory.mktemp("starter") / "starter.sqlite"
    with gzip.open(STARTER_GZ, "rb") as fin, open(out, "wb") as fout:
        shutil.copyfileobj(fin, fout)
    return out


@pytest.fixture(scope="module")
def conn(starter_path):
    c = sqlite3.connect(f"file:{starter_path}?mode=ro", uri=True)
    yield c
    c.close()


def _sets(conn):
    return {
        code: (level, track)
        for code, level, track in conn.execute("SELECT code, level, track FROM reading_set")
    }


def test_one_set_per_course_unit(conn):
    # The basic tier and the themes get a set; the intermediate and advanced
    # tiers do not (ADR-057: MONTH_BAND stops at month 6).
    units = {u["code"] for u in course.UNITS if u["tier"] == "basic"}
    assert set(_sets(conn)) == units
    assert len(_sets(conn)) == 36


def test_core_levels_follow_the_month(conn):
    for u in course.UNITS:
        if u["tier"] != "basic":
            continue
        if u["track"] == "core":
            assert _sets(conn)[u["code"]][0] == reading_sets.MONTH_BAND[u["month"]]
        else:
            assert _sets(conn)[u["code"]][0] in ("A1", "A2")


def test_titles_name_the_unit_in_both_languages(conn):
    by_code = {u["code"]: u for u in course.UNITS}
    for code, ru, en in conn.execute("SELECT code, title_ru, title_en FROM reading_set"):
        assert ru == f"Живые примеры: {by_code[code]['title_ru']}"
        assert en == f"Real examples: {by_code[code]['title_en']}"
        assert "корпус" not in ru.lower()


def test_validator_is_clean(conn):
    assert reading_sets.validate(conn) == []


def test_every_set_is_within_the_size_bounds(conn):
    for code, n in conn.execute(
        "SELECT set_code, COUNT(*) FROM reading_set_sentence GROUP BY 1"
    ):
        assert reading_sets.MIN_SENTENCES <= n <= reading_sets.MAX_SENTENCES, code


def test_no_set_is_empty(conn):
    for (code,) in conn.execute("SELECT code FROM reading_set"):
        n = conn.execute(
            "SELECT COUNT(*) FROM reading_set_sentence WHERE set_code=?", (code,)
        ).fetchone()[0]
        assert n > 0, code


def test_a_sentence_is_in_at_most_one_set(conn):
    dup = conn.execute(
        "SELECT sentence_id FROM reading_set_sentence GROUP BY 1 HAVING COUNT(*) > 1"
    ).fetchall()
    assert dup == []


def test_every_sentence_exists_and_is_tatoeba(conn):
    bad = conn.execute(
        "SELECT r.sentence_id FROM reading_set_sentence r "
        "LEFT JOIN sentence s ON s.id = r.sentence_id "
        "WHERE s.id IS NULL OR s.source <> 'tatoeba'"
    ).fetchall()
    assert bad == []


def test_all_content_lemmas_are_within_the_sets_band(conn):
    cefr = {i: c for i, c in conn.execute("SELECT id, cefr FROM lemma")}
    upos = {i: u for i, u in conn.execute("SELECT id, upos FROM lemma")}
    for code, level, _ in conn.execute("SELECT code, level, track FROM reading_set"):
        limit = reading_sets.BANDS.index(level)
        for sid, lid in conn.execute(
            "SELECT t.sentence_id, t.lemma_id FROM sentence_token t "
            "JOIN reading_set_sentence r ON r.sentence_id = t.sentence_id "
            "WHERE r.set_code = ?", (code,)
        ):
            assert lid is not None, (code, sid)
            if upos[lid] in reading_sets.CONTENT_UPOS:
                assert reading_sets._rank(cefr[lid]) <= limit, (code, sid, lid)


def test_every_sentence_carries_a_word_of_its_unit(conn):
    for u in course.UNITS:
        words = [w[0] for w in u["words"]]
        hits = reading_sets._unit_hits(conn, words)
        ids = [r[0] for r in conn.execute(
            "SELECT sentence_id FROM reading_set_sentence WHERE set_code=?", (u["code"],))]
        assert all(i in hits for i in ids), u["code"]


def test_sentences_are_within_the_length_window(conn):
    # The STARTER keeps the shortest sentences of the corpus (3-4 tokens), so
    # the 5-12 preference can only bite on a full pack; the window is the floor.
    lo, hi = conn.execute(
        "SELECT MIN(s.len_tokens), MAX(s.len_tokens) FROM reading_set_sentence r "
        "JOIN sentence s ON s.id = r.sentence_id").fetchone()
    assert reading_sets.WIDE_MIN <= lo and hi <= reading_sets.WIDE_MAX


def test_selection_is_deterministic(starter_path, tmp_path):
    a = tmp_path / "a.sqlite"
    shutil.copy(starter_path, a)
    c = sqlite3.connect(a)
    before = c.execute(
        "SELECT set_code, sentence_id, ord FROM reading_set_sentence ORDER BY 1,3").fetchall()
    reading_sets.build(c)
    after = c.execute(
        "SELECT set_code, sentence_id, ord FROM reading_set_sentence ORDER BY 1,3").fetchall()
    c.close()
    assert before == after


def _legacy_unit_hits(conn, words):
    """The original per-unit full scan of sentence_token, kept as the oracle."""
    ph = ",".join("?" * len(words))
    lemma_ids = [r[0] for r in conn.execute(f"SELECT id FROM lemma WHERE lemma IN ({ph})", words)]
    forms = {w.lower() for w in words}
    if lemma_ids:
        lph = ",".join("?" * len(lemma_ids))
        forms |= {r[0].lower() for r in conn.execute(
            f"SELECT form FROM wordform WHERE lemma_id IN ({lph})", lemma_ids)}
    lset = set(lemma_ids)
    hit = {}
    for sid, form, lid in conn.execute("SELECT sentence_id, form, lemma_id FROM sentence_token"):
        if lid in lset or form.lower() in forms:
            hit[sid] = hit.get(sid, 0) + 1
    return hit


def _legacy_eligible(conn, band, lo, hi):
    limit = reading_sets._rank(band)
    ok = {r[0] for r in conn.execute("SELECT id, cefr FROM lemma") if reading_sets._rank(r[1]) <= limit}
    content = {r[0] for r in conn.execute(
        "SELECT id FROM lemma WHERE upos IN (%s)" % ",".join("?" * len(reading_sets.CONTENT_UPOS)),
        reading_sets.CONTENT_UPOS)}
    bad = {sid for sid, lid in conn.execute("SELECT sentence_id, lemma_id FROM sentence_token")
           if lid is None or (lid in content and lid not in ok)}
    tr = {}
    for sid, lang in conn.execute("SELECT sentence_id, lang FROM sentence_trans"):
        tr.setdefault(sid, set()).add(lang)
    return {sid: (n, "ru" in tr.get(sid, ()), "en" in tr.get(sid, ()))
            for sid, n in conn.execute(
                "SELECT id, len_tokens FROM sentence WHERE source = 'tatoeba' "
                "AND len_tokens BETWEEN ? AND ?", (lo, hi)) if sid not in bad}


def test_indexed_selection_matches_the_per_query_scans(conn):
    """The one-pass index is a speed-up only: same hits, same eligible sentences."""
    ix = reading_sets._Index(conn)
    for u in course.UNITS[::5]:
        words = [w[0] for w in u["words"]]
        assert reading_sets._unit_hits(conn, words, ix) == _legacy_unit_hits(conn, words), u["code"]
    for band in ("A1", "A2", "B1"):
        for lo, hi in ((reading_sets.SHORT_MIN, reading_sets.SHORT_MAX),
                       (reading_sets.WIDE_MIN, reading_sets.WIDE_MAX)):
            assert reading_sets._eligible(conn, band, lo, hi, ix) == _legacy_eligible(conn, band, lo, hi)


def test_plan_reproduces_the_committed_sets(conn):
    """The sets the starter ships are exactly what plan() chooses from its sentences."""
    rows, chosen = reading_sets.plan(conn)
    stored = {}
    for code, sid in conn.execute(
            "SELECT set_code, sentence_id FROM reading_set_sentence ORDER BY set_code, ord"):
        stored.setdefault(code, []).append(sid)
    assert chosen == stored
