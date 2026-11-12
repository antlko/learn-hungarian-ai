"""Month 6 stories (see stories.py for the format and the checks).

STORIES = {unit_code: [[(hu, ru, en), ...], [...]]}. AI-written, not checked by a
native speaker; every word is attested in the pack at build time. Regenerate the
brief with `python -m pipeline.stories PACK --brief UNIT`.
"""

STORIES: dict[str, list[list[tuple[str, str, str]]]] = {
    "m6-u1-velemeny": [
        [
            ("Éva és Gábor együtt dolgozik.", "Эва и Габор работают вместе.", "Éva and Gábor work together."),
            ("Ma ebédre keresnek egy helyet.", "Сегодня они ищут, где пообедать.", "Today they are looking for a place for lunch."),
            ("Gábor szerint az új hely jó.", "По мнению Габора, новое место хорошее.", "According to Gábor, the new place is good."),
            ("Éva gondolkodik.", "Эва задумывается.", "Éva stops to think."),
        ],
        [
            ("Szerintem ott sokat kell várni.", "По-моему, там приходится долго ждать.", "I think you have to wait a lot there."),
            ("Gábor nevet.", "Габор смеётся.", "Gábor laughs."),
            ("Dehogy, ez nem igaz!", "Да нет, это неправда!", "Not at all, that is not true!"),
            ("Tegnap is ott voltam, gyors volt.", "Вчера я тоже там был, было быстро.", "I was there yesterday too, and it was quick."),
        ],
        [
            ("Éva szerint ez egy jó érv.", "По мнению Эвы, это хороший довод.", "Éva thinks that is a good argument."),
            ("Akkor tévedtem, igazad van.", "Тогда я ошиблась, ты прав.", "Then I was wrong, you are right."),
            ("Nincs vita, menjünk!", "Спорить не о чем, идём!", "No argument, let's go!"),
            ("Gábor nagyon örül, Éva is.", "Габор очень рад, Эва тоже.", "Gábor is very glad, and so is Éva."),
        ],
    ],
    "m6-u2-erzesek": [
        [
            ("Anna otthon van, Peti is.", "Анна дома, Пети тоже.", "Anna is at home, and so is Peti."),
            ("Peti ma szomorú, fáradt, és egy kicsit mérges is.", "Пети сегодня грустный, усталый и немного сердитый.", "Peti is sad and tired today, and a little cross too."),
            ("Anna nézi, és aggódik.", "Анна смотрит на него и беспокоится.", "Anna looks at him and worries."),
        ],
        [
            ("Anna kérdez.", "Анна спрашивает.", "Anna asks."),
            ("Mi a baj, Peti?", "Что случилось, Пети?", "What is wrong, Peti?"),
            ("Peti felel.", "Пети отвечает.", "Peti answers."),
            ("Nem érzem jól magam.", "Мне нехорошо.", "I don't feel well."),
            ("Peti sír egy kicsit.", "Пети немного плачет.", "Peti cries a little."),
            ("Anna ölel, és vigasztal.", "Анна обнимает и утешает его.", "Anna hugs him and comforts him."),
        ],
        [
            ("Semmi baj, itt vagyok.", "Ничего страшного, я рядом.", "It is all right, I am here."),
            ("Peti is mosolyog.", "Пети тоже улыбается.", "Peti smiles too."),
            ("Már nem vagyok szomorú.", "Я уже не грустный.", "I am not sad any more."),
            ("Anna büszke rá.", "Анна гордится им.", "Anna is proud of him."),
            ("Peti boldog, és nagyon nyugodt.", "Пети счастлив и совсем спокоен.", "Peti is happy and very calm."),
        ],
    ],
    "m6-u3-fogalmak": [
        [
            ("Kata ma későn ér be a munkába.", "Ката сегодня опаздывает на работу.", "Kata gets to work late today."),
            ("A főnök tudni akarja, mi az ok.", "Начальник хочет знать, в чём причина.", "The boss wants to know the reason."),
            ("Kata szerint nehéz a helyzet.", "По словам Каты, ситуация сложная.", "Kata says it is a hard situation."),
            ("Az ok egyszerű, a busz nem jött.", "Причина простая: автобус не пришёл.", "The reason is simple: the bus did not come."),
        ],
        [
            ("A főnök szerint ez nem probléma, de van egy lehetőség.", "По мнению начальника, это не проблема, но есть один вариант.", "The boss says it is not a problem, but there is an option."),
            ("Kata holnap korán jön, és kész.", "Ката завтра придёт пораньше, и всё.", "Kata will come early tomorrow, and that is that."),
            ("Más lehetőség nincs, ez a döntés.", "Других вариантов нет, это решение.", "There is no other option, that is the decision."),
            ("Kata dönt, és ez az ő felelőssége.", "Ката решает, и это её ответственность.", "Kata decides, and that is her responsibility."),
        ],
        [
            ("Így van esély, hogy nem késik.", "Так есть шанс, что она не опоздает.", "This way there is a chance she will not be late."),
            ("A főnök is örül az eredménynek.", "Начальник тоже рад результату.", "The boss is happy with the result too."),
            ("A lényeg, hogy mindenki jól van.", "Главное, что у всех всё хорошо.", "The main thing is that everyone is fine."),
        ],
    ],
    "m6-u4-kotoszavak": [
        [
            ("Na, képzeld, tegnap nem mentem el.", "Ну, представь, вчера я не пошёл.", "Well, guess what, I didn't go yesterday."),
            ("Szóval, a meccs, ugye, este volt.", "Так вот, матч, помнишь, был вечером.", "So, the match, right, was in the evening."),
            ("Hiszen tudod, hogy szeretem a meccset!", "Ведь ты знаешь, как я люблю матчи!", "You know how much I love a good match!"),
        ],
        [
            ("Tehát mentem volna, de nem sikerült.", "Так что я бы пошёл, но не вышло.", "So I would have gone, but it didn't work out."),
            ("Ugyanis a főnök azt mondta, hogy maradjak.", "Дело в том, что начальник сказал остаться.", "You see, the boss told me to stay."),
            ("Ráadásul esett is az eső.", "Вдобавок ещё и дождь шёл.", "On top of that, it was raining too."),
            ("Viszont ma reggel már szép idő van.", "Зато сегодня утром уже хорошая погода.", "On the other hand, the weather is nice this morning."),
        ],
        [
            ("Egyébként te hogy vagy?", "Кстати, как ты сам?", "By the way, how are you?"),
            ("Amúgy holnap is lesz meccs.", "Кстати, завтра тоже будет матч.", "Anyway, there is a match tomorrow too."),
            ("Hátha te is jössz!", "А вдруг ты тоже придёшь!", "Maybe you'll come too!"),
        ],
    ],
}
