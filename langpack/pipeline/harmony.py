"""Hungarian vowel harmony.

Harmony is the first thing a learner must automate and the single most frequent
source of suffix errors, so it gets its own module with its own tests rather than
being inlined into the inflection code.

The rule looks simple and is not:

  * Back vowels:            a á o ó u ú
  * Front rounded vowels:   ö ő ü ű
  * Front unrounded vowels: e é i í   -- these are NEUTRAL, which is the catch.

A word takes back suffixes if its harmony is decided by a back vowel, front
suffixes otherwise. Neutral vowels do not decide anything on their own: `papír`
(back a + neutral í) takes `-ban`, not `-ben`, because the `a` decides. Only when
a word contains *nothing but* neutral vowels does it default to front (`víz` ->
`vízben`).

Three-way suffixes (`-hoz/-hez/-höz`) additionally need to know whether a front
word is rounded, which is decided by the last front vowel.

Reference: docs/PEDAGOGY.md and .claude/research/15-hungarian-difficulty.md.
"""

from __future__ import annotations

from enum import Enum

BACK_VOWELS = frozenset("aáoóuú")
FRONT_ROUNDED_VOWELS = frozenset("öőüű")
FRONT_UNROUNDED_VOWELS = frozenset("eéií")
VOWELS = BACK_VOWELS | FRONT_ROUNDED_VOWELS | FRONT_UNROUNDED_VOWELS

# Transparent to the ROUNDING harmony: `i`/`í` do not decide whether a front word
# takes `-hez` or `-höz`, so we look past them to the vowel that does. `e`/`é` are
# NOT transparent here -- they select the unrounded form. This is the rule as it
# is actually taught, and it is a simplification of the full phonological
# picture; corpus-attested forms in the pack always override generated ones.
ROUNDING_TRANSPARENT = frozenset("ií")

# Native words that look front but take back suffixes. This is a genuinely
# closed, memorisable list -- learners are taught it as a list, so we store it as
# one. Far from exhaustive; the pack's corpus-attested forms override anything
# here, and this only governs generated forms.
ANTIHARMONIC = frozenset(
    """
    híd cél derék férj héj nyíl sír szíj víz zsír
    cím ír híg nyit szid
    """.split()
)


class Harmony(str, Enum):
    """Which suffix allomorph a stem selects."""

    BACK = "back"
    FRONT_UNROUNDED = "front_unrounded"
    FRONT_ROUNDED = "front_rounded"

    @property
    def is_front(self) -> bool:
        return self is not Harmony.BACK


def vowels_of(word: str) -> list[str]:
    """Return the vowels of a word, in order."""
    return [c for c in word.lower() if c in VOWELS]


def classify(word: str) -> Harmony:
    """Classify a stem's harmony class.

    The algorithm, in the order the exceptions actually apply:

    1. A known antiharmonic stem takes back suffixes despite front vowels.
    2. Otherwise, if any back vowel is present, the word is back. (Neutral
       vowels never override a back vowel: `papír` -> `-ban`.)
    3. Otherwise the word is front. Rounded or unrounded is decided by the LAST
       front vowel that is not transparent -- the one nearest the suffix. This
       is why `könyv` takes `-höz` but its plural `könyvek` takes `-hez`: the
       `e` is now the vowel adjacent to the suffix and it is unrounded.
    4. A word with no vowels at all (an acronym, a foreign fragment) defaults to
       front unrounded, which is the less jarring guess.
    """
    lowered = word.lower()

    if lowered in ANTIHARMONIC:
        return Harmony.BACK

    vs = vowels_of(lowered)
    if not vs:
        return Harmony.FRONT_UNROUNDED

    if any(v in BACK_VOWELS for v in vs):
        return Harmony.BACK

    for v in reversed(vs):
        if v in ROUNDING_TRANSPARENT:
            continue
        return Harmony.FRONT_ROUNDED if v in FRONT_ROUNDED_VOWELS else Harmony.FRONT_UNROUNDED
    # Nothing but transparent vowels (e.g. `ing`): unrounded.
    return Harmony.FRONT_UNROUNDED


def pick(harmony: Harmony, back: str, front_unrounded: str, front_rounded: str | None = None) -> str:
    """Select the allomorph for a harmony class.

    Two-form suffixes (`-ban/-ben`) pass only two arguments; the front-rounded
    class then shares the front-unrounded form, which is exactly how Hungarian
    behaves: `köd` -> `ködben`, not a third variant.
    """
    if harmony is Harmony.BACK:
        return back
    if harmony is Harmony.FRONT_ROUNDED and front_rounded is not None:
        return front_rounded
    return front_unrounded


def ends_in_vowel(word: str) -> bool:
    return bool(word) and word[-1].lower() in VOWELS


def lengthen_final_vowel(word: str) -> str:
    """Apply low-vowel lengthening: `a` -> `á`, `e` -> `é` before a suffix.

    `alma` + `-t` -> `almát`, `epe` + `-t` -> `epét`. This applies to stem-final
    `a`/`e` only, and it is one of the first things learners get wrong because
    the change is invisible in speech if you have no vowel-length contrast in
    your L1 -- which is the case for Russian and English speakers. See
    .claude/research/07-hvpt.md.
    """
    if not word:
        return word
    if word[-1] == "a":
        return word[:-1] + "á"
    if word[-1] == "e":
        return word[:-1] + "é"
    return word
