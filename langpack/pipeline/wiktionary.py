"""Parse the wiktextract Hungarian extract into lemmas, glosses, IPA and forms.

Wiktionary is the best freely-licensed source of Hungarian *paradigms* -- it
carries full declension tables, which is exactly what an agglutinative language
needs and what a plain bilingual wordlist cannot give.

Attested forms from here outrank anything the generator in `inflect.py` produces:
`inflect` covers the productive rules, Wiktionary covers the exceptions. See
docs/LANGPACK.md.

Licence: CC BY-SA 3.0 / GFDL, which is why the emitted pack is CC BY-SA 4.0 even
though the application code is MIT.
"""

from __future__ import annotations

import json
import re
from collections.abc import Iterator
from dataclasses import dataclass, field
from pathlib import Path

# Wiktionary tag -> UD feature. Anything unmapped is dropped rather than guessed:
# a wrong feature is worse than a missing one, because it corrupts the mastery
# heatmap for that learner.
TAG_TO_FEATURE: dict[str, tuple[str, str]] = {
    "nominative": ("Case", "Nom"),
    "accusative": ("Case", "Acc"),
    "dative": ("Case", "Dat"),
    "instrumental": ("Case", "Ins"),
    "causal-final": ("Case", "Cau"),
    "translative": ("Case", "Tra"),
    "terminative": ("Case", "Ter"),
    "essive-formal": ("Case", "Ess"),
    "essive-modal": ("Case", "Ess"),
    "inessive": ("Case", "Ine"),
    "superessive": ("Case", "Sup"),
    "adessive": ("Case", "Ade"),
    "illative": ("Case", "Ill"),
    "sublative": ("Case", "Sbl"),
    "allative": ("Case", "All"),
    "elative": ("Case", "Ela"),
    "delative": ("Case", "Del"),
    "ablative": ("Case", "Abl"),
    "singular": ("Number", "Sing"),
    "plural": ("Number", "Plur"),
    "first-person": ("Person", "1"),
    "second-person": ("Person", "2"),
    "third-person": ("Person", "3"),
    "comparative": ("Degree", "Cmp"),
    "superlative": ("Degree", "Sup"),
    "infinitive": ("VerbForm", "Inf"),
    "past": ("Tense", "Past"),
    "present": ("Tense", "Pres"),
    "conditional": ("Mood", "Cnd"),
    "imperative": ("Mood", "Imp"),
    "indicative": ("Mood", "Ind"),
    "definite": ("Definite", "Def"),
    "indefinite": ("Definite", "Ind"),
}

POS_TO_UPOS: dict[str, str] = {
    "noun": "NOUN",
    "proper noun": "PROPN",
    "verb": "VERB",
    "adj": "ADJ",
    "adv": "ADV",
    "pron": "PRON",
    "num": "NUM",
    "det": "DET",
    "conj": "CCONJ",
    "postp": "ADP",
    "prep": "ADP",
    "intj": "INTJ",
    "particle": "PART",
    "prefix": "X",
    "suffix": "X",
    # `a`/`az` are articles and were being dropped entirely, which is why the
    # two most frequent tokens in any Hungarian text were unresolvable.
    "article": "DET",
    "name": "PROPN",
    "phrase": "X",
    "proverb": "X",
    "contraction": "X",
}

# Tags that mark a table-layout artefact rather than a real wordform.
#
# `error-unrecognized-form` is the important one: wiktextract sets it when it
# fails to parse an inflection table, and on Hungarian VERB tables the resulting
# rows are shifted by one — `megyek` (1sg) arrives carrying the tags of the next
# row, and cells like "intransitive verb" appear as pseudo-forms. Importing
# those would put wrong Person/Number features into the morphological state
# space, which is precisely the corruption that makes a mastery heatmap
# meaningless. When the source says it failed, believe it.
SKIP_FORM_TAGS = frozenset(
    {"table-tags", "inflection-template", "class", "error-unrecognized-form"}
)

# Parts of speech whose Wiktionary inflection tables we trust.
#
# VERB is deliberately absent. Even after dropping the rows wiktextract flags as
# errors, the Hungarian conjugation tables it produces are still wrong: converbs
# arrive tagged with a person, unsplit table cells appear as pseudo-forms
# ("menve (menvén)"), and the definite/indefinite columns do not line up. Since
# the definite conjugation is the single most important thing this app teaches,
# importing a plausible-looking but wrong verb paradigm would be worse than
# having none. Verb forms come from the generator and from corpus attestation
# instead. Noun and adjective tables, by contrast, are excellent.
TRUSTED_FORM_POS = frozenset({"NOUN", "PROPN", "ADJ", "NUM", "PRON"})


@dataclass
class Entry:
    """A headword: a real lemma."""

    word: str
    upos: str
    glosses_en: list[str] = field(default_factory=list)
    ipa: str = ""
    # (form, {feature: value})
    forms: list[tuple[str, dict[str, str]]] = field(default_factory=list)


@dataclass
class FormOf:
    """An inflected form that Wiktionary happens to give its own page.

    These are NOT lemmas. `házban`, `almát` and `könyvek` all have Wiktionary
    entries, and importing them as headwords was a real bug: it inflated the
    lemma count, and the form generator then went on to inflect *them*, producing
    nonsense like `könyvekhez` analysed as the allative of a lemma `könyvek`.

    Treated correctly they are a bonus: ~tens of thousands of extra attested
    forms with features taken straight from the gloss.
    """

    form: str
    lemma: str
    upos: str
    feats: dict[str, str] = field(default_factory=dict)


# wiktextract writes inflection glosses as "<description> of <lemma>", sometimes
# with a parenthetical gloss. Some entries (notably possessives) carry no
# form-of tag at all, so the gloss is the only signal.
_OF_RE = re.compile(
    r"^(?P<desc>[a-z0-9 \-]+?)\s+of\s+(?P<lemma>[^\s(,]+)", re.IGNORECASE
)
_INFLECTION_WORDS = (
    "singular", "plural", "accusative", "dative", "possessive", "possession",
    "inessive", "illative", "elative", "superessive", "sublative", "delative",
    "adessive", "allative", "ablative", "instrumental", "causal", "terminative",
    "translative", "essive", "distributive", "comparative", "superlative",
    "nominative", "person", "tense", "conditional", "imperative", "participle",
)


def _feats_from_tags(tags: list[str]) -> dict[str, str]:
    """Map Wiktionary tags to UD features, ignoring what we cannot map."""
    out: dict[str, str] = {}
    possessive = "possessive" in tags
    for tag in tags:
        mapped = TAG_TO_FEATURE.get(tag)
        if not mapped:
            continue
        key, value = mapped
        # Inside a possessive column, person/number describe the POSSESSOR, not
        # the word itself. Getting this wrong would put `házam` in the same
        # bucket as a first-person verb.
        if possessive and key in ("Person", "Number"):
            key = f"{key}[psor]"
        out[key] = value
    if possessive and "Person[psor]" not in out:
        out["Person[psor]"] = "3"
    return out


# Hungarian orthography plus the hyphen used in compounds.
_FORM_CHARS = set("aábcdeéfghiíjklmnoóöőpqrstuúüűvwxyzáéíóöőúüű-")


def _is_plausible_form(form: str) -> bool:
    """Reject unsplit table cells and layout placeholders.

    wiktextract sometimes emits a whole cell as one "form": "menve (menvén)",
    "mehetve / mehetvén", or a template name like "hu-infl-nom". A real wordform
    is a single orthographic token.
    """
    if form.startswith("hu-") or form.startswith("no-table"):
        return False
    if any(ch in form for ch in " /(),;"):
        return False
    return bool(form) and set(form.lower()) <= _FORM_CHARS


def _first_ipa(sounds: list[dict]) -> str:
    for s in sounds or []:
        ipa = s.get("ipa")
        if ipa:
            return ipa
    return ""


def _as_form_of(raw: dict, upos: str) -> FormOf | None:
    """Detect an inflected-form page and turn it into a FormOf.

    Returns None if any sense is a real lexical sense -- a word can be both a
    lemma and an inflected form of something else, and in that case we keep the
    lemma reading.
    """
    senses = raw.get("senses") or []
    if not senses:
        return None

    lemma = ""
    feats: dict[str, str] = {}

    for sense in senses:
        tags = sense.get("tags") or []
        gloss = (sense.get("glosses") or [""])[0] or ""

        target = ""
        if sense.get("form_of"):
            target = (sense["form_of"][0] or {}).get("word", "")
        if not target and "form-of" in tags:
            m = _OF_RE.match(gloss)
            target = m.group("lemma") if m else ""
        if not target:
            # No tag: accept only an unambiguous inflection gloss.
            m = _OF_RE.match(gloss)
            if not m or not any(w in m.group("desc").lower() for w in _INFLECTION_WORDS):
                return None  # a real lexical sense -> keep as a lemma
            target = m.group("lemma")

        if not target:
            return None
        lemma = lemma or target

        feats.update(_feats_from_tags(tags))
        if not feats:
            # Recover features from the gloss wording when tags are absent.
            desc = gloss.lower()
            for word, mapped in (
                ("first-person", ("Person[psor]", "1")),
                ("second-person", ("Person[psor]", "2")),
                ("third-person", ("Person[psor]", "3")),
                ("multiple-possession", ("Number[psed]", "Plur")),
                ("plural", ("Number", "Plur")),
                ("singular", ("Number", "Sing")),
            ):
                if word in desc:
                    feats[mapped[0]] = mapped[1]

    if not lemma or not _is_plausible_form(raw.get("word", "")):
        return None
    return FormOf(form=raw["word"], lemma=lemma, upos=upos, feats=feats)


def parse(path: Path, *, limit: int | None = None) -> Iterator[Entry | FormOf]:
    """Stream entries. The file is ~580 MB, so nothing is held in memory.

    Yields `Entry` for headwords and `FormOf` for inflected-form pages; callers
    must distinguish them.
    """
    seen = 0
    with path.open("r", encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            try:
                raw = json.loads(line)
            except json.JSONDecodeError:
                continue

            if raw.get("lang_code") != "hu":
                continue
            word = raw.get("word")
            if not word:
                continue

            upos = POS_TO_UPOS.get(raw.get("pos", ""), None)
            if upos is None:
                continue

            form_of = _as_form_of(raw, upos)
            if form_of is not None:
                yield form_of
                seen += 1
                if limit is not None and seen >= limit:
                    return
                continue

            entry = Entry(word=word, upos=upos, ipa=_first_ipa(raw.get("sounds", [])))

            for sense in raw.get("senses", []):
                for gloss in sense.get("glosses", []) or []:
                    if gloss and gloss not in entry.glosses_en:
                        entry.glosses_en.append(gloss)

            if upos in TRUSTED_FORM_POS:
                for f in raw.get("forms", []) or []:
                    form = (f.get("form") or "").strip()
                    tags = f.get("tags") or []
                    if not form or form == word:
                        continue
                    if SKIP_FORM_TAGS & set(tags):
                        continue
                    if not _is_plausible_form(form):
                        continue
                    feats = _feats_from_tags(tags)
                    if feats:
                        entry.forms.append((form, feats))

            # An entry with neither a gloss nor a form teaches nothing.
            if not entry.glosses_en and not entry.forms:
                continue

            yield entry
            seen += 1
            if limit is not None and seen >= limit:
                return
