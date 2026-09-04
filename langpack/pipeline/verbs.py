"""Generate the indicative conjugation of regular Hungarian verbs.

This is the half of `inflect.py`'s job that its docstring deferred to Phase 5
and nobody built. The cost of the gap was concrete: Wiktionary's extraction is
spotty for verbs — `hoz` arrived with 24 forms, holding `hoztuk` (definite) but
not `hoztunk` (indefinite) — so a learner clicking `hoztunk` on their own course
card was told the word is not in the pack. Six corpus sentences contained it at
the time.

Scope, deliberately: **present and past indicative, both definitenesses, plus
the -lak/-lek form.** That is what months one to four of the course produce and
what review cards are made of. Conditional, imperative and potential stay out —
their assimilation rules are exactly where a generator starts inventing wrong
Hungarian, and they arrive with the grammar trainer's own attested material.

The philosophy is inflect.py's, restated:

* These rows are for **recognition** (`form -> lemma + features`), where recall
  dominates and a junk entry is harmless unless it collides with a real word.
  Attested rows always win at merge and at lookup (`trustRank`), so a generated
  cell can never displace a dictionary form.
* Nothing generated is ever presented as the correct answer to a drill without
  a caveat — `Form.Attested()` on the Go side is the gate, and these rows are
  `source='generated'`.

Two anchoring rules keep the output honest:

* **The past stem is extracted from an attested past form when one exists.**
  `hoztuk` → past stem `hozt` → `hoztunk`, `hoztál`, `hoztatok`… all at MEDIUM.
  With no attested past the stem is built by rule and everything derived from
  it drops to LOW.
* **The dozen core irregular verbs are skipped entirely.** `megy`, `van`,
  `eszik` — their suppletive stems make every generated cell garbage, their
  real forms are the best-attested in the language, and a generator that emits
  `megytunk` has stopped being a safety net.
"""

from __future__ import annotations

from dataclasses import dataclass

from .harmony import Harmony, classify, pick
from .inflect import Confidence

# Suppletive or heavily irregular: generation would be wrong more often than
# right, and every one of these is richly attested already.
IRREGULAR = frozenset({
    "van", "lesz", "megy", "jön", "eszik", "iszik", "tesz", "vesz", "visz",
    "hisz", "alszik", "fekszik", "nyugszik", "esküszik", "öregszik", "él",
})

SIBILANTS = ("s", "sz", "z", "dz", "zs")


@dataclass(frozen=True)
class VerbForm:
    form: str
    feats: dict[str, str]
    confidence: Confidence

    @property
    def feats_key(self) -> str:
        return "|".join(f"{k}={v}" for k, v in sorted(self.feats.items()))


def _feats(person: int, number: str, tense: str, definite: str) -> dict[str, str]:
    return {
        "Mood": "Ind", "Tense": tense, "Person": str(person),
        "Number": number, "Definite": definite,
    }


def present_stem(lemma: str) -> str:
    """The stem the present tense builds on: the lemma, minus -ik."""
    if lemma.endswith("ik") and len(lemma) > 3:
        return lemma[:-2]
    return lemma


def _ends_with_sibilant(stem: str) -> str | None:
    """The sibilant a stem ends in, longest digraph first, or None."""
    for s in sorted(SIBILANTS, key=len, reverse=True):
        if stem.endswith(s):
            return s
    return None


def _j_assimilated(stem: str, ending: str) -> str:
    """Attach a j-initial definite ending, geminating a final sibilant.

    hoz + ja -> hozza, olvas + juk -> olvassuk, néz + jük -> nézzük. After any
    other consonant the j survives: mond + ja -> mondja.
    """
    sib = _ends_with_sibilant(stem)
    if sib is None:
        return stem + ending
    # The gemination doubles the FIRST letter of a digraph: sz+sz -> ssz.
    return stem + sib[0] + ending[1:]


def past_stem_from_attested(attested: list[tuple[str, dict[str, str]]]) -> str | None:
    """Extract the past stem from any attested past-tense form.

    The fused endings are unambiguous once the form's own features say which
    cell it is, so this is a reversal, not a guess. 3SG indefinite (`hozott`)
    is skipped: its linking vowel belongs to that cell alone.
    """
    endings = {
        ("1", "Sing", "Ind"): ["am", "em"], ("2", "Sing", "Ind"): ["ál", "él"],
        ("1", "Plur", "Ind"): ["unk", "ünk"], ("2", "Plur", "Ind"): ["atok", "etek"],
        ("3", "Plur", "Ind"): ["ak", "ek"],
        ("1", "Sing", "Def"): ["am", "em"], ("2", "Sing", "Def"): ["ad", "ed"],
        ("3", "Sing", "Def"): ["a", "e"], ("1", "Plur", "Def"): ["uk", "ük"],
        ("2", "Plur", "Def"): ["átok", "étek"], ("3", "Plur", "Def"): ["ák", "ék"],
    }
    for form, feats in attested:
        if feats.get("Tense") != "Past":
            continue
        key = (feats.get("Person", ""), feats.get("Number", ""), feats.get("Definite", ""))
        for suffix in endings.get(key, []):
            if form.endswith(suffix) and len(form) > len(suffix) + 1:
                stem = form[: -len(suffix)]
                if stem.endswith("t"):
                    return stem
    return None


def conjugate(lemma: str, attested: list[tuple[str, dict[str, str]]]) -> list[VerbForm]:
    """Every present/past indicative cell, plus -lak/-lek and the infinitive.

    `attested` is the verb's existing wordform rows; cells already covered are
    skipped at insert time by the caller — this function just generates.
    """
    if lemma in IRREGULAR or len(lemma) < 3:
        return []
    stem = present_stem(lemma)
    h = classify(lemma)
    ik = lemma.endswith("ik") and stem != lemma
    out: list[VerbForm] = []

    def add(form: str, feats: dict[str, str], conf: Confidence) -> None:
        out.append(VerbForm(form=form, feats=feats, confidence=conf))

    sib = _ends_with_sibilant(stem)

    # ── present, indefinite ──────────────────────────────────────────────
    add(stem + pick(h, "ok", "ek", "ök"), _feats(1, "Sing", "Pres", "Ind"), Confidence.MEDIUM)
    if sib:
        add(stem + pick(h, "ol", "el", "öl"), _feats(2, "Sing", "Pres", "Ind"), Confidence.MEDIUM)
    else:
        add(stem + "sz", _feats(2, "Sing", "Pres", "Ind"), Confidence.MEDIUM)
        # After a consonant cluster the ending takes a linking vowel
        # (mondasz). Emitting both is the documented recognition posture.
        add(stem + pick(h, "asz", "esz"), _feats(2, "Sing", "Pres", "Ind"), Confidence.LOW)
    add(lemma if ik else stem, _feats(3, "Sing", "Pres", "Ind"), Confidence.MEDIUM)
    add(stem + pick(h, "unk", "ünk"), _feats(1, "Plur", "Pres", "Ind"), Confidence.MEDIUM)
    add(stem + pick(h, "tok", "tek", "tök"), _feats(2, "Plur", "Pres", "Ind"), Confidence.MEDIUM)
    add(stem + pick(h, "otok", "etek", "ötök"), _feats(2, "Plur", "Pres", "Ind"), Confidence.LOW)
    add(stem + pick(h, "nak", "nek"), _feats(3, "Plur", "Pres", "Ind"), Confidence.MEDIUM)
    add(stem + pick(h, "anak", "enek"), _feats(3, "Plur", "Pres", "Ind"), Confidence.LOW)

    # ── present, definite ────────────────────────────────────────────────
    add(stem + pick(h, "om", "em", "öm"), _feats(1, "Sing", "Pres", "Def"), Confidence.MEDIUM)
    add(stem + pick(h, "od", "ed", "öd"), _feats(2, "Sing", "Pres", "Def"), Confidence.MEDIUM)
    if h is Harmony.BACK:
        add(_j_assimilated(stem, "ja"), _feats(3, "Sing", "Pres", "Def"), Confidence.MEDIUM)
        add(_j_assimilated(stem, "juk"), _feats(1, "Plur", "Pres", "Def"), Confidence.MEDIUM)
        add(_j_assimilated(stem, "játok"), _feats(2, "Plur", "Pres", "Def"), Confidence.MEDIUM)
        add(_j_assimilated(stem, "ják"), _feats(3, "Plur", "Pres", "Def"), Confidence.MEDIUM)
    else:
        add(stem + "i", _feats(3, "Sing", "Pres", "Def"), Confidence.MEDIUM)
        add(_j_assimilated(stem, "jük"), _feats(1, "Plur", "Pres", "Def"), Confidence.MEDIUM)
        add(stem + "itek", _feats(2, "Plur", "Pres", "Def"), Confidence.MEDIUM)
        add(stem + "ik", _feats(3, "Plur", "Pres", "Def"), Confidence.MEDIUM)

    # ── -lak/-lek ────────────────────────────────────────────────────────
    add(stem + pick(h, "lak", "lek"),
        {"Mood": "Ind", "Tense": "Pres", "Person": "1", "Number": "Sing", "Definite": "2"},
        Confidence.MEDIUM)

    # ── past, both definitenesses, from the anchored stem ────────────────
    pstem = past_stem_from_attested(attested)
    conf = Confidence.MEDIUM if pstem else Confidence.LOW
    if pstem is None:
        pstem = present_stem(lemma) + "t"

    past = [
        (pick(h, "am", "em"), 1, "Sing", "Ind"), (pick(h, "ál", "él"), 2, "Sing", "Ind"),
        (pick(h, "unk", "ünk"), 1, "Plur", "Ind"), (pick(h, "atok", "etek"), 2, "Plur", "Ind"),
        (pick(h, "ak", "ek"), 3, "Plur", "Ind"),
        (pick(h, "am", "em"), 1, "Sing", "Def"), (pick(h, "ad", "ed"), 2, "Sing", "Def"),
        (pick(h, "a", "e"), 3, "Sing", "Def"), (pick(h, "uk", "ük"), 1, "Plur", "Def"),
        (pick(h, "átok", "étek"), 2, "Plur", "Def"), (pick(h, "ák", "ék"), 3, "Plur", "Def"),
    ]
    for ending, person, number, definite in past:
        add(pstem + ending, _feats(person, number, "Past", definite), conf)
    # 3SG indefinite carries its own linking vowel (hozott); derivable from the
    # anchored stem only when that stem is bare lemma+t.
    if pstem == present_stem(lemma) + "t":
        add(present_stem(lemma) + pick(h, "ott", "ett", "ött"),
            _feats(3, "Sing", "Past", "Ind"), Confidence.LOW)

    # ── infinitive ───────────────────────────────────────────────────────
    add(stem + "ni", {"VerbForm": "Inf"}, Confidence.MEDIUM)

    return out


# ── backfilling an existing pack ──────────────────────────────────────────
#
#     python -m pipeline.verbs --pack ../data/langpack-hu-v1.sqlite
#
# Two backfills, one command:
#
#   1. every VERB lemma gets its generated indicative cells — insert-if-absent
#      per (form, lemma_id), so an attested surface form is never shadowed and
#      a re-run is a no-op rather than a duplicate;
#   2. corpus-attested forms that the build should have folded into `wordform`
#      and did not (7,649 of them on hu-v1) go in as source='corpus' with no
#      feature claims — the sentence proves the form exists, not what it marks.
#
# Stop the server first; the app treats a pack as read-only.

def _parse_feats(raw: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for part in raw.split("|"):
        if "=" in part:
            k, v = part.split("=", 1)
            out[k] = v
    return out


def backfill(conn) -> dict[str, int]:
    have = {
        (form, lid)
        for form, lid in conn.execute("SELECT form, lemma_id FROM wordform")
    }

    generated = 0
    rows = conn.execute(
        "SELECT id, lemma FROM lemma WHERE upos = 'VERB'").fetchall()
    for lid, lemma in rows:
        attested = [
            (f, _parse_feats(feats))
            for f, feats in conn.execute(
                "SELECT form, feats FROM wordform WHERE lemma_id = ?", (lid,))
        ]
        for vf in conjugate(lemma, attested):
            if (vf.form, lid) in have:
                continue
            have.add((vf.form, lid))
            conn.execute(
                "INSERT INTO wordform (form, lemma_id, upos, feats, source, confidence) "
                "VALUES (?,?,?,?,?,?)",
                (vf.form, lid, "VERB", vf.feats_key, "generated", vf.confidence.value))
            generated += 1

    corpus = 0
    for form, lid, upos in conn.execute(
        """SELECT DISTINCT t.form, t.lemma_id, l.upos
             FROM sentence_token t JOIN lemma l ON l.id = t.lemma_id
            WHERE t.lemma_id IS NOT NULL"""):
        if (form, lid) in have:
            continue
        have.add((form, lid))
        conn.execute(
            "INSERT INTO wordform (form, lemma_id, upos, feats, source, confidence) "
            "VALUES (?,?,?,?,?,?)", (form, lid, upos, "", "corpus", "medium"))
        corpus += 1

    return {"verbs": len(rows), "generated_forms": generated, "corpus_forms": corpus}


def main() -> int:
    import argparse
    import sqlite3
    from pathlib import Path

    ap = argparse.ArgumentParser(
        description="Backfill verb conjugations and corpus-attested forms into a pack")
    ap.add_argument("--pack", type=Path, required=True)
    args = ap.parse_args()

    conn = sqlite3.connect(args.pack)
    try:
        with conn:
            stats = backfill(conn)
    finally:
        conn.close()
    for k, v in stats.items():
        print(f"  {k}: {v}")
    print("\n  now run `make starter` and commit")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
