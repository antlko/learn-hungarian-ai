"""Month 3 stories (see stories.py for the format and the checks).

STORIES = {unit_code: [[(hu, ru, en), ...], [...]]}. AI-written, not checked by a
native speaker; every word is attested in the pack at build time.
"""

STORIES: dict[str, list[list[tuple[str, str, str]]]] = {
    "m3-u1-ugyintezes": [
        [
            ("Kata ma a hatóságnál van.", "Сегодня Ката в ведомстве.", "Today Kata is at the agency."),
            ("Van időpontja, de nagy a várakozás.", "У неё есть запись, но ожидание долгое.", "She has an appointment, but the wait is long."),
            ("Kata vár. A száma hét.", "Ката ждёт. Её номер — семь.", "Kata waits. Her number is seven."),
        ],
        [
            ("Végre a pulthoz megy.", "Наконец она подходит к стойке.", "At last she goes to the counter."),
            ("Gábor ott dolgozik, és ad egy űrlapot.", "Габор там работает и даёт ей бланк.", "Gábor works there, and gives her a form."),
            ("Kata kitölt egy űrlapot, aztán aláír.", "Ката заполняет бланк, потом подписывает.", "Kata fills in the form, then signs."),
            ("Gábor nézi az aláírást. Jó!", "Габор смотрит на подпись. Хорошо!", "Gábor looks at the signature. Good!"),
        ],
        [
            ("Aztán megnézi az okmányokat is.", "Потом смотрит и документы.", "Then he looks at the documents too."),
            ("Az útlevél érvényes, a lakcím is jó.", "Загранпаспорт действителен, адрес тоже в порядке.", "The passport is valid, and the address is fine too."),
            ("De egy másolat hiányzik.", "Но не хватает одной копии.", "But one copy is missing."),
            ("Semmi baj, Katának van türelme.", "Ничего страшного, у Каты есть терпение.", "No problem, Kata has patience."),
            ("Holnap jön, és elintéz mindent.", "Завтра придёт и всё уладит.", "Tomorrow she will come and get it sorted."),
        ],
    ],
    "m3-u2-orvos": [
        [
            ("Eszter ma beteg.", "Эстер сегодня болеет.", "Eszter is ill today."),
            ("Fáj a torka, és köhög is.", "У неё болит горло, и она кашляет.", "Her throat hurts, and she is coughing too."),
            ("Láza is van, és szédül.", "У неё ещё жар, и кружится голова.", "She has a fever too, and she feels dizzy."),
            ("Hát, ez nem jó!", "Ну, это нехорошо!", "Well, this is not good!"),
        ],
        [
            ("Eszter elmegy a háziorvoshoz.", "Эстер идёт к участковому врачу.", "Eszter goes to the family doctor."),
            ("A rendelő nincs messze.", "Кабинет недалеко.", "The surgery is not far."),
            ("Az orvos vizsgál egy kicsit.", "Врач её немного осматривает.", "The doctor examines her for a while."),
            ("Az orvos kérdez. Mióta fáj a torkod?", "Врач спрашивает. С каких пор у тебя болит горло?", "The doctor asks. Since when has your throat hurt?"),
            ("Eszter azt mondja, hétfő óta.", "Эстер отвечает, что с понедельника.", "Eszter says, since Monday."),
        ],
        [
            ("Az orvos ír egy receptet. Pihenni kell.", "Врач выписывает рецепт. Нужно отдыхать.", "The doctor writes a prescription. Rest is needed."),
            ("Eszter a gyógyszertárba megy.", "Эстер идёт в аптеку.", "Eszter goes to the pharmacy."),
            ("Kap tablettát és cseppet.", "Она получает таблетки и капли.", "She gets pills and drops."),
            ("Otthon szed egy tablettát, és alszik.", "Дома она принимает таблетку и спит.", "At home she takes a pill and sleeps."),
            ("Holnap már jó lesz!", "Завтра уже будет лучше!", "Tomorrow she will be better!"),
        ],
    ],
    "m3-u3-bank": [
        [
            ("Béla ma kap egy csekket.", "Сегодня Бела получает квитанцию.", "Today Béla gets a payment slip."),
            ("Ez az első rezsi. Az áram.", "Это первая коммуналка. Электричество.", "It is the first utility bill. The electricity."),
            ("A határidő péntek.", "Крайний срок — пятница.", "The deadline is Friday."),
            ("Béla spórol, de ezt meg kell fizetni.", "Бела экономит, но это надо оплатить.", "Béla saves, but this has to be paid."),
        ],
        [
            ("Béla a bankba megy.", "Бела идёт в банк.", "Béla goes to the bank."),
            ("Az automatánál kell egy kód.", "У банкомата нужен код.", "At the machine he needs a code."),
            ("Béla jól tudja a kódot.", "Бела хорошо знает код.", "Béla knows the code well."),
            ("Az utaláshoz kell az azonosító.", "Для перевода нужен номер клиента.", "For the transfer he needs the customer number."),
        ],
        [
            ("Béla néz egy csekket. Ott van!", "Бела смотрит на квитанцию. Вот он!", "Béla looks at the slip. There it is!"),
            ("Béla átutal egy kis pénzt.", "Бела переводит немного денег.", "Béla transfers a little money."),
            ("Az utalás kész, a nyugta is megvan.", "Перевод готов, квитанция тоже есть.", "The transfer is done, and he has the receipt too."),
            ("Béla örül. A rezsi rendben van.", "Бела рад. Коммуналка оплачена.", "Béla is happy. The utilities are sorted."),
        ],
    ],
    "m3-u4-posta": [
        [
            ("A postaládában ma egy értesítés van.", "Сегодня в почтовом ящике извещение.", "Today there is a notice in the letterbox."),
            ("Tegnapelőtt jött egy ajánlott levél.", "Позавчера пришло заказное письмо.", "A registered letter came the day before yesterday."),
            ("A postás itt volt, de Éva nem volt otthon.", "Почтальон приходил, но Евы не было дома.", "The postman was here, but Éva was not at home."),
        ],
        [
            ("Éva a postára megy.", "Ева идёт на почту.", "Éva goes to the post office."),
            ("Ott a pultnál vár egy kicsit.", "Там у стойки она немного ждёт.", "There at the counter she waits a little."),
            ("Ezt csak személyesen lehet.", "Это можно только лично.", "You can only do this in person."),
            ("Éva átvesz egy levelet, és aláír.", "Ева получает письмо и расписывается.", "Éva takes it and signs."),
        ],
        [
            ("A feladó Anna, egy régi barátnő.", "Отправитель — Анна, старая подруга.", "The sender is Anna, an old friend."),
            ("A borítékban egy képeslap van.", "В конверте открытка.", "There is a postcard in the envelope."),
            ("Éva is küld egy levelet, és ragaszt egy bélyeget.", "Ева тоже отправляет письмо и клеит марку.", "Éva sends a letter too, and sticks on a stamp."),
            ("Anna hamar megkap egy levelet!", "Анна скоро получит письмо!", "Anna will get a letter soon!"),
        ],
    ],
}
