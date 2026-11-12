"""Month 5 stories (see stories.py for the format and the checks).

STORIES = {unit_code: [[(hu, ru, en), ...], [...]]}. AI-written, not checked by a
native speaker; every word is attested in the pack at build time.
"""

STORIES: dict[str, list[list[tuple[str, str, str]]]] = {
    "m5-u1-elromlott": [
        [
            ("Kati otthon van.", "Кати дома.", "Kati is at home."),
            ("Hallja, hogy csöpög a csap.", "Она слышит, что капает кран.", "She hears the tap dripping."),
            ("Csöpög, csöpög, csöpög!", "Капает, капает, капает!", "Drip, drip, drip!"),
            ("Folyik a víz, a csap rossz.", "Течёт вода, кран сломан.", "Water is running, the tap is broken."),
        ],
        [
            ("Kati hívja Gábort, a szerelőt.", "Кати звонит Габору, мастеру.", "Kati calls Gábor, the repairman."),
            ("Gábor azonnal jön.", "Габор приходит сразу.", "Gábor comes right away."),
            ("Hol a gond? Itt van? Ott van?", "В чём проблема? Здесь? Там?", "Where is the problem? Here? There?"),
            ("Itt van! Ez a csap rossz.", "Вот она! Этот кран сломан.", "Here it is! This tap is broken."),
        ],
        [
            ("Gábor megnézi a csapot.", "Габор осматривает кран.", "Gábor takes a look at the tap."),
            ("Kati kérdez.", "Кати спрашивает.", "Kati asks."),
            ("Kell egy új alkatrész?", "Нужна новая запчасть?", "Does it need a new part?"),
            ("Gábor felel.", "Габор отвечает.", "Gábor answers."),
            ("Igen, de van garancia, ez jó.", "Да, но есть гарантия, это хорошо.", "Yes, but there is a warranty, which is good."),
            ("Na, most már nem csöpög!", "Ну вот, теперь не капает!", "There, now it is not dripping!"),
        ],
    ],
    "m5-u2-idopont": [
        [
            ("Anna telefonál.", "Анна звонит по телефону.", "Anna is making a phone call."),
            ("Csörög, csörög, csörög.", "Звонок, звонок, звонок.", "It rings and rings and rings."),
            ("Foglalt. Anna vár.", "Занято. Анна ждёт.", "It's busy. Anna waits."),
            ("Aztán megint telefonál.", "Потом снова звонит.", "Then she calls again."),
        ],
        [
            ("Végre egy kedves hang válaszol.", "Наконец отвечает приятный голос.", "At last a pleasant voice answers."),
            ("Halló! Jó napot kívánok!", "Алло! Добрый день!", "Hello! Good afternoon!"),
            ("Jó napot! Időpontot szeretnék kérni.", "Добрый день! Я хотела бы записаться на приём.", "Good afternoon! I would like to make an appointment."),
            ("Szerdán ráér, vagy inkább csütörtökön?", "В среду вы свободны или лучше в четверг?", "Are you free on Wednesday, or rather Thursday?"),
        ],
        [
            ("Szerdán nekem pont jó.", "В среду мне как раз удобно.", "Wednesday suits me just fine."),
            ("Rendben, szerdán pontosan nyolc órakor!", "Хорошо, в среду ровно в восемь часов!", "All right, on Wednesday at exactly eight o'clock!"),
            ("Ha Anna nem ér rá, szól.", "Если Анна не сможет, она предупредит.", "If Anna can't make it, she will let them know."),
        ],
    ],
    "m5-u3-ovoda-hivatal": [
        [
            ("Ma van az óvoda első napja.", "Сегодня первый день в детском саду.", "Today is the first day of kindergarten."),
            ("Éva a kislányt, Zsófit hozza.", "Эва ведёт свою малышку Жофи.", "Éva is bringing her little girl Zsófi."),
            ("Zsófi szeret játszani és énekelni.", "Жофи любит играть и петь.", "Zsófi loves to play and sing."),
            ("Itt a játék! Ott a játék!", "Вот игрушка! Вот ещё игрушка!", "Here is a toy! There is a toy!"),
        ],
        [
            ("Az óvónő jön, és köszön.", "Приходит воспитательница и здоровается.", "The teacher comes and says hello."),
            ("Jó napot! Szia, Zsófi!", "Добрый день! Привет, Жофи!", "Good afternoon! Hi, Zsófi!"),
            ("Az óvónő kér egy másolatot az igazolványról.", "Воспитательница просит копию удостоверения.", "The teacher asks for a copy of the ID."),
            ("Nincs gond, itt van.", "Не проблема, вот она.", "No problem, here it is."),
        ],
        [
            ("Az óvónő nézi a gyerekeket.", "Воспитательница смотрит на детей.", "The teacher watches the children."),
            ("Zsófi már nem fél, hanem játszik.", "Жофи уже не боится, а играет.", "Zsófi isn't scared any more, she is playing."),
            ("Éva örül, mert Zsófi jól van.", "Эва рада, потому что у Жофи всё хорошо.", "Éva is happy because Zsófi is fine."),
        ],
    ],
    "m5-u4-etterem": [
        [
            ("Ma Péter és Eszter az étteremben van.", "Сегодня Петер и Эстер в ресторане.", "Today Péter and Eszter are in the restaurant."),
            ("Éhesek, és nagyon szomjasak.", "Они голодные и очень хотят пить.", "They are hungry and very thirsty."),
            ("Itt a pincér, és hozza az étlapot.", "Подходит официант и приносит меню.", "The waiter comes and brings the menu."),
        ],
        [
            ("Péter levest kér, mert hideg van.", "Петер просит суп, потому что холодно.", "Péter asks for soup because it is cold."),
            ("Eszter rántott húst kér.", "Эстер просит шницель.", "Eszter asks for breaded meat."),
            ("Péter bort iszik, Eszter csak vizet.", "Петер пьёт вино, Эстер только воду.", "Péter drinks wine, Eszter only water."),
            ("Ízlik? Nagyon finom!", "Вкусно? Очень вкусно!", "Do you like it? Delicious!"),
        ],
        [
            ("Aztán jön a desszert, a palacsinta!", "Потом десерт, блинчики!", "Then comes dessert, the pancakes!"),
            ("Eszter szereti az édes palacsintát.", "Эстер любит сладкие блинчики.", "Eszter likes sweet pancakes."),
            ("Péter fizet, Eszter pedig szépen köszön.", "Петер платит, а Эстер мило благодарит.", "Péter pays, and Eszter thanks him nicely."),
        ],
    ],
}
