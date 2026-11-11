"""Theme dialogues: numbers, time, dates, colours, the body, weather, food,
clothes, home things, travel, moods, hobbies. Several are the kind of talk a
child hears while learning to speak (counting, colours, «hol az orrod?»).

AI-written, not checked by a native speaker (see dialogues.py).
Each line: (Hungarian, Russian, English).
"""

DIALOGUES = {
    # Counting with a child.
    "t-szamok": [
        ("Gyere, mondjuk együtt! Egy, kettő, három!", "Давай скажем вместе! Раз, два, три!", "Come on, let's say it together! One, two, three!"),
        ("Négy, öt, hat!", "Четыре, пять, шесть!", "Four, five, six!"),
        ("Nagyon ügyes vagy! Hét, nyolc, kilenc!", "Какой ты молодец! Семь, восемь, девять!", "Well done! Seven, eight, nine!"),
        ("Tíz! Jó vagyok?", "Десять! Я молодец?", "Ten! Am I good?"),
        ("Igen! Mi jön tizenkettő után?", "Да! Что идёт после двенадцати?", "Yes! What comes after twelve?"),
        ("Tizenhárom!", "Тринадцать!", "Thirteen!"),
        ("Okos vagy! Hány alma van itt?", "Умница! Сколько здесь яблок?", "Clever! How many apples are here?"),
        ("Egy, kettő, három, négy, öt. Öt alma!", "Раз, два, три, четыре, пять. Пять яблок!", "One, two, three, four, five. Five apples!"),
        ("Igen, öt. Mennyi kétszer öt?", "Да, пять. Сколько будет дважды пять?", "Yes, five. What is twice five?"),
        ("Tíz! Kétszer kettő négy!", "Десять! Дважды два — четыре!", "Ten! Twice two is four!"),
        ("Ügyes vagy! Most egyél egy almát.", "Молодец! А теперь съешь яблоко.", "Well done! Now eat an apple."),
    ],
    # Fixing a time to meet.
    "t-ido-ora": [
        ("Szia! Mikor találkozunk ma?", "Привет! Когда мы сегодня встретимся?", "Hi! When shall we meet today?"),
        ("Délután. Három órakor jó?", "После обеда. В три часа подойдёт?", "In the afternoon. Is three o'clock good?"),
        ("Három órakor még dolgozom. Fél négy jó?", "В три я ещё работаю. А в половине четвёртого?", "At three I'm still working. Is half past three good?"),
        ("Fél négy jó. Hol?", "Половина четвёртого подходит. Где?", "Half past three is fine. Where?"),
        ("A megállónál. Pontos leszel?", "На остановке. Ты будешь вовремя?", "At the stop. Will you be on time?"),
        ("Persze! Mindig pontos vagyok.", "Конечно! Я всегда вовремя.", "Of course! I'm always on time."),
        ("Nem mindig! Tegnap tíz percet késtél.", "Не всегда! Вчера ты опоздал на десять минут.", "Not always! Yesterday you were ten minutes late."),
        ("Igaz, de a busz késett. Ma korán indulok.", "Правда, но автобус опоздал. Сегодня я выйду пораньше.", "True, but the bus was late. Today I'll set off early."),
        ("Rendben. Szólj, ha késel!", "Хорошо. Скажи, если опоздаешь!", "All right. Tell me if you're late!"),
        ("Szólok. Szia, délután találkozunk!", "Скажу. Пока, увидимся днём!", "I will. Bye, see you this afternoon!"),
    ],
    # Birthdays, seasons, the weekend.
    "t-napok-honapok": [
        ("Szia! Milyen nap van ma?", "Привет! Какой сегодня день?", "Hi! What day is it today?"),
        ("Ma péntek van. Holnap már hétvége!", "Сегодня пятница. Завтра уже выходные!", "It's Friday today. Tomorrow it's already the weekend!"),
        ("Jó! Mit csinálsz a hétvégén?", "Здорово! Что ты делаешь на выходных?", "Great! What are you doing at the weekend?"),
        ("Szombaton dolgozom, de vasárnap szabad vagyok.", "В субботу я работаю, а в воскресенье свободен.", "On Saturday I work, but on Sunday I'm free."),
        ("Akkor gyere hozzánk vasárnap! Anna születésnapja van.", "Тогда приходи к нам в воскресенье! У Анны день рождения.", "Then come to ours on Sunday! It's Anna's birthday."),
        ("Tényleg? Mikor van a te születésnapod?", "Правда? А когда твой день рождения?", "Really? When is your birthday?"),
        ("Októberben. És a tied?", "В октябре. А твой?", "In October. And yours?"),
        ("Márciusban. Az már tavasz!", "В марте. Это уже весна!", "In March. That's already spring!"),
        ("Igen, szeretem a tavaszt. Karácsonykor mit csinálsz?", "Да, я люблю весну. Что ты делаешь на Рождество?", "Yes, I like spring. What do you do at Christmas?"),
        ("Otthon vagyok a családdal.", "Я дома, с семьёй.", "I'm at home with my family."),
    ],
    # Colours, a child's game.
    "t-szinek": [
        ("Mi a kedvenc színed?", "Какой твой любимый цвет?", "What's your favourite colour?"),
        ("A kék. És a tiéd?", "Синий. А твой?", "Blue. And yours?"),
        ("Az enyém a zöld. Nézd, a fű is zöld!", "Мой — зелёный. Смотри, трава тоже зелёная!", "Mine is green. Look, the grass is green too!"),
        ("Igen! És az ég kék.", "Да! А небо синее.", "Yes! And the sky is blue."),
        ("A nap sárga vagy kék?", "Солнце жёлтое или синее?", "Is the sun yellow or blue?"),
        ("Sárga! És a tej?", "Жёлтое! А молоко?", "Yellow! And the milk?"),
        ("A tej fehér, a kenyér barna.", "Молоко белое, хлеб коричневый.", "Milk is white, bread is brown."),
        ("Barna? Nem fekete?", "Коричневый? Не чёрный?", "Brown? Not black?"),
        ("Nem, a kávé fekete. Szeretem a piros almát is.", "Нет, чёрный — это кофе. Ещё я люблю красные яблоки.", "No, coffee is black. I like red apples too."),
        ("Én is! Gyere, játszunk!", "Я тоже! Пойдём играть!", "Me too! Come on, let's play!"),
    ],
    # «Hol az orrod?» — a game with a small child.
    "t-test": [
        ("Hol van a fejed? Itt van!", "Где твоя голова? Вот она!", "Where's your head? Here it is!"),
        ("Itt! És hol van az orrod?", "Вот! А где твой нос?", "Here! And where's your nose?"),
        ("Itt az orrom! Ez a szemem.", "Вот мой нос! Это мой глаз.", "Here's my nose! This is my eye."),
        ("Nagyon jó! És hol van a szád?", "Отлично! А где твой рот?", "Very good! And where's your mouth?"),
        ("A szám itt van. Ez a kezem.", "Мой рот вот здесь. Это моя рука.", "My mouth is here. This is my hand."),
        ("Hány kezed van?", "Сколько у тебя рук?", "How many hands do you have?"),
        ("Kettő! Egy itt, egy ott.", "Две! Одна здесь, одна там.", "Two! One here, one there."),
        ("És hány lábad van?", "А сколько у тебя ног?", "And how many legs do you have?"),
        ("Az is kettő. Nézd, a hasam is!", "Тоже две. Смотри, и живот!", "Also two. Look, and my tummy too!"),
        ("Ügyes vagy! Most fáj valami?", "Молодец! Сейчас что-нибудь болит?", "Well done! Does anything hurt now?"),
        ("Nem, semmi sem fáj.", "Нет, ничего не болит.", "No, nothing hurts."),
    ],
    # A chat at the bus stop.
    "t-idojaras": [
        ("Szia! Milyen idő van ma?", "Привет! Какая сегодня погода?", "Hi! What's the weather like today?"),
        ("Hát, esik az eső, és fúj a szél.", "Ну, идёт дождь и дует ветер.", "Well, it's raining and the wind is blowing."),
        ("Jaj! Nincs esernyőm.", "Ой! У меня нет зонта.", "Oh! I don't have an umbrella."),
        ("Nekem van. Gyere ide alá!", "У меня есть. Иди сюда под зонт!", "I do. Come under here!"),
        ("Köszönöm! Holnap is esik?", "Спасибо! Завтра тоже будет дождь?", "Thank you! Will it rain tomorrow too?"),
        ("Nem, holnap süt a nap, de hideg van.", "Нет, завтра будет солнце, но холодно.", "No, tomorrow the sun will shine, but it's cold."),
        ("Ma is hideg van. Nekem nagyon hideg van!", "Сегодня тоже холодно. Мне очень холодно!", "It's cold today too. I'm very cold!"),
        ("Tél van, nem nyár. Hol a sál és a sapka?", "Сейчас зима, а не лето. Где шарф и шапка?", "It's winter, not summer. Where are your scarf and hat?"),
        ("Otthon vannak. Hó is lesz?", "Дома. А снег будет?", "They're at home. Will there be snow?"),
        ("Talán. Itt jön a busz!", "Может быть. Вот идёт автобус!", "Maybe. Here comes the bus!"),
    ],
    # Lunch at a friend's home.
    "t-etel-ital": [
        ("Gyere, kész az ebéd! Éhes vagy?", "Иди, обед готов! Ты голоден?", "Come, lunch is ready! Are you hungry?"),
        ("Nagyon! Mi van ma?", "Очень! Что сегодня?", "Very! What is there today?"),
        ("Gulyás. És utána sütemény.", "Гуляш. А потом сладкое.", "Goulash. And cake after that."),
        ("Szeretem a gulyást! Hol a kenyér?", "Люблю гуляш! Где хлеб?", "I love goulash! Where's the bread?"),
        ("Itt van. Kérsz sót?", "Вот он. Хочешь соли?", "Here it is. Do you want salt?"),
        ("Nem, köszönöm. Így is jó. Hol a víz?", "Нет, спасибо. И так хорошо. Где вода?", "No, thank you. It's good as it is. Where's the water?"),
        ("Itt van. Hozok még kenyeret is.", "Вот. Я ещё принесу хлеба.", "Here it is. I'll bring some more bread too."),
        ("Köszönöm! Nagyon finom!", "Спасибо! Очень вкусно!", "Thank you! It's very tasty!"),
        ("Örülök! Kérsz még?", "Рада! Хочешь ещё?", "I'm glad! Would you like more?"),
        ("Igen, kérek még. Utána jöhet a sütemény!", "Да, ещё. А потом можно сладкое!", "Yes, more please. Then the cake can come!"),
    ],
    # Trying on a coat.
    "t-ruhazat": [
        ("Jó napot! Egy kabátot keresek.", "Добрый день! Я ищу пальто.", "Good day! I'm looking for a coat."),
        ("Tessék! Milyen szín tetszik?", "Пожалуйста! Какой цвет вам нравится?", "Here you are! What colour do you like?"),
        ("Fekete vagy piros. Ez szép!", "Чёрное или красное. Это красивое!", "Black or red. This one is nice!"),
        ("Itt az öltöző, tessék.", "Вот примерочная, пожалуйста.", "Here's the fitting room, please."),
        ("Igen. Hmm, ez a kabát nagyon szűk.", "Да. Хм, это пальто очень тесное.", "Yes. Hmm, this coat is very tight."),
        ("Van nagyobb is. Tessék.", "Есть и побольше. Пожалуйста.", "There's a bigger one too. Here you are."),
        ("Ez jó! Kényelmes és meleg.", "Это хорошо! Удобное и тёплое.", "This is good! Comfortable and warm."),
        ("Nagyon jól áll önnek.", "Вам очень идёт.", "It suits you very well."),
        ("Köszönöm. Mennyibe kerül?", "Спасибо. Сколько оно стоит?", "Thank you. How much does it cost?"),
        ("Ez nem drága. Csak ezer forint.", "Это недорого. Всего тысяча форинтов.", "It's not expensive. Only a thousand forints."),
        ("Rendben, ezt kérem.", "Хорошо, я беру это.", "All right, I'll take this."),
    ],
    # Moving things around in a flat.
    "t-otthon-targyak": [
        ("Hol van a kulcsom? Nem találom!", "Где мой ключ? Я не могу его найти!", "Where's my key? I can't find it!"),
        ("Nézd meg az asztalon.", "Посмотри на столе.", "Look on the table."),
        ("Ott nincs. Talán a szekrényben van.", "Там нет. Может, в шкафу.", "Not there. Maybe it's in the wardrobe."),
        ("Itt van egy kulcs a polcon.", "Вот ключ на полке.", "There's a key here on the shelf."),
        ("Az nem az enyém. Az a szekrény kulcsa.", "Это не мой. Это ключ от шкафа.", "That's not mine. That's the wardrobe key."),
        ("Nézd, a szék alatt van valami!", "Смотри, под стулом что-то лежит!", "Look, there's something under the chair!"),
        ("Hol? Aha, itt van! Köszönöm!", "Где? Ага, вот он! Спасибо!", "Where? Aha, here it is! Thank you!"),
        ("Nincs mit! Most pedig tegyük el a virágot.", "Не за что! А теперь уберём цветок.", "No problem! And now let's put the flower away."),
        ("Hova tegyük? A polcra?", "Куда поставим? На полку?", "Where shall we put it? On the shelf?"),
        ("Igen, a polcra. Jó lesz ott.", "Да, на полку. Там будет хорошо.", "Yes, on the shelf. It'll be fine there."),
    ],
    # At the airport.
    "t-utazas": [
        ("Jó napot! Hova utazik?", "Добрый день! Куда вы летите?", "Good day! Where are you travelling to?"),
        ("Jó napot! A tengerpartra utazom.", "Добрый день! Я еду на море.", "Good day! I'm travelling to the seaside."),
        ("Nagyon szép! Hol a bőröndje?", "Прекрасно! Где ваш чемодан?", "Lovely! Where is your suitcase?"),
        ("Itt van. A hátizsák is nálam van.", "Вот он. Рюкзак тоже со мной.", "Here it is. I have the backpack with me too."),
        ("Köszönöm. Itt a jegye. A repülő délben indul.", "Спасибо. Вот ваш билет. Самолёт вылетает в полдень.", "Thank you. Here's your ticket. The plane leaves at noon."),
        ("Mikor kell a kapuhoz menni?", "Когда нужно идти к выходу на посадку?", "When do I have to go to the gate?"),
        ("Egy óra múlva. Addig van még idő.", "Через час. До этого ещё есть время.", "In an hour. There's still time until then."),
        ("Rendben. És hol a kapu?", "Хорошо. А где выход на посадку?", "All right. And where's the gate?"),
        ("Ott, balra. Jó utat!", "Там, налево. Счастливого пути!", "Over there, on the left. Have a good trip!"),
        ("Köszönöm! Viszlát!", "Спасибо! До свидания!", "Thank you! Bye!"),
    ],
    # How are you, really?
    "t-erzesek": [
        ("Szia! Hogy vagy ma?", "Привет! Как ты сегодня?", "Hi! How are you today?"),
        ("Hát, fáradt vagyok és álmos.", "Ну, я устал и хочу спать.", "Well, I'm tired and sleepy."),
        ("Miért? Nem alszol jól?", "Почему? Ты плохо спишь?", "Why? Aren't you sleeping well?"),
        ("Nem. Nagyon izgatott vagyok.", "Нет. Я очень взволнован.", "No. I'm very nervous."),
        ("Mi a baj? Valami rossz történt?", "Что не так? Случилось что-то плохое?", "What's wrong? Did something bad happen?"),
        ("Nem, csak lelkes vagyok. Holnap kezdek új munkát!", "Нет, я просто воодушевлён. Завтра начинаю новую работу!", "No, I'm just excited. Tomorrow I start my new job!"),
        ("Nagyon jó! Én is örülök neked.", "Отлично! Я тоже рада за тебя.", "Great! I'm happy for you too."),
        ("Köszönöm! Kicsit félek is.", "Спасибо! Но мне немного и страшно.", "Thank you! I'm a bit scared too."),
        ("Ez normális. Szerencsés vagy!", "Это нормально. Тебе повезло!", "That's normal. You're lucky!"),
        ("Igen, tényleg szerencsés vagyok. Köszönöm!", "Да, мне действительно повезло. Спасибо!", "Yes, I really am lucky. Thank you!"),
    ],
    # Free time on a Friday evening.
    "t-hobbi": [
        ("Szia! Mit csinálsz szabadidőben?", "Привет! Что ты делаешь в свободное время?", "Hi! What do you do in your free time?"),
        ("Szeretek zenét hallgatni és gitáron játszani.", "Я люблю слушать музыку и играть на гитаре.", "I like listening to music and playing the guitar."),
        ("Nagyon jó! Milyen zenét szeretsz?", "Здорово! Какую музыку ты любишь?", "Great! What kind of music do you like?"),
        ("Mindenfélét. Dalokat is hallgatok. És te?", "Всякую. И песни слушаю. А ты?", "All kinds. I listen to songs too. And you?"),
        ("Én futok és úszom. Szeretem a sportot.", "Я бегаю и плаваю. Я люблю спорт.", "I run and swim. I like sport."),
        ("Minden nap futsz?", "Ты бегаешь каждый день?", "Do you run every day?"),
        ("Nem, csak hétvégén. Akkor meccs is van.", "Нет, только по выходным. Тогда ещё бывает матч.", "No, only at the weekend. Then there's a match too."),
        ("Jó! Moziba is jársz?", "Здорово! А в кино ходишь?", "Nice! Do you go to the cinema too?"),
        ("Néha. Tegnap láttam egy jó filmet.", "Иногда. Вчера я видел хороший фильм.", "Sometimes. Yesterday I saw a good film."),
        ("Hogy hívják a filmet? Én is megnézem.", "Как называется фильм? Я тоже посмотрю.", "What's the film called? I'll watch it too."),
    ],
}
