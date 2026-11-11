"""Month 3 dialogues: the office window, the doctor, the bank, the post.

Grammar leaned on: dative (-nak/-nek), possessives (-m, -d), the past tense in
unit 4. AI-written, not checked by a native speaker (see dialogues.py).
Each line: (Hungarian, Russian, English).
"""

DIALOGUES = {
    # At the office window: a certificate, an ID, a form, a missing copy.
    "m3-u1-ugyintezes": [
        ("Jó napot kívánok! Mit szeretne?", "Добрый день! Что вы хотели?", "Good day! What would you like?"),
        ("Jó napot! Igazolást szeretnék kérni.", "Добрый день! Мне нужна справка.", "Good day! I'd like to ask for a certificate."),
        ("Értem. Van önnél személyi igazolvány?", "Понятно. Удостоверение личности при вас?", "I see. Do you have an ID card with you?"),
        ("Igen, itt van. Tessék.", "Да, вот оно. Пожалуйста.", "Yes, here it is. Here you are."),
        ("Köszönöm. Ezt az űrlapot kell kitölteni.", "Спасибо. Нужно заполнить этот бланк.", "Thank you. This form needs to be filled in."),
        ("Igen. Itt írom alá?", "Да. Я здесь подписываю?", "Yes. Do I sign here?"),
        ("Igen, ott. Még egy másolat hiányzik.", "Да, там. Не хватает ещё одной копии.", "Yes, there. One more copy is missing."),
        ("Sajnos nincs nálam másolat.", "К сожалению, копии у меня нет.", "Unfortunately I don't have a copy with me."),
        ("Semmi baj. Holnap is nyitva vagyunk.", "Ничего страшного. Завтра мы тоже открыты.", "No problem. We're open tomorrow too."),
        ("Rendben, akkor holnap jövök. Köszönöm!", "Хорошо, тогда приду завтра. Спасибо!", "All right, then I'll come tomorrow. Thank you!"),
        ("Szívesen. Viszontlátásra!", "Пожалуйста. До свидания!", "You're welcome. Goodbye!"),
    ],
    # The GP: what hurts, since when, the prescription.
    "m3-u2-orvos": [
        ("Jó napot! Mi a baj?", "Добрый день! Что случилось?", "Good day! What seems to be the trouble?"),
        ("Jó napot, doktor úr! Fáj a fejem és a torkom.", "Добрый день, доктор! У меня болят голова и горло.", "Good day, doctor! My head and my throat hurt."),
        ("Mióta fáj?", "С каких пор болит?", "Since when does it hurt?"),
        ("Tegnap óta fáj. Lázam is van.", "Со вчерашнего дня. И температура есть.", "Since yesterday. I have a fever too."),
        ("Nagy a láza?", "Температура высокая?", "Is the fever high?"),
        ("Igen, nagy. És fáj a hátam is.", "Да, высокая. И спина тоже болит.", "Yes, it's high. And my back hurts too."),
        ("Megnézem. Ez csak nátha. Itt a recept.", "Посмотрю. Это просто простуда. Вот рецепт.", "Let me see. It's just a cold. Here's the prescription."),
        ("Köszönöm. Hol van a gyógyszertár?", "Спасибо. Где аптека?", "Thank you. Where is the pharmacy?"),
        ("A sarkon van. Sokat kell pihenni.", "На углу. Нужно много отдыхать.", "On the corner. You need to rest a lot."),
        ("Köszönöm, doktor úr. Viszontlátásra!", "Спасибо, доктор. До свидания!", "Thank you, doctor. Goodbye!"),
    ],
    # Opening an account.
    "m3-u3-bank": [
        ("Jó napot! Számlát szeretnék nyitni.", "Добрый день! Я хотел бы открыть счёт.", "Good day! I'd like to open an account."),
        ("Értem. Van önnél igazolvány?", "Понятно. Удостоверение при вас?", "I see. Do you have an ID with you?"),
        ("Igen, itt van. Mennyi a havi díj?", "Да, вот оно. Сколько стоит обслуживание в месяц?", "Yes, here it is. What is the monthly fee?"),
        ("A havi díj ezer forint. Van bankkártya is.", "Ежемесячная плата тысяча форинтов. Есть и карта.", "The monthly fee is a thousand forints. There's a bank card too."),
        ("Mennyibe kerül a kártya?", "Сколько стоит карта?", "How much does the card cost?"),
        ("A kártya ingyenes.", "Карта бесплатная.", "The card is free."),
        ("Az jó! Van internetes bank is?", "Это хорошо! Интернет-банк есть?", "That's good! Is there online banking too?"),
        ("Igen, van. Itt a kód és a jelszó.", "Да. Вот код и пароль.", "Yes, there is. Here's the code and the password."),
        ("Köszönöm. A jelszót nem mondom senkinek.", "Спасибо. Пароль я никому не скажу.", "Thank you. I won't tell the password to anyone."),
        ("Nagyon jó, ez fontos. Itt a szerződés.", "Очень хорошо, это важно. Вот договор.", "Very good, that's important. Here's the contract."),
        ("Köszönöm. Mikor lesz kész a kártya?", "Спасибо. Когда карта будет готова?", "Thank you. When will the card be ready?"),
        ("Jövő héten.", "На следующей неделе.", "Next week."),
    ],
    # The post office: a notice, a registered letter, a stamp; the past tense.
    "m3-u4-posta": [
        ("Jó napot! Értesítést kaptam.", "Добрый день! Я получил извещение.", "Good day! I received a notice."),
        ("Az értesítést kérem.", "Давайте извещение.", "The notice, please."),
        ("Tessék. Egy levelet küldtek nekem.", "Пожалуйста. Мне прислали письмо.", "Here you are. They sent me a letter."),
        ("Értem. Ajánlott levél. Honnan jött?", "Понятно. Заказное письмо. Откуда оно?", "I see. A registered letter. Where is it from?"),
        ("Nem tudom. A postás tegnap nem talált otthon.", "Не знаю. Вчера почтальон не застал меня дома.", "I don't know. Yesterday the postman didn't find me at home."),
        ("Igen, ma érkezett. Az igazolványa megvan?", "Да, оно пришло сегодня. Удостоверение при вас?", "Yes, it arrived today. Do you have your ID?"),
        ("Itt van. Tessék.", "Вот. Пожалуйста.", "Here it is. Here you are."),
        ("Köszönöm. Itt a levél.", "Спасибо. Вот письмо.", "Thank you. Here's the letter."),
        ("Köszönöm. Mennyibe kerül egy bélyeg?", "Спасибо. Сколько стоит марка?", "Thank you. How much is a stamp?"),
        ("Egy bélyeg száz forint.", "Марка стоит сто форинтов.", "A stamp is a hundred forints."),
        ("Kérek kettőt. Tegnap is itt voltam, de zárva volt.", "Мне две. Вчера я тоже был здесь, но было закрыто.", "Two, please. I was here yesterday too, but it was closed."),
        ("Elnézést, tegnap zárva volt a posta.", "Простите, вчера почта была закрыта.", "Sorry, the post office was closed yesterday."),
    ],
}
