"""release.py: the publishable pack is the full pack minus non-Tatoeba sentences.

Runs on a tiny pack built from schema.sql, so it needs no 357 MB file.
"""

import gzip
import sqlite3
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from pipeline import release

SCHEMA = Path(__file__).resolve().parents[1] / "pipeline" / "schema.sql"


def _fixture(path: Path) -> None:
    c = sqlite3.connect(path)
    c.executescript(SCHEMA.read_text(encoding="utf-8"))
    c.execute("INSERT INTO pack_meta VALUES ('version','1')")
    c.execute("INSERT INTO pack_meta VALUES ('stats','{\"sentences\": 4, \"lemmas\": 0}')")
    rows = [(1, "a ház nagy", "tatoeba"), (2, "a ház kicsi", "tatoeba"),
            (3, "a ház jó", "opensubtitles"), (4, "a ház nagy", "opensubtitles")]
    for i, hu, src in rows:
        c.execute("INSERT INTO sentence VALUES (?,?,?,?,?,NULL)", (i, hu, src, "x", 3))
        c.execute("INSERT INTO sentence_trans VALUES (?,?,?)", (i, "ru", "t"))
        for k, w in enumerate(hu.split()):
            c.execute("INSERT INTO sentence_token VALUES (?,?,?,NULL,'')", (i, k, w))
    c.execute("INSERT INTO reading_set VALUES ('s1','A1','core',1,'a','a','','')")
    c.execute("INSERT INTO reading_set VALUES ('s2','A1','core',2,'b','b','','')")
    c.execute("INSERT INTO reading_set_sentence (set_code, sentence_id, ord) VALUES ('s1',1,1)")
    c.execute("INSERT INTO reading_set_sentence (set_code, sentence_id, ord) VALUES ('s2',3,1)")
    c.commit()
    c.close()


def test_derive_keeps_only_tatoeba(tmp_path):
    full = tmp_path / "full.sqlite"
    _fixture(full)
    r = release.derive(full, tmp_path / "out")

    assert r["per_source"] == {"tatoeba": 2}
    assert r["removed_sentences"] == 2
    c = sqlite3.connect(r["sqlite"])
    assert [x[0] for x in c.execute("SELECT id FROM sentence ORDER BY id")] == [1, 2]  # ids kept
    assert c.execute("SELECT COUNT(*) FROM sentence_token WHERE sentence_id > 2").fetchone()[0] == 0
    assert c.execute("SELECT code FROM reading_set").fetchall() == [("s1",)]
    # "a ház" occurs in both kept sentences, so it stays attested.
    assert c.execute("SELECT count FROM ngram WHERE gram='a ház'").fetchone() == (2,)
    assert release.check(c) == []
    c.close()

    out = sqlite3.connect(":memory:")
    assert r["gz"].exists() and r["gz_sha256"] and len(r["gz_sha256"]) == 64
    with gzip.open(r["gz"]) as fh:
        assert fh.read(15) == b"SQLite format 3"
    out.close()
    assert full.exists()  # the source is only read


def test_check_flags_foreign_source(tmp_path):
    full = tmp_path / "full.sqlite"
    _fixture(full)
    c = sqlite3.connect(full)
    assert any("sources" in p for p in release.check(c))
    c.close()


def test_derive_keeps_authored_dialogues_but_never_counts_them_as_ngrams(tmp_path):
    full = tmp_path / "full.sqlite"
    _fixture(full)
    c = sqlite3.connect(full)
    # Two authored lines repeating a bigram that no corpus sentence has.
    for sid, hu in ((9000001, "zöld alma kell"), (9000002, "zöld alma van")):
        c.execute("INSERT INTO sentence VALUES (?,?,?,?,?,NULL)", (sid, hu, "authored", "MIT", 3))
        c.execute("INSERT INTO sentence_trans VALUES (?,?,?)", (sid, "ru", "t"))
    c.execute("INSERT INTO reading_set VALUES ('s1-dialogue','A1','dialogue',0,'a','a','','')")
    c.execute("INSERT INTO reading_set_sentence (set_code, sentence_id, ord) VALUES ('s1-dialogue',9000001,0)")
    c.commit()
    c.close()

    r = release.derive(full, tmp_path / "out")
    assert r["per_source"] == {"authored": 2, "tatoeba": 2}
    out = sqlite3.connect(r["sqlite"])
    assert out.execute("SELECT COUNT(*) FROM sentence WHERE source = 'authored'").fetchone() == (2,)
    # "zöld alma" occurs twice, but only in authored text: it is NOT attested.
    assert out.execute("SELECT COUNT(*) FROM ngram WHERE gram = 'zöld alma'").fetchone() == (0,)
    assert out.execute("SELECT count FROM ngram WHERE gram='a ház'").fetchone() == (2,)
    assert release.check(out) == []
    out.close()
