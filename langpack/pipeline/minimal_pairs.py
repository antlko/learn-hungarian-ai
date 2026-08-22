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
    MinimalPair("hus", "hús", "u/ú", "(нет слова)", "мясо", "(non-word)", "meat", 3),
    MinimalPair("tur", "túr", "u/ú", "(нет слова)", "роет", "(non-word)", "digs", 3),
    MinimalPair("fut", "fűt", "u/ű", "бежит", "топит, отапливает", "runs", "heats", 2),
    MinimalPair("tuz", "tűz", "u/ű", "(нет слова)", "огонь", "(non-word)", "fire", 2),
    # sor/sör is the textbook real-word pair for this opposition; the previous
    # orul/örül differed in TWO vowels (o→ö AND u→ü) and trained an unattributable blend.
    MinimalPair("sor", "sör", "o/ö", "ряд, очередь", "пиво", "row, queue", "beer", 2),

    # --- rounded front vowels: absent from both L1s --------------------------
    MinimalPair("tud", "tűd", "u/ű", "знает", "твоя игла", "knows", "your needle", 3),
    MinimalPair("busz", "bűsz", "u/ű", "автобус", "(нет слова)", "bus", "(non-word)", 4),
    MinimalPair("kor", "kör", "o/ö", "возраст", "круг", "age", "circle", 2),
    MinimalPair("fold", "föld", "o/ö", "(нет слова)", "земля", "(non-word)", "earth", 2),

    # --- sibilants and affricates -------------------------------------------
    MinimalPair("sok", "szok", "s/sz", "много", "(основа: привыкать)", "many", "(stem: get used to)", 2),
    MinimalPair("sír", "szír", "s/sz", "плачет; могила", "сириец", "cries; grave", "Syrian", 2),
    MinimalPair("csak", "sak", "cs/s", "только", "(нет слова)", "only", "(non-word)", 3),
    MinimalPair("zsák", "szák", "zs/sz", "мешок", "(нет слова)", "sack", "(non-word)", 3),
    MinimalPair("cél", "csél", "c/cs", "цель", "(нет слова)", "goal", "(non-word)", 3),

    # --- palatals: easier for a Russian ear, still trained --------------------
    MinimalPair("hagy", "hadj", "gy/dj", "оставляет", "(нет слова)", "leaves", "(non-word)", 5),
    MinimalPair("anya", "ana", "ny/n", "мать", "(нет слова)", "mother", "(non-word)", 5),
    MinimalPair("tyúk", "tuk", "ty/t", "курица", "(нет слова)", "hen", "(non-word)", 5),

    # --- gemination ----------------------------------------------------------
    MinimalPair("hal", "hall", "l/ll", "рыба; умирает", "слышит", "fish; dies", "hears", 2,
                "A hal úszik.", "Hallod ezt?"),
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
    return problems


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
RATES = (1.0, 0.85)  # 1.0 natural; 0.85 slightly slow — length_scale = 1/rate
VOICE_LICENSE = "CC0 (OHF voice-datasets, per piper model card)"


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
                    rel = f"hvpt/{pair_id}_{member}_{voice}_{int(rate*100)}{_EXT}"
                    dest = Path(media_dir) / rel
                    row = pack.execute(
                        "SELECT 1 FROM pair_audio WHERE pair_id=? AND member=? "
                        "AND talker_id=? AND rate=?",
                        (pair_id, member, voice, rate)).fetchone()
                    # The clip on disk is the expensive artefact; the row is
                    # free. Skip only the RENDER when the file is already there
                    # and still upsert the row, so a pack rebuild — which starts
                    # from an empty pair_audio — costs seconds instead of
                    # re-synthesising the whole inventory.
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
                         "--length-scale", str(round(1.0 / rate, 3)),
                         "--sentence-silence", "0.1"],
                        input=word.encode(), check=True, capture_output=True)
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
