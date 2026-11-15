"""Authored reading texts, three per unit: short, medium, long (ADR-075, ADR-076).

Every basic-tier unit (24 core + 12 themes) gets a NARROW-READING CLUSTER: three
texts about one topic that differ in size AND genre, so the unit's words come back
in a note, a conversation and a story (R6, R21; research/22-text-length-and-genre.md).

SET CODES (they never reach learners; they only let an import sync in place):
    <unit>-dialogue = SHORT     <unit> = MEDIUM     <unit>-long = LONG
    <unit>-talk = TALK          <unit>-talk-<slug> = MINI   (colloquial register, ADR-077)
The codes are the old «Диалог» / «Рассказ» codes on purpose; what a learner sees is
the text's own title and the tier/genre/words fields.

OUTPUT (ADR-076): the texts are NOT written into the language pack. `--export`
validates them and writes ONE committed, deterministic catalogue,
backend/internal/readsets/catalog.json, which the Go binary embeds. `--check-export`
byte-compares it with a fresh export (no pack needed; `make texts-check`).

SOURCE: `langpack/texts/<unit>.json`

    {"short"|"medium"|"long": {
        "genre": "note", "checked": "ai",
        "title": {"ru": "...", "en": "..."},
        "names": ["Kata", ...],                 # proper names the text uses
        "paras": [[{"hu": "...", "ru": "...", "en": "..."}, ...], ...]}}

A `dialogue` text: one sentence entry is one turn (the app adds the "– ").
`--require-all` demands a full valid triplet for every unit.

TOKEN RULE V1: a word passes if it is (a) an attested non-generated wordform,
(b) a `generated` paradigm form (reading is recognition, never production; the
count per text is reported), (c) a row of pipeline/texts_allow.tsv, or (d) a name
declared in the text's `names`. (b)-(d) are never written to `wordform`.
V2 content lemma in the unit's band or taught earlier; V3 frequency gate (stories.py
mechanism); V4 size/shape per band; V5 (require-all) three different genres,
at most one dialogue, a prose long text; V6 ru and en for every sentence.

COLLOQUIAL MATERIAL (ADR-077): `langpack/talk/<unit>.json` holds one TALK text (a 4th
text per unit: dialogue|phone|voicemsg|chat) and 5-10 short MINI dialogues, all in real
spoken Hungarian. Their words may also come from pipeline/texts_colloquial.tsv
`[allowed]` (level-gated: an A1 text only uses A1 rows); `[banned]` stems fail in EVERY
text. V4c sizes below; every mini needs >= 1 colloquial marker, markers <= 20% of words.
`--require-talk` demands a talk text and >= 5 minis for every unit (separate from
`--require-all`, so the standard texts validate alone).

    python -m pipeline.texts PACK --brief m1-u1-koszones --tier long|talk|mini
    python -m pipeline.texts PACK [--require-all] [--require-talk] [--export]
    python -m pipeline.texts --check-export
"""

from __future__ import annotations

import argparse
import json
import re
import sqlite3
import sys
from pathlib import Path

from . import course, dialogues, stories
from . import reading_sets as rs

TEXTS_DIR = Path(__file__).resolve().parents[1] / "texts"
TALK_DIR = Path(__file__).resolve().parents[1] / "talk"
ALLOW_FILE = Path(__file__).resolve().parent / "texts_allow.tsv"
COLLOQUIAL_FILE = Path(__file__).resolve().parent / "texts_colloquial.tsv"
GUIDE_PATH = ".agent-context/COLLOQUIAL_GUIDE.md"
MIN_REASON = 10

TIERS = ("short", "medium", "long")
# Catalogue ord = unitOrd*100 + slot (+ index for a mini text); `talk`/`mini` are the
# colloquial tiers of ADR-076, supported by the exporter before any text exists.
CATALOG_PATH = Path(__file__).resolve().parents[2] / "backend" / "internal" / "readsets" / "catalog.json"
CATALOG_FORMAT = 1
ORD_SLOT = {"short": 1, "medium": 2, "long": 3, "talk": 4}
ORD_MINI = 10
REGISTERS = ("standard", "colloquial")
CHECKED = ("ai", "native")

# Genre id -> (label ru, label en, one-line hint for the brief). The ids are stored
# in `reading_set.genre`; the Vue layer keeps its own ru/en key per id. Extend HERE.
GENRES: dict[str, tuple[str, str, str]] = {
    "sms": ("SMS", "Text message", "two or three phone messages, one sender"),
    "note": ("Записка", "Note", "a note left on a door or table"),
    "notice": ("Объявление", "Notice", "a sign or ad: what, where, when, price"),
    "postcard": ("Открытка", "Postcard", "a few lines from a trip, signed"),
    "list": ("Список", "List", "a shopping or to-do list with short lines"),
    "dialogue": ("Диалог", "Dialogue", "turns between two people, one per entry"),
    "email": ("Письмо", "Email", "a short email: greeting, news, request, sign-off"),
    "description": ("Описание", "Description", "a place, a person or an object described"),
    "chat": ("Переписка", "Chat", "a messenger exchange between two people"),
    "timetable": ("Расписание", "Timetable", "a timetable or schedule plus a short exchange about it"),
    "myday": ("Мой день", "My day", "first person: how I spend (or spent) my day"),
    "story": ("Рассказ", "Story", "third person, a small plot with a start and an end"),
    "diary": ("Дневник", "Diary", "first-person entries"),
    "blog": ("Блог", "Blog", "a blog post addressed to readers"),
    "phone": ("Звонок", "Phone call", "a phone call, both sides, one turn per entry"),
    "voicemsg": ("Голосовое", "Voice message", "one speaker leaving a spoken message (prose)"),
}
MENU = {  # the genres the brief offers per tier
    "short": ("sms", "note", "notice", "postcard", "list", "dialogue"),
    "medium": ("dialogue", "email", "description", "chat", "timetable"),
    "long": ("myday", "story", "diary", "blog"),
}
PROSE = ("story", "myday", "diary", "blog", "description", "email")  # a long text is one of these
TURN_BASED = ("dialogue",)  # paragraph limits do not apply: one entry is one turn

# Colloquial register (ADR-077).
TALK_GENRES = ("dialogue", "phone", "voicemsg", "chat")
TALK_PROSE = ("voicemsg",)  # the other talk genres are turn-based
LEVELS = ("A1", "A2", "B1")
KINDS = ("particle", "interjection", "greeting", "clipping", "contraction", "ellipsis", "chat")
WHO = ("A", "B", "C")
SLUG = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
MINI_COUNT = (5, 10)
# V4c: size per band group. Mini: (turns, max words a turn, total words); talk: total words.
MINI_SIZE = {"a1": ((2, 6), 6, (12, 40)), "a2": ((4, 8), 8, (30, 70)), "b": ((6, 10), 10, (50, 110))}
TALK_SIZE = {"a1": (60, 120), "a2": (100, 180), "b": (150, 260)}
VOICEMSG_PARAS = (1, 4)
MARKER_MAX = 0.20  # caricature guard: colloquial markers / running words (greeting-kind rows excluded)
NOT_CAPPED = ("greeting",)  # social formulas are not "stuffing": they count for the minimum only
# V4. Word counts (running words as `_WORD` sees them): an INFERENCE from the
# coverage and extensive-reading literature, not a published table (research/22).
RANGES = {
    "a1": {"short": (30, 70), "medium": (90, 160), "long": (180, 280)},
    "a2": {"short": (50, 100), "medium": (140, 220), "long": (250, 380)},
    "b": {"short": (70, 130), "medium": (180, 300), "long": (320, 480)},
}
MAX_SENTENCE = {"a1": 10, "a2": 12, "b": 14}
PARAS = {"short": (1, 5), "medium": (2, 8), "long": (3, 16)}
MAX_PARA_SENTENCES = 6  # a wall of text is hard to read: split paragraphs of a prose text
RECYCLE_MIN, RECYCLE_TARGET = 3, 0.70

PAST_FROM_ORD = 304  # m3-u4-posta: where the course first teaches the past tense

_WORD = dialogues._WORD
_ALLOWED = dialogues._ALLOWED


def set_code(unit_code: str, tier: str) -> str:
    return {"short": f"{unit_code}-dialogue", "medium": unit_code, "long": f"{unit_code}-long"}[tier]


def size_group(unit: dict) -> str:
    if unit["track"] != "core" or unit["month"] >= 5:
        return "b"
    return "a1" if unit["month"] <= 2 else "a2"


def past_taught(unit: dict) -> bool:
    return unit["track"] != "core" or unit["ord"] >= PAST_FROM_ORD


def count_words(hu: str) -> int:
    return len(_WORD.findall(hu))


def _units() -> dict[str, dict]:
    return {u["code"]: u for u in dialogues._basic_units()}


# ── loading ────────────────────────────────────────────────────────────────

def load(directory: Path | None = None,
         talk_directory: Path | None = None) -> tuple[dict[str, dict[str, dict]], list[str]]:
    """({unit: {tier: text}}, problems). A text is the JSON object plus `unit`,
    `tier`; sentences are normalised to dicts with hu/ru/en. Shape errors are
    reported, the offending text is dropped. With no `directory` the colloquial
    material of `langpack/talk` is merged in under the keys `talk` and `mini-<slug>`
    (ADR-077); an explicit `directory` reads the standard texts only unless a
    `talk_directory` is passed too."""
    if directory is None and talk_directory is None:
        talk_directory = TALK_DIR  # looked up per call so tests can point it elsewhere
    directory = directory or TEXTS_DIR
    out: dict[str, dict[str, dict]] = {}
    bad: list[str] = []
    units = _units()
    for path in sorted(directory.glob("*.json")):
        code = path.stem
        if code not in units:
            bad.append(f"{path.name}: no such basic-tier unit")
            continue
        try:
            raw = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError) as e:
            bad.append(f"{path.name}: unreadable JSON ({e})")
            continue
        if not isinstance(raw, dict):
            bad.append(f"{path.name}: top level must be an object")
            continue
        for tier, t in raw.items():
            where = f"{code}.{tier}"
            if tier not in TIERS:
                bad.append(f"{code}: unknown tier {tier!r}, want {'|'.join(TIERS)}")
                continue
            errs = _shape(where, t)
            bad.extend(errs)
            if not errs:
                out.setdefault(code, {})[tier] = {**t, "unit": code, "tier": tier}
    if talk_directory is not None and talk_directory.exists():
        talk, tbad = load_talk(talk_directory)
        bad.extend(tbad)
        for code, items in talk.items():
            out.setdefault(code, {}).update(items)
    return out, bad


def _entry_shape(w: str, s: object, who: str) -> list[str]:
    """One sentence/turn. `who`: 'req' (a turn of a mini), 'opt' (a talk entry)."""
    want = {"hu", "ru", "en"}
    if not isinstance(s, dict) or not want <= set(s) <= want | {"who"} or (who == "req" and "who" not in s):
        return [f"{w}: an entry is exactly {{hu, ru, en}}" + (" plus who (A|B|C)" if who == "req" else "")]
    if not all(isinstance(s[k], str) and s[k].strip() for k in ("hu", "ru", "en")):
        return [f"{w}: hu, ru and en must all be present (V6)"]
    if "who" in s and s["who"] not in WHO:
        return [f"{w}: who must be one of {'|'.join(WHO)}"]
    return []


def _head_shape(where: str, t: dict, extra: set[str]) -> list[str]:
    e = [f"{where}: unknown key {k!r}" for k in sorted(set(t) - {"genre", "title", "names", "checked"} - extra)]
    if t.get("checked", "ai") not in CHECKED:
        e.append(f"{where}: checked must be ai|native")
    for k in ("title", "scene"):
        if k in extra or k == "title":
            v = t.get(k)
            if not (isinstance(v, dict) and all(isinstance(v.get(x), str) and v[x].strip() for x in ("ru", "en"))):
                e.append(f"{where}: {k} needs ru and en")
    names = t.get("names", [])
    if not (isinstance(names, list) and all(isinstance(n, str) and n.strip() for n in names)):
        e.append(f"{where}: names must be a list of strings")
    return e


def load_talk(directory: Path) -> tuple[dict[str, dict[str, dict]], list[str]]:
    """({unit: {'talk': text, 'mini-<slug>': text, ...}}, problems) from langpack/talk.
    A talk text and every mini are normalised to the shape of a standard text
    (`paras`, `unit`, `tier`, `register: colloquial`); a mini is one paragraph of
    turns with `slug`, `index` (file order) and `scene`."""
    out: dict[str, dict[str, dict]] = {}
    bad: list[str] = []
    units = _units()
    for path in sorted(directory.glob("*.json")):
        code = path.stem
        if code not in units:
            bad.append(f"talk/{path.name}: no such basic-tier unit")
            continue
        try:
            raw = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError) as e:
            bad.append(f"talk/{path.name}: unreadable JSON ({e})")
            continue
        if not isinstance(raw, dict) or not set(raw) <= {"talk", "dialogues"}:
            bad.append(f"talk/{path.name}: top level is an object with `talk` and/or `dialogues`")
            continue
        t = raw.get("talk")
        if t is not None:
            where = f"{code}.talk"
            errs = _head_shape(where, t, {"paras"}) if isinstance(t, dict) else [f"{where}: must be an object"]
            if isinstance(t, dict):
                if t.get("genre") not in TALK_GENRES:
                    errs.append(f"{where}: genre {t.get('genre')!r} is not one of {', '.join(TALK_GENRES)}")
                paras = t.get("paras")
                if not (isinstance(paras, list) and paras and all(isinstance(p, list) and p for p in paras)):
                    errs.append(f"{where}: paras must be a non-empty list of non-empty lists")
                else:
                    for pi, para in enumerate(paras):
                        for si, s in enumerate(para):
                            errs.extend(_entry_shape(f"{where}[{pi}][{si}]", s, "opt"))
                    whos = [("who" in s) for para in paras for s in para if isinstance(s, dict)]
                    if any(whos) and not all(whos):
                        errs.append(f"{where}: either every entry has `who` or none does")
            bad.extend(errs)
            if not errs:
                out.setdefault(code, {})["talk"] = {**t, "unit": code, "tier": "talk", "register": "colloquial"}
        ds = raw.get("dialogues", [])
        if not isinstance(ds, list):
            bad.append(f"{code}.dialogues: must be a list")
            continue
        for i, d in enumerate(ds):
            slug = d.get("slug") if isinstance(d, dict) else None
            where = f"{code}.dialogues[{i}]"
            if not isinstance(d, dict):
                bad.append(f"{where}: must be an object")
                continue
            errs = _head_shape(where, d, {"slug", "scene", "turns"})
            if not (isinstance(slug, str) and SLUG.match(slug)):
                errs.append(f"{where}: slug must be kebab-case ([a-z0-9-]), got {slug!r}")
            elif f"mini-{slug}" in out.get(code, {}):
                errs.append(f"{where}: duplicate slug {slug!r} in {code}")
            turns = d.get("turns")
            if not (isinstance(turns, list) and turns):
                errs.append(f"{where}: turns must be a non-empty list")
            else:
                for ti, s in enumerate(turns):
                    errs.extend(_entry_shape(f"{where}.turns[{ti}]", s, "req"))
            bad.extend(errs)
            if not errs:
                out.setdefault(code, {})[f"mini-{slug}"] = {
                    "genre": "dialogue", "checked": d.get("checked", "ai"), "title": d["title"],
                    "scene": d["scene"], "names": d.get("names", []), "paras": [turns],
                    "unit": code, "tier": "mini", "register": "colloquial", "slug": slug,
                    "index": i}
    return out, bad


def _shape(where: str, t: object) -> list[str]:
    if not isinstance(t, dict):
        return [f"{where}: must be an object"]
    e: list[str] = []
    for k in set(t) - {"genre", "title", "names", "checked", "paras"}:
        e.append(f"{where}: unknown key {k!r}")
    if t.get("genre") not in GENRES:
        e.append(f"{where}: genre {t.get('genre')!r} is not one of {', '.join(GENRES)}")
    if t.get("checked", "ai") not in CHECKED:
        e.append(f"{where}: checked must be ai|native")
    title = t.get("title")
    if not (isinstance(title, dict) and all(isinstance(title.get(k), str) and title[k].strip()
                                            for k in ("ru", "en"))):
        e.append(f"{where}: title needs ru and en")
    names = t.get("names", [])
    if not (isinstance(names, list) and all(isinstance(n, str) and n.strip() for n in names)):
        e.append(f"{where}: names must be a list of strings")
    paras = t.get("paras")
    if not (isinstance(paras, list) and paras and all(isinstance(p, list) and p for p in paras)):
        return e + [f"{where}: paras must be a non-empty list of non-empty lists"]
    for pi, para in enumerate(paras):
        for si, s in enumerate(para):
            w = f"{where}[{pi}][{si}]"
            if not isinstance(s, dict) or set(s) != {"hu", "ru", "en"}:
                e.append(f"{w}: a sentence is exactly {{hu, ru, en}}")
            elif not all(isinstance(s[k], str) and s[k].strip() for k in ("hu", "ru", "en")):
                e.append(f"{w}: hu, ru and en must all be present (V6)")  # V6
    return e


def lines_of(text: dict) -> list[dict]:
    """[{para, hu, ru, en}] in reading order."""
    return [{"para": p, **s} for p, para in enumerate(text["paras"]) for s in para]


def full_title(text: dict) -> tuple[str, str]:
    """The text's own title. The genre is NOT put in front of it: the library and the
    unit page show the genre as its own label (library.genre.<id>), so a prefixed title
    stuttered («Записка: Записка на двери», «Мой день: Мой обычный день»)."""
    return text["title"]["ru"].strip(), text["title"]["en"].strip()


# ── checks that need no pack: V4, V5, V6, characters ───────────────────────

def size_problems(unit: dict, tier: str, genre: str, hu_by_para: list[list[str]],
                  label: str) -> list[str]:
    """V4 for one text given its Hungarian sentences grouped by paragraph."""
    g = size_group(unit)
    flat = [hu for para in hu_by_para for hu in para]
    out: list[str] = []
    n = sum(count_words(hu) for hu in flat)
    lo, hi = RANGES[g][tier]
    if not lo <= n <= hi:
        out.append(f"{label}: {n} words, a {tier} text of {unit['code']} wants {lo}-{hi}")
    cap = MAX_SENTENCE[g]
    for i, hu in enumerate(flat):
        k = count_words(hu)
        if not 1 <= k <= cap:
            out.append(f"{label}[{i}]: {k} words in a sentence, at most {cap}: {hu!r}")
    plo, phi = PARAS[tier]
    if genre not in TURN_BASED and not plo <= len(hu_by_para) <= phi:
        out.append(f"{label}: {len(hu_by_para)} paragraphs, a {tier} text wants {plo}-{phi}")
    if genre not in TURN_BASED:
        for pi, para in enumerate(hu_by_para):
            if len(para) > MAX_PARA_SENTENCES:
                out.append(f"{label}: paragraph {pi} has {len(para)} sentences, at most {MAX_PARA_SENTENCES}")
    return out


def data_problems(texts: dict[str, dict[str, dict]], require_all: bool = False,
                  require_talk: bool = False, banned: list[str] | None = None) -> list[str]:
    out: list[str] = []
    units = _units()
    banned = load_colloquial()[1] if banned is None else banned
    for code, tiers in texts.items():
        unit = units[code]
        for key, t in tiers.items():
            out.extend(banned_problems(f"{code}.{key}", [s["hu"] for s in lines_of(t)], banned))
        std = {k: t for k, t in tiers.items() if k in TIERS}
        for tier, t in std.items():
            label = f"{code}.{tier}"
            ls = lines_of(t)
            out.extend(size_problems(unit, tier, t["genre"],
                                     [[s["hu"] for s in p] for p in t["paras"]], label))
            seen: set[str] = set()
            for i, s in enumerate(ls):
                if not _ALLOWED.match(s["hu"]):
                    out.append(f"{label}[{i}]: characters outside letters and punctuation "
                               f"(digits? a speaker label?): {s['hu']!r}")
                if s["hu"] in seen:
                    out.append(f"{label}[{i}]: a sentence repeated: {s['hu']!r}")
                seen.add(s["hu"])
            if tier == "long" and t["genre"] not in PROSE:
                out.append(f"{label}: a long text is connected prose ({'|'.join(PROSE)}), "
                           f"not {t['genre']!r}")
            if t["genre"] in ("phone", "voicemsg"):
                out.append(f"{label}: genre {t['genre']!r} is for the colloquial talk text only")
        out.extend(_triplet_problems(code, std, strict=require_all))
    if require_all:
        for code in units:
            if not any(k in TIERS for k in texts.get(code, {})):
                out.append(f"{code}: no langpack/texts/{code}.json (a legacy text is not accepted)")
    out.extend(talk_problems(texts, require_talk))
    return out


def _triplet_problems(code: str, tiers: dict[str, dict], strict: bool) -> list[str]:
    """V5: three tiers, three different genres, at most one dialogue."""
    out: list[str] = []
    if strict:
        for tier in TIERS:
            if tier not in tiers:
                out.append(f"{code}: the {tier} text is missing")
    genres = [t["genre"] for k, t in tiers.items() if k in TIERS]
    if len(set(genres)) != len(genres) and (strict or len(tiers) > 1):
        out.append(f"{code}: genres must differ between tiers, got {', '.join(genres)}")
    if genres.count("dialogue") > 1:
        out.append(f"{code}: at most one dialogue per unit")
    return out


# ── the colloquial material: V4c and the banned list (ADR-077) ─────────────

def banned_hits(hu: str, banned: list[str]) -> list[str]:
    """The tokens of `hu` that a banned rule matches. A rule is a lower-case prefix, or
    an exact form when it ends with `$` (for short words that start longer, innocent
    ones: `szar$` must not ban szarvas)."""
    hits = []
    for tok in _WORD.findall(hu):
        for piece in re.split(r"[-']", tok):
            low = piece.lower()
            if any(low == r[:-1] if r.endswith("$") else low.startswith(r) for r in banned):
                hits.append(piece)
    return hits


def banned_problems(label: str, hu_lines: list[str], banned: list[str]) -> list[str]:
    return [f"{label}[{i}]: banned (vulgar or inappropriate) word {w!r} in {hu!r}"
            for i, hu in enumerate(hu_lines) for w in banned_hits(hu, banned)]


def talk_problems(texts: dict[str, dict[str, dict]], require_talk: bool = False) -> list[str]:
    """V4c for the talk text and the minis of every unit (no pack needed), the unique
    slugs, 5-10 minis, no turn repeated across a unit's minis."""
    out: list[str] = []
    units = _units()
    for code, tiers in texts.items():
        unit = units[code]
        g = size_group(unit)
        talk = tiers.get("talk")
        minis = sorted((t for t in tiers.values() if t.get("tier") == "mini"), key=lambda t: t["index"])
        if talk:
            label = f"{code}.talk"
            hus = [[s["hu"] for s in p] for p in talk["paras"]]
            n = sum(count_words(h) for p in hus for h in p)
            lo, hi = TALK_SIZE[g]
            if not lo <= n <= hi:
                out.append(f"{label}: {n} words, a talk text of {code} wants {lo}-{hi}")
            cap = MAX_SENTENCE[g]
            for i, h in enumerate(h for p in hus for h in p):
                if not 1 <= count_words(h) <= cap:
                    out.append(f"{label}[{i}]: {count_words(h)} words in an entry, at most {cap}: {h!r}")
                if not _ALLOWED.match(h):
                    out.append(f"{label}[{i}]: characters outside letters and punctuation: {h!r}")
            plo, phi = VOICEMSG_PARAS
            if talk["genre"] in TALK_PROSE and not plo <= len(hus) <= phi:
                out.append(f"{label}: {len(hus)} paragraphs, a voice message wants {plo}-{phi}")
        (tlo, thi), cap, (wlo, whi) = MINI_SIZE[g]
        seen: dict[str, str] = {}
        for m in minis:
            label = f"{code}.mini[{m['slug']}]"
            turns = m["paras"][0]
            n = sum(count_words(t["hu"]) for t in turns)
            if not tlo <= len(turns) <= thi:
                out.append(f"{label}: {len(turns)} turns, a mini of {code} wants {tlo}-{thi}")
            if not wlo <= n <= whi:
                out.append(f"{label}: {n} words, a mini of {code} wants {wlo}-{whi}")
            for i, t in enumerate(turns):
                k = count_words(t["hu"])
                if not 1 <= k <= cap:
                    out.append(f"{label}[{i}]: {k} words in a turn, at most {cap}: {t['hu']!r}")
                if not _ALLOWED.match(t["hu"]):
                    out.append(f"{label}[{i}]: characters outside letters and punctuation: {t['hu']!r}")
                if t["hu"] in seen and seen[t["hu"]] != m["slug"]:
                    out.append(f"{label}[{i}]: the turn {t['hu']!r} is also in mini {seen[t['hu']]!r}")
                seen.setdefault(t["hu"], m["slug"])
        if len(minis) > MINI_COUNT[1] or (require_talk and len(minis) < MINI_COUNT[0]):
            out.append(f"{code}: {len(minis)} minis, a unit wants {MINI_COUNT[0]}-{MINI_COUNT[1]}")
    if require_talk:
        for code in units:
            if "talk" not in texts.get(code, {}):
                out.append(f"{code}: no talk text (langpack/talk/{code}.json)")
            if code not in texts:
                out.append(f"{code}: no minis (langpack/talk/{code}.json)")
    return list(dict.fromkeys(out))


# ── the lexicon: V1 ────────────────────────────────────────────────────────

def _rows(path: Path, section: str, want: int) -> list[list[str]]:
    """Tab-separated rows of one section of the allow file. Sections start with a
    `[forms]` or `[band-exempt]` line (rows before any marker are `forms`)."""
    out: list[list[str]] = []
    if not path.exists():
        return out
    cur = "forms"
    for raw in path.read_text(encoding="utf-8").splitlines():
        t = raw.strip()
        if not t or t.startswith("#"):
            continue
        if t in ("[forms]", "[band-exempt]"):
            cur = t.strip("[]")
            continue
        if cur != section or raw.startswith(("form\t", "lemma\t")):
            continue
        cols = [c.strip() for c in raw.split("\t")]
        if len(cols) < want or (section == "band-exempt" and len(cols[3]) < MIN_REASON):
            raise ValueError(f"{path.name} [{section}]: want {want} columns"
                             + (f" and a reason of >= {MIN_REASON} characters" if section == "band-exempt" else "")
                             + f": {raw!r}")
        out.append(cols)
    return out


def load_allow(path: Path = ALLOW_FILE) -> dict[str, list[tuple[str, str, str]]]:
    """{form (lower): [(lemma, upos, unit or '*')]} from the `[forms]` section."""
    out: dict[str, list[tuple[str, str, str]]] = {}
    for cols in _rows(path, "forms", 4):
        out.setdefault(cols[0].lower(), []).append((cols[1], cols[2], cols[3] or "*"))
    return out


def load_band_exempt(path: Path = ALLOW_FILE) -> dict[str, list[tuple[str, str]]]:
    """{lemma: [(upos or '*', unit or '*')]} from `[band-exempt]`: lemmas exempt from
    the V2 band check only (V3 still counts them)."""
    out: dict[str, list[tuple[str, str]]] = {}
    for cols in _rows(path, "band-exempt", 4):
        out.setdefault(cols[0], []).append((cols[1] or "*", cols[2] or "*"))
    return out


def load_colloquial(path: Path | None = None) -> tuple[dict[str, list[dict]], list[str]]:
    """(allowed, banned) from pipeline/texts_colloquial.tsv (ADR-077).
    `[allowed]` rows: form, lemma or '-', upos or '-', kind, level, ru, en, reason
    (>= MIN_REASON characters) -> {form (lower): [row]}. `[banned]` rows: a lower-case
    stem (prefix match; a trailing `$` means the exact form), optional note column."""
    path = path or COLLOQUIAL_FILE
    allowed: dict[str, list[dict]] = {}
    banned: list[str] = []
    if not path.exists():
        return allowed, banned
    cur = None
    for raw in path.read_text(encoding="utf-8").splitlines():
        t = raw.strip()
        if not t or t.startswith("#"):
            continue
        if t in ("[allowed]", "[banned]"):
            cur = t.strip("[]")
            continue
        if cur is None:
            raise ValueError(f"{path.name}: a row before any [allowed] / [banned] marker: {raw!r}")
        cols = [c.strip() for c in raw.split("\t")]
        if cur == "banned":
            banned.append(cols[0].lower())
            continue
        if cols[0] == "form":
            continue
        if len(cols) < 8 or cols[3] not in KINDS or cols[4] not in LEVELS \
                or not (cols[5] and cols[6]) or len(cols[7]) < MIN_REASON:
            raise ValueError(f"{path.name} [allowed]: want form, lemma|-, upos|-, kind ({'|'.join(KINDS)}), "
                             f"level ({'|'.join(LEVELS)}), ru, en, reason >= {MIN_REASON} characters: {raw!r}")
        allowed.setdefault(cols[0].lower(), []).append({
            "lemma": None if cols[1] == "-" else cols[1], "upos": "" if cols[2] == "-" else cols[2],
            "kind": cols[3], "level": cols[4], "ru": cols[5], "en": cols[6], "reason": cols[7]})
    return allowed, banned


class TextLexicon(stories.FreqLexicon):
    """The story lexicon plus the V1 sources beyond attested forms."""

    def __init__(self, conn: sqlite3.Connection, allow: dict | None = None,
                 exempt: dict | None = None, colloquial: dict | None = None) -> None:
        super().__init__(conn)
        self.allow = load_allow() if allow is None else allow
        self.exempt = load_band_exempt() if exempt is None else exempt
        self.coll = load_colloquial()[0] if colloquial is None else colloquial
        self._gen: dict[str, list] = {}

    def _generated(self, token: str) -> list:
        if token not in self._gen:
            variants = list(dict.fromkeys([token, token.lower()]))
            ph = ",".join("?" * len(variants))
            reads = list(self.conn.execute(
                "SELECT DISTINCT l.id, l.lemma, l.upos, l.cefr FROM wordform w "
                f"JOIN lemma l ON l.id = w.lemma_id WHERE w.form IN ({ph}) AND w.source = 'generated'",
                variants))
            common = [r for r in reads if r[2] != "PROPN"]
            self._gen[token] = common or reads
        return self._gen[token]

    def _allowed(self, token: str, unit_code: str) -> list:
        out = []
        for lemma, upos, unit in self.allow.get(token.lower(), []):
            if unit not in ("*", unit_code):
                continue
            row = self.conn.execute(
                "SELECT id, lemma, upos, cefr FROM lemma WHERE lemma = ? AND (? = '' OR upos = ?) "
                "ORDER BY freq_rank IS NULL, freq_rank LIMIT 1", (lemma, upos, upos)).fetchone()
            out.append(tuple(row) if row else (-1, lemma, upos or "X", None))
        return out

    def coll_row(self, token: str) -> dict | None:
        """The lowest-level colloquial row of a token, whatever the text's band."""
        rows = self.coll.get(token.lower())
        return min(rows, key=lambda r: LEVELS.index(r["level"])) if rows else None

    def _colloquial(self, token: str, band: str | None) -> list:
        """Readings of a colloquial row at a level <= `band`. A row with lemma '-' (or a
        lemma the pack does not have) reads as the lemma '-': exempt from V2/V3."""
        row = self.coll_row(token)
        if band is None or row is None or rs._rank(row["level"]) > rs._rank(band):
            return []
        if row["lemma"]:
            hit = self.conn.execute(
                "SELECT id, lemma, upos, cefr FROM lemma WHERE lemma = ? AND (? = '' OR upos = ?) "
                "ORDER BY freq_rank IS NULL, freq_rank LIMIT 1", (row["lemma"], row["upos"], row["upos"])).fetchone()
            if hit:
                return [tuple(hit)]
        return [(-1, "-", row["upos"] or "X", None)]

    def band_exempt(self, lemma: str, upos: str, unit_code: str) -> bool:
        return any(u in ("*", upos) and sc in ("*", unit_code)
                   for u, sc in self.exempt.get(lemma, []))

    def resolve(self, piece: str, unit_code: str, names: set[str],
                band: str | None = None) -> tuple[str, list]:
        """(kind, readings): kind is name | attested | generated | allow | colloquial | none."""
        if piece in names and piece[:1].isupper():
            return "name", [(-1, piece, "PROPN", None)]
        r = self.readings(piece)
        if r:
            return "attested", r
        g = self._generated(piece)
        if g:
            return "generated", g
        a = self._allowed(piece, unit_code)
        if a:
            return "allow", a
        c = self._colloquial(piece, band)
        if c:
            return "colloquial", c
        return "none", []

    def hint(self, piece: str, unit_code: str) -> str:
        """For a rejected token: the closest pack lemmas (same first 4+ letters) and the
        exact allow-row to paste, so an author fixes a form in seconds (ADR-077)."""
        low = piece.lower()
        pre = low[:4] if len(low) >= 4 else low
        stems = [low[:i] for i in range(3, len(low))]  # a lemma that is a prefix of the form
        ph = ",".join("?" * len(stems))
        cands = [(lm, up) for lm, up in self.conn.execute(
            "SELECT lemma, upos FROM lemma WHERE upos <> 'PROPN' AND ((lemma >= ? AND lemma < ?) "
            + (f"OR lemma IN ({ph})" if stems else "") + ") ORDER BY freq_rank IS NULL, freq_rank LIMIT 5",
            (pre, pre + "\U0010ffff", *stems))]
        top = cands[0] if cands else ("<lemma>", "<UPOS>")
        near = ("closest pack lemmas: " + ", ".join(f"{lm}/{up}" for lm, up in cands)) if cands \
            else "no pack lemma starts with " + repr(pre)
        return (f"hint: {near}. A regular form of one of them? paste into pipeline/texts_allow.tsv [forms]: "
                f"{low}\t{top[0]}\t{top[1]}\t{unit_code}\t<why it is regular, >= {MIN_REASON} chars>"
                "  (spoken particle or clipping: pipeline/texts_colloquial.tsv [allowed])")


def name_forms(names: list[str]) -> set[str]:
    return {p for n in names for p in _WORD.findall(n)}


def check_text(lex: TextLexicon, unit: dict, band: str, names: list[str],
               hu_lines: list[str], register: str = "standard") -> tuple[dict, list[str]]:
    """(report, problems) for one text: V1 + V2 + V3 over its running words. A
    `colloquial` text (talk/mini, ADR-077): colloquial rows above the band fail, lemmas
    taught earlier and colloquial markers are V3-exempt, and markers must be present
    (>= 1) without exceeding MARKER_MAX of the running words."""
    colloquial = register == "colloquial"
    own = {w[0] if isinstance(w, tuple) else w for w in unit["words"]}
    taught = dialogues._unit_words(unit)
    nm = name_forms(names)
    limit = rs._rank(band)
    topn = stories.top_n(unit)
    problems: list[str] = []
    gen: dict[str, int] = {}
    allow: dict[str, int] = {}
    coll: dict[str, int] = {}
    counted = top = exempt = tokens = markers = capped = 0
    misses: dict[str, int] = {}
    lemma_hits: dict[str, int] = {}
    for hu in hu_lines:
        for tok in _WORD.findall(hu):
            for piece in (tok.split("-") if "-" in tok else [tok]):
                tokens += 1
                kind, reads = lex.resolve(piece, unit["code"], nm, band)
                row = lex.coll_row(piece)
                row_ok = row is not None and rs._rank(row["level"]) <= limit
                if row is not None and not row_ok and (colloquial or kind == "none"):
                    problems.append(f"{piece!r}: colloquial item of level {row['level']} in a {band} text "
                                    f"(texts_colloquial.tsv); an {band} text may only use rows up to {band}")
                    continue  # one clear message; do not let the same word also trip V2/V3
                elif kind == "none":
                    problems.append(f"{piece!r}: not attested, not generated, not in the allow-list "
                                    f"and not a declared name\n      {lex.hint(piece, unit['code'])}")
                    continue
                if row_ok:
                    markers += 1
                    capped += row["kind"] not in NOT_CAPPED
                if kind == "name":
                    exempt += 1
                    continue
                if kind == "generated":
                    gen[piece.lower()] = gen.get(piece.lower(), 0) + 1
                elif kind == "allow":
                    allow[piece.lower()] = allow.get(piece.lower(), 0) + 1
                elif kind == "colloquial":
                    coll[piece.lower()] = coll.get(piece.lower(), 0) + 1
                reads = [r for r in reads if r[1] != "-"]
                if not reads:  # a colloquial row with no known lemma: exempt from V2/V3
                    exempt += 1
                    continue
                for _i, lemma, _u, _c in reads:
                    if lemma in own:
                        lemma_hits[lemma] = lemma_hits.get(lemma, 0) + 1
                        break
                if not row_ok and not any(u not in rs.CONTENT_UPOS or rs._rank(c) <= limit or lm in taught
                           or lex.band_exempt(lm, u, unit['code'])
                           for _i, lm, u, c in reads):
                    lem = ", ".join(sorted({f"{lm}/{c or '?'}" for _i, lm, _u, c in reads}))
                    problems.append(f"{piece!r}: lemma above {band} ({lem})")
                if any(u == "PROPN" or lm in own for _i, lm, u, _c in reads):
                    exempt += 1
                    continue
                if colloquial and (row_ok or any(lm in taught for _i, lm, _u, _c in reads)):
                    exempt += 1  # V3 for talk/mini: earlier-taught lemmas and markers are free
                    continue
                best = min(reads, key=lambda r: lex.rank.get(r[0], stories.NO_RANK))
                rk = lex.rank.get(best[0], stories.NO_RANK)
                counted += 1
                if rk <= topn:
                    top += 1
                else:
                    misses[best[1]] = rk
    share = top / counted if counted else 1.0
    if share < stories.MIN_TOP_SHARE:
        problems.append(f"frequency gate: {share:.0%} of words in the top {topn}, "
                        f"want at least {stories.MIN_TOP_SHARE:.0%}")
    if colloquial:
        if markers == 0:
            problems.append("no colloquial marker: use at least one [allowed] word of "
                            "pipeline/texts_colloquial.tsv at this level (hát, na, persze, szia, kösz...)")
        elif capped / max(tokens, 1) > MARKER_MAX:
            problems.append(f"{capped} colloquial markers (greetings and thanks not counted) in {tokens} words "
                            f"({capped / tokens:.0%}): at most {MARKER_MAX:.0%} (a caricature, not speech)")
    rep = {"n": topn, "counted": counted, "top": top, "exempt": exempt, "share": share,
           "misses": misses, "generated": gen, "allowed": allow, "colloquial": coll,
           "markers": markers, "capped": capped, "tokens": tokens, "unit_hits": lemma_hits,
           "words": sum(count_words(h) for h in hu_lines)}
    return rep, list(dict.fromkeys(problems))


def recycling(unit: dict, reports: list[dict]) -> float:
    """Share of the unit's words that occur >= RECYCLE_MIN times across the texts."""
    own = [w[0] if isinstance(w, tuple) else w for w in unit["words"]]
    if not own:
        return 1.0
    total: dict[str, int] = {}
    for r in reports:
        for lm, k in r["unit_hits"].items():
            total[lm] = total.get(lm, 0) + k
    return sum(1 for w in own if total.get(w, 0) >= RECYCLE_MIN) / len(own)


# ── the catalogue (ADR-076) ────────────────────────────────────────────────

def set_code(unit_code: str, tier: str, slug: str = "") -> str:
    """The set's stable code; an import of the set syncs by it."""
    if tier == "mini":
        return f"{unit_code}-talk-{slug}"
    return {"short": f"{unit_code}-dialogue", "medium": unit_code, "long": f"{unit_code}-long",
            "talk": f"{unit_code}-talk"}[tier]


def set_ord(unit_ord: int, tier: str, index: int = 0) -> int:
    return unit_ord * 100 + (ORD_MINI + index if tier == "mini" else ORD_SLOT[tier])


def catalog_sets(texts: dict[str, dict[str, dict]]) -> list[dict]:
    """One catalogue entry per text, sorted by (ord, code). Unit metadata (order,
    band, titles, track) comes from the course tables in this repo, so no pack is
    read. A text may carry `register` (default standard), `slug`/`index` (mini)."""
    units = _units()
    out = []
    for code, tiers in texts.items():
        u = units[code]
        for key, t in tiers.items():
            tier = t.get("tier", key)
            ls = lines_of(t)
            ru, en = full_title(t)
            lines = []
            for s in ls:
                line = {"hu": s["hu"].strip(), "ru": s["ru"].strip(), "en": s["en"].strip(),
                        "para": s["para"]}
                if s.get("who"):
                    line["who"] = s["who"]
                lines.append(line)
            out.append({
                "code": set_code(code, tier, t.get("slug", "")),
                "unit": code, "unitOrd": u["ord"], "level": rs.unit_band(None, u),
                "track": u["track"], "register": t.get("register", "standard"),
                "tier": tier, "genre": t["genre"],
                "ord": set_ord(u["ord"], tier, t.get("index", 0)),
                "titleRu": ru, "titleEn": en,
                "themeRu": u["title_ru"], "themeEn": u["title_en"],
                "checked": t.get("checked", "ai"),
                "words": sum(count_words(s["hu"]) for s in ls),
                "lines": lines})
            if t.get("scene"):
                out[-1]["sceneRu"], out[-1]["sceneEn"] = t["scene"]["ru"].strip(), t["scene"]["en"].strip()
    out.sort(key=lambda s: (s["ord"], s["code"]))
    return out


def catalog_text(texts: dict[str, dict[str, dict]]) -> str:
    """The committed file: one set per line, sorted keys, UTF-8, trailing newline."""
    sets = catalog_sets(texts)
    for what, vals in (("code", [s["code"] for s in sets]), ("ord", [s["ord"] for s in sets])):
        dup = {v for v in vals if vals.count(v) > 1}
        if dup:
            raise ValueError(f"duplicate catalogue {what}: {sorted(dup)}")
    rows = [json.dumps(s, ensure_ascii=False, sort_keys=True, separators=(",", ":")) for s in sets]
    body = ",\n".join(rows)
    return f'{{"format":{CATALOG_FORMAT},"sets":[\n{body}\n]}}\n'


def export(texts: dict[str, dict[str, dict]], path: Path | None = None) -> int:
    """Write the catalogue; returns the number of sets."""
    path = path or CATALOG_PATH
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(catalog_text(texts).encode("utf-8"))
    return len(catalog_sets(texts))


def check_export(texts: dict[str, dict[str, dict]], path: Path | None = None) -> bool:
    """True when the committed catalogue equals a fresh export, byte for byte."""
    path = path or CATALOG_PATH
    return path.exists() and path.read_bytes() == catalog_text(texts).encode("utf-8")

# ── the author's CLI ───────────────────────────────────────────────────────

EXAMPLE = {
    "genre": "note", "checked": "ai",
    "title": {"ru": "Привет, сосед!", "en": "Hi, neighbour!"},
    "names": ["Kata"],
    "paras": [[{"hu": "Szia, Kata!", "ru": "Привет, Ката!", "en": "Hi, Kata!"},
               {"hu": "Itt vagyok.", "ru": "Я здесь.", "en": "I am here."}]],
}


def brief(conn: sqlite3.Connection, code: str, tier: str) -> str:
    """What an author (or a neural net) needs to write one text; read-only."""
    unit = _units().get(code)
    if unit is None:
        raise SystemExit(f"{code}: not a basic-tier unit; known: " + ", ".join(_units()))
    if tier in ("talk", "mini"):
        return brief_talk(conn, code, tier)
    if tier not in TIERS:
        raise SystemExit(f"--tier must be one of {', '.join(TIERS + ('talk', 'mini'))}")
    band = rs.unit_band(conn, unit)
    g = size_group(unit)
    lo, hi = RANGES[g][tier]
    plo, phi = PARAS[tier]
    n = stories.top_n(unit)
    taught = dialogues._unit_words(unit)
    own = [w[0] if isinstance(w, tuple) else w for w in unit["words"]]
    texts, _bad = load()
    mine = texts.get(code, {})
    used = {t["genre"] for k, t in mine.items() if k != tier and k in TIERS}
    L = [f"UNIT {code}: {unit['title_ru']} / {unit['title_en']}",
         f"CEFR band: {band}   tier: {tier}   topic: {unit.get('situation_ru', '')}",
         f"SIZE: {lo}-{hi} words in all; at most {MAX_SENTENCE[g]} words a sentence; "
         f"{plo}-{phi} paragraphs (a dialogue: one turn per entry, no paragraph rule).",
         f"FREQUENCY: at least {stories.MIN_TOP_SHARE:.0%} of words within the top {n} lemmas "
         "(the unit's own words and names exempt).", "",
         f"GENRE MENU for a {tier} text (pick one"
         + (f"; already used by this unit's other tiers: {', '.join(sorted(used))}" if used else "")
         + "; three different genres per unit, at most one dialogue):"]
    for gid in MENU[tier]:
        mark = "  (used)" if gid in used else ""
        L.append(f"  {gid:<11} {GENRES[gid][1]} / {GENRES[gid][0]}: {GENRES[gid][2]}{mark}")
    if tier == "long":
        if past_taught(unit):
            L.append("  The course has taught the past tense by this unit: «how I spent my day» is fine.")
        else:
            L.append("  NO PAST TENSE yet in this month: a long text is a present-tense "
                     "«Мой обычный день» (myday), not a story in the past.")
    L += ["", "UNIT WORDS (taught; use each at least 3 times across the three texts):"]
    for w in unit["words"]:
        L.append(f"  {w[0]} ({w[1]}): {w[2]} / {w[3]}" if isinstance(w, tuple) else f"  {w}")
    seen: set[str] = set()
    ok = []
    for _lid, lemma, upos, cefr, rank in conn.execute(
            "SELECT id, lemma, upos, cefr, freq_rank FROM lemma "
            "WHERE freq_rank IS NOT NULL AND upos <> 'PROPN' ORDER BY freq_rank"):
        if lemma not in seen and (lemma in taught or rs._rank(cefr) <= rs._rank(band)):
            seen.add(lemma)
            ok.append((rank, lemma, upos, lemma in taught and lemma not in own))
    L += ["", f"ALLOWED LEMMAS by frequency rank (band {band} or taught earlier; top "
          f"{stories.BRIEF_LEMMAS} shown; * = taught earlier):",
          "  " + "  ".join(f"{lm}/{up}{'*' if t else ''}={rk}"
                           for rk, lm, up, t in ok[:stories.BRIEF_LEMMAS])]
    for other in TIERS:
        if other == tier:
            continue
        if other in mine:  # (talk and minis are not part of the narrow-reading triplet)
            t = mine[other]
            L += ["", f"THE UNIT'S {other.upper()} TEXT ({t['genre']}; reuse its words, do not copy it):"]
            L += [f"  {s['hu']}  |  {s['ru']}" for s in lines_of(t)]
        else:
            L += ["", f"THE UNIT'S {other.upper()} TEXT: not written yet."]
    L += ["", "V2 BAND EXEMPTIONS: a lemma whose CEFR label is wrong or that is core to the topic "
          "can be exempted from the band check (not the frequency gate) in the [band-exempt] "
          "section of pipeline/texts_allow.tsv; do not use rare words.",
          "", "V1 WORD RULES: every word must be (a) an attested form of the pack, or (b) a regular "
          "generated form (fine for reading), or (c) in pipeline/texts_allow.tsv, or (d) a proper "
          "name listed in \"names\" (as spelled in the text, capitalised). No digits (write numbers "
          "as words), no speaker labels, no leading dash. Both ru and en for every sentence.",
          "", f"WRITE langpack/texts/{code}.json (merge this tier into the file's \"{tier}\" key). "
          "SCHEMA, one tier shown:",
          json.dumps({tier: EXAMPLE}, ensure_ascii=False, indent=2),
          "", "THEN: make texts PACK=...  (validates and rewrites backend/internal/readsets/catalog.json), commit, deploy."]
    return "\n".join(L)


SCENES = {  # a few starting points per band group; the real situation lists are in GUIDE_PATH
    "a1": "meeting a neighbour, buying bread, asking the way, a phone call home, at the door, "
          "a dog in the park, ordering a coffee, a text to a friend",
    "a2": "a small problem at the shop, a plan with a friend, the doctor's waiting room, a phone call "
          "to book something, a voicemail, a colleague at work, a delayed bus, borrowing something",
    "b": "a disagreement settled, excuses and apologies, a rumour at work, changing plans by phone, "
         "a neighbour's complaint, a misunderstanding, a favour, small talk that drifts",
}
TALK_EXAMPLE = {
    "talk": {"genre": "dialogue", "checked": "ai", "title": {"ru": "У двери", "en": "At the door"},
             "names": ["Kata"],
             "paras": [[{"hu": "Szia, Kata!", "ru": "Привет, Ката!", "en": "Hi, Kata!"},
                        {"hu": "Szia! Gyere be.", "ru": "Привет! Заходи.", "en": "Hi! Come in."}]]},
    "dialogues": [{"slug": "ajtonal", "title": {"ru": "У двери", "en": "At the door"},
                   "scene": {"ru": "Сосед стучит.", "en": "A neighbour knocks."},
                   "names": [], "checked": "ai",
                   "turns": [{"who": "A", "hu": "Szia!", "ru": "Привет!", "en": "Hi!"},
                             {"who": "B", "hu": "Szia, gyere be.", "ru": "Привет, заходи.", "en": "Hi, come in."}]}],
}


def brief_talk(conn: sqlite3.Connection, code: str, tier: str) -> str:
    """The author's brief for a talk text or the minis of one unit (ADR-077)."""
    unit = _units().get(code)
    if unit is None:
        raise SystemExit(f"{code}: not a basic-tier unit; known: " + ", ".join(_units()))
    band = rs.unit_band(conn, unit)
    g = size_group(unit)
    n = stories.top_n(unit)
    taught = dialogues._unit_words(unit)
    own = [w[0] if isinstance(w, tuple) else w for w in unit["words"]]
    allowed, banned = load_colloquial()
    texts, _bad = load()
    mine = texts.get(code, {})
    L = [f"UNIT {code}: {unit['title_ru']} / {unit['title_en']}",
         f"CEFR band: {band}   tier: {tier}   topic: {unit.get('situation_ru', '')}", ""]
    if tier == "talk":
        lo, hi = TALK_SIZE[g]
        L += [f"TALK TEXT (the unit's 4th text, real spoken Hungarian, NOT literary): {lo}-{hi} words in all; "
              f"at most {MAX_SENTENCE[g]} words an entry; genre one of {', '.join(TALK_GENRES)}: "
              "dialogue/phone/chat are turn-based (one entry per turn, optional `who` A|B|C on EVERY entry "
              "or none), voicemsg is prose in 1-4 paragraphs."]
    else:
        (tlo, thi), cap, (wlo, whi) = MINI_SIZE[g]
        L += [f"MINI DIALOGUES: {MINI_COUNT[0]}-{MINI_COUNT[1]} per unit, each {tlo}-{thi} turns, at most {cap} "
              f"words a turn, {wlo}-{whi} words in all; `who` A|B|C on every turn; slug unique, kebab-case; "
              "no turn repeated across this unit's minis; one scene (`scene`, ru+en) each. Easy ones first, "
              "harder at higher levels."]
    L += [f"FREQUENCY: at least {stories.MIN_TOP_SHARE:.0%} of words within the top {n} lemmas; lemmas taught "
          "EARLIER in the course, the unit's own words, names and colloquial words are exempt (V3).",
          "COLLOQUIAL MARKERS: every mini (and the talk text) needs at least one [allowed] colloquial word and "
          f"at most {MARKER_MAX:.0%} of its words may be markers (greetings, thanks and apologies do not count toward the cap). Items above the text's level FAIL.",
          f"SCENARIOS (ideas): {SCENES[g]}.",
          f"GUIDE (read it before writing; situations, register, do-not-use): {GUIDE_PATH}", ""]
    rows = sorted(((r["level"], f, r) for f, rs_ in allowed.items() for r in rs_
                   if rs._rank(r["level"]) <= rs._rank(band)), key=lambda x: (x[0], x[1]))
    L.append(f"COLLOQUIAL WORDS ALLOWED at {band} (form [kind, level]: ru / en):")
    L += [f"  {f} [{r['kind']}, {lv}]: {r['ru']} / {r['en']}" for lv, f, r in rows]
    L.append(f"BANNED (vulgar words fail in EVERY text; stem prefixes, `$` = exact form): {' '.join(banned)}")
    L += ["", "UNIT WORDS (taught; reuse them):"]
    L += [f"  {w[0]} ({w[1]}): {w[2]} / {w[3]}" if isinstance(w, tuple) else f"  {w}" for w in unit["words"]]
    seen: set[str] = set()
    ok = []
    for _lid, lemma, upos, cefr, rank in conn.execute(
            "SELECT id, lemma, upos, cefr, freq_rank FROM lemma "
            "WHERE freq_rank IS NOT NULL AND upos <> 'PROPN' ORDER BY freq_rank"):
        if lemma not in seen and (lemma in taught or rs._rank(cefr) <= rs._rank(band)):
            seen.add(lemma)
            ok.append((rank, lemma, upos, lemma in taught and lemma not in own))
    L += ["", f"ALLOWED LEMMAS by frequency rank (band {band} or taught earlier; top "
          f"{stories.BRIEF_LEMMAS} shown; * = taught earlier):",
          "  " + "  ".join(f"{lm}/{up}{'*' if t else ''}={rk}" for rk, lm, up, t in ok[:stories.BRIEF_LEMMAS])]
    if tier == "talk":
        t = mine.get("talk")
        L += ["", "THE UNIT'S TALK TEXT: " + (f"({t['genre']}) " if t else "not written yet.")]
        if t:
            L += [f"  {s['hu']}  |  {s['ru']}" for s in lines_of(t)]
    existing = sorted((m for m in mine.values() if m.get("tier") == "mini"), key=lambda m: m["index"])
    L += ["", f"EXISTING MINIS of this unit ({len(existing)}; do not duplicate a scene or a turn):"]
    for m in existing:
        L.append(f"  [{m['slug']}] {m['title']['en']}: " + " / ".join(s["hu"] for s in m["paras"][0]))
    L += ["", "V1 WORD RULES: every word is (a) an attested or generated pack form, (b) a texts_allow.tsv [forms] "
          "row, (c) a texts_colloquial.tsv [allowed] row at a level <= the band, or (d) a declared name "
          "(capitalised, in \"names\"). No digits, no speaker labels in the text, no leading dash. "
          "A rejected word prints a hint with the closest pack lemmas and the row to paste.",
          "", f"WRITE langpack/talk/{code}.json (a single object; merge). SCHEMA with one tiny example:",
          json.dumps(TALK_EXAMPLE if tier == "mini" else {"talk": TALK_EXAMPLE["talk"]},
                     ensure_ascii=False, indent=2),
          "", "THEN: make texts PACK=...  (validates and rewrites backend/internal/readsets/catalog.json), commit."]
    return "\n".join(L)


def _report_line(code: str, tier: str, t: dict, rep: dict, probs: list[str]) -> str:
    gen = rep["generated"]
    s = (f"{code}.{tier} [{t['genre']}]: {rep['words']} words, top-{rep['n']} share {rep['share']:.0%}, "
         f"{sum(gen.values())} generated forms ({len(gen)} distinct)"
         + (f": {', '.join(sorted(gen)[:8])}" if gen else "")
         + (f", {sum(rep['allowed'].values())} allow-listed" if rep["allowed"] else "")
         + (f", {rep['markers']} colloquial markers" if t.get("register") == "colloquial" else ""))
    if probs:
        s += "\n    " + "\n    ".join(probs)
    return s


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(
        prog="python -m pipeline.texts",
        description="Validate langpack/texts/*.json against a pack (read-only); --brief prints "
                    "an author's brief for one unit and tier.")
    ap.add_argument("pack", type=Path, nargs="?")
    ap.add_argument("--export", action="store_true",
                    help="after a clean validation write backend/internal/readsets/catalog.json (ADR-076)")
    ap.add_argument("--check-export", action="store_true",
                    help="exit 1 when the committed catalogue differs from a fresh export (no pack)")
    ap.add_argument("--brief", metavar="UNIT")
    ap.add_argument("--tier", choices=TIERS + ("talk", "mini"))
    ap.add_argument("--require-all", action="store_true",
                    help="every unit needs a full valid triplet; legacy text is not accepted "
                         "(what `make starter` enforces)")
    ap.add_argument("--require-talk", action="store_true",
                    help="every unit needs a talk text and 5-10 minis (langpack/talk, ADR-077); "
                         "separate from --require-all so the standard texts validate alone")
    args = ap.parse_args(argv)
    if args.check_export:
        texts, bad = load()
        if bad:
            print(*bad, sep="\n")
            return 1
        if not check_export(texts):
            print(f"{CATALOG_PATH} is stale: run `make texts`, commit the result")
            return 1
        print(f"{CATALOG_PATH.name}: in sync")
        return 0
    if args.pack is None:
        ap.error("the pack path is required (except with --check-export)")
    conn = sqlite3.connect(f"file:{args.pack}?mode=ro", uri=True)
    if args.brief:
        if not args.tier:
            ap.error("--brief needs --tier short|medium|long|talk|mini")
        print(brief(conn, args.brief, args.tier))
        return 0
    texts, bad = load()
    probs = list(bad) + data_problems(texts, args.require_all, args.require_talk)
    lex = TextLexicon(conn)
    units = _units()
    print("== texts")
    pack_bad = 0
    for code, u in units.items():
        reports = []
        band = rs.unit_band(conn, u)
        mine = texts.get(code, {})
        order = [k for k in TIERS if k in mine] + (["talk"] if "talk" in mine else []) + sorted(
            (k for k in mine if k not in TIERS and k != "talk"), key=lambda k: mine[k]["index"])
        for key in order:
            t = mine[key]
            colloquial = key not in TIERS
            rep, p = check_text(lex, u, band, t.get("names", []), [s["hu"] for s in lines_of(t)],
                                "colloquial" if colloquial else "standard")
            if not colloquial:
                reports.append(rep)
            print(_report_line(code, key, t, rep, p))
            pack_bad += len(p)
        if reports:
            r = recycling(u, reports)
            print(f"{code}: recycling {r:.0%} of unit words >= {RECYCLE_MIN}x "
                  f"(target {RECYCLE_TARGET:.0%}){'  <-- below target' if r < RECYCLE_TARGET else ''}")
    std = [c for c in units if any(k in TIERS for k in texts.get(c, {}))]
    print(f"{len(std)} units with JSON texts, {len(units) - len(std)} without")
    if args.require_all:
        full = [c for c in units if sum(k in TIERS for k in texts.get(c, {})) == len(TIERS)]
        print(f"missing a full triplet: {len(units) - len(full)} units: "
              + ", ".join(c for c in units if c not in full))
    if args.require_talk or any(k not in TIERS for t in texts.values() for k in t):
        with_talk = [c for c in units if "talk" in texts.get(c, {})]
        mini_n = {c: sum(1 for t in texts.get(c, {}).values() if t.get("tier") == "mini") for c in units}
        print(f"talk: {len(with_talk)} units with a talk text, "
              f"{sum(1 for c in units if mini_n[c] >= MINI_COUNT[0])} with >= {MINI_COUNT[0]} minis "
              f"({sum(mini_n.values())} minis)")
    for p in probs:
        print(p)
    bad_n = len(probs) + pack_bad
    print(f"{bad_n} problems")
    if bad_n:
        return 1
    if args.export:
        n = export(texts)
        print(f"exported {n} sets to {CATALOG_PATH}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
