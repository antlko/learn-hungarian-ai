"""The guided six-month course: aggregation and validation.

The grammar DAG (curriculum.py) fixes the order rules are learned in. It says
nothing about WORDS — and a learner told to "mine your own sentences" on day one
has been handed a research project, not a course. [R11] is explicit that reading
volume complements explicit instruction rather than replacing it, and volume
needs a vocabulary floor to stand on.

So the pack also ships a syllabus: twenty-four ordered units across six months,
each naming the real errand it buys you, the grammar it leans on, and the words
to own. The content lives in course_m1..course_m6, one module per month, so the
months can be written and argued about independently. This module only joins
them and refuses the joins that would break.

After the last unit the app stops prescribing and hands over to resources.py.
That hand-off is the design, not an afterthought: a course that never ends
produces a learner who cannot read anything that was not made for them.
"""

from __future__ import annotations

from . import (
    course_m1,
    course_m2,
    course_m3,
    course_m4,
    course_m5,
    course_m6,
)

MONTHS = (course_m1, course_m2, course_m3, course_m4, course_m5, course_m6)

_RAW: list[dict] = [u for m in MONTHS for u in m.UNITS]
_RAW.sort(key=lambda u: u["ord"])


def _dedup(units: list[dict]) -> tuple[list[dict], list[tuple[str, str, str]]]:
    """Keep the first teaching of a word and drop the later ones.

    The months are authored independently — that is what makes them arguable
    one at a time — so overlap between them is normal rather than a mistake:
    `kér` belongs in month 1's survival phrases AND in month 2's shop. Resolving
    it by hand across six files would be a merge conflict that never ends.

    First occurrence wins because the syllabus is ordered: a word is introduced
    where it is first needed, and meeting it again later is the review deck's
    job, not the plan's. The dropped pairs are returned rather than swallowed so
    the build can print them — a silent drop would make a unit quietly shorter
    than its author intended.
    """
    seen: dict[str, str] = {}
    dropped: list[tuple[str, str, str]] = []
    out: list[dict] = []
    for unit in units:
        kept = []
        for w in unit["words"]:
            lemma = w[0]
            if lemma in seen:
                dropped.append((unit["code"], lemma, seen[lemma]))
                continue
            seen[lemma] = unit["code"]
            kept.append(w)
        out.append({**unit, "words": kept})
    return out, dropped


UNITS, DUPLICATES = _dedup(_RAW)


def words() -> list[tuple]:
    """Flatten to (unit_code, lemma, upos, gloss_ru, gloss_en, note, ord) rows."""
    out = []
    for unit in UNITS:
        for i, w in enumerate(unit["words"]):
            lemma, upos, gloss_ru, gloss_en, note = (list(w) + ["", "", "", ""])[:5]
            out.append((unit["code"], lemma, upos, gloss_ru, gloss_en, note, i))
    return out


def validate(known_lemmas: set[str], topic_codes: set[str]) -> list[str]:
    """Structural checks. Every one of these has bitten a real content edit.

    A course that references a word the pack cannot look up, or a grammar topic
    that does not exist, fails at the worst possible moment: in front of the
    learner, on the screen that is supposed to be the plan.
    """
    problems: list[str] = []
    seen_codes: set[str] = set()
    seen_ord: dict[int, str] = {}

    for unit in UNITS:
        code = unit["code"]
        if code in seen_codes:
            problems.append(f"duplicate unit code: {code}")
        seen_codes.add(code)

        if unit["ord"] in seen_ord:
            problems.append(
                f"{code}: ord {unit['ord']} collides with {seen_ord[unit['ord']]}"
            )
        seen_ord[unit["ord"]] = code

        for field in ("title_ru", "title_en", "goal_ru", "goal_en",
                      "situation_ru", "situation_en"):
            if not str(unit.get(field, "")).strip():
                problems.append(f"{code}: missing {field}")

        for tc in filter(None, (t.strip() for t in unit.get("topic_codes", "").split(","))):
            if tc not in topic_codes:
                problems.append(f"{code}: unknown grammar topic {tc}")

        n = len(unit["words"])
        if not 15 <= n <= 60:
            problems.append(f"{code}: {n} words — a unit is 15–60, not a dictionary dump")

        for w in unit["words"]:
            lemma = w[0]
            if lemma not in known_lemmas:
                problems.append(f"{code}: {lemma} is not in the pack — unteachable")
            if not str(w[2]).strip() or not str(w[3]).strip():
                problems.append(f"{code}: {lemma} is missing a gloss in one locale")

    # Duplicates are resolved by _dedup rather than reported as errors — see its
    # docstring. What IS an error is a unit that dedup left too thin to teach.
    return problems


def stats() -> dict:
    return {
        "units": len(UNITS),
        "words": sum(len(u["words"]) for u in UNITS),
        "months": len({u["month"] for u in UNITS}),
        "deduped": len(DUPLICATES),
    }
