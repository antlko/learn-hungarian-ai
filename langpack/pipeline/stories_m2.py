"""Month 2 stories (see stories.py for the format and the checks).

STORIES = {unit_code: [[(hu, ru, en), ...], [...]]}. AI-written, not checked by
a native speaker; every word is attested in the pack at build time.
"""

STORIES: dict[str, list[list[tuple[str, str, str]]]] = {
    "m2-u1-bolt": [
        [
            ("Ma szép nap van, Anna a piacra megy.", "Сегодня хороший день, Анна идёт на рынок.", "It is a nice day today, Anna is going to the market."),
            ("Péter is jön, nála van a kosár.", "Петер тоже идёт, корзина у него.", "Péter comes too, he has the basket."),
            ("Anna nem a boltba megy, ma a piac kell.", "Анна идёт не в магазин, сегодня ей нужен рынок.", "Anna is not going to the shop, today she wants the market."),
            ("Anna mindig a piacon vásárol, nem a boltban.", "Анна всегда делает покупки на рынке, а не в магазине.", "Anna always shops at the market, not in the shop."),
        ],
        [
            ("Nézd, Péter, friss kenyér!", "Смотри, Петер, свежий хлеб!", "Look, Péter, fresh bread!"),
            ("Anna vesz egy kenyeret.", "Анна берёт хлеб.", "Anna buys a loaf."),
            ("Nézd, Péter, friss tej!", "Смотри, Петер, свежее молоко!", "Look, Péter, fresh milk!"),
            ("Anna vesz tejet is.", "Анна берёт и молоко.", "Anna buys milk too."),
            ("Nézd, friss alma! Az is kell.", "Смотри, свежие яблоки! Они тоже нужны.", "Look, fresh apples! We need those too."),
            ("Hát, a kosár már majdnem tele van!", "Ну вот, корзина уже почти полная!", "Well, the basket is almost full already!"),
        ],
        [
            ("Jaj, nincs tojás! Péter, van itt tojás?", "Ой, яиц нет! Петер, здесь есть яйца?", "Oh, no eggs! Péter, is there any egg here?"),
            ("Nézd, Anna, ott van tojás és sajt is!", "Смотри, Анна, вон там и яйца, и сыр!", "Look, Anna, over there are eggs and cheese too!"),
            ("Anna vesz tojást és sajtot.", "Анна берёт яйца и сыр.", "Anna buys eggs and cheese."),
            ("Most már tele van a kosár.", "Теперь корзина полная.", "Now the basket is full."),
        ],
    ],
    "m2-u2-utca": [
        [
            ("Péter ma az új kerületbe megy.", "Сегодня Петер едет в новый район.", "Today Péter is going to the new district."),
            ("Nála van a cím és a térkép.", "У него адрес и карта.", "He has the address and the map."),
            ("Hol van a megálló? Ott van a sarkon!", "Где остановка? Вот она, на углу!", "Where is the stop? There, on the corner!"),
        ],
        [
            ("Jön a busz. Péter felszáll.", "Идёт автобус. Петер садится.", "The bus is coming. Péter gets on."),
            ("Jaj, jön az ellenőr! Hol van a bérlet?", "Ой, идёт контролёр! Где проездной?", "Oh, here comes the inspector! Where is the pass?"),
            ("Itt van a bérlet, rendben.", "Вот проездной, всё в порядке.", "Here is the pass, all fine."),
            ("A busz megy, megy, és Péter leszáll.", "Автобус едет и едет, и Петер выходит.", "The bus goes on and on, and Péter gets off."),
        ],
        [
            ("Hát, ez nem az utca! Péter megkérdez egy embert.", "Ну, это не та улица! Петер спрашивает прохожего.", "Well, this is not the street! Péter asks a passer-by."),
            ("Az ember mutatja az utat.", "Прохожий показывает дорогу.", "The passer-by shows the way."),
            ("Egyenesen, balra, aztán jobbra!", "Прямо, налево, потом направо!", "Straight, left, then right!"),
            ("Péter egyenesen megy, a térnél balra fordul.", "Петер идёт прямо, на площади поворачивает налево.", "Péter goes straight, at the square he turns left."),
            ("A híd mellett jobbra fordul, és ott van az utca.", "У моста он поворачивает направо, и вот улица.", "By the bridge he turns right, and there is the street."),
            ("Gyalog megy tovább. Nincs messze!", "Он идёт дальше пешком. Это недалеко!", "He walks on. It is not far!"),
        ],
    ],
    "m2-u3-lakas": [
        [
            ("Anna új lakásba költözik.", "Анна переезжает в новую квартиру.", "Anna is moving into a new flat."),
            ("Péter segít, és a tulajdonos is itt van.", "Петер помогает, и хозяин тоже здесь.", "Péter helps, and the owner is here too."),
            ("Itt van a kulcs, tessék!", "Вот ключ, пожалуйста!", "Here is the key, here you go!"),
        ],
        [
            ("Ez a nappali, ott van a konyha.", "Это гостиная, там кухня.", "This is the living room, over there is the kitchen."),
            ("Nézd, Péter, szép nagy ablak!", "Смотри, Петер, красивое большое окно!", "Look, Péter, a lovely big window!"),
            ("Nézd, Péter, szép nagy asztal!", "Смотри, Петер, красивый большой стол!", "Look, Péter, a lovely big table!"),
            ("Van szék is, és van egy szép lámpa.", "Есть и стул, и красивая лампа.", "There is a chair too, and a nice lamp."),
        ],
        [
            ("Na, a hűtő! Jaj, nem jó, elromlik!", "Так, холодильник! Ой, плохо, он ломается!", "Now, the fridge! Oh no, it is breaking down!"),
            ("A tulajdonos azt mondja, majd javítja.", "Хозяин говорит: починю.", "The owner says: I'll repair it."),
            ("Anna takarít, Péter mosogat.", "Анна убирает, Петер моет посуду.", "Anna tidies up, Péter washes up."),
            ("Most már jó a lakás. Nagyon jó!", "Теперь квартира в порядке. Очень хорошо!", "Now the flat is fine. Really good!"),
        ],
    ],
    "m2-u4-mennyi": [
        [
            ("Péter a boltban vesz paradicsomot.", "Петер покупает в магазине помидоры.", "Péter buys tomatoes in the shop."),
            ("Az eladó a mérlegre néz.", "Продавец смотрит на весы.", "The seller looks at the scales."),
            ("Kérek egy kilót, kérem.", "Мне килограмм, пожалуйста.", "I would like a kilo, please."),
            ("Nem, ez kevés! Kérek még egy kilót.", "Нет, этого мало! Мне ещё килограмм.", "No, that is too little! I want another kilo."),
        ],
        [
            ("Most két kiló van. Elég?", "Теперь два килограмма. Хватит?", "Now there are two kilos. Enough?"),
            ("Igen, elég. Még egy üveg víz kell.", "Да, хватит. Ещё нужна бутылка воды.", "Yes, enough. I also need a bottle of water."),
            ("A zacskó nagy, de nem nehéz.", "Пакет большой, но не тяжёлый.", "The bag is big, but not heavy."),
            ("Az akció jó, ez kicsit olcsóbb.", "Скидка хорошая, так немного дешевле.", "The offer is good, this is a little cheaper."),
        ],
        [
            ("Péter a pénztárhoz megy.", "Петер идёт к кассе.", "Péter goes to the till."),
            ("Mennyi lesz? Péter kéri a blokkot.", "Сколько с меня? Петер просит чек.", "How much? Péter asks for the receipt."),
        ],
    ],
}
