"""Month 1 dialogues: greetings, survival words, numbers and time, te/ön.

Written for the unit's own words and for month-1 grammar only: the present
tense, van/nincs, frozen polite forms. AI-written, not checked by a native
speaker (see dialogues.py); every word is attested in the pack at build time.
Each line: (Hungarian, Russian, English). Two speakers, turns alternate.
"""

DIALOGUES = {
    # Neighbours on the stairs.
    "m1-u1-koszones": [
        ("Jó reggelt! Te vagy az új szomszéd?", "Доброе утро! Ты наша новая соседка?", "Good morning! Are you the new neighbour?"),
        ("Igen, én vagyok. Szia! Anna vagyok.", "Да, это я. Привет! Я Анна.", "Yes, that's me. Hi! I'm Anna."),
        ("Szia, Anna! Én Péter vagyok. Nagyon örülök!", "Привет, Анна! Я Петер. Очень рад!", "Hi, Anna! I'm Péter. Very nice to meet you!"),
        ("Én is örülök. Hogy vagy?", "Я тоже рада. Как дела?", "Nice to meet you too. How are you?"),
        ("Köszönöm, jól. És te?", "Спасибо, хорошо. А ты?", "Fine, thanks. And you?"),
        ("Én is jól vagyok, köszönöm.", "У меня тоже всё хорошо, спасибо.", "I'm fine too, thanks."),
        ("Honnan jössz? Magyar vagy?", "Откуда ты? Ты венгерка?", "Where are you from? Are you Hungarian?"),
        ("Nem, orosz vagyok, de már itt lakom.", "Нет, я русская, но уже живу здесь.", "No, I'm Russian, but I already live here."),
        ("Akkor szomszédok vagyunk!", "Тогда мы соседи!", "Then we're neighbours!"),
        ("Igen! Viszlát, Péter! Szép napot!", "Да! Пока, Петер! Хорошего дня!", "Yes! See you, Péter! Have a nice day!"),
        ("Neked is! Szia!", "И тебе! Пока!", "You too! Bye!"),
    ],
    # A learner and a colleague: «не понимаю», «что это значит».
    "m1-u2-alapszavak": [
        ("Szia! Segítesz? Nem értem ezt a szót.", "Привет! Поможешь? Я не понимаю это слово.", "Hi! Can you help? I don't understand this word."),
        ("Persze! Melyik szó az?", "Конечно! Какое слово?", "Of course! Which word is it?"),
        ("Szívesen. Mit jelent ez?", "«Szívesen». Что это значит?", "\"Szívesen\". What does it mean?"),
        ("Azt jelenti, hogy nincs mit.", "Это значит «не за что».", "It means \"you're welcome\"."),
        ("Aha, értem! És a kösz?", "Ага, понятно! А «kösz»?", "Aha, I see! And \"kösz\"?"),
        ("Az ugyanaz. Csak a barátok mondják.", "Это то же самое. Так говорят только друзья.", "That's the same. Only friends say it."),
        ("Rendben! Nagyon köszönöm!", "Хорошо! Большое спасибо!", "Okay! Thank you very much!"),
        ("Szívesen!", "Пожалуйста!", "You're welcome!"),
    ],
    # A small shop: opening hours, the price, paying.
    "m1-u3-szamok-penz-ido": [
        ("Jó napot! Nyitva van még?", "Добрый день! Вы ещё открыты?", "Good afternoon! Are you still open?"),
        ("Igen, nyitva van. Öt órakor zár.", "Да, открыто. Закрываемся в пять.", "Yes, we're open. We close at five."),
        ("Hány óra van most?", "Который сейчас час?", "What time is it now?"),
        ("Fél négy van. Van még idő.", "Половина четвёртого. Время ещё есть.", "It's half past three. There's still time."),
        ("Köszönöm! Mennyibe kerül ez?", "Спасибо! Сколько это стоит?", "Thank you! How much does this cost?"),
        ("Ez ezer forint.", "Это тысяча форинтов.", "That's a thousand forints."),
        ("Nagyon drága! Van olcsó is?", "Очень дорого! А подешевле есть?", "Very expensive! Is there a cheap one too?"),
        ("Van. Ez csak száz forint.", "Есть. Этот всего сто форинтов.", "Yes. This one is only a hundred forints."),
        ("Az jó! Tessék, itt van a pénz.", "Это хорошо! Пожалуйста, вот деньги.", "That's good! Here you go, here's the money."),
        ("Köszönöm! Viszlát!", "Спасибо! Пока!", "Thank you! Bye!"),
    ],
    # The office window: ön, hölgyem, maga, and the polite answers.
    "m1-u4-tegezes-magazas": [
        ("Jó napot kívánok! Mit parancsol?", "Добрый день! Чем могу помочь?", "Good day! How can I help you?"),
        ("Jó napot! Anna vagyok. Itt van az igazolványom.", "Добрый день! Я Анна. Вот моё удостоверение.", "Good day! I'm Anna. Here is my ID."),
        ("Köszönöm, hölgyem. Ön magyar?", "Спасибо. Вы венгерка?", "Thank you, madam. Are you Hungarian?"),
        ("Nem, orosz vagyok. De tudok magyarul.", "Нет, я русская. Но говорю по-венгерски.", "No, I'm Russian. But I speak Hungarian."),
        ("Nagyon jó! Ön jól beszél magyarul.", "Очень хорошо! Вы хорошо говорите по-венгерски.", "Very good! You speak Hungarian well."),
        ("Köszönöm! Nagyon kedves.", "Спасибо! Вы очень любезны.", "Thank you! You're very kind."),
        ("Szívesen. Itt a kártya, tessék.", "Пожалуйста. Вот ваша карточка.", "You're welcome. Here's the card, please."),
        ("Köszönöm szépen. Viszontlátásra!", "Большое спасибо. До свидания!", "Thank you very much. Goodbye!"),
        ("Viszontlátásra, hölgyem!", "До свидания!", "Goodbye, madam!"),
    ],
}
