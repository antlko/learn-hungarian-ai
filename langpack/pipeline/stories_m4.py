"""Month 4 stories (see stories.py for the format and the checks).

STORIES = {unit_code: [[(hu, ru, en), ...], [...]]}: paragraphs of
(Hungarian, Russian, English) sentences. AI-written, not checked by a native
speaker; every word is attested in the pack at build time.
"""

STORIES: dict[str, list[list[tuple[str, str, str]]]] = {
    "m4-u1-munka": [
        [
            ("Bence munkát keres.", "Бенце ищет работу.", "Bence is looking for a job."),
            ("Talál egy hirdetést az újságban.", "Он находит объявление в газете.", "He finds a listing in the paper."),
            ("Egy kis cég új munkavállalót keres.", "Небольшая фирма ищет нового работника.", "A small company is looking for a new employee."),
            ("Ez állandó állás az irodában.", "Это постоянная должность в офисе.", "It is a steady position in the office."),
        ],
        [
            ("Bence jelentkezik.", "Бенце откликается.", "Bence applies."),
            ("Ír egy önéletrajzot.", "Он пишет резюме.", "He writes a CV."),
            ("Van diplomája, de nincs sok tapasztalata.", "Диплом у него есть, а опыта мало.", "He has a degree, but not much experience."),
        ],
        [
            ("Hétfőn van az állásinterjú.", "В понедельник собеседование.", "The interview is on Monday."),
            ("A vezető nagyon kedves.", "Руководитель очень приятный.", "The manager is very nice."),
            ("A munkaidő reggel nyolctól négyig tart.", "Рабочее время с восьми утра до четырёх.", "Working hours are from eight to four."),
            ("Túlóra nincs, a bér jó.", "Сверхурочных нет, зарплата хорошая.", "There is no overtime, and the pay is good."),
        ],
        [
            ("Pár nap múlva hív a vezető.", "Через пару дней руководитель ему звонит.", "A few days later the manager phones him."),
            ("Bence kap egy állást! Nagyon örül.", "Бенце получает работу! Он очень рад.", "Bence gets the job! He is very happy."),
        ],
    ],
    "m4-u2-alberlet": [
        [
            ("Réka albérletet keres.", "Река ищет съёмную квартиру.", "Réka is looking for a flat to rent."),
            ("Dani kiadja a lakást.", "Дани сдаёт свою квартиру.", "Dani is letting his flat."),
            ("Ez egy világos, tágas lakás.", "Это светлая, просторная квартира.", "This is a bright, spacious flat."),
            ("Hány négyzetméter? Elég sok, és van erkély is.", "Сколько квадратных метров? Довольно много, и ещё есть балкон.", "How many square metres? Quite a lot, and there is a balcony too."),
        ],
        [
            ("Réka bérel egy lakást a társasházban.", "Река снимает квартиру в многоквартирном доме.", "Réka rents a flat in the block."),
            ("Az udvar szép, a pince is jó.", "Двор красивый, подвал тоже хороший.", "The yard is nice, and the cellar is good too."),
            ("A bérleti szerződés egy évre szól.", "Арендный договор заключён на один год.", "The rental contract is for one year."),
            ("A letét egy hónap, a közös költség külön.", "Залог за один месяц, общие расходы отдельно.", "The deposit is one month, the shared costs are separate."),
        ],
        [
            ("Dani átadja a kulcsot.", "Дани передаёт ключ.", "Dani hands over the key."),
            ("Együtt nézik a villanyórát.", "Вместе они смотрят на электросчётчик.", "Together they look at the electricity meter."),
            ("Mindent a jegyzőkönyvbe írnak.", "Всё записывают в акт.", "They write everything into the handover record."),
            ("Réka még ma beköltözik.", "Река въезжает уже сегодня.", "Réka moves in today."),
        ],
    ],
    "m4-u3-csalad": [
        [
            ("Zsuzsa az iskola előtt vár.", "Жужа ждёт у школы.", "Zsuzsa is waiting in front of the school."),
            ("Gábor, a kollégája, is ott áll.", "Габор, её коллега, тоже стоит там.", "Gábor, her colleague, is standing there too."),
            ("Zsuzsa az unokára vár.", "Жужа ждёт своего внука.", "Zsuzsa is waiting for her grandchild."),
            ("Gábor a kisfiút várja.", "Габор ждёт своего сына.", "Gábor is waiting for his son."),
            ("Zsuzsa nagymama, Gábor pedig apa.", "Жужа бабушка, а Габор отец.", "Zsuzsa is a grandmother, and Gábor is a dad."),
        ],
        [
            ("Jön az unokája! Alacsony, szőke és sovány.", "Идёт её внук! Невысокий, светловолосый и худенький.", "Here comes her grandchild! Short, blond and slim."),
            ("Nagymama, szia! Anya is jön?", "Бабушка, привет! Мама тоже придёт?", "Hi, Grandma! Is Mum coming too?"),
            ("Nem, anya dolgozik, de apa jön érted.", "Нет, мама работает, но папа придёт за тобой.", "No, Mum is working, but Dad is coming for you."),
        ],
        [
            ("Gábor kopasz, de van szakálla.", "Габор лысый, но у него есть борода.", "Gábor is bald, but he has a beard."),
            ("Gábor fia barna, és nagyon hasonlít rá.", "Сын Габора тёмноволосый и очень похож на него.", "Gábor's son is dark-haired and looks a lot like him."),
            ("Zsuzsa nevet, mert jó a család!", "Жужа смеётся: хорошо, когда есть семья!", "Zsuzsa laughs, because family is a good thing!"),
        ],
    ],
    "m4-u4-multido": [
        [
            ("Tamás felhívja Juditot, a vezetőt.", "Тамаш звонит Юдит, руководителю.", "Tamás phones Judit, the manager."),
            ("Bocsánat, Judit, tegnap késtem.", "Простите, Юдит, вчера я опоздал.", "Sorry, Judit, I was late yesterday."),
            ("Mi történt, Tamás?", "Что случилось, Тамаш?", "What happened, Tamás?"),
        ],
        [
            ("Reggel kerestem a kulcsot.", "Утром я искал ключ.", "In the morning I was looking for my key."),
            ("Sokáig kerestem, de nem találtam.", "Долго искал, но не нашёл.", "I looked for a long time, but I did not find it."),
            ("Azt hittem, az asztalon van!", "Я думал, он лежит на столе!", "I thought it was on the table!"),
            ("Közben a busz elment.", "Тем временем автобус уехал.", "Meanwhile the bus left."),
        ],
        [
            ("Végül Anna, a feleségem, megtalálta a kulcsot.", "В конце концов Анна, моя жена, нашла ключ.", "In the end Anna, my wife, found the key."),
            ("A táskában volt! De akkor már késő volt.", "Он был в сумке! Но тогда было уже поздно.", "It was in the bag! But by then it was too late."),
            ("Azóta a kulcs mindig a táskában van.", "С тех пор ключ всегда лежит в сумке.", "Since then the key is always in the bag."),
            ("Nem baj, Tamás, szerencsére ma itt vagy.", "Ничего, Тамаш, к счастью, сегодня ты здесь.", "Never mind, Tamás, fortunately you are here today."),
        ],
    ],
}
