"""Month 6 dialogues: an opinion, feelings, an abstract choice, and the little
words of live speech (hát, na, szóval, ugye, hiszen, képzeld, izé).

AI-written, not checked by a native speaker (see dialogues.py).
Each line: (Hungarian, Russian, English).
"""

DIALOGUES = {
    # A disagreement about the utility bill.
    "m6-u1-velemeny": [
        ("Jó napot! Szerintem ez a számla nem jó.", "Добрый день! По-моему, этот счёт неверный.", "Good day! I think this bill is wrong."),
        ("Miért gondolja ezt? Én azt hiszem, jó.", "Почему вы так думаете? Я считаю, что он верный.", "Why do you think so? I believe it's right."),
        ("Az összeg túl magas. Ezt nem értem.", "Сумма слишком большая. Этого я не понимаю.", "The amount is too high. I don't understand that."),
        ("Értem. De lehetséges, hogy tévedés történt.", "Понятно. Но возможно, что произошла ошибка.", "I see. But it's possible that a mistake was made."),
        ("Igen, szerintem is. Nézze meg, kérem!", "Да, я тоже так думаю. Посмотрите, пожалуйста!", "Yes, I think so too. Please have a look!"),
        ("Persze, megnézem. Igaza van! Hibás a számla.", "Конечно, посмотрю. Вы правы! Счёт неверный.", "Of course, I'll check. You're right! The bill is wrong."),
        ("Köszönöm. Akkor nem kell fizetni, ugye?", "Спасибо. Тогда платить не нужно, да?", "Thank you. Then I don't have to pay, right?"),
        ("Nem, csak a jó számlát. Sajnálom a hibát.", "Нет, только правильный счёт. Прошу прощения за ошибку.", "No, only the correct bill. I'm sorry about the mistake."),
        ("Semmi baj. Mindenki téved néha.", "Ничего страшного. Все иногда ошибаются.", "No problem. Everyone makes mistakes sometimes."),
        ("Köszönöm szépen. Szép napot!", "Большое спасибо. Хорошего дня!", "Thank you very much. Have a nice day!"),
    ],
    # A friend notices that something is wrong.
    "m6-u2-erzesek": [
        ("Szia! Mi a baj? Szomorú vagy.", "Привет! Что случилось? Ты грустный.", "Hi! What's wrong? You look sad."),
        ("Igen, egy kicsit szomorú vagyok. Fáradt is.", "Да, немного грустно. И устал.", "Yes, I'm a bit sad. And tired too."),
        ("Miért? Mi történt?", "Почему? Что случилось?", "Why? What happened?"),
        ("Semmi komoly. Csak a főnököm idegesít.", "Ничего серьёзного. Просто начальник меня раздражает.", "Nothing serious. It's just my boss who annoys me."),
        ("Értem. Ez nem könnyű. Én is érzek ilyet néha.", "Понимаю. Это нелегко. Я тоже иногда такое чувствую.", "I see. That's not easy. I feel like that sometimes too."),
        ("Tényleg? Te mindig vidám vagy!", "Правда? Ты всегда весёлый!", "Really? You're always cheerful!"),
        ("Nem mindig. De remélem, hogy minden jó lesz.", "Не всегда. Но надеюсь, что всё будет хорошо.", "Not always. But I hope everything will be fine."),
        ("Köszönöm. Jól esik, hogy itt vagy.", "Спасибо. Мне приятно, что ты рядом.", "Thank you. It feels good that you're here."),
        ("Gyere, sétálunk egyet!", "Пойдём, прогуляемся!", "Come on, let's go for a walk!"),
        ("Jó ötlet! Máris jobban érzem magam.", "Хорошая идея! Мне уже лучше.", "Good idea! I already feel better."),
    ],
    # Two colleagues talk about a choice.
    "m6-u3-fogalmak": [
        ("Nehéz döntés előtt állok.", "Мне предстоит трудное решение.", "I'm facing a difficult decision."),
        ("Mi a probléma?", "В чём проблема?", "What's the problem?"),
        ("Van egy új lehetőség. Nem tudom, jó-e.", "Есть новая возможность. Не знаю, хороша ли она.", "There's a new opportunity. I don't know if it's good."),
        ("Mi a különbség a két munka között?", "Какая разница между двумя работами?", "What's the difference between the two jobs?"),
        ("Az új munka jobban fizet, de nem biztos.", "Новая работа платит больше, но она ненадёжная.", "The new job pays better, but it isn't secure."),
        ("Az élet nem mindig ad ilyen alkalmat.", "Жизнь не всегда даёт такую возможность.", "Life doesn't always give such a chance."),
        ("Igaz. De a régi munkát szeretem.", "Правда. Но старую работу я люблю.", "True. But I like the old job."),
        ("Akkor mi lesz? Maradsz?", "Так что будет? Остаёшься?", "So what's going to happen? Are you staying?"),
        ("Nem tudom. Holnap döntök.", "Не знаю. Завтра решу.", "I don't know. I'll decide tomorrow."),
        ("Jó. Ez a te döntésed, nem az enyém.", "Хорошо. Это твоё решение, а не моё.", "Good. It's your decision, not mine."),
    ],
    # Live speech: hát, na, szóval, hiszen, képzeld, izé, ugye.
    "m6-u4-kotoszavak": [
        ("Szia! Na, mi újság?", "Привет! Ну, что нового?", "Hi! So, what's new?"),
        ("Szia! Hát, nincs sok. Képzeld, nem működik a lift!", "Привет! Ну, немного. Представь, лифт не работает!", "Hi! Well, not much. Imagine, the lift isn't working!"),
        ("Tényleg? Hiszen tegnap még jó volt!", "Правда? Ведь вчера ещё был исправен!", "Really? But it was fine yesterday!"),
        ("Igen, de ma reggel már nem. Szóval gyalog jöttem.", "Да, но сегодня утром уже нет. Так что я пришёл пешком.", "Yes, but not this morning. So I came on foot."),
        ("Jaj! Ugye a harmadikon laksz?", "Ой! Ты ведь живёшь на третьем?", "Oh! You live on the third floor, right?"),
        ("Igen. Egyébként a szerelő már jön.", "Да. Кстати, мастер уже едет.", "Yes. By the way, the repairman is already coming."),
        ("Hát, remélem. Amúgy te hova mész?", "Ну, надеюсь. А ты, кстати, куда идёшь?", "Well, I hope so. Anyway, where are you going?"),
        ("A boltba. Izé... kérsz valamit?", "В магазин. Э-э... тебе что-нибудь нужно?", "To the shop. Um... do you want anything?"),
        ("Nem, köszönöm. Vagyis... igen, egy kenyeret!", "Нет, спасибо. То есть... да, один хлеб!", "No, thanks. I mean... yes, a loaf of bread!"),
        ("Rendben. Akkor mindjárt jövök.", "Хорошо. Тогда я сейчас вернусь.", "All right. Then I'll be right back."),
    ],
}
