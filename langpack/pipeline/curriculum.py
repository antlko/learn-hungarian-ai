"""The grammar curriculum: a prerequisite DAG over teachable topics.

Ordering follows the corrected difficulty map, not the conventional
"here are the 18 cases" presentation. Three commitments shape it:

1. **Vowel harmony first**, treated as a motor skill needing volume rather than a
   fact needing explanation.
2. **Politeness in month one**, because a `te`/`ön` mistake costs more socially
   than a case mistake costs grammatically.
3. **Word order and focus last**, gated behind listening volume.

The definite conjugation sits in the middle, at the point where it starts
blocking real communication -- and it is the topic the blocked->interleaved gate
in `internal/drill/schedule.go` exists for.

See .claude/research/15-hungarian-difficulty.md and docs/PEDAGOGY.md.
"""

from __future__ import annotations

from dataclasses import dataclass, field

from . import lessons


@dataclass(frozen=True)
class Topic:
    """One node of the teaching DAG.

    The lesson text lives in `lessons.py`, keyed by `code`, and is reached
    through the `explain_ru` / `explain_en` properties below. It used to sit
    inline here as two string literals per topic, which is why every lesson was
    a single paragraph: nobody wants to put a page of prose in the middle of a
    graph definition. Separating them is what made it possible to write lessons
    that actually teach — see the module docstring in `lessons.py` and [R4].
    """

    code: str
    level: str                      # A1 | A2 | B1 | B2
    title_ru: str
    title_en: str
    teach_order: int
    prereq_codes: tuple[str, ...] = ()
    feature_ids: tuple[str, ...] = field(default_factory=tuple)

    @property
    def explain_ru(self) -> str:
        return lessons.LESSONS.get(self.code, ("", ""))[0]

    @property
    def explain_en(self) -> str:
        return lessons.LESSONS.get(self.code, ("", ""))[1]


TOPICS: list[Topic] = [
    # code, level, title_ru, title_en, teach_order, prereqs, feature_ids.
    # The lesson prose for each of these lives in lessons.py, keyed by code.

    # ── A1: sound, harmony, survival ──────────────────────────────────────
    Topic("orthography", "A1", "Алфавит и чтение", "Alphabet and reading",
          10),
    Topic("vowel-harmony", "A1", "Гармония гласных", "Vowel harmony",
          20, ("orthography",), ("Skill=VowelHarmony",)),
    Topic("politeness", "A1", "Вежливость: te / ön / maga", "Politeness: te / ön / maga",
          30, ("orthography",), ("Skill=Politeness",)),
    Topic("van-nincs", "A1", "van / nincs, простое предложение", "van / nincs, the simple sentence",
          40, ("vowel-harmony",), ("Mood=Ind", "Tense=Pres", "Case=Nom")),
    Topic("present-indefinite", "A1", "Настоящее время, неопределённое спряжение",
          "Present tense, indefinite conjugation",
          50, ("van-nincs",), ("Definite=Ind", "Tense=Pres")),
    Topic("questions-negation", "A1", "Вопросы и отрицание", "Questions and negation",
          55, ("present-indefinite",)),
    Topic("plural", "A1", "Множественное число", "The plural",
          60, ("vowel-harmony",), ("Number=Plur", "Number=Sing")),
    Topic("numerals", "A1", "Числительные и счёт", "Numerals and counting",
          65, ("plural",)),
    Topic("accusative", "A1", "Винительный падеж", "The accusative",
          70, ("plural",), ("Case=Acc",)),
    Topic("pronouns-demonstratives", "A1", "Местоимения и ez / az", "Pronouns and ez / az",
          75, ("accusative",), ("Person=1", "Person=2", "Person=3")),
    Topic("locative-grid", "A1", "Локативная система: 9 клеток", "The locative system: nine cells",
          80, ("accusative",),
          ("Case=Ine", "Case=Ill", "Case=Ela", "Case=Sup", "Case=Sbl",
           "Case=Del", "Case=Ade", "Case=All", "Case=Abl")),
    Topic("dative", "A1", "Дательный падеж и обладание", "The dative and possession",
          90, ("locative-grid",), ("Case=Dat",)),
    Topic("possessive", "A1", "Притяжательные суффиксы", "Possessive suffixes",
          100, ("dative",),
          ("Person[psor]=1", "Person[psor]=2", "Person[psor]=3",
           "Number[psor]=Sing", "Number[psor]=Plur", "Number[psed]=Plur")),

    # ── A2: past, the definite conjugation, modality ──────────────────────
    Topic("past-tense", "A2", "Прошедшее время", "The past tense",
          110, ("present-indefinite", "possessive"), ("Tense=Past",)),
    Topic("postpositions", "A2", "Послелоги: alatt, mellett, előtt…", "Postpositions: alatt, mellett, előtt…",
          115, ("locative-grid", "possessive")),
    Topic("definite-conjugation", "A2", "Определённое спряжение", "The definite conjugation",
          120, ("past-tense",), ("Definite=Def", "Definite=Ind")),
    Topic("time-and-date", "A2", "Время и дата", "Time and the date",
          125, ("numerals", "postpositions")),
    Topic("lak-lek", "A2", "Форма -lak/-lek (я тебя)", "The -lak/-lek form (I ... you)",
          130, ("definite-conjugation",), ("Definite=2",)),
    Topic("infinitive", "A2", "Инфинитив с личными окончаниями", "The infinitive with personal endings",
          140, ("definite-conjugation",), ("VerbForm=Inf",)),
    Topic("potential", "A2", "Потенциалис -hat/-het", "The potential -hat/-het",
          150, ("definite-conjugation",), ("Mood=Pot",)),
    Topic("instrumental", "A2", "Инструменталис и ассимиляция v", "The instrumental and v-assimilation",
          160, ("locative-grid",), ("Case=Ins",)),
    Topic("comparison", "A2", "Степени сравнения", "Comparison",
          170, ("accusative",), ("Degree=Cmp", "Degree=Sup")),
    Topic("preverbs", "A2", "Глагольные приставки", "Preverbs (igekötők)",
          180, ("past-tense",), ("Skill=Preverb",)),

    # ── B1: mood, stems, the remaining cases ──────────────────────────────
    Topic("conditional", "B1", "Условное наклонение", "The conditional",
          190, ("infinitive",), ("Mood=Cnd",)),
    Topic("stem-alternation", "B1", "Чередование основ", "Stem alternation",
          200, ("plural",), ("Skill=StemAlternation",)),
    Topic("imperative", "B1", "Повелительное наклонение", "The imperative",
          210, ("conditional",), ("Mood=Imp",)),
    Topic("participles", "B1", "Причастия", "Participles",
          220, ("past-tense",),
          ("VerbForm=PartPres", "VerbForm=PartPast", "VerbForm=PartFut")),
    Topic("remaining-cases", "B1", "Остальные падежи", "The remaining cases",
          230, ("instrumental",),
          ("Case=Cau", "Case=Ter", "Case=Tra", "Case=Tem", "Case=Ess", "Case=Dis",
           "Case=Loc")),
    Topic("causative", "B1", "Каузатив", "The causative",
          240, ("potential",), ("Voice=Cau",)),
    Topic("converb", "B1", "Деепричастие", "The converb",
          250, ("participles",), ("VerbForm=Conv",)),

    # ── B2: what separates correct from native ────────────────────────────
    Topic("iterative", "B2", "Многократность -gat/-get", "The frequentative -gat/-get",
          260, ("causative",), ("Aspect=Iter",)),
    Topic("focus-word-order", "B2", "Порядок слов и фокус", "Word order and focus",
          270, ("preverbs", "converb"), ("Skill=PreverbDetach",)),
    Topic("register", "B2", "Регистры и стиль", "Register and style",
          280, ("focus-word-order",), ()),
]


# The shortest thing that can still be a lesson rather than a blurb. Not a
# quality judgement — a placeholder detector. docs/LANGPACK.md states it too.
MIN_LESSON_CHARS = 800


def validate(known_feature_ids: set[str]) -> list[str]:
    problems: list[str] = []
    codes = {t.code for t in TOPICS}
    index = {t.code: t for t in TOPICS}
    seen: set[str] = set()

    for t in TOPICS:
        if t.code in seen:
            problems.append(f"duplicate topic code: {t.code}")
        seen.add(t.code)

        # [R4]: a topic without an explicit explanation cannot be taught
        # explicitly, and explicit instruction is the whole point.
        for name, value in (
            ("title_ru", t.title_ru), ("title_en", t.title_en),
            ("explain_ru", t.explain_ru), ("explain_en", t.explain_en),
        ):
            if not value.strip():
                problems.append(f"{t.code}: missing {name}")

        # And a lesson has to BE a lesson. Every explanation in this file was
        # once a single paragraph — as little as fifty characters — sitting
        # above an exercise that asked about a feature the paragraph had not
        # named. That is citing [R4], not applying it. The floor is deliberately
        # crude: it cannot judge quality, but it does catch a topic that was
        # added with a placeholder and never written.
        for name, value in (("explain_ru", t.explain_ru), ("explain_en", t.explain_en)):
            if 0 < len(value.strip()) < MIN_LESSON_CHARS:
                problems.append(
                    f"{t.code}: {name} is {len(value.strip())} chars — a lesson states the "
                    f"rule, shows the forms and names the trap; minimum {MIN_LESSON_CHARS}"
                )

        for code in t.prereq_codes:
            if code not in codes:
                problems.append(f"{t.code}: unknown prerequisite {code}")
            elif index[code].teach_order >= t.teach_order:
                problems.append(
                    f"{t.code} (order {t.teach_order}) requires {code} "
                    f"(order {index[code].teach_order}) — prerequisites must come first"
                )
        for fid in t.feature_ids:
            if fid not in known_feature_ids:
                problems.append(f"{t.code}: unknown feature {fid}")
    return problems


def uncovered_features(known_feature_ids: set[str]) -> set[str]:
    """Features no topic teaches. A non-empty result is a curriculum hole."""
    covered = {fid for t in TOPICS for fid in t.feature_ids}
    return known_feature_ids - covered
