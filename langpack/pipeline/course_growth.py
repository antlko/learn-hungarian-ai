"""Growth batch: about five hundred more words for the 24 basic-tier core units.

The month files carry each unit's hand-argued opening (30-40 words). This file
is the second layer: more of the high-frequency lemmas that the same errand
needs, appended to the unit AFTER its original words. The basic tier stays
24 units — the course ends (R11), see ADR-057 — and they get longer, not more numerous.

Rules this file keeps, each enforced by langpack/tests/test_course.py:

  * a word appears in exactly one core unit (course._dedup would silently drop
    a repeat; the test fails instead, so a unit cannot quietly shrink);
  * every lemma exists in the pack with the stated part of speech;
  * at least nine in ten course words have a real sentence in the starter pack,
    or enrolling a unit would teach a shorter version of itself;
  * glosses are hand-written. When unsure of one, the word is left out.

Syntax is documented in _wordlist.py. To add words: append lines to the unit's
block, keep the unit at 60 words or fewer, run `make refresh` and
`make starter` (see docs/LANGPACK.md, "Growing the course").
"""

from __future__ import annotations

from ._wordlist import parse

GROWTH: dict[str, list[tuple[str, str, str, str, str]]] = {
    "m1-u1-koszones": parse("""
üdvözöl|V|приветствовать|to greet
búcsúzik|V|прощаться|to say goodbye
találkozás|N|встреча|meeting, encounter
találkozó|N|встреча (назначенная)|appointment, meet-up
mindenki|PRON|все; каждый|everyone
valaki|PRON|кто-то|someone
senki|PRON|никто|nobody
valami|PRON|что-нибудь; что-то|something
minden|DET|всё; каждый|every; everything
ki|PRON|кто|who
nyelv|N|язык|language
ország|N|страна|country
város|N|город|city, town
falu|N|деревня|village
származik|V|происходить|to come from
éves|A|…летний|… years old
külföld|N|заграница|abroad
új|A|новый|new
régi|A|старый (давний)|old (long-standing)
kellemes|A|приятный|pleasant
szeret|V|любить; нравиться|to love; to like
válaszol|V|отвечать|to answer
mosolyog|V|улыбаться|to smile
nevet|V|смеяться|to laugh
"""),
    "m1-u2-alapszavak": parse("""
érthető|A|понятный|understandable
érdekel|V|интересовать|to interest
akar|V|хотеть|to want
szeretne|V|хотел бы|would like
próbál|V|пытаться|to try
megért|V|понять|to understand
kimond|V|произнести|to pronounce
szó|N|слово|word
mondat|N|предложение|sentence
betű|N|буква|letter
írás|N|письмо; запись|writing
kiejtés|N|произношение|pronunciation
fordít|V|переводить|to translate
gyorsan|D|быстро|quickly
hangosan|D|громко|loudly
halkan|D|тихо|quietly
egyszerű|A|простой|simple
fontos|A|важный|important
tényleg|D|правда; в самом деле|really
csak|D|только|only
még|D|ещё|still, yet
már|D|уже|already
"""),
    "m1-u3-szamok-penz-ido": parse("""
nulla|M|ноль|zero
tizenegy|M|одиннадцать|eleven
harminc|M|тридцать|thirty
negyven|M|сорок|forty
ötven|M|пятьдесят|fifty
millió|M|миллион|million
első|M|первый|first
második|M|второй|second
harmadik|M|третий|third
fillér|N|филлер (мелкая монета)|fillér (small coin)
érme|N|монета|coin
bankjegy|N|банкнота|banknote
aprópénz|N|мелочь|small change
összeg|N|сумма|sum
összesen|D|всего; итого|in total
délelőtt|N|первая половина дня|late morning
éjfél|N|полночь|midnight
hónap|N|месяц|month
év|N|год|year
mindig|D|всегда|always
soha|D|никогда|never
néha|D|иногда|sometimes
"""),
    "m1-u4-tegezes-magazas": parse("""
férfi|N|мужчина|man
nő|N|женщина|woman
ember|N|человек|person
fiú|N|мальчик; парень|boy
lány|N|девочка; девушка|girl
felnőtt|N|взрослый|adult
úriember|N|джентльмен|gentleman
kisasszony|N|барышня|miss
tisztelet|N|уважение|respect
szólít|V|обращаться; звать|to address; to call
enged|V|позволять; уступать|to allow; to yield
kíván|V|желать|to wish
személy|N|лицо; человек|person
hivatalnok|N|чиновник|clerk, official
köszönet|N|благодарность|thanks
kérés|N|просьба|request
szívesség|N|услуга; любезность|favour
bók|N|комплимент|compliment
illik|V|подобать; быть уместным|to be proper
bemutat|V|представить; показать|to introduce; to show
"""),
    "m2-u1-bolt": parse("""
bevásárol|V|делать покупки|to shop
árucikk|N|товар|item, product
áru|N|товар|goods
sütemény|N|выпечка|pastry
körte|N|груша|pear
narancs|N|апельсин|orange
citrom|N|лимон|lemon
banán|N|банан|banana
szőlő|N|виноград|grapes
répa|N|морковь|carrot
fokhagyma|N|чеснок|garlic
burgonya|N|картофель|potato
saláta|N|салат|salad
sonka|N|ветчина|ham
joghurt|N|йогурт|yoghurt
csoki|N|шоколад (разг.)|chocolate (informal)
tea|N|чай|tea
gyümölcslé|N|сок|juice
ásványvíz|N|минеральная вода|mineral water
fűszer|N|специя|spice
szappan|N|мыло|soap
papír|N|бумага|paper
szatyor|N|сумка-пакет|carrier bag
zsák|N|мешок|sack
"""),
    "m2-u2-utca": parse("""
közlekedés|N|транспорт; движение|traffic, transport
közlekedik|V|ходить (о транспорте)|to run (of transport)
kocsi|N|машина|car
autó|N|автомобиль|car
taxi|N|такси|taxi
bicikli|N|велосипед|bike
kerékpár|N|велосипед|bicycle
gyalogos|N|пешеход|pedestrian
járda|N|тротуар|pavement
zebra|N|пешеходный переход|zebra crossing
forgalom|N|движение (транспорта)|traffic
dugó|N|пробка|traffic jam
parkol|V|парковаться|to park
irány|N|направление|direction
tovább|D|дальше|further
átmegy|V|переходить|to cross
vezet|V|вести; водить|to lead; to drive
sétál|V|гулять|to walk
eltéved|V|заблудиться|to get lost
megkérdez|V|спросить|to ask
bejárat|N|вход|entrance
kijárat|N|выход|exit
"""),
    "m2-u3-lakas": parse("""
ház|N|дом|house
épület|N|здание|building
nappali|N|гостиная|living room
hálószoba|N|спальня|bedroom
kert|N|сад|garden
tető|N|крыша|roof
zuhany|N|душ|shower
kád|N|ванна|bathtub
vécé|N|туалет|toilet
fotel|N|кресло|armchair
kanapé|N|диван|sofa
szőnyeg|N|ковёр|carpet
függöny|N|занавеска|curtain
takaró|N|одеяло|blanket
tükör|N|зеркало|mirror
edény|N|посуда|dish, pot
mosogat|V|мыть посуду|to wash up
porszívó|N|пылесос|vacuum cleaner
szemét|N|мусор|rubbish
szemetes|N|мусорное ведро|bin
radiátor|N|батарея|radiator
"""),
    "m2-u4-mennyi": parse("""
rengeteg|A|огромное количество|loads of
néhány|DET|несколько|a few, some
mindegyik|PRON|каждый|each one
gramm|N|грамм|gram
kilogramm|N|килограмм|kilogram
méter|N|метр|metre
centiméter|N|сантиметр|centimetre
kilométer|N|километр|kilometre
szélesség|N|ширина|width
hosszúság|N|длина|length
magasság|N|высота; рост|height
vastag|A|толстый|thick
vékony|A|тонкий|thin
széles|A|широкий|wide
keskeny|A|узкий|narrow
mély|A|глубокий|deep
kicsit|D|немного|a little
túl|D|слишком|too
majdnem|D|почти|almost
körülbelül|D|приблизительно|approximately
átlag|N|среднее|average
mennyiség|N|количество|quantity
különböző|A|разный|different
egyforma|A|одинаковый|identical
"""),
    "m3-u1-ugyintezes": parse("""
kapu|N|ворота|gate
folyosó|N|коридор|corridor
terem|N|зал; помещение|hall, room
ellenőrzés|N|проверка|check, inspection
hiányos|A|неполный|incomplete
töröl|V|стереть; отменить|to delete; to cancel
elutasít|V|отклонить|to reject
szükséges|A|необходимый|necessary
engedélyez|V|разрешить|to permit
intézet|N|институт; учреждение|institute
igény|N|требование; потребность|claim, need
bizottság|N|комиссия|committee
szolgálat|N|служба|service
kifogás|N|возражение; претензия|objection
részletes|A|подробный|detailed
alapos|A|основательный|thorough
megfelelő|A|подходящий|suitable
kötelez|V|обязывать|to oblige
határoz|V|решать; определять|to decide; to determine
támogat|V|поддерживать|to support
tartózkodik|V|находиться; пребывать|to stay, to be present
kapcsolat|N|связь; отношения|connection, relationship
"""),
    "m3-u2-orvos": parse("""
orvosi|A|медицинский|medical
egészség|N|здоровье|health
egészséges|A|здоровый|healthy
fej|N|голова|head
szem|N|глаз|eye
fog|N|зуб|tooth
kéz|N|рука (кисть)|hand
láb|N|нога|leg, foot
szív|N|сердце|heart
alszik|V|спать|to sleep
pihen|V|отдыхать|to rest
gyógyul|V|выздоравливать|to recover
szed|V|принимать (лекарство)|to take (medicine)
vizsgál|V|осматривать; исследовать|to examine
fertőzés|N|инфекция|infection
műtét|N|операция|operation
röntgen|N|рентген|X-ray
tünet|N|симптом|symptom
egészségügyi|A|медицинский; санитарный|health-care
páciens|N|пациент|patient
sebész|N|хирург|surgeon
"""),
    "m3-u3-bank": parse("""
pénzügy|N|финансы|finances
pénztárca|N|кошелёк|wallet
spórol|V|экономить; копить|to save
költ|V|тратить|to spend
tartozik|V|быть должным|to owe
nyugdíj|N|пенсия|pension
vagyon|N|имущество; богатство|property; wealth
gazdag|A|богатый|rich
szegény|A|бедный|poor
dolgozó|N|работающий|worker
fedezet|N|покрытие; обеспечение|cover, security
internetes|A|интернет-|internet
pénznem|N|валюта|currency
hitelkártya|N|кредитная карта|credit card
adósság|N|долг|debt
veszteség|N|потеря; убыток|loss
haszon|N|выгода; прибыль|profit, benefit
megfizet|V|оплатить|to pay in full
pénzügyi|A|финансовый|financial
tartalék|N|запас; резерв|reserve
csalás|N|мошенничество|fraud
gazdasági|A|экономический|economic
részvény|N|акция|share
"""),
    "m3-u4-posta": parse("""
képeslap|N|открытка|postcard
légi|A|воздушный|air
tengeri|A|морской|sea
gyors|A|быстрый|fast
lassú|A|медленный|slow
ragaszt|V|клеить|to stick
tartalom|N|содержимое|contents
számít|V|считаться; рассчитывать|to count; to expect
várakozik|V|ждать|to wait
tartalmaz|V|содержать|to contain
kiadás|N|расход; издание|expense; edition
jelzés|N|сигнал|signal
láda|N|ящик|crate
raktár|N|склад|warehouse
teherautó|N|грузовик|lorry
kamion|N|фура|lorry (large)
szállít|V|перевозить|to transport
útközben|D|по дороге|on the way
megígér|V|пообещать|to promise
megáll|V|остановиться|to stop
"""),
    "m4-u1-munka": parse("""
munkás|N|рабочий|worker
alkalmazott|N|сотрудник|employee
csapat|N|команда|team
vállalat|N|компания|company
gyár|N|завод|factory
üzem|N|предприятие; цех|plant, workshop
üzlet|N|дело; магазин|business; shop
ügyvéd|N|адвокат|lawyer
mérnök|N|инженер|engineer
szakács|N|повар|cook
sofőr|N|водитель|driver
ápoló|N|санитар; сиделка|nurse, carer
titkár|N|секретарь|secretary
programozó|N|программист|programmer
kereset|N|заработок|earnings
prémium|N|премия|bonus
szabadnap|N|выходной|day off
nyugdíjas|N|пенсионер|pensioner
képzés|N|обучение|training
tanfolyam|N|курсы|course
tervez|V|планировать|to plan
"""),
    "m4-u2-alberlet": parse("""
tulajdon|N|собственность|property
tiszta|A|чистый|clean
koszos|A|грязный|dirty
csendes|A|тихий|quiet
központ|N|центр|centre
környék|N|район; окрестность|neighbourhood
otthon|N|дом (родной)|home
környezet|N|окружение; окружающая среда|surroundings
kilátás|N|вид (из окна)|view
piszkos|A|грязный|dirty
vidék|N|провинция; сельская местность|countryside
belváros|N|центр города|city centre
használat|N|использование|use
rendelkezik|V|располагать|to have at one's disposal
megegyezik|V|договориться|to agree
csend|N|тишина|silence
hely|N|место|place
központi|A|центральный|central
ideiglenes|A|временный|temporary
végleges|A|окончательный|final
bérelt|A|арендованный|rented
"""),
    "m4-u3-csalad": parse("""
nagyszülő|N|дедушка или бабушка|grandparent
nagyanya|N|бабушка|grandmother
nagypapa|N|дедушка|grandfather
nagybácsi|N|дядя|uncle
nagynéni|N|тётя|aunt
unokatestvér|N|двоюродный брат; сестра|cousin
após|N|тесть; свёкор|father-in-law
anyós|N|тёща; свекровь|mother-in-law
sógor|N|шурин; зять|brother-in-law
baba|N|малыш; кукла|baby; doll
öreg|A|старый (о человеке)|old (of a person)
felnő|V|вырасти|to grow up
szül|V|рожать|to give birth
esküvő|N|свадьба|wedding
házasság|N|брак|marriage
szerelem|N|любовь (чувство)|love
szeretet|N|любовь; привязанность|affection
szerelmes|A|влюблённый|in love
élet|N|жизнь|life
születésnap|N|день рождения|birthday
vezetéknév|N|фамилия|surname
"""),
    "m4-u4-multido": parse("""
múltkor|D|в прошлый раз|last time
tavalyi|A|прошлогодний|last year's
elmúlt|A|прошедший|past
történet|N|история; рассказ|story
esemény|N|событие|event
baleset|N|несчастный случай|accident
előfordul|V|случаться|to occur
megtörténik|V|произойти|to happen
meghal|V|умереть|to die
született|A|родившийся|born
iskolás|N|школьник|schoolchild
nyár|N|лето|summer
tél|N|зима|winter
tavasz|N|весна|spring
ősz|N|осень|autumn
vakáció|N|каникулы|holiday
megszületik|V|родиться|to be born
érettségi|N|выпускной экзамен|school-leaving exam
egyetem|N|университет|university
elmegy|V|уйти; уехать|to leave
megtud|V|узнать|to find out
élmény|N|впечатление; переживание|experience
kaland|N|приключение|adventure
"""),
    "m5-u1-elromlott": parse("""
leáll|V|остановиться|to stop
kinyit|V|открыть|to open
becsuk|V|закрыть (дверь, глаза)|to shut
kábel|N|кабель|cable
csatlakozik|V|подключаться|to connect
telepít|V|устанавливать|to install
alkatrész|N|запчасть|spare part
csavar|N|винт; болт|screw, bolt
szerszám|N|инструмент|tool
motor|N|мотор; двигатель|engine
füst|N|дым|smoke
tűz|N|огонь; пожар|fire
láng|N|пламя|flame
szag|N|запах|smell
hangos|A|громкий|loud
csökken|V|уменьшаться|to decrease
romlik|V|ухудшаться|to deteriorate
elveszt|V|терять|to lose
rendszer|N|система|system
szerkezet|N|механизм; конструкция|mechanism
eszköz|N|устройство; средство|device, means
veszély|N|опасность|danger
vigyáz|V|быть осторожным|to be careful
óvatos|A|осторожный|cautious
robbanás|N|взрыв|explosion
"""),
    "m5-u2-idopont": parse("""
telefon|N|телефон|telephone
mobil|N|мобильный|mobile
hívás|N|звонок|call
fogad|V|принимать|to receive
beszélget|V|беседовать|to chat
hallgat|V|слушать|to listen
megbeszél|V|обсудить; договориться|to discuss, to agree
elérhető|A|доступный|available
korán|D|рано|early
későn|D|поздно|late
pont|N|точка; ровно|point; exactly
számítógép|N|компьютер|computer
internet|N|интернет|internet
oldal|N|страница; сторона|page; side
fájl|N|файл|file
ismét|D|снова|again
előző|A|предыдущий|previous
utolsó|A|последний|last
azonnali|A|немедленный|immediate
tévé|N|телевизор|TV
rádió|N|радио|radio
videó|N|видео|video
"""),
    "m5-u3-ovoda-hivatal": parse("""
tanterem|N|класс|classroom
tanév|N|учебный год|school year
félév|N|семестр; полугодие|term
tantárgy|N|предмет|subject
tankönyv|N|учебник|textbook
füzet|N|тетрадь|exercise book
toll|N|ручка|pen
ceruza|N|карандаш|pencil
táska|N|сумка; портфель|bag
játszótér|N|детская площадка|playground
játék|N|игрушка; игра|toy; game
játszik|V|играть|to play
rajzol|V|рисовать|to draw
énekel|V|петь|to sing
ebédel|V|обедать|to have lunch
óvónő|N|воспитательница|kindergarten teacher
tanítónő|N|учительница|teacher (primary)
osztálytárs|N|одноклассник|classmate
iskolai|A|школьный|school
tanulmány|N|исследование; учёба|study
kutatás|N|исследование|research
főiskola|N|колледж; институт|college
középiskola|N|средняя школа|secondary school
tanulás|N|учение; учёба|learning
"""),
    "m5-u4-etterem": parse("""
foglalás|N|бронь|reservation
menü|N|меню; комплексный обед|set menu
sertés|N|свинина|pork
marhahús|N|говядина|beef
torta|N|торт|cake
fagylalt|N|мороженое|ice cream
pálinka|N|палинка|fruit brandy
bors|N|перец|pepper
szendvics|N|бутерброд|sandwich
pizza|N|пицца|pizza
hamburger|N|гамбургер|hamburger
édesség|N|сладость|sweet
cukorka|N|конфета|sweet, candy
vodka|N|водка|vodka
alkohol|N|алкоголь|alcohol
csésze|N|чашка|cup
ennivaló|N|еда|food
evés|N|еда (процесс)|eating
élelem|N|пища; провизия|food, provisions
megeszik|V|съесть|to eat up
pezsgő|N|шампанское|sparkling wine
"""),
    "m6-u1-velemeny": parse("""
gondolkodik|V|думать|to think
gondolat|N|мысль|thought
elképzel|V|представить|to imagine
kérdéses|A|спорный|questionable
biztos|A|уверенный; надёжный|sure; certain
bizonytalan|A|неуверенный|uncertain
lehetséges|A|возможный|possible
lehetetlen|A|невозможный|impossible
nyilvánvaló|A|очевидный|obvious
érdemes|A|стоящий|worthwhile
hasznos|A|полезный|useful
felesleges|A|лишний|unnecessary
helytelen|A|неправильный|wrong
igazságos|A|справедливый|fair
tisztességes|A|честный; порядочный|decent
őszinte|A|искренний|sincere
beismer|V|признавать|to admit
hasonló|A|похожий|similar
eltérő|A|отличающийся|different
ellentét|N|противоположность|contrast
egyetértés|N|согласие|agreement
logikus|A|логичный|logical
ésszerű|A|разумный|reasonable
elképzelés|N|представление; замысел|idea, notion
következtetés|N|вывод|conclusion
ellenkező|A|противоположный|opposite
mindenképpen|D|в любом случае|in any case
valószínű|A|вероятный|probable
elmélet|N|теория|theory
"""),
    "m6-u2-erzesek": parse("""
szomorúság|N|грусть|sadness
boldogság|N|счастье|happiness
bánat|N|печаль|sorrow
harag|N|гнев|anger
sír|V|плакать|to cry
vidám|A|весёлый|cheerful
félénk|A|робкий|shy
bátor|A|храбрый|brave
gyáva|A|трусливый|cowardly
izgatott|A|взволнованный|excited
nyugtalan|A|беспокойный|restless
elégedetlen|A|недовольный|dissatisfied
megijed|V|испугаться|to get scared
ölel|V|обнимать|to hug
vigasztal|V|утешать|to console
haragszik|V|сердиться|to be angry
gyűlöl|V|ненавидеть|to hate
érzelem|N|эмоция|emotion
szenvedély|N|страсть|passion
izgalom|N|волнение|excitement
szenved|V|страдать|to suffer
düh|N|ярость|rage
büszkeség|N|гордость|pride
"""),
    "m6-u3-fogalmak": parse("""
halál|N|смерть|death
béke|N|мир|peace
háború|N|война|war
hazugság|N|ложь|lie
hit|N|вера|faith
bizalom|N|доверие|trust
barátság|N|дружба|friendship
társadalom|N|общество|society
közösség|N|сообщество|community
állam|N|государство|state
kormány|N|правительство|government
kötelesség|N|обязанность|duty
tudomány|N|наука|science
művészet|N|искусство|art
kultúra|N|культура|culture
történelem|N|история|history
jövő|N|будущее|the future
természet|N|природа|nature
biztonság|N|безопасность|safety
civilizáció|N|цивилизация|civilisation
vallás|N|религия|religion
hagyomány|N|традиция|tradition
gazdaság|N|экономика; хозяйство|economy
politika|N|политика|politics
jelenség|N|явление|phenomenon
folyamat|N|процесс|process
fejlődés|N|развитие|development
"""),
    "m6-u4-kotoszavak": parse("""
és|CCONJ|и|and
vagy|CCONJ|или|or
de|CCONJ|но|but
akkor|D|тогда|then
így|D|так|thus
azért|D|поэтому; всё же|therefore; still
mindenesetre|D|во всяком случае|anyway
szinte|D|почти|almost
főleg|D|особенно|especially
különösen|D|особенно|particularly
egyszerűen|D|просто|simply
ugyanakkor|D|в то же время|at the same time
ugyanúgy|D|точно так же|just the same
eleinte|D|сначала; поначалу|at first
elsősorban|D|прежде всего|primarily
hiába|D|напрасно|in vain
leginkább|D|больше всего|most of all
általában|D|обычно; вообще|generally
gyakorlatilag|D|практически|practically
mindössze|D|всего лишь|merely
egyelőre|D|пока; на данный момент|for now
amint|D|как только|as soon as
csupán|D|лишь|merely
mindjárt|D|сейчас; вот-вот|right away
"""),
}

