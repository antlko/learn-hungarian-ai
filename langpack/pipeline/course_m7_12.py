"""Intermediate tier, months 7-12: twenty core units that take a learner past the survival
vocabulary of the basic tier.

The basic tier (course_m1..m6) buys errands. This tier buys the topics a person talks
about once the errands are routine: a job, an illness in some detail, the news, a city,
a trip with connections, a contract, a quarrel, the weather, a screen, a school, a
kitchen. The words were chosen from the pack's frequency list (roughly ranks 1 500 to
5 000, the B1 band) and are disjoint from every basic-tier and theme-unit word.

`month` is an internal ordering and checkpoint-placement field; the learner never sees
it (ADR-057). `tier` is what they see. Lines use the syntax of `_wordlist.py`. Glosses
are hand-written; a word whose gloss was uncertain was left out, not guessed.
"""

from __future__ import annotations

from ._wordlist import parse

UNITS = [
    {
        "code": "m7-u1-munka-palya",
        "tier": "intermediate", "month": 7, "ord": 701,
        "title_ru": "Работа и карьера",
        "title_en": "Work and career",
        "goal_ru": "Рассказываю, кем и где работаю, читаю объявление о вакансии и договариваюсь о собеседовании.",
        "goal_en": "I say what I do and where, read a job advert and arrange an interview.",
        "situation_ru": "Объявление о работе, собеседование, разговор с коллегой, заявление на отпуск.",
        "situation_en": "A job advert, an interview, a chat with a colleague, a leave request.",
        "topic_codes": "conditional,participles",
        "words": parse("""
pálya|N|карьера; путь|career; track
karrier|N|карьера|career
szakember|N|специалист|specialist
beosztott|N|подчинённый|subordinate
kirúg|V|уволить (разг.)|to fire
munkanélküli|N|безработный|unemployed
részmunkaidős|A|на неполный день|part-time
jelentkező|N|кандидат; заявитель|applicant
teljesít|V|выполнять|to fulfil, to carry out
irányít|V|направлять; управлять|to direct, to manage
megbíz|V|поручать|to entrust
elkezd|V|начать|to begin
fáradtság|N|усталость|tiredness
siker|N|успех|success
kudarc|N|неудача|failure
kezdő|A|начинающий|beginner
tapasztalt|A|опытный|experienced
ajánlat|N|предложение; оферта|offer
partner|N|партнёр|partner
ügynökség|N|агентство|agency
személyzet|N|персонал|staff
részleg|N|отдел|department
tanácsadó|N|консультант|consultant
szakértő|N|эксперт|expert
kockázat|N|риск|risk
projekt|N|проект|project
pozíció|N|позиция; должность|position
üzleti|A|деловой|business
munkatárs|N|сотрудник|colleague
menedzser|N|менеджер|manager
vállalkozás|N|предприятие; бизнес|business, enterprise
szakmai|A|профессиональный|professional
képzett|A|квалифицированный|skilled
tevékenység|N|деятельность|activity
szolgáltatás|N|услуга|service
befektető|N|инвестор|investor
alapít|V|основывать|to found
alapító|N|основатель|founder
hatékony|A|эффективный|effective
működés|N|работа; функционирование|operation
"""),
    },
    {
        "code": "m7-u2-egeszseg-reszletesen",
        "tier": "intermediate", "month": 7, "ord": 702,
        "title_ru": "Здоровье: подробнее",
        "title_en": "Health in more detail",
        "goal_ru": "Описываю болезнь и самочувствие, понимаю назначение врача и инструкцию к лекарству.",
        "goal_en": "I describe an illness and how I feel, and follow the doctor's orders and a leaflet.",
        "situation_ru": "Приём у терапевта, аптека, больничный лист, разговор о привычках и режиме.",
        "situation_en": "A GP appointment, the pharmacy, a sick note, a talk about habits and routine.",
        "topic_codes": "causative,participles",
        "words": parse("""
köhögés|N|кашель|cough
megfázás|N|простуда|a cold
influenza|N|грипп|flu
allergia|N|аллергия|allergy
gyulladás|N|воспаление|inflammation
törés|N|перелом|fracture
vérzik|V|кровоточить|to bleed
fertőz|V|заражать|to infect
gyógyít|V|лечить|to cure, to heal
kezel|V|лечить; обрабатывать|to treat
adag|N|доза; порция|dose; portion
mellékhatás|N|побочное действие|side effect
súlyos|A|тяжёлый; серьёзный|serious, severe
fertőző|A|заразный|contagious
krónikus|A|хронический|chronic
ápol|V|ухаживать; лечить|to nurse, to care for
ápolónő|N|медсестра|nurse
pihenés|N|отдых|rest
csont|N|кость|bone
bőr|N|кожа|skin
tüdő|N|лёгкое|lung
terhes|A|беременная|pregnant
ideg|N|нерв|nerve
sejt|N|клетка|cell
vírus|N|вирус|virus
terápia|N|терапия|therapy
klinika|N|клиника|clinic
szívroham|N|инфаркт|heart attack
vérzés|N|кровотечение|bleeding
eszméletlen|A|без сознания|unconscious
sérült|A|раненый; повреждённый|injured
orr|N|нос|nose
fül|N|ухо|ear
nyak|N|шея|neck
térd|N|колено|knee
váll|N|плечо|shoulder
ujj|N|палец|finger
kar|N|рука (до кисти)|arm
ajak|N|губа|lip
vak|A|слепой|blind
süket|A|глухой|deaf
halálos|A|смертельный|fatal
járvány|N|эпидемия|epidemic
baktérium|N|бактерия|bacterium
pszichológus|N|психолог|psychologist
pszichiáter|N|психиатр|psychiatrist
csecsemő|N|младенец|infant
szülés|N|роды|childbirth
terhesség|N|беременность|pregnancy
légzés|N|дыхание|breathing
masszázs|N|массаж|massage
mentális|A|психический|mental
dohányzik|V|курить|to smoke
cigaretta|N|сигарета|cigarette
orvosság|N|лекарство|remedy
kábítószer|N|наркотик|narcotic drug
"""),
    },
    {
        "code": "m7-u3-media-hirek",
        "tier": "intermediate", "month": 7, "ord": 703,
        "title_ru": "Новости и СМИ",
        "title_en": "News and media",
        "goal_ru": "Понимаю заголовки и короткие новости, пересказываю, что услышал, и говорю, откуда это знаю.",
        "goal_en": "I follow headlines and short news items, retell what I heard and say where I heard it.",
        "situation_ru": "Утренние новости по радио, заголовок на сайте, разговор о прочитанной статье.",
        "situation_en": "The morning radio news, a headline online, a chat about an article you read.",
        "topic_codes": "participles,register",
        "words": parse("""
hír|N|новость|news item
cikk|N|статья|article
riport|N|репортаж|report
interjú|N|интервью|interview
újságíró|N|журналист|journalist
szerkesztő|N|редактор|editor
közvélemény|N|общественное мнение|public opinion
sajtó|N|пресса|the press
adás|N|эфир; передача|broadcast
reklám|N|реклама|advert
közöl|V|сообщать; публиковать|to report; to publish
megjelenik|V|появляться; выходить (в печати)|to appear; to come out
beszámol|V|докладывать; рассказывать о|to report on
állítás|N|утверждение|claim, statement
tény|N|факт|fact
forrás|N|источник|source
botrány|N|скандал|scandal
áldozat|N|жертва|victim; sacrifice
rendőrség|N|полиция|police
szavazás|N|голосование|vote
elnök|N|председатель; президент|chairman; president
miniszter|N|министр|minister
úgynevezett|A|так называемый|so-called
nyilvános|A|публичный|public
titkos|A|тайный|secret
aktuális|A|актуальный; текущий|current, topical
beszéd|N|речь|speech
beszélgetés|N|беседа; разговор|conversation
felirat|N|надпись; субтитры|caption, subtitles
riporter|N|репортёр|reporter
média|N|СМИ; медиа|media
stúdió|N|студия|studio
bemutató|N|премьера; презентация|premiere, presentation
pletyka|N|слух; сплетня|gossip
híres|A|известный|famous
ismert|A|известный|known
népszerű|A|популярный|popular
ismeretlen|A|неизвестный|unknown
nyilvánosság|N|гласность; публика|publicity, the public
említ|V|упоминать|to mention
kifejezés|N|выражение|expression
híradó|N|выпуск новостей|news bulletin
sajtótájékoztató|N|пресс-конференция|press conference
kommunikáció|N|коммуникация|communication
epizód|N|эпизод; серия|episode
képernyő|N|экран|screen
mikrofon|N|микрофон|microphone
konferencia|N|конференция|conference
bejelentés|N|объявление; заявление|announcement
elemzés|N|анализ|analysis
leírás|N|описание|description
leír|V|описывать; записывать|to describe, to write down
kritikus|N|критик|critic
reakció|N|реакция|reaction
reagál|V|реагировать|to react
benyomás|N|впечатление|impression
"""),
    },
    {
        "code": "m7-u4-varosi-elet",
        "tier": "intermediate", "month": 7, "ord": 704,
        "title_ru": "Жизнь в городе",
        "title_en": "City life",
        "goal_ru": "Ориентируюсь в городе, пользуюсь услугами и объясняю, где что находится.",
        "goal_en": "I find my way round a city, use its services and explain where things are.",
        "situation_ru": "Спросить дорогу, пробка, парковка, ремонт на улице, соседи и шум.",
        "situation_en": "Asking the way, a traffic jam, parking, roadworks, neighbours and noise.",
        "topic_codes": "remaining-cases,postpositions",
        "words": parse("""
büntetés|N|штраф; наказание|fine; punishment
szobor|N|статуя|statue
part|N|берег|bank, shore
tömeg|N|толпа; масса|crowd; mass
biztonságos|A|безопасный|safe
helyi|A|местный|local
távol|D|далеко|far
park|N|парк|park
templom|N|церковь; храм|church
városi|A|городской|urban
utcai|A|уличный|street
torony|N|башня|tower
kastély|N|замок|castle
palota|N|дворец|palace
alagút|N|туннель|tunnel
jármű|N|транспортное средство|vehicle
reptér|N|аэропорт (разг.)|airport
kocsma|N|пивная; кабак|pub
hotel|N|отель|hotel
közeli|A|близкий; соседний|nearby
távoli|A|далёкий|distant
messzi|A|далёкий|distant
messzire|D|далеко (куда)|far away
errefelé|D|в этих краях|around here
mindenhol|D|везде|everywhere
mindenütt|D|повсюду|everywhere
sehol|D|нигде|nowhere
máshol|D|в другом месте|elsewhere
valahova|D|куда-нибудь|somewhere (to)
sehova|D|никуда|nowhere (to)
lefelé|D|вниз|downwards
felfelé|D|вверх|upwards
befelé|D|внутрь|inwards
hazafelé|D|по дороге домой|on the way home
északi|A|северный|northern
déli|A|южный|southern
keleti|A|восточный|eastern
nyugati|A|западный|western
észak|N|север|north
nyugat|N|запад|west
kelet|N|восток|east
vidéki|A|сельский; провинциальный|rural, provincial
falusi|A|деревенский|village
kerti|A|садовый|garden
hegyi|A|горный|mountain
áruház|N|универмаг|department store
pláza|N|торговый центр|shopping centre
állatkert|N|зоопарк|zoo
városháza|N|мэрия|town hall
megye|N|область; комитат|county
lakosság|N|население|population
lakos|N|житель|inhabitant
kút|N|колодец|well
kanyar|N|поворот|bend
felül|D|вверху; сверху|above
elöl|D|впереди|in front
"""),
    },
    {
        "code": "m8-u1-utazas-szervezes",
        "tier": "intermediate", "month": 8, "ord": 705,
        "title_ru": "Поездки и пересадки",
        "title_en": "Trips and connections",
        "goal_ru": "Планирую поездку, покупаю билеты, решаю проблему с рейсом и отелем.",
        "goal_en": "I plan a trip, buy tickets and sort out a problem with a flight or a hotel.",
        "situation_ru": "Касса вокзала, регистрация в аэропорту, бронь отеля, задержка поезда.",
        "situation_en": "A station ticket window, airport check-in, a hotel booking, a delayed train.",
        "topic_codes": "conditional,remaining-cases",
        "words": parse("""
vízum|N|виза|visa
pályaudvar|N|вокзал|railway station
módosít|V|изменять|to change, to amend
kártérítés|N|компенсация|compensation
tájékoztat|V|информировать|to inform
tájékoztatás|N|информация; справка|information
visszatér|V|возвращаться|to return
tábor|N|лагерь|camp
óceán|N|океан|ocean
farm|N|ферма|farm
medence|N|бассейн|pool
fedélzet|N|палуба|deck
felszerelés|N|снаряжение|equipment
látogató|N|посетитель|visitor
látogatás|N|визит|visit
látogat|V|навещать|to visit
távozik|V|уходить; уезжать|to leave
belép|V|входить|to enter
kilép|V|выходить|to step out
kimegy|V|выйти|to go out
bejön|V|входить (сюда)|to come in
kijön|V|выходить (сюда)|to come out
elhagy|V|покидать|to leave
eljut|V|добраться|to reach
menekül|V|бежать; спасаться|to flee
halad|V|продвигаться|to proceed
repül|V|летать|to fly
repülőgép|N|самолёт|aeroplane
repülés|N|полёт|flight
mérföld|N|миля|mile
távolság|N|расстояние|distance
sebesség|N|скорость|speed
séta|N|прогулка|walk
utazási|A|туристический|travel
lakókocsi|N|жилой прицеп|caravan
közlekedési|A|транспортный|traffic
francia|A|французский|French
német|A|немецкий|German
olasz|A|итальянский|Italian
spanyol|A|испанский|Spanish
kínai|A|китайский|Chinese
japán|A|японский|Japanese
görög|A|греческий|Greek
török|A|турецкий|Turkish
lengyel|A|польский|Polish
svéd|A|шведский|Swedish
holland|A|нидерландский|Dutch
svájci|A|швейцарский|Swiss
arab|A|арабский|Arabic
indiai|A|индийский|Indian
európai|A|европейский|European
"""),
    },
    {
        "code": "m8-u2-penz-szerzodes",
        "tier": "intermediate", "month": 8, "ord": 706,
        "title_ru": "Деньги и договоры",
        "title_en": "Money and contracts",
        "goal_ru": "Разбираюсь в счёте, договоре и налоге, спрашиваю условия и не подписываю то, чего не понял.",
        "goal_en": "I make sense of a bill, a contract and a tax form, ask about terms and don't sign what I haven't understood.",
        "situation_ru": "Банк, договор аренды, коммунальные платежи, возврат товара, налоговая.",
        "situation_en": "The bank, a lease, utility bills, returning goods, the tax office.",
        "topic_codes": "conditional,register",
        "words": parse("""
befektetés|N|инвестиция|investment
biztosító|N|страховая компания|insurer
kedvezmény|N|скидка; льгота|discount
költséges|A|затратный|costly
árul|V|продавать; торговать|to sell
elad|V|продать|to sell
alku|N|торг; сделка|bargain
vétel|N|покупка|purchase
megvesz|V|купить|to buy
értékes|A|ценный|valuable
kincs|N|сокровище|treasure
jutalom|N|награда|reward
gyémánt|N|алмаз|diamond
ékszer|N|украшение|jewel
gyűrű|N|кольцо|ring
biztosít|V|страховать; обеспечивать|to insure, to ensure
vásárló|N|покупатель|customer
vásár|N|ярмарка|fair
kereskedő|N|торговец|merchant
kereskedik|V|торговать|to trade
kereskedelmi|A|торговый|commercial
csőd|N|банкротство|bankruptcy
profit|N|прибыль|profit
adomány|N|пожертвование|donation
márka|N|марка; бренд|brand
luxus|N|роскошь|luxury
olcsón|D|дёшево|cheaply
bankár|N|банкир|banker
üzletember|N|бизнесмен|businessman
élelmiszer|N|продукты питания|groceries
készlet|N|запас|stock, supply
"""),
    },
    {
        "code": "m8-u3-erzelmek-kapcsolatok",
        "tier": "intermediate", "month": 8, "ord": 707,
        "title_ru": "Чувства и отношения",
        "title_en": "Feelings and relationships",
        "goal_ru": "Говорю о близких людях, о ссоре и примирении, о том, что я чувствую, спокойно и по делу.",
        "goal_en": "I talk about people close to me, a quarrel and making up, and what I feel, calmly and to the point.",
        "situation_ru": "Разговор с другом, семейный ужин, свидание, извинение за опоздание.",
        "situation_en": "A talk with a friend, a family dinner, a date, an apology for being late.",
        "topic_codes": "conditional,causative",
        "words": parse("""
ellenség|N|враг|enemy
megbocsát|V|прощать|to forgive
sért|V|оскорблять|to offend
csalódás|N|разочарование|disappointment
meglepetés|N|сюрприз; удивление|surprise
hiány|N|нехватка; отсутствие|lack, absence
hűséges|A|верный|faithful
megbízható|A|надёжный|reliable
önző|A|эгоистичный|selfish
nagylelkű|A|великодушный|generous
csók|N|поцелуй|kiss
csinos|A|хорошенький|pretty
aranyos|A|милый|sweet, cute
cuki|A|милый (разг.)|cute
jóképű|A|красивый (о мужчине)|handsome
szépség|N|красота|beauty
bájos|A|очаровательный|charming
vonzó|A|привлекательный|attractive
vonz|V|привлекать|to attract
romantikus|A|романтичный|romantic
szerető|N|любовник; любовница|lover
menyasszony|N|невеста|bride
vőlegény|N|жених|groom
kedvel|V|симпатизировать|to like
nevetséges|A|смешной; нелепый|ridiculous
ijesztő|A|страшный|scary
ijeszt|V|пугать|to frighten
rémes|A|ужасный|awful
borzalmas|A|ужасный|horrible
undorító|A|отвратительный|disgusting
szánalmas|A|жалкий|pathetic
kínos|A|неловкий|awkward, embarrassing
mókás|A|забавный|funny
izgalmas|A|захватывающий|exciting
rettenetes|A|ужасный|terrible
feszült|A|напряжённый|tense
idegesítő|A|раздражающий|annoying
boldogan|D|счастливо|happily
félt|V|бояться за|to fear for
un|V|скучать от; надоедать|to be bored of
szórakozik|V|развлекаться|to have fun
szórakozás|N|развлечение|entertainment
szórakoztató|A|занимательный|entertaining
bosszú|N|месть|revenge
dicsőség|N|слава|glory
szenvedés|N|страдание|suffering
kegyelem|N|милосердие; помилование|mercy
hűség|N|верность|faithfulness
becsület|N|честь|honour
bölcs|A|мудрый|wise
gyengéd|A|нежный|gentle, tender
ostoba|A|глупый|foolish
butaság|N|глупость|stupidity
zaklatott|A|взволнованный|agitated
kétségbeesett|A|отчаявшийся|desperate
meglepett|A|удивлённый|surprised
"""),
    },
    {
        "code": "m9-u1-vita-erv",
        "tier": "intermediate", "month": 9, "ord": 708,
        "title_ru": "Спор и аргумент",
        "title_en": "Debate and argument",
        "goal_ru": "Привожу довод, отвечаю на возражение, признаю правоту собеседника и меняю мнение.",
        "goal_en": "I give a reason, answer an objection, concede a point and change my mind.",
        "situation_ru": "Обсуждение на собрании жильцов, дискуссия за столом, комментарий под статьёй.",
        "situation_en": "A residents' meeting, a discussion at the table, a comment under an article.",
        "topic_codes": "conditional,focus-word-order",
        "words": parse("""
indok|N|основание; повод|ground, reason
feltevés|N|предположение|assumption
álláspont|N|точка зрения|stance
nézet|N|взгляд|view
tárgyal|V|вести переговоры; обсуждать|to negotiate; to discuss
egyezik|V|совпадать|to agree; to match
megvéd|V|защищать|to defend
támad|V|нападать; возникать|to attack; to arise
bizonyít|V|доказывать|to prove
igazol|V|подтверждать; оправдывать|to justify; to confirm
kételkedik|V|сомневаться|to doubt
feltételez|V|предполагать|to assume
meggyőződés|N|убеждение|conviction
meggyőző|A|убедительный|convincing
elismer|V|признавать|to admit, to acknowledge
ellentmond|V|противоречить|to contradict
különbözik|V|отличаться|to differ
hangsúlyoz|V|подчёркивать|to stress
megjegyzés|N|замечание|remark
észrevétel|N|замечание; наблюдение|observation
félreérthető|A|двусмысленный|ambiguous
elfogult|A|предвзятый|biased
magyarázat|N|объяснение|explanation
megmagyaráz|V|объяснить|to explain
bizonyára|D|несомненно|surely
egyértelműen|D|однозначно|clearly
határozottan|D|решительно|firmly
kétségtelen|A|несомненный|undoubted
kétségtelenül|D|несомненно|undoubtedly
hamis|A|ложный; фальшивый|false
valós|A|реальный|real
valóság|N|реальность|reality
valóságos|A|настоящий|actual
szándékosan|D|умышленно|on purpose
szándékos|A|умышленный|deliberate
feltétlenül|D|непременно|absolutely
kizárólag|D|исключительно|exclusively
csakis|D|только|only
függ|V|зависеть|to depend
vonatkozik|V|относиться|to apply to
utal|V|ссылаться; указывать|to refer
törődik|V|заботиться; обращать внимание|to care about
kapcsolatos|A|связанный с|related to
ellenfél|N|противник|opponent
konkrét|A|конкретный|concrete
konkrétan|D|конкретно|specifically
lényeges|A|существенный|essential
alapvető|A|основной|fundamental
alapvetően|D|в основном|fundamentally
elvileg|D|в принципе|in theory
tulajdonság|N|свойство|property, quality
jellemző|A|характерный|characteristic
jelentőség|N|значение|significance
egyedi|A|уникальный|individual
elsődleges|A|первичный|primary
tisztáz|V|прояснить|to clarify
"""),
    },
    {
        "code": "m9-u2-tarsadalom-jog",
        "tier": "intermediate", "month": 9, "ord": 709,
        "title_ru": "Общество и закон",
        "title_en": "Society and the law",
        "goal_ru": "Понимаю простые тексты о правах и обязанностях, объясняю свою ситуацию чиновнику.",
        "goal_en": "I follow plain texts on rights and duties and explain my situation to an official.",
        "situation_ru": "Миграционная служба, штраф, свидетель аварии, регистрация по месту жительства.",
        "situation_en": "The immigration office, a fine, witnessing an accident, registering an address.",
        "topic_codes": "participles,register",
        "words": parse("""
bíróság|N|суд|court
bíró|N|судья|judge
per|N|судебный процесс|lawsuit
vád|N|обвинение|charge
tanú|N|свидетель|witness
vallomás|N|показание|testimony
ítélet|N|приговор|verdict, sentence
bűn|N|грех; преступление|sin; crime
bűncselekmény|N|преступление|criminal offence
bűnös|A|виновный|guilty
ártatlan|A|невиновный|innocent
lop|V|красть|to steal
tolvaj|N|вор|thief
börtön|N|тюрьма|prison
tilt|V|запрещать|to forbid
képviselő|N|депутат; представитель|MP; representative
polgármester|N|мэр|mayor
felügyel|V|надзирать|to supervise
igazságtalan|A|несправедливый|unfair
menekült|N|беженец|refugee
gyanúsított|N|подозреваемый|suspect
gyanú|N|подозрение|suspicion
gyanús|A|подозрительный|suspicious
gyanúsít|V|подозревать|to suspect
nyomozás|N|расследование|investigation
bűnöző|N|преступник|criminal
ügyész|N|прокурор|prosecutor
esküdt|N|присяжный|juror
szemtanú|N|очевидец|eyewitness
elkövet|V|совершить|to commit
elkövető|N|исполнитель; нарушитель|perpetrator
rablás|N|грабёж|robbery
tettes|N|виновник|culprit
törvényes|A|законный|lawful
illegális|A|незаконный|illegal
jogi|A|юридический|legal
őrizet|N|содержание под стражей|custody
fogoly|N|заключённый; пленный|prisoner
rab|N|заключённый|prisoner
felügyelő|N|надзиратель; инспектор|inspector
felügyelet|N|надзор|supervision
eskü|N|присяга|oath
esküdtszék|N|суд присяжных|jury
követel|V|требовать|to demand
megakadályoz|V|предотвращать|to prevent
tiltott|A|запрещённый|forbidden
bírósági|A|судебный|court
bűntény|N|преступление|crime
rendőri|A|полицейский|police
menedék|N|убежище|refuge
szökevény|N|беглец|fugitive
zaklat|V|преследовать; домогаться|to harass
csal|V|обманывать|to cheat
fenyeget|V|угрожать|to threaten
azonosít|V|опознавать|to identify
megszerez|V|получить; приобрести|to obtain
"""),
    },
    {
        "code": "m9-u3-termeszet-idojaras",
        "tier": "intermediate", "month": 9, "ord": 710,
        "title_ru": "Природа и погода",
        "title_en": "Nature and weather",
        "goal_ru": "Рассказываю о погоде и о поездке на природу, понимаю прогноз и предупреждение.",
        "goal_en": "I talk about the weather and a trip into nature, and follow a forecast or a warning.",
        "situation_ru": "Прогноз на выходные, поход в горы, гроза, разговор о смене климата.",
        "situation_en": "The weekend forecast, a hike in the hills, a storm, a talk about climate change.",
        "topic_codes": "iterative,participles",
        "words": parse("""
fagyos|A|морозный|frosty
havazik|V|идти (о снеге)|to snow
szárad|V|сохнуть|to dry
földrengés|N|землетрясение|earthquake
táj|N|пейзаж|landscape
mező|N|поле; луг|field, meadow
rét|N|луг|meadow
patak|N|ручей|stream
kő|N|камень|stone
föld|N|земля|earth, soil
levegő|N|воздух|air
fa|N|дерево|tree
állat|N|животное|animal
madár|N|птица|bird
farkas|N|волк|wolf
medve|N|медведь|bear
róka|N|лиса|fox
kígyó|N|змея|snake
macska|N|кошка|cat
nyúl|N|заяц; кролик|hare, rabbit
tehén|N|корова|cow
kecske|N|коза|goat
birka|N|овца|sheep
malac|N|поросёнок|piglet
szamár|N|осёл|donkey
elefánt|N|слон|elephant
oroszlán|N|лев|lion
majom|N|обезьяна|monkey
patkány|N|крыса|rat
béka|N|лягушка|frog
pók|N|паук|spider
rovar|N|насекомое|insect
méh|N|пчела|bee
hangya|N|муравей|ant
bogár|N|жук|beetle
sas|N|орёл|eagle
bagoly|N|сова|owl
galamb|N|голубь|pigeon
tyúk|N|курица|hen
teknős|N|черепаха|tortoise
mókus|N|белка|squirrel
cápa|N|акула|shark
bálna|N|кит|whale
dzsungel|N|джунгли|jungle
mocsár|N|болото|swamp
szárazföld|N|суша; материк|mainland
hullám|N|волна|wave
hurrikán|N|ураган|hurricane
gőz|N|пар|steam
hamu|N|пепел|ash
por|N|пыль|dust
talaj|N|почва|soil
ág|N|ветка|branch
gyökér|N|корень|root
mag|N|семя|seed
bogyó|N|ягода|berry
"""),
    },
    {
        "code": "m10-u1-technologia",
        "tier": "intermediate", "month": 10, "ord": 711,
        "title_ru": "Техника и интернет",
        "title_en": "Technology and the internet",
        "goal_ru": "Пользуюсь техникой и сервисами по-венгерски, объясняю поломку и прошу помощи с настройкой.",
        "goal_en": "I use devices and online services in Hungarian, describe a fault and ask for help setting something up.",
        "situation_ru": "Звонок в службу поддержки, настройка телефона, пароль, онлайн-заявка.",
        "situation_en": "A call to support, setting up a phone, a password, an online form.",
        "topic_codes": "imperative,causative",
        "words": parse("""
technika|N|техника|technology
készülék|N|устройство; прибор|device, appliance
számítás|N|вычисление|calculation
hálózat|N|сеть|network
honlap|N|сайт|website
letölt|V|скачивать|to download
elment|V|сохранять|to save
nyomtat|V|печатать (на принтере)|to print
nyomtató|N|принтер|printer
billentyűzet|N|клавиатура|keyboard
egér|N|мышь|mouse
kijelző|N|экран; дисплей|display
akkumulátor|N|аккумулятор|battery
program|N|программа|program
alkalmazás|N|приложение|application
beállít|V|настраивать|to set, to adjust
fejlesztés|N|разработка|development
fejlett|A|развитой; продвинутый|developed, advanced
automatikus|A|автоматический|automatic
digitális|A|цифровой|digital
csillag|N|звезда|star
hold|N|луна|moon
égbolt|N|небосвод|sky
bolygó|N|планета|planet
univerzum|N|вселенная|universe
világegyetem|N|вселенная|universe
űr|N|космос; пустота|space, void
űrhajó|N|космический корабль|spaceship
űrhajós|N|космонавт|astronaut
rakéta|N|ракета|rocket
sugárzás|N|излучение|radiation
robot|N|робот|robot
energia|N|энергия|energy
elektromos|A|электрический|electric
nukleáris|A|ядерный|nuclear
oxigén|N|кислород|oxygen
vegyi|A|химический|chemical
kémiai|A|химический|chemical
fizika|N|физика|physics
kísérlet|N|эксперимент|experiment
kísérleti|A|экспериментальный|experimental
tudományos|A|научный|scientific
technológia|N|технология|technology
technikai|A|технический|technical
modell|N|модель|model
elektronikus|A|электронный|electronic
laptop|N|ноутбук|laptop
memória|N|память (устройства)|memory
számítógépes|A|компьютерный|computer
hacker|N|хакер|hacker
verzió|N|версия|version
mesterséges|A|искусственный|artificial
"""),
    },
    {
        "code": "m10-u2-oktatas",
        "tier": "intermediate", "month": 10, "ord": 712,
        "title_ru": "Образование и учёба",
        "title_en": "Education and study",
        "goal_ru": "Говорю об учёбе: что изучаю, как сдаю экзамены, что планирую дальше.",
        "goal_en": "I talk about study: what I am learning, how I take exams and what I plan next.",
        "situation_ru": "Запись на курс, консультация преподавателя, экзамен, разговор о школе детей.",
        "situation_en": "Enrolling on a course, a tutor's consultation, an exam, a talk about the children's school.",
        "topic_codes": "participles,infinitive",
        "words": parse("""
oktatás|N|образование; обучение|education, teaching
előadás|N|лекция; доклад|lecture; talk
szeminárium|N|семинар|seminar
tananyag|N|учебный материал|course material
jegyzet|N|конспект|notes
gyakorlat|N|практика; упражнение|practice; exercise
felvételi|N|вступительный экзамен|entrance exam
gyakorol|V|тренировать|to practise
megjegyez|V|запомнить|to remember
felejt|V|забывать|to forget
megtanul|V|выучить|to learn (fully)
kutat|V|исследовать|to research
tudós|N|учёный|scholar
tanárnő|N|учительница|female teacher
egyetemista|N|студент|university student
hallgató|N|слушатель; студент|student
ösztöndíj|N|стипендия|scholarship
kollégium|N|общежитие|hall of residence
szótár|N|словарь|dictionary
nyelvtan|N|грамматика|grammar
professzor|N|профессор|professor
teszt|N|тест|test
próba|N|проба; репетиция|trial, rehearsal
egyetemi|A|университетский|university
matek|N|математика (разг.)|maths
töri|N|история (шк. разг.)|history (school slang)
végzős|A|выпускной|final-year
gyakornok|N|стажёр|trainee
tanítvány|N|ученик|disciple, pupil
művelt|A|образованный|educated
olvasott|A|начитанный|well-read
értelmes|A|разумный|intelligent
tanult|A|образованный|learned
képzelet|N|воображение|imagination
fantázia|N|фантазия|fantasy
kreatív|A|творческий|creative
elolvas|V|прочитать|to read through
felkészül|V|подготовиться|to prepare
felkészült|A|подготовленный|prepared
példány|N|экземпляр|copy
fejezet|N|глава|chapter
ábra|N|рисунок; схема|figure
tábla|N|доска; табличка|board
mondás|N|поговорка|saying
megismer|V|познакомиться; узнать|to get to know
felismer|V|узнать; распознать|to recognise
felfedez|V|открыть|to discover
alkot|V|создавать|to create
teremt|V|творить|to create
"""),
    },
    {
        "code": "m10-u3-kultura-muveszet",
        "tier": "intermediate", "month": 10, "ord": 713,
        "title_ru": "Культура и искусство",
        "title_en": "Culture and the arts",
        "goal_ru": "Говорю о книге, фильме, концерте и выставке: что понравилось и почему.",
        "goal_en": "I talk about a book, a film, a concert and an exhibition: what I liked and why.",
        "situation_ru": "Билеты в театр, обсуждение фильма, музей, вечер поэзии.",
        "situation_en": "Theatre tickets, discussing a film, a museum, a poetry evening.",
        "topic_codes": "participles,register",
        "words": parse("""
művész|N|художник; артист|artist
színésznő|N|актриса|actress
szerep|N|роль|role
jelmez|N|костюм (театральный)|costume
közönség|N|публика|audience
taps|N|аплодисменты|applause
szöveg|N|текст|text, lyrics
dob|N|барабан|drum
rendező|N|режиссёр|director
mű|N|произведение|work
kiadó|N|издательство|publisher
dráma|N|драма|drama
forgatókönyv|N|сценарий|screenplay
szereplő|N|персонаж; участник|character, participant
opera|N|опера|opera
album|N|альбом|album
ritmus|N|ритм|rhythm
kórus|N|хор|choir
zenei|A|музыкальный|musical
művészeti|A|художественный|artistic
legenda|N|легенда|legend
mesél|V|рассказывать (историю)|to tell a story
humor|N|юмор|humour
tréfa|N|шутка|joke
poén|N|острота; соль анекдота|punchline
cirkusz|N|цирк|circus
bohóc|N|клоун|clown
bál|N|бал|ball
szalon|N|салон|salon
stílus|N|стиль|style
látvány|N|зрелище|spectacle
fotó|N|фото|photo
sztár|N|звезда (артист)|star
táncos|N|танцор|dancer
színpad|N|сцена|stage
hagyományos|A|традиционный|traditional
klasszikus|A|классический|classic
modern|A|современный|modern
szimbólum|N|символ|symbol
ünnepi|A|праздничный|festive
szertartás|N|обряд|ceremony
egyház|N|церковь (организация)|church
vallási|A|религиозный|religious
vallásos|A|верующий|religious
ima|N|молитва|prayer
imádkozik|V|молиться|to pray
pap|N|священник|priest
szerzetes|N|монах|monk
apáca|N|монахиня|nun
mennyország|N|рай; небеса|heaven
isteni|A|божественный|divine
angyal|N|ангел|angel
boszorkány|N|ведьма|witch
varázslat|N|колдовство; волшебство|magic
tündér|N|фея|fairy
sárkány|N|дракон|dragon
"""),
    },
    {
        "code": "m10-u4-fozes-etel",
        "tier": "intermediate", "month": 10, "ord": 714,
        "title_ru": "Кухня и приготовление еды",
        "title_en": "Cooking and kitchen",
        "goal_ru": "Читаю рецепт, готовлю по нему и объясняю, как приготовить блюдо.",
        "goal_en": "I read a recipe, cook from it and explain how to make a dish.",
        "situation_ru": "Рецепт бабушки, поход на рынок, совместная готовка, заказ на кухне.",
        "situation_en": "A grandmother's recipe, a trip to the market, cooking together, ordering in the kitchen.",
        "topic_codes": "imperative,iterative",
        "words": parse("""
fogás|N|блюдо (подача)|course
sül|V|печься; жариться|to bake; to roast
fő|V|вариться|to boil
forral|V|кипятить|to boil
fazék|N|кастрюля; горшок|pot
krém|N|крем|cream
whisky|N|виски|whisky
kóla|N|кола|cola
fagyi|N|мороженое (разг.)|ice cream
jégkrém|N|мороженое (эскимо)|ice lolly
pirítós|N|тост|toast
palack|N|бутылка|bottle
tál|N|блюдо; миска|dish, bowl
tálca|N|поднос|tray
hordó|N|бочка|barrel
sütő|N|духовка|oven
kamra|N|кладовая|pantry
séf|N|шеф-повар|chef
korty|N|глоток|gulp
falat|N|кусок (еды)|mouthful
zsír|N|жир|fat
zsíros|A|жирный|fatty
folyadék|N|жидкость|liquid
ízletes|A|вкусный|tasty
ízlés|N|вкус (предпочтения)|taste
rág|V|жевать|to chew
nyel|V|глотать|to swallow
harap|V|кусать|to bite
dohány|N|табак|tobacco
pipa|N|трубка|pipe
szivar|N|сигара|cigar
disznó|N|свинья|pig
"""),
    },
    {
        "code": "m11-u1-otthon-javitas",
        "tier": "intermediate", "month": 11, "ord": 715,
        "title_ru": "Дом и мелкий ремонт",
        "title_en": "Home and repairs",
        "goal_ru": "Объясняю мастеру, что сломалось, договариваюсь о ремонте и о цене, понимаю инструкцию.",
        "goal_en": "I tell a repairman what is broken, agree on the work and the price, and follow instructions.",
        "situation_ru": "Течёт кран, нет отопления, покраска стен, сборка мебели.",
        "situation_en": "A leaking tap, no heating, painting walls, assembling furniture.",
        "topic_codes": "causative,imperative",
        "words": parse("""
kazán|N|котёл|boiler
cső|N|труба|pipe
biztosíték|N|предохранитель|fuse
konnektor|N|розетка|socket
kapcsoló|N|выключатель|switch
villanyszerelő|N|электрик|electrician
mos|V|мыть; стирать|to wash
kunyhó|N|хижина|hut
pajta|N|сарай|barn
istálló|N|конюшня|stable
lakótárs|N|сосед по квартире|flatmate
szobatárs|N|сосед по комнате|roommate
bejárati|A|входной|entrance
helyiség|N|помещение|room
pad|N|скамья|bench
törülköző|N|полотенце|towel
pelenka|N|подгузник|nappy
fék|N|тормоз|brake
lánc|N|цепь|chain
fonál|N|нить; пряжа|yarn
szál|N|нить; волокно|thread
szövet|N|ткань|fabric
bot|N|палка|stick
fém|N|металл|metal
vas|N|железо|iron
acél|N|сталь|steel
szén|N|уголь|coal
kristály|N|кристалл|crystal
műanyag|N|пластмасса|plastic
gumi|N|резина|rubber
bánya|N|шахта|mine
lap|N|лист|sheet
lemez|N|пластина|plate
gödör|N|яма|pit
árok|N|канава|ditch
gát|N|плотина|dam
rács|N|решётка|grating
korlát|N|перила|railing
lyuk|N|дыра|hole
rés|N|щель|slit
gomb|N|пуговица; кнопка|button
sisak|N|шлем|helmet
tartály|N|резервуар|tank
folt|N|пятно|stain
piszok|N|грязь|dirt
rongy|N|тряпка|rag
"""),
    },
    {
        "code": "m11-u2-sport-hobbi",
        "tier": "intermediate", "month": 11, "ord": 716,
        "title_ru": "Спорт и увлечения",
        "title_en": "Sport and hobbies",
        "goal_ru": "Рассказываю о спорте и хобби, договариваюсь о тренировке и обсуждаю матч.",
        "goal_en": "I talk about sport and hobbies, arrange a workout and discuss a match.",
        "situation_ru": "Запись в зал, разговор о матче, поход с друзьями, выходные за городом.",
        "situation_en": "Joining a gym, talking about a match, a hike with friends, a weekend out of town.",
        "topic_codes": "potential,iterative",
        "words": parse("""
sportoló|N|спортсмен|sportsperson
versenyző|N|участник; гонщик|competitor
mérkőzés|N|матч|match
gól|N|гол|goal
győzelem|N|победа|victory
vereség|N|поражение|defeat
nyer|V|выигрывать|to win
döntetlen|N|ничья|draw
szurkoló|N|болельщик|supporter
úszás|N|плавание|swimming
korcsolya|N|коньки|skates
mászik|V|лазить; подниматься|to climb
rúg|V|бить ногой|to kick
tornaterem|N|спортзал|gym hall
köt|V|вязать; связывать|to knit; to tie
golf|N|гольф|golf
rekord|N|рекорд|record
érem|N|медаль|medal
ütő|N|ракетка; бита|racket, bat
vadászat|N|охота|hunt
vadászik|V|охотиться|to hunt
vadász|N|охотник|hunter
baseball|N|бейсбол|baseball
tempó|N|темп|tempo
edzőterem|N|тренажёрный зал|gym
kocka|N|кубик; куб|cube, die
játszma|N|партия|game, set
dobás|N|бросок|throw
találat|N|попадание|hit
ugrás|N|прыжок|jump
győztes|A|победный; победивший|winning
vesztes|A|проигравший|losing
nyertes|A|выигравший|winning
szerencsejáték|N|азартная игра|gambling
lottó|N|лотерея|lottery
főnyeremény|N|главный приз|jackpot
rajongó|N|поклонник; фанат|fan
rajong|V|увлекаться; обожать|to be a fan of
mozgalom|N|движение|movement
"""),
    },
    {
        "code": "m11-u3-ido-tervezes",
        "tier": "intermediate", "month": 11, "ord": 717,
        "title_ru": "Время и планы",
        "title_en": "Time and planning",
        "goal_ru": "Планирую день и неделю, переношу встречу, говорю о сроках и о том, что успею.",
        "goal_en": "I plan a day and a week, reschedule a meeting and talk about deadlines and what I can manage.",
        "situation_ru": "Календарь на неделю, перенос встречи, планы на отпуск, сроки проекта.",
        "situation_en": "A weekly calendar, moving a meeting, holiday plans, project deadlines.",
        "topic_codes": "time-and-date,conditional",
        "words": parse("""
terv|N|план|plan
korszak|N|эпоха|era
nemsokára|D|вскоре|soon
időben|D|вовремя|on time
régóta|D|давно|for a long time
mostantól|D|отныне|from now on
ezúttal|D|на этот раз|this time
ezután|D|после этого|after this
legutóbb|D|в последний раз|last time
amióta|D|с тех пор как|since
ameddig|D|до тех пор, пока|as long as
addigra|D|к тому времени|by then
másodszor|D|во второй раз|for the second time
többször|D|несколько раз|several times
máskor|D|в другой раз|another time
mindennap|D|каждый день|every day
idén|D|в этом году|this year
folyamatosan|D|постоянно; непрерывно|continuously
egyfolytában|D|без перерыва|non-stop
mostanáig|D|до сих пор|until now
ezentúl|D|впредь|from now on
utóbb|D|позднее|afterwards
harmadszor|D|в третий раз|for the third time
percenként|D|каждую минуту|every minute
legtöbbször|D|чаще всего|most of the time
mindenkor|D|всегда; во всякое время|at all times
imént|D|только что|just now
sokára|D|спустя долгое время|after a long while
immár|D|уже; отныне|by now
pillanatnyilag|D|в данный момент|at the moment
néhányszor|D|несколько раз|a few times
idei|A|нынешний|this year's
korábbi|A|прежний; более ранний|earlier
legutóbbi|A|последний|latest
akkori|A|тогдашний|of that time
eddigi|A|прежний; до сих пор|so far
utóbbi|A|последний (из двух)|latter
hétköznapi|A|будничный|everyday
mindennapi|A|повседневный|everyday
délutáni|A|дневной (послеполуденный)|afternoon
esti|A|вечерний|evening
éjszakai|A|ночной|night
éjjeli|A|ночной|night
hajnali|A|предрассветный|of dawn
vasárnapi|A|воскресный|Sunday
évi|A|годовой|annual
mostani|A|нынешний|present
jelenlegi|A|текущий|current
egykori|A|бывший|former
est|N|вечер|evening
nappal|N|день (светлое время)|daytime
kezdet|N|начало|beginning
végleg|D|окончательно|for good
örökre|D|навсегда|forever
jövőbeli|A|будущий|future
tervezett|A|запланированный|planned
"""),
    },
    {
        "code": "m12-u1-jellem-viselkedes",
        "tier": "intermediate", "month": 12, "ord": 718,
        "title_ru": "Характер и поведение",
        "title_en": "Character and behaviour",
        "goal_ru": "Описываю человека: характер, привычки, как он ведёт себя, и вежливо оцениваю его поступок.",
        "goal_en": "I describe a person: character, habits and manners, and politely judge what they did.",
        "situation_ru": "Рассказ о коллеге, характеристика, разговор о детях, отзыв о человеке.",
        "situation_en": "Telling about a colleague, a reference, a talk about children, a review of a person.",
        "topic_codes": "comparison,participles",
        "words": parse("""
személyiség|N|личность|personality
tehetség|N|талант|talent
képesség|N|способность|ability
okos|A|умный|clever
buta|A|глупый|silly, stupid
ügyes|A|ловкий; умелый|skilful
lusta|A|ленивый|lazy
komoly|A|серьёзный|serious
szerény|A|скромный|modest
makacs|A|упрямый|stubborn
figyelmes|A|внимательный|attentive
durva|A|грубый|rough, rude
rendes|A|порядочный; аккуратный|decent; tidy
becsületes|A|честный|honest
hazug|A|лживый|lying
viselkedik|V|вести себя|to behave
szokik|V|привыкать; иметь обыкновение|to be used to; to be wont
igyekszik|V|стараться|to try hard
tisztel|V|уважать|to respect
kegyetlen|A|жестокий|cruel
vad|A|дикий|wild
ravasz|A|хитрый|sly
naiv|A|наивный|naive
merész|A|смелый; дерзкий|bold
aljas|A|подлый|vile
meggondolatlan|A|опрометчивый|hasty
gyerekes|A|ребяческий|childish
szenvedélyes|A|страстный|passionate
heves|A|горячий; бурный|fiery
agresszív|A|агрессивный|aggressive
erőszakos|A|насильственный; настойчивый|violent, pushy
hajlamos|A|склонный|prone
tehetséges|A|талантливый|talented
zseniális|A|гениальный|brilliant
zseni|N|гений|genius
kiváló|A|превосходный|excellent
kitűnő|A|отличный|excellent
őszinteség|N|искренность|honesty
hajlandó|A|готовый|willing
képtelen|A|неспособный|unable
alkalmas|A|пригодный|suitable
méltó|A|достойный|worthy
érdemel|V|заслуживать|to deserve
megérdemel|V|заслужить|to deserve
elvisel|V|выносить; терпеть|to endure
ragaszkodik|V|привязываться; настаивать|to cling to, to insist
hozzáállás|N|отношение|attitude
szokatlan|A|необычный|unusual
szokásos|A|обычный|usual
szokott|A|привычный|usual
megszokott|A|привычный|accustomed
átlagos|A|средний; обычный|average
közönséges|A|обыкновенный; пошлый|ordinary, vulgar
kivételes|A|исключительный|exceptional
rendkívüli|A|чрезвычайный|extraordinary
speciális|A|специальный|special
"""),
    },
    {
        "code": "m12-u2-cselekves-mozgas",
        "tier": "intermediate", "month": 12, "ord": 719,
        "title_ru": "Действия и движение",
        "title_en": "Actions and movement",
        "goal_ru": "Точно описываю, что делаю руками и телом: взять, поднять, толкнуть, положить, собрать.",
        "goal_en": "I describe precisely what I do with hands and body: take, lift, push, put down, assemble.",
        "situation_ru": "Переезд, сборка шкафа, помощь соседу, инструкция «как сделать».",
        "situation_en": "Moving house, assembling a wardrobe, helping a neighbour, a \"how to\" guide.",
        "topic_codes": "preverbs,converb",
        "words": parse("""
elfog|V|поймать; захватить|to capture; to take
tol|V|толкать|to push
húz|V|тянуть|to pull
visz|V|нести; везти|to take
hoz|V|принести|to bring
ad|V|давать|to give
elvesz|V|взять; отобрать|to take away
megtart|V|сохранить; удержать|to keep
tör|V|ломать; разбивать|to break
szúr|V|колоть|to prick, to stab
üt|V|бить|to hit
leül|V|садиться|to sit down
rak|V|класть; складывать|to put, to stack
ás|V|копать|to dig
épít|V|строить|to build
épül|V|строиться|to be built
lóg|V|висеть|to hang
dől|V|клониться; валиться|to lean, to topple
fed|V|покрывать|to cover
fon|V|прясть; плести|to spin
emel|V|поднимать|to lift
indít|V|запускать; заводить|to start
helyez|V|помещать|to place
rögzít|V|закреплять|to fix
ront|V|портить|to spoil
hűt|V|охлаждать|to cool
gyújt|V|зажигать|to light
csuk|V|закрывать|to shut
nyújt|V|протягивать|to stretch, to offer
ken|V|мазать|to smear
rejt|V|прятать|to hide
rejtőzik|V|прятаться|to hide oneself
bújik|V|прятаться; залезать|to hide, to crawl
ráz|V|трясти|to shake
ránt|V|дёргать|to yank
dug|V|совать|to stick in
told|V|добавлять; двигать|to add, to push
mozdul|V|двигаться|to move
szalad|V|бежать; мчаться|to run
rohan|V|мчаться|to rush
forog|V|вращаться|to rotate
hajt|V|гнать; сгибать|to drive, to fold
hajlik|V|гнуться|to bend
dagad|V|опухать|to swell
terjed|V|распространяться|to spread
kitör|V|извергаться; вспыхивать|to erupt
felrobban|V|взорваться|to explode
ragad|V|прилипать; хватать|to stick, to snatch
akad|V|застрять; зацепиться|to get stuck
bukik|V|падать; терпеть неудачу|to fall
kelt|V|будить; вставать|to wake
nyílik|V|открываться|to open
ejt|V|ронять|to drop
mér|V|измерять|to measure
oszt|V|делить; раздавать|to divide, to deal
old|V|решать; развязывать|to solve, to untie
"""),
    },
    {
        "code": "m12-u3-kotoszavak-tovabb",
        "tier": "intermediate", "month": 12, "ord": 720,
        "title_ru": "Связки и оговорки: дальше",
        "title_en": "Connectors and hedges, continued",
        "goal_ru": "Связываю мысли в длинную фразу: уступка, причина, следствие, уточнение, оговорка.",
        "goal_en": "I tie thoughts into a long sentence: concession, cause, result, clarification, caveat.",
        "situation_ru": "Рассказ о случившемся, письмо с объяснением, деловой звонок, устный экзамен.",
        "situation_en": "Telling what happened, an explanatory email, a business call, an oral exam.",
        "topic_codes": "focus-word-order,register",
        "words": parse("""
emiatt|D|из-за этого|because of this
azonban|CCONJ|однако|however
illetve|CCONJ|или; а также|or rather; and
valamint|CCONJ|а также|as well as
habár|CCONJ|хотя|though
míg|D|пока; в то время как|while; whereas
mert|CCONJ|потому что|because
nehogy|CCONJ|чтобы не|lest
mintha|CCONJ|как будто|as if
amíg|D|пока|while, until
ahogy|CCONJ|как; по мере того как|as, the way
hacsak|CCONJ|разве что|unless
nemcsak|CCONJ|не только|not only
valójában|D|на самом деле|actually
lényegében|D|по существу|essentially
nyilvánvalóan|D|очевидно|evidently
könnyen|D|легко|easily
nehezen|D|с трудом|with difficulty
óvatosan|D|осторожно|carefully
erősen|D|сильно|strongly
mélyen|D|глубоко|deeply
szörnyen|D|ужасно|terribly
szigorúan|D|строго|strictly
hivatalosan|D|официально|officially
nagyszerűen|D|великолепно|splendidly
remélhetőleg|D|надеюсь|hopefully
bármikor|D|в любое время|at any time
bárhol|D|где угодно|anywhere
akárhogy|D|как угодно|however
kevésbé|D|менее|less
sokan|D|многие|many (people)
néhányan|D|некоторые|some (people)
hárman|D|втроём|the three of us
hányan|D|сколько человек|how many people
hányszor|D|сколько раз|how many times
meddig|D|до каких пор|until when
kifejezetten|D|специально|expressly
alaposan|D|основательно|thoroughly
egyszerre|D|одновременно|at once
túlságosan|D|слишком|too much
rendkívül|D|чрезвычайно|extremely
meglehetősen|D|довольно|fairly
amennyire|D|насколько|as far as
ahogyan|D|как|as, the way
ahelyett|D|вместо того|instead
anélkül|D|без этого|without that
mellesleg|D|между прочим|by the way
ilyenkor|D|в такое время|at such a time
másképp|D|иначе|differently
máshogy|D|по-другому|otherwise
amiatt|D|из-за того|because of that
ehelyett|D|вместо этого|instead of this
emellett|D|кроме того|besides
miszerint|D|согласно которому|according to which
világosan|D|ясно|clearly
eredetileg|D|первоначально|originally
"""),
    },
]
