"""Frequency ranks from the OpenSubtitles-derived list.

Subtitle frequency is the right register for a learner: it weights spoken,
everyday language over the news-and-Wikipedia bias of a web corpus. The list is
of *wordforms*, not lemmas, so ranks are aggregated onto lemmas by summing the
counts of every form that maps to them.

CEFR bands are estimated from the rank, following the lemma targets in the source
brief §2.6 (A1 700-900, A2 ~2000, B1 3500-4500, B2 6000-8000). This is an
estimate and the UI labels it as one.
"""

from __future__ import annotations

from pathlib import Path

# (upper rank bound, label). Derived from the brief's lemma targets.
CEFR_BANDS: list[tuple[int, str]] = [
    (900, "A1"),
    (2000, "A2"),
    (4500, "B1"),
    (8000, "B2"),
    (16000, "C1"),
]


def cefr_for_rank(rank: int | None) -> str | None:
    if rank is None:
        return None
    for bound, label in CEFR_BANDS:
        if rank <= bound:
            return label
    return "C2"


def load(path: Path) -> dict[str, int]:
    """Parse `{word} {count}` lines into {form: count}."""
    counts: dict[str, int] = {}
    with path.open("r", encoding="utf-8", errors="replace") as fh:
        for line in fh:
            parts = line.split()
            if len(parts) != 2:
                continue
            word, raw = parts
            try:
                counts[word.lower()] = int(raw)
            except ValueError:
                continue
    return counts


def rank_lemmas(
    form_counts: dict[str, int],
    form_to_lemmas: dict[str, set[str]],
) -> dict[str, tuple[int, float]]:
    """Aggregate wordform counts onto lemmas, then rank.

    A form that maps to several lemmas contributes its full count to each: we
    cannot disambiguate without context, and splitting the count would
    systematically understate homographs. Returns {lemma: (rank, per_million)}.
    """
    totals: dict[str, int] = {}
    for form, count in form_counts.items():
        for lemma in form_to_lemmas.get(form, ()):
            totals[lemma] = totals.get(lemma, 0) + count

    grand = sum(form_counts.values()) or 1
    ordered = sorted(totals.items(), key=lambda kv: (-kv[1], kv[0]))
    return {
        lemma: (rank, count * 1_000_000 / grand)
        for rank, (lemma, count) in enumerate(ordered, start=1)
    }
