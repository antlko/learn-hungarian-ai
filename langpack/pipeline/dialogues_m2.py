"""Month 2 dialogues: the shop, the street, the flat, quantities and prices.

Grammar leaned on: the accusative, the plural, locative cases (-ban, -ra, -ról),
present tense, van/nincs. AI-written, not checked by a native speaker (see
dialogues.py). Each line: (Hungarian, Russian, English).
"""

DIALOGUES = {
    # The shop on the corner.
    "m2-u1-bolt": [
        ("Jó napot! Kérek öt almát.", "Добрый день! Мне пять яблок.", "Good afternoon! Five apples, please."),
        ("Tessék! Még valamit?", "Пожалуйста! Что-нибудь ещё?", "Here you are! Anything else?"),
        ("Igen. Van friss kenyér?", "Да. Свежий хлеб есть?", "Yes. Do you have fresh bread?"),
        ("Van. Mennyit kér?", "Есть. Сколько вам?", "Yes. How many would you like?"),
        ("Egyet kérek. És tojást is.", "Один. И яйца тоже.", "One, please. And eggs too."),
        ("Hányat?", "Сколько?", "How many?"),
        ("Hatot, kérem. És még tejet.", "Шесть, пожалуйста. И ещё молока.", "Six, please. And some milk too."),
        ("Tessék. Ez minden?", "Пожалуйста. Это всё?", "Here you are. Is that everything?"),
        ("Igen, ennyi. Mennyi lesz?", "Да, всё. Сколько с меня?", "Yes, that's it. How much will it be?"),
        ("Ezer forint, kérem.", "Тысяча форинтов, пожалуйста.", "A thousand forints, please."),
        ("Tessék. Köszönöm szépen!", "Пожалуйста. Большое спасибо!", "Here you go. Thank you very much!"),
        ("Szívesen! Viszlát!", "Пожалуйста! До свидания!", "You're welcome! Bye!"),
    ],
    # Asking the way to the metro.
    "m2-u2-utca": [
        ("Elnézést! Hol van a metró?", "Извините! Где метро?", "Excuse me! Where is the metro?"),
        ("Menjen egyenesen, aztán balra.", "Идите прямо, потом налево.", "Go straight, then left."),
        ("Messze van?", "Это далеко?", "Is it far?"),
        ("Nem, nincs messze. Gyalog öt perc.", "Нет, недалеко. Пять минут пешком.", "No, it's not far. Five minutes on foot."),
        ("Köszönöm! És a megálló? Az hol van?", "Спасибо! А остановка? Где она?", "Thank you! And the stop? Where is that?"),
        ("A megálló a sarkon van.", "Остановка на углу.", "The stop is on the corner."),
        ("Van busz is?", "Автобус тоже есть?", "Is there a bus too?"),
        ("Van. Hova megy ön?", "Есть. А вы куда едете?", "Yes. Where are you going?"),
        ("A városba megyek.", "Мне в город.", "I'm going into town."),
        ("Akkor jó a busz. A város nincs messze.", "Тогда автобус подойдёт. Город недалеко.", "Then the bus is fine. The town isn't far."),
        ("Nagyon köszönöm! Viszlát!", "Огромное спасибо! До свидания!", "Thank you so much! Bye!"),
    ],
    # The owner shows a flat.
    "m2-u3-lakas": [
        ("Tessék, ez a lakás. Itt van a nappali.", "Вот квартира. Это гостиная.", "Here's the flat. This is the living room."),
        ("Nagyon szép! És hol van a konyha?", "Очень красиво! А где кухня?", "Very nice! And where is the kitchen?"),
        ("A konyha ott van, balra.", "Кухня там, налево.", "The kitchen is there, on the left."),
        ("Van hűtő a konyhában?", "Холодильник на кухне есть?", "Is there a fridge in the kitchen?"),
        ("Igen, van hűtő és mosógép is.", "Да, есть и холодильник, и стиральная машина.", "Yes, there's a fridge and a washing machine too."),
        ("És a fürdőszoba? Van zuhany?", "А ванная? Душ есть?", "And the bathroom? Is there a shower?"),
        ("Van zuhany, de kád nincs.", "Душ есть, а ванны нет.", "There's a shower, but no bathtub."),
        ("Az nem baj. Hol van a hálószoba?", "Ничего страшного. Где спальня?", "That's okay. Where is the bedroom?"),
        ("Ez a hálószoba. Van benne ágy és szekrény.", "Вот спальня. В ней есть кровать и шкаф.", "This is the bedroom. There's a bed and a wardrobe in it."),
        ("Köszönöm! Nagyon tetszik a lakás.", "Спасибо! Квартира мне очень нравится.", "Thank you! I like the flat very much."),
        ("Örülök! Tessék, itt a kulcs.", "Рад! Вот ключ.", "I'm glad! Here's the key."),
        ("Köszönöm szépen!", "Большое спасибо!", "Thank you very much!"),
    ],
    # At the market: kilos, sizes, the price.
    "m2-u4-mennyi": [
        ("Mennyibe kerül egy kiló paradicsom?", "Сколько стоит килограмм помидоров?", "How much is a kilo of tomatoes?"),
        ("Ezer forint. Friss és nagyon jó.", "Тысяча форинтов. Свежие и очень хорошие.", "A thousand forints. Fresh and very good."),
        ("Ez a paradicsom túl kicsi. Van nagy is?", "Эти помидоры слишком мелкие. Крупные есть?", "These tomatoes are too small. Do you have big ones?"),
        ("Van. Tessék, ez nagy. Mennyit kér?", "Есть. Вот, эти крупные. Сколько вам?", "Yes. Here, these are big. How much would you like?"),
        ("Fél kilót kérek. Az elég.", "Полкило. Этого хватит.", "Half a kilo, please. That's enough."),
        ("Rendben. Még valamit?", "Хорошо. Что-нибудь ещё?", "All right. Anything else?"),
        ("Igen. Egy nagy üveg vizet, kérem.", "Да. Большую бутылку воды, пожалуйста.", "Yes. A big bottle of water, please."),
        ("Tessék. Ez egy liter. Elég?", "Пожалуйста. Здесь литр. Хватит?", "Here you are. That's one litre. Enough?"),
        ("Igen, köszönöm. Mennyi összesen?", "Да, спасибо. Сколько всего?", "Yes, thank you. How much altogether?"),
        ("Ez összesen ezer forint.", "Всего тысяча форинтов.", "That's a thousand forints altogether."),
        ("Tessék. Köszönöm! Viszlát!", "Пожалуйста. Спасибо! До свидания!", "Here you go. Thank you! Bye!"),
        ("Viszlát! Szép napot!", "До свидания! Хорошего дня!", "Bye! Have a nice day!"),
    ],
}
