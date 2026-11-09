"""The guided course: size, tracks, the gates every word must pass.

The course is data, and data fails quietly: a lemma the pack cannot look up, a
word taught twice, a unit that quietly shrank, a gloss with a Latin letter typed
into a Russian word. Each of those has happened or nearly did, so each has a
check here.

Checks that need real sentences read the STARTER pack (the one compiled into the
binary), because that is the artefact a fresh install gets. They skip if the
starter file is absent rather than pass vacuously.
"""

import gzip
import re
import shutil
import sqlite3
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from pipeline import course, course_growth, course_themes, curriculum, refresh

STARTER_GZ = (Path(__file__).resolve().parents[2]
              / "backend/internal/lang/starter/langpack-hu-starter.sqlite.gz")


@pytest.fixture(scope="module")
def starter(tmp_path_factory):
    if not STARTER_GZ.exists():
        pytest.skip("starter pack not present")
    out = tmp_path_factory.mktemp("starter") / "starter.sqlite"
    with gzip.open(STARTER_GZ, "rb") as fin, open(out, "wb") as fout:
        shutil.copyfileobj(fin, fout)
    return out


@pytest.fixture(scope="module")
def starter_conn(starter):
    conn = sqlite3.connect(f"file:{starter}?mode=ro", uri=True)
    yield conn
    conn.close()


def _core():
    return [u for u in course.UNITS if u["track"] == "core"]


def _themes():
    return [u for u in course.UNITS if u["track"] == "theme"]


def _tier(name):
    return [u for u in _core() if u["tier"] == name]


def test_the_course_still_ends_at_three_finite_tiers():
    """24 basic + 20 intermediate + 20 advanced core units, then it ENDS (ADR-057)."""
    assert course.TIER_UNITS == {"basic": 24, "intermediate": 20, "advanced": 20}
    assert len(_core()) == course.CORE_UNITS == 64
    for tier, want in course.TIER_UNITS.items():
        assert len(_tier(tier)) == want, tier


def test_every_unit_has_a_known_tier_and_themes_are_basic():
    assert {u["tier"] for u in course.UNITS} == set(course.TIERS)
    assert all(u["tier"] == "basic" for u in _themes())
    # the month files never name a tier; the pipeline gives them 'basic'
    from pipeline import course_m1, course_m7_12, course_m13_18
    assert all("tier" not in u for u in course_m1.UNITS)
    assert {u["tier"] for u in course_m7_12.UNITS} == {"intermediate"}
    assert {u["tier"] for u in course_m13_18.UNITS} == {"advanced"}


def test_tiers_are_consecutive_in_ord_and_months_run_1_to_18():
    ranks = [course.TIERS.index(u["tier"]) for u in sorted(_core(), key=lambda u: u["ord"])]
    assert ranks == sorted(ranks)
    assert {u["month"] for u in _core()} == set(range(1, 19))
    assert {u["month"] for u in _tier("intermediate")} == set(range(7, 13))
    assert {u["month"] for u in _tier("advanced")} == set(range(13, 19))


def test_tier_sizes_are_unit_sized_and_the_basic_tier_is_unchanged():
    basic = sum(len(u["words"]) for u in _tier("basic"))
    assert 1300 <= basic <= 1500, basic
    for tier in ("intermediate", "advanced"):
        n = sum(len(u["words"]) for u in _tier(tier))
        assert n >= 600, (tier, n)
        for u in _tier(tier):
            assert course.MIN_WORDS <= len(u["words"]) <= course.MAX_WORDS, u["code"]


def test_new_tiers_lose_nothing_to_dedup():
    """_dedup keeps the first teaching of a word silently; the new tiers must
    already be disjoint from every earlier unit so no unit is quietly shorter."""
    new = {u["code"] for u in _core() if u["tier"] != "basic"}
    assert not [d for d in course.DUPLICATES if d[0] in new], \
        [d for d in course.DUPLICATES if d[0] in new][:10]


def test_twelve_theme_units_of_a_unit_sized_vocabulary():
    themes = _themes()
    assert len(themes) == 12
    for u in themes:
        assert course.MIN_WORDS <= len(u["words"]) <= course.MAX_WORDS, u["code"]
        assert u["track"] == "theme"


def test_themes_sort_after_the_whole_core_path():
    assert min(u["ord"] for u in _themes()) > max(u["ord"] for u in _core())


def test_every_unit_has_a_track_and_the_default_is_core():
    assert {u["track"] for u in course.UNITS} == {"core", "theme"}
    # The month files never name a track; core is what they get.
    from pipeline import course_m1
    assert all("track" not in u for u in course_m1.UNITS)
    assert all(u["track"] == "core" for u in course.CORE_ONLY)


def test_the_course_validates_against_the_starter(starter_conn):
    known = {r[0] for r in starter_conn.execute("SELECT lemma FROM lemma")}
    topics = {t.code for t in curriculum.TOPICS}
    assert course.validate(known, topics) == []


def test_validate_refuses_a_course_that_stopped_ending(monkeypatch):
    monkeypatch.setattr(course, "CORE_UNITS", 65)
    problems = course.validate(
        {w[0] for u in course.UNITS for w in u["words"]},
        {t.code for t in curriculum.TOPICS})
    assert any("it ENDS" in p for p in problems)


def test_validate_refuses_a_tier_with_the_wrong_number_of_units(monkeypatch):
    monkeypatch.setattr(course, "TIER_UNITS",
                        {"basic": 24, "intermediate": 21, "advanced": 19})
    problems = course.validate(
        {w[0] for u in course.UNITS for w in u["words"]},
        {t.code for t in curriculum.TOPICS})
    assert any("intermediate tier has 20" in p for p in problems)


def test_no_new_word_is_silently_dropped_as_a_duplicate():
    """_dedup keeps the first teaching and drops later ones without a sound.

    For the pre-existing month files that is a documented convenience. For the
    growth batch and the themes it would mean a unit quietly shorter than its
    author wrote, so the lists must already be disjoint."""
    grown = {(code, w[0]) for code, ws in course_growth.GROWTH.items() for w in ws}
    themed = {(u["code"], w[0]) for u in course_themes.UNITS for w in u["words"]}
    dropped = {(unit, lemma) for unit, lemma, _ in course.DUPLICATES}
    assert not (grown & dropped), sorted(grown & dropped)[:10]
    assert not (themed & dropped), sorted(themed & dropped)[:10]


def test_a_word_is_taught_in_exactly_one_unit():
    seen: dict[str, str] = {}
    for u in course.UNITS:
        for w in u["words"]:
            assert w[0] not in seen, f"{w[0]} in {seen[w[0]]} and {u['code']}"
            seen[w[0]] = u["code"]


def test_glosses_exist_in_both_locales_and_do_not_mix_scripts():
    mixed = re.compile(r"[А-Яа-яЁё][A-Za-z]|[A-Za-z][А-Яа-яЁё]")
    cyr = re.compile(r"[А-Яа-яЁё]")
    for u in course.UNITS:
        for lemma, upos, ru, en, _note in u["words"]:
            assert ru.strip() and en.strip(), f"{u['code']}: {lemma}"
            assert not mixed.search(ru), f"{lemma}: mixed script in {ru!r}"
            assert cyr.search(ru), f"{lemma}: Russian gloss has no Cyrillic: {ru!r}"
            assert not cyr.search(en), f"{lemma}: English gloss has Cyrillic: {en!r}"


def test_every_course_lemma_is_attested_with_its_part_of_speech(starter_conn):
    """The unattested-word gate: a word the pack cannot look up with the stated
    part of speech is unteachable. (validate() checks the lemma alone.)"""
    have = set(starter_conn.execute("SELECT lemma, upos FROM lemma"))
    missing = [(w[0], w[1]) for u in course.UNITS for w in u["words"]
               if (w[0], w[1]) not in have]
    assert not missing, missing[:15]


def test_the_starter_carries_this_course(starter_conn):
    """A pipeline change that was never followed by `make starter` fails here."""
    got = set(starter_conn.execute(
        "SELECT unit_code, lemma, upos, gloss_ru, gloss_en, note, ord FROM course_word"))
    assert got == set(course.words())
    units = {r[0]: r[1:] for r in starter_conn.execute(
        "SELECT code, track, tier, month, ord FROM course_unit")}
    assert units == {u["code"]: (u["track"], u["tier"], u["month"], u["ord"])
                     for u in course.UNITS}


def test_at_least_nine_in_ten_words_can_be_a_cloze(starter_conn):
    """Enrolling a unit turns each word into a sentence card; a word with no
    real sentence is skipped, and the unit teaches less than it says."""
    total = len({w[0] for u in course.UNITS for w in u["words"]})
    missing = course.unclozeable(starter_conn)
    share = 1 - len(missing) / total
    assert share >= course.CLOZEABLE_FLOOR, f"{share:.3f}; first gaps: {missing[:15]}"
    core = {w[0] for u in _core() for w in u["words"]}
    core_share = 1 - len([m for m in missing if m in core]) / len(core)
    assert core_share >= course.CLOZEABLE_FLOOR, f"core {core_share:.3f}"
    for tier in course.TIERS:
        words = {w[0] for u in _tier(tier) for w in u["words"]}
        share = 1 - len([m for m in missing if m in words]) / len(words)
        print(f"clozeable share, {tier}: {share:.1%} of {len(words)}")
        assert share >= course.CLOZEABLE_FLOOR, f"{tier} {share:.3f}"


def test_refresh_adds_the_track_and_tier_columns_to_an_older_pack(starter, tmp_path, monkeypatch):
    """The app opens a pack read-only and cannot add a column; refresh must."""
    # The starter has no lemmas for the ear trainer's rarer pair members, which
    # refresh would (rightly) refuse on; that gate is not what is tested here.
    monkeypatch.setattr(refresh.minimal_pairs, "unattested_in", lambda _p: [])
    pack = tmp_path / "old.sqlite"
    shutil.copy(starter, pack)
    conn = sqlite3.connect(pack)
    conn.execute("ALTER TABLE course_unit DROP COLUMN track")
    conn.execute("ALTER TABLE course_unit DROP COLUMN tier")
    conn.commit()
    conn.close()

    written = refresh.refresh(pack)
    assert written["course_unit"] == len(course.UNITS)

    conn = sqlite3.connect(pack)
    try:
        tracks = dict(conn.execute("SELECT track, COUNT(*) FROM course_unit GROUP BY track"))
        tiers = dict(conn.execute(
            "SELECT tier, COUNT(*) FROM course_unit WHERE track = 'core' GROUP BY tier"))
    finally:
        conn.close()
    assert tracks == {"core": 64, "theme": 12}
    assert tiers == {"basic": 24, "intermediate": 20, "advanced": 20}
