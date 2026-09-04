"""The verb conjugator, pinned against forms a grammar checks by hand.

Every expected string here was verified against a reference conjugation table
before it went in. If one of these fails, the generator is inventing Hungarian —
which is the one thing this pipeline is never allowed to do silently.
"""

from pipeline import verbs
from pipeline.inflect import Confidence


def forms_of(lemma, attested=()):
    return {f.form: f for f in verbs.conjugate(lemma, list(attested))}


def test_hoz_present_and_anchored_past():
    """The bug this module exists for: hoztunk, absent from Wiktionary."""
    attested = [("hoztuk", {"Tense": "Past", "Person": "1",
                            "Number": "Plur", "Definite": "Def"})]
    d = forms_of("hoz", attested)
    for expect in ["hozok", "hozol", "hoz", "hozunk", "hoztok", "hoznak",
                   "hozom", "hozod", "hozza", "hozzuk", "hozzátok", "hozzák",
                   "hoztunk", "hoztál", "hoztatok", "hoztak",
                   "hozlak", "hozni"]:
        assert expect in d, f"missing {expect}"
    # Anchored on an attested past form, so the past cells are MEDIUM.
    assert d["hoztunk"].confidence is Confidence.MEDIUM
    # 2SG present after a sibilant is -ol, never -sz.
    assert "hozsz" not in d


def test_sibilant_gemination_in_definite():
    """hoz+ja -> hozza; olvas+juk -> olvassuk; néz+jük -> nézzük."""
    assert "hozza" in forms_of("hoz")
    assert "olvassuk" in forms_of("olvas")
    assert "nézzük" in forms_of("néz")
    # After a non-sibilant the j survives: mond+ja -> mondja.
    assert "mondja" in forms_of("mond")


def test_front_verbs_take_i_forms():
    d = forms_of("néz")
    for expect in ["nézek", "nézel", "nézi", "nézitek", "nézik", "nézlek"]:
        assert expect in d, f"missing {expect}"


def test_ik_verb_drops_ik_outside_3sg():
    d = forms_of("dolgozik")
    assert "dolgozik" in d          # 3SG keeps it
    assert "dolgozunk" in d         # others build on the bare stem
    assert "dolgozni" in d
    assert "dolgozikni" not in d


def test_unanchored_past_is_low_confidence():
    d = forms_of("mond")            # no attested past supplied
    assert d["mondtunk"].confidence is Confidence.LOW


def test_irregular_verbs_are_refused():
    """A generator that emits `megytunk` has stopped being a safety net."""
    for lemma in ["megy", "van", "eszik", "jön", "tesz"]:
        assert verbs.conjugate(lemma, []) == [], lemma


def test_backfill_is_idempotent(tmp_path):
    import sqlite3
    db = tmp_path / "pack.sqlite"
    conn = sqlite3.connect(db)
    conn.executescript("""
        CREATE TABLE lemma (id INTEGER PRIMARY KEY, lemma TEXT, upos TEXT);
        CREATE TABLE wordform (form TEXT, lemma_id INTEGER, upos TEXT,
            feats TEXT DEFAULT '', source TEXT, confidence TEXT);
        CREATE TABLE sentence_token (sentence_id INTEGER, idx INTEGER,
            form TEXT, lemma_id INTEGER, feats TEXT DEFAULT '');
        INSERT INTO lemma VALUES (1, 'hoz', 'VERB');
        INSERT INTO wordform VALUES ('hoz', 1, 'VERB', '', 'wiktionary', 'high');
        INSERT INTO sentence_token VALUES (1, 0, 'Elhozták', 1, '');
    """)
    first = verbs.backfill(conn)
    assert first["generated_forms"] > 10
    assert first["corpus_forms"] == 1          # Elhozták, linked but unindexed
    again = verbs.backfill(conn)
    assert again["generated_forms"] == 0       # a re-run adds nothing
    assert again["corpus_forms"] == 0
    # The attested row was never displaced.
    n = conn.execute("SELECT COUNT(*) FROM wordform WHERE form='hoz'").fetchone()[0]
    assert n == 1
