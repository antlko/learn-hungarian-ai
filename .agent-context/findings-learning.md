# Findings: learning loop, content, level (researcher, 2026-11-08; partly unverified)

## "No mistake in a row" — no code path requires an N-in-a-row streak
- Word graduation: FSRS learning steps {1,10} min; Again resets to 1-min step. UI shows «учу — шаги через минуты» (ru.json:585). Most likely cause of the complaint (a miss visibly resets a word).
- Topic gate blocked→interleaved: EWMA accuracy (srs/emit.go:121-134), window 20, needs >=20 answers and >=80%, demote <70% (drill/schedule.go:275-307). Mastered: 50 interleaved answers >=90% (schedule.go:326-352). Deliberate (R5) — keep.
- Placement: 3 probes/topic, 2 must be right, ladder ends after two consecutive failed topics (curriculum/placement.go:27-35).
- Checkpoints: all-criteria gate (api/handlers_progress.go:294-302).
- Course unit done = all words learning/known (course.Resolve). Not gated.
- Lenient options consistent with invariants: show «верно N из M за последнее время» instead of bare %; label a miss with when it returns; make placement bar softer + «ориентировочно»; keep FSRS maths unmodified (learning steps are config).

## Content (full pack manifest; starter in docs/LANGPACK.md:418-426)
- Lemmas 35,311 (starter 5,132); wordforms 1.98M; sentences 60,000 (starter 12,000, Tatoeba only); frequency-ranked lemmas 10,578; CEFR bands by rank A1<=900 A2<=2000 B1<=4500 B2<=8000 C1<=16000 (langpack/pipeline/frequency.py:18-24; estimates).
- 28 grammar topics (langpack/pipeline/curriculum.py:55-130), lessons in lessons.py (>=800 chars each). A1 10, A2 8, B1 7, B2 3. ~13 have drills (unverified).
- Course: 24 units/6 months (course_m1..m6.py), 853 words, 15-60 words/unit. Titles: M1 greetings, survival, numbers/money/time, te/ön; M2 shop/market, street/transport, flat, size/price; M3 office, doctor, bank, post; M4 work, renting, people/family, the past; M5 broken things, phone/appointments, school/authorities, restaurants; M6 opinion, feelings, abstract nouns, connectives.
- Minimal pairs ~30-32; HVPT 18 contrasts. No shipped reading texts (library user-populated). 29 external resources (resources.py). Placement generated.
- Gaps: numbers/time as grammar; graded texts; thematic vocab decks beyond 24 units; 853 course words vs 10k ranked lemmas.

## Adding content
- Unit: dict in course_mN.py UNITS (code, month, ord, titles/goal/situation ru+en, topic_codes, words (lemma,upos,gloss_ru,gloss_en,note)); validators course.validate (course.py:80-125). Topic: Topic(...) in curriculum.py + lesson in lessons.py; prereqs earlier; feature ids in features.py. Build: make langpack; make starter with ABSOLUTE STARTER_DIR; commit backend/internal/lang/starter/*.gz. Gates: make test, make docs-check, langpack/tests/.

## Level signal
- Exists: text level estimator (mining/level.go:85-174, ADR-023), checkpoints (stats/checkpoint.go), stats.LemmaCount. No learner-level estimate yet.
- Honest method: CEFR band = highest frequency band with >=90% (tunable) lemmas known/learning, gated by checkpoints, with stated uncertainty and band-share bars; bands are estimates.

## Words list
- WordsView.vue:188-189 shows only lemma status badge; stage (new/learning/young/mature) only in detail.
