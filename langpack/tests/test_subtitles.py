"""The subtitle filters, against the shapes that actually occur in OPUS.

Every case here was copied out of the real corpus. The filters exist because
subtitle text is dirty in specific, repetitive ways, and a regression in any one
of them puts film-credit text or half a sentence in front of a learner.
"""

from pipeline import subtitles as sub


def test_clean_strips_subtitle_furniture():
    assert sub.clean("<i>Jó napot!</i>") == "Jó napot!"
    assert sub.clean("- Szia!") == "Szia!"
    assert sub.clean("♪ Egy dal ♪") == "Egy dal"
    assert sub.clean("{\\an8}Felirat") == "Felirat"
    # ASS style overrides split mid-tag, which is how the Russian side arrives.
    assert "298" not in sub.clean("298)}Код Гиасс")


def test_usable_requires_a_whole_sentence():
    ok = "Nagyon szeretem a kávét."
    assert sub.usable(ok, "Я очень люблю кофе.")
    # A fragment continued on the next card.
    assert not sub.usable("Nagyon szeretem a", "Я очень люблю")
    assert not sub.usable("...és akkor elment.", "...и тогда ушёл.")
    # Too short to carry grammar, too long to be a first encounter.
    assert not sub.usable("Igen.", "Да.")
    assert not sub.usable(" ".join(["szó"] * 20) + ".", "x " * 20)


def test_usable_rejects_shouting_and_foreign_lines():
    assert not sub.usable("EZ EGY NAGY HÁZ.", "ЭТО БОЛЬШОЙ ДОМ.")
    # Cyrillic in the "Hungarian" column.
    assert not sub.usable("Это дом.", "Это дом.")


def test_alignment_guards():
    # NUMERALS have to survive translation. Spelled-out numbers are not
    # checked — this guard reads digits, and `öt` against `три` is a job for a
    # translation model, not a regex.
    assert not sub.aligned("5 óra van.", "Сейчас 3 часа.")
    assert sub.aligned("5 óra van.", "Сейчас 5 часов.") is True
    # A question stays a question.
    assert not sub.aligned("Hol vagy?", "Я здесь.")
    # Translator credits are not a translation.
    assert not sub.aligned("Jó napot!", "Перевод: Иван")
    # Wild length mismatch.
    assert not sub.aligned("Jó napot kívánok!", "Здравствуйте, меня зовут Иван, и я очень рад вас видеть здесь сегодня вечером.")


def test_best_translation_needs_agreement():
    from collections import Counter

    # A clear winner is trusted.
    assert sub.best_translation(Counter({"Привет!": 9, "шум": 1})) == "Привет!"
    # A single observation is not evidence.
    assert sub.best_translation(Counter({"Привет!": 1})) == ""
    # Noise with no majority is dropped rather than guessed at: this is the
    # signature of a misaligned document, where every occurrence pairs with a
    # different neighbour.
    votes = Counter({f"вариант {i}": 1 for i in range(20)})
    assert sub.best_translation(votes) == ""


def test_wholesome_keeps_ordinary_dialogue():
    assert sub.wholesome("Kérek egy kávét.")
    assert not sub.wholesome("Kurva jó!")


def _pool(n_short: int, n_long: int) -> list:
    """A pool where short sentences recur far more, as they really do."""
    out = []
    for i in range(n_short):
        out.append(sub.Selected(hu=f"Ez egy {i}.", ru=f"Это {i}.", en="", count=1000 - i))
    for i in range(n_long):
        words = " ".join(["szó"] * 8)
        out.append(sub.Selected(hu=f"Ez {words} {i}.", ru=f"Это длинное {i}.", en="", count=5))
    return out


def test_choose_does_not_collapse_onto_short_sentences():
    """The bug this pins: recurrence alone gave 79% three-token utterances.

    Short lines repeat far more than long ones, so an unconstrained top-N
    starves the 6-14 band the reader and the n+1 rail live in. The bands get
    quotas so one cannot eat the other.
    """
    pool = _pool(n_short=5_000, n_long=5_000)
    picked, stats = sub.choose(pool, known={}, limit=1000)

    assert len(picked) == 1000
    long_ones = [s for s in picked if len(sub.tokens(s.hu)) >= 6]
    # The 6-14 band's share, give or take the rounding in the quota.
    assert 400 <= len(long_ones) <= 460, f"got {len(long_ones)} long sentences"
    assert stats["selected"] == 1000


def test_choose_releases_an_unfilled_quota():
    """A band with nothing in it must not shrink the result."""
    pool = _pool(n_short=5_000, n_long=0)
    picked, _ = sub.choose(pool, known={}, limit=1000)
    assert len(picked) == 1000


def test_choose_prefers_sentences_that_have_a_translation():
    pool = [
        sub.Selected(hu="Ez egy ház.", ru="", en="", count=9_999),
        sub.Selected(hu="Ez egy fa.", ru="Это дерево.", en="", count=1),
    ]
    picked, _ = sub.choose(pool, known={}, limit=1)
    assert picked[0].ru == "Это дерево."
