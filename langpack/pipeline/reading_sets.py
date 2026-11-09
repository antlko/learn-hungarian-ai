"""Ready-made reading sets: «Живые примеры» for every course unit.

A set is 20-30 sentences a person wrote (Tatoeba), picked from the pack itself,
for one course unit, at one CEFR band. The learner imports a set as an ordinary
text with one click and reads it in the reader. The app never writes Hungarian
for a set: every row of `reading_set_sentence` points at a `sentence` that
already exists, and the build refuses a pack in which one does not.

Selection, all deterministic (no randomness; ties fall to the sentence id):

  a. the sentence contains at least one word of the unit (by lemma, or by
     surface form -- the tagger's lemma and the dictionary's disagree, see
     CLAUDE.md);
  b. every CONTENT lemma in it (noun, verb, adjective, adverb, numeral,
     interjection) has a CEFR band at or below the set's. The band is
     `lemma.cefr`, which is estimated from frequency rank -- a stated estimate,
     not a measurement. A lemma with no band is treated as above every band;
  c. each sentence is in at most one set. Units are filled scarcest-first so a
     narrow unit is not starved by a broad one;
  d. short sentences (5-12 tokens) first, a Russian translation preferred, then
     an English one.

Tuning knobs are the constants below. See docs/LANGPACK.md "Reading sets".
"""

from __future__ import annotations

import sqlite3

from . import course

BANDS = ("A1", "A2", "B1", "B2", "C1")
CONTENT_UPOS = ("NOUN", "VERB", "ADJ", "ADV", "NUM", "INTJ")

MIN_SENTENCES = 20
MAX_SENTENCES = 30
TARGET_SENTENCES = 25
SHORT_MIN, SHORT_MAX = 5, 12
# A set may fall back to this wider range only when the short one is too thin.
WIDE_MIN, WIDE_MAX = 3, 14
IDEAL_LEN = 8

# Core months -> band. Themes are decided from their own words.
MONTH_BAND = {1: "A1", 2: "A1", 3: "A2", 4: "A2", 5: "B1", 6: "B1"}
# A theme is A1 when this share of its words is A1 and the A1 pool can fill it.
# Theme vocabulary is mostly A2+, so the bar is low on purpose.
THEME_A1_SHARE = 0.2

SCHEMA = """
CREATE TABLE IF NOT EXISTS reading_set (
    code     TEXT PRIMARY KEY,
    level    TEXT NOT NULL,
    track    TEXT NOT NULL DEFAULT 'core',
    ord      INTEGER NOT NULL,
    title_ru TEXT NOT NULL, title_en TEXT NOT NULL,
    theme_ru TEXT NOT NULL DEFAULT '', theme_en TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS reading_set_sentence (
    set_code    TEXT NOT NULL REFERENCES reading_set(code),
    sentence_id INTEGER NOT NULL REFERENCES sentence(id),
    ord         INTEGER NOT NULL,
    PRIMARY KEY (set_code, sentence_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rss_sentence ON reading_set_sentence(sentence_id);
"""


def _rank(band: str | None) -> int:
    return BANDS.index(band) if band in BANDS else 99


class _Index:
    """The pack's token and translation tables read ONCE.

    `sentence_token` is the big table (hundreds of thousands of rows on a full
    pack); scanning it per unit and per band made a refresh take minutes. One
    pass here feeds every `_eligible` and `_unit_hits` call.
    """

    def __init__(self, conn: sqlite3.Connection) -> None:
        rank = {r[0]: _rank(r[1]) for r in conn.execute("SELECT id, cefr FROM lemma")}
        content = {
            r[0] for r in conn.execute(
                f"SELECT id FROM lemma WHERE upos IN ({','.join('?' * len(CONTENT_UPOS))})",
                CONTENT_UPOS)
        }
        # sentence id -> the highest band rank among its content lemmas; an
        # unresolved token counts as above every band. A sentence is eligible
        # for a band when this does not exceed the band's rank.
        self.max_rank: dict[int, int] = {}
        # Every token, once, keyed two ways for _unit_hits.
        self.by_lemma: dict[int, list[int]] = {}
        self.by_form: dict[str, list[tuple[int, int | None]]] = {}
        for sid, form, lid in conn.execute(
            "SELECT sentence_id, form, lemma_id FROM sentence_token"
        ):
            if lid is None:
                r = 99
            elif lid in content:
                r = rank.get(lid, 99)
            else:
                r = 0
            if r > self.max_rank.get(sid, 0):
                self.max_rank[sid] = r
            if lid is not None:
                self.by_lemma.setdefault(lid, []).append(sid)
            self.by_form.setdefault(form.lower(), []).append((sid, lid))
        self.langs: dict[int, set[str]] = {}
        for sid, lang in conn.execute("SELECT sentence_id, lang FROM sentence_trans"):
            self.langs.setdefault(sid, set()).add(lang)
        self.sentences = list(conn.execute(
            "SELECT id, len_tokens FROM sentence WHERE source = 'tatoeba' ORDER BY rowid"))


def _eligible(conn: sqlite3.Connection, band: str, lo: int, hi: int,
              index: _Index | None = None) -> dict[int, tuple]:
    """sentence id -> (len, has_ru, has_en) for sentences whose every content
    lemma is at or below `band`, all tokens resolved."""
    ix = index or _Index(conn)
    limit = _rank(band)
    out = {}
    for sid, n in ix.sentences:
        if n is not None and lo <= n <= hi and ix.max_rank.get(sid, 0) <= limit:
            langs = ix.langs.get(sid, set())
            out[sid] = (n, "ru" in langs, "en" in langs)
    return out


def _unit_hits(conn: sqlite3.Connection, words: list[str],
               index: _Index | None = None) -> dict[int, int]:
    """sentence id -> how many of its tokens are words of the unit, by lemma or
    by surface form."""
    if not words:
        return {}
    ix = index or _Index(conn)
    ph = ",".join("?" * len(words))
    lemma_ids = [r[0] for r in conn.execute(
        f"SELECT id FROM lemma WHERE lemma IN ({ph})", words)]
    forms = {w.lower() for w in words}
    if lemma_ids:
        lph = ",".join("?" * len(lemma_ids))
        forms |= {r[0].lower() for r in conn.execute(
            f"SELECT form FROM wordform WHERE lemma_id IN ({lph})", lemma_ids)}
    lset = set(lemma_ids)
    hit: dict[int, int] = {}
    for lid in lset:
        for sid in ix.by_lemma.get(lid, ()):
            hit[sid] = hit.get(sid, 0) + 1
    # A token that matched by lemma is already counted; count it once.
    for f in forms:
        for sid, lid in ix.by_form.get(f, ()):
            if lid not in lset:
                hit[sid] = hit.get(sid, 0) + 1
    return hit


def _theme_band(conn: sqlite3.Connection, words: list[str]) -> str:
    if not words:
        return "A2"
    ph = ",".join("?" * len(words))
    bands = [r[0] for r in conn.execute(
        f"SELECT cefr FROM lemma WHERE lemma IN ({ph})", words)]
    if bands and sum(1 for b in bands if b == "A1") / len(bands) >= THEME_A1_SHARE:
        return "A1"
    return "A2"


def _title(prefix: str, unit_title: str) -> str:
    return f"{prefix}: {unit_title}"


def plan(conn: sqlite3.Connection) -> tuple[list[dict], dict[str, list[int]]]:
    """Choose the sets. Returns (set rows, code -> ordered sentence ids)."""
    # Only the basic tier and the themes get a set (ADR-057): MONTH_BAND stops at
    # month 6, and the starter's short Tatoeba sentences cannot fill a B2 set.
    units = [
        u for u in course.UNITS
        if u.get("tier", "basic") == "basic"
        and conn.execute("SELECT 1 FROM course_unit WHERE code = ?", (u["code"],)).fetchone()
    ]
    rows: list[dict] = []
    pools: dict[str, tuple[str, list[int]]] = {}
    eligible_cache: dict[tuple, dict[int, tuple]] = {}
    index = _Index(conn)

    def eligible(band, lo, hi):
        key = (band, lo, hi)
        if key not in eligible_cache:
            eligible_cache[key] = _eligible(conn, band, lo, hi, index)
        return eligible_cache[key]

    unit_words = {u["code"]: [w[0] if isinstance(w, tuple) else w for w in u["words"]]
                  for u in units}
    hits = {c: _unit_hits(conn, w, index) for c, w in unit_words.items()}

    for u in units:
        words = unit_words[u["code"]]
        band = MONTH_BAND[u["month"]] if u["track"] == "core" else _theme_band(conn, words)
        if u["track"] == "theme" and band == "A1":
            if len(set(hits[u["code"]]) & set(eligible("A1", SHORT_MIN, SHORT_MAX))) < MIN_SENTENCES:
                band = "A2"
        rows.append({
            "code": u["code"], "level": band, "track": u["track"], "ord": u["ord"],
            "title_ru": _title("Живые примеры", u["title_ru"]),
            "title_en": _title("Real examples", u["title_en"]),
            "theme_ru": u.get("situation_ru", ""), "theme_en": u.get("situation_en", ""),
        })
        pool = set(hits[u["code"]]) & set(eligible(band, SHORT_MIN, SHORT_MAX))
        if len(pool) < MIN_SENTENCES:
            pool = set(hits[u["code"]]) & set(eligible(band, WIDE_MIN, WIDE_MAX))
        pools[u["code"]] = (band, sorted(pool))

    def score(sid: int, band: str, code: str) -> tuple:
        n, has_ru, has_en = eligible(band, WIDE_MIN, WIDE_MAX)[sid]
        in_short = SHORT_MIN <= n <= SHORT_MAX
        return (not in_short, -min(hits[code][sid], 2), not has_ru, not has_en,
                abs(n - IDEAL_LEN), sid)

    taken: set[int] = set()
    chosen: dict[str, list[int]] = {}
    # Scarcest pool first, so a narrow unit is not starved by a broad one.
    for code in sorted(pools, key=lambda c: (len(pools[c][1]), c)):
        band, pool = pools[code]
        free = sorted((s for s in pool if s not in taken), key=lambda s: score(s, band, code))
        pick = free[:TARGET_SENTENCES]
        taken.update(pick)
        chosen[code] = pick

    # Reading order inside a set: short and translated first, so the first
    # screen is the easiest. Stable in the sentence id.
    for code, ids in chosen.items():
        band = pools[code][0]
        ids.sort(key=lambda s: (eligible(band, WIDE_MIN, WIDE_MAX)[s][0], s))
    rows.sort(key=lambda r: (r["track"] != "core", r["ord"]))
    return rows, chosen


def build(conn: sqlite3.Connection) -> dict[str, int]:
    """(Re)write the two tables in `conn`, from the sentences it holds."""
    conn.executescript(SCHEMA)
    rows, chosen = plan(conn)
    conn.execute("DELETE FROM reading_set_sentence")
    conn.execute("DELETE FROM reading_set")
    conn.executemany(
        "INSERT INTO reading_set (code, level, track, ord, title_ru, title_en, theme_ru, theme_en) "
        "VALUES (:code,:level,:track,:ord,:title_ru,:title_en,:theme_ru,:theme_en)", rows)
    n = 0
    for code, ids in chosen.items():
        conn.executemany(
            "INSERT INTO reading_set_sentence (set_code, sentence_id, ord) VALUES (?,?,?)",
            [(code, sid, i) for i, sid in enumerate(ids)])
        n += len(ids)
    return {"reading_set": len(rows), "reading_set_sentence": n}


def validate(conn: sqlite3.Connection) -> list[str]:
    """Problems in the sets a pack carries; empty is good."""
    problems: list[str] = []
    for code, level, track in conn.execute("SELECT code, level, track FROM reading_set"):
        n = conn.execute(
            "SELECT COUNT(*) FROM reading_set_sentence WHERE set_code = ?", (code,)).fetchone()[0]
        if not MIN_SENTENCES <= n <= MAX_SENTENCES:
            problems.append(f"{code}: {n} sentences, want {MIN_SENTENCES}-{MAX_SENTENCES}")
        if level not in BANDS:
            problems.append(f"{code}: unknown level {level!r}")
    for (sid,) in conn.execute(
        "SELECT sentence_id FROM reading_set_sentence "
        "WHERE sentence_id NOT IN (SELECT id FROM sentence WHERE source = 'tatoeba')"
    ):
        problems.append(f"sentence {sid}: missing or not a Tatoeba sentence")
    for sid, k in conn.execute(
        "SELECT sentence_id, COUNT(*) FROM reading_set_sentence GROUP BY 1 HAVING COUNT(*) > 1"
    ):
        problems.append(f"sentence {sid}: in {k} sets")
    return problems
