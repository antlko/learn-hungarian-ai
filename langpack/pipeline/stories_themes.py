"""Theme stories (see stories.py for the format and the checks).

STORIES = {unit_code: [[(hu, ru, en), ...], [...]]}. Fill it from
`python -m pipeline.stories PACK --brief UNIT`. AI-written, not checked by a
native speaker; every word is attested in the pack at build time.
"""

STORIES: dict[str, list[list[tuple[str, str, str]]]] = {
    "t-szamok": [
        [
            ('Anna néni és Bence a házban vannak.', 'Анна-нени и Бенце в доме.', 'Auntie Anna and Bence are in the house.'),
            ('Bence könyvet vesz, Anna néni a dobozt fogja.', 'Бенце покупает книги, а Анна-нени держит коробку.', 'Bence buys books and Auntie Anna holds the box.'),
            ('Egy könyv, két könyv, három könyv, négy könyv!', 'Одна книга, две книги, три книги, четыре книги!', 'One book, two books, three books, four books!'),
            ('Ez a negyedik könyv, az ötödik még az asztalon van.', 'Это четвёртая книга, а пятая ещё на столе.', 'This is the fourth book, the fifth is still on the table.'),
        ],
        [
            ('Hány könyv van a dobozban?', 'Сколько книг в коробке?', 'How many books are in the box?'),
            ('Tizenkettő! Az egy tucat.', 'Двенадцать! Это дюжина.', 'Twelve! That is a dozen.'),
            ('Egy tucat? Az tizenkettő, ugye?', 'Дюжина? Это двенадцать, да?', 'A dozen? That is twelve, right?'),
            ('Igen, Bence, egy tucat tizenkettő.', 'Да, Бенце, дюжина — это двенадцать.', 'Yes, Bence, a dozen is twelve.'),
        ],
        [
            ('Kétszer hat az tizenkettő.', 'Дважды шесть — двенадцать.', 'Twice six is twelve.'),
            ('Bence hat könyvet kap, Anna néni is hat könyvet.', 'Бенце получает шесть книг, и Анна-нени тоже шесть.', 'Bence gets six books, and Auntie Anna gets six too.'),
            ('Egyenlő! Hat páros szám, hét páratlan.', 'Поровну! Шесть — чётное число, семь — нечётное.', 'Equal! Six is an even number, seven is odd.'),
        ],
    ],
    "t-ido-ora": [
        [
            ('Az ébresztőóra reggelente hatkor szól.', 'Будильник по утрам звонит в шесть.', 'The alarm clock goes off at six every morning.'),
            ('Péter hat órakor ébred, de csak hét órakor kel fel.', 'Петер просыпается в шесть, а встаёт только в семь.', 'Péter wakes up at six but only gets up at seven.'),
            ('Ritkán kel fel hamar.', 'Он редко встаёт рано.', 'He rarely gets up early.'),
            ('Kata nevet, mert Péter gyakran késik.', 'Ката смеётся, потому что Петер часто опаздывает.', 'Kata laughs, because Péter is often late.'),
        ],
        [
            ('Péter a karórára néz.', 'Петер смотрит на часы.', 'Péter looks at his watch.'),
            ('Háromnegyed nyolc van!', 'Без четверти восемь!', 'It is a quarter to eight!'),
            ('A busz nyolc órakor indul, még negyedóra van.', 'Автобус в восемь, осталась ещё четверть часа.', 'The bus leaves at eight, there is still a quarter of an hour.'),
            ('A megálló közel van.', 'Остановка близко.', 'The stop is close.'),
            ('Péter gyorsan eszik, és megy.', 'Петер быстро ест и идёт.', 'Péter eats quickly and goes.'),
        ],
        [
            ('A busz pontos, Péter is pontos.', 'Автобус вовремя, и Петер тоже вовремя.', 'The bus is on time, and Péter is on time too.'),
            ('Kata mosolyog, mert hétköznap ez ritka.', 'Ката улыбается: в будни это редкость.', 'Kata smiles, because on weekdays this is rare.'),
            ('Holnap is pontos leszel, Péter?', 'Завтра ты тоже будешь вовремя, Петер?', 'Will you be on time tomorrow too, Péter?'),
        ],
    ],
    "t-napok-honapok": [
        [
            ("Eszter és Dani a naptárt nézik.", "Эстер и Дани смотрят на календарь.", "Eszter and Dani are looking at the calendar."),
            ("Itt a mai dátum.", "Вот сегодняшняя дата.", "Here is today's date."),
            ("Most november van, utána december jön, aztán január.", "Сейчас ноябрь, потом идёт декабрь, потом январь.", "It is November now, then comes December, then January."),
        ],
        [
            ("Dani szerint december a legszebb hónap.", "По мнению Дани, декабрь — самый красивый месяц.", "Dani thinks December is the nicest month."),
            ("Mert decemberben karácsony van!", "Ведь в декабре Рождество!", "Because Christmas is in December!"),
            ("Karácsonykor mindenki ünnepel.", "На Рождество все празднуют.", "At Christmas everyone celebrates."),
            ("Szilveszter az év utolsó napja, aztán jön az újév.", "Сильвестр — последний день года, а потом наступает Новый год.", "New Year's Eve is the last day of the year, then comes the new year."),
        ],
        [
            ("Dani ajándékot vár.", "Дани ждёт подарок.", "Dani is waiting for a present."),
            ("Eszter nevet, még sok nap van addig.", "Эстер смеётся: до этого ещё много дней.", "Eszter laughs, there are still many days to go."),
            ("Dani minden nap számol, egy nap, két nap, három nap.", "Дани каждый день считает: один день, два дня, три дня.", "Dani counts every day: one day, two days, three days."),
        ],
    ],
    "t-szinek": [
        [
            ('Lili fest, és anya segít.', 'Лили рисует, а мама помогает.', 'Lili is painting and Mum helps.'),
            ('Itt a festék, piros, kék és sárga.', 'Вот краски: красная, синяя и жёлтая.', 'Here is the paint: red, blue and yellow.'),
            ('Mit festesz, Lili?', 'Что ты рисуешь, Лили?', 'What are you painting, Lili?'),
            ('Szivárványt!', 'Радугу!', 'A rainbow!'),
        ],
        [
            ('Itt a piros, itt a sárga, itt a kék.', 'Вот красный, вот жёлтый, вот синий.', 'Here is the red, here is the yellow, here is the blue.'),
            ('Hol a zöld?', 'А где зелёный?', 'Where is the green?'),
            ('Nincs zöld festék, Lili.', 'Зелёной краски нет, Лили.', 'There is no green paint, Lili.'),
            ('De a sárga és a kék együtt zöld!', 'Но жёлтый и синий вместе дают зелёный!', 'But yellow and blue together make green!'),
        ],
        [
            ('Lili kéket tesz a sárgához.', 'Лили добавляет синий к жёлтому.', 'Lili adds blue to the yellow.'),
            ('Nézd, anya, zöld lett!', 'Смотри, мама, получился зелёный!', 'Look, Mum, it turned green!'),
            ('A kép színes és nagyon szép.', 'Картина яркая и очень красивая.', 'The picture is colourful and very pretty.'),
            ('Milyen szép szivárvány, Lili!', 'Какая красивая радуга, Лили!', 'What a lovely rainbow, Lili!'),
        ],
    ],
    "t-test": [
        [
            ('Reggel van, Marci mozog, anya is mozog.', 'Утро, Марци двигается, и мама тоже двигается.', 'It is morning, Marci is moving and Mum is moving too.'),
            ('Marci ugrik. Egy, kettő, három!', 'Марци прыгает. Раз, два, три!', 'Marci jumps. One, two, three!'),
            ('Aztán lép egyet jobbra, egyet balra.', 'Потом шагает вправо, потом влево.', 'Then he steps to the right, then to the left.'),
            ('Jó ez a mozgás!', 'Хорошая зарядка!', 'This is good exercise!'),
        ],
        [
            ('Most anya ül, Marci a földön fekszik.', 'Теперь мама сидит, а Марци лежит на полу.', 'Now Mum sits and Marci lies on the floor.'),
            ('Hol a bokád, Marci?', 'Где твоя лодыжка, Марци?', 'Where is your ankle, Marci?'),
            ('Itt a bokám, itt a combom!', 'Вот моя лодыжка, вот моё бедро!', 'Here is my ankle, here is my thigh!'),
            ('Itt a lábujjam, itt a tenyerem!', 'Вот мой палец на ноге, вот моя ладонь!', 'Here is my toe, here is my palm!'),
        ],
        [
            ('Nézd, anya, milyen erős az izmom!', 'Смотри, мама, какая у меня сильная мышца!', 'Look, Mum, how strong my muscle is!'),
            ('Nagyon erős vagy, Marci!', 'Ты очень сильный, Марци!', 'You are very strong, Marci!'),
        ],
    ],
    "t-idojaras": [
        [
            ('Reggel van, Dani az ablaknál áll.', 'Утро, Дани стоит у окна.', 'It is morning, and Dani stands at the window.'),
            ('Kint esik az eső.', 'На улице идёт дождь.', 'It is raining outside.'),
            ('Fúj a szél, és felhő van az égen.', 'Дует ветер, и на небе облака.', 'The wind is blowing and there are clouds in the sky.'),
            ('Hűvös van, nagyon hűvös.', 'Прохладно, очень прохладно.', 'It is cool, very cool.'),
        ],
        [
            ('Anya azt mondja, vedd fel a sálat és a sapkát!', 'Мама говорит: надень шарф и шапку!', 'Mum says, put on your scarf and your hat!'),
            ('Dani a kesztyűt is kéri.', 'Дани просит и перчатки.', 'Dani asks for his gloves too.'),
            ('Anya fogja az esernyőt, és mennek ki.', 'Мама берёт зонт, и они выходят.', 'Mum takes the umbrella and they go out.'),
            ('Kint nagy pocsolya van, és Dani ugrik bele.', 'На улице большая лужа, и Дани прыгает в неё.', 'There is a big puddle outside, and Dani jumps into it.'),
        ],
        [
            ('Egyszer csak nem esik az eső.', 'Вдруг дождь перестаёт.', 'Suddenly it stops raining.'),
            ('A felhő elmegy, süt a nap.', 'Облако уходит, светит солнце.', 'The cloud goes away and the sun shines.'),
            ('Napsütés van! Nem kell az esernyő.', 'Солнечно! Зонт не нужен.', 'It is sunny! No umbrella needed.'),
        ],
    ],
    "t-etel-ital": [
        [
            ('Szombat reggel van, Péter és Kata a piacon vannak.', 'Субботнее утро, Петер и Ката на рынке.', 'It is Saturday morning, and Péter and Kata are at the market.'),
            ('Itt az eper, piros és érett.', 'Вот клубника, красная и спелая.', 'Here are the strawberries, red and ripe.'),
            ('Ott a cseresznye, ott a szilva, ott a barack.', 'Там черешня, там слива, там персики.', 'There are the cherries, there the plums, there the peaches.'),
            ('Kata egy kiló epret kér.', 'Ката просит килограмм клубники.', 'Kata asks for a kilo of strawberries.'),
        ],
        [
            ('Kata eszik egy epret. Nagyon édes!', 'Ката съедает ягодку. Очень сладкая!', 'Kata eats a strawberry. Very sweet!'),
            ('Péter dinnyét akar, nagy, érett dinnyét.', 'Петер хочет арбуз, большой, спелый.', 'Péter wants a melon, a big, ripe melon.'),
            ('Ez a dinnye szép, de még nem érett.', 'Этот арбуз красивый, но ещё не спелый.', 'This melon is pretty, but it is not ripe yet.'),
            ('Ez a másik dinnye már érett!', 'А вот этот арбуз уже спелый!', 'This other melon is ripe already!'),
        ],
        [
            ('Péter mézet is kér a reggelihez.', 'Петер просит ещё мёд к завтраку.', 'Péter asks for some honey for breakfast too.'),
            ('Aztán cseresznyét is vesz.', 'А потом покупает и черешню.', 'Then he buys some cherries too.'),
            ('Úton haza Péter eszik egy cseresznyét.', 'По дороге домой Петер ест черешню.', 'On the way home Péter eats a cherry.'),
            ('Kata nevet, és ő is kér egyet.', 'Ката смеётся и тоже просит одну.', 'Kata laughs and asks for one too.'),
        ],
    ],
    "t-ruhazat": [
        [
            ('Reggel van, Bence öltözik, és anya segít.', 'Утро, Бенце одевается, а мама помогает.', 'It is morning, Bence is getting dressed and Mum helps.'),
            ('Itt a nadrág, itt az ing, itt a pulóver.', 'Вот брюки, вот рубашка, вот свитер.', 'Here are the trousers, here is the shirt, here is the jumper.'),
            ('Itt egy zokni. Hol a másik zokni?', 'Вот один носок. А где второй носок?', 'Here is one sock. Where is the other sock?'),
            ('Keressük, Bence!', 'Давай искать, Бенце!', "Let's look for it, Bence!"),
        ],
        [
            ('A cipő is kell, Bence!', 'Ботинки тоже нужны, Бенце!', 'The shoes are needed too, Bence!'),
            ('A pulóver nagy és kényelmes.', 'Свитер большой и удобный.', 'The jumper is big and comfortable.'),
            ('Kint hideg van, kell a kabát és a kalap.', 'На улице холодно, нужны куртка и шапка.', 'It is cold outside, so he needs a coat and a hat.'),
            ('A kabátnak nagy zsebe van.', 'У куртки большой карман.', 'The coat has a big pocket.'),
        ],
        [
            ('Ott van a másik zokni, a cipőben!', 'Вот и второй носок — в ботинке!', 'There is the other sock, in the shoe!'),
            ('Kész vagy, Bence, megyünk!', 'Ты готов, Бенце, идём!', "You are ready, Bence, let's go!"),
        ],
    ],
    "t-otthon-targyak": [
        [
            ('Anna és Péter az új lakásban vannak.', 'Анна и Петер в новой квартире.', 'Anna and Péter are in the new flat.'),
            ('Sok minden a földön van, de a hűtőszekrény már áll.', 'На полу много вещей, но холодильник уже стоит.', 'There is a lot of stuff on the floor, but the fridge is already standing.'),
            ('A konyhában a mosogató tiszta.', 'На кухне раковина чистая.', 'In the kitchen the sink is clean.'),
            ('Hol a seprű, Péter? Ott! És a vödör? Itt!', 'Где веник, Петер? Вот он! А ведро? Вот оно!', 'Where is the broom, Péter? There! And the bucket? Here!'),
        ],
        [
            ('Most a szobában vannak, ott az íróasztal és a könyvespolc.', 'Теперь они в комнате: там письменный стол и книжная полка.', 'Now they are in the room: there are the desk and the bookshelf.'),
            ('Péter az íróasztalt az ablak mellé teszi.', 'Петер ставит письменный стол к окну.', 'Péter puts the desk next to the window.'),
            ('Anna a festményt nézi. Hova tegye?', 'Анна смотрит на картину. Куда её повесить?', 'Anna looks at the painting. Where should she put it?'),
            ('A falra, a könyvespolc fölé!', 'На стену, над книжной полкой!', 'On the wall, above the bookshelf!'),
        ],
        [
            ('A virág a vázában van, a váza az íróasztalon.', 'Цветок в вазе, ваза на письменном столе.', 'The flower is in the vase, the vase is on the desk.'),
            ('Nézd, milyen szép lett!', 'Смотри, как красиво получилось!', 'Look how nice it has turned out!'),
        ],
    ],
    "t-utazas": [
        [
            ('Eszter és Tamás nyaralásra utaznak.', 'Эстер и Тамаш едут в отпуск.', 'Eszter and Tamás are going on holiday.'),
            ('A tóhoz mennek, ott van a szálloda.', 'Они едут к озеру, там гостиница.', 'They are going to the lake, where the hotel is.'),
            ('Most az állomáson vannak.', 'Сейчас они на вокзале.', 'Now they are at the station.'),
            ('A bőrönd nagy, a hátizsák kicsi.', 'Чемодан большой, рюкзак маленький.', 'The suitcase is big, the rucksack is small.'),
        ],
        [
            ('Tamás a menetrendet nézi.', 'Тамаш смотрит расписание.', 'Tamás looks at the timetable.'),
            ('A vonat az első peronról indul.', 'Поезд отправляется с первой платформы.', 'The train leaves from platform one.'),
            ('Itt a vonat! Az utazás két óra.', 'Вот поезд! Дорога займёт два часа.', 'Here is the train! The journey takes two hours.'),
            ('Eszter az ablaknál ül, nézi a hegyet és az erdőt.', 'Эстер сидит у окна и смотрит на гору и лес.', 'Eszter sits by the window and looks at the mountain and the forest.'),
        ],
        [
            ('Végre a tónál vannak.', 'Наконец они у озера.', 'At last they are by the lake.'),
            ('A szálloda közel van, és a strand is.', 'Гостиница рядом, и пляж тоже.', 'The hotel is close, and so is the beach.'),
            ('Eszter a napszemüveget keresi.', 'Эстер ищет солнечные очки.', 'Eszter is looking for her sunglasses.'),
            ('Itt van a hátizsákban!', 'Вот они, в рюкзаке!', 'Here they are in the rucksack!'),
            ('Nagyon szeretem a nyaralást, mondja Eszter.', '— Как же я люблю отпуск, — говорит Эстер.', '"I love holidays so much," says Eszter.'),
        ],
    ],
    "t-erzesek": [
        [
            ('Ma esik az eső, és Zsófi unatkozik.', 'Сегодня идёт дождь, и Жофи скучает.', 'It is raining today and Zsófi is bored.'),
            ('Nincs kedve olvasni, nincs kedve játszani.', 'Ей не хочется ни читать, ни играть.', 'She does not feel like reading or playing.'),
            ('Egyedül van otthon, és kicsit szomorú.', 'Она одна дома и немного грустит.', 'She is alone at home and a bit sad.'),
        ],
        [
            ('Egyszer csak jön Peti.', 'Вдруг приходит Пети.', 'All of a sudden Peti arrives.'),
            ('Szia, Zsófi! Mi a baj?', 'Привет, Жофи! Что случилось?', 'Hi, Zsófi! What is wrong?'),
            ('Nincs kedvem semmihez, mondja Zsófi.', '— Мне ничего не хочется, — говорит Жофи.', '"I do not feel like anything," says Zsófi.'),
            ('Peti lelkes, mert van egy ötlete.', 'Пети оживляется: у него есть идея.', 'Peti is enthusiastic, because he has an idea.'),
            ('Gyere, játszunk!', 'Пойдём играть!', "Come on, let's play!"),
        ],
        [
            ('Zsófi mosolyog, már nem szomorú és nem unatkozik.', 'Жофи улыбается: ей уже не грустно и не скучно.', 'Zsófi smiles, she is not sad or bored any more.'),
            ('Játszanak, és Zsófi nagyon jókedvű.', 'Они играют, и у Жофи отличное настроение.', 'They play, and Zsófi is in a very good mood.'),
            ('Zsófi szerencsés, mert van barátja.', 'Жофи повезло: у неё есть друг.', 'Zsófi is lucky, because she has a friend.'),
        ],
    ],
    "t-hobbi": [
        [
            ('Dani szereti a futballt.', 'Дани любит футбол.', 'Dani loves football.'),
            ('Ma meccs van, a labda már a pályán van.', 'Сегодня матч, мяч уже на поле.', 'Today there is a match, and the ball is already on the pitch.'),
            ('Az edző azt mondja, játékosok, gyerünk!', 'Тренер говорит: игроки, вперёд!', 'The coach says, come on, players!'),
        ],
        [
            ('Dani fut, és fut, és fut.', 'Дани бежит, и бежит, и бежит.', 'Dani runs, and runs, and runs.'),
            ('A másik csapat erős, de Dani csapata is erős.', 'Другая команда сильная, но и команда Дани сильная.', "The other team is strong, but Dani's team is strong too."),
            ('Dani ügyes, a labda a kapuban van!', 'Дани молодец, мяч в воротах!', 'Dani is skilful, and the ball is in the goal!'),
            ('A csapat győz, nem veszít!', 'Команда побеждает, а не проигрывает!', 'The team wins, it does not lose!'),
        ],
        [
            ('Anya és apa is ott van, nézik a meccset.', 'Мама и папа тоже там, смотрят матч.', 'Mum and Dad are there too, watching the match.'),
            ('Dani minden héten edz, ezért ügyes.', 'Дани тренируется каждую неделю, поэтому он молодец.', 'Dani trains every week, that is why he is skilful.'),
            ('Jövő héten újra meccs lesz!', 'На следующей неделе снова матч!', 'Next week there is another match!'),
        ],
    ],
}
