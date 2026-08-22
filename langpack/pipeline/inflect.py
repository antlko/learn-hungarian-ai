"""Generate Hungarian wordforms from a lemma.

Two different jobs with different tolerances, and conflating them is the trap:

* **Recognition** (`form -> lemma + features`) is what the runtime lookup does.
  Here *recall* dominates. If we are unsure whether a stem takes `-ok` or `-ak`,
  emitting both is harmless: the reader still resolves `házak` correctly, and the
  frequency prior settles genuine ambiguity.

* **Production** (showing the learner "the accusative is `házat`") is what the
  paradigm table does. Here *precision* dominates, and a wrong form is actively
  taught. So the paradigm only ever contains high-confidence forms.

Every generated row carries a `Confidence`, and `source` distinguishes generated
forms from corpus- or Wiktionary-attested ones. Attested always wins at merge
time -- see docs/LANGPACK.md.

Scope: this covers the productive nominal paradigm. Verb conjugation, which needs
the definite/indefinite split and far more stem classes, lands with the grammar
trainer in Phase 5.
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import Enum

from .harmony import (
    VOWELS,
    Harmony,
    classify,
    ends_in_vowel,
    lengthen_final_vowel,
    pick,
)


class Confidence(str, Enum):
    """How much we trust a generated form."""

    HIGH = "high"      # a fully regular rule with no lexical conditioning
    MEDIUM = "medium"  # regular, but the stem class is inferred
    LOW = "low"        # a coin-flip we resolve by emitting both candidates


@dataclass(frozen=True)
class Form:
    form: str
    feats: dict[str, str]
    confidence: Confidence

    @property
    def feats_key(self) -> str:
        """UD-style rendering, sorted so it is stable across runs."""
        return "|".join(f"{k}={v}" for k, v in sorted(self.feats.items()))


# ---------------------------------------------------------------------------
# Linking vowels
# ---------------------------------------------------------------------------

# Consonants after which the accusative is normally a bare `-t`. This is the rule
# as taught, and it is genuinely leaky -- `ház` ends in `z` and still takes
# `házat`. Rather than pretend the rule is exceptionless we emit both candidates
# at LOW confidence for the leaky cases, which costs nothing for recognition.
BARE_T_FINALS = ("j", "l", "ly", "n", "ny", "r", "s", "sz", "z", "zs")

# The linking vowel is lexically conditioned in BOTH harmony classes, and this
# is one of the places a rule-only generator quietly produces wrong Hungarian:
#
#   back stems:          ház -> házak   but  vonat -> vonatok
#   front-rounded stems: köd -> ködök   but  könyv -> könyvek
#
# Note that `könyv` still takes `-höz` in the allative, so the plural linking
# vowel and the suffix allomorph genuinely disagree — harmony alone cannot
# predict it. High-frequency exceptions are listed; everything else gets both
# candidates at reduced confidence.
KNOWN_A_STEMS = frozenset(
    """
    ház híd nyár tár ág ágy hal fal fog has láb nyak vár had lyuk fiú
    darab falu álom szám tag part
    """.split()
)

# Front-rounded stems whose linking vowel is `e`, not `ö`.
KNOWN_E_STEMS = frozenset(
    """
    könyv föld zöld völgy tölgy csönd bölcs ölt öv ök
    öt öl ötlet öröm környék özvegy szövet
    """.split()
)


def _linking_vowel(stem: str, harmony: Harmony) -> str:
    """The vowel inserted between a consonant-final stem and a consonantal suffix."""
    lowered = stem.lower()
    if harmony is Harmony.BACK:
        return "a" if lowered in KNOWN_A_STEMS else "o"
    if harmony is Harmony.FRONT_ROUNDED:
        return "e" if lowered in KNOWN_E_STEMS else "ö"
    return "e"


def _linking_alternative(harmony: Harmony, chosen: str) -> str | None:
    """The other candidate, where the choice is genuinely lexical."""
    if harmony is Harmony.BACK:
        return "a" if chosen == "o" else "o"
    if harmony is Harmony.FRONT_ROUNDED:
        return "e" if chosen == "ö" else "ö"
    return None  # front unrounded has no alternative


DIGRAPHS = ("cs", "dz", "gy", "ly", "ny", "sz", "ty", "zs")


def _is_geminate_final(stem: str) -> bool:
    """True if the stem already ends in a doubled consonant.

    `meggy` ends in `ggy`, which is already the doubled form of `gy`, so
    assimilation must not double it again (`meggyel`, not `megggyel`). Same for
    `rossz` -> `rosszal`.
    """
    low = stem.lower()
    for d in DIGRAPHS:
        if low.endswith(d[0] + d):
            return True
    return len(low) >= 2 and low[-1] == low[-2] and low[-1] not in VOWELS


def _assimilate_v(stem: str, suffix_vowel: str) -> str:
    """Instrumental/translative `v` assimilates to a preceding consonant.

    `vonat` + `-val` -> `vonattal`; `kés` + `-vel` -> `késsel`. After a vowel the
    `v` survives: `alma` + `-val` -> `almával`.
    """
    if ends_in_vowel(stem):
        return lengthen_final_vowel(stem) + "v" + suffix_vowel + "l"
    # Already geminate: nothing to double.
    if _is_geminate_final(stem):
        return stem + suffix_vowel + "l"
    # Hungarian orthography writes a doubled digraph as first-letter + digraph:
    # `sz` doubles to `ssz`, not `szsz`. So `busz` + `-val` is `busszal`.
    for digraph in DIGRAPHS:
        if stem.lower().endswith(digraph):
            return stem[: -len(digraph)] + digraph[0] + digraph + suffix_vowel + "l"
    return stem + stem[-1] + suffix_vowel + "l"


# ---------------------------------------------------------------------------
# Individual case builders
# ---------------------------------------------------------------------------

def accusative(lemma: str, harmony: Harmony, *, linking_override: str | None = None) -> list[Form]:
    """The accusative.

    `linking_override` is used when suffixing an ALREADY-INFLECTED stem. After a
    plural or possessive ending the accusative takes the low vowel a/e/ö rather
    than the stem's own linking vowel: `házak` -> `házakat` (not `házakot`),
    `házam` -> `házamat`, `könyvek` -> `könyveket`.
    """
    feats = {"Case": "Acc", "Number": "Sing"}
    if ends_in_vowel(lemma):
        # alma -> almát, kocsi -> kocsit
        return [Form(lengthen_final_vowel(lemma) + "t", feats, Confidence.HIGH)]

    lowered = lemma.lower()
    takes_bare_t = any(lowered.endswith(c) for c in BARE_T_FINALS)
    linking = linking_override or _linking_vowel(lemma, harmony)

    if linking_override:
        # A derived stem is not lexically conditioned: there is one right answer.
        return [Form(lemma + linking + "t", feats, Confidence.MEDIUM)]

    if takes_bare_t and lowered not in KNOWN_A_STEMS and lowered not in KNOWN_E_STEMS:
        # Rule says bare -t, but the rule leaks. Emit the bare form confidently
        # and the linked form as a low-confidence fallback so recognition of a
        # real exception still succeeds.
        return [
            Form(lemma + "t", feats, Confidence.MEDIUM),
            Form(lemma + linking + "t", feats, Confidence.LOW),
        ]

    out = [Form(lemma + linking + "t", feats, Confidence.MEDIUM)]
    alt = _linking_alternative(harmony, linking)
    if alt:
        out.append(Form(lemma + alt + "t", feats, Confidence.LOW))
    return out


def plural(lemma: str, harmony: Harmony) -> list[Form]:
    feats = {"Number": "Plur", "Case": "Nom"}
    if ends_in_vowel(lemma):
        return [Form(lengthen_final_vowel(lemma) + "k", feats, Confidence.HIGH)]

    primary = _linking_vowel(lemma, harmony)
    out = [Form(lemma + primary + "k", feats, Confidence.MEDIUM)]
    # Emit the other candidate for recognition wherever the choice is lexical.
    alt = _linking_alternative(harmony, primary)
    if alt:
        out.append(Form(lemma + alt + "k", feats, Confidence.LOW))
    return out


def superessive(lemma: str, harmony: Harmony) -> list[Form]:
    """`-n` after a vowel, `-on/-en/-ön` after a consonant."""
    feats = {"Case": "Sup", "Number": "Sing"}
    if ends_in_vowel(lemma):
        return [Form(lengthen_final_vowel(lemma) + "n", feats, Confidence.HIGH)]
    linking = _linking_vowel(lemma, harmony)
    out = [Form(lemma + linking + "n", feats, Confidence.MEDIUM)]
    alt = _linking_alternative(harmony, linking)
    if alt:
        out.append(Form(lemma + alt + "n", feats, Confidence.LOW))
    return out


# Cases whose suffix simply attaches, with low-vowel lengthening after a vowel.
# These are the reliable ones -- and, not coincidentally, most of the 3x3 grid.
_SIMPLE_CASES: dict[str, tuple[str, str, str | None]] = {
    "Ine": ("ban", "ben", None),
    "Ill": ("ba", "be", None),
    "Ela": ("ból", "ből", None),
    "Sbl": ("ra", "re", None),
    "Del": ("ról", "ről", None),
    "Ade": ("nál", "nél", None),
    "All": ("hoz", "hez", "höz"),
    "Abl": ("tól", "től", None),
    "Dat": ("nak", "nek", None),
    "Ter": ("ig", "ig", None),
    "Cau": ("ért", "ért", None),
    "Ess": ("ként", "ként", None),
}


def simple_case(lemma: str, harmony: Harmony, case: str) -> list[Form]:
    back, front, rounded = _SIMPLE_CASES[case]
    suffix = pick(harmony, back, front, rounded)
    stem = lengthen_final_vowel(lemma) if ends_in_vowel(lemma) else lemma
    return [Form(stem + suffix, {"Case": case, "Number": "Sing"}, Confidence.HIGH)]


def instrumental(lemma: str, harmony: Harmony) -> list[Form]:
    vowel = pick(harmony, "a", "e")
    return [Form(_assimilate_v(lemma, vowel), {"Case": "Ins", "Number": "Sing"}, Confidence.MEDIUM)]


# ---------------------------------------------------------------------------
# Possessives
# ---------------------------------------------------------------------------

# Possessive endings split into (linking-vowel?, consonantal part).
#
# Only some of them take the ordinary linking vowel, and getting this wrong
# produces `házom` for `ház` where the real form is `házam` — the same lexical
# a/o split that governs the plural. The 1pl and 3pl endings use u/ü regardless,
# and the 3sg is a bare a/e that is not a linking vowel at all.
_POSSESSIVE: list[tuple[str, str, bool, tuple[str, str, str]]] = [
    # (Person[psor], Number[psor], uses_linking_vowel, (back, front_unr, front_rnd))
    ("1", "Sing", True,  ("m", "m", "m")),
    ("2", "Sing", True,  ("d", "d", "d")),
    ("3", "Sing", False, ("a", "e", "e")),
    ("1", "Plur", False, ("unk", "ünk", "ünk")),
    ("2", "Plur", True,  ("tok", "tek", "tök")),
    ("3", "Plur", False, ("uk", "ük", "ük")),
]


# Plural-possessum endings: "my houseS" rather than "my house". The `-i-` infix
# is what marks the plural of the thing possessed, and it sits between the stem
# and the personal ending. This is the layer that makes forms like `házaimban`
# and `könyveinket` possible, and it is exactly the stacking that makes a
# wordform-as-atom model fail for Hungarian.
_POSSESSUM_PLURAL: list[tuple[str, str, str]] = [
    # (Person[psor], Number[psor], ending after the -i- infix)
    ("1", "Sing", "m"),
    ("2", "Sing", "d"),
    ("3", "Sing", ""),
    ("1", "Plur", "nk"),
    ("2", "Plur", "tok"),
    ("3", "Plur", "k"),
]


def possessives_plural_possessum(lemma: str, harmony: Harmony) -> list[Form]:
    """`ház` -> `házaim` (my houses), `könyv` -> `könyveim`.

    Structure: stem + `-ai-`/`-ei-` + personal ending.
    """
    out: list[Form] = []
    if ends_in_vowel(lemma):
        stem = lengthen_final_vowel(lemma) + "i"
    else:
        # The plural-possessum infix is -ai-/-ei- and does NOT follow the plural's
        # linking vowel: `barát` pluralises as `barátok` but possesses as
        # `barátaim`, and `gyerek` gives `gyerekek` but `gyerekeim`.
        stem = lemma + pick(harmony, "ai", "ei", "ei")

    for person, number, ending in _POSSESSUM_PLURAL:
        if number == "Plur" and ending == "tok":
            ending = pick(harmony, "tok", "tek", "tek")
        feats = {
            "Person[psor]": person,
            "Number[psor]": number,
            "Number[psed]": "Plur",
            "Number": "Sing",
            "Case": "Nom",
        }
        out.append(Form(stem + ending, feats, Confidence.MEDIUM))
    return out


def possessives(lemma: str, harmony: Harmony) -> list[Form]:
    """Singular-possessum possessive forms.

    After a vowel the third person takes `-ja/-je` rather than `-a/-e`, and the
    other persons drop their linking vowel. Choosing between `-a/-e` and
    `-ja/-je` on consonant-final stems is lexically conditioned, so both are
    emitted for recognition.
    """
    out: list[Form] = []
    vowel_final = ends_in_vowel(lemma)
    stem = lengthen_final_vowel(lemma) if vowel_final else lemma

    linking = _linking_vowel(lemma, harmony)

    for person, number, uses_linking, forms in _POSSESSIVE:
        tail = pick(harmony, *forms)
        feats = {"Person[psor]": person, "Number[psor]": number, "Number": "Sing", "Case": "Nom"}

        if vowel_final:
            if person == "3" and number == "Sing":
                out.append(Form(stem + pick(harmony, "ja", "je", "je"), feats, Confidence.MEDIUM))
                continue
            if number == "Plur" and person == "3":
                out.append(Form(stem + pick(harmony, "juk", "jük", "jük"), feats, Confidence.MEDIUM))
                continue
            # alma + 1sg -> almám: no linking vowel after a vowel.
            out.append(Form(stem + tail, feats, Confidence.MEDIUM))
            continue

        suffix = (linking + tail) if uses_linking else tail
        out.append(Form(lemma + suffix, feats, Confidence.MEDIUM))

        if person == "3":
            # The -ja/-je variant, e.g. `barát` -> `barátja`. Lexical, so both.
            j = (pick(harmony, "ja", "je", "je") if number == "Sing"
                 else pick(harmony, "juk", "jük", "jük"))
            out.append(Form(lemma + j, feats, Confidence.LOW))
    return out


# ---------------------------------------------------------------------------
# Whole-paradigm generation
# ---------------------------------------------------------------------------

# Cases stacked onto a possessive or plural stem. Restricted to the highest-value
# ones: the full cross-product would multiply the wordform table without adding
# much recognition value, since the long tail is vanishingly rare in real text.
_STACKED_CASES = ("Ine", "Ill", "Ela", "Sbl", "Del", "Ade", "All", "Abl", "Dat")
# Instrumental is stacked separately because it assimilates rather than
# concatenating: `barátaim` + `-val` -> `barátaimmal`.


def _stack(stem_forms: list[Form]) -> list[Form]:
    """Apply case suffixes to already-inflected stems.

    `házam` + `-ban` -> `házamban`; `házaim` + `-ban` -> `házaimban`.
    Harmony is recomputed from the derived stem, not the lemma, because the
    possessive ending can change which vowel sits next to the suffix.
    """
    out: list[Form] = []
    for base in stem_forms:
        stem = base.form
        h = classify(stem)
        for case in _STACKED_CASES:
            for f in simple_case(stem, h, case):
                feats = dict(base.feats)
                feats["Case"] = case
                out.append(Form(f.form, feats, Confidence.MEDIUM))
        low_vowel = pick(h, "a", "e", "ö")
        for f in accusative(stem, h, linking_override=low_vowel):
            feats = dict(base.feats)
            feats["Case"] = "Acc"
            out.append(Form(f.form, feats, f.confidence))
        for f in instrumental(stem, h):
            feats = dict(base.feats)
            feats["Case"] = "Ins"
            out.append(Form(f.form, feats, f.confidence))
    return out


def noun_forms(lemma: str) -> list[Form]:
    """Every generated form for a noun lemma, deduplicated.

    Duplicates are real: `-ig` does not harmonise, so a two-form generator can
    produce the same string twice. Keep the highest confidence for each
    (form, feats) pair.
    """
    harmony = classify(lemma)
    out: list[Form] = [Form(lemma, {"Case": "Nom", "Number": "Sing"}, Confidence.HIGH)]

    out += accusative(lemma, harmony)
    plurals = plural(lemma, harmony)
    out += plurals
    out += superessive(lemma, harmony)
    out += instrumental(lemma, harmony)

    poss_sing = possessives(lemma, harmony)
    poss_plur = possessives_plural_possessum(lemma, harmony)
    out += poss_sing + poss_plur

    for case in _SIMPLE_CASES:
        out += simple_case(lemma, harmony, case)

    # The stacked layer: possessive + case, and plural + case.
    out += _stack([f for f in poss_sing if f.confidence is not Confidence.LOW])
    out += _stack(poss_plur)
    out += _stack([f for f in plurals if f.confidence is not Confidence.LOW])

    rank = {Confidence.HIGH: 3, Confidence.MEDIUM: 2, Confidence.LOW: 1}
    best: dict[tuple[str, str], Form] = {}
    for f in out:
        key = (f.form, f.feats_key)
        if key not in best or rank[f.confidence] > rank[best[key].confidence]:
            best[key] = f
    return sorted(best.values(), key=lambda f: (f.form, f.feats_key))


# The cells a learner actually wants to see. The full generated set is ~175
# forms per noun, which is a lookup index, not a table anyone can read.
_DISPLAY_CASES = (
    "Nom", "Acc", "Dat", "Ins",
    "Ine", "Ill", "Ela", "Sup", "Sbl", "Del", "Ade", "All", "Abl",
)


def slot_for(feats: dict[str, str]) -> str | None:
    """The display-table slot a feature set belongs to, or None.

    Factored out of `paradigm` so that build.py can apply the SAME naming to the
    attested Wiktionary forms and overlay them cell by cell. Before that, the
    paradigm table was generator-only: for `ház` it showed the generated `házan`
    while `házon` sat attested in `wordform` beside it, and the reader taught the
    wrong form.
    """
    case = feats.get("Case", "Nom")
    if "Person[psor]" in feats:
        if case != "Nom":
            return None
        number = feats.get("Number[psor]")
        if not number:
            return None
        possessum = "Pl" if feats.get("Number[psed]") == "Plur" else ""
        return f"Poss{possessum}.{feats['Person[psor]']}{number}"
    if case not in _DISPLAY_CASES:
        return None
    return f"{case}.{feats.get('Number', 'Sing')}"


def paradigm(lemma: str) -> dict[str, str]:
    """High-confidence forms only, keyed by slot, for display to the learner.

    Anything the generator is unsure about is omitted rather than guessed: an
    empty cell is honest, a wrong cell is taught.

    Possessive slots carry the number of the POSSESSUM as well as the possessor,
    because `házam` (my house) and `házaim` (my houses) are different cells and
    collapsing them silently loses one.
    """
    out: dict[str, str] = {}
    for f in noun_forms(lemma):
        if f.confidence is Confidence.LOW:
            continue
        slot = slot_for(f.feats)
        if slot is None:
            continue
        out.setdefault(slot, f.form)
    return out
