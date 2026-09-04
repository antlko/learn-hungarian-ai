"""The lesson text is content, and content regresses silently.

These do not judge whether a lesson is good — nothing automatable does. They
catch the two failures that have actually happened: a topic added with a
placeholder explanation, and a lesson written in one locale and not the other.
"""

from pipeline import curriculum, lessons


def test_every_topic_has_a_lesson_in_both_locales():
    missing = [
        t.code for t in curriculum.TOPICS
        if not t.explain_ru.strip() or not t.explain_en.strip()
    ]
    assert missing == [], f"topics with no lesson: {missing}"


def test_no_lesson_is_a_blurb():
    """[R4] is applied by stating the rule, not by gesturing at it.

    Every lesson here was once a single paragraph — 51 characters at the
    shortest — above an exercise asking about a feature the paragraph never
    named.
    """
    short = [
        (t.code, locale, len(text.strip()))
        for t in curriculum.TOPICS
        for locale, text in (("ru", t.explain_ru), ("en", t.explain_en))
        if len(text.strip()) < curriculum.MIN_LESSON_CHARS
    ]
    assert short == [], f"lessons below the floor: {short}"


def test_lessons_has_no_orphans():
    """A lesson keyed to a topic code that no longer exists is dead text."""
    codes = {t.code for t in curriculum.TOPICS}
    orphans = sorted(set(lessons.LESSONS) - codes)
    assert orphans == [], f"lessons for unknown topics: {orphans}"
