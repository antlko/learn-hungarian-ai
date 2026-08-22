import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from pipeline.curriculum import TOPICS, validate
from pipeline.features import by_id


def test_curriculum_is_valid():
    assert not validate(set(by_id()))


def test_prerequisites_form_a_dag_reachable_from_the_root():
    """Every topic must be reachable by following prerequisites from a root."""
    index = {t.code: t for t in TOPICS}
    roots = [t for t in TOPICS if not t.prereq_codes]
    assert roots, "the curriculum has no entry point"

    seen: set[str] = set()

    def walk(code: str, stack: tuple[str, ...] = ()) -> None:
        assert code not in stack, f"cycle: {' -> '.join(stack + (code,))}"
        if code in seen:
            return
        seen.add(code)
        for t in TOPICS:
            if code in t.prereq_codes:
                walk(t.code, stack + (code,))

    for r in roots:
        walk(r.code)
    assert seen == set(index), f"unreachable topics: {sorted(set(index) - seen)}"


def test_ordering_matches_the_difficulty_map():
    order = {t.code: t.teach_order for t in TOPICS}
    # Harmony before any case work.
    assert order["vowel-harmony"] < order["locative-grid"]
    # Politeness in month one — before the accusative.
    assert order["politeness"] < order["accusative"]
    # The definite conjugation before the conditional and the imperative.
    assert order["definite-conjugation"] < order["conditional"]
    assert order["definite-conjugation"] < order["imperative"]
    # Focus and word order last among grammar topics.
    assert order["focus-word-order"] == max(
        t.teach_order for t in TOPICS if t.code != "register"
    )


def test_definite_conjugation_explains_the_error_distribution():
    """The topic a learner fails most must say what the failure looks like."""
    topic = next(t for t in TOPICS if t.code == "definite-conjugation")
    for text in (topic.explain_ru, topic.explain_en):
        assert "55" in text and "25" in text, "error distribution missing"
    assert "80" in topic.explain_ru, "the blocked->interleaved gate is not explained"


def test_levels_are_monotonic_in_teach_order():
    rank = {"A1": 0, "A2": 1, "B1": 2, "B2": 3}
    ordered = sorted(TOPICS, key=lambda t: t.teach_order)
    seen_max = -1
    for t in ordered:
        assert rank[t.level] >= seen_max, f"{t.code} ({t.level}) comes after a higher level"
        seen_max = max(seen_max, rank[t.level])
