"""The morphological feature inventory: the atoms of the second state space.

This is the heart of the product. Every feature here gets its own FSRS state per
learner, is scheduled independently of the words that carry it, and shows up as a
cell in the mastery heatmap. See docs/ARCHITECTURE.md §2.

Identifiers are Universal Dependencies feature=value strings (`Case=Ine`), because
they are what HuSpaCy emits and what the UD Hungarian treebank annotates -- so the
pack, the analyser and the golden tests all speak the same language.

**`Feature.id` values are a stable API.** A learner's `morph_state` rows reference
them across pack upgrades, so renaming one is a breaking change requiring a
migration that remaps user state. See docs/LANGPACK.md §7.

Ordering (`teach_order`) follows the corrected difficulty map, not the traditional
"here are the 18 cases" presentation: vowel harmony first as a motor skill, the
definite conjugation at the point it actually blocks progress, and word-order
focus last. See .claude/research/15-hungarian-difficulty.md.
"""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(frozen=True)
class Feature:
    """One teachable morphological atom."""

    id: str                      # UD feature=value, e.g. "Case=Ine"
    key: str                     # "Case"
    value: str                   # "Ine"
    label_ru: str
    label_en: str
    # Allomorphs keyed by harmony class. A single-entry dict means the suffix does
    # not harmonise (`-ig`, `-kor`, `-ért`).
    allomorphs: dict[str, str] = field(default_factory=dict)
    explain_ru: str = ""
    explain_en: str = ""
    teach_order: int = 999
    prereq_ids: tuple[str, ...] = ()
    # Grouping for the UI heatmap: cases in the 3x3 locative grid get a cell.
    grid: str | None = None      # "interior/where", "surface/whither", ...


B, FU, FR = "back", "front_unrounded", "front_rounded"


def _h(back: str, front: str, rounded: str | None = None) -> dict[str, str]:
    """Build an allomorph table. Two-form suffixes share the front form."""
    return {B: back, FU: front, FR: rounded if rounded is not None else front}


def _flat(suffix: str) -> dict[str, str]:
    """A suffix that does not harmonise."""
    return {B: suffix, FU: suffix, FR: suffix}


# ---------------------------------------------------------------------------
# Cases. The 3x3 locative grid is taught as a grid, in one sitting, because that
# is what makes it easy -- see the source brief §2.1. Everything else is ordered
# by frequency x error probability.
# ---------------------------------------------------------------------------

CASES: list[Feature] = [
    Feature("Case=Nom", "Case", "Nom", "именительный", "nominative", _flat(""),
            "Словарная форма. Без окончания.", "The dictionary form. No ending.", 10),
    Feature("Case=Acc", "Case", "Acc", "винительный (кого/что)", "accusative (object)",
            _h("t", "t", "t"),
            "Прямой объект. После согласной обычно нужна связующая гласная: házat, kertet, öt.",
            "The direct object. A linking vowel usually appears after a consonant: házat, kertet, öt.",
            20),
    # --- the 3x3 locative grid -------------------------------------------------
    Feature("Case=Ine", "Case", "Ine", "инессив — в чём", "inessive — in",
            _h("ban", "ben"),
            "Нахождение внутри: a házban — в доме.", "Being inside: a házban — in the house.",
            30, grid="interior/where"),
    Feature("Case=Ill", "Case", "Ill", "иллатив — во что", "illative — into",
            _h("ba", "be"),
            "Движение внутрь: a házba — в дом.", "Movement into: a házba — into the house.",
            31, grid="interior/whither"),
    Feature("Case=Ela", "Case", "Ela", "элатив — из чего", "elative — out of",
            _h("ból", "ből"),
            "Движение изнутри: a házból — из дома.", "Movement out of: a házból — out of the house.",
            32, grid="interior/whence"),
    Feature("Case=Sup", "Case", "Sup", "суперессив — на чём", "superessive — on",
            _h("on", "en", "ön"),
            "На поверхности: az asztalon. После гласной просто -n: a kocsin.",
            "On a surface: az asztalon. Just -n after a vowel: a kocsin.",
            33, grid="surface/where"),
    Feature("Case=Sbl", "Case", "Sbl", "сублатив — на что", "sublative — onto",
            _h("ra", "re"),
            "Движение на поверхность: az asztalra.", "Movement onto a surface: az asztalra.",
            34, grid="surface/whither"),
    Feature("Case=Del", "Case", "Del", "делатив — с чего", "delative — off",
            _h("ról", "ről"),
            "Движение с поверхности, а также «о чём»: a filmről — о фильме.",
            "Movement off a surface, and also 'about': a filmről — about the film.",
            35, grid="surface/whence"),
    Feature("Case=Ade", "Case", "Ade", "адессив — у чего", "adessive — at",
            _h("nál", "nél"),
            "Рядом с чем-то или у кого-то: Péternél — у Петера.",
            "Beside something or at someone's: Péternél — at Peter's.",
            36, grid="proximity/where"),
    Feature("Case=All", "Case", "All", "аллатив — к чему", "allative — to",
            _h("hoz", "hez", "höz"),
            "Движение к чему-то: az asztalhoz. Трёхформный суффикс — проверьте огублённость.",
            "Movement towards: az asztalhoz. A three-form suffix — check rounding.",
            37, grid="proximity/whither"),
    Feature("Case=Abl", "Case", "Abl", "аблатив — от чего", "ablative — from",
            _h("tól", "től"),
            "Движение от чего-то: az asztaltól.", "Movement away from: az asztaltól.",
            38, grid="proximity/whence"),
    # --- the rest --------------------------------------------------------------
    Feature("Case=Dat", "Case", "Dat", "дательный — кому", "dative — to/for",
            _h("nak", "nek"),
            "Адресат, а также владелец: Péternek a háza — дом Петера.",
            "The recipient, and also the possessor: Péternek a háza — Peter's house.",
            40),
    Feature("Case=Ins", "Case", "Ins", "инструменталис — чем/с кем", "instrumental — with",
            _h("val", "vel"),
            "«С» или «чем». v ассимилируется с предыдущей согласной: késsel, vonattal.",
            "'With' or 'by means of'. The v assimilates to a preceding consonant: késsel, vonattal.",
            45),
    Feature("Case=Cau", "Case", "Cau", "каузалис-финалис — за/ради", "causal-final — for",
            _flat("ért"),
            "Цена или цель: ezért — за это, поэтому. Не гармонирует.",
            "Price or purpose: ezért — for this, therefore. Does not harmonise.",
            50),
    Feature("Case=Ter", "Case", "Ter", "терминатив — до", "terminative — until",
            _flat("ig"),
            "Предел во времени или пространстве: hatig — до шести. Не гармонирует.",
            "A limit in time or space: hatig — until six. Does not harmonise.",
            51),
    Feature("Case=Tra", "Case", "Tra", "транслатив — во что (превращение)", "translative — into (becoming)",
            _h("vá", "vé"),
            "Превращение: orvossá lett — стал врачом. v ассимилируется.",
            "Becoming: orvossá lett — became a doctor. The v assimilates.",
            55),
    Feature("Case=Tem", "Case", "Tem", "темпоралис — в (о времени)", "temporal — at (time)",
            _flat("kor"),
            "Момент времени: hatkor — в шесть. Не гармонирует и не удлиняет гласную.",
            "A point in time: hatkor — at six. Neither harmonises nor lengthens.",
            56),
    Feature("Case=Ess", "Case", "Ess", "эссив-формалис — как, в качестве", "essive-formal — as",
            _flat("ként"),
            "В качестве кого-то: tanárként — как учитель.",
            "In the capacity of: tanárként — as a teacher.",
            60),
    Feature("Case=Dis", "Case", "Dis", "дистрибутив — по, за каждый", "distributive — per",
            _h("nként", "nként"),
            "«За каждый»: hetenként — еженедельно.", "'Per each': hetenként — weekly.", 70),
    Feature("Case=Loc", "Case", "Loc", "локатив — в (в топонимах)", "locative — at (place names)",
            _flat("t"),
            "Архаичный, живёт в названиях: Pécsett, Győrött.",
            "Archaic, survives in place names: Pécsett, Győrött.", 90),
]

# ---------------------------------------------------------------------------
# Number, possession, degree
# ---------------------------------------------------------------------------

NOMINAL: list[Feature] = [
    Feature("Number=Sing", "Number", "Sing", "единственное число", "singular", _flat(""),
            "После числительного венгерский использует ЕДИНСТВЕННОЕ число: két ház, не két házak.",
            "After a numeral Hungarian uses the SINGULAR: két ház, not két házak.", 15),
    Feature("Number=Plur", "Number", "Plur", "множественное число", "plural",
            _h("k", "k"),
            "Суффикс -k со связующей гласной: házak, kertek, könyvek, tükrök.",
            "The suffix -k with a linking vowel: házak, kertek, könyvek, tükrök.", 25),
    Feature("Number[psor]=Sing", "Number[psor]", "Sing", "один обладатель", "one possessor", _flat(""),
            "Обладатель один: a házam — мой дом.", "A single possessor: a házam — my house.", 65),
    Feature("Number[psor]=Plur", "Number[psor]", "Plur", "несколько обладателей", "several possessors", _flat(""),
            "Обладателей несколько: a házunk — наш дом.", "Several possessors: a házunk — our house.", 66),
    Feature("Person[psor]=1", "Person[psor]", "1", "притяжательный 1 л. — мой/наш", "possessive 1st — my/our",
            _h("m", "m"),
            "-m/-om/-em/-öm в ед. ч., -nk/-unk/-ünk во мн. ч.",
            "-m/-om/-em/-öm for one possessor, -nk/-unk/-ünk for several.", 61),
    Feature("Person[psor]=2", "Person[psor]", "2", "притяжательный 2 л. — твой/ваш", "possessive 2nd — your",
            _h("d", "d"),
            "-d/-od/-ed/-öd в ед. ч., -tok/-tek/-tök во мн. ч.",
            "-d/-od/-ed/-öd for one possessor, -tok/-tek/-tök for several.", 62),
    Feature("Person[psor]=3", "Person[psor]", "3", "притяжательный 3 л. — его/их", "possessive 3rd — his/their",
            _h("a", "e"),
            "-a/-e после согласной, -ja/-je после гласной. Выбор между ними — частая ошибка.",
            "-a/-e after a consonant, -ja/-je after a vowel. Choosing between them is a common error.", 63),
    Feature("Number[psed]=Plur", "Number[psed]", "Plur", "несколько предметов владения", "several possessed",
            _h("i", "i"),
            "-i- перед притяжательным окончанием: házaim — мои дома.",
            "-i- before the possessive ending: házaim — my houses.", 67),
    Feature("Degree=Cmp", "Degree", "Cmp", "сравнительная степень", "comparative",
            _h("bb", "bb"),
            "-bb: nagyobb — больше. Со связующей гласной после согласной.",
            "-bb: nagyobb — bigger. With a linking vowel after a consonant.", 75),
    Feature("Degree=Sup", "Degree", "Sup", "превосходная степень", "superlative",
            _flat("leg…bb"),
            "leg- + сравнительная: legnagyobb — самый большой.",
            "leg- + comparative: legnagyobb — the biggest.", 76),
]

# ---------------------------------------------------------------------------
# Verbs. Definite/indefinite conjugation is deliberately placed at teach_order 80
# -- early enough to matter, late enough that the learner has the vocabulary to
# practise it. It is the topic the gate in internal/drill/schedule.go exists for.
# ---------------------------------------------------------------------------

VERBAL: list[Feature] = [
    Feature("Mood=Ind", "Mood", "Ind", "изъявительное наклонение", "indicative", _flat(""),
            "Обычное утверждение.", "A plain statement.", 12),
    Feature("Tense=Pres", "Tense", "Pres", "настоящее время", "present", _flat(""),
            "Настоящее также выражает будущее: holnap megyek — завтра иду/пойду.",
            "The present also expresses the future: holnap megyek — I go / will go tomorrow.", 13),
    Feature("Tense=Past", "Tense", "Past", "прошедшее время", "past",
            _h("t", "t"),
            "-t/-tt со связующей гласной. Форма зависит от типа основы.",
            "-t/-tt with a linking vowel. The exact form depends on the stem type.", 70),
    # The VERB's person, as distinct from the possessor's `Person[psor]` above.
    #
    # Every conjugated form carries one, so a cloze answer grades it and it has
    # always had a row in `morph_state` — but it had no row HERE, so the app
    # could not name it. The progress heatmap printed `Person=1` at the learner
    # under a heading promising there would be no such thing, and the fallback
    # that made that possible is deliberate and correct: a feature with no label
    # shows its id rather than a blank. The fix belongs in the inventory.
    #
    # No allomorph table: the personal endings depend on tense, mood and
    # definiteness together, so a single row of suffixes would be a lie. The
    # explanation carries the examples instead.
    Feature("Person=1", "Person", "1", "1 лицо — я / мы", "1st person — I / we", _flat(""),
            "Я или мы: megyek — иду, megyünk — идём. Окончание зависит и от времени, "
            "и от наклонения, и от определённости.",
            "I or we: megyek — I go, megyünk — we go. The ending depends on tense, mood "
            "and definiteness together.", 14),
    Feature("Person=2", "Person", "2", "2 лицо — ты / вы", "2nd person — you", _flat(""),
            "Ты или вы: mész — идёшь, mentek — идёте. С вежливым ön глагол стоит в 3 лице.",
            "You: mész — you go, mentek — you (pl.) go. With the polite ön the verb takes "
            "the THIRD person.", 15),
    Feature("Person=3", "Person", "3", "3 лицо — он / она / они", "3rd person — he / she / they",
            _flat(""),
            "Он, она, они — и вежливое обращение на ön/maga: megy — идёт, mennek — идут.",
            "He, she, they — and the polite ön/maga: megy — goes, mennek — go.", 16),
    Feature("Definite=Ind", "Definite", "Ind", "неопределённое спряжение", "indefinite conjugation",
            _flat(""),
            "Объекта нет или он неопределённый: Látok egy házat — вижу (какой-то) дом.",
            "No object, or an indefinite one: Látok egy házat — I see a house.", 80),
    Feature("Definite=Def", "Definite", "Def", "определённое спряжение", "definite conjugation",
            _flat(""),
            "Объект определённый: Látom a házat — вижу этот дом. Аналога в русском и "
            "английском нет; 55 % ошибок изучающих — определённый объект с неопределённым "
            "спряжением, ещё 25 % — объект-указательное местоимение.",
            "A definite object: Látom a házat — I see the house. There is no analogue in "
            "Russian or English; 55% of learner errors are a definite object taking the "
            "indefinite conjugation, and a further 25% involve demonstrative-pronoun objects.",
            81),
    Feature("Definite=2", "Definite", "2", "спряжение на -lak/-lek (я тебя)", "-lak/-lek (I ... you)",
            _h("lak", "lek"),
            "Особая форма «я — тебя»: szeretlek — люблю тебя.",
            "A dedicated 'I ... you' form: szeretlek — I love you.", 85),
    Feature("VerbForm=Inf", "VerbForm", "Inf", "инфинитив", "infinitive",
            _h("ni", "ni"),
            "-ni. С личными окончаниями, когда есть субъект: mennem kell — мне нужно идти.",
            "-ni. Takes personal endings when the subject is expressed: mennem kell — I must go.", 88),
    Feature("Mood=Cnd", "Mood", "Cnd", "условное наклонение", "conditional",
            _h("nék", "nék"),
            "-nék/-nál/-na… Часто в вежливых просьбах, поэтому нужно рано в быту: "
            "kérnék egy kávét.",
            "-nék/-nál/-na… Common in polite requests, so it is needed early in daily life: "
            "kérnék egy kávét.", 100),
    Feature("Mood=Imp", "Mood", "Imp", "повелительное / сослагательное", "imperative / subjunctive",
            _h("j", "j"),
            "-j- с обширными ассимиляциями: keres → keress, olvas → olvass. "
            "Морфонологически самая неприятная тема.",
            "-j- with extensive assimilation: keres → keress, olvas → olvass. "
            "Morphophonologically the nastiest topic.", 110),
    Feature("Mood=Pot", "Mood", "Pot", "потенциалис — мочь", "potential — can/may",
            _h("hat", "het"),
            "-hat/-het: olvashat — может читать. Полностью регулярно.",
            "-hat/-het: olvashat — can read. Fully regular.", 95),
    Feature("Voice=Cau", "Voice", "Cau", "каузатив — заставить/дать сделать", "causative",
            _h("tat", "tet"),
            "-tat/-tet: megcsináltat — заставить сделать.",
            "-tat/-tet: megcsináltat — to have something done.", 105),
    Feature("Aspect=Iter", "Aspect", "Iter", "многократность", "iterative/frequentative",
            _h("gat", "get"),
            "-gat/-get: olvasgat — почитывает.", "-gat/-get: olvasgat — reads a bit, now and then.", 115),
    Feature("VerbForm=PartPres", "VerbForm", "PartPres", "причастие настоящего времени", "present participle",
            _h("ó", "ő"),
            "-ó/-ő: olvasó — читающий.", "-ó/-ő: olvasó — reading (one who reads).", 118),
    Feature("VerbForm=PartPast", "VerbForm", "PartPast", "причастие прошедшего времени", "past participle",
            _h("t", "t"),
            "-t/-tt: olvasott — прочитанный.", "-t/-tt: olvasott — read (having been read).", 119),
    Feature("VerbForm=PartFut", "VerbForm", "PartFut", "причастие будущего времени", "future participle",
            _h("andó", "endő"),
            "-andó/-endő: fizetendő — подлежащий оплате.",
            "-andó/-endő: fizetendő — to be paid.", 125),
    Feature("VerbForm=Conv", "VerbForm", "Conv", "деепричастие", "converb",
            _h("va", "ve"),
            "-va/-ve: állva — стоя.", "-va/-ve: állva — standing.", 120),
]

# ---------------------------------------------------------------------------
# Non-UD pedagogical atoms. These are not morphological features in the UD sense,
# but they behave exactly like them for the learner: they are learned, forgotten
# and relearned separately, and they belong in the heatmap. The `Skill=` prefix
# keeps them from ever colliding with a real UD feature.
# ---------------------------------------------------------------------------

SKILLS: list[Feature] = [
    Feature("Skill=VowelHarmony", "Skill", "VowelHarmony", "гармония гласных", "vowel harmony",
            _flat(""),
            "Не знание, а моторный навык: правило понимается за 20 минут, автоматизируется "
            "за 3 месяца. Поэтому мастерство здесь измеряется СКОРОСТЬЮ, а не только "
            "точностью.",
            "Not knowledge but a motor skill: the rule takes 20 minutes to understand and "
            "about three months to automate. Mastery here is therefore measured by LATENCY, "
            "not accuracy alone.",
            1),
    Feature("Skill=Politeness", "Skill", "Politeness", "вежливость: te / ön / maga", "politeness: te / ön / maga",
            _flat(""),
            "Социально критично: ошибка обходится дороже грамматической. ön требует формы "
            "3-го лица. Вводится в первый месяц.",
            "Socially critical: getting this wrong costs more than a grammar mistake. ön takes "
            "third-person forms. Introduced in month one.",
            5),
    Feature("Skill=Preverb", "Skill", "Preverb", "глагольные приставки (igekötők)", "preverbs (igekötők)",
            _flat(""),
            "meg-, el-, be-, ki-, fel-, le-… Похоже на русские приставки, и это помогает.",
            "meg-, el-, be-, ki-, fel-, le-… Similar to Russian prefixes, which helps.", 92),
    Feature("Skill=PreverbDetach", "Skill", "PreverbDetach", "отрыв приставки и фокус", "preverb detachment and focus",
            _flat(""),
            "Приставка отрывается, когда что-то другое занимает позицию фокуса — прямо перед "
            "глаголом. Elmegyek a boltba / A boltba megyek el. Осваивается последним: сначала "
            "нужен объём слышанного.",
            "The preverb detaches when something else occupies the focus position immediately "
            "before the verb. Elmegyek a boltba / A boltba megyek el. Acquired last: it needs "
            "listening volume first.",
            150, prereq_ids=("Skill=Preverb",)),
    Feature("Skill=StemAlternation", "Skill", "StemAlternation", "чередование основ", "stem alternation",
            _flat(""),
            "ló→lovak, tó→tavak, fa→fát. Закрытый список примерно из 200 слов — учится как "
            "список слов, а не как правило.",
            "ló→lovak, tó→tavak, fa→fát. A closed list of roughly 200 words — learned as a list "
            "of items, not as a rule.",
            72),
]

ALL_FEATURES: list[Feature] = CASES + NOMINAL + VERBAL + SKILLS


def by_id() -> dict[str, Feature]:
    return {f.id: f for f in ALL_FEATURES}


def validate() -> list[str]:
    """Check the inventory's internal consistency. Returns a list of problems."""
    problems: list[str] = []
    seen: set[str] = set()
    index = by_id()

    for f in ALL_FEATURES:
        if f.id in seen:
            problems.append(f"duplicate feature id: {f.id}")
        seen.add(f.id)

        # ADR-004: both locales, always. A feature without a label in one locale
        # would render as a raw UD string to half the audience.
        if not f.label_ru.strip():
            problems.append(f"{f.id}: missing label_ru")
        if not f.label_en.strip():
            problems.append(f"{f.id}: missing label_en")
        # [R4] A feature the app cannot explain cannot be taught explicitly.
        if not f.explain_ru.strip():
            problems.append(f"{f.id}: missing explain_ru")
        if not f.explain_en.strip():
            problems.append(f"{f.id}: missing explain_en")

        if set(f.allomorphs) and set(f.allomorphs) != {B, FU, FR}:
            problems.append(f"{f.id}: allomorphs must cover all three harmony classes")

        # The id IS the key=value pair. A mismatch means the pack and the
        # analyser disagree about what a feature is called, and a learner's
        # morph_state would point at nothing.
        if f.id != f"{f.key}={f.value}":
            problems.append(f"{f.id}: id does not match key={f.key} value={f.value}")

        for prereq in f.prereq_ids:
            if prereq not in index:
                problems.append(f"{f.id}: unknown prerequisite {prereq}")
            elif index[prereq].teach_order >= f.teach_order:
                problems.append(
                    f"{f.id} (order {f.teach_order}) requires {prereq} "
                    f"(order {index[prereq].teach_order}) — a prerequisite must come first"
                )
    return problems
