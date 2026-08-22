"""The HVPT contrast inventory.

Talker variability is the active ingredient of high-variability phonetic
training, not repetition -- see .claude/research/07-hvpt.md. This module owns
*what* to contrast; rendering the audio across talkers is a separate stage.

Selection is driven by what a Russian or English speaker actually cannot hear:

* **Vowel length is phonemic in Hungarian and in neither L1.** `kor`/`kór` are
  different words. This is the highest-yield set and comes first.
* **The rounded front vowels ö/ő and ü/ű** have no Russian or English counterpart
  at all, so they are both a perception and a production problem.
* **Sibilants and affricates** (`s` = [ʃ], `sz` = [s], `zs`, `cs`, `c`) trip up
  English speakers because the orthography inverts their expectations.
* **Palatals** (`gy`, `ny`, `ty`) are close to Russian palatalised stops, so they
  are easier for a Russian speaker -- included, but weighted lower.
* **Gemination** (`hal`/`hall`) is contrastive and routinely inaudible to both.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class MinimalPair:
    a: str
    b: str
    contrast: str
    gloss_a_ru: str
    gloss_b_ru: str
    gloss_a_en: str
    gloss_b_en: str
    # 1 = train first. Ordered by (difficulty for the L1) x (functional load).
    priority: int = 5
    context_a: str = ""
    context_b: str = ""


PAIRS: list[MinimalPair] = [
    # --- vowel length: the highest-yield contrast set ------------------------
    MinimalPair("kor", "kór", "o/ó", "возраст, эпоха", "болезнь", "age, era", "disease", 1,
                "A húszas évek kora volt.", "Ez egy ritka kór."),
    MinimalPair("öt", "őt", "ö/ő", "пять", "его/её (вин.)", "five", "him/her (acc.)", 1,
                "Öt óra van.", "Őt kerestem."),
    MinimalPair("tör", "tőr", "ö/ő", "ломает", "кинжал", "breaks", "dagger", 1),
    MinimalPair("hat", "hát", "a/á", "шесть", "спина; ну", "six", "back; well", 1,
                "Hat gyerek.", "Fáj a hátam."),
    MinimalPair("bal", "bál", "a/á", "левый", "бал", "left", "ball (dance)", 1),
    MinimalPair("agy", "ágy", "a/á", "мозг", "кровать", "brain", "bed", 1),
    MinimalPair("szel", "szél", "e/é", "режет", "ветер", "slices", "wind", 1),
    MinimalPair("kerek", "kerék", "e/é", "круглый", "колесо", "round", "wheel", 2),
    MinimalPair("ver", "vér", "e/é", "бьёт", "кровь", "beats", "blood", 2),
    MinimalPair("irt", "írt", "i/í", "истребляет", "написал", "exterminates", "wrote", 2),
    MinimalPair("hus", "hús", "u/ú", "(нет слова)", "мясо", "(non-word)", "meat", 3),
    MinimalPair("tur", "túr", "u/ú", "(нет слова)", "роет", "(non-word)", "digs", 3),
    MinimalPair("fut", "fűt", "u/ű", "бежит", "топит, отапливает", "runs", "heats", 2),
    MinimalPair("tuz", "tűz", "u/ű", "(нет слова)", "огонь", "(non-word)", "fire", 2),
    MinimalPair("orul", "örül", "o/ö", "(нет слова)", "радуется", "(non-word)", "rejoices", 2),

    # --- rounded front vowels: absent from both L1s --------------------------
    MinimalPair("tud", "tűd", "u/ű", "знает", "твоя игла", "knows", "your needle", 3),
    MinimalPair("busz", "bűsz", "u/ű", "автобус", "(нет слова)", "bus", "(non-word)", 4),
    MinimalPair("kor", "kör", "o/ö", "возраст", "круг", "age", "circle", 2),
    MinimalPair("fold", "föld", "o/ö", "(нет слова)", "земля", "(non-word)", "earth", 2),

    # --- sibilants and affricates -------------------------------------------
    MinimalPair("sok", "szok", "s/sz", "много", "(основа: привыкать)", "many", "(stem: get used to)", 2),
    MinimalPair("sír", "szír", "s/sz", "плачет; могила", "сириец", "cries; grave", "Syrian", 2),
    MinimalPair("csak", "sak", "cs/s", "только", "(нет слова)", "only", "(non-word)", 3),
    MinimalPair("zsák", "szák", "zs/sz", "мешок", "(нет слова)", "sack", "(non-word)", 3),
    MinimalPair("cél", "csél", "c/cs", "цель", "(нет слова)", "goal", "(non-word)", 3),

    # --- palatals: easier for a Russian ear, still trained --------------------
    MinimalPair("hagy", "hadj", "gy/dj", "оставляет", "(нет слова)", "leaves", "(non-word)", 6),
    MinimalPair("anya", "ana", "ny/n", "мать", "(нет слова)", "mother", "(non-word)", 6),
    MinimalPair("tyúk", "tuk", "ty/t", "курица", "(нет слова)", "hen", "(non-word)", 6),

    # --- gemination ----------------------------------------------------------
    MinimalPair("hal", "hall", "l/ll", "рыба; умирает", "слышит", "fish; dies", "hears", 2,
                "A hal úszik.", "Hallod ezt?"),
    MinimalPair("megy", "meggy", "gy/ggy", "идёт", "вишня", "goes", "sour cherry", 2),
    MinimalPair("eset", "esett", "t/tt", "случай", "упал", "case", "fell", 3),
    MinimalPair("örül", "őrül", "ö/ő", "радуется", "сходит с ума", "rejoices", "goes mad", 2),
    MinimalPair("álom", "állom", "l/ll", "сон", "выдерживаю", "dream", "I endure", 4),
]


def validate() -> list[str]:
    """Consistency checks that run as a pack quality gate."""
    problems: list[str] = []
    seen: set[tuple[str, str]] = set()
    for p in PAIRS:
        if p.a == p.b:
            problems.append(f"{p.a}: a pair must contrast two different forms")
        # A contrast must name two phonemes. An inflectional difference is not a
        # perceptual contrast and would train nothing.
        if "/" not in p.contrast:
            problems.append(f"{p.a}/{p.b}: contrast {p.contrast!r} is not a phoneme pair")
        key = (p.a, p.b)
        if key in seen:
            problems.append(f"duplicate pair {p.a}/{p.b}")
        seen.add(key)
        for field, value in (
            ("gloss_a_ru", p.gloss_a_ru), ("gloss_b_ru", p.gloss_b_ru),
            ("gloss_a_en", p.gloss_a_en), ("gloss_b_en", p.gloss_b_en),
        ):
            if not value.strip():
                problems.append(f"{p.a}/{p.b}: missing {field}")
    return problems


def contrasts() -> dict[str, list[MinimalPair]]:
    """Group pairs by contrast, which is the unit FSRS schedules."""
    out: dict[str, list[MinimalPair]] = {}
    for p in PAIRS:
        out.setdefault(p.contrast, []).append(p)
    return out
