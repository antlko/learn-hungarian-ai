"""Conversational Hungarian from film subtitles, with its translations.

Tatoeba is 60,000 sentences of careful, written, often stilted Hungarian, and
only 3,746 of them have a Russian translation. That is a thin corpus for an app
whose stated goal is conversation: the reader runs out of material, the `n+1`
rail repeats itself, a course word with no attested sentence gets no card, and
the review screen shows a Hungarian sentence it cannot translate.

OPUS's OpenSubtitles is the standard free source of conversational Hungarian —
film and television dialogue, sentence-aligned against other languages. The
frequency list this pack already ships is derived from it. Two parallel corpora
are read here:

    hu-ru   690 MB   Hungarian + Russian
    en-hu   1.3 GB   Hungarian + English

Aligned, so a selected sentence arrives WITH its translation. That is the part
Tatoeba could not give us at volume.

**Subtitle text is dirty and most of it is unusable.** Speaker dashes, italics
markup, music glyphs, sentences split across two cards, screaming capitals,
transliterated names, and a great deal of profanity and violence that has no
business in a beginner's first hundred sentences. `usable()` below is therefore
aggressive: it throws away the overwhelming majority of the corpus, and that is
the intended behaviour. We are not building a corpus, we are picking a few
hundred thousand sentences that a learner should meet.

**Nothing here is generated.** Every sentence is one a person wrote and a human
translator translated; the pipeline only chooses.

**Licence.** OPUS OpenSubtitles is free for research and widely redistributed,
but it is not as cleanly licensed as Tatoeba's CC BY. Sentences from this source
are therefore marked `source='opensubtitles'` and the starter pack — the one
compiled INTO the distributed binary — takes Tatoeba only. See
langpack/LICENSES.md and pipeline/starter.py.
"""

from __future__ import annotations

import re
import unicodedata
import zipfile
import json
from collections import Counter
from dataclasses import dataclass
from pathlib import Path
from typing import Iterator

# ── cleaning ──────────────────────────────────────────────────────────────

# Subtitle furniture, stripped before anything else looks at the line.
#
# The `\d+)}` alternative catches the debris of ASS/SSA style overrides that the
# corpus splits mid-tag: real lines in the Russian side read
# `298)}Код Гиасс: 353)}Восстание Лелуша`. Without it those survive every other
# check, because they do contain letters.
_MARKUP = re.compile(r"<[^>]{1,20}>|\{\\?[^}]{0,40}\}|\[[^\]]{1,40}\]|\d+\)\}")
_MUSIC = re.compile(r"[♪♫#]+")
_SPEAKER = re.compile(r"^\s*[-–—]\s*")
_SPACES = re.compile(r"\s+")

# A line that is only punctuation, digits or Latin abbreviations.
_HAS_LETTER = re.compile(r"\p{L}" if False else r"[^\W\d_]", re.UNICODE)

_URL = re.compile(r"https?://|www\.|@\w+\.\w", re.IGNORECASE)

# Hungarian sentence-final punctuation. A subtitle ending in a comma or an
# ellipsis is half of a sentence continued on the next card.
_ENDS_SENTENCE = re.compile(r"[.!?]$")

_WORD = re.compile(r"[^\W\d_]+(?:[-'][^\W\d_]+)*", re.UNICODE)

# Characters Hungarian actually uses. A line containing anything else is either
# another language, a transliteration, or mojibake.
_HU_ALPHABET = set("aábcdeéfghiíjklmnoóöőpqrstuúüűvwxyzAÁBCDEÉFGHIÍJKLMNOÓÖŐPQRSTUÚÜŰVWXYZ")
_PUNCT_OK = set(" .,!?:;-–—'\"„”«»()…")


def clean(line: str) -> str:
    """Strip subtitle furniture. Returns '' when nothing usable is left."""
    s = unicodedata.normalize("NFC", line)
    s = _MARKUP.sub(" ", s)
    s = _MUSIC.sub(" ", s)
    s = _SPEAKER.sub("", s)
    s = s.replace("​", "").replace("\xa0", " ")
    return _SPACES.sub(" ", s).strip()


def tokens(s: str) -> list[str]:
    return _WORD.findall(s)


# Styling debris that survived `clean`, and the credit lines that open almost
# every fansubbed film. A line carrying either is not dialogue.
_RESIDUE = re.compile(r"[{}]|\d+\)|www|\.com\b", re.IGNORECASE)
_CREDITS = re.compile(
    r"(перевод|редактор|тайминг|правка|субтитр|озвуч|релиз|"
    r"subtitle|sync|corrected by|translated by|encoded)",
    re.IGNORECASE,
)
_DIGITS = re.compile(r"\d+")


def aligned(hu: str, trans: str) -> bool:
    """Whether the two lines plausibly say the same thing.

    OpenSubtitles is aligned automatically and the alignment is imperfect —
    reliably so at the start of a film, where one side runs translator credits
    and opening-song lyrics against the other side's dialogue. A wrong
    translation presented as fact is worse than no translation at all, so a pair
    that fails any of these is dropped rather than trusted.

    None of this checks meaning. It checks the cheap structural invariants a
    real translation pair almost always satisfies and a misaligned pair
    frequently violates — which is the most that can be done for free, and quite
    a lot in practice.
    """
    if _RESIDUE.search(trans) or _CREDITS.search(trans):
        return False

    # Numbers survive translation. "Öt óra." against a line containing 3 is not
    # the same sentence.
    if _DIGITS.findall(hu) != _DIGITS.findall(trans):
        return False

    # A question stays a question.
    if hu.endswith("?") != trans.rstrip().endswith("?"):
        return False

    # Length ratio. Russian, English and Hungarian differ in wordiness, but not
    # by twofold on a single sentence.
    a, b = len(tokens(hu)), len(tokens(trans))
    if a == 0 or b == 0:
        return False
    return 0.5 <= a / b <= 2.0


@dataclass(frozen=True)
class Pair:
    """One Hungarian sentence with a translation, ready to score."""

    hu: str
    trans: str
    lang: str  # 'ru' | 'en'


def usable(hu: str, trans: str) -> bool:
    """Whether this pair is worth a learner's attention.

    Every clause here was written against real OpenSubtitles output. The rules
    are deliberately strict: throwing away a good sentence costs nothing when
    there are fifty million of them, and keeping a bad one costs the learner.
    """
    if not hu or not trans:
        return False
    if len(hu) > 120 or len(trans) > 200:
        return False
    if _URL.search(hu) or _URL.search(trans):
        return False

    # A complete sentence, not a fragment continued on the next subtitle card.
    if not _ENDS_SENTENCE.search(hu):
        return False
    if hu.startswith(("...", "…")):
        return False
    if not hu[:1].isupper():
        return False

    toks = tokens(hu)
    # The app teaches sentences in the 3–14 token band; below that there is no
    # grammar to see, above it the first encounter with a word is also a parsing
    # exercise. Matches mining.targetLength.
    if not 3 <= len(toks) <= 14:
        return False

    # Subtitle typography: a comma jammed against the next word (`Leszopták,de`)
    # is a line-join artifact, and the app would be TEACHING that spelling on a
    # card. A comma before a digit stays: `1,5` is the Hungarian decimal.
    if _COMMA_JAMMED.search(hu):
        return False

    # SHOUTING, and lines that are mostly acronyms.
    upper = sum(1 for t in toks if len(t) > 1 and t.isupper())
    if upper > 1:
        return False

    # Hungarian, spelled in Hungarian letters.
    if any(ch not in _HU_ALPHABET and ch not in _PUNCT_OK and not ch.isdigit() for ch in hu):
        return False
    # At least one accented or Hungarian-specific letter across the line is a
    # cheap, effective filter against English/German lines left in the corpus.
    if not any(ch in "áéíóöőúüűÁÉÍÓÖŐÚÜŰ" for ch in hu) and len(toks) < 5:
        return False

    # The translation has to be a translation, not a repeat of the source.
    if trans.strip().lower() == hu.strip().lower():
        return False
    if not _HAS_LETTER.search(trans):
        return False

    return aligned(hu, trans)


_COMMA_JAMMED = re.compile(r",[^\W\d_]", re.UNICODE)

# Content a beginner's first few hundred sentences should not be made of. This
# is a blunt instrument and it is meant to be: the corpus is film dialogue, the
# app is used by one person learning to order coffee, and there is no reason for
# the sweet-spot sentence rail to surface a torture scene.
#
# Two patterns, because Hungarian defeats word boundaries. An obscene stem
# rarely appears bare: it arrives wrapped in a preverb and suffixes —
# `Leszopták` is `le` + `szop` + past + 3PL, and `\bszopj` never fires on it.
# So the obscene stems match ANYWHERE in the line, and the collateral is
# accepted deliberately: losing `szarvas` (deer) and `baszk` (Basque) from a
# pool of eight million candidates costs nothing, while one of these surfacing
# as a course card costs trust. The violence/drugs list keeps its boundaries —
# those stems are short and genuinely ambiguous without them.
_UNWANTED_ANYWHERE = re.compile(
    r"kurv|bassz|basz[dtoiá]|kibasz|fasz|gec[is]|picsá|picsa|szop[jtoá]|"
    r"szar[jtdon]|köcsög|ribanc|buzi|rohadék",
    re.IGNORECASE,
)
_UNWANTED_BOUNDED = re.compile(
    r"\b("
    r"szar|öl(?:d|ni|te|lek)|megöl|gyilkos|erőszak|drog|hero(?:in|int)"
    r")\b",
    re.IGNORECASE,
)


def wholesome(hu: str) -> bool:
    return not (_UNWANTED_ANYWHERE.search(hu) or _UNWANTED_BOUNDED.search(hu))


# ── reading the OPUS moses archives ───────────────────────────────────────


def _member(zf: zipfile.ZipFile, suffix: str) -> str:
    """The archive entry whose name ends with this language suffix."""
    for name in zf.namelist():
        if name.endswith(suffix) and not name.startswith("__"):
            return name
    raise KeyError(f"no {suffix} member in {zf.filename}")


def read_pairs(archive: Path, other: str) -> Iterator[Pair]:
    """Stream (hungarian, translation) pairs out of an OPUS moses zip.

    Streamed rather than extracted: the two archives expand to about eight
    gigabytes of plain text and nothing needs them on disk.

    The moses format is two line-aligned text files. Reading them in lockstep is
    the entire parser; the value of this module is in the filtering above.
    """
    with zipfile.ZipFile(archive) as zf:
        hu_name = _member(zf, ".hu")
        tr_name = _member(zf, f".{other}")
        with zf.open(hu_name) as fh, zf.open(tr_name) as ft:
            for raw_hu, raw_tr in zip(fh, ft):
                hu = clean(raw_hu.decode("utf-8", "replace"))
                tr = clean(raw_tr.decode("utf-8", "replace"))
                if usable(hu, tr) and wholesome(hu):
                    yield Pair(hu=hu, trans=tr, lang=other)


# ── choosing what to keep ─────────────────────────────────────────────────


def _key(hu: str) -> str:
    """Deduplication key: casing and punctuation are not different sentences."""
    return " ".join(t.lower() for t in tokens(hu))


@dataclass
class Selected:
    hu: str
    ru: str
    en: str
    # How many times this sentence occurs in the corpus. This IS the measure of
    # "a phrase people actually say", and it is why the selection is worth
    # anything: a line said in two hundred different films is worth a card, and
    # a line said once in one film is a line from that film.
    count: int = 0


# A sentence has to recur before it counts as a phrase rather than a line of
# dialogue. Three is low enough to keep a very large vocabulary and high enough
# to make the majority vote below mean something.
MIN_OCCURRENCES = 3


def count_sentences(archive: Path, other: str, *, min_len: int = 0) -> Counter[str]:
    """Pass one: how often does each Hungarian sentence occur?

    Keys only, and only for pairs that survived `usable`. Counting the whole
    corpus would need the values too, and the values are what make it large.
    """
    counts: Counter[str] = Counter()
    for pair in read_pairs(archive, other):
        counts[_key(pair.hu)] += 1
    if min_len:
        for k in [k for k, v in counts.items() if v < min_len]:
            del counts[k]
    return counts


def vote_translations(
    archive: Path, other: str, wanted: set[str]
) -> dict[str, tuple[str, Counter[str]]]:
    """Pass two: the most common translation of each wanted sentence.

    **This is the alignment filter that actually works.** OpenSubtitles is
    aligned automatically, and where it fails it fails randomly — the head of a
    film pairs dialogue against translator credits, and a drifted document pairs
    every line against its neighbour. Random noise does not agree with itself.
    The true translation of a sentence that occurs forty times recurs; forty
    different misalignments do not.

    Returns, per sentence key, the best surface spelling seen and the tally of
    translations, so the caller can require a majority rather than a plurality.
    """
    out: dict[str, tuple[str, Counter[str]]] = {}
    for pair in read_pairs(archive, other):
        k = _key(pair.hu)
        if k not in wanted:
            continue
        entry = out.get(k)
        if entry is None:
            entry = (pair.hu, Counter())
            out[k] = entry
        entry[1][pair.trans.strip()] += 1
    return out


# A translation has to win its vote by this share to be trusted. Below it the
# sentence is kept and the translation is dropped: real Hungarian with no
# translation is useful, and a wrong translation is worse than none.
MIN_AGREEMENT = 0.34


def best_translation(votes: Counter[str]) -> str:
    if not votes:
        return ""
    text, n = votes.most_common(1)[0]
    total = sum(votes.values())
    if total < 2 or n / total < MIN_AGREEMENT:
        return ""
    return text


# The scan is the expensive half — two passes over 19M lines per language, about
# 47 minutes for both. The choice is arithmetic on the result. Splitting them
# means the selection can be re-tuned without paying for the scan again, which
# matters because the first tuning was wrong: recurrence alone collapsed the
# selection onto three-word utterances.
CACHE_VERSION = 1


def harvest(archives: dict[str, Path], cache: Path | None = None) -> tuple[list[Selected], dict]:
    """Every recurring sentence in the corpora, with its voted translations.

    Written to `cache` as JSONL when given, and read back from it when it is
    present and current. Delete the file to force a re-scan.
    """
    if cache and cache.exists():
        with cache.open(encoding="utf-8") as fh:
            header = json.loads(fh.readline())
            if header.get("version") == CACHE_VERSION:
                pool = [Selected(**json.loads(line)) for line in fh]
                header["from_cache"] = True
                return pool, header

    merged: dict[str, Selected] = {}
    stats: Counter[str] = Counter()

    for lang, path in archives.items():
        if not path or not path.exists():
            continue
        counts = count_sentences(path, lang)
        stats[f"usable_{lang}"] = sum(counts.values())
        stats[f"distinct_{lang}"] = len(counts)

        wanted = {k for k, n in counts.items() if n >= MIN_OCCURRENCES}
        stats[f"recurring_{lang}"] = len(wanted)

        for k, (surface, votes) in vote_translations(path, lang, wanted).items():
            cur = merged.get(k)
            if cur is None:
                cur = Selected(hu=surface, ru="", en="", count=0)
                merged[k] = cur
            cur.count += counts[k]
            text = best_translation(votes)
            if not text:
                stats[f"no_majority_{lang}"] += 1
                continue
            if lang == "ru" and not cur.ru:
                cur.ru = text
            elif lang == "en" and not cur.en:
                cur.en = text

    pool = list(merged.values())
    info = dict(stats)
    info["version"] = CACHE_VERSION
    info["pool"] = len(pool)

    if cache:
        cache.parent.mkdir(parents=True, exist_ok=True)
        with cache.open("w", encoding="utf-8") as fh:
            fh.write(json.dumps(info, ensure_ascii=False) + "\n")
            for sel in pool:
                fh.write(json.dumps(sel.__dict__, ensure_ascii=False) + "\n")
    return pool, info


# How the selection is spread across sentence length.
#
# Recurrence alone does not spread it at all: short utterances repeat far more
# than long ones, so an unconstrained top-N came out 79% three- and four-token
# sentences and gave the reader almost nothing. But the two ends serve different
# parts of the app and both are wanted:
#
#   3-5 tokens   the word inspector's examples, course-word cards, cloze frames.
#                Short is a FEATURE there: a first encounter with a word should
#                not also be a parsing exercise.
#   6-14 tokens  the reader and the n+1 rail, whose sweet spot is `mining
#                .targetLength` = [6, 14]. Starving this band starves reading.
#
# Quotas rather than a blended score, because a score would let one band eat the
# other whenever the corpus happened to favour it.
LENGTH_BANDS: tuple[tuple[int, int, float], ...] = (
    (3, 5, 0.55),
    (6, 14, 0.45),
)


def choose(pool: list[Selected], known: dict[str, int], limit: int) -> tuple[list[Selected], dict]:
    """Pick the most teachable sentences, spread across the length bands."""

    # The content filters run here AS WELL as in the scan, because the harvest
    # cache outlives them: a pool scanned under last month's filter must not
    # smuggle last month's misses past this month's rules. Belt and braces on
    # purpose — the scan is 47 minutes and this is a list comprehension.
    before = len(pool)
    pool = [s for s in pool if wholesome(s.hu) and not _COMMA_JAMMED.search(s.hu)]
    dropped = before - len(pool)

    def score(s: Selected) -> tuple:
        toks = [t.lower() for t in tokens(s.hu)]
        ranks = [known.get(t) for t in toks]
        unknown = sum(1 for r in ranks if r is None)
        seen = [r for r in ranks if r is not None]
        mean_rank = sum(seen) / len(seen) if seen else 10**6
        # Best first: has a translation, uses words a learner is likely to have,
        # is said often, and is built from common vocabulary.
        return (0 if (s.ru or s.en) else 1, unknown, -s.count, mean_rank)

    out: list[Selected] = []
    stats: Counter[str] = Counter()
    for lo, hi, share in LENGTH_BANDS:
        band = [s for s in pool if lo <= len(tokens(s.hu)) <= hi]
        band.sort(key=score)
        take = band[: int(limit * share)]
        stats[f"band_{lo}_{hi}"] = len(take)
        out.extend(take)

    # A band that could not fill its quota releases it to the others rather than
    # shrinking the result.
    if len(out) < limit:
        have = {id(s) for s in out}
        rest = [s for s in pool if id(s) not in have]
        rest.sort(key=score)
        out.extend(rest[: limit - len(out)])

    stats["selected"] = len(out)
    stats["refiltered"] = dropped
    stats["with_ru"] = sum(1 for s in out if s.ru)
    stats["with_en"] = sum(1 for s in out if s.en)
    return out, dict(stats)


def select(
    archives: dict[str, Path],
    known: dict[str, int],
    limit: int,
    cache: Path | None = None,
) -> tuple[list[Selected], dict[str, int]]:
    """Harvest (cached) then choose. The entry point the build calls."""
    pool, harvest_stats = harvest(archives, cache)
    picked, choose_stats = choose(pool, known, limit)
    return picked, {**harvest_stats, **choose_stats}
