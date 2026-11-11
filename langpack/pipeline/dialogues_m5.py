"""Month 5 dialogues: a repair, a phone booking, the kindergarten, a restaurant.

Grammar leaned on: the conditional, -hat/-het, preverbs, the past tense, the
definite conjugation. AI-written, not checked by a native speaker (see
dialogues.py). Each line: (Hungarian, Russian, English).
"""

DIALOGUES = {
    # A phone call to the landlord: the tap, the lift, the washing machine.
    "m5-u1-elromlott": [
        ("Jó napot! Csöpög a csap a fürdőszobában.", "Добрый день! В ванной капает кран.", "Good day! The tap in the bathroom is dripping."),
        ("Értem. Nagyon csöpög?", "Понятно. Сильно капает?", "I see. Is it dripping a lot?"),
        ("Igen, és folyik is a víz.", "Да, и вода ещё течёт.", "Yes, and the water is running too."),
        ("Azonnal küldöm a szerelőt.", "Сейчас же пришлю мастера.", "I'll send the repairman right away."),
        ("Köszönöm! Jó lenne még ma.", "Спасибо! Хорошо бы ещё сегодня.", "Thank you! It would be good if it were still today."),
        ("Egy óra múlva ott lesz.", "Через час он будет у вас.", "He'll be there in an hour."),
        ("Rendben. És a lift sem működik.", "Хорошо. И лифт тоже не работает.", "All right. And the lift isn't working either."),
        ("Azt is megnézi a szerelő.", "Лифт мастер тоже посмотрит.", "The repairman will look at that too."),
        ("Nagyon köszönöm. A mosógép is rossz. Van rá garancia?", "Большое спасибо. Стиральная машина тоже сломана. На неё есть гарантия?", "Thank you very much. The washing machine is broken too. Is there a warranty on it?"),
        ("Igen, még van garancia. Hozza a számlát!", "Да, гарантия ещё действует. Принесите чек!", "Yes, there's still a warranty. Bring the receipt!"),
        ("Rendben, keresem. Köszönöm!", "Хорошо, найду. Спасибо!", "All right, I'll find it. Thank you!"),
        ("Szívesen! Viszontlátásra!", "Пожалуйста! До свидания!", "You're welcome! Goodbye!"),
    ],
    # Booking an appointment by phone.
    "m5-u2-idopont": [
        ("Rendelő, jó napot kívánok!", "Приёмная, добрый день!", "Surgery, good day!"),
        ("Jó napot! Időpontot szeretnék kérni.", "Добрый день! Я хотела бы записаться на приём.", "Good day! I'd like to make an appointment."),
        ("Értem. Mikor ráér?", "Понятно. Когда вам удобно?", "I see. When are you free?"),
        ("Szerdán vagy csütörtökön. Reggel jobb lenne.", "В среду или в четверг. Лучше утром.", "Wednesday or Thursday. Morning would be better."),
        ("Szerdán nyolc órakor van szabad időpont.", "В среду в восемь часов есть свободное время.", "On Wednesday at eight there's a free slot."),
        ("Az nekem megfelel. Köszönöm!", "Мне это подходит. Спасибо!", "That suits me. Thank you!"),
        ("Mi a neve?", "Как вас зовут?", "What's your name?"),
        ("Anna vagyok. Ha nem érek rá, szólok.", "Меня зовут Анна. Если не смогу, я сообщу.", "I'm Anna. If I can't make it, I'll let you know."),
        ("Rendben. Kérem, szóljon, ha változik valami.", "Хорошо. Скажите, пожалуйста, если что-то изменится.", "All right. Please tell us if anything changes."),
        ("Természetesen. Köszönöm szépen!", "Конечно. Большое спасибо!", "Of course. Thank you very much!"),
        ("Szívesen! Viszontlátásra!", "Пожалуйста! До свидания!", "You're welcome! Goodbye!"),
    ],
    # Enrolling a child at the kindergarten.
    "m5-u3-ovoda-hivatal": [
        ("Jó napot! A lányom három éves. Óvodába szeretném hozni.", "Добрый день! Моей дочери три года. Я хотел бы привести её в детский сад.", "Good day! My daughter is three years old. I'd like to bring her to the kindergarten."),
        ("Jó napot! Szeret játszani és énekelni?", "Добрый день! Она любит играть и петь?", "Good day! Does she like to play and sing?"),
        ("Igen, nagyon szeret. Mindig játszik és énekel.", "Да, очень любит. Всё время играет и поёт.", "Yes, very much. She's always playing and singing."),
        ("Nagyon jó! Itt a nyomtatvány.", "Очень хорошо! Вот бланк.", "Very good! Here's the form."),
        ("Köszönöm. Kell másolat az igazolványról?", "Спасибо. Нужна копия удостоверения?", "Thank you. Do I need a copy of the ID?"),
        ("Igen, kérem. Az eredeti is kell.", "Да, пожалуйста. И оригинал тоже нужен.", "Yes, please. The original is needed too."),
        ("Itt van mindkettő. Mikor kezdődik az óvoda?", "Вот и то и другое. Когда начинается детский сад?", "Here are both. When does the kindergarten start?"),
        ("Hétfőn. Reggel nyolc órakor hozza a gyereket.", "В понедельник. Приведите ребёнка в восемь утра.", "On Monday. Bring the child at eight in the morning."),
        ("Rendben. És az ebéd? Az is van?", "Хорошо. А обед? Он тоже есть?", "All right. And lunch? Is that there too?"),
        ("Igen, az óvodában ebéd is van.", "Да, обед в детском саду тоже есть.", "Yes, there is lunch at the kindergarten too."),
        ("Köszönöm! Viszontlátásra!", "Спасибо! До свидания!", "Thank you! Goodbye!"),
    ],
    # Dinner at a restaurant.
    "m5-u4-etterem": [
        ("Jó estét! Van szabad asztal?", "Добрый вечер! Есть свободный столик?", "Good evening! Is there a free table?"),
        ("Igen, tessék. Itt az étlap.", "Да, пожалуйста. Вот меню.", "Yes, please. Here's the menu."),
        ("Köszönöm. Mit ajánl?", "Спасибо. Что вы порекомендуете?", "Thank you. What do you recommend?"),
        ("A leves nagyon jó, a rántott hús is finom.", "Суп очень хороший, и шницель тоже вкусный.", "The soup is very good, and the breaded meat is tasty too."),
        ("Akkor egy levest kérek, és a rántott húst.", "Тогда мне суп и шницель.", "Then I'll have a soup and the breaded meat."),
        ("És inni? Bort vagy sört?", "А пить? Вино или пиво?", "And to drink? Wine or beer?"),
        ("Nem iszom alkoholt. Egy ásványvizet kérek.", "Я не пью алкоголь. Минеральную воду, пожалуйста.", "I don't drink alcohol. A mineral water, please."),
        ("Persze. Desszertnek palacsintát?", "Конечно. На десерт блинчики?", "Of course. Pancakes for dessert?"),
        ("Igen, az jó lenne. Köszönöm!", "Да, это было бы хорошо. Спасибо!", "Yes, that would be nice. Thank you!"),
        ("Hogy ízlik a leves?", "Как вам суп?", "How do you like the soup?"),
        ("Nagyon finom! A számlát kérem.", "Очень вкусно! Счёт, пожалуйста.", "Very tasty! The bill, please."),
        ("Tessék. Köszönjük, hogy itt volt!", "Пожалуйста. Спасибо, что были у нас!", "Here you are. Thank you for being here!"),
    ],
}
