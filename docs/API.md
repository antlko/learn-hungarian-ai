# API

REST over JSON. Cookie sessions. All routes under `/api`.

> **Status legend:** ✅ implemented · 🔨 planned this phase · ⏳ later phase.
> Endpoints are documented **when they are built**, in the same change. An endpoint that appears
> here without a status marker is a documentation bug.

## Conventions

- **Errors** — every failure renders as `{"error": "message"}` with an appropriate status, via
  the central Fiber `ErrorHandler`. Handlers return `fiber.NewError(code, msg)`; they never
  format error bodies themselves.
- **Success** — `c.JSON(dto)` or `c.Status(201).JSON(dto)`.
- **Casing** — DTOs are camelCase. `web-ui/src/types/` mirrors
  `backend/internal/api/dto.go`; keep them in sync when a shape changes.
- **Auth** — `session` cookie, `HttpOnly`, `SameSite=Lax`, `Secure` when served over TLS.
  Route guards in the SPA are UX only; **the API enforces roles independently**.
- **Rate limiting** — auth routes are limited per IP.
- **Time** — RFC 3339 UTC everywhere.
- **No pack** — every route that reads the language pack answers **503** with a message rather
  than an empty success. On a normal install this is unreachable: the binary installs its
  embedded starter pack at boot, so a 503 here means that installation failed.
- **Unknown `/api/*` path** — 404 in the API's own `{"error": …}` shape, never `index.html`.

---

## Setup & auth

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/bootstrap` | ✅ | `{needsSetup, packPresent, packVersion, locales}` — unauthenticated; drives the wizard |
| `POST` | `/api/setup` | ✅ | first admin. `{email, password, uiLocale, glossLang}`. 409 once a user exists |
| `POST` | `/api/login` | ✅ | `{email, password}` → sets cookie |
| `POST` | `/api/logout` | ✅ | clears cookie |
| `GET` | `/api/me` | ✅ | current user + settings |

`GET /api/bootstrap` reports `packPresent: true` on a fresh install: the binary carries a
starter pack and installs it at boot ([ARCHITECTURE §3a](ARCHITECTURE.md#3a-pack-bootstrap)).
A client should still handle `false` — it means the embedded pack could not be installed and the
server log says why — but it is a failure state, no longer the normal first-run one.

## Accounts

`POST /api/setup` makes the first admin and then closes itself with a 409. Every account after
that is made here.

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/users` | ✅ | **admin.** `{users: [{id, email, role, createdAt}]}` |
| `POST` | `/api/users` | ✅ | **admin.** `{email, password, role?}` → **201** + the account. Omitted `role` means `learner`, the safe default. 400 on a bad email or a password below the setup wizard's strength floor; **409** on a duplicate email |
| `PATCH` | `/api/users/:id` | ✅ | **admin.** `{role?, password?}` — an omitted field means "leave this alone", which is what lets the UI change a role without resending a password it never had. 200 + the account; 404 unknown id; **409** demoting the last admin |
| `DELETE` | `/api/users/:id` | ✅ | **admin.** **204**; 404 unknown id; **409** deleting the last admin |
| `POST` | `/api/me/password` | ✅ | **any signed-in user** — the one account route open to a learner. `{currentPassword, newPassword}` → **204**; 403 when the current password is wrong; 400 when the new one is too weak |

Five properties are enforced by `handlers_users.go` and pinned by `handlers_users_test.go`:

- **No password hash ever reaches the wire.** `UserDTO` has no field for one and `ListUsers`
  does not select the column; the test greps every account response — create, list, `/api/me` —
  for `$2a$`, `$2b$`, `passwordHash` and `password_hash`.
- **The last admin can be neither demoted nor deleted.** 409, with a message that names the way
  out ("promote someone else first"), because settings and this screen are the only route back
  and there is none once the last admin is gone. The refusal is a rollback, not a partial write,
  and a second admin lifts it — the rule is "at least one", not "this one".
- **A duplicate email is 409, not 500.** The uniqueness index is `COLLATE NOCASE`, so
  `BEA@Example.com` collides with `bea@example.com`; an unmapped constraint error would surface
  as a server error and tell the admin nothing.
- **A rejected half rejects the whole edit.** A `PATCH` carrying both a role change and a
  password below the floor lands neither, so the caller never has to guess what applied.
- **A password change evicts every OTHER session and keeps the one that made it.** That is what
  makes it the way to evict a device you no longer control, without signing yourself out of the
  browser you just typed the new password into. An admin resetting *someone else's* password
  ends all of that account's sessions — a reset is normally the answer to a compromise.

`DELETE` cascades: the account's rows in both state spaces, its notes and its live sessions go
with it. An admin who deletes their own account gets the session cookie cleared in the same
response, so the next request is a login screen rather than a 401 the SPA has to interpret.

A learner calling any `/api/users` route gets **403** and an anonymous caller **401**. The SPA's
guard on `/users` is UX only; this is the boundary that matters, and a test walks every verb.

## Settings & capabilities

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/settings` | ✅ | secrets stripped; presence exposed as `secrets: {openaiKey: true}` |
| `PUT` | `/api/settings` | ✅ | partial; **blank/omitted secret preserves the stored one** |
| `GET` | `/api/capabilities` | ✅ | `[{name, configured, reachable, latencyMs, detail}]`, cached 30 s |
| `POST` | `/api/capabilities/:name/test` | ⏳ | round-trip test against a provider |

## Language & morphology

| Method | Path | Status | Notes |
|---|---|---|---|
| `POST` | `/api/analyze` | ✅ | `{text}` → tokens with `lemma`, `upos`, `feats`, `segments`, `certainty` |
| `GET` | `/api/lemma/:id` | ✅ | lemma, glosses (ru/en + `glossFallback` flag), IPA, frequency, CEFR, paradigm + `paradigmDerived` marking the cells our generator produced rather than the dictionary |
| `GET` | `/api/paradigm/:lemmaId` | ⏳ | folded into `/api/lemma/:id`, which returns `paradigm` inline |
| `GET` | `/api/features` | ✅ | the `morph_feature` inventory with labels and teach order |
| `GET` | `/api/breakdown?form=` | ✅ | one form → `{form, analysis, segments, explain}`: the verified segmentation plus a per-morpheme explanation in both locales, with the caller's own correction already overlaid. 400 without `form` or above 100 characters |
| `POST` | `/api/morph/correction` | ✅ | `{form, lemma?, feats?, note?}` → **201** `{"ok": true}`. Upsert: re-reporting a form replaces the previous correction, because the learner's latest opinion wins. 400 without `form` |
| `DELETE` | `/api/morph/correction?form=` | ✅ | retracts one → `{"ok": true}`. 400 without `form` |
| `GET` | `/api/morph/corrections` | ✅ | `{corrections: [{form, lemma, feats, note, createdAt}]}`, newest first |

`POST /api/analyze` tokens each carry `segments`: the morpheme split, with **verified
boundaries only**. The segmenter peels a suffix only when the analysis carries that feature and
the surface ends in one of its allomorphs; anything it cannot justify comes back as ONE fused
`ending` segment listing the features it must carry (`látta` = `lát` + `[Tense=Past …]`) — no
invented boundaries. `GET /api/breakdown` adds `explain`: per-morpheme labels and the allomorph
rule (`задняя гармония → -ban`), assembled from the pack's own bilingual labels.

A learner's correction (`POST /api/morph/correction {form, lemma?, feats?, note?}`) overlays the
pack's reading for that user and renders with `certainty: "corrected"` — an override is shown as
an override, never as the dictionary. An empty correction (just the form) is a valid report:
"this is wrong, I don't know what's right" is signal for the next pack build.

`POST /api/analyze` response shape:

```json
{ "tokens": [ {
    "idx": 2, "form": "házaimban", "lemma": "ház", "lemmaId": 4211, "upos": "NOUN",
    "feats": { "Case": "Ine", "Number[psor]": "Plur", "Person[psor]": "1", "Number": "Sing" },
    "segments": [
      {"form":"ház","kind":"stem"},
      {"form":"a","kind":"linking"},
      {"form":"im","kind":"suffix","featureId":"Number[psor]=Plur"},
      {"form":"ban","kind":"suffix","featureId":"Case=Ine","allomorphRule":"back"}
    ],
    "certainty": "certain"
} ], "sentences": [{"start":0,"end":5}] }
```

`certainty` ∈ `certain | uncertain | derived` — surfaced in the UI, never hidden.

## Review (SRS)

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/review/queue` | ✅ | today's queue; `?limit=` |
| `POST` | `/api/review/answer` | ✅ | `{cardId, rating, typed?, durationMs}` → next card + graded features |
| `POST` | `/api/review/undo` | ✅ | reverts the last answer including its feature reviews. Delete-and-replay: the log is the truth, state is recomputed from it (bit-exact — the property test in `internal/srs/replay_test.go` covers fuzzed due dates too). 409 when there is nothing to undo |
| `GET` | `/api/review/forecast` | ✅ | 90-day projection for the governor |
| `POST` | `/api/notes` | ✅ | creates a note; the server locates the target span, reads its features off the live analysis, and spawns one card per planned direction |
| `GET/PUT/DELETE` | `/api/notes/:id` | ⏳ | editing and deletion |

## Texts & mining

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET/POST/DELETE` | `/api/texts[/:id]` | ✅ | ingest paste/URL/`.srt`/`.epub` |
| `GET` | `/api/texts/:id` | ✅ | pre-analysed tokens, per-token knowledge state and both coverage numbers |
| `GET` | `/api/texts/:id/next` | ✅ | the `n+1` rail |
| `POST` | `/api/knowledge/lemma` | ✅ | mark known / learning / ignored |

## The guided course

The grammar DAG fixes the order *rules* arrive in and says nothing about words; the miner finds
good sentences but only once there is a vocabulary to score them against. Between them sits the
question every beginner actually asks, and "mine your own sentences" is not an answer to it. So
the pack ships an ordered six-month syllabus — `course_unit` + `course_word` — and these three
endpoints serve it, then hand over.

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/course` | ✅ | the syllabus resolved against this learner's deck: `{plan, enrolled}`. 503 when there is no pack, or when the pack carries no course |
| `POST` | `/api/course/:code/enrol` | ✅ | takes a unit up: one sentence note per word, into the review deck. `{unitCode, notesMade, duplicate, skipped[]}`. 404 unknown unit; 400 when every review direction is disabled |
| `GET` | `/api/resources` | ✅ | `{resources: [{kind, title, url, level, blurbRu, blurbEn, free}]}` — the hand-off catalogue, read from the pack. 503 without a pack; a pack predating the table yields `{"resources": []}` rather than an error |

```json
{ "plan": {
    "units": [{
      "unit": {"code": "m2-u3", "month": 2, "ord": 7,
               "titleRu": "…", "titleEn": "At the doctor",
               "goalRu": "…", "goalEn": "I can register at a GP and describe a symptom",
               "situationRu": "…", "situationEn": "orvosi rendelő",
               "topicCodes": ["locative-grid", "past-tense"],
               "words": [{"lemma": "orvos", "upos": "NOUN", "glossRu": "…",
                          "glossEn": "doctor", "note": ""}]},
      "status": "current", "enrolled": true, "wordsTotal": 34, "wordsInPlay": 12
    }],
    "current": "m2-u3", "handedOver": false, "completed": 6, "total": 24 },
  "enrolled": {"m2-u3": {"unitCode": "m2-u3", "enrolled": true,
                         "notesMade": 31, "skipped": 3}} }
```

**A unit is `done` when its words are in the review deck — a fact, not a button.**
`status` ∈ `done | current | ahead`, and `done` means every word of the unit is in that
learner's `lemma_state` as `learning` or `known` (`course.Resolve`, joined to the deck in
`Server.inPlaySet`). There is no completion flag anywhere in the schema — `course_progress`
records enrolment and nothing else — so the plan screen cannot drift from the deck, and marking
a unit finished without learning it is not a thing the API can express.

**`ahead` is not `locked`.** Nothing stops a learner jumping to month five, and calling it
locked would be a lie about a restriction the app does not enforce. The order is advice.

`enrolled` is separate from `done` for the same reason: taking a unit up creates cards, and
whether the learner then *learned* them is the deck's answer, not the enrolment's.

**`skipped` is surfaced, never hidden.** A card must be a sentence — a bare lemma teaches
nothing about the morphology that carries the meaning — so a unit word the pack has no attested
sentence for gets no card at all, and the enrolment response names it. A unit that quietly
taught 26 of its 30 words would be a unit whose progress bar lies. `duplicate` counts words that
were already in the deck; enrolling twice does not double anyone's review load.

`plan.handedOver` goes true only when every unit is done. That is the point of the course, not
the end of it: the app stops prescribing vocabulary and `GET /api/resources` takes over, because
a course that never ends produces a learner who can only read material made for them
([R11](../.claude/research/11-extensive-reading.md)).

> `UnitState.missing` exists on the wire and is reserved: nothing populates it in v1.0. The
> words the pack cannot supply a sentence for are reported by the enrolment's `skipped`, which
> is where the learner actually sees them.

## Curriculum & drills

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/curriculum` | ✅ | topic DAG with per-topic phase, the reason for it, and prerequisites |
| `GET` | `/api/curriculum/:code` | ✅ | rule card (both locales) + per-feature mastery + what is blocking it |
| `GET` | `/api/drill/next` | ✅ | `?topic=&count=` — items honouring the blocked→interleaved gate |
| `POST` | `/api/drill/answer` | ✅ | `{itemId, answer, durationMs}` → grades the item, emits feature reviews, moves the gate |
| `GET` | `/api/placement` | ✅ | next probe on the adaptive ladder, or the seeds it decided |
| `POST` | `/api/placement` | ✅ | `{action: "apply" \| "reset"}` |

**The expected answer never leaves the server.** `/api/drill/next` returns items with an `id`
and no answer; the server holds the item in memory until it is answered
(`internal/drill/pending.go`). A client therefore cannot reveal the answer early, and — more
importantly — cannot claim which features an item graded, which is what keeps the mastery
heatmap trustworthy. An expired or replayed id gets **410 Gone**, not 404: the id was real.

`GET /api/curriculum` response:

```json
{
  "topics": [{
    "code": "definite-conjugation", "level": "A2", "teachOrder": 120,
    "titleRu": "…", "titleEn": "…",
    "prereqCodes": ["past-tense"], "unlocks": ["lak-lek", "infinitive", "potential"],
    "featureIds": ["Definite=Def", "Definite=Ind"], "motorSkill": false,
    "phase": "blocked",
    "decision": {
      "phase": "blocked", "from": "locked",
      "reason": "gate.blocked.accuracy",
      "args": {"accuracy": "63", "gate": "80"},
      "current": 0.63, "target": 0.80,
      "observations": 24, "window": 20, "ready": true, "demoted": false
    },
    "progress": {"blockedAccuracy": 0.63, "blockedTotal": 24, "interleavedAccuracy": 0.0,
                 "interleavedTotal": 0, "medianMs": 1400, "attempts": 24, "correct": 15,
                 "fallbacks": 1},
    "drills": {"available": true, "kinds": ["conjugation"]}
  }],
  "current": "definite-conjugation",
  "placementTaken": true,
  "gate": {"threshold": 0.8, "window": 20, "masteryThreshold": 0.9,
           "masteryWindow": 50, "hysteresis": 0.1, "harmonyLatencyMs": 2500}
}
```

`phase` ∈ `locked | explained | blocked | interleaved | mastered`. **`explained`** is the honest
value for a topic that has a real rule card and no drill generator behind it (the alphabet,
politeness registers, preverb detachment): it teaches by explanation and deliberately does
**not** gate its dependents, because a topic we have not built exercises for must not wall off
the rest of the curriculum. See [R5](../.claude/research/05-interleaving.md) and
`internal/drill/schedule.go`.

`decision.reason` is an **i18n key**, and `args` are its substitutions — the backend never picks
a language. Both locales render every key; a key present in one and not the other is an
incomplete change ([ADR-004](../.claude/DECISIONS.md#adr-004)).

`drills.available` is **proven, not assumed**: it is the result of actually building an item,
cached per topic. A topic can pass every structural test and still yield nothing — the pack's
attested `olvashat` carries no features at all, so the potential mood looks drillable and is
not. `drills.reason` then says which: `drill.unavailable.noGenerator` (no generator exists for
these features), `noMaterial` (a generator exists, the pack has nothing attested), `noPack`.

`GET /api/drill/next` item shapes — six kinds, `allomorph | case | conjugation | cloze |
transform | mcq`. `cloze` and `transform` are typed; the rest ship `options`:

```json
{ "id": "8f2c…", "topicCode": "locative-grid", "kind": "allomorph", "phase": "blocked",
  "promptKey": "drill.prompt.allomorph",
  "promptArgs": {"lemma": "könyv", "feature": "Case=All", "harmony": "front_rounded"},
  "lemma": "könyv", "lemmaId": 747, "glossRu": "", "glossEn": "book",
  "options": [{"text": "könyvhöz", "attested": true},
              {"text": "könyvhez", "attested": false},
              {"text": "könyvhoz", "attested": false}],
  "featureIds": ["Case=All", "Skill=VowelHarmony"], "headline": "Case=All",
  "attested": true }
```

`featureIds` are the features the item **actually required** — the only ones a wrong answer is
charged against. A pure harmony item grades `Skill=VowelHarmony` alone, because the learner
choosing `könyvhöz` over `könyvhez` was never asked what the allative means. A case choice
between forms of the same noun does **not** grade harmony, because every option shares the
stem's harmony class. `option.attested = false` means that exact string is not a Hungarian word,
which is shown after the answer.

`POST /api/drill/answer` returns the outcome, the graded features, the gate's new verdict, the
post-answer state of the features it touched, and — on a wrong answer — what the mistake says to
practise next, including the cases a drill cannot treat:

```json
{ "correct": false, "expected": "könyvhöz", "rating": 1,
  "gradedFeatures": [{"FeatureID": "Skill=VowelHarmony", "Rating": 1, "Correct": false}],
  "featureState": [{"featureId": "Skill=VowelHarmony", "retrievability": 0.42,
                    "accuracy": 0.71, "observations": 34, "medianMs": 1800,
                    "automated": false}],
  "progress": {…}, "decision": {…}, "topic": {…},
  "followUp": [{"featureId": "Skill=VowelHarmony", "topicCode": "vowel-harmony",
                "kinds": ["allomorph", "transform"], "remedy": "drill.remedy.volume",
                "misses": 1}] }
```

`remedy` is deliberately not always a drill. A vowel-length or rounding error returns
`drill.remedy.ear`: those contrasts are not distinguishable to a Russian or English ear without
training, and no amount of typing practice fixes that — the app says so rather than pretending
otherwise. `drill.remedy.vocabulary` routes back to the review deck.

`GET /api/placement` walks an adaptive ladder up the DAG, three items per topic, stopping after
two consecutive failed topics. It returns the next probe, or — when done — the `seeds` it
proposes, which are shown to the learner **before** `POST {"action":"apply"}` writes them.
No topic is ever seeded `mastered`, and a seed carries the number of observations actually made,
so the gate knows how thin the estimate is. `{"action":"reset"}` discards the test; it does not
delete the feature reviews the probes produced, because those were real retrievals.

## HVPT & listening

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/hvpt/session` | ✅ | a trial set. Below `hvpt.minTalkers` distinct voices it returns `refused: {have, need}` with **no trials** — a designed screen, not an error status |
| `POST` | `/api/hvpt/answer` | ✅ | `{trialId, chosen, rtMs}` → reveals `played`, updates the contrast's FSRS + accuracy. Replay of an id is 410 |
| `GET` | `/api/hvpt/contrasts` | ✅ | per-contrast state; **no contrast is ever complete** [R7] |
| `POST` | `/api/listen/material` | ✅ | multipart `audio` + `title` + `transcript`; the transcript is ingested as a normal text (coverage pre-check for free) and the response carries the verdict |
| `GET` | `/api/listen/:id/state` | ✅ | the three-pass position [R13] |
| `POST` | `/api/listen/:id/progress` | ✅ | `{pass, positionS, secondsHeard}` — seconds are clamped to 60 per report: **hours accumulate, they cannot be claimed**. Feeds the word-order listening gate [R15] and the daily ledger |

HVPT trials never carry the played member — it is held server-side like a drill's expected
answer (ADR-008 applies). The audio URL, the talker id and the rate are in the payload, because
talker variability is the mechanism and must be auditable on screen.

## Progress

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/stats/overview` | ✅ | lemma counts + cumulative curve, streak, today's ledger, listening hours, checkpoint history + the next spec with live evidence, blocked levels, exam date |
| `GET` | `/api/stats/mastery` | ✅ | per-feature mastery |
| `POST` | `/api/checkpoints` | ✅ | `{code, attested[], notes}` → verdict. Measured criteria the server computes; attested criteria are the learner's claims, stored as such. A failure returns the diagnostic against the three documented failure modes and **locks curriculum levels** at/above the checkpoint's gate (`gate.locked.checkpoint`) until a retake passes |
| `POST` | `/api/today/human` | ✅ | Loop 4 attestation. Recorded, shown, **ignored by the streak** — the app cannot verify a conversation and does not pretend to |

The streak in `/api/stats/overview` counts **obligations met** — due queue cleared AND the
stated daily minutes reached — never activity. A day with no work breaks it; that is design,
not accident ([R14], and missed days are failure mode №1 in the source plan).

> **There is no export or backup API.** `/api/export/apkg`, `/api/export/csv`,
> `/api/export/backup` and `/api/import/backup` were removed along with `internal/export`; no
> route, package or UI for them exists. The learner's data is one self-contained directory, so
> backup is a filesystem operation and restore is putting the directory back — see
> [SELFHOSTING.md](SELFHOSTING.md#backup-and-restore). Do not re-document these endpoints
> without re-implementing them.

## Speech & LLM *(v1.1)*

| Method | Path | Status | Notes |
|---|---|---|---|
| `POST` | `/api/speech/tts` | ⏳ P8 | text → audio; content-addressed cache |
| `POST` | `/api/speech/asr` | ⏳ P8 | audio → transcript + confidence |
| `POST` | `/api/speech/score` | ⏳ P8 | target + audio → **explicit segmental feedback** + enqueued HVPT contrasts |
| `POST` | `/api/llm/generate` | ⏳ P9 | comprehensible input; returns `attested` per span |
| `POST` | `/api/llm/correct` | ⏳ P9 | staged correction — **server releases stage 3 only after an attempt** |
| `POST` | `/api/llm/roleplay` | ⏳ P9 | scenario turn, optionally voice in/out |

## Static

| Path | Serves |
|---|---|
| `/media/*` | cached TTS, HVPT clips, learner recordings |
| `/*` | SPA catch-all from the embedded FS |
