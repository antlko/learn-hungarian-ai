"""Advanced tier, months 13-18: twenty core units for the learner who has finished the
intermediate tier and wants the vocabulary of newspapers, offices, courts and novels.

Words come from the pack's frequency list (roughly ranks 3 500 to 8 000, the B1-B2
band) and are disjoint from every other tier and from the theme units. Where a theme
has an intermediate unit, the advanced unit takes its rarer words ("in depth"). The tier
is finite like the others: after the last unit the course hands over (ADR-057).

`month` is an internal ordering and checkpoint-placement field the learner never sees.
Glosses are hand-written; a word whose gloss was uncertain was left out, not guessed.
"""

from __future__ import annotations

from ._wordlist import parse

UNITS = [
    {
        "code": "m13-u1-uzlet-gazdasag",
        "tier": "advanced", "month": 13, "ord": 801,
        "title_ru": "Бизнес и экономика",
        "title_en": "Business and the economy",
        "goal_ru": "Читаю деловую новость, понимаю отчёт о бюджете и обсуждаю рынок, производство и рост.",
        "goal_en": "I read a business story, follow a budget report and discuss markets, production and growth.",
        "situation_ru": "Деловая газета, совещание о бюджете, профсоюз и забастовка, запуск продукта.",
        "situation_en": "A business paper, a budget meeting, a union and a strike, a product launch.",
        "topic_codes": "register,participles",
        "words": parse("""
tőzsde|N|биржа|stock market
tőke|N|капитал|capital
költségvetés|N|бюджет|budget
költségvetési|A|бюджетный|budgetary
nyereség|N|прибыль|profit
kliens|N|клиент|client
ingatlanügynök|N|риелтор|estate agent
fizetésemelés|N|повышение зарплаты|pay rise
szakszervezet|N|профсоюз|trade union
sztrájk|N|забастовка|strike
kereskedelem|N|торговля|trade
bevásárlóközpont|N|торговый центр|shopping mall
hatáskör|N|компетенция|remit, competence
vezetőség|N|руководство|management
hivatás|N|призвание; профессия|vocation, profession
hivatásos|A|профессиональный|professional
hivatali|A|служебный; официальный|official, office
kérvény|N|прошение|petition, application
rendelkezés|N|распоряжение; положение|provision, order
rendelet|N|постановление|decree
erőforrás|N|ресурс|resource
termel|V|производить|to produce
termés|N|урожай|crop
gyárt|V|изготавливать|to manufacture
ipari|A|промышленный|industrial
ipar|N|промышленность|industry
minőség|N|качество|quality
minőségi|A|качественный|high-quality
szoftver|N|программное обеспечение|software
weboldal|N|веб-страница|web page
merevlemez|N|жёсткий диск|hard drive
mobiltelefon|N|мобильный телефон|mobile phone
televízió|N|телевидение|television
növekedés|N|рост|growth
növel|V|увеличивать|to increase
fejleszt|V|развивать|to develop
fejlődik|V|развиваться|to develop
fejlemény|N|ход событий; развитие|development
haladás|N|прогресс|progress
megvalósít|V|осуществлять|to carry out
megállapít|V|устанавливать; констатировать|to establish, to find
fokozat|N|степень|degree
ütem|N|темп|pace
kompromisszum|N|компромисс|compromise
"""),
    },
    {
        "code": "m13-u2-orvostudomany-reszletek",
        "tier": "advanced", "month": 13, "ord": 802,
        "title_ru": "Медицина: подробности",
        "title_en": "Medicine: the details",
        "goal_ru": "Понимаю выписку, назначение и статью о лечении; называю осложнение, анализ, процедуру.",
        "goal_en": "I follow a discharge note, a prescription and an article on treatment, and name a complication, a test, a procedure.",
        "situation_ru": "Выписка из больницы, разговор с хирургом, аптека, статья о здоровье.",
        "situation_en": "A hospital discharge, a talk with a surgeon, the pharmacy, a health article.",
        "topic_codes": "causative,participles",
        "words": parse("""
sebészet|N|хирургия|surgery
operáció|N|операция|operation
agyvérzés|N|инсульт|stroke
infúzió|N|капельница|drip, infusion
szérum|N|сыворотка|serum
gyógyulás|N|выздоровление|recovery
kezelhető|A|излечимый|treatable
komplikáció|N|осложнение|complication
vitamin|N|витамин|vitamin
kalória|N|калория|calorie
diéta|N|диета|diet
aszpirin|N|аспирин|aspirin
vércsoport|N|группа крови|blood type
elsősegély|N|первая помощь|first aid
állatorvos|N|ветеринар|vet
ártalmas|A|вредный|harmful
káros|A|вредный|harmful
mérgező|A|ядовитый|toxic
kiütés|N|сыпь|rash
viszket|V|чесаться|to itch
csípő|N|бедро|hip
ököl|N|кулак|fist
hasi|A|брюшной|abdominal
anyai|A|материнский|maternal
újszülött|A|новорождённый|newborn
várandós|A|беременная|expectant
sav|N|кислота|acid
vegyszer|N|химикат|chemical
szennyezett|A|загрязнённый|contaminated
környezetvédelmi|A|природоохранный|environmental
kiegészítés|N|дополнение|supplement
hízik|V|полнеть|to put on weight
étrend|N|рацион; диета|diet
táplálkozás|N|питание|nutrition
"""),
    },
    {
        "code": "m13-u3-politika-allam",
        "tier": "advanced", "month": 13, "ord": 803,
        "title_ru": "Политика и государство",
        "title_en": "Politics and the state",
        "goal_ru": "Читаю политическую новость и понимаю, кто что решает: президент, парламент, министерство, партия.",
        "goal_en": "I read a political story and see who decides what: president, parliament, ministry, party.",
        "situation_ru": "Выпуск новостей, выборы, заседание парламента, дискуссия за столом.",
        "situation_en": "A news bulletin, an election, a parliament sitting, a discussion at the table.",
        "topic_codes": "register,focus-word-order",
        "words": parse("""
alkotmány|N|конституция|constitution
köztársaság|N|республика|republic
demokrácia|N|демократия|democracy
demokratikus|A|демократический|democratic
politikus|N|политик|politician
politikai|A|политический|political
párt|N|партия|party
szavaz|V|голосовать|to vote
szavazat|N|голос (на выборах)|vote
miniszterelnök|N|премьер-министр|prime minister
külügyminiszter|N|министр иностранных дел|foreign minister
minisztérium|N|министерство|ministry
nagykövet|N|посол|ambassador
nagykövetség|N|посольство|embassy
diplomata|N|дипломат|diplomat
diplomáciai|A|дипломатический|diplomatic
szenátus|N|сенат|senate
szenátor|N|сенатор|senator
kongresszus|N|конгресс|congress
alelnök|N|вице-президент|vice-president
uralkodik|V|править|to reign
uralkodó|N|правитель|ruler
uralom|N|господство|rule
császár|N|император|emperor
trón|N|трон|throne
korona|N|корона|crown
királyság|N|королевство|kingdom
herceg|N|принц; князь|prince
hercegnő|N|принцесса|princess
báró|N|барон|baron
nemes|A|благородный|noble
birodalom|N|империя|empire
tartomány|N|провинция|province
régió|N|регион|region
övezet|N|зона|zone
nép|N|народ|people
nemzet|N|нация|nation
hazafi|N|патриот|patriot
korrupció|N|коррупция|corruption
korrupt|A|коррумпированный|corrupt
egyezmény|N|конвенция; соглашение|convention, treaty
közlemény|N|коммюнике; сообщение|communiqué
kampány|N|кампания|campaign
forradalom|N|революция|revolution
lázadás|N|бунт|rebellion
folyóirat|N|журнал (периодика)|periodical
szenzáció|N|сенсация|sensation
"""),
    },
    {
        "code": "m13-u4-haboru-biztonsag",
        "tier": "advanced", "month": 13, "ord": 804,
        "title_ru": "Конфликт, армия и безопасность",
        "title_en": "Conflict, the army and security",
        "goal_ru": "Понимаю новость о конфликте, армии и безопасности и называю оружие, звания и последствия.",
        "goal_en": "I follow a news story on conflict, the army and security and name weapons, ranks and consequences.",
        "situation_ru": "Военная хроника, исторический документальный фильм, разговор о службе в армии.",
        "situation_en": "A war report, a history documentary, a talk about army service.",
        "topic_codes": "register,participles",
        "words": parse("""
hadsereg|N|армия|army
sereg|N|войско|host, army
katonai|A|военный|military
tábornok|N|генерал|general
őrmester|N|сержант|sergeant
százados|N|капитан (армейский)|captain
őrnagy|N|майор|major
tiszt|N|офицер|officer
harcos|N|воин|warrior
csata|N|сражение|battle
háborús|A|военный (о войне)|war
fegyveres|A|вооружённый|armed
pisztoly|N|пистолет|pistol
puska|N|ружьё|rifle
kard|N|меч|sword
lövés|N|выстрел|shot
lövedék|N|снаряд; пуля|projectile, bullet
bomba|N|бомба|bomb
atombomba|N|атомная бомба|atom bomb
ágyú|N|пушка|cannon
tank|N|танк|tank
flotta|N|флот|fleet
tengeralattjáró|N|подводная лодка|submarine
haditengerészet|N|военно-морской флот|navy
légierő|N|военно-воздушные силы|air force
szövetség|N|союз; альянс|alliance
szövetséges|A|союзный|allied
szövetségi|A|федеральный|federal
terrorista|N|террорист|terrorist
túsz|N|заложник|hostage
fogság|N|плен|captivity
menekülés|N|бегство|escape, flight
védelem|N|защита|defence
véd|V|защищать|to protect
védett|A|защищённый|protected
védelmi|A|оборонительный|defensive
támadás|N|нападение|attack
biztonsági|A|охранный; безопасный|security
testőr|N|телохранитель|bodyguard
őr|N|охранник|guard
őrség|N|караул|guard, sentry
riadó|N|тревога|alarm
riasztás|N|сигнал тревоги|alert
katasztrófa|N|катастрофа|catastrophe
szerencsétlenség|N|несчастье; авария|misfortune, accident
tilalom|N|запрет|prohibition
"""),
    },
    {
        "code": "m14-u1-fold-vilag",
        "tier": "advanced", "month": 14, "ord": 805,
        "title_ru": "Страны, регионы и мир",
        "title_en": "Countries, regions and the world",
        "goal_ru": "Называю страны, народы и регионы, читаю о географии и путешествиях подробнее.",
        "goal_en": "I name countries, peoples and regions and read about geography and travel in more detail.",
        "situation_ru": "Рассказ о путешествии, урок географии, новость из-за границы.",
        "situation_en": "A travel story, a geography lesson, a story from abroad.",
        "topic_codes": "remaining-cases,register",
        "words": parse("""
brazil|A|бразильский|Brazilian
norvég|A|норвежский|Norwegian
vietnámi|A|вьетнамский|Vietnamese
iszlám|A|исламский|Islamic
buddhista|A|буддийский|Buddhist
kisváros|N|небольшой город|small town
szomszédos|A|соседний|neighbouring
sarki|A|полярный|polar
trópusi|A|тропический|tropical
ókori|A|античный; древний|ancient
középkori|A|средневековый|medieval
vasút|N|железная дорога|railway
vasúti|A|железнодорожный|railway
országút|N|шоссе|highway
táv|N|расстояние; дистанция|distance
telep|N|посёлок; площадка|settlement, site
kolónia|N|колония|colony
vulkán|N|вулкан|volcano
piramis|N|пирамида|pyramid
tájékozódik|V|ориентироваться|to find one's bearings
útkereszteződés|N|перекрёсток|junction
lakótelep|N|жилой массив|housing estate
sétány|N|променад|promenade
emlékmű|N|памятник|monument
forgalmas|A|оживлённый|busy
külváros|N|пригород|suburb
felújítás|N|ремонт; обновление|renovation
alul|D|внизу|below
"""),
    },
    {
        "code": "m14-u2-penz-vagyon",
        "tier": "advanced", "month": 14, "ord": 806,
        "title_ru": "Деньги, собственность, наследство",
        "title_en": "Money, property and inheritance",
        "goal_ru": "Понимаю разговор о наследстве, долге и состоянии и называю то, что стоит за этими словами.",
        "goal_en": "I follow talk of inheritance, debt and wealth and name what stands behind those words.",
        "situation_ru": "Завещание, долг в семье, покупка недвижимости, разговор о бедности и достатке.",
        "situation_en": "A will, a family debt, buying property, a talk about poverty and comfort.",
        "topic_codes": "register,participles",
        "words": parse("""
adós|N|должник|debtor
fizetség|N|плата|payment
jólét|N|благосостояние|prosperity
gazdagság|N|богатство|wealth
szegénység|N|бедность|poverty
koldus|N|нищий|beggar
végrendelet|N|завещание|will
örökség|N|наследство|inheritance
örökös|N|наследник|heir
örököl|V|наследовать|to inherit
telek|N|участок земли|plot of land
birtok|N|владение; имение|estate, possession
felbecsülhetetlen|A|бесценный|priceless
kedvező|A|благоприятный; выгодный|favourable
értéktelen|A|ничего не стоящий|worthless
haszontalan|A|бесполезный|useless
használhatatlan|A|непригодный|unusable
használható|A|пригодный|usable
felhasznál|V|использовать|to use up
banki|A|банковский|bank
jótékony|A|благотворительный|charitable
jótékonysági|A|благотворительный|charity
"""),
    },
    {
        "code": "m14-u3-csalad-unnepek",
        "tier": "advanced", "month": 14, "ord": 807,
        "title_ru": "Семья, обряды и события жизни",
        "title_en": "Family, ceremonies and life events",
        "goal_ru": "Рассказываю о семье и о важных событиях: свадьба, рождение, похороны, праздник.",
        "goal_en": "I talk about family and the big events: a wedding, a birth, a funeral, a feast.",
        "situation_ru": "Семейный праздник, рассказ о родне, соболезнования, приглашение на свадьбу.",
        "situation_en": "A family gathering, talk about relatives, condolences, a wedding invitation.",
        "topic_codes": "register,possessive",
        "words": parse("""
házaspár|N|супружеская пара|married couple
házigazda|N|хозяин (приёма)|host
háziasszony|N|хозяйка|hostess, housewife
agglegény|N|холостяк|bachelor
hajadon|A|незамужняя|unmarried
jegyes|N|жених; невеста|betrothed
sógornő|N|невестка; золовка|sister-in-law
unokaöcs|N|племянник|nephew
unokahúg|N|племянница|niece
fivér|N|брат|brother
öregember|N|старик|old man
öregasszony|N|старуха|old woman
fiatalság|N|молодость|youth
kisfiú|N|мальчик|little boy
kisgyerek|N|малыш|small child
kisbaba|N|младенец|baby
édesanya|N|мать|mother
édesapa|N|отец|father
nagyi|N|бабушка (разг.)|grandma
papa|N|папа; дедушка (разг.)|dad; grandpa
ceremónia|N|церемония|ceremony
lakoma|N|пир|banquet
gyász|N|траур|mourning
koporsó|N|гроб|coffin
iker|N|близнец|twin
nászút|N|медовый месяц|honeymoon
temető|N|кладбище|cemetery
temet|V|хоронить|to bury
keresztelő|N|крестины|christening
"""),
    },
    {
        "code": "m15-u1-gondolkodas-elemzes",
        "tier": "advanced", "month": 15, "ord": 808,
        "title_ru": "Рассуждение и анализ",
        "title_en": "Reasoning and analysis",
        "goal_ru": "Оцениваю довод, различаю вероятное и невероятное и аккуратно формулирую вывод.",
        "goal_en": "I weigh an argument, tell the likely from the unlikely and word a conclusion carefully.",
        "situation_ru": "Рецензия, учебная дискуссия, разбор статьи, письменный ответ на экзамене.",
        "situation_en": "A review, a seminar discussion, analysing an article, a written exam answer.",
        "topic_codes": "register,conditional",
        "words": parse("""
hihető|A|правдоподобный|believable
elképzelhetetlen|A|невообразимый|unimaginable
valószínűtlen|A|маловероятный|implausible
kétséges|A|сомнительный|doubtful
megoldatlan|A|нерешённый|unsolved
párhuzamos|A|параллельный|parallel
ellentétes|A|противоположный|opposite, contrary
elméleti|A|теоретический|theoretical
gyakorlati|A|практический|practical
praktikus|A|практичный|practical
hangsúly|N|ударение; акцент|stress, emphasis
nézőpont|N|точка зрения|point of view
jelkép|N|символ|symbol
idézet|N|цитата|quotation
idéz|V|цитировать|to quote
ige|N|глагол|verb
intelligencia|N|интеллект|intelligence
filozófia|N|философия|philosophy
logika|N|логика|logic
irónia|N|ирония|irony
érthetően|D|понятно|clearly
végeredmény|N|итог|final result
viszonylag|D|относительно|relatively
mindenekelőtt|D|прежде всего|above all
csaknem|D|почти|almost
nagyrészt|D|в основном|mostly
feltehetőleg|D|предположительно|presumably
valószínűség|N|вероятность|probability
lehetőleg|D|по возможности|if possible
esetleges|A|возможный; случайный|possible, occasional
kritika|N|критика|criticism
kritizál|V|критиковать|to criticise
ellenvetés|N|возражение|objection
ellenez|V|возражать|to oppose
ellenérv|N|контраргумент|counter-argument
képvisel|V|представлять|to represent
befolyásol|V|влиять|to influence
befolyás|N|влияние|influence
megkülönböztet|V|различать|to distinguish
vegyes|A|смешанный|mixed
összefüggés|N|взаимосвязь|connection
tényező|N|фактор|factor
voltaképpen|D|в сущности|in fact
mihelyt|CCONJ|как только|as soon as
másrészt|D|с другой стороны|on the other hand
mindaddig|D|до тех пор|until then
semmiképpen|D|ни в коем случае|by no means
noha|CCONJ|хотя|although
amellett|D|к тому же|besides
valahányszor|D|всякий раз, когда|whenever
röviden|D|коротко|briefly
mindvégig|D|до самого конца|all along
pontosabban|D|точнее|more precisely
mialatt|D|пока|while
amennyiben|CCONJ|если; в случае|if, provided that
"""),
    },
    {
        "code": "m15-u2-jog-igazsag",
        "tier": "advanced", "month": 15, "ord": 809,
        "title_ru": "Право и правосудие: подробнее",
        "title_en": "Law and justice in detail",
        "goal_ru": "Читаю заметку из зала суда и бытовой документ, понимаю, кто обвиняет, защищает и решает.",
        "goal_en": "I read a court report and an everyday legal document and see who accuses, defends and decides.",
        "situation_ru": "Судебная хроника, объяснение с адвокатом, жалоба в инстанцию.",
        "situation_en": "A court report, an explanation from a lawyer, a complaint to an authority.",
        "topic_codes": "register,participles",
        "words": parse("""
halálbüntetés|N|смертная казнь|capital punishment
életfogytiglan|N|пожизненное заключение|life sentence
szabálytalan|A|неправильный; нарушающий правила|irregular
tisztességtelen|A|нечестный|dishonest
tisztesség|N|порядочность|decency
korlátoz|V|ограничивать|to restrict
kényszer|N|принуждение|compulsion
beleegyezés|N|согласие|consent
betörő|N|взломщик|burglar
csempész|N|контрабандист|smuggler
tiltakozás|N|протест|protest
indítvány|N|ходатайство; предложение|motion, proposal
megkülönböztetés|N|дискриминация|discrimination
önkormányzat|N|самоуправление|local government
kötelezettség|N|обязательство|obligation
egyenlőség|N|равенство|equality
jogosult|A|имеющий право|entitled
jogász|N|юрист|lawyer
parlament|N|парламент|parliament
megengedett|A|разрешённый|permitted
bevándorló|N|иммигрант|immigrant
szabályzat|N|регламент|regulations
rabol|V|грабить|to rob
állampolgár|N|гражданин|citizen
nyilvántartás|N|реестр|register
bűntett|N|тяжкое преступление|felony
védő|N|защитник|defence counsel
büntet|V|наказывать|to punish
bűnözés|N|преступность|crime
vádol|V|обвинять|to accuse
ügyészség|N|прокуратура|prosecutor's office
igazságszolgáltatás|N|правосудие|justice system
üldöz|V|преследовать|to pursue
önvédelem|N|самооборона|self-defence
szélhámos|N|аферист|swindler
legális|A|легальный|legal
"""),
    },
    {
        "code": "m15-u3-allatok-taj",
        "tier": "advanced", "month": 15, "ord": 810,
        "title_ru": "Животные, пейзаж и небо",
        "title_en": "Animals, landscape and sky",
        "goal_ru": "Описываю природу точнее: животных, ландшафт, небо и перемены в нём.",
        "goal_en": "I describe nature more exactly: animals, landscape, sky and how it changes.",
        "situation_ru": "Рассказ о путешествии на природу, передача о животных, фотографии из похода.",
        "situation_en": "A story of a trip into nature, a wildlife show, photos from a hike.",
        "topic_codes": "iterative,participles",
        "words": parse("""
gyík|N|ящерица|lizard
csótány|N|таракан|cockroach
csiga|N|улитка|snail
delfin|N|дельфин|dolphin
keselyű|N|гриф|vulture
kakas|N|петух|rooster
krokodil|N|крокодил|crocodile
hattyú|N|лебедь|swan
holló|N|ворон|raven
varjú|N|ворона|crow
szúnyog|N|комар|mosquito
borjú|N|телёнок|calf
ökör|N|вол|ox
csődör|N|жеребец|stallion
mosómedve|N|енот|raccoon
tonhal|N|тунец|tuna
napsugár|N|солнечный луч|sunbeam
naplemente|N|закат|sunset
napnyugta|N|закат|sunset
holdfény|N|лунный свет|moonlight
szellő|N|ветерок|breeze
dagály|N|прилив|high tide
fészek|N|гнездо|nest
szarv|N|рог|horn
szőr|N|шерсть; волосы|fur, hair
hab|N|пена|foam
dörgés|N|гром|thunder
borul|V|затягиваться (тучами)|to cloud over
lebeg|V|парить; плавать|to float
kering|V|кружить; циркулировать|to circulate
tép|V|рвать|to tear
szór|V|сыпать|to sprinkle
"""),
    },
    {
        "code": "m16-u1-tudomany-kutatas",
        "tier": "advanced", "month": 16, "ord": 811,
        "title_ru": "Наука и исследование",
        "title_en": "Science and research",
        "goal_ru": "Читаю популярную статью о науке и называю предмет, метод и вывод.",
        "goal_en": "I read a popular science article and name the subject, the method and the finding.",
        "situation_ru": "Научно-популярная передача, лекция в университете, разговор об изобретении.",
        "situation_en": "A popular-science programme, a university lecture, a talk about an invention.",
        "topic_codes": "participles,register",
        "words": parse("""
kémia|N|химия|chemistry
biológia|N|биология|biology
matematika|N|математика|mathematics
matematikai|A|математический|mathematical
gravitáció|N|гравитация|gravity
atom|N|атом|atom
kén|N|сера|sulphur
ólom|N|свинец|lead
elektromosság|N|электричество|electricity
gömb|N|шар; сфера|sphere
háromszög|N|треугольник|triangle
találmány|N|изобретение|invention
jártas|A|сведущий|well-versed
megfigyelés|N|наблюдение|observation
megfigyelő|N|наблюдатель|observer
mérték|N|мера; размер|measure, extent
réteg|N|слой|layer
szakasz|N|этап; отрезок|stage, section
fázis|N|фаза|phase
arány|N|пропорция|ratio
változat|N|вариант|variant
kutató|N|исследователь|researcher
naprendszer|N|Солнечная система|solar system
"""),
    },
    {
        "code": "m16-u2-foglalkozasok",
        "tier": "advanced", "month": 16, "ord": 812,
        "title_ru": "Профессии и ремёсла",
        "title_en": "Professions and trades",
        "goal_ru": "Называю профессии и ремёсла, расспрашиваю о чужой работе и читаю объявления о найме.",
        "goal_en": "I name professions and trades, ask about other people's work and read hiring ads.",
        "situation_ru": "Знакомство с новым соседом, ярмарка профессий, объявление о работе.",
        "situation_en": "Meeting a new neighbour, a careers fair, a job ad.",
        "topic_codes": "possessive,register",
        "words": parse("""
fotós|N|фотограф|photographer
építész|N|архитектор|architect
halász|N|рыбак|fisherman
bányász|N|шахтёр|miner
hentes|N|мясник|butcher
kertész|N|садовник|gardener
bokszoló|N|боксёр|boxer
focista|N|футболист|footballer
portás|N|портье; вахтёр|doorkeeper, porter
bűvész|N|фокусник|magician
mágus|N|маг|magician
fényképész|N|фотограф|photographer
titkárnő|N|секретарша|secretary
pincérnő|N|официантка|waitress
műhely|N|мастерская|workshop
gimnázium|N|гимназия|grammar school
dolgozat|N|письменная работа|paper, test
oktatási|A|образовательный|educational
tanítás|N|преподавание|teaching
kézirat|N|рукопись|manuscript
kézírás|N|почерк|handwriting
beszámoló|N|отчёт|report
vázlat|N|набросок; план|draft, outline
pék|N|пекарь|baker
cipész|N|сапожник|cobbler
asztalos|N|столяр|carpenter
kőműves|N|каменщик|bricklayer
fordító|N|переводчик|translator
fodrász|N|парикмахер|hairdresser
"""),
    },
    {
        "code": "m16-u3-irodalom-tortenelem",
        "tier": "advanced", "month": 16, "ord": 813,
        "title_ru": "Литература и история",
        "title_en": "Literature and history",
        "goal_ru": "Читаю о книгах и о прошлом: эпохи, жанры, герои, легенды.",
        "goal_en": "I read about books and the past: eras, genres, heroes, legends.",
        "situation_ru": "Обсуждение романа, школьный урок истории, экскурсия по замку.",
        "situation_en": "Discussing a novel, a school history lesson, a castle tour.",
        "topic_codes": "participles,register",
        "words": parse("""
mítosz|N|миф|myth
költészet|N|поэзия|poetry
ének|N|пение; песня|song
nóta|N|песня; мелодия|tune, song
komédia|N|комедия|comedy
musical|N|мюзикл|musical
irodalmi|A|литературный|literary
színdarab|N|пьеса|play
színházi|A|театральный|theatrical
főszerep|N|главная роль|lead role
hírnök|N|вестник|messenger
legény|N|парень; холостяк|young man
zsarnok|N|тиран|tyrant
vándor|N|странник|wanderer
hős|N|герой|hero
hősies|A|героический|heroic
lény|N|существо|being
szörny|N|чудовище|monster
szörnyeteg|N|чудовище|monster
varázsló|N|волшебник|wizard
kísértet|N|привидение|ghost
rémálom|N|кошмар|nightmare
sors|N|судьба|fate
végzet|N|рок|doom
világháború|N|мировая война|world war
olimpia|N|Олимпиада|the Olympics
költemény|N|стихотворение|poem
hangverseny|N|концерт (классический)|concert
szobrász|N|скульптор|sculptor
karmester|N|дирижёр|conductor
hegedű|N|скрипка|violin
olvasó|N|читатель|reader
hangszer|N|музыкальный инструмент|instrument
dallam|N|мелодия|melody
irodalom|N|литература|literature
költői|A|поэтический|poetic
galéria|N|галерея|gallery
kulturális|A|культурный|cultural
szerző|N|автор|author
főszereplő|N|главный герой|lead
"""),
    },
    {
        "code": "m16-u4-gazdalkodas-etkezes",
        "tier": "advanced", "month": 16, "ord": 814,
        "title_ru": "Хозяйство, продукты и застолье",
        "title_en": "Farming, produce and the table",
        "goal_ru": "Говорю о сельском хозяйстве, продуктах и застолье и читаю этикетки.",
        "goal_en": "I talk about farming, produce and the table, and read labels.",
        "situation_ru": "Рынок и ферма, этикетка на банке, обед у родственников.",
        "situation_en": "The market and the farm, a label on a jar, lunch with relatives.",
        "topic_codes": "iterative,register",
        "words": parse("""
mezőgazdasági|A|сельскохозяйственный|agricultural
gabona|N|зерно|grain
lekvár|N|варенье|jam
mártás|N|соус|sauce
pác|N|маринад|marinade
narancslé|N|апельсиновый сок|orange juice
rum|N|ром|rum
szalvéta|N|салфетка|napkin
bögre|N|кружка|mug
kanna|N|кувшин; чайник|can, jug
korsó|N|кружка (пивная)|jug, tankard
konzerv|N|консервы|tinned food
mazsola|N|изюм|raisin
ebédlő|N|столовая|dining room
étkező|N|столовая; обеденная зона|dining area
cigi|N|сигарета (разг.)|cig
szár|N|стебель|stem
hulladék|N|отходы|waste
pite|N|пирог|pie
süti|N|печенье; пирожное (разг.)|cookie, cake
csokor|N|букет|bouquet
ehető|A|съедобный|edible
tanya|N|хутор|farmstead
háztartás|N|домашнее хозяйство|household
kosz|N|грязь|dirt
halom|N|куча|heap
juh|N|овца|sheep
"""),
    },
    {
        "code": "m17-u1-anyagok-szerszamok",
        "tier": "advanced", "month": 17, "ord": 815,
        "title_ru": "Материалы, инструменты и детали",
        "title_en": "Materials, tools and parts",
        "goal_ru": "Называю материалы, инструменты и детали и объясняю, как что-то сделано или сломалось.",
        "goal_en": "I name materials, tools and parts and explain how something is made or broke.",
        "situation_ru": "Мастерская, магазин стройматериалов, ремонт в доме, инструкция.",
        "situation_en": "A workshop, a builders' merchant, home renovation, an instruction sheet.",
        "topic_codes": "causative,converb",
        "words": parse("""
beton|N|бетон|concrete
tinta|N|чернила|ink
lakat|N|навесной замок|padlock
hurok|N|петля|loop
szekér|N|телега|cart
balta|N|топор|axe
nyél|N|ручка (инструмента)|handle
fedél|N|крышка|lid
kapocs|N|скоба; застёжка|clasp, clip
zsinór|N|шнур|string
gyöngy|N|жемчужина; бусина|pearl, bead
borotva|N|бритва|razor
öngyújtó|N|зажигалка|lighter
repedés|N|трещина|crack
mélység|N|глубина|depth
sáv|N|полоса|strip, lane
verem|N|яма|pit
pálca|N|палочка|stick, wand
vászon|N|холст; полотно|linen, canvas
kendő|N|платок|scarf, cloth
zakó|N|пиджак|jacket
köntös|N|халат|robe
zsebkendő|N|носовой платок|handkerchief
fürdőkád|N|ванна|bathtub
plafon|N|потолок|ceiling
gépezet|N|механизм|machinery
betesz|V|положить внутрь|to put in
kitesz|V|выложить; выставить|to put out
összeszed|V|собрать|to gather
kihúz|V|вытащить|to pull out
megőriz|V|сберечь|to preserve
ugrál|V|прыгать (многократно)|to jump about
megfog|V|схватить|to catch, to grab
csúszik|V|скользить|to slide
tárol|V|хранить|to store
csíp|V|щипать; кусать|to pinch
"""),
    },
    {
        "code": "m17-u2-verseny-szabadido",
        "tier": "advanced", "month": 17, "ord": 816,
        "title_ru": "Состязания и досуг",
        "title_en": "Competition and leisure",
        "goal_ru": "Говорю об играх, спорте как зрелище и о том, как провести свободный день.",
        "goal_en": "I talk about games, sport as a spectacle and how to spend a free day.",
        "situation_ru": "Стадион, пикник, праздник на улице, олимпийская трансляция.",
        "situation_en": "A stadium, a picnic, a street fair, an Olympic broadcast.",
        "topic_codes": "potential,iterative",
        "words": parse("""
stadion|N|стадион|stadium
olimpiai|A|олимпийский|Olympic
helyezett|N|призёр|place winner
rúgás|N|удар ногой|kick
ütés|N|удар|blow, hit
úszó|A|плавающий|swimming
búvár|N|ныряльщик; дайвер|diver
piknik|N|пикник|picnic
parádé|N|парад|parade
mulatság|N|веселье; гулянье|fun, merrymaking
mulatságos|A|забавный|amusing
rejtvény|N|головоломка; загадка|puzzle
tűzijáték|N|фейерверк|fireworks
vásárlás|N|покупки|shopping
ünnepélyes|A|торжественный|solemn
látványos|A|эффектный|spectacular
élvezetes|A|приятный; увлекательный|enjoyable
szórakozóhely|N|развлекательное заведение|venue, club
"""),
    },
    {
        "code": "m17-u3-valtozas-folyamat",
        "tier": "advanced", "month": 17, "ord": 817,
        "title_ru": "Перемены и процессы",
        "title_en": "Change and process",
        "goal_ru": "Рассказываю, как что-то меняется, начинается и заканчивается, и называю причины перемен.",
        "goal_en": "I tell how something changes, starts and ends, and name the causes of change.",
        "situation_ru": "Отчёт о проекте, рассказ о переезде, статья об изменениях.",
        "situation_en": "A project report, a story of moving house, an article on changes.",
        "topic_codes": "converb,participles",
        "words": parse("""
elmarad|V|отменяться; не состояться|to be cancelled; to fall behind
megszűnik|V|прекращаться|to cease
elfogy|V|кончаться|to run out
elterjedt|A|распространённый|widespread
növekvő|A|растущий|growing
változtatás|N|изменение|change
változatlan|A|неизменный|unchanged
elkövetkező|A|предстоящий|coming
későbbi|A|более поздний|later
előbbi|A|прежний|former
közelgő|A|приближающийся|approaching
esedékes|A|подлежащий уплате|due
időközben|D|тем временем|meanwhile
utólag|D|впоследствии|afterwards
rövidesen|D|вскоре|shortly
újonnan|D|заново; недавно|newly
közeljövő|N|ближайшее будущее|near future
alakul|V|складываться; формироваться|to take shape
kialakul|V|сформироваться|to form
adódik|V|подворачиваться; возникать|to arise
zajlik|V|протекать; проходить|to take place
beválik|V|оправдаться|to prove effective
megváltozik|V|измениться|to change
megváltoztat|V|изменить|to change
időtartam|N|продолжительность|duration
alkonyat|N|сумерки|dusk
elkésik|V|опоздать|to be late
ebédidő|N|обеденное время|lunchtime
"""),
    },
    {
        "code": "m18-u1-lelek-tarsas",
        "tier": "advanced", "month": 18, "ord": 818,
        "title_ru": "Характер и общество: оттенки",
        "title_en": "Character and society: shades",
        "goal_ru": "Различаю оттенки в оценке человека: тактичный, мягкий, резкий, двуличный, и говорю об этом осторожно.",
        "goal_en": "I tell the shades in judging a person: tactful, mild, sharp, two-faced, and say it with care.",
        "situation_ru": "Рекомендация, разговор о коллеге, воспитание детей, отзыв о знакомом.",
        "situation_en": "A reference, a chat about a colleague, bringing up children, an opinion of an acquaintance.",
        "topic_codes": "comparison,register",
        "words": parse("""
optimista|A|оптимистичный|optimistic
beképzelt|A|самодовольный|conceited
szívtelen|A|бессердечный|heartless
mogorva|A|угрюмый|morose
tudatlan|A|невежественный|ignorant
higgadt|A|хладнокровный|composed
szeszélyes|A|капризный|capricious
eszes|A|смышлёный|clever
jószívű|A|добросердечный|kind-hearted
megfontolt|A|обдуманный; рассудительный|thoughtful, deliberate
zárkózott|A|замкнутый|reserved
mohó|A|жадный|greedy
képmutató|A|лицемерный|hypocritical
rosszindulatú|A|злобный|malevolent
szimpatikus|A|симпатичный (приятный)|likeable
beszédes|A|разговорчивый|talkative
engedelmeskedik|V|подчиняться|to obey
szelíd|A|кроткий|gentle
fürge|A|проворный|nimble
komor|A|мрачный|gloomy
udvariasan|D|вежливо|politely
udvariasság|N|вежливость|courtesy
gonoszság|N|злоба; злодеяние|evil, malice
kegyetlenség|N|жестокость|cruelty
erkölcs|N|нравственность|morals
fegyelem|N|дисциплина|discipline
hidegvér|N|хладнокровие|composure
ösztönösen|D|инстинктивно|instinctively
modor|N|манеры|manners
kiemelkedő|A|выдающийся|outstanding
hiteles|A|достоверный|authentic
kitartó|A|стойкий|persistent
szid|V|ругать|to scold
dicsér|V|хвалить|to praise
hajlam|N|склонность|tendency
alkalmazkodik|V|приспосабливаться|to adapt
törekszik|V|стремиться|to strive
jellem|N|характер|character
közömbös|A|равнодушный|indifferent
figyelmetlen|A|невнимательный|inattentive
erény|N|добродетель|virtue
szorgalmas|A|прилежный|diligent
panaszkodik|V|жаловаться|to complain
udvariatlan|A|невежливый|rude
segítőkész|A|отзывчивый|helpful
ügyetlen|A|неловкий|clumsy
szemtelen|A|дерзкий; наглый|impudent
pimasz|A|нахальный|cheeky
vakmerő|A|безрассудный|daring
felelőtlen|A|безответственный|irresponsible
érzéketlen|A|бесчувственный|insensitive
hálátlan|A|неблагодарный|ungrateful
"""),
    },
    {
        "code": "m18-u2-erzelmek-melyen",
        "tier": "advanced", "month": 18, "ord": 819,
        "title_ru": "Чувства: глубже",
        "title_en": "Feelings in depth",
        "goal_ru": "Называю тонкие состояния: тоска, зависть, облегчение, вина, и говорю, откуда они.",
        "goal_en": "I name subtle states: yearning, envy, relief, guilt, and say where they come from.",
        "situation_ru": "Личное письмо, откровенный разговор, рассказ о тяжёлом периоде.",
        "situation_en": "A personal letter, a frank talk, an account of a hard time.",
        "topic_codes": "conditional,register",
        "words": parse("""
bú|N|скорбь|sorrow
kétségbeesés|N|отчаяние|despair
féltékenység|N|ревность|jealousy
nyomasztó|A|гнетущий|oppressive
nyugtalanító|A|тревожный|disturbing
nyomott|A|подавленный|downcast
örömteli|A|радостный|joyful
szomorúan|D|грустно|sadly
vidáman|D|весело|cheerfully
ártatlanság|N|невинность|innocence
bosszant|V|раздражать|to annoy
megőrül|V|сойти с ума|to go mad
éhség|N|голод|hunger
kín|N|мука|torment
megrázó|A|потрясающий|shocking
sikoly|N|крик|scream
borzalmasan|D|ужасно|terribly
hátborzongató|A|жуткий|spooky
kiábrándít|V|разочаровывать|to disillusion
csábít|V|соблазнять|to tempt
aggódó|A|обеспокоенный|worried
kiált|V|кричать|to shout
kiabál|V|орать|to shout
házastárs|N|супруг|spouse
veszekszik|V|ссориться|to quarrel
szívélyes|A|сердечный|cordial
vonzódik|V|тянуться к|to be drawn to
randevú|N|свидание|date
viszály|N|раздор|feud
undok|A|противный|nasty
bölcsesség|N|мудрость|wisdom
önbizalom|N|уверенность в себе|self-confidence
ijed|V|испугаться|to get frightened
goromba|A|грубый|rude
arrogáns|A|высокомерный|arrogant
veszekedés|N|ссора|quarrel
kíváncsiság|N|любопытство|curiosity
megkönnyebbülés|N|облегчение|relief
magabiztos|A|уверенный в себе|self-confident
aggodalom|N|тревога|worry
"""),
    },
    {
        "code": "m18-u3-erzekeles-leiras",
        "tier": "advanced", "month": 18, "ord": 820,
        "title_ru": "Ощущения и описание",
        "title_en": "Senses and description",
        "goal_ru": "Описываю вещи и ощущения: вид, цвет, фактуру, запах, состояние.",
        "goal_en": "I describe things and sensations: look, colour, texture, smell, condition.",
        "situation_ru": "Описание комнаты, письмо с впечатлениями, рассказ о картине или еде.",
        "situation_en": "Describing a room, a letter of impressions, talking about a painting or a meal.",
        "topic_codes": "comparison,participles",
        "words": parse("""
sáros|A|грязный (от грязи)|muddy
poros|A|пыльный|dusty
rozsdás|A|ржавый|rusty
csúszós|A|скользкий|slippery
meredek|A|крутой|steep
görbe|A|кривой|crooked
ferde|A|наклонный|slanting
hegyes|A|острый (о конце)|pointed
feszes|A|тугой; облегающий|tight
csupasz|A|голый (о предмете)|bare
tömör|A|плотный; сжатый|dense, concise
fekvő|A|лежачий|lying
ülő|A|сидячий|sitting
álló|A|стоящий|standing
éles|A|острый|sharp
tompa|A|тупой|blunt
puha|A|мягкий|soft
sűrű|A|густой|thick
lágy|A|нежный; мягкий|soft
merev|A|жёсткий; неподвижный|rigid
mozdulatlan|A|неподвижный|motionless
szilárd|A|твёрдый|solid
kerek|A|круглый|round
lapos|A|плоский|flat
ízű|A|…на вкус|flavoured
égett|A|сожжённый; подгоревший|burnt
vizes|A|мокрый; водянистый|watery
zavaros|A|мутный|cloudy
homályos|A|тусклый; смутный|dim, vague
"""),
    },
]
