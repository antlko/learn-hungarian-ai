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


@dataclass(frozen=True)
class Topic:
    code: str
    level: str                      # A1 | A2 | B1 | B2
    title_ru: str
    title_en: str
    explain_ru: str
    explain_en: str
    teach_order: int
    prereq_codes: tuple[str, ...] = ()
    feature_ids: tuple[str, ...] = field(default_factory=tuple)


TOPICS: list[Topic] = [
    # ── A1: sound, harmony, survival ──────────────────────────────────────
    Topic("orthography", "A1", "Алфавит и чтение", "Alphabet and reading",
          "Венгерская орфография фонетична: одна буква — один звук, всегда. "
          "Диграфы (cs, gy, ly, ny, sz, ty, zs) — это ОДНА буква. Ударение всегда "
          "на первом слоге и никогда не меняет смысл — в отличие от длины гласной, "
          "которая его меняет.",
          "Hungarian spelling is phonetic: one letter, one sound, always. Digraphs "
          "(cs, gy, ly, ny, sz, ty, zs) are single letters. Stress is always on the "
          "first syllable and never changes meaning — unlike vowel length, which does.",
          10),
    Topic("vowel-harmony", "A1", "Гармония гласных", "Vowel harmony",
          "Каждый суффикс имеет 2–3 варианта, и выбор определяется гласными основы. "
          "Правило понимается за 20 минут, а автоматизируется за три месяца — это "
          "моторный навык, а не знание. Поэтому здесь важна СКОРОСТЬ ответа, а не "
          "только правильность.",
          "Every suffix has two or three variants, selected by the stem's vowels. The "
          "rule takes twenty minutes to understand and about three months to automate: "
          "it is a motor skill, not knowledge. That is why SPEED of response matters "
          "here, not just accuracy.",
          20, ("orthography",), ("Skill=VowelHarmony",)),
    Topic("politeness", "A1", "Вежливость: te / ön / maga", "Politeness: te / ön / maga",
          "ön и maga требуют форм ТРЕТЬЕГО лица, хотя обращаются ко второму. "
          "Социально это критично: обратиться на te к чиновнику — ошибка дороже "
          "любой грамматической.",
          "ön and maga take THIRD-person verb forms even though they address the second "
          "person. This is socially critical: using te with an official costs more than "
          "any grammatical error.",
          30, ("orthography",), ("Skill=Politeness",)),
    Topic("van-nincs", "A1", "van / nincs, простое предложение", "van / nincs, the simple sentence",
          "В настоящем времени связка «есть» опускается: Ez egy ház — это дом. "
          "Но van нужен для местоположения и существования, а nincs — его "
          "отрицательная пара.",
          "The copula is omitted in the present tense: Ez egy ház — this is a house. "
          "But van is required for location and existence, and nincs is its negative "
          "counterpart.",
          40, ("vowel-harmony",), ("Mood=Ind", "Tense=Pres", "Case=Nom")),
    Topic("present-indefinite", "A1", "Настоящее время, неопределённое спряжение",
          "Present tense, indefinite conjugation",
          "Базовое спряжение. Пока объект неопределённый или его нет, используется "
          "именно оно: Olvasok egy könyvet.",
          "The base conjugation. As long as the object is indefinite or absent, this is "
          "the one you use: Olvasok egy könyvet.",
          50, ("van-nincs",), ("Definite=Ind", "Tense=Pres")),
    Topic("plural", "A1", "Множественное число", "The plural",
          "Суффикс -k со связующей гласной. Важно: ПОСЛЕ ЧИСЛИТЕЛЬНОГО венгерский "
          "использует единственное число — két ház, а не két házak.",
          "The suffix -k with a linking vowel. Note that AFTER A NUMERAL Hungarian uses "
          "the singular — két ház, not két házak.",
          60, ("vowel-harmony",), ("Number=Plur", "Number=Sing")),
    Topic("accusative", "A1", "Винительный падеж", "The accusative",
          "Прямой объект помечается -t. Связующая гласная перед ним лексически "
          "обусловлена: ház→házat, но vonat→vonatot.",
          "The direct object is marked with -t. The linking vowel before it is lexically "
          "conditioned: ház→házat, but vonat→vonatot.",
          70, ("plural",), ("Case=Acc",)),
    Topic("locative-grid", "A1", "Локативная система: 9 клеток", "The locative system: nine cells",
          "Три плоскости (внутри / на поверхности / рядом) × три направления "
          "(где / куда / откуда) = таблица из девяти клеток. Учится как таблица, "
          "за один вечер. Для носителя русского логика уже есть в голове — меняется "
          "только способ выражения.",
          "Three planes (interior / surface / proximity) × three directions (where / "
          "whither / whence) = a nine-cell table. Learn it as a table, in one sitting. "
          "A Russian speaker already has the logic; only the encoding changes.",
          80, ("accusative",),
          ("Case=Ine", "Case=Ill", "Case=Ela", "Case=Sup", "Case=Sbl",
           "Case=Del", "Case=Ade", "Case=All", "Case=Abl")),
    Topic("dative", "A1", "Дательный падеж и обладание", "The dative and possession",
          "-nak/-nek обозначает адресата, а также ОБЛАДАТЕЛЯ: Péternek a háza — "
          "дом Петера. Второе применение неочевидно и встречается постоянно.",
          "-nak/-nek marks the recipient and also the POSSESSOR: Péternek a háza — "
          "Peter's house. The second use is not obvious and comes up constantly.",
          90, ("locative-grid",), ("Case=Dat",)),
    Topic("possessive", "A1", "Притяжательные суффиксы", "Possessive suffixes",
          "Обладание выражается на ПРЕДМЕТЕ, а не на владельце: a házam — мой дом. "
          "Третье лицо выбирает между -a/-e и -ja/-je лексически.",
          "Possession is marked on the THING possessed, not the possessor: a házam — my "
          "house. The third person chooses between -a/-e and -ja/-je lexically.",
          100, ("dative",),
          ("Person[psor]=1", "Person[psor]=2", "Person[psor]=3",
           "Number[psor]=Sing", "Number[psor]=Plur", "Number[psed]=Plur")),

    # ── A2: past, the definite conjugation, modality ──────────────────────
    Topic("past-tense", "A2", "Прошедшее время", "The past tense",
          "-t/-tt со связующей гласной. Регулярно, но форма зависит от типа основы.",
          "-t/-tt with a linking vowel. Regular, but the exact shape depends on the "
          "stem type.",
          110, ("present-indefinite", "possessive"), ("Tense=Past",)),
    Topic("definite-conjugation", "A2", "Определённое спряжение", "The definite conjugation",
          "Látok egy házat против Látom a házat. Аналога нет ни в русском, ни в "
          "английском, и именно по этому венгры мгновенно узнают иностранца.\n\n"
          "По данным корпуса ошибок: 55 % ошибок — определённый объект с "
          "НЕОПРЕДЕЛЁННЫМ спряжением, ещё 25 % — объект-указательное местоимение "
          "(azt a filmet). Упражнения взвешены под это.\n\n"
          "Тема отрабатывается БЛОКАМИ до точности 80 %, и только потом "
          "перемешивается с другими. Ранний интерливинг мешает закрепить правило.",
          "Látok egy házat versus Látom a házat. There is no analogue in Russian or "
          "English, and it is what makes a foreigner instantly recognisable.\n\n"
          "From learner-corpus data: 55% of errors are a definite object taking the "
          "INDEFINITE conjugation, and a further 25% involve a demonstrative-pronoun "
          "object (azt a filmet). The drills are weighted accordingly.\n\n"
          "This topic is practised in BLOCKS until accuracy reaches 80%, and only then "
          "interleaved. Interleaving too early prevents the rule from consolidating.",
          120, ("past-tense",), ("Definite=Def", "Definite=Ind")),
    Topic("lak-lek", "A2", "Форма -lak/-lek (я тебя)", "The -lak/-lek form (I ... you)",
          "Отдельная форма для «я → тебя»: szeretlek. Одна из немногих вещей в "
          "венгерском, которые проще, чем кажется.",
          "A dedicated 'I → you' form: szeretlek. One of the few things in Hungarian "
          "that is simpler than it looks.",
          130, ("definite-conjugation",), ("Definite=2",)),
    Topic("infinitive", "A2", "Инфинитив с личными окончаниями", "The infinitive with personal endings",
          "-ni, а при выраженном субъекте — с личным окончанием: mennem kell — мне "
          "нужно идти. Нужно постоянно и в быту.",
          "-ni, and with an expressed subject it takes a personal ending: mennem kell — "
          "I must go. Needed constantly in daily life.",
          140, ("definite-conjugation",), ("VerbForm=Inf",)),
    Topic("potential", "A2", "Потенциалис -hat/-het", "The potential -hat/-het",
          "«Мочь» выражается суффиксом, а не отдельным глаголом: olvashat. "
          "Полностью регулярно — редкий подарок.",
          "'Can' is a suffix, not a separate verb: olvashat. Fully regular — a rare gift.",
          150, ("definite-conjugation",), ("Mood=Pot",)),
    Topic("instrumental", "A2", "Инструменталис и ассимиляция v", "The instrumental and v-assimilation",
          "-val/-vel, где v уподобляется предыдущей согласной: vonattal, késsel. "
          "Удвоенный диграф пишется как ssz, а не szsz: busszal.",
          "-val/-vel, where the v assimilates to the preceding consonant: vonattal, "
          "késsel. A doubled digraph is written ssz, not szsz: busszal.",
          160, ("locative-grid",), ("Case=Ins",)),
    Topic("comparison", "A2", "Степени сравнения", "Comparison",
          "-bb для сравнительной, leg- + -bb для превосходной.",
          "-bb for the comparative, leg- + -bb for the superlative.",
          170, ("accusative",), ("Degree=Cmp", "Degree=Sup")),
    Topic("preverbs", "A2", "Глагольные приставки", "Preverbs (igekötők)",
          "meg-, el-, be-, ki-, fel-, le-… Похоже на русские приставки, и это "
          "серьёзно помогает: megy → elmegy, bemegy, kimegy.",
          "meg-, el-, be-, ki-, fel-, le-… Similar to Russian prefixes, which genuinely "
          "helps: megy → elmegy, bemegy, kimegy.",
          180, ("past-tense",), ("Skill=Preverb",)),

    # ── B1: mood, stems, the remaining cases ──────────────────────────────
    Topic("conditional", "B1", "Условное наклонение", "The conditional",
          "-nék/-nál/-na… Нужно РАНО, потому что это форма вежливой просьбы: "
          "kérnék egy kávét. Грамматически это B1, прагматически — первый месяц.",
          "-nék/-nál/-na… Needed EARLY, because it is how polite requests are made: "
          "kérnék egy kávét. Grammatically B1; pragmatically, month one.",
          190, ("infinitive",), ("Mood=Cnd",)),
    Topic("stem-alternation", "B1", "Чередование основ", "Stem alternation",
          "ló→lovak, tó→tavak, fa→fát. Закрытый список примерно из 200 слов. "
          "Это не правило, это список — и учить его надо как список.",
          "ló→lovak, tó→tavak, fa→fát. A closed list of roughly 200 words. This is not a "
          "rule, it is a list — and it should be learned as one.",
          200, ("plural",), ("Skill=StemAlternation",)),
    Topic("imperative", "B1", "Повелительное наклонение", "The imperative",
          "-j- с обширными ассимиляциями: keres→keress, olvas→olvass, "
          "lát→láss. Морфонологически самая неприятная тема курса.",
          "-j- with extensive assimilation: keres→keress, olvas→olvass, lát→láss. "
          "Morphophonologically the nastiest topic in the course.",
          210, ("conditional",), ("Mood=Imp",)),
    Topic("participles", "B1", "Причастия", "Participles",
          "-ó/-ő (настоящее), -t/-tt (прошедшее), -andó/-endő (будущее). "
          "Венгерский использует их там, где русский взял бы придаточное.",
          "-ó/-ő (present), -t/-tt (past), -andó/-endő (future). Hungarian uses these "
          "where Russian or English would use a relative clause.",
          220, ("past-tense",),
          ("VerbForm=PartPres", "VerbForm=PartPast", "VerbForm=PartFut")),
    Topic("remaining-cases", "B1", "Остальные падежи", "The remaining cases",
          "-ért, -ig, -vá/-vé, -kor, -ként. Малочастотные по отдельности, "
          "но вместе покрывают заметную долю текста. Половина из них не "
          "гармонирует вовсе.",
          "-ért, -ig, -vá/-vé, -kor, -ként. Individually infrequent, collectively a "
          "noticeable share of any text. Half of them do not harmonise at all.",
          230, ("instrumental",),
          ("Case=Cau", "Case=Ter", "Case=Tra", "Case=Tem", "Case=Ess", "Case=Dis",
           "Case=Loc")),
    Topic("causative", "B1", "Каузатив", "The causative",
          "-tat/-tet: заставить или дать сделать. megcsináltatom a kocsit — "
          "отдаю машину в ремонт.",
          "-tat/-tet: to have something done. megcsináltatom a kocsit — I'm getting the "
          "car fixed.",
          240, ("potential",), ("Voice=Cau",)),
    Topic("converb", "B1", "Деепричастие", "The converb",
          "-va/-ve: állva — стоя. Часто там, где русский использует деепричастие.",
          "-va/-ve: állva — standing. Often where Russian would use a gerund.",
          250, ("participles",), ("VerbForm=Conv",)),

    # ── B2: what separates correct from native ────────────────────────────
    Topic("iterative", "B2", "Многократность -gat/-get", "The frequentative -gat/-get",
          "olvasgat — почитывает. Оттенок, который в русском передаётся "
          "интонацией или наречием.",
          "olvasgat — reads a bit, on and off. A nuance Russian or English carries with "
          "an adverb or with intonation.",
          260, ("causative",), ("Aspect=Iter",)),
    Topic("focus-word-order", "B2", "Порядок слов и фокус", "Word order and focus",
          "Венгерский — дискурсивно-конфигурационный язык: порядок слов кодирует не "
          "роли (их кодируют падежи), а ФОКУС. Позиция прямо перед глаголом — "
          "фокусная, и приставка при этом отрывается:\n\n"
          "  Elmegyek a boltba.    — нейтрально\n"
          "  A boltba megyek el.   — именно В МАГАЗИН я иду\n"
          "  Én megyek el a boltba. — именно Я иду\n\n"
          "Это осваивается последним и отличает «правильно, но странно» от «как "
          "венгр». Тема открывается только после достаточного объёма аудирования.",
          "Hungarian is discourse-configurational: word order encodes FOCUS, not roles "
          "(cases encode those). The slot immediately before the verb is the focus "
          "position, and the preverb detaches when something else occupies it:\n\n"
          "  Elmegyek a boltba.     — neutral\n"
          "  A boltba megyek el.    — it's TO THE SHOP that I'm going\n"
          "  Én megyek el a boltba. — it's I who am going\n\n"
          "This is acquired last and separates 'correct but odd' from 'native'. The "
          "topic unlocks only after enough listening volume.",
          270, ("preverbs", "converb"), ("Skill=PreverbDetach",)),
    Topic("register", "B2", "Регистры и стиль", "Register and style",
          "Разговорное против официального, письменное против устного, "
          "бюрократический венгерский (ügyintézés) как отдельный поджанр.",
          "Colloquial versus formal, written versus spoken, and bureaucratic Hungarian "
          "(ügyintézés) as a genre of its own.",
          280, ("focus-word-order",), ()),
]


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
