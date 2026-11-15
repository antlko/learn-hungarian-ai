"""Keep the real langpack/texts/*.json (the pilot content, which changes) out of
the unit tests: they test the pipeline on fixtures and on the legacy modules."""

import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))


@pytest.fixture(autouse=True, scope="session")
def _no_real_texts(tmp_path_factory):
    from pipeline import texts

    old, old_talk = texts.TEXTS_DIR, texts.TALK_DIR
    texts.TEXTS_DIR = tmp_path_factory.mktemp("no-texts")
    texts.TALK_DIR = tmp_path_factory.mktemp("no-talk")
    yield
    texts.TEXTS_DIR, texts.TALK_DIR = old, old_talk
