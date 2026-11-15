"""Three authored texts per unit (pipeline/texts.py, ADR-075, ADR-076): V1-V6,
--require-all and the catalogue export.

No real text is checked here; one tiny fixture text runs against a tiny pack built
from schema.sql, with the size table loosened where the test is not about size.
"""

import sqlite3
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from pipeline import course, texts

REAL_LOAD_COLLOQUIAL = texts.load_colloquial  # the autouse fixture below swaps the module's
SCHEMA = Path(__file__).resolve().parents[1] / "pipeline" / "schema.sql"
BASIC = [u for u in course.UNITS if u.get("tier", "basic") == "basic"]
UNIT = next(u for u in BASIC if u["code"] == "m1-u1-koszones")
CODE = UNIT["code"]


def S(hu, ru="р", en="e"):
    return {"hu": hu, "ru": ru, "en": en}


def text(genre, *paras, names=("Kata",), tier="short"):
    return {"unit": CODE, "tier": tier, "genre": genre, "checked": "ai",
            "title": {"ru": "Привет", "en": "Hi"}, "names": list(names),
            "paras": [[S(h) for h in p] for p in paras]}


@pytest.fixture
def pack():
    c = sqlite3.connect(":memory:")
    c.executescript(SCHEMA.read_text(encoding="utf-8"))
    c.execute("INSERT INTO course_unit (code, month, ord, title_ru, title_en, goal_ru, goal_en, "
              "situation_ru, situation_en) VALUES (?,1,101,'т','t','г','g','с','s')", (CODE,))
    lemmas = [  # id, lemma, upos, rank, cefr
        (1, "ház", "NOUN", 100, "A1"), (2, "nagy", "ADJ", 50, "A1"), (3, "van", "VERB", 2, "A1"),
        (4, "szia", "INTJ", 3000, "A1"), (5, "zsilip", "NOUN", 9000, "A1"),
        (6, "kutya", "NOUN", 200, "A1"), (7, "bonyolult", "ADJ", 800, "B2"),
        (8, "Anna", "PROPN", None, None),
    ]
    c.executemany("INSERT INTO lemma (id, lemma, upos, freq_rank, cefr) VALUES (?,?,?,?,?)", lemmas)
    forms = [("ház", 1, "wiktionary"), ("nagy", 2, "wiktionary"), ("van", 3, "wiktionary"),
             ("szia", 4, "wiktionary"), ("zsilip", 5, "wiktionary"), ("kutya", 6, "wiktionary"),
             ("bonyolult", 7, "wiktionary"), ("Anna", 8, "wiktionary"),
             ("házban", 1, "generated"), ("kutyák", 6, "generated")]
    c.executemany("INSERT INTO wordform (form, lemma_id, upos, source, confidence) "
                  "VALUES (?,?,'X',?,'high')", forms)
    c.commit()
    return c


@pytest.fixture(autouse=True)
def loose(monkeypatch):
    """Size table wide open (V4 has its own tests), no allow-list on disk."""
    wide = {g: {t: (1, 1000) for t in texts.TIERS} for g in texts.RANGES}
    monkeypatch.setattr(texts, "RANGES", wide)
    monkeypatch.setattr(texts, "load_allow", lambda *a: {})
    monkeypatch.setattr(texts, "load_band_exempt", lambda *a: {})
    monkeypatch.setattr(texts, "load_colloquial", lambda *a: ({}, []))


def check(pack, t, allow=None, exempt=None):
    lex = texts.TextLexicon(pack, allow or {}, exempt or {})
    return texts.check_text(lex, UNIT, "A1", t["names"], [s["hu"] for p in t["paras"] for s in p])


# ── V1: what a word may be ─────────────────────────────────────────────────

def test_an_attested_word_passes_and_an_unknown_one_fails(pack):
    _rep, probs = check(pack, text("note", ["Ház van.", "Nagy ház."]))
    assert probs == []
    _rep, probs = check(pack, text("note", ["Ház xyzzy."]))
    assert any("'xyzzy'" in p and "allow-list" in p for p in probs)


def test_a_generated_form_passes_and_is_counted(pack):
    rep, probs = check(pack, text("note", ["Ház van.", "Nagy házban.", "Házban van."]))
    assert probs == []
    assert rep["generated"] == {"házban": 2}


def test_the_allow_list_rescues_a_form_for_its_unit_only(pack):
    t = text("note", ["Ház ablakok."])
    assert check(pack, t)[1]
    rep, probs = check(pack, t, {"ablakok": [("ház", "NOUN", CODE)]})
    assert probs == [] and rep["allowed"] == {"ablakok": 1}
    assert check(pack, t, {"ablakok": [("ház", "NOUN", "m2-u1-bolt")]})[1]
    assert check(pack, t, {"ablakok": [("ház", "NOUN", "*")]})[1] == []


def test_an_allow_listed_lemma_still_faces_the_band_gate(pack):
    _rep, probs = check(pack, text("note", ["Ház bonyolultak."]),
                        {"bonyolultak": [("bonyolult", "ADJ", "*")]})
    assert any("above A1" in p for p in probs)


def test_a_declared_name_passes_an_undeclared_one_does_not(pack):
    assert check(pack, text("note", ["Kata van."]))[1] == []
    assert any("'Dani'" in p for p in check(pack, text("note", ["Dani van."]))[1])
    # a lowercase word cannot hide behind a name entry
    assert check(pack, text("note", ["Ház xyzzy."], names=("xyzzy",)))[1]


def test_checking_never_writes_to_the_pack(pack):
    t = text("note", ["Ház van.", "Házban Kata."])
    n = pack.execute("SELECT COUNT(*) FROM wordform").fetchone()[0]
    check(pack, t)
    assert pack.execute("SELECT COUNT(*) FROM wordform").fetchone()[0] == n


def test_a_band_exempt_lemma_skips_v2_but_still_counts_in_v3(pack):
    t = text("note", ["Ház bonyolult.", "Ház van."])
    assert any("above A1" in p for p in check(pack, t)[1])
    rep, probs = check(pack, t, exempt={"bonyolult": [("*", CODE)]})
    assert not any("above A1" in p for p in probs)
    assert rep["counted"] == 3 and rep["top"] == 2  # van is a unit word (exempt); bonyolult still counts and misses top-500
    assert check(pack, t, exempt={"bonyolult": [("ADJ", "m2-u1-bolt")]})[1]  # other unit: still strict
    assert check(pack, t, exempt={"bonyolult": [("NOUN", "*")]})[1]  # other upos: still strict


def test_the_exemption_file_needs_a_reason_and_the_shipped_one_loads(tmp_path, monkeypatch):
    monkeypatch.undo()
    f = tmp_path / "a.tsv"
    f.write_text("[band-exempt]\nlemma\tupos\tunit\treason\nbolt\t*\t*\tshort\n", encoding="utf-8")
    with pytest.raises(ValueError, match="reason"):
        texts.load_band_exempt(f)
    f.write_text("[band-exempt]\nbolt\t*\t*\ta long enough reason\n[forms]\nx\tház\tNOUN\t*\tr\n",
                 encoding="utf-8")
    assert texts.load_band_exempt(f) == {"bolt": [("*", "*")]}
    assert texts.load_allow(f) == {"x": [("ház", "NOUN", "*")]}
    assert texts.load_band_exempt(texts.ALLOW_FILE)["hétfő"] == [("*", "*")]
    assert "hétkor" in texts.load_allow(texts.ALLOW_FILE)  # the shipped clock-time forms load


# ── V2 / V3: the band and frequency gates are unchanged ────────────────────

def test_a_lemma_above_the_band_fails_unless_the_unit_teaches_it(pack):
    assert any("above A1" in p for p in check(pack, text("note", ["Ház bonyolult."]))[1])


def test_the_frequency_gate_is_still_a_hard_gate(pack):
    _rep, probs = check(pack, text("note", ["Zsilip van.", "Zsilip ház."]))
    assert any("frequency gate" in p for p in probs)


# ── V4: size and shape ─────────────────────────────────────────────────────

def test_the_size_table_by_band_group(monkeypatch):
    monkeypatch.undo()  # the real table
    by = {u["code"]: u for u in BASIC}
    assert texts.size_group(by["m2-u4-mennyi"]) == "a1"
    assert texts.size_group(by["m3-u1-ugyintezes"]) == "a2"
    assert texts.size_group(by["m5-u1-elromlott"]) == "b" == texts.size_group(by["t-szamok"])
    assert texts.RANGES["a1"]["long"] == (180, 280)
    assert texts.MAX_SENTENCE == {"a1": 10, "a2": 12, "b": 14}


def test_size_problems_report_words_sentence_cap_and_paragraphs(monkeypatch):
    monkeypatch.undo()
    few = texts.size_problems(UNIT, "short", "note", [["Ház van."]], "x")
    assert any("2 words" in p and "30-70" in p for p in few)
    long_sentence = " ".join(["ház"] * 11) + "."
    out = texts.size_problems(UNIT, "short", "note", [[long_sentence] * 3], "x")
    assert any("at most 10" in p for p in out)
    five = [["Ház van."]] * 5
    assert any("paragraphs" in p for p in texts.size_problems(UNIT, "short", "note", five, "x"))
    assert not any("paragraphs" in p for p in texts.size_problems(UNIT, "short", "dialogue", five, "x"))


# ── V5 / V6, shape ─────────────────────────────────────────────────────────

def full(**over):
    t = {"short": text("note", ["Ház van."]),
         "medium": text("dialogue", ["Ház van."], ["Nagy ház."], tier="medium"),
         "long": text("story", ["Ház van."], ["Nagy ház."], ["Kata van."], tier="long")}
    t.update(over)
    return {CODE: t}


def test_a_full_triplet_of_three_genres_is_clean(pack):
    assert texts.data_problems(full(), require_all=False) == [
        p for p in texts.data_problems(full()) if "no langpack/texts" in p]
    assert not [p for p in texts.data_problems(full(), True) if p.startswith(CODE + ".")
                or p.startswith(CODE + ":")]


def test_equal_genres_two_dialogues_and_a_non_prose_long_are_refused():
    same = full(short=text("dialogue", ["Ház van."]))
    p = texts.data_problems(same, True)
    assert any("genres must differ" in x for x in p) and any("at most one dialogue" in x for x in p)
    nonprose = full(long=text("note", ["Ház van."], ["Nagy ház."], ["Kata van."], tier="long"))
    assert any("connected prose" in x for x in texts.data_problems(nonprose))


def test_require_all_names_every_unit_without_a_full_triplet():
    p = texts.data_problems({}, require_all=True)
    assert len([x for x in p if "no langpack/texts/" in x]) == len(BASIC) == 36
    partial = {CODE: {"short": text("note", ["Ház van."])}}
    p = texts.data_problems(partial, require_all=True)
    assert any(f"{CODE}: the medium text is missing" in x for x in p)
    assert not any(f"{CODE}: the medium" in x for x in texts.data_problems(partial))


def test_shape_rules_cover_v6_genres_tiers_and_keys(tmp_path):
    (tmp_path / f"{CODE}.json").write_text("""{
      "short": {"genre": "nope", "title": {"ru": "а", "en": "b"}, "paras": [[{"hu": "Ház.", "ru": "", "en": "x"}]]},
      "huge": {}}""", encoding="utf-8")
    (tmp_path / "ghost.json").write_text("{}", encoding="utf-8")
    loaded, bad = texts.load(tmp_path)
    assert loaded == {}
    joined = "\n".join(bad)
    assert "genre 'nope'" in joined and "present (V6)" in joined
    assert "unknown tier 'huge'" in joined and "ghost.json: no such basic-tier unit" in joined


def test_digits_and_speaker_labels_are_refused():
    p = texts.data_problems({CODE: {"short": text("note", ["Ház 5."])}})
    assert any("characters outside" in x for x in p)


def test_the_report_counts_recycling_and_generated_forms(pack):
    t = text("note", ["Szia, szia, szia.", "Házban."])
    rep, probs = check(pack, t)
    assert probs == []
    assert rep["unit_hits"]["szia"] == 3
    assert texts.recycling({"words": ["szia", "sziasztok"]}, [rep]) == 0.5


def test_the_genre_list_has_labels_and_every_menu_entry_exists():
    for tier, ids in texts.MENU.items():
        assert set(ids) <= set(texts.GENRES), tier
    assert all(ru and en for ru, en, _h in texts.GENRES.values())
    assert set(texts.PROSE) <= set(texts.GENRES)


# ── the catalogue (ADR-076) ────────────────────────────────────────────────

def test_catalog_codes_ords_and_shape():
    t = full()
    t[CODE]["short"]["register"] = "standard"
    sets = texts.catalog_sets(t)
    assert [(s["code"], s["tier"], s["ord"]) for s in sets] == [
        (f"{CODE}-dialogue", "short", 10101), (CODE, "medium", 10102), (f"{CODE}-long", "long", 10103)]
    s = sets[0]
    assert (s["unit"], s["unitOrd"], s["level"], s["track"], s["register"]) == (
        CODE, 101, "A1", "core", "standard")
    assert (s["themeRu"], s["themeEn"]) == (UNIT["title_ru"], UNIT["title_en"])
    assert (s["titleRu"], s["titleEn"], s["checked"], s["words"]) == ("Привет", "Hi", "ai", 2)
    assert s["lines"] == [{"hu": "Ház van.", "ru": "р", "en": "e", "para": 0}]
    assert [ln["para"] for ln in sets[2]["lines"]] == [0, 1, 2]


def test_catalog_supports_the_colloquial_tiers():
    talk = text("chat", ["Ház van."], tier="talk")
    talk["register"] = "colloquial"
    mini = text("sms", ["Ház van."], tier="mini")
    mini.update(register="colloquial", slug="phone", index=2)
    mini["paras"][0][0]["who"] = "A"
    sets = texts.catalog_sets({CODE: {"talk": talk, "mini": mini}})
    assert [(s["code"], s["ord"], s["register"]) for s in sets] == [
        (f"{CODE}-talk", 10104, "colloquial"), (f"{CODE}-talk-phone", 10112, "colloquial")]


def test_catalog_file_is_deterministic_one_set_per_line(tmp_path):
    out = texts.catalog_text(full())
    assert out == texts.catalog_text(dict(reversed(list(full().items()))))
    lines = out.split("\n")
    assert lines[0] == '{"format":1,"sets":[' and lines[-2] == "]}" and lines[-1] == ""
    assert len(lines) == 3 + 3 and lines[1].startswith('{"checked":"ai","code":')
    path = tmp_path / "sub" / "catalog.json"
    assert texts.export(full(), path) == 3
    assert texts.check_export(full(), path)
    path.write_text(out.replace("Привет", "Привет!"), encoding="utf-8")
    assert not texts.check_export(full(), path)
    assert not texts.check_export(full(), tmp_path / "missing.json")


def test_duplicate_ords_are_refused():
    t = full()
    t[CODE]["medium"]["tier"] = "short"
    with pytest.raises(ValueError, match="duplicate"):
        texts.catalog_text(t)


def test_catalog_in_sync():
    """The committed catalogue equals a re-export of the real langpack/texts. (conftest
    points TEXTS_DIR at an empty dir for the other tests, so restore the real one.)"""
    real = Path(__file__).resolve().parents[1] / "texts"
    if not texts.CATALOG_PATH.exists() or not any(real.glob("*.json")):
        pytest.skip("no committed catalogue or no real texts")
    loaded, bad = texts.load(real, real.parent / "talk")
    assert not bad
    assert texts.check_export(loaded), "run `make texts` and commit backend/internal/readsets/catalog.json"


# ── colloquial material: talk text, minis, lexicon, banned list (ADR-077) ──

COLL = {  # form -> rows, as load_colloquial returns them
    "hát": [{"lemma": None, "upos": "", "kind": "particle", "level": "A1", "ru": "ну", "en": "well",
             "reason": "spoken discourse particle"}],
    "nemtom": [{"lemma": None, "upos": "", "kind": "contraction", "level": "A2", "ru": "не знаю",
                "en": "dunno", "reason": "spoken contraction"}],
    "sztem": [{"lemma": None, "upos": "", "kind": "chat", "level": "B1", "ru": "по-моему",
               "en": "imo", "reason": "chat abbreviation"}],
    "szia": [{"lemma": None, "upos": "", "kind": "greeting", "level": "A1", "ru": "привет", "en": "hi",
              "reason": "informal greeting"}],
    "kaja": [{"lemma": "kutya", "upos": "NOUN", "kind": "clipping", "level": "A1", "ru": "еда",
              "en": "food", "reason": "clipped word for food"}],
}


def mini(slug, *turns, index=0, names=()):
    who = "AB"
    return {"unit": CODE, "tier": "mini", "register": "colloquial", "genre": "dialogue", "checked": "ai",
            "slug": slug, "index": index, "title": {"ru": "т", "en": "t"},
            "scene": {"ru": "сцена", "en": "scene"}, "names": list(names),
            "paras": [[{"who": who[i % 2], **S(h)} for i, h in enumerate(turns)]]}


def coll_check(pack, t, band="A1", unit=UNIT):
    lex = texts.TextLexicon(pack, {}, {}, COLL)
    return texts.check_text(lex, unit, band, t["names"], [s["hu"] for p in t["paras"] for s in p],
                            "colloquial")


def write_talk(directory, code, raw):
    import json
    (directory / f"{code}.json").write_text(json.dumps(raw, ensure_ascii=False), encoding="utf-8")


def talk_file(**over):
    d = {"slug": "udvozles", "title": {"ru": "т", "en": "t"}, "scene": {"ru": "с", "en": "s"},
         "names": [], "checked": "ai",
         "turns": [{"who": "A", "hu": "Szia!", "ru": "р", "en": "e"},
                   {"who": "B", "hu": "Hát szia.", "ru": "р", "en": "e"}]}
    d.update(over)
    return {"talk": {"genre": "chat", "title": {"ru": "т", "en": "t"}, "names": [], "checked": "ai",
                     "paras": [[S("Szia!"), S("Hát szia.")]]},
            "dialogues": [d]}


def test_the_talk_loader_normalises_and_reports_shape_errors(tmp_path):
    write_talk(tmp_path, CODE, talk_file())
    loaded, bad = texts.load_talk(tmp_path)
    assert bad == []
    t, m = loaded[CODE]["talk"], loaded[CODE]["mini-udvozles"]
    assert (t["tier"], t["register"], t["genre"]) == ("talk", "colloquial", "chat")
    assert (m["tier"], m["slug"], m["index"], m["genre"]) == ("mini", "udvozles", 0, "dialogue")
    assert m["paras"][0][1]["who"] == "B" and m["scene"]["en"] == "s"
    for bad_over, msg in (({"slug": "Bad Slug"}, "kebab"), ({"turns": []}, "turns"),
                          ({"turns": [{"who": "Z", "hu": "a", "ru": "r", "en": "e"}]}, "who"),
                          ({"turns": [{"hu": "a", "ru": "r", "en": "e"}]}, "who"),
                          ({"scene": {"ru": "x"}}, "scene")):
        write_talk(tmp_path, CODE, talk_file(**bad_over))
        loaded, bad = texts.load_talk(tmp_path)
        assert any(msg in b for b in bad), (bad_over, bad)
        assert "mini-udvozles" not in loaded.get(CODE, {}) and "mini-bad slug" not in loaded.get(CODE, {})
    raw = talk_file()
    raw["dialogues"].append(raw["dialogues"][0])
    write_talk(tmp_path, CODE, raw)
    assert any("duplicate slug" in b for b in texts.load_talk(tmp_path)[1])
    raw = talk_file()
    raw["talk"]["genre"] = "story"
    write_talk(tmp_path, CODE, raw)
    assert any("genre" in b for b in texts.load_talk(tmp_path)[1])
    write_talk(tmp_path, "nope", talk_file())
    assert any("no such basic-tier unit" in b for b in texts.load_talk(tmp_path)[1])


def test_load_merges_the_talk_directory_only_by_default_or_on_request(tmp_path):
    write_talk(tmp_path, CODE, talk_file())
    std = tmp_path / "std"
    std.mkdir()
    assert texts.load(std)[0] == {}  # an explicit standard directory: no talk
    merged, bad = texts.load(std, tmp_path)
    assert bad == [] and set(merged[CODE]) == {"talk", "mini-udvozles"}


def test_v4c_sizes_by_band_group():
    ok = mini("a", "Szia!", "Hát szia, ház van.", "Ház van.", "Nagy ház van, hát.", "Nagy ház.")  # 13 words
    assert texts.talk_problems({CODE: {"mini-a": ok}}) == []
    few = mini("b", "Szia!", "Szia!")
    assert any("words, a mini" in p for p in texts.talk_problems({CODE: {"mini-b": few}}))
    one = mini("c", "Szia, ház van, nagy ház van.")
    assert any("1 turns" in p for p in texts.talk_problems({CODE: {"mini-c": one}}))
    long_turn = mini("d", "Ház van nagy ház van nagy ház.", "Szia.", "Ház.")
    assert any("words in a turn, at most 6" in p for p in texts.talk_problems({CODE: {"mini-d": long_turn}}))
    b_unit = next(u["code"] for u in BASIC if texts.size_group(u) == "b")
    lo, hi = texts.TALK_SIZE["b"]
    assert (lo, hi) == (150, 260) and texts.MINI_SIZE["a2"][2] == (30, 70)
    talk = {"unit": b_unit, "tier": "talk", "genre": "chat", "paras": [[S("Ház van.")]]}
    assert any("a talk text" in p for p in texts.talk_problems({b_unit: {"talk": talk}}))


def test_mini_count_slug_uniqueness_and_duplicate_turns_across_minis(monkeypatch):
    monkeypatch.setitem(texts.MINI_SIZE, "a1", ((1, 10), 10, (1, 100)))
    same = {f"mini-m{i}": mini(f"m{i}", "Szia ház.", f"Hát ház van {'ház ' * i}.", index=i) for i in range(5)}
    assert any("also in mini 'm0'" in p for p in texts.talk_problems({CODE: same}))
    uniq = {f"mini-m{i}": mini(f"m{i}", "Szia!" + " Ház" * i, "Hát ház van.", "Nagy ház van, ház." + " Ház" * i,
                               index=i) for i in range(1, 6)}
    for i, k in enumerate(uniq):
        uniq[k]["paras"][0][1]["hu"] = "Hát " + "kutya " * (i + 1) + "van."
    assert texts.talk_problems({CODE: uniq}) == []
    one = {"mini-m1": uniq["mini-m1"]}
    assert texts.talk_problems({CODE: one}) == []  # too few is an error only under --require-talk
    assert any("1 minis, a unit wants 5-10" in p for p in texts.talk_problems({CODE: one}, require_talk=True))
    eleven = {f"mini-x{i}": mini(f"x{i}", "Szia" + " ház" * i, "Nagy ház.", index=i) for i in range(11)}
    assert any("11 minis" in p for p in texts.talk_problems({CODE: eleven}))


def test_require_talk_names_units_without_talk_or_minis():
    probs = texts.talk_problems({}, require_talk=True)
    assert any(p.startswith(f"{CODE}: no talk text") for p in probs)
    assert any(p.startswith(f"{CODE}: no minis") for p in probs)
    assert texts.talk_problems({}, require_talk=False) == []
    # the standard texts validate alone: --require-all does not ask for talk
    assert not any("talk" in p for p in texts.data_problems(full(), require_all=True)
                   if p.startswith(CODE))


def test_banned_stems_and_exact_forms():
    banned = ["kurv", "szar$", "fasz$", "fasza$", "geci"]
    assert texts.banned_hits("Ez kurvára jó. Geci!", banned) == ["kurvára", "Geci"]
    assert texts.banned_hits("Szar. Fasza! Fasz.", banned) == ["Szar", "Fasza", "Fasz"]
    assert texts.banned_hits("A szarvas és a szarka a faszekrényben.", banned) == []
    t = {CODE: {"short": text("note", ["Ház van, szar."])}}
    assert any("banned" in p and "'szar'" in p for p in texts.data_problems(t, banned=banned))
    t = {CODE: {"mini-a": mini("a", "Szia!", "Kurva ház.")}}  # a talk text too
    assert any("banned" in p for p in texts.data_problems(t, banned=banned))


def test_the_shipped_lexicon_loads_and_is_consistent():
    allowed, banned = REAL_LOAD_COLLOQUIAL(texts.COLLOQUIAL_FILE)
    assert len(allowed) >= 40 and len(banned) >= 20
    for form, rows in allowed.items():
        assert not texts.banned_hits(form, banned), form
        assert all(r["level"] in texts.LEVELS and r["kind"] in texts.KINDS for r in rows)
    for must in ("kurva", "bazdmeg", "bazmeg", "geci", "fasz", "fasza", "szar", "picsa", "faszom", "baszki"):
        assert texts.banned_hits(must, banned), must
    assert not texts.banned_hits("szarvas faszekrény", banned)


def test_the_lexicon_file_needs_all_columns_and_a_reason(tmp_path):
    f = tmp_path / "c.tsv"
    f.write_text("[allowed]\nhát\t-\t-\tparticle\tA1\tну\twell\tshort\n", encoding="utf-8")
    with pytest.raises(ValueError, match="reason"):
        REAL_LOAD_COLLOQUIAL(f)
    f.write_text("[allowed]\nhát\t-\t-\tmeme\tA1\tну\twell\tspoken particle ok\n", encoding="utf-8")
    with pytest.raises(ValueError, match="kind"):
        REAL_LOAD_COLLOQUIAL(f)
    f.write_text("[allowed]\nhát\t-\t-\tparticle\tA1\tну\twell\tspoken particle ok\n[banned]\nkurv\tx\n",
                 encoding="utf-8")
    assert REAL_LOAD_COLLOQUIAL(f) == ({"hát": [{"lemma": None, "upos": "", "kind": "particle",
                                                  "level": "A1", "ru": "ну", "en": "well",
                                                  "reason": "spoken particle ok"}]}, ["kurv"])


def test_a_colloquial_row_passes_v1_at_its_level_and_fails_above_it(pack):
    rep, probs = coll_check(pack, mini("a", "Hát ház van.", "Nagy ház."))
    assert probs == [] and rep["markers"] == 1 and rep["colloquial"] == {"hát": 1}
    _rep, probs = coll_check(pack, mini("a", "Nemtom, ház.", "Hát ház van."))  # A2 row in an A1 text
    assert any("'Nemtom'" in p and "level A2" in p for p in probs)
    _rep, probs = coll_check(pack, mini("a", "Nemtom, ház.", "Hát ház van."), band="A2",
                             unit=next(u for u in BASIC if u["month"] == 3))
    assert not any("Nemtom" in p for p in probs)
    _rep, probs = coll_check(pack, mini("a", "Sztem ház.", "Hát ház van."), band="A2",
                             unit=next(u for u in BASIC if u["month"] == 3))
    assert any("'Sztem'" in p and "level B1" in p for p in probs)
    # a standard text never needs a row, and an unknown word is still rejected there
    lex = texts.TextLexicon(pack, {}, {}, COLL)
    _r, probs = texts.check_text(lex, UNIT, "A1", [], ["Hát ház van.", "Nemtom."])
    assert any("level A2" in p for p in probs)


def test_a_dash_lemma_is_exempt_from_v2_v3_and_a_known_lemma_is_not(pack):
    # kaja reads as the pack lemma kutya (rank 200, A1); hát has no lemma at all
    rep, probs = coll_check(pack, mini("a", "Kaja van.", "Hát nagy ház.", "Ház van nagy, ház.", "Nagy ház van, nagy ház."))
    assert probs == [] and rep["markers"] == 2 and rep["exempt"] >= 1


def test_marker_density_needs_one_and_caps_at_twenty_percent(pack):
    _rep, probs = coll_check(pack, mini("a", "Ház van.", "Nagy ház."))
    assert any("no colloquial marker" in p for p in probs)
    _rep, probs = coll_check(pack, mini("a", "Hát hát.", "Hát ház."))
    assert any("caricature" in p for p in probs)
    _rep, probs = coll_check(pack, mini("a", "Hát ház van.", "Nagy ház van ház.", "Ház van, nagy ház."))
    assert probs == []
    # greetings count as a marker (the minimum) but never toward the cap
    rep, probs = coll_check(pack, mini("a", "Szia, szia!", "Szia, ház.", "Szia!"))
    assert probs == [] and rep["markers"] == 4 and rep["capped"] == 0
    _rep, probs = coll_check(pack, mini("a", "Szia, szia!", "Ház van.", "Nagy ház."))
    assert probs == []  # a lone greeting satisfies the minimum


def test_v3_frees_earlier_taught_lemmas_in_colloquial_texts_only(pack, monkeypatch):
    # zsilip is rank 9000 (outside the top 500) and is not this unit's word
    monkeypatch.setattr(texts.dialogues, "_unit_words", lambda u: {"zsilip"})
    t = mini("a", "Hát zsilip van.", "Zsilip nagy.")
    _rep, probs = coll_check(pack, t)
    assert not any("frequency gate" in p for p in probs)
    lex = texts.TextLexicon(pack, {}, {}, COLL)
    _rep, probs = texts.check_text(lex, UNIT, "A1", [], ["Zsilip van.", "Zsilip nagy."])
    assert any("frequency gate" in p for p in probs)  # a standard text keeps V3 for earlier words
    monkeypatch.setattr(texts.dialogues, "_unit_words", lambda u: set())
    _rep, probs = coll_check(pack, t)
    assert any("frequency gate" in p for p in probs)


def test_a_rejected_token_prints_candidates_and_the_row_to_paste(pack):
    _rep, probs = check(pack, text("note", ["Háznak van."]))
    p = next(p for p in probs if "'Háznak'" in p)
    assert "hint:" in p and "ház/NOUN" in p
    assert f"háznak\tház\tNOUN\t{CODE}\t" in p and "texts_allow.tsv" in p
    _rep, probs = check(pack, text("note", ["Qwertz."]))
    assert "no pack lemma starts with 'qwer'" in probs[0] and "<lemma>" in probs[0]


def test_talk_and_minis_export_with_who_register_scene_and_ords(tmp_path):
    write_talk(tmp_path, CODE, talk_file())
    (tmp_path / "std").mkdir()
    loaded, bad = texts.load(tmp_path / "std", tmp_path)
    assert bad == []
    sets = texts.catalog_sets(loaded)
    assert [(s["code"], s["tier"], s["ord"], s["register"], s["genre"]) for s in sets] == [
        (f"{CODE}-talk", "talk", 10104, "colloquial", "chat"),
        (f"{CODE}-talk-udvozles", "mini", 10110, "colloquial", "dialogue")]
    mini_set = sets[1]
    assert (mini_set["sceneRu"], mini_set["sceneEn"]) == ("с", "s")
    assert [ln["who"] for ln in mini_set["lines"]] == ["A", "B"]
    assert "sceneRu" not in sets[0] and "who" not in sets[0]["lines"][0]
    path = tmp_path / "out" / "catalog.json"
    assert texts.export(loaded, path) == 2 and texts.check_export(loaded, path)


def test_talk_texts_do_not_disturb_the_standard_triplet_rules():
    t = full()
    t[CODE]["talk"] = {**text("chat", ["Ház van."], tier="talk"), "register": "colloquial"}
    t[CODE]["mini-a"] = mini("a", "Szia!", "Hát szia.")
    assert not any("genres must differ" in p or "at most one dialogue" in p
                   for p in texts.data_problems(t, require_all=True))


def test_phone_and_voicemsg_are_talk_only_genres():
    assert {"phone", "voicemsg"} <= set(texts.GENRES) and set(texts.TALK_GENRES) <= set(texts.GENRES)
    t = {CODE: {"short": text("phone", ["Ház van."])}}
    assert any("colloquial talk text only" in p for p in texts.data_problems(t))
