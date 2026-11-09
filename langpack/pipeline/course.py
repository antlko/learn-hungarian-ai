"""The guided course: aggregation and validation.

The grammar DAG (curriculum.py) fixes the order rules are learned in. It says
nothing about WORDS — and a learner told to "mine your own sentences" on day one
has been handed a research project, not a course. [R11] is explicit that reading
volume complements explicit instruction rather than replacing it, and volume
needs a vocabulary floor to stand on.

So the pack also ships a syllabus in three FINITE tiers (ADR-057): 24 basic units
(course_m1..course_m6, the original six months), 20 intermediate (course_m7_12) and
20 advanced (course_m13_18). Each unit names the real errand it buys you, the
grammar it leans on, and the words to own. The learner picks their own pace, so
`month` is only an internal ordering/checkpoint field; `tier` is what they see.
This module only joins the files and refuses the joins that would break.

After the last unit of the last tier the app stops prescribing and hands over to
resources.py. That hand-off is the design, not an afterthought: a course that
never ends produces a learner who cannot read anything that was not made for them.
"""

from __future__ import annotations

from . import (
    course_growth,
    course_m1,
    course_m2,
    course_m3,
    course_m4,
    course_m5,
    course_m6,
    course_m7_12,
    course_m13_18,
    course_themes,
)

MONTHS = (course_m1, course_m2, course_m3, course_m4, course_m5, course_m6)
TIERS = ("basic", "intermediate", "advanced")
# Core units per tier; the course ends after their sum (ADR-057).
TIER_UNITS = {"basic": 24, "intermediate": 20, "advanced": 20}

# Core words are the month files' own plus the growth batch (course_growth.py),
# appended after them so a unit keeps its original opening words in order.
_RAW: list[dict] = [
    {**u, "track": "core", "tier": "basic",
     "words": list(u["words"]) + list(course_growth.GROWTH.get(u["code"], []))}
    for m in MONTHS for u in m.UNITS
] + [
    {**u, "track": "core"}  # these modules name their own `tier`
    for m in (course_m7_12, course_m13_18) for u in m.UNITS
]
_RAW.sort(key=lambda u: u["ord"])

# Optional side units (ADR-050). Authored apart from the path so they can grow
# without touching it, and never decide `current`/`handedOver`. They carry the
# basic tier (the vocabulary they belong to is the first six months').
_THEMES: list[dict] = sorted(
    ({**u, "track": "theme", "tier": "basic"} for u in course_themes.UNITS),
    key=lambda u: u["ord"],
)

CORE_UNITS = sum(TIER_UNITS.values())  # 64
MIN_WORDS, MAX_WORDS = 15, 60
# Share of course words that must have a real sentence in the starter pack.
CLOZEABLE_FLOOR = 0.90


def _dedup(
    units: list[dict], seen: dict[str, str] | None = None
) -> tuple[list[dict], list[tuple[str, str, str]]]:
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
    seen = {} if seen is None else seen
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


_CORE, _CORE_DUP = _dedup(_RAW)
# Themes dedup against everything the path already teaches: a word is owned by
# the core unit that introduced it, and the side unit simply lacks it.
_SEEN_CORE = {w[0]: u["code"] for u in _CORE for w in u["words"]}
_THEME, _THEME_DUP = _dedup(_THEMES, dict(_SEEN_CORE))

CORE_ONLY = _CORE
THEME_ONLY = _THEME
UNITS = _CORE + _THEME
DUPLICATES = _CORE_DUP + _THEME_DUP


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

        if unit.get("track") not in ("core", "theme"):
            problems.append(f"{code}: track must be 'core' or 'theme'")
        if unit.get("tier") not in TIERS:
            problems.append(f"{code}: tier must be one of {', '.join(TIERS)}")

        n = len(unit["words"])
        if not MIN_WORDS <= n <= MAX_WORDS:
            problems.append(
                f"{code}: {n} words — a unit is {MIN_WORDS}–{MAX_WORDS}, not a dictionary dump")

        for w in unit["words"]:
            lemma = w[0]
            if lemma not in known_lemmas:
                problems.append(f"{code}: {lemma} is not in the pack — unteachable")
            if not str(w[2]).strip() or not str(w[3]).strip():
                problems.append(f"{code}: {lemma} is missing a gloss in one locale")

    core = [u for u in UNITS if u["track"] == "core"]
    themes = [u for u in UNITS if u["track"] == "theme"]
    if len(core) != CORE_UNITS:
        problems.append(f"the course has {len(core)} core units, not {CORE_UNITS}: it ENDS")
    for tier, want in TIER_UNITS.items():
        got = sum(1 for u in core if u.get("tier") == tier)
        if got != want:
            problems.append(f"the {tier} tier has {got} core units, not {want}: it ENDS")
    # Tiers are consecutive in `ord`, so "the next unit" never jumps a tier.
    tier_rank = {t: i for i, t in enumerate(TIERS)}
    by_ord = sorted(core, key=lambda u: u["ord"])
    ranks = [tier_rank.get(u.get("tier"), 99) for u in by_ord]
    if ranks != sorted(ranks):
        problems.append("core units are not ordered basic -> intermediate -> advanced")
    if core and themes and min(u["ord"] for u in themes) <= max(u["ord"] for u in core):
        problems.append("a theme unit sorts before the end of the core path")

    # Duplicates are resolved by _dedup rather than reported as errors — see its
    # docstring. What IS an error is a unit that dedup left too thin to teach.
    return problems


def unclozeable(conn) -> list[str]:
    """Course lemmas with no usable sentence in `conn` (a full or starter pack).

    Mirrors the app's enrolment (`noteFromSentences`): a sentence of three or
    more tokens whose token is tagged with the lemma's id, or whose surface form
    is an attested wordform of the lemma. The app additionally re-analyses the
    token, which this does not, so it is a slight over-estimate; the floor
    leaves room for that.
    """
    missing = []
    for lemma in sorted({w[0] for u in UNITS for w in u["words"]}):
        row = conn.execute(
            """
            SELECT 1 FROM sentence s
              JOIN sentence_token t ON t.sentence_id = s.id
             WHERE s.len_tokens >= 3 AND (
                   t.lemma_id IN (SELECT id FROM lemma WHERE lemma = ?)
                OR t.form IN (SELECT w.form FROM wordform w
                                JOIN lemma l ON l.id = w.lemma_id
                               WHERE l.lemma = ? AND w.source <> 'generated'))
             LIMIT 1""",
            (lemma, lemma),
        ).fetchone()
        if row is None:
            missing.append(lemma)
    return missing


def warn_unclozeable(conn, say=print) -> list[str]:
    """Report, never fail, the share of course words with no usable sentence.

    A full pack legitimately has a lower sentence coverage per word than the
    starter (which was curated against this very course), so the floor is
    enforced by the starter's test and only WARNED about here.
    """
    total = len({w[0] for u in UNITS for w in u["words"]})
    missing = unclozeable(conn)
    share = 1 - len(missing) / total if total else 1.0
    if share < CLOZEABLE_FLOOR:
        say(f"  ! only {share:.1%} of {total} course words have a sentence to build a "
            f"cloze from ({len(missing)} without; floor {CLOZEABLE_FLOOR:.0%}). "
            f"First: {', '.join(missing[:10])}")
    else:
        say(f"  course words with a sentence: {share:.1%} "
            f"({len(missing)} of {total} without)")
    return missing


def stats() -> dict:
    return {
        "units": len(UNITS),
        "core_units": len(_CORE),
        "theme_units": len(_THEME),
        "words": sum(len(u["words"]) for u in UNITS),
        "core_words": sum(len(u["words"]) for u in _CORE),
        "tier_units": {t: sum(1 for u in _CORE if u["tier"] == t) for t in TIERS},
        "tier_words": {t: sum(len(u["words"]) for u in _CORE if u["tier"] == t)
                       for t in TIERS},
        "theme_words": sum(len(u["words"]) for u in _THEME),
        "months": len({u["month"] for u in _CORE}),
        "deduped": len(DUPLICATES),
    }
