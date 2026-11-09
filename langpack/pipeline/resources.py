"""The hand-off: what to read, watch and listen to once the course runs out.

course.py prescribes 64 units in three tiers and then stops. This file is what the app
shows next — and the stopping is the design, not a gap in it. [R11] measures the
effect of reading VOLUME, and volume is the one thing a syllabus cannot hand
you: past some point the learner picks their own material or they read a few
thousand curated words a month forever.
See .claude/research/11-extensive-reading.md.

The selection principle is a single question asked of every candidate: *what
does this unlock, and on what day does it stop being a wall?* Hence `level` is
the level a source becomes USABLE at, not the level it is written at. Telex is
written at C1 and is listed B1, because at B1 you can get through one Telex item
a day with the reader's gloss key and come out ahead. A source that is merely
more of what is already listed is not a resource, and is not here.

What that question forces, worked through:

  * **The ramp is covered from the bottom.** An A2 learner pointed at a national
    daily has been handed a wall. Simplified texts, folk tales and a graded
    reader hold the first rung; mainstream news is the second; literature and
    unscripted speech are the third. Difficulty is controlled by what you pick,
    which is exactly the skill the course was not able to teach.
  * **Audio counts as audio only when a transcript exists.** The Listen screen
    needs both halves. Sources without one are still listed — an unscripted
    podcast is the only honest measure of where your ear actually is — but the
    blurb says so plainly, so nobody imports one and blames the screen.
  * **The categories a person living in Hungary needs beat the categories a
    course would list.** A dictionary that shows whole paradigms rather than one
    citation form, a conjugator, the spelling arbiter, the tax office, and a
    place to find a human being.
  * **Loop 4 gets a row.** The app says everywhere else that it cannot be your
    interlocutor. The catalogue is where it has to say where to find one.
  * **Licence belongs in the blurb, not a footnote.** The reader invites you to
    paste text in. Public-domain and CC sources say copy freely; in-copyright
    ones say read it there and bring an excerpt. Nothing is listed that forbids
    the learner reading it.

`free` is False for anything paywalled or subscription-only, and the blurb says
so in words as well, because a boolean in the corner of a card is not a
disclosure.

Every URL below resolved when this file was written. Link rot is real and this
list has an expiry date: treat a dead row as a content bug, not a fact of life.
"""

from __future__ import annotations

RESOURCES: list[dict] = [
    # ── news ────────────────────────────────────────────────────────────────
    # The second rung. Everything here is written for natives; what makes it
    # reachable at B1 is length — a news item is finishable in one sitting, and
    # finishing is what keeps the volume up.
    {
        "kind": "news",
        "title": "Telex",
        "url": "https://telex.hu",
        "level": "B1",
        "blurb_ru": "Первое место, куда идти, когда учебные тексты кончились: короткие заметки, ясный синтаксис и каждый день что-нибудь на двести-триста слов. Начинайте, когда разбираете заголовок без словаря — примерно на четвёртом месяце курса.",
        "blurb_en": "The first place to go when the course texts run out: short items, plain syntax, and something every day that is only two or three hundred words. Start when you can parse a headline unaided — around month four.",
        "free": True,
        "ord": 100,
    },
    {
        "kind": "news",
        "title": "444",
        "url": "https://444.hu",
        "level": "B1",
        "blurb_ru": "Разговорнее и злее, чем Telex: ирония, сленг и обороты, которых нет ни в одном учебнике. Берите вторым источником, когда Telex перестал быть трудным, — живой язык начинается именно здесь.",
        "blurb_en": "Looser and sharper than Telex: irony, slang and turns of phrase no textbook contains. Add it as your second news source once Telex stops being hard — this is where the living language starts.",
        "free": True,
        "ord": 110,
    },
    {
        "kind": "news",
        "title": "Szabad Európa",
        "url": "https://www.szabadeuropa.hu",
        "level": "B1",
        "blurb_ru": "Пишут для широкой аудитории, неспешно и нейтрально — читается заметно легче венгерской прессы вообще. Отдельная выгода: многие сюжеты вы уже знаете по русским новостям, так что силы уходят только на язык, а не на содержание.",
        "blurb_en": "Written for a broad audience in a slower, neutral register — noticeably easier than the Hungarian press in general. The bonus is that you already know many of the stories, so all your effort goes on the language rather than the content.",
        "free": True,
        "ord": 120,
    },
    {
        "kind": "news",
        "title": "Euronews magyarul",
        "url": "https://hu.euronews.com",
        "level": "B1",
        "blurb_ru": "Одна и та же новость выходит по-венгерски и по-русски — откройте две вкладки и получите готовый параллельный текст. Лучший способ проверить догадку о значении, не разрушая чтение словарём.",
        "blurb_en": "The same story runs in Hungarian and in Russian — open two tabs and you have a parallel text for free. The best way to check a guess at a meaning without breaking your reading to look it up.",
        "free": True,
        "ord": 130,
    },
    {
        "kind": "news",
        "title": "Híradó (közmédia)",
        "url": "https://hirado.hu",
        "level": "B1",
        "blurb_ru": "Новости общественного вещателя: сухой официальный венгерский — ровно тот, каким написаны письма из учреждений. Читайте вместе с Telex: одно и то же событие, изложенное двумя разными словарями, — это отдельный урок.",
        "blurb_en": "The public broadcaster's news: dry, official Hungarian — the same register your letters from institutions are written in. Read it alongside Telex: one event told with two different vocabularies is a lesson in itself.",
        "free": True,
        "ord": 140,
    },
    {
        "kind": "news",
        "title": "HVG",
        "url": "https://hvg.hu",
        "level": "B2",
        "blurb_ru": "Длинная аналитика и экономика — тот регистр, в котором пишут отчёты и договоры. Не раньше B2: предложение здесь бывает длиной в абзац. Часть материалов закрыта подпиской HVG360, но открытой ленты хватает надолго.",
        "blurb_en": "Long-form analysis and business writing — the register that reports and contracts are written in. Not before B2: a sentence here can run a paragraph. Part of the site sits behind the HVG360 subscription, though the open feed will keep you busy for months.",
        "free": False,
        "ord": 150,
    },

    # ── reading ─────────────────────────────────────────────────────────────
    # The first rung and the third. Simplified prose to start on, then the
    # library — plus the one register that costs money to misread.
    {
        "kind": "reading",
        "title": "Easy Hungarian — Szövegek",
        "url": "https://www.easyhungarian.com/texts.html",
        "level": "A2",
        "blurb_ru": "Тексты, специально переписанные простым языком, — почти единственное, что есть для венгерского между учебником и настоящей прессой. Отсюда и начинайте свободное чтение, не дожидаясь конца курса. Еженедельный журнал платный, но открытых текстов хватит на месяцы.",
        "blurb_en": "Texts deliberately rewritten in simple Hungarian — close to the only thing that exists between a textbook and the real press. Start your free reading here, without waiting for the course to end. The weekly magazine is paid, but the open texts will last you months.",
        "free": True,
        "ord": 200,
    },
    {
        "kind": "reading",
        "title": "Magyar Wikipédia",
        "url": "https://hu.wikipedia.org",
        "level": "A2",
        "blurb_ru": "Статья о том, что вы и так знаете — о своём городе, любимом фильме, футбольном клубе, — читается на уровень выше вашего, потому что содержание уже у вас в голове. Лицензия CC BY-SA: текст можно спокойно копировать в «Читалку».",
        "blurb_en": "An article about something you already know — your own town, a film you love, a football club — reads a level above your own, because you are supplying the content yourself. CC BY-SA licensed, so you can paste it straight into the Reader.",
        "free": True,
        "ord": 210,
    },
    {
        "kind": "reading",
        "title": "NAV — az adóhivatal",
        "url": "https://nav.gov.hu",
        "level": "B1",
        "blurb_ru": "Сайт налоговой — не развлечение, а тренировка того регистра, ошибка в котором стоит денег. Разберите одну страницу про свою настоящую ситуацию — adókártya, регистрация, декларация; это полезнее десяти нейтральных текстов.",
        "blurb_en": "The tax office is not entertainment; it is practice in the one register where misreading costs money. Work through a single page about a situation you actually have — adókártya, registration, a return — and it will do more for you than ten neutral texts.",
        "free": True,
        "ord": 220,
    },
    {
        "kind": "reading",
        "title": "Nyelv és Tudomány (nyest.hu)",
        "url": "https://www.nyest.hu",
        "level": "B2",
        "blurb_ru": "Научно-популярно о языке — и по большей части о венгерском. Редкий случай, когда содержание статьи прямо помогает тому, что вы учите: про гармонию гласных здесь пишут по-венгерски и для венгров.",
        "blurb_en": "Popular writing about language, most of it about Hungarian. A rare case where the content of the article feeds directly into what you are studying: vowel harmony discussed in Hungarian, for Hungarians.",
        "free": True,
        "ord": 230,
    },
    {
        "kind": "reading",
        "title": "Magyar Elektronikus Könyvtár",
        "url": "https://mek.oszk.hu",
        "level": "B2",
        "blurb_ru": "Электронная библиотека Национальной библиотеки: венгерская классика целиком, бесплатно и часто в чистом текстовом виде. Открывайте, когда газета перестала быть испытанием: язык здесь старше и плотнее, зато права на большинство книг истекли и копировать их можно без опасений.",
        "blurb_en": "The National Library's digital collection: Hungarian classics in full, free, and often as plain text. Open it once the newspaper has stopped being a test — the language is older and denser, but most of it is out of copyright and safe to copy.",
        "free": True,
        "ord": 240,
    },
    {
        "kind": "reading",
        "title": "Digitális Irodalmi Akadémia",
        "url": "https://pim.hu/hu/dia",
        "level": "B2",
        "blurb_ru": "Современная венгерская проза и поэзия XX–XXI веков, выложенная для чтения самими авторами и наследниками. Читать можно свободно, но тексты под авторским правом: держите книгу в браузере, а в «Читалку» переносите отрывок, а не том целиком.",
        "blurb_en": "Twentieth- and twenty-first-century Hungarian prose and poetry, put online for reading by the authors and their estates. Free to read, but in copyright: keep the book in the browser and bring an excerpt to the Reader, not the whole volume.",
        "free": True,
        "ord": 250,
    },

    # ── listening ───────────────────────────────────────────────────────────
    # The Listen screen needs a transcript. The first two rows have one; the
    # last three are ear training and say so.
    {
        "kind": "listening",
        "title": "FSI Hungarian Graded Reader",
        "url": "https://www.livelingua.com/course/fsi/hungarian_-_graded_reader",
        "level": "A2",
        "blurb_ru": "Пятьдесят шесть текстов по нарастающей сложности и к ним четыре с половиной часа начитки — ровно то, что нужно экрану «Слушание»: звук и точная расшифровка одного и того же. Материал американского госдепартамента, общественное достояние, копировать можно как угодно; лексика местами устарела, но выстроенность подачи это окупает.",
        "blurb_en": "Fifty-six texts of rising difficulty with four and a half hours of recordings — exactly what the Listen screen wants: audio and an exact transcript of the same thing. A US government course, public domain, free to copy; some vocabulary has dated, but the grading more than pays for it.",
        "free": True,
        "ord": 300,
    },
    {
        "kind": "listening",
        "title": "LibriVox — magyar hangoskönyvek",
        "url": "https://librivox.org/search?primary_key=8&search_category=language&search_page=1&search_form=advanced",
        "level": "B1",
        "blurb_ru": "Аудиокниги, начитанные волонтёрами по книгам из общественного достояния, — а сам текст лежит в MEK, так что расшифровка у вас уже есть. Венгерского здесь немного и качество записи разное, зато это бесплатное и совершенно законное длинное аудио с текстом.",
        "blurb_en": "Volunteer-read audiobooks of public-domain titles — and the text itself is in MEK, so you already have the transcript. There is not much Hungarian and the recording quality varies, but this is long-form audio with text, free and entirely legal.",
        "free": True,
        "ord": 310,
    },
    {
        "kind": "listening",
        "title": "Plain Hungarian",
        "url": "https://plainhungarian.com",
        "level": "A2",
        "blurb_ru": "Короткие истории, начитанные медленно и специально для учащихся, с пословной расшифровкой и списком слов. Сами выпуски бесплатны в подкаст-приложениях, но расшифровки и словники — по платной подписке, а без них для «Слушания» толку немного.",
        "blurb_en": "Short stories read slowly and made for learners, with word-by-word transcripts and vocabulary lists. The episodes themselves are free in any podcast app, but the transcripts and word lists sit behind a paid membership — and without them there is little for the Listen screen to work with.",
        "free": False,
        "ord": 320,
    },
    {
        "kind": "listening",
        "title": "Szabad Európa — podcastok",
        "url": "https://www.szabadeuropa.hu/podcasts",
        "level": "B2",
        "blurb_ru": "Студийная речь без уличного шума и без перебиваний — самое разборчивое венгерское аудио в новостях. Расшифровок нет, поэтому для «Слушания» не годится: это тренировка уха, а не материал для разбора. Включайте, когда на слух понимаете примерно половину.",
        "blurb_en": "Studio speech with no street noise and nobody talking over anybody — the most intelligible Hungarian audio in the news. There are no transcripts, so it is not material for the Listen screen: this is ear training, not something to take apart. Start when you catch roughly half.",
        "free": True,
        "ord": 330,
    },
    {
        "kind": "listening",
        "title": "Telex — podcastok",
        "url": "https://telex.hu/podcast",
        "level": "B2",
        "blurb_ru": "Неподготовленный разговор нескольких человек — то, обо что спотыкается всякий, кто учился по дикторской речи. Расшифровок нет. Возвращайтесь сюда раз в месяц как к замеру: то, что было шумом, постепенно становится словами.",
        "blurb_en": "Several people talking unscripted — the thing everyone who learned from newsreaders trips over. No transcripts. Come back once a month as a measurement: what used to be noise slowly turns into words.",
        "free": True,
        "ord": 340,
    },

    # ── video ───────────────────────────────────────────────────────────────
    {
        "kind": "video",
        "title": "Easy Hungarian (YouTube)",
        "url": "https://www.youtube.com/@EasyHungarian",
        "level": "A2",
        "blurb_ru": "Уличные интервью с двойными субтитрами — венгерский и английский одновременно. Это то же аудио с расшифровкой, только в виде видео: слышно, как люди на самом деле проглатывают окончания. Смотрите параллельно с курсом, не откладывая на потом.",
        "blurb_en": "Street interviews with dual subtitles, Hungarian and English at once. It is audio-with-transcript in video form: you hear how people actually swallow their endings. Watch it alongside the course rather than after it.",
        "free": True,
        "ord": 400,
    },
    {
        "kind": "video",
        "title": "Magyar népmesék (Kecskemétfilm)",
        "url": "https://www.youtube.com/@Kecskemetfilm",
        "level": "A2",
        "blurb_ru": "«Венгерские народные сказки»: медленный закадровый рассказчик, повторяющиеся обороты и сюжет, который держит внимание и без знания языка. Детское здесь оправдано не «простотой», а тем, что одну серию можно пересмотреть пять раз и не заскучать. Субтитров нет — только автоматические.",
        "blurb_en": "Hungarian Folk Tales: a slow narrator, formulaic phrasing, and a story that holds you without the language. The point of children's material is not that it is simple but that you can rewatch one episode five times without boredom. No real subtitles — only the automatic ones.",
        "free": True,
        "ord": 410,
    },
    {
        "kind": "video",
        "title": "MédiaKlikk",
        "url": "https://mediaklikk.hu",
        "level": "B1",
        "blurb_ru": "Архив общественного телевидения и радио: новости, документальное, детские программы. Наличие субтитров зависит от передачи — ищите пометку о скрытых титрах: с ними это лучший бесплатный источник длинного видео.",
        "blurb_en": "The public broadcaster's television and radio archive: news, documentaries, children's programming. Subtitles depend on the programme — look for the closed-caption marking, and where it is there this is the best free source of long-form video.",
        "free": True,
        "ord": 420,
    },

    # ── reference ───────────────────────────────────────────────────────────
    # A dictionary that shows paradigms, a conjugator, a grammar, an arbiter.
    # For an agglutinative language a citation-form-only dictionary is close to
    # useless, which is why the first row is Wiktionary and not a bilingual.
    {
        "kind": "reference",
        "title": "Wiktionary — Hungarian lemmas",
        "url": "https://en.wiktionary.org/wiki/Category:Hungarian_lemmas",
        "level": "A2",
        "blurb_ru": "Лучший словарь для агглютинативного языка: у каждого слова развёрнутая таблица всех форм, а не одна словарная. Сюда идите, когда наш разбор помечен как неточный. Английский раздел полнее венгерского; лицензия CC BY-SA.",
        "blurb_en": "The best dictionary for an agglutinative language: every word carries a full inflection table rather than a single citation form. Come here when our analysis is flagged uncertain. The English edition is fuller than the Hungarian one; CC BY-SA.",
        "free": True,
        "ord": 500,
    },
    {
        "kind": "reference",
        "title": "Cooljugator — magyar igék",
        "url": "https://cooljugator.com/hu",
        "level": "A2",
        "blurb_ru": "Быстрая таблица спряжения для любого глагола, с определённым и неопределённым спряжением рядом. Формы генерируются автоматически, так что редкий глагол стоит перепроверить, — но на вопрос «kérek или kérem» отвечает за две секунды.",
        "blurb_en": "A fast conjugation table for any verb, with the definite and indefinite paradigms side by side. The forms are generated, so a rare verb is worth double-checking — but it answers kérek-or-kérem in two seconds.",
        "free": True,
        "ord": 510,
    },
    {
        "kind": "reference",
        "title": "HungarianReference.com",
        "url": "https://www.hungarianreference.com",
        "level": "A2",
        "blurb_ru": "Грамматический справочник на английском, написанный человеком, а не издательством: коротко, по делу и с примерами. Держите открытым рядом с курсом — сюда идут, когда объяснения в приложении не хватило.",
        "blurb_en": "A grammar reference in English written by a person rather than a publisher: short, to the point, worked through with examples. Keep it open beside the course — it is where you go when the app's explanation was not enough.",
        "free": True,
        "ord": 520,
    },
    {
        "kind": "reference",
        "title": "Glosbe magyar–orosz",
        "url": "https://glosbe.com/hu/ru",
        "level": "A2",
        "blurb_ru": "Венгерско-русский с примерами из параллельных корпусов — незаменим, когда нужно не значение, а сочетаемость. Примеры подобраны машинно и качество перевода скачет: доверяйте тому, что повторяется, а не единичному.",
        "blurb_en": "Hungarian–Russian with examples pulled from parallel corpora — invaluable when what you need is not the meaning but what the word goes with. The examples are machine-matched and the translation quality swings: trust what repeats, not what appears once.",
        "free": True,
        "ord": 530,
    },
    {
        "kind": "reference",
        "title": "Helyesírási tanácsadó (MTA)",
        "url": "https://helyesiras.mta.hu",
        "level": "B1",
        "blurb_ru": "Сервис Института языкознания: скажет, как присоединить окончание к нестандартному слову, слитно или через дефис пишется составное, как склоняется фамилия. Единственный арбитр, на которого можно сослаться в споре.",
        "blurb_en": "The Linguistics Institute's service: how to attach a suffix to an awkward word, whether a compound is one word or hyphenated, how a surname declines. The one arbiter you can actually cite in an argument.",
        "free": True,
        "ord": 540,
    },
    {
        "kind": "reference",
        "title": "WikiSzótár",
        "url": "https://wikiszotar.hu",
        "level": "B2",
        "blurb_ru": "Толковый словарь: значение объясняется по-венгерски. Переходите на него, когда перевод начинает мешать, — примерно там же, где вы перестаёте переводить в голове. До B2 будет только раздражать.",
        "blurb_en": "A monolingual dictionary: meanings explained in Hungarian. Switch to it when translating starts getting in the way — roughly the point where you stop translating in your head. Before B2 it will only annoy you.",
        "free": True,
        "ord": 550,
    },

    # ── practice ────────────────────────────────────────────────────────────
    # Loop 4 lives here. The app cannot be your interlocutor; the least it can
    # do is say where one is.
    {
        "kind": "practice",
        "title": "italki",
        "url": "https://www.italki.com/en/teachers/hungarian",
        "level": "A2",
        "blurb_ru": "Живой преподаватель — то, что приложение не заменяет и не притворяется, что заменяет. Уроки платные, от нескольких евро за полчаса с community tutor. Одного разговора в неделю начиная с четвёртого месяца хватает, чтобы речь не отставала от чтения.",
        "blurb_en": "A live teacher: the thing this app does not replace and does not pretend to. Lessons are paid, from a few euros for half an hour with a community tutor. One conversation a week from month four onwards is enough to keep your speaking from falling behind your reading.",
        "free": False,
        "ord": 600,
    },
    {
        "kind": "practice",
        "title": "ConversationExchange",
        "url": "https://www.conversationexchange.com",
        "level": "B1",
        "blurb_ru": "Бесплатный поиск партнёра по обмену: вы ему русский или английский, он вам венгерский. Работает начиная примерно с B1 — раньше разговор целиком держит носитель, и вы получаете урок вежливости, а не практику.",
        "blurb_en": "A free way to find an exchange partner: your Russian or English for their Hungarian. It works from about B1 — earlier than that the native speaker carries the whole conversation and you get a lesson in politeness rather than practice.",
        "free": True,
        "ord": 610,
    },
    {
        "kind": "practice",
        "title": "ECL nyelvvizsga — mintafeladatok",
        "url": "https://eclexam.eu",
        "level": "B2",
        "blurb_ru": "Экзамен ECL и, главное, бесплатные образцы заданий прошлых лет вместе с аудио. Даже если сертификат вам не нужен, один прорешанный вариант покажет ваш настоящий уровень честнее любого внутреннего прогресс-бара. Сам экзамен платный.",
        "blurb_en": "The ECL exam and, more to the point, free past sample papers with their audio. Even if you never want the certificate, working through one paper will tell you your real level more honestly than any progress bar in here. Sitting the exam itself costs money.",
        "free": True,
        "ord": 620,
    },
]
