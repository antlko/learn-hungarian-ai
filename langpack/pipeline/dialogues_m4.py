"""Month 4 dialogues: work, a rented flat, family, what happened.

Grammar leaned on: the infinitive, the accusative, possessives, and the past
tense (definite and indefinite) in unit 4. AI-written, not checked by a native
speaker (see dialogues.py). Each line: (Hungarian, Russian, English).
"""

DIALOGUES = {
    # A job interview.
    "m4-u1-munka": [
        ("Jó napot! Anna vagyok. Állást keresek.", "Добрый день! Я Анна. Я ищу работу.", "Good day! I'm Anna. I'm looking for a job."),
        ("Jó napot, Anna! Hol dolgozik most?", "Добрый день, Анна! Где вы сейчас работаете?", "Good day, Anna! Where do you work now?"),
        ("Egy irodában dolgozom, de új állást keresek.", "Я работаю в офисе, но ищу новое место.", "I work in an office, but I'm looking for a new job."),
        ("Beszél magyarul és angolul?", "Вы говорите по-венгерски и по-английски?", "Do you speak Hungarian and English?"),
        ("Igen, jól beszélek magyarul és angolul.", "Да, я хорошо говорю по-венгерски и по-английски.", "Yes, I speak Hungarian and English well."),
        ("Nagyon jó. Mi a szakmája?", "Очень хорошо. Какая у вас профессия?", "Very good. What is your profession?"),
        ("Titkár vagyok.", "Я секретарь.", "I'm a secretary."),
        ("Rendben. Nálunk a munkaidő nyolc óra.", "Хорошо. У нас рабочий день восемь часов.", "All right. Our working day is eight hours."),
        ("És mennyi a fizetés?", "А какая зарплата?", "And what's the pay?"),
        ("A bér jó, és sok a szabadság.", "Зарплата хорошая, и отпуск большой.", "The pay is good, and there's a lot of leave."),
        ("Nagyon jó! Mikor kezdek?", "Отлично! Когда мне начинать?", "Great! When do I start?"),
        ("Hétfőn! Üdvözlöm a csapatban.", "В понедельник! Добро пожаловать в команду.", "On Monday! Welcome to the team."),
    ],
    # Viewing a rented flat: the deposit and the contract.
    "m4-u2-alberlet": [
        ("Jó napot! Ez az albérlet?", "Добрый день! Это та квартира в аренду?", "Good day! Is this the flat to rent?"),
        ("Igen. Tessék megnézni! Tágas és világos.", "Да. Пожалуйста, посмотрите! Просторная и светлая.", "Yes. Please have a look! Spacious and bright."),
        ("Valóban szép. Csendes a környék?", "Правда красивая. В районе тихо?", "Really nice. Is the area quiet?"),
        ("Igen, nagyon csendes. Van parkoló és garázs is.", "Да, очень тихо. Есть парковка и гараж.", "Yes, very quiet. There's parking and a garage too."),
        ("Mennyi a bérleti díj?", "Сколько арендная плата?", "How much is the rent?"),
        ("Nem sok. De van még a közös költség is.", "Немного. Но есть ещё коммунальные расходы.", "Not much. But there's the shared cost too."),
        ("És a letét? Mennyi a letét?", "А залог? Сколько залог?", "And the deposit? How much is the deposit?"),
        ("A letét egy havi bérleti díj.", "Залог — одна месячная плата.", "The deposit is one month's rent."),
        ("Rendben. Mikor írjuk alá a szerződést?", "Хорошо. Когда мы подпишем договор?", "All right. When do we sign the contract?"),
        ("Holnap, itt, délután. Rendben?", "Завтра, здесь, днём. Хорошо?", "Tomorrow, here, in the afternoon. Okay?"),
        ("Rendben. Köszönöm! Nagyon tetszik a lakás.", "Хорошо. Спасибо! Квартира мне очень нравится.", "Okay. Thank you! I like the flat very much."),
        ("Örülök! Akkor holnap találkozunk!", "Рад! Тогда до завтра!", "I'm glad! Then see you tomorrow!"),
    ],
    # Two colleagues talk about their families.
    "m4-u3-csalad": [
        ("Anna, van testvéred?", "Анна, у тебя есть брат или сестра?", "Anna, do you have siblings?"),
        ("Igen, van egy nővérem és egy öcsém.", "Да, у меня есть старшая сестра и младший брат.", "Yes, I have an older sister and a younger brother."),
        ("Hány éves a nővéred?", "Сколько лет твоей сестре?", "How old is your sister?"),
        ("Harminc éves. Férjnél van, két gyereke van.", "Тридцать. Она замужем, у неё двое детей.", "Thirty. She's married, she has two children."),
        ("Nagyon szép! Hol laknak?", "Здорово! Где они живут?", "Lovely! Where do they live?"),
        ("Itt laknak a városban.", "Они живут здесь, в городе.", "They live here in the city."),
        ("És a szüleid? Hol élnek?", "А твои родители? Где они живут?", "And your parents? Where do they live?"),
        ("Anyám és apám otthon élnek. Már nyugdíjasok.", "Мама и папа живут дома. Они уже на пенсии.", "My mum and dad live back home. They're already retired."),
        ("És te? Van feleséged?", "А ты? У тебя есть жена?", "And you? Do you have a wife?"),
        ("Nincs, egyedülálló vagyok. De van egy barátnőm.", "Нет, я не женат. Но у меня есть девушка.", "No, I'm single. But I have a girlfriend."),
        ("Nagyon jó! Hogy hívják?", "Отлично! Как её зовут?", "Great! What's her name?"),
        ("Évának hívják.", "Её зовут Éва.", "Her name is Éva."),
    ],
    # Where were you yesterday?
    "m4-u4-multido": [
        ("Szia, Péter! Hol voltál tegnap?", "Привет, Петер! Где ты был вчера?", "Hi, Péter! Where were you yesterday?"),
        ("Szia! Sajnálom, beteg voltam.", "Привет! Прости, я болел.", "Hi! Sorry, I was ill."),
        ("Tényleg? Mi történt?", "Правда? Что случилось?", "Really? What happened?"),
        ("Reggel fájt a fejem, aztán hirtelen jött a láz.", "Утром болела голова, потом вдруг поднялась температура.", "In the morning my head hurt, then suddenly I got a fever."),
        ("Orvoshoz mentél?", "Ты ходил к врачу?", "Did you go to a doctor?"),
        ("Igen, délután ott voltam. Azóta jobban vagyok.", "Да, днём был у него. С тех пор мне лучше.", "Yes, I was there in the afternoon. I've been better since."),
        ("Örülök! Tegnap sok munka volt.", "Рада! Вчера было много работы.", "I'm glad! There was a lot of work yesterday."),
        ("Sajnálom! Ma sokat dolgozom.", "Прости! Сегодня буду много работать.", "Sorry! I'll be working a lot today."),
        ("Nem baj. Végül minden rendben lett.", "Ничего. В конце концов всё получилось.", "Never mind. In the end everything turned out fine."),
        ("Szerencsére! Akkor megyek dolgozni.", "К счастью! Тогда я иду работать.", "Luckily! Then I'm off to work."),
        ("Jó! Majd később találkozunk.", "Хорошо! Увидимся позже.", "Good! See you later."),
        ("Rendben. Szia!", "Ладно. Пока!", "Okay. Bye!"),
    ],
}
