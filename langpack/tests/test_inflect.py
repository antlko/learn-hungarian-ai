"""Inflection tests.

Every expected form here is one a learner would actually meet. Where Hungarian
is genuinely lexically conditioned, the test asserts that the correct form is
*among* the generated candidates (recognition) and, separately, that the display
paradigm does not assert something wrong (production).
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import pytest

from pipeline.harmony import Harmony, classify
from pipeline.inflect import (
    Confidence,
    accusative,
    instrumental,
    noun_forms,
    paradigm,
    plural,
    possessives,
    simple_case,
    superessive,
)


def forms_of(lemma: str) -> set[str]:
    return {f.form for f in noun_forms(lemma)}


def form_for(lemma: str, **feats) -> set[str]:
    """All generated strings matching a feature subset."""
    want = {k.replace("__", "["): v for k, v in feats.items()}
    out = set()
    for f in noun_forms(lemma):
        if all(f.feats.get(k) == v for k, v in want.items()):
            out.add(f.form)
    return out


# --- the 3x3 locative grid, which must be exactly right -----------------------

@pytest.mark.parametrize(
    "lemma,case,expected",
    [
        # interior
        ("ház", "Ine", "házban"), ("ház", "Ill", "házba"), ("ház", "Ela", "házból"),
        ("kert", "Ine", "kertben"), ("kert", "Ill", "kertbe"), ("kert", "Ela", "kertből"),
        ("köd", "Ine", "ködben"), ("köd", "Ill", "ködbe"), ("köd", "Ela", "ködből"),
        # surface (whither / whence; `where` is superessive, tested separately)
        ("ház", "Sbl", "házra"), ("ház", "Del", "házról"),
        ("kert", "Sbl", "kertre"), ("kert", "Del", "kertről"),
        # proximity -- the allative is the three-form suffix
        ("ház", "Ade", "háznál"), ("ház", "All", "házhoz"), ("ház", "Abl", "háztól"),
        ("kert", "Ade", "kertnél"), ("kert", "All", "kerthez"), ("kert", "Abl", "kerttől"),
        ("köd", "All", "ködhöz"),
        ("gyümölcs", "All", "gyümölcshöz"),
        # neutral vowels must not flip harmony
        ("papír", "Ine", "papírban"), ("papír", "All", "papírhoz"),
        # antiharmonic
        ("híd", "Ine", "hídban"), ("víz", "Ine", "vízban"),
        # vowel-final stems lengthen
        ("alma", "Ine", "almában"), ("alma", "All", "almához"),
        ("epe", "Ine", "epében"),
    ],
)
def test_simple_cases(lemma, case, expected):
    got = simple_case(lemma, classify(lemma), case)
    assert [f.form for f in got] == [expected]


def test_superessive():
    assert superessive("asztal", classify("asztal"))[0].form == "asztalon"
    assert superessive("kert", classify("kert"))[0].form == "kerten"
    assert superessive("gyümölcs", classify("gyümölcs"))[0].form == "gyümölcsön"
    # After a vowel it is a bare -n, with lengthening.
    assert superessive("kocsi", classify("kocsi"))[0].form == "kocsin"
    assert superessive("alma", classify("alma"))[0].form == "almán"


def test_non_harmonising_suffixes_are_identical_in_both_classes():
    for lemma in ("ház", "kert", "köd"):
        assert simple_case(lemma, classify(lemma), "Ter")[0].form == lemma + "ig"
        assert simple_case(lemma, classify(lemma), "Cau")[0].form == lemma + "ért"


# --- accusative: the leaky one ------------------------------------------------

def test_accusative_after_vowel_lengthens():
    assert [f.form for f in accusative("alma", classify("alma"))] == ["almát"]
    assert [f.form for f in accusative("kocsi", classify("kocsi"))] == ["kocsit"]


def test_accusative_lexical_exceptions_are_recoverable():
    """`ház` ends in `z`, which the rule says takes a bare `-t`. It does not.

    The known-exception list must produce `házat` outright.
    """
    assert "házat" in form_for("ház", Case="Acc")


def test_accusative_emits_both_candidates_when_the_rule_leaks():
    """Where the linking vowel is lexically conditioned, both candidates are
    generated but only one is confident.

    Over-generation is deliberate and is not free: `vonatat` is not a real word,
    so a bare "is this string in the table?" check would wrongly accept a
    learner's error. That is exactly why `confidence` exists — the morpheme diff
    treats a LOW match as "recognised, but this is a generated guess, and the
    attested form is the MEDIUM one", which is how it can say *what* you got
    wrong instead of just *that* you did.
    """
    got = accusative("vonat", classify("vonat"))
    assert {f.form for f in got} == {"vonatot", "vonatat"}
    confident = [f.form for f in got if f.confidence is not Confidence.LOW]
    assert confident == ["vonatot"], "the real form must be the confident one"

    got = accusative("kanál", classify("kanál"))
    assert {f.form for f in got} == {"kanált", "kanálot"}
    confident = [f for f in got if f.confidence is not Confidence.LOW]
    assert [f.form for f in confident] == ["kanált"]


# --- plural -------------------------------------------------------------------

@pytest.mark.parametrize(
    "lemma,expected",
    [
        ("ház", "házak"),
        ("kert", "kertek"),
        ("könyv", "könyvek"),
        ("tükör", "tükörök"),
        ("alma", "almák"),
        ("kocsi", "kocsik"),
    ],
)
def test_plural(lemma, expected):
    assert expected in {f.form for f in plural(lemma, classify(lemma))}


def test_plural_flips_the_next_suffix():
    """The regression from the harmony module, end to end through inflection."""
    assert simple_case("könyv", classify("könyv"), "All")[0].form == "könyvhöz"
    assert simple_case("könyvek", classify("könyvek"), "All")[0].form == "könyvekhez"


# --- instrumental: v-assimilation --------------------------------------------

@pytest.mark.parametrize(
    "lemma,expected",
    [
        ("vonat", "vonattal"),
        ("kés", "késsel"),
        ("busz", "busszal"),      # digraph doubles as a unit, not `szsz`
        ("alma", "almával"),      # after a vowel the v survives
        ("könyv", "könyvvel"),
    ],
)
def test_instrumental_assimilation(lemma, expected):
    assert instrumental(lemma, classify(lemma))[0].form == expected


# --- possessives --------------------------------------------------------------

def test_possessives_consonant_final():
    """The possessive linking vowel follows the same lexical a/o split as the
    plural: `ház` gives `házam`, not the harmony-predicted `házom`."""
    got = {f.form for f in possessives("ház", classify("ház"))}
    for expected in ("házam", "házad", "háza", "házunk", "házatok", "házuk"):
        assert expected in got, f"missing {expected} in {sorted(got)}"

    # A stem outside the exception list keeps the default vowel.
    got = {f.form for f in possessives("vonat", classify("vonat"))}
    for expected in ("vonatom", "vonatod", "vonatunk"):
        assert expected in got, f"missing {expected} in {sorted(got)}"


def test_possessives_third_person_ja_variant_is_generated():
    """`barát` -> `barátja`, not `baráta`. Lexically conditioned, so both exist."""
    got = {f.form for f in possessives("barát", classify("barát"))}
    assert "barátja" in got


def test_possessives_after_vowel():
    got = {f.form for f in possessives("alma", classify("alma"))}
    assert "almám" in got   # not `almaom`
    assert "almája" in got  # -ja after a vowel


# --- whole paradigm -----------------------------------------------------------

def test_noun_forms_are_deduplicated():
    forms = noun_forms("ház")
    keys = [(f.form, f.feats_key) for f in forms]
    assert len(keys) == len(set(keys)), "duplicate (form, feats) pairs generated"


def test_noun_forms_cover_the_grid():
    forms = noun_forms("ház")
    cases = {f.feats.get("Case") for f in forms}
    for case in ("Nom", "Acc", "Ine", "Ill", "Ela", "Sup", "Sbl", "Del", "Ade", "All", "Abl", "Dat"):
        assert case in cases, f"{case} missing from generated paradigm"


def test_paradigm_omits_low_confidence_rather_than_guessing():
    """Production must never assert a form we are unsure of."""
    p = paradigm("kanál")
    # The rule-following accusative is shown; the coin-flip alternative is not.
    assert p["Acc.Sing"] == "kanált"
    assert "kanálot" not in p.values()


def test_paradigm_is_display_ready():
    p = paradigm("ház")
    assert p["Nom.Sing"] == "ház"
    assert p["Ine.Sing"] == "házban"
    assert p["All.Sing"] == "házhoz"
    assert p["Poss.1Sing"] == "házam"
    assert p["PossPl.1Sing"] == "házaim"  # my houseS is a different cell


def test_front_rounded_linking_vowel_is_lexical():
    """Regression: harmony alone predicts `könyvök`. The real plural is `könyvek`.

    Note that the same stem still takes `-höz` in the allative — the plural
    linking vowel and the suffix allomorph genuinely disagree, so no single
    harmony class can produce both.
    """
    assert paradigm("könyv")["Nom.Plur"] == "könyvek"
    assert paradigm("könyv")["All.Sing"] == "könyvhöz"
    # An unlisted front-rounded stem keeps the harmony-predicted vowel.
    assert paradigm("köd")["Nom.Plur"] == "ködök"
    # ...but the alternative is still generated, so recognition survives.
    assert "ködek" in forms_of("köd")


def test_doubled_digraph_orthography():
    """Regression: `busz` + `-val` is `busszal`, not `buszszal`.

    A doubled digraph is written first-letter + digraph.
    """
    assert instrumental("busz", classify("busz"))[0].form == "busszal"
    assert instrumental("meggy", classify("meggy"))[0].form == "meggyel"
    assert instrumental("rossz", classify("rossz"))[0].form == "rosszal"


def test_possessive_and_case_stack():
    """The product thesis, concretely.

    `házaimban` is stem + linking + plural-possessum -i- + 1sg possessor +
    inessive. Five morphemes, four of them separately learnable. A model that
    treats a wordform as an atom cannot represent this at all.
    """
    forms = forms_of("ház")
    for expected in ("házam", "házaim", "házamban", "házaimban", "házaimhoz"):
        assert expected in forms, f"missing {expected}"

    forms = forms_of("könyv")
    for expected in ("könyvem", "könyveim", "könyveink", "könyveinket", "könyvemben"):
        assert expected in forms, f"missing {expected}"


def test_stacked_forms_carry_all_their_features():
    """A stacked form must report the possessor AND the case, or the second
    state space cannot credit both."""
    match = [f for f in noun_forms("ház") if f.form == "házaimban"]
    assert match, "házaimban not generated"
    feats = match[0].feats
    assert feats["Case"] == "Ine"
    assert feats["Person[psor]"] == "1"
    assert feats["Number[psed]"] == "Plur"


def test_paradigm_stays_readable():
    """The display table is a table, not the whole lookup index."""
    p = paradigm("ház")
    assert 20 <= len(p) <= 40, f"{len(p)} cells is not a readable paradigm"
    assert all("." in slot for slot in p)


def test_instrumental_stacks_with_assimilation():
    """`barátaim` + `-val` -> `barátaimmal`: the v assimilates to the possessive
    ending, not to the lemma's final consonant."""
    assert "barátaimmal" in forms_of("barát")
    assert "házammal" in forms_of("ház")


def test_accusative_of_a_derived_stem_uses_the_low_vowel():
    """After a plural or possessive ending the accusative takes a/e/ö.

    Regression: the display paradigm showed `házakot` for the accusative plural.
    The correct form is `házakat` — a derived stem is not lexically conditioned,
    so there is exactly one right answer and no low-confidence alternative.
    """
    p = paradigm("ház")
    assert p["Acc.Plur"] == "házakat"
    assert paradigm("könyv")["Acc.Plur"] == "könyveket"

    forms = forms_of("ház")
    for expected in ("házamat", "házaimat", "házakat"):
        assert expected in forms, f"missing {expected}"
    for wrong in ("házakot", "házamot"):
        assert wrong not in forms, f"generated the wrong form {wrong}"
