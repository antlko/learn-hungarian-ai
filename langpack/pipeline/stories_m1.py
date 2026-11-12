"""Month 1 stories (see stories.py for the format and the checks).

STORIES = {unit_code: [[(hu, ru, en), ...], [...]]}: paragraphs of
(Hungarian, Russian, English) sentences. AI-written, not checked by a native
speaker; every word is attested in the pack at build time. Fill the other units
from `python -m pipeline.stories PACK --brief UNIT`.
"""

STORIES: dict[str, list[list[tuple[str, str, str]]]] = {
    # Reference sample: two neighbours, one morning, one short frame reused.
    "m1-u1-koszones": [
        [
            ("Reggel van.", "Утро.", "It is morning."),
            ("Szép nap van.", "Хороший день.", "It is a nice day."),
            ("Anna az új szomszéd.", "Анна — новая соседка.", "Anna is the new neighbour."),
            ("Péter a régi szomszéd.", "Петер — давний сосед.", "Péter is the old neighbour."),
        ],
        [
            ("Péter köszön.", "Петер здоровается.", "Péter says hello."),
            ("Jó reggelt, Anna!", "Доброе утро, Анна!", "Good morning, Anna!"),
            ("Anna is köszön.", "Анна тоже здоровается.", "Anna says hello too."),
            ("Szia, Péter! Nagyon örülök!", "Привет, Петер! Очень рада!", "Hi, Péter! Very nice to meet you!"),
        ],
        [
            ("Hogy vagy, Anna?", "Как ты, Анна?", "How are you, Anna?"),
            ("Jól vagyok, köszönöm. És te?", "Хорошо, спасибо. А ты?", "Fine, thanks. And you?"),
            ("Én is jól vagyok.", "Я тоже хорошо.", "I am fine too."),
        ],
        [
            ("Anna nagyon örül.", "Анна очень рада.", "Anna is very glad."),
            ("Péter is nagyon örül.", "Петер тоже очень рад.", "Péter is very glad too."),
            ("Viszlát, Péter!", "Пока, Петер!", "See you, Péter!"),
        ],
    ],

    "m1-u2-alapszavak": [
        [
            ("Anna itt áll.", "Анна стоит здесь.", "Anna is standing here."),
            ("Péter is itt áll.", "Петер тоже стоит здесь.", "Péter is standing here too."),
            ("Péter gyorsan beszél.", "Петер говорит быстро.", "Péter talks fast."),
            ("Anna nem érti.", "Анна не понимает.", "Anna does not understand."),
        ],
        [
            ("Bocsánat, nem értem.", "Извините, я не понимаю.", "Sorry, I do not understand."),
            ("Tessék? Kérem, mondja újra, lassan.", "Что? Повторите, пожалуйста, медленно.", "Sorry? Please say it again, slowly."),
            ("Hát, Péter újra mondja, lassan.", "Ну вот, Петер повторяет медленно.", "Well, Péter says it again, slowly."),
            ("Most már Anna egy kicsit érti.", "Теперь Анна немного понимает.", "Now Anna understands a little."),
        ],
        [
            ("Anna egy telefont néz.", "Анна смотрит в телефон.", "Anna is looking at a phone."),
            ("Mit jelent ez a szó, Péter?", "Что значит это слово, Петер?", "What does this word mean, Péter?"),
            ("Péter fordít, ez egy egyszerű szó.", "Петер переводит, это простое слово.", "Péter translates, it is a simple word."),
            ("Nincs baj, Anna! Péter segít.", "Ничего страшного, Анна! Петер поможет.", "No problem, Anna! Péter will help."),
            ("Rendben, köszönöm!", "Хорошо, спасибо!", "All right, thank you!"),
        ],
    ],
    "m1-u3-szamok-penz-ido": [
        [
            ("Anna az ajtónál áll.", "Анна стоит у двери.", "Anna is standing at the door."),
            ("Az ajtó zárva van.", "Дверь закрыта.", "The door is closed."),
            ("Hány óra van most? Fél egy.", "Который сейчас час? Половина первого.", "What time is it now? Half past twelve."),
            ("Anna vár még tíz percet.", "Анна ждёт ещё десять минут.", "Anna waits another ten minutes."),
        ],
        [
            ("Jön Péter, és nyitja az ajtót.", "Приходит Петер и открывает дверь.", "Péter arrives and opens the door."),
            ("Most már nyitva van.", "Теперь уже открыто.", "It is open now."),
            ("Mennyibe kerül ez?", "Сколько это стоит?", "How much is this?"),
            ("Ezer forint.", "Тысяча форинтов.", "A thousand forints."),
            ("Jaj, drága! Van olcsó is?", "Ой, дорого! А подешевле есть?", "Oh, expensive! Is there a cheaper one?"),
        ],
        [
            ("Van, ez csak száz forint.", "Есть, этот всего сто форинтов.", "Yes, this one is only a hundred forints."),
            ("Anna fizet, de nincs nála készpénz.", "Анна платит, но у неё нет наличных.", "Anna pays, but she has no cash on her."),
            ("Kártyával fizet, persze.", "Конечно, она платит картой.", "She pays by card, of course."),
            ("Köszönöm, holnap újra jövök!", "Спасибо, завтра я снова приду!", "Thanks, I will come again tomorrow!"),
        ],
    ],
    "m1-u4-tegezes-magazas": [
        [
            ("Anna a hivatalban van.", "Анна в учреждении.", "Anna is at the office."),
            ("Sor van, sok ügyfél vár.", "Очередь, много посетителей ждёт.", "There is a queue, many clients are waiting."),
            ("Anna ügye nagyon egyszerű.", "Дело Анны очень простое.", "Anna's matter is very simple."),
            ("Itt van az igazolványa.", "Вот её удостоверение.", "Her ID is here."),
        ],
        [
            ("Ott áll Péter, a hivatalnok.", "Там стоит Петер, чиновник.", "There stands Péter, the clerk."),
            ("Péter nagyon udvarias.", "Петер очень вежлив.", "Péter is very polite."),
            ("Jó napot, hölgyem! Mit parancsol?", "Добрый день, сударыня! Чем могу помочь?", "Good day, madam! What can I do for you?"),
            ("Anna is udvarias, és köszön.", "Анна тоже вежлива и здоровается.", "Anna is polite too, and says hello."),
        ],
        [
            ("Aztán jön Kata, a barátnő.", "Потом приходит Ката, подруга.", "Then Kata, her friend, arrives."),
            ("Szia, Kata! Te is itt vagy?", "Привет, Ката! Ты тоже здесь?", "Hi, Kata! Are you here too?"),
            ("Szia, Anna! Igen, én is itt vagyok.", "Привет, Анна! Да, я тоже здесь.", "Hi, Anna! Yes, I am here too."),
            ("Anna és Kata együtt vár.", "Анна и Ката ждут вместе.", "Anna and Kata wait together."),
        ],
    ],
}
