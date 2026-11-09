"""The HVPT contrast inventory.

Talker variability is the active ingredient of high-variability phonetic
training, not repetition -- see .claude/research/07-hvpt.md. This module owns
*what* to contrast; rendering the audio across talkers is a separate stage.

Selection is driven by what a Russian or English speaker actually cannot hear:

* **Vowel length is phonemic in Hungarian and in neither L1.** `kor`/`kór` are
  different words. This is the highest-yield set and comes first.
* **The rounded front vowels ö/ő and ü/ű** have no Russian or English counterpart
  at all, so they are both a perception and a production problem.
* **Sibilants and affricates** (`s` = [ʃ], `sz` = [s], `zs`, `cs`, `c`) trip up
  English speakers because the orthography inverts their expectations.
* **Palatals** (`gy`, `ny`, `ty`) are close to Russian palatalised stops, so they
  are easier for a Russian speaker -- included, but weighted lower.
* **Gemination** (`hal`/`hall`) is contrastive and routinely inaudible to both.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class MinimalPair:
    a: str
    b: str
    contrast: str
    gloss_a_ru: str
    gloss_b_ru: str
    gloss_a_en: str
    gloss_b_en: str
    # 1 = train first. Ordered by (difficulty for the L1) x (functional load).
    priority: int = 5
    context_a: str = ""
    context_b: str = ""


PAIRS: list[MinimalPair] = [
    # Every member is a WORD the dictionary carries. The inventory once held a
    # dozen non-words — `hus`, `tuz`, `fold`, `sak`, `csél`, `ana` … glossed
    # «(нет слова)» — as the partner of a real word, and the first learner's
    # verdict on the ear trainer was «как будто таких слов нет». They were
    # right. A 2AFC trial presents both members as things the language says;
    # one that is not teaches a form the language does not have, and the
    # honesty rules do not allow it as a drill answer anywhere else. The build
    # refuses a pair whose member is not in `wordform` (`unattested`).
    # --- vowel length: the highest-yield contrast set ------------------------
    MinimalPair("kor", "kór", "o/ó", "возраст, эпоха", "болезнь", "age, era", "disease", 1,
                "A húszas évek kora volt.", "Ez egy ritka kór."),
    MinimalPair("öt", "őt", "ö/ő", "пять", "его/её (вин.)", "five", "him/her (acc.)", 1,
                "Öt óra van.", "Őt kerestem."),
    MinimalPair("tör", "tőr", "ö/ő", "ломает", "кинжал", "breaks", "dagger", 1),
    MinimalPair("hat", "hát", "a/á", "шесть", "спина; ну", "six", "back; well", 1,
                "Hat gyerek.", "Fáj a hátam."),
    MinimalPair("bal", "bál", "a/á", "левый", "бал", "left", "ball (dance)", 1),
    MinimalPair("agy", "ágy", "a/á", "мозг", "кровать", "brain", "bed", 1),
    MinimalPair("szel", "szél", "e/é", "режет", "ветер", "slices", "wind", 1),
    MinimalPair("kerek", "kerék", "e/é", "круглый", "колесо", "round", "wheel", 2),
    MinimalPair("ver", "vér", "e/é", "бьёт", "кровь", "beats", "blood", 2),
    MinimalPair("irt", "írt", "i/í", "истребляет", "написал", "exterminates", "wrote", 2),
    MinimalPair("huzat", "húzat", "u/ú", "сквозняк; чехол", "велит тянуть", "draught; cover",
                "has (it) pulled", 3),
    MinimalPair("hurok", "húrok", "u/ú", "петля", "струны", "loop, noose", "strings", 3),
    MinimalPair("fut", "fűt", "u/ű", "бежит", "топит, отапливает", "runs", "heats", 2),
    MinimalPair("tud", "tűd", "u/ű", "знает", "твоя игла", "knows", "your needle", 3),
    # szurok/szűrök was here and is not a minimal pair: it differs in TWO vowels
    # (u→ű AND o→ö), the same flaw that retired orul/örül below. u/ű stays
    # trained by fut/fűt and tud/tűd. No replacement is added until its clips
    # can be rendered; a pair without audio would be a trial with no stimulus.
    # sor/sör is the textbook real-word pair for this opposition; the previous
    # orul/örül differed in TWO vowels (o→ö AND u→ü) and trained an unattributable blend.
    MinimalPair("sor", "sör", "o/ö", "ряд, очередь", "пиво", "row, queue", "beer", 2),
    MinimalPair("kor", "kör", "o/ö", "возраст", "круг", "age", "circle", 2),
    MinimalPair("tok", "tök", "o/ö", "футляр, чехол", "тыква", "case, sheath", "pumpkin", 2),
    # --- sibilants and affricates -------------------------------------------
    MinimalPair("sál", "szál", "s/sz", "шарф", "нить; штука", "scarf", "thread; a single piece", 2),
    MinimalPair("sír", "szír", "s/sz", "плачет; могила", "сириец", "cries; grave", "Syrian", 3),
    MinimalPair("csók", "sók", "cs/s", "поцелуй", "соли (мн.)", "kiss", "salts", 3),
    MinimalPair("zsír", "szír", "zs/sz", "жир", "сириец", "fat", "Syrian", 3),
    MinimalPair("cikk", "csikk", "c/cs", "статья", "окурок", "article", "cigarette butt", 3),
    # --- palatals: easier for a Russian ear, still trained --------------------
    MinimalPair("hagy", "had", "gy/d", "оставляет", "войско", "leaves", "army", 5),
    # --- gemination ----------------------------------------------------------
    MinimalPair("hal", "hall", "l/ll", "рыба; умирает", "слышит", "fish; dies", "hears", 2,
                "A hal úszik.", "Hallod ezt?"),
    MinimalPair("szál", "száll", "l/ll", "нить; штука", "летит; садится (в транспорт)", "thread",
                "flies; boards", 3),
    MinimalPair("megy", "meggy", "gy/ggy", "идёт", "вишня", "goes", "sour cherry", 2),
    MinimalPair("eset", "esett", "t/tt", "случай", "упал", "case", "fell", 3),
    MinimalPair("örül", "őrül", "ö/ő", "радуется", "сходит с ума", "rejoices", "goes mad", 2),
    MinimalPair("álom", "állom", "l/ll", "сон", "выдерживаю", "dream", "I endure", 4),
]


def validate() -> list[str]:
    """Consistency checks that run as a pack quality gate."""
    problems: list[str] = []
    seen: set[tuple[str, str]] = set()
    for p in PAIRS:
        if p.a == p.b:
            problems.append(f"{p.a}: a pair must contrast two different forms")
        # A contrast must name two phonemes. An inflectional difference is not a
        # perceptual contrast and would train nothing.
        if "/" not in p.contrast:
            problems.append(f"{p.a}/{p.b}: contrast {p.contrast!r} is not a phoneme pair")
        key = (p.a, p.b)
        if key in seen:
            problems.append(f"duplicate pair {p.a}/{p.b}")
        seen.add(key)
        for field, value in (
            ("gloss_a_ru", p.gloss_a_ru), ("gloss_b_ru", p.gloss_b_ru),
            ("gloss_a_en", p.gloss_a_en), ("gloss_b_en", p.gloss_b_en),
        ):
            if not value.strip():
                problems.append(f"{p.a}/{p.b}: missing {field}")
            # A member the language does not have is not a stimulus, whatever
            # the contrast; the dictionary check below is the real gate, this
            # catches the marker before a pack is even opened.
            if "нет слова" in value or "non-word" in value:
                problems.append(f"{p.a}/{p.b}: {field} says the member is not a word")
    return problems


def unattested(conn) -> list[str]:
    """Pair members the pack's own dictionary does not carry.

    A drill's forms must be attested (CLAUDE.md); a 2AFC member is a form the
    learner is told the language has. Run against a pack whose `wordform`
    table is complete — the build, after the lexicon; `refresh`, before it
    writes.
    """
    problems: list[str] = []
    for p in PAIRS:
        for word in (p.a, p.b):
            row = conn.execute(
                "SELECT 1 FROM wordform WHERE form = ? LIMIT 1", (word,)).fetchone()
            if row is None:
                problems.append(f"{p.a}/{p.b}: `{word}` is not in the dictionary")
    return problems


def unattested_in(pack_path) -> list[str]:
    """`unattested`, over a pack on disk, opened read-only."""
    import sqlite3
    conn = sqlite3.connect(f"file:{pack_path}?mode=ro", uri=True)
    try:
        return unattested(conn)
    finally:
        conn.close()


def contrasts() -> dict[str, list[MinimalPair]]:
    """Group pairs by contrast, which is the unit FSRS schedules."""
    out: dict[str, list[MinimalPair]] = {}
    for p in PAIRS:
        out.setdefault(p.contrast, []).append(p)
    return out


# ── the rendering stage ──────────────────────────────────────────────────────
#
# Talker variability is the ACTIVE INGREDIENT of HVPT, not a nicety [R7], so
# rendering is organised around talkers: every pair member × every available
# Piper voice × two speaking rates. Rate variants are recorded as rates, never
# counted as extra talkers — three synthetic voices are three talkers, and the
# app says so rather than inflating the number.
#
# Voices: hu_HU-{anna,berta,imre}-medium from rhasspy/piper-voices. Dataset
# licence per model card: CC0. Clips land under <media>/hvpt/ and are
# registered in the pack's pair_audio table, which is exactly the "audio is
# added by the rendering stage" contract in schema.sql.

import shutil as _shutil

_HAVE_FFMPEG = _shutil.which("ffmpeg") is not None
# Without ffmpeg the render still works, it just ships WAV — degraded, never broken.
_EXT = ".ogg" if _HAVE_FFMPEG else ".wav"

VOICES = ("anna", "berta", "imre")
RATES = (1.0, 0.85)  # 1.0 natural; 0.85 slightly slow — length_scale scales by 1/rate

# Citation-form rendering, because the first render was unusable and the ear
# trainer ran at chance because of it.
#
# Piper's VITS models are trained on SENTENCES. Handed a bare three-letter word
# they produce an out-of-distribution mumble: the original inventory came out
# with a MEDIAN clip of 0.35 s and a minimum of 0.16 s, in which the kor/kór
# vowel-length contrast — the thing the trainer exists to teach — is physically
# not present. Learners reported hearing "different words entirely", and their
# accuracy sat at chance, which is exactly what stimuli carrying no signal
# produce.
#
# Two changes, both standard practice for single-word TTS stimuli:
#
#   * the word is rendered as a one-word SENTENCE ("kór.") — the final stop
#     gives the model a prosodic frame and citation-form articulation instead
#     of a clipped fragment;
#   * CITATION_SCALE slows every render — HVPT stimuli are deliberately clear
#     citation speech in the literature, not conversational tempo, and vowel
#     length needs room to be audible at all.
CITATION_SCALE = 1.7
# Trailing silence so the Opus encode never clips the final consonant — öt/őt
# is decided in the last hundred milliseconds.
SENTENCE_SILENCE = "0.3"
VOICE_LICENSE = "CC0 (OHF voice-datasets, per piper model card)"


def clip_name(word: str, voice: str, rate: float) -> str:
    """The clip's path under the media dir, keyed by the WORD.

    Not by pair id: `refresh` reassigns ids in list order, and a clip named
    `5_a_anna_100` was reused — file present, render skipped — for whatever
    pair 5 had become. The diacritics are folded for the file system and the
    hash keeps `kor` and `kór` apart.
    """
    import hashlib
    import unicodedata
    folded = "".join(
        c for c in unicodedata.normalize("NFKD", word) if not unicodedata.combining(c))
    digest = hashlib.sha1(word.encode("utf-8")).hexdigest()[:6]
    return f"hvpt/{folded}-{digest}_{voice}_{int(rate * 100)}{_EXT}"


def render(pack_path, media_dir, voices_dir, *, only_missing=True):
    """Render every pair member across voices and rates; register in the pack.

    Idempotent: with only_missing (the default) it skips clips whose file and
    row both exist, so a re-run after adding pairs renders just the new ones.
    """
    import sqlite3
    import subprocess
    import sys
    from pathlib import Path

    pack = sqlite3.connect(pack_path)
    out_root = Path(media_dir) / "hvpt"
    out_root.mkdir(parents=True, exist_ok=True)

    pairs = pack.execute("SELECT id, a, b FROM minimal_pair").fetchall()
    rendered = skipped = 0
    for pair_id, a, b in pairs:
        for member, word in (("a", a), ("b", b)):
            for voice in VOICES:
                model = Path(voices_dir) / f"hu_HU-{voice}-medium.onnx"
                if not model.exists():
                    raise SystemExit(f"voice model missing: {model}")
                for rate in RATES:
                    rel = clip_name(word, voice, rate)
                    dest = Path(media_dir) / rel
                    row = pack.execute(
                        "SELECT path FROM pair_audio WHERE pair_id=? AND member=? "
                        "AND talker_id=? AND rate=?",
                        (pair_id, member, voice, rate)).fetchone()
                    # The clip on disk is the expensive artefact; the row is
                    # free. A row whose file is still there (a refresh kept it)
                    # is done; a file under this word's name with no row (a pack
                    # rebuild starts from an empty pair_audio) gets its row back
                    # without a render. Only a clip that exists nowhere is
                    # synthesised.
                    if only_missing and row and (Path(media_dir) / row[0]).exists():
                        skipped += 1
                        continue
                    if only_missing and dest.exists():
                        skipped += 1
                        pack.execute(
                            "INSERT OR REPLACE INTO pair_audio (pair_id, member, talker_id, "
                            "talker_kind, rate, path, license) VALUES (?,?,?,?,?,?,?)",
                            (pair_id, member, voice, "piper", rate, rel, VOICE_LICENSE))
                        continue
                    wav = dest.with_suffix(".wav")
                    subprocess.run(
                        [sys.executable, "-m", "piper",
                         "-m", str(model), "-f", str(wav),
                         "--length-scale", str(round(CITATION_SCALE / rate, 3)),
                         "--sentence-silence", SENTENCE_SILENCE],
                        input=(word + ".").encode(), check=True, capture_output=True)
                    # Transcode to Opus. A single spoken word is ~0.5 s, so the
                    # whole 384-clip inventory drops from ~7 MB of WAV to well
                    # under a megabyte — which is what makes it small enough to
                    # ship INSIDE the binary alongside the starter pack. Every
                    # target browser decodes Opus in Ogg natively.
                    if _HAVE_FFMPEG:
                        subprocess.run(
                            ["ffmpeg", "-y", "-loglevel", "error", "-i", str(wav),
                             "-c:a", "libopus", "-b:a", "24k", "-ac", "1", str(dest)],
                            check=True, capture_output=True)
                        wav.unlink()
                    elif wav != dest:
                        wav.rename(dest)
                    # OR REPLACE, not a pre-checked INSERT: the render is meant
                    # to be re-runnable after an interruption, and a row whose
                    # clip was rewritten must not abort the whole stage.
                    pack.execute(
                        "INSERT OR REPLACE INTO pair_audio (pair_id, member, talker_id, "
                        "talker_kind, rate, path, license) VALUES (?,?,?,?,?,?,?)",
                        (pair_id, member, voice, "piper", rate, rel, VOICE_LICENSE))
                    rendered += 1
        pack.commit()
    pack.commit()
    n_clips = pack.execute("SELECT COUNT(*) FROM pair_audio").fetchone()[0]
    n_talkers = pack.execute(
        "SELECT COUNT(DISTINCT talker_id) FROM pair_audio").fetchone()[0]
    pack.close()
    print(f"rendered {rendered} clips (skipped {skipped}); "
          f"pair_audio now holds {n_clips} rows across {n_talkers} talkers")


if __name__ == "__main__":
    import argparse

    ap = argparse.ArgumentParser(description="Render HVPT pair audio via Piper.")
    ap.add_argument("--pack", required=True)
    ap.add_argument("--media", required=True)
    ap.add_argument("--voices-dir", required=True)
    ap.add_argument("--force", action="store_true", help="re-render existing clips")
    args = ap.parse_args()
    render(args.pack, args.media, args.voices_dir, only_missing=not args.force)
