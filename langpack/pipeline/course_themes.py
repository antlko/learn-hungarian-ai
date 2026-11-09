"""Themed side units: twelve optional word-families beside the guided path.

The twenty-four core units are ordered by errand — what fails first in a real
day. Some vocabulary does not belong to one errand but to a family: the colours,
the parts of the body, the weather, the months. A learner who wants those words
should be able to ask for them without the plan pretending they are the next
step, and without the path growing a twenty-fifth unit. So they live here, with
`track = 'theme'`.

What a theme unit is, precisely (ADR-050):

  * optional and finite — twelve, about forty words each, never extended into a
    drip. The course still ENDS at the last core unit (R11);
  * a side branch — it never decides `current` or `handedOver`, and it is not
    counted in "unit N of 24". It still shows its own derived done/ahead state;
  * disjoint from the path — a word the core already teaches is not repeated
    here (course.py drops it from the theme, never from the core), so finishing
    a theme never double-books a card;
  * hand-written glosses, and only words the pack can look up AND show in a real
    sentence. Nothing here is generated Hungarian.

Word syntax is documented in _wordlist.py.
"""

from __future__ import annotations

from ._wordlist import parse

UNITS = [
    {
        "code": "t-szamok",
        "month": 6,
        "ord": 1001,
        "title_ru": "Числа и счёт",
        "title_en": "Numbers and counting",
        "goal_ru": "Считаю до ста и дальше, называю порядок и доли, сравниваю количества.",
        "goal_en": "I count to a hundred and beyond, give order and fractions, and compare amounts.",
        "situation_ru": "Очередь за номерком, счёт в кафе, адрес и этаж, цена за килограмм.",
        "situation_en": "A queue ticket, the café bill, an address and a floor, the price per kilo.",
        "topic_codes": "vowel-harmony,plural",
        "words": parse("""
tizenkettő|M|двенадцать|twelve
hatvan|M|шестьдесят|sixty
hetven|M|семьдесят|seventy
nyolcvan|M|восемьдесят|eighty
kilencven|M|девяносто|ninety
negyedik|M|четвёртый|fourth
ötödik|M|пятый|fifth
negyed|M|четверть|quarter
dupla|A|двойной|double
páros|A|чётный; парный|even; paired
páratlan|A|нечётный; непарный|odd; unpaired
egyenlő|A|равный|equal
százalék|N|процент|per cent
tucat|N|дюжина|dozen
kétszer|D|дважды|twice
háromszor|D|трижды|three times
tizenhárom|M|тринадцать|thirteen
tizenöt|M|пятнадцать|fifteen
tizennyolc|M|восемнадцать|eighteen
kétszáz|M|двести|two hundred
ötszáz|M|пятьсот|five hundred
milliárd|M|миллиард|billion
hatodik|M|шестой|sixth
hetedik|M|седьмой|seventh
nyolcadik|M|восьмой|eighth
tizedik|M|десятый|tenth
egyik|PRON|один из; первый (из двух)|one of
négyszer|D|четыре раза|four times
ötször|D|пять раз|five times
sokszor|D|много раз|many times
hármas|N|тройка|three, a «3»
kettes|N|двойка|two, a «2»
ötös|N|пятёрка|five, a «5»
legalább|D|по меньшей мере|at least
legfeljebb|D|самое большее|at most
"""),
    },
    {
        "code": "t-ido-ora",
        "month": 6,
        "ord": 1002,
        "title_ru": "Время и часы",
        "title_en": "Time and the clock",
        "goal_ru": "Называю время суток и точный час, договариваюсь «через час», «до обеда», «в срок».",
        "goal_en": "I name the part of the day and the exact hour, and agree on «in an hour», «before lunch», «on time».",
        "situation_ru": "Расписание автобуса, встреча в три часа, опоздание на десять минут.",
        "situation_en": "A bus timetable, a meeting at three, being ten minutes late.",
        "topic_codes": "locative-grid,dative",
        "words": parse("""
hajnal|N|рассвет|dawn
holnapután|D|послезавтра|the day after tomorrow
gyakran|D|часто|often
ritkán|D|редко|rarely
pontos|A|точный|punctual, exact
hamar|D|скоро; быстро|soon; quickly
addig|D|до тех пор|until then
jelen|N|настоящее|the present
negyedóra|N|четверть часа|quarter of an hour
háromnegyed|M|три четверти|three quarters
óránként|D|каждый час|hourly
reggelente|D|по утрам|in the mornings
esténként|D|по вечерам|in the evenings
hétköznap|N|будний день|weekday
ünnepnap|N|праздничный день|public holiday
karóra|N|наручные часы|wristwatch
ébresztőóra|N|будильник|alarm clock
ébreszt|V|будить|to wake (someone)
ébred|V|просыпаться|to wake up
felkel|V|вставать|to get up
elalszik|V|засыпать|to fall asleep
utána|D|после этого|afterwards
rendszeresen|D|регулярно|regularly
rendszerint|D|обычно|usually
többnyire|D|большей частью|mostly
időszak|N|период|period
évtized|N|десятилетие|decade
évszázad|N|век|century
mostanában|D|в последнее время|lately
jelenleg|D|в настоящее время|currently
folyton|D|постоянно|constantly
állandóan|D|всё время|all the time
valaha|D|когда-либо|ever
valamikor|D|когда-нибудь; когда-то|sometime
ekkor|D|тогда; в этот момент|then
aznap|D|в тот же день|that day
másnap|D|на следующий день|the next day
napközben|D|в течение дня|during the day
eddig|D|до сих пор|so far
mostanra|D|к настоящему времени|by now
időnként|D|время от времени|from time to time
olykor|D|порой|at times
gyakori|A|частый|frequent
rendszeres|A|регулярный|regular
ritka|A|редкий|rare
idejében|D|вовремя|in time
"""),
    },
    {
        "code": "t-napok-honapok",
        "month": 6,
        "ord": 1003,
        "title_ru": "Дни, месяцы, даты",
        "title_en": "Days, months, dates",
        "goal_ru": "Называю день недели, месяц и дату, спрашиваю «когда?» и отвечаю.",
        "goal_en": "I name the weekday, the month and the date, and ask and answer «when?».",
        "situation_ru": "Запись в календарь, дата в заявлении, праздники и выходные.",
        "situation_en": "A calendar entry, a date on a form, holidays and days off.",
        "topic_codes": "locative-grid,dative",
        "words": parse("""
hétvége|N|выходные|weekend
január|N|январь|January
február|N|февраль|February
március|N|март|March
április|N|апрель|April
május|N|май|May
június|N|июнь|June
július|N|июль|July
augusztus|N|август|August
szeptember|N|сентябрь|September
október|N|октябрь|October
november|N|ноябрь|November
december|N|декабрь|December
évszak|N|время года|season
dátum|N|дата|date
naptár|N|календарь|calendar
ünnep|N|праздник|holiday, feast
karácsony|N|Рождество|Christmas
húsvét|N|Пасха|Easter
évforduló|N|годовщина|anniversary
naponta|D|ежедневно|daily
hetente|D|еженедельно|weekly
havonta|D|ежемесячно|monthly
évente|D|ежегодно|yearly
tavaszi|A|весенний|spring (adj.)
nyári|A|летний|summer (adj.)
téli|A|зимний|winter (adj.)
őszi|A|осенний|autumn (adj.)
heti|A|недельный|weekly
napi|A|дневной; ежедневный|daily
mai|A|сегодняшний|today's
tegnapi|A|вчерашний|yesterday's
holnapi|A|завтрашний|tomorrow's
szilveszter|N|Новый год (31 декабря)|New Year's Eve
újév|N|Новый год|New Year
ünnepel|V|праздновать|to celebrate
ünnepség|N|торжество|celebration
fesztivál|N|фестиваль|festival
nemzeti|A|национальный|national
karácsonyi|A|рождественский|Christmas (adj.)
meghívó|N|приглашение|invitation
ajándék|N|подарок|present, gift
születés|N|рождение|birth
"""),
    },
    {
        "code": "t-szinek",
        "month": 6,
        "ord": 1004,
        "title_ru": "Цвета",
        "title_en": "Colours",
        "goal_ru": "Называю цвет предмета, прошу «другого цвета», описываю, как выглядит вещь.",
        "goal_en": "I name an object's colour, ask for «another colour» and describe how a thing looks.",
        "situation_ru": "Выбор одежды в магазине, описание пропавшей сумки, покраска стен.",
        "situation_en": "Choosing clothes in a shop, describing a lost bag, painting the walls.",
        "topic_codes": "plural,accusative",
        "words": parse("""
szín|N|цвет|colour
fehér|A|белый|white
fekete|A|чёрный|black
piros|A|красный|red
vörös|A|красный; рыжий|red; ginger
kék|A|синий; голубой|blue
zöld|A|зелёный|green
sárga|A|жёлтый|yellow
szürke|A|серый|grey
narancssárga|A|оранжевый|orange
rózsaszín|A|розовый|pink
lila|A|фиолетовый|purple
arany|N|золото; золотой|gold
ezüst|N|серебро; серебряный|silver
sötét|A|тёмный|dark
színes|A|цветной; яркий|coloured
halvány|A|бледный|pale, faint
élénk|A|яркий; живой|vivid, lively
fényes|A|блестящий; яркий|shiny, bright
tarka|A|пёстрый|motley, mottled
festék|N|краска|paint
fest|V|красить; рисовать красками|to paint
kép|N|картина; изображение|picture, image
árnyalat|N|оттенок|shade
szivárvány|N|радуга|rainbow
csíkos|A|полосатый|striped
pöttyös|A|в горошек|dotted
kockás|A|клетчатый|checked
sápadt|A|бледный|pale
türkiz|A|бирюзовый|turquoise
színtelen|A|бесцветный|colourless
átlátszó|A|прозрачный|transparent
sötétség|N|темнота|darkness
árnyék|N|тень|shadow, shade
fény|N|свет|light
ragyogó|A|сияющий|radiant
csillog|V|блестеть; сверкать|to glitter
fakó|A|выцветший; тусклый|faded, dull
"""),
    },
    {
        "code": "t-test",
        "month": 6,
        "ord": 1005,
        "title_ru": "Тело",
        "title_en": "The body",
        "goal_ru": "Называю части тела и показываю, что и где болит.",
        "goal_en": "I name the parts of the body and show what hurts and where.",
        "situation_ru": "Приём у врача, спортзал, разговор о травме или головной боли.",
        "situation_en": "At the doctor's, at the gym, talking about an injury or a headache.",
        "topic_codes": "possessive,locative-grid",
        "words": parse("""
test|N|тело|body
haj|N|волосы|hair
arc|N|лицо|face
homlok|N|лоб|forehead
szemöldök|N|бровь|eyebrow
áll|N|подбородок|chin
könyök|N|локоть|elbow
köröm|N|ноготь|nail
mell|N|грудь|chest
boka|N|лодыжка|ankle
talp|N|подошва|sole
izom|N|мышца|muscle
agy|N|мозг|brain
comb|N|бедро|thigh
lábujj|N|палец ноги|toe
tenyér|N|ладонь|palm
csukló|N|запястье|wrist
derék|N|талия; поясница|waist
mellkas|N|грудная клетка|chest
gerinc|N|позвоночник|spine
ízület|N|сустав|joint
máj|N|печень|liver
vese|N|почка|kidney
ér|N|сосуд; жила|vein
pulzus|N|пульс|pulse
lélegzet|N|дыхание|breath
szempilla|N|ресница|eyelash
bajusz|N|усы|moustache
kövér|A|толстый (о человеке)|fat
erős|A|сильный|strong
gyenge|A|слабый|weak
fizikai|A|физический|physical
mozog|V|двигаться|to move
mozgás|N|движение|movement
ül|V|сидеть|to sit
fekszik|V|лежать|to lie
lép|V|шагать|to step
ugrik|V|прыгать|to jump
érint|V|трогать; касаться|to touch
"""),
    },
    {
        "code": "t-idojaras",
        "month": 6,
        "ord": 1006,
        "title_ru": "Погода",
        "title_en": "The weather",
        "goal_ru": "Говорю о погоде сегодня и завтра, понимаю прогноз и выбираю одежду.",
        "goal_en": "I talk about today's and tomorrow's weather, follow a forecast and choose what to wear.",
        "situation_ru": "Разговор на остановке, прогноз на выходные, сборы в поездку.",
        "situation_en": "Small talk at the stop, the weekend forecast, packing for a trip.",
        "topic_codes": "present-indefinite,van-nincs",
        "words": parse("""
időjárás|N|погода|weather
előrejelzés|N|прогноз|forecast
eső|N|дождь|rain
esik|V|падать; идти (о дожде)|to fall; to rain
hó|N|снег|snow
szél|N|ветер|wind
fúj|V|дуть|to blow
felhő|N|облако|cloud
köd|N|туман|fog
vihar|N|буря; гроза|storm
villám|N|молния|lightning
mennydörgés|N|гром|thunder
ég|N|небо|sky
hőmérséklet|N|температура|temperature
fok|N|градус|degree
hűvös|A|прохладный|cool
forró|A|горячий; жаркий|hot, scorching
fagy|N|мороз; заморозок|frost
jég|N|лёд|ice
napos|A|солнечный|sunny
felhős|A|облачный|cloudy
esős|A|дождливый|rainy
szeles|A|ветреный|windy
ködös|A|туманный|foggy
száraz|A|сухой|dry
nedves|A|влажный; мокрый|wet, damp
süt|V|светить; печь|to shine; to bake
olvad|V|таять|to melt
esernyő|N|зонт|umbrella
sál|N|шарф|scarf
kesztyű|N|перчатка|glove
sapka|N|шапка; кепка|cap, hat
pocsolya|N|лужа|puddle
sár|N|грязь|mud
napsütés|N|солнечный свет|sunshine
zivatar|N|гроза|thunderstorm
jégeső|N|град|hail
árvíz|N|наводнение|flood
napfény|N|солнечный свет|sunlight
enyhe|A|мягкий; тёплый (о погоде)|mild
borult|A|пасмурный|overcast
derült|A|ясный|clear
fülledt|A|душный|muggy
hűl|V|остывать; холодать|to cool
melegszik|V|теплеть|to warm up
fázik|V|мёрзнуть|to feel cold
izzad|V|потеть|to sweat
hőség|N|жара|heat
csapadék|N|осадки|precipitation
éghajlat|N|климат|climate
"""),
    },
    {
        "code": "t-etel-ital",
        "month": 6,
        "ord": 1007,
        "title_ru": "Еда и напитки",
        "title_en": "Food and drink",
        "goal_ru": "Называю продукты и блюда, заказываю, прошу «без соли» и «ещё немного».",
        "goal_en": "I name foods and dishes, order, and ask for «no salt» or «a little more».",
        "situation_ru": "Рынок, меню в столовой, ужин у друзей, рецепт от соседки.",
        "situation_en": "The market, the canteen menu, dinner at a friend's, a neighbour's recipe.",
        "topic_codes": "accusative,plural",
        "words": parse("""
méz|N|мёд|honey
csokoládé|N|шоколад|chocolate
kukorica|N|кукуруза|sweetcorn, maize
bab|N|фасоль|bean
borsó|N|горох|pea
cseresznye|N|черешня; вишня|cherry
meggy|N|вишня|sour cherry
eper|N|клубника|strawberry
szilva|N|слива|plum
barack|N|персик; абрикос|peach; apricot
dinnye|N|арбуз; дыня|melon
gomba|N|гриб|mushroom
gulyás|N|гуляш|goulash
főzelék|N|овощное рагу|vegetable stew
zsemle|N|булочка|bread roll
kakaó|N|какао|cocoa
keksz|N|печенье|biscuit
ecet|N|уксус|vinegar
szósz|N|соус|sauce
mustár|N|горчица|mustard
főtt|A|варёный|boiled
savanyú|A|кислый; квашеный|sour; pickled
keserű|A|горький|bitter
sós|A|солёный|salty
hozzávaló|N|ингредиент|ingredient
reggeli|N|завтрак|breakfast
szalámi|N|салями|salami
virsli|N|сосиска|frankfurter
káposzta|N|капуста|cabbage
tök|N|тыква; кабачок|pumpkin, squash
spenót|N|шпинат|spinach
retek|N|редис|radish
dió|N|грецкий орех|walnut
mandula|N|миндаль|almond
szalonna|N|сало; бекон|bacon
marha|N|корова; говядина|cow; beef
bárány|N|ягнёнок; баранина|lamb
kacsa|N|утка|duck
liba|N|гусь|goose
pulyka|N|индейка|turkey
lazac|N|лосось|salmon
töltött|A|фаршированный|stuffed
érett|A|спелый|ripe
vág|V|резать|to cut
önt|V|наливать|to pour
étkezés|N|приём пищи|meal
"""),
    },
    {
        "code": "t-ruhazat",
        "month": 6,
        "ord": 1008,
        "title_ru": "Одежда",
        "title_en": "Clothes",
        "goal_ru": "Называю предметы одежды и размер, прошу примерить и говорю, что не подходит.",
        "goal_en": "I name garments and sizes, ask to try something on and say what does not fit.",
        "situation_ru": "Магазин одежды, примерочная, сбор чемодана, возврат вещи.",
        "situation_en": "A clothes shop, the fitting room, packing a suitcase, returning an item.",
        "topic_codes": "accusative,comparison",
        "words": parse("""
kabát|N|пальто; куртка|coat
nadrág|N|брюки|trousers
szoknya|N|юбка|skirt
ing|N|рубашка|shirt
póló|N|футболка|T-shirt
pulóver|N|свитер|jumper
zokni|N|носок|sock
cipő|N|ботинок; обувь|shoe
csizma|N|сапог|boot
kalap|N|шляпа|hat
öv|N|ремень; пояс|belt
zseb|N|карман|pocket
cipzár|N|молния (застёжка)|zip
nyaklánc|N|цепочка; ожерелье|necklace
fülbevaló|N|серьга|earring
öltöny|N|костюм|suit
ruhatár|N|гардероб; гардеробная|cloakroom
pizsama|N|пижама|pyjamas
fehérnemű|N|бельё|underwear
hord|V|носить|to wear; to carry
visel|V|носить (одежду)|to wear
szűk|A|узкий; тесный|tight, narrow
bő|A|свободный; широкий|loose, wide
divat|N|мода|fashion
elegáns|A|элегантный|elegant
kényelmes|A|удобный|comfortable
ruházat|N|одежда (в целом)|clothing
blúz|N|блузка|blouse
harisnya|N|колготки; чулки|tights
papucs|N|тапочки|slipper
nyakkendő|N|галстук|tie
mellény|N|жилет|waistcoat
fürdőruha|N|купальник|swimsuit
anyag|N|ткань; материал|fabric, material
gyapjú|N|шерсть|wool
pamut|N|хлопок|cotton
selyem|N|шёлк|silk
farmer|N|джинсы|jeans
divatos|A|модный|fashionable
szakadt|A|рваный|torn
használt|A|подержанный|second-hand
szabó|N|портной|tailor
öltözik|V|одеваться|to dress
"""),
    },
    {
        "code": "t-otthon-targyak",
        "month": 6,
        "ord": 1009,
        "title_ru": "Дом и вещи",
        "title_en": "Home and things",
        "goal_ru": "Называю комнаты, мебель и предметы быта, прошу передать, найти и починить.",
        "goal_en": "I name rooms, furniture and household things, and ask to pass, find or fix them.",
        "situation_ru": "Переезд, уборка, разговор с хозяином квартиры, поиск потерянного ключа.",
        "situation_en": "Moving in, cleaning, talking to the landlord, looking for a lost key.",
        "topic_codes": "locative-grid,possessive",
        "words": parse("""
mennyezet|N|потолок|ceiling
hűtőszekrény|N|холодильник|fridge
mosogató|N|раковина|sink
seprű|N|веник; метла|broom
földszint|N|первый этаж|ground floor
kerítés|N|забор|fence
izzó|N|лампочка|bulb
gyertya|N|свеча|candle
serpenyő|N|сковорода|frying pan
lábas|N|кастрюля|saucepan
fogkefe|N|зубная щётка|toothbrush
fogkrém|N|зубная паста|toothpaste
fésű|N|расчёска|comb
sampon|N|шампунь|shampoo
vasaló|N|утюг|iron
vasal|V|гладить|to iron
szárít|V|сушить|to dry
lepedő|N|простыня|bed sheet
matrac|N|матрас|mattress
íróasztal|N|письменный стол|desk
könyvespolc|N|книжная полка|bookshelf
vödör|N|ведро|bucket
váza|N|ваза|vase
festmény|N|картина|painting
tégla|N|кирпич|brick
szög|N|гвоздь|nail
kalapács|N|молоток|hammer
fűrész|N|пила|saw
csavarhúzó|N|отвёртка|screwdriver
létra|N|стремянка|ladder
drót|N|проволока|wire
kötél|N|верёвка|rope
zseblámpa|N|фонарик|torch
elem|N|батарейка|battery
olló|N|ножницы|scissors
tű|N|игла|needle
cérna|N|нитка|thread
lapát|N|лопата|shovel
virág|N|цветок|flower
növény|N|растение|plant
fű|N|трава|grass
bokor|N|куст|bush
ágynemű|N|постельное бельё|bedding
kandalló|N|камин|fireplace
kályha|N|печь|stove
kémény|N|дымоход; труба|chimney
"""),
    },
    {
        "code": "t-utazas",
        "month": 6,
        "ord": 1010,
        "title_ru": "Путешествия",
        "title_en": "Travel",
        "goal_ru": "Покупаю билет, ищу остановку и гостиницу, спрашиваю дорогу и рассказываю о поездке.",
        "goal_en": "I buy a ticket, find the stop and the hotel, ask the way and tell about a trip.",
        "situation_ru": "Вокзал, аэропорт, регистрация в гостинице, поход в горы.",
        "situation_en": "The station, the airport, hotel check-in, a hike in the hills.",
        "topic_codes": "locative-grid,past-tense",
        "words": parse("""
utazik|V|ехать; путешествовать|to travel
utazás|N|поездка; путешествие|journey, trip
repülő|N|самолёт|aeroplane
repülőtér|N|аэропорт|airport
bőrönd|N|чемодан|suitcase
tenger|N|море|sea
tengerpart|N|побережье|seaside
hegy|N|гора|mountain
tó|N|озеро|lake
nyaral|V|отдыхать (в отпуске)|to holiday
nyaralás|N|отпуск; отдых|holiday
turista|N|турист|tourist
idegenvezető|N|гид|guide
látnivaló|N|достопримечательность|sight
múzeum|N|музей|museum
indulás|N|отправление|departure
érkezés|N|прибытие|arrival
kikötő|N|порт; пристань|harbour
hajó|N|корабль; судно|ship
benzin|N|бензин|petrol
szálloda|N|гостиница|hotel
jogosítvány|N|водительские права|driving licence
kerék|N|колесо|wheel
autópálya|N|автострада|motorway
peron|N|платформа|platform
vágány|N|путь (ж/д)|track
menetrend|N|расписание|timetable
kalauz|N|кондуктор|conductor
poggyász|N|багаж|luggage
strand|N|пляж|beach
homok|N|песок|sand
napszemüveg|N|солнечные очки|sunglasses
szállás|N|жильё; ночлег|accommodation
sátor|N|палатка|tent
hátizsák|N|рюкзак|rucksack
túra|N|поход|hike
ösvény|N|тропа|path, trail
sziget|N|остров|island
főváros|N|столица|capital city
tolmács|N|переводчик (устный)|interpreter
útvonal|N|маршрут|route
komp|N|паром|ferry
csónak|N|лодка|boat
utas|N|пассажир|passenger
pilóta|N|пилот|pilot
helikopter|N|вертолёт|helicopter
benzinkút|N|заправка|petrol station
üzemanyag|N|топливо|fuel
határ|N|граница|border
sivatag|N|пустыня|desert
erdő|N|лес|forest
folyó|N|река|river
völgy|N|долина|valley
domb|N|холм|hill
szikla|N|скала|rock, cliff
barlang|N|пещера|cave
kontinens|N|континент|continent
"""),
    },
    {
        "code": "t-erzesek",
        "month": 6,
        "ord": 1011,
        "title_ru": "Настроение и самочувствие",
        "title_en": "Moods and how you feel",
        "goal_ru": "Говорю, как себя чувствую, и называю простые состояния: голод, усталость, радость, тревогу.",
        "goal_en": "I say how I feel and name the plain states: hunger, tiredness, joy, worry.",
        "situation_ru": "Ответ на «как дела?», разговор с другом, жалоба на усталость.",
        "situation_en": "Answering «how are you?», talking to a friend, complaining of tiredness.",
        "topic_codes": "present-indefinite,van-nincs",
        "words": parse("""
álmos|A|сонный|sleepy
reménykedik|V|надеяться|to hope
unatkozik|V|скучать|to be bored
szégyell|V|стыдиться|to be ashamed of
kedv|N|настроение; охота|mood; inclination
szégyenlős|A|стеснительный|bashful
irigy|A|завистливый|envious
lelkes|A|увлечённый; восторженный|enthusiastic
lelkesedés|N|энтузиазм|enthusiasm
szorongás|N|тревога|anxiety
gyűlölet|N|ненависть|hatred
részvét|N|сочувствие|sympathy
gyanakszik|V|подозревать|to suspect
magány|N|одиночество|loneliness
bűntudat|N|чувство вины|guilt
pánik|N|паника|panic
mosoly|N|улыбка|smile
nevetés|N|смех|laughter
könny|N|слеза|tear
vágy|N|желание|desire
kedvesség|N|доброта|kindness
fájdalmas|A|болезненный|painful
élvez|V|наслаждаться|to enjoy
élvezet|N|удовольствие|pleasure
bátorság|N|смелость|courage
reménytelen|A|безнадёжный|hopeless
boldogtalan|A|несчастный|unhappy
jókedvű|A|весёлый; в хорошем настроении|cheerful
szerencsés|A|счастливый; удачливый|lucky
szerencsétlen|A|несчастливый; неудачливый|unlucky
érzékeny|A|чувствительный|sensitive
sértés|N|обида; оскорбление|insult
féltékeny|A|ревнивый|jealous
türelmetlen|A|нетерпеливый|impatient
kényelmetlen|A|неудобный|uncomfortable
borzasztó|A|ужасный|dreadful
félelmetes|A|страшный|frightening
remeg|V|дрожать|to tremble
kitartás|N|упорство|perseverance
lelkiismeret|N|совесть|conscience
sajnálat|N|сожаление|regret
"""),
    },
    {
        "code": "t-hobbi",
        "month": 6,
        "ord": 1012,
        "title_ru": "Хобби и свободное время",
        "title_en": "Hobbies and free time",
        "goal_ru": "Рассказываю, чем занимаюсь в свободное время, зову на прогулку, в кино или на матч.",
        "goal_en": "I say what I do in my free time and invite someone for a walk, a film or a match.",
        "situation_ru": "Знакомство с коллегами, вечер пятницы, секция по выходным.",
        "situation_en": "Getting to know colleagues, a Friday evening, a weekend club.",
        "topic_codes": "infinitive,instrumental",
        "words": parse("""
szabadidő|N|свободное время|free time
hobbi|N|хобби|hobby
zene|N|музыка|music
gitár|N|гитара|guitar
zongora|N|пианино; рояль|piano
dal|N|песня|song
táncol|V|танцевать|to dance
tánc|N|танец|dance
fényképez|V|фотографировать|to photograph
fénykép|N|фотография|photograph
film|N|фильм|film
mozi|N|кино|cinema
színház|N|театр|theatre
sport|N|спорт|sport
futball|N|футбол|football
labda|N|мяч|ball
úszik|V|плавать|to swim
fut|V|бегать|to run
főz|V|готовить|to cook
sakk|N|шахматы|chess
gyűjt|V|собирать; коллекционировать|to collect
gyűjtemény|N|коллекция|collection
varr|V|шить|to sew
klub|N|клуб|club
edz|V|тренироваться|to train
verseny|N|соревнование|competition
meccs|N|матч|match
zenekar|N|оркестр; группа|orchestra, band
koncert|N|концерт|concert
rajz|N|рисунок|drawing
kiállítás|N|выставка|exhibition
könyvtár|N|библиотека|library
regény|N|роман|novel
vers|N|стихотворение|poem
újság|N|газета|newspaper
magazin|N|журнал|magazine
sorozat|N|сериал; серия|series
műsor|N|передача; программа|programme
tenisz|N|теннис|tennis
kosárlabda|N|баскетбол|basketball
bajnok|N|чемпион|champion
győz|V|побеждать|to win
veszít|V|проигрывать; терять|to lose
játékos|N|игрок|player
edző|N|тренер|coach
torna|N|гимнастика|gymnastics
fényképezőgép|N|фотоаппарат|camera
író|N|писатель|writer
költő|N|поэт|poet
színész|N|актёр|actor
énekes|N|певец|singer
zenész|N|музыкант|musician
festő|N|художник|painter
mese|N|сказка|fairy tale
buli|N|вечеринка|party
"""),
    },
]
