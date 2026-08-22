import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from pipeline.features import ALL_FEATURES, CASES, by_id, validate


def test_inventory_is_self_consistent():
    problems = validate()
    assert not problems, "\n".join(problems)


def test_inventory_is_substantial():
    # The design claims roughly 120 teachable atoms once allomorph variants are
    # merged. Guard the order of magnitude so a botched edit is obvious.
    assert len(ALL_FEATURES) >= 45, f"only {len(ALL_FEATURES)} features"


def test_locative_grid_is_complete():
    """The 3x3 grid is a UI component, so all nine cells must exist."""
    cells = {f.grid for f in CASES if f.grid}
    expected = {
        f"{plane}/{direction}"
        for plane in ("interior", "surface", "proximity")
        for direction in ("where", "whither", "whence")
    }
    assert cells == expected, f"missing: {expected - cells}"


def test_harmony_ordering_matches_the_difficulty_map():
    """Vowel harmony is first; focus/word order is last."""
    index = by_id()
    assert index["Skill=VowelHarmony"].teach_order < index["Case=Ine"].teach_order
    assert index["Skill=PreverbDetach"].teach_order == max(f.teach_order for f in ALL_FEATURES)
    # Politeness lands in month one because the social cost outranks the grammar.
    assert index["Skill=Politeness"].teach_order < index["Case=Acc"].teach_order


def test_definite_conjugation_is_introduced_before_conditional():
    index = by_id()
    assert index["Definite=Def"].teach_order < index["Mood=Cnd"].teach_order
    assert index["Definite=Def"].teach_order < index["Mood=Imp"].teach_order


def test_three_form_suffixes_really_have_three_forms():
    index = by_id()
    allative = index["Case=All"].allomorphs
    assert allative == {"back": "hoz", "front_unrounded": "hez", "front_rounded": "höz"}
    # Two-form suffixes share the front variant, which is correct Hungarian.
    inessive = index["Case=Ine"].allomorphs
    assert inessive["front_unrounded"] == inessive["front_rounded"] == "ben"


def test_non_harmonising_suffixes_are_flat():
    index = by_id()
    for fid in ("Case=Ter", "Case=Tem", "Case=Cau"):
        forms = set(index[fid].allomorphs.values())
        assert len(forms) == 1, f"{fid} should not harmonise, got {forms}"
