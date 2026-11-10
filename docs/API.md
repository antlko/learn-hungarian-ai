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
| `GET` | `/api/bootstrap` | ✅ | `{needsSetup, packPresent, packVersion, packEdition, locales, version, minPassword}` — unauthenticated; drives the wizard. `minPassword` is the floor the server enforces (ADR-038) |
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
| `GET` | `/api/capabilities` | ✅ | `[{name, provider, configured, reachable, latencyMs, detail, reason?, models?}]`, cached 30 s. `reason` is set only when a configured service could not be reached: `tlsToPlainHttp` (an `https://` address on a plain-HTTP server, the usual Ollama mistake), `refused`, `timeout`, `dns`, `tlsCert` or `unreachable`; `detail` stays `unreachable`, and no raw error, URL or address is sent. `detail` is one of `not configured`, `bad endpoint`, `unreachable`, `unhealthy`, `device`, and for `llm` also `no model` / `unknown model`; `models` is what a reachable `llm` endpoint listed on `GET /v1/models` (absent when it has no such route) |
| `POST` | `/api/capabilities/:name/test` | ⏳ | round-trip test against a provider |

**`tts` with `provider: "device"` reports `configured: true, reachable: false, detail: "device"`,
and that is not a fault.** Device speech is the Web Speech API in the learner's browser, which is
the default source of Hungarian audio and the one optional capability that needs nothing
deployed. This process cannot reach that browser and cannot know whether it has a Hungarian
voice, so it declines to claim either: the client asks `speechSynthesis`, names the voice it
found, and disables the 🔊 controls with a reason when there is none
(`stores/speech.ts`, `lib/speech.ts`). Asserting a reachability we have not tested is the one
thing this endpoint must not do.

## Admin: full pack, restart, components

All four routes are **admin-only** (learner 403, anonymous 401) — [ADR-049](../.claude/DECISIONS.md#adr-049).
None accepts a URL: the pinned default lives in the binary; `langpack.url` / `langpack.sha256` are an
optional override in `config.yaml` and `GET/PUT /api/settings` (`settings.langpack: {url, sha256}`;
`http` accepted only for localhost). Empty fields mean "use the binary's pin" (ADR-049).

| Method | Path | Notes |
|---|---|---|
| `POST` | `/api/admin/pack/install` | starts the single install job → **202** with the status below. **409** `errors.packInstallBusy` while one runs; **422** `errors.packNotPublished` when the effective checksum (stored, else the binary's) is empty — no request is made |
| `GET` | `/api/admin/pack/install` | `{state, bytes, total, errorKey, restartRequired, published, sizeBytes, url}` |
| `POST` | `/api/admin/restart` | → **202** `{supervised: bool}`; ~400 ms later the process gets SIGTERM (graceful shutdown). `supervised` is true iff `TANULOM_SUPERVISED=1`. **409** while an install runs |
| `GET` | `/api/admin/components` | `[{id, configured, reachable, state, progress, model, ramHint, composeProfile, built, managed?}]` — `managed` on `mt` and `nlp` only, below |
| `POST` | `/api/admin/components/:id/install` | `:id` is `mt` or `nlp` (anything else **404** `errors.componentUnknown`). **202** with the `managed` object; the job runs in the background and the card polls `GET /api/admin/components`. **409** `errors.componentBusy` (a job runs) / `errors.componentInstalled`; **422** `errors.componentNoPython` (or `errors.componentNoPythonSlimImage` on the `:latest-slim` image) / `errors.componentReadOnly`; **507** `errors.diskFull` (preflight, nothing written) |
| `POST` | `/api/admin/components/:id/start` | **202**; **409** `errors.componentNotInstalled` / `errors.componentBusy`. Idempotent while running |
| `POST` | `/api/admin/components/:id/stop` | **202**; SIGTERM then SIGKILL after 5 s. Idempotent. **409** `errors.componentBusy` during an install |
| `POST` | `/api/admin/components/:id/uninstall` | **202**; stops the child, deletes `<data>/components/<id>/`, and sets `services.<id>.provider` to `off` iff it was `managed`. **409** `errors.componentBusy` during an install |

All four are admin-only (403 for a learner, 401 signed out) and take no body: nothing a client sends
can reach a command line ([ADR-061](../.claude/DECISIONS.md#adr-061)).

```jsonc
// `managed` on the mt / nlp rows of GET /api/admin/components (and the 202 body above)
{ "available": true,            // python3>=3.10 with venv+pip found AND the data dir is writable
  "unavailableKey": "errors.componentNoPython",  // | errors.componentReadOnly — only when available=false
  "pythonVersion": "3.12.1",
  "state": "running",           // absent | installing | installed | starting | running | failed | stopped
  "step": "component.step.pip", // installing only: component.step.venv|pip|model|start
  "progress": 0.42,             // 0..1 over the whole install, never decreases within a job
  "port": 41873,                // 127.0.0.1 only, in memory; absent unless starting/running
  "errorKey": "errors.componentNetwork",   // | errors.componentInstallFailed | errors.diskFull | errors.componentStartFailed | errors.componentCrashed
  "errorDetail": "short redacted tail of the failing command",
  "log": ["…last ≤40 lines, paths outside the data dir reduced to their last element…"],
  "diskNeed": "≈6 GB (estimate)", "ramNeed": "≈2–3 GB (estimate, not measured)" }
```

On success of an install the server sets `services.<id>.provider = "managed"` itself; the settings
DTO then carries `provider: "managed", endpoint: ""`. A `managed` service whose child is not running
probes as `configured: true, reachable: false` (`detail: "not running"`).

```jsonc
// GET /api/admin/pack/install
{ "state": "downloading",   // idle | downloading | verifying | ready | failed
  "bytes": 52428800, "total": 262144000,   // compressed bytes; total 0 when the server sent no length
  "errorKey": "",            // on failed: errors.packNotPublished | packChecksum | packDownloadFailed |
                             //   packCorrupt | packIncompatible | packOlder | diskFull
  "restartRequired": false,  // true once state is "ready"
  "published": true,         // effective checksum non-empty; false = install would 422 packNotPublished
  "sizeBytes": 0,            // approximate download size for the button label; 0 = unknown
  "url": "https://…" }       // the effective source (stored override, else the binary's pin)
```

`ready` means the verified pack is parked as `langpack-hu-v1.sqlite.new`; the live pack is untouched
and **the next boot applies it** (repeating the id check; a refusal shows here as `failed` +
`errors.packIncompatible`). On any
failure the live pack is untouched and the temporary files are gone. `components[].id` is one of
`pack | mt | nlp | tts | asr | llm`; `state` for `pack` is `starter | full | installing |
restartRequired | missing`, for the rest `off | device | unreachable | ready` (mt passes its own
`loading | ready | failed`); `progress` is 0..1 (0 when nothing is in flight); `built` is false for
a sidecar this repository does not ship yet (`tts`, `asr`; `nlp` is built — its state, `loading` while the model downloads, with `progress`, comes from the sidecar's `/health`); `ramHint` is a documented
figure or blank, never measured here. Reachability comes from the same probe as
`/api/capabilities`.

## Language & morphology

| Method | Path | Status | Notes |
|---|---|---|---|
| `POST` | `/api/analyze` | ✅ | `{text}` → tokens with `lemma`, `upos`, `feats`, `segments`, `certainty` |
| `GET` | `/api/lemma/:id` | ✅ | lemma, glosses (ru/en + `glossFallback` flag), IPA, frequency, CEFR, paradigm + `paradigmDerived` marking the cells our generator produced rather than the dictionary |
| `GET` | `/api/lemma/:id/examples` | ✅ | `?limit=1..10` (default 3) → `{examples: [{hu, ru?, en?, form}]}` — attested sentences using the lemma, with their translations. Same two-pass search as course enrolment, and the analyser re-verifies each candidate, so a sentence the tagger filed under another lemma is still found and one that does not really contain the word is still rejected. `form` is the surface shape the lemma takes in that sentence, which is the part worth pointing at |
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

`analysis.via` (optional, `"context"`): the reading of an `uncertain` word was chosen by the opt-in
HuSpaCy sidecar from the pack's own candidates ([ADR-054](../.claude/DECISIONS.md#adr-054)); the
UI says «по контексту (HuSpaCy)» (`reader.viaContext`). Absent when no sidecar is configured or
reachable. `certainty` stays `uncertain`.

`POST /api/admin/nlp/warmup` (admin) asks the sidecar to download/load its model and returns the
`nlp` component row (`state: loading|ready|failed`, `progress`); 503 with
`errors.nlpNotConfigured` / `errors.nlpUnreachable` otherwise. Poll `GET /api/admin/components`.

## Review (SRS)

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/review/queue` | ✅ | today's queue; `?scope=all\|course\|reader\|text:<id>` (default `all`; an unknown value is a 400). **`scope=trash` is a 400 here** — the trash is practised, never scheduled ([ADR-025](../.claude/DECISIONS.md#adr-025)). `?order=due\|random` (default `due`) arranges the same cards: `random` is stable for the day and never changes which cards are served ([ADR-026](../.claude/DECISIONS.md#adr-026)); the response echoes `order` |
| `GET` | `/api/words` | ✅ | the words browser: `?status=&stage=&q=&sort=&page=&pageSize=` → one row per deck word with status, strongest-card retrievability, next due and lapses. Each row also carries `stage` (`learning`\|`relearning`\|`young`\|`mature`\|`known` — `known` is the learner's «Знаю», otherwise `srs.StageOf` the strongest card; young/mature split at a 21-day interval), `intervalDays`, `reps`, `stepsLeft`, `stepsTotal` (the strongest card's `srs.Memory` figures). `?stage=` filters on it (400 on an unknown value); pagination stays in Go. Search resolves through pack case-variants, never `COLLATE NOCASE` |
| `POST` | `/api/review/answer` | ✅ | `{cardId, rating, typed?, via?, durationMs}` → next card + graded features, plus `memory`: the card's figures AFTER the answer, so the screen can show «3 дн → 8 дн» ([ADR-026](../.claude/DECISIONS.md#adr-026)). `expected` is the **dictionary form** on `produce` and the **surface form** on `cloze`/`spell` — see below. With `review.accentTolerance` (default on), a typed answer wrong only in diacritics is accepted at **Hard** and flagged `acceptedWithAccents: true` — never silently; `spell` is exempt |
| `POST` | `/api/review/undo` | ✅ | reverts the last answer including its feature reviews. Delete-and-replay: the log is the truth, state is recomputed from it (bit-exact — the property test in `internal/srs/replay_test.go` covers fuzzed due dates too). 409 when there is nothing to undo |
| `GET` | `/api/review/forecast` | ✅ | 90-day projection for the governor |
| `POST` | `/api/notes` | ✅ | creates a note; the server locates the target span, reads its features off the live analysis, backfills a missing gloss from the pack, and spawns one card per planned direction. Returns `lemmaStatus`, the lemma's state after the call, and `restored: true` when the duplicate it matched was in the trash and has been put back |
| `POST` | `/api/notes/:id/trash` | ✅ | sets a word aside: every direction of it leaves the queue and every count. Not a delete — nothing is removed and `review_log` is untouched. 404 on a note that is not the caller's; a second call is `changed: false`, not an error |
| `POST` | `/api/notes/:id/know` | ✅ | the same handler under the name the client uses. «Знаю» additionally marks the note's lemma `known`; `/restore` reverts it to `learning` ([ADR-041](../.claude/DECISIONS.md#adr-041)). `/trash` is kept so a stale tab keeps working |
| `POST` | `/api/notes/:id/restore` | ✅ | puts it back with the FSRS state it left with — a word set aside for a fortnight returns overdue |
| `GET` | `/api/trash` | ✅ | the set-aside words, most recent first: dictionary form, gloss, the sentence, origin, `trashedAt` and how many cards a restore brings back |

**A queue item carries `target: {before, after}`, not offsets.** `note.target_start` and
`target_end` are BYTE offsets into UTF-8 and never cross the wire. They used to, and the client
sliced `hu` with them as if they were JavaScript string indices — which they are not, because JS
counts UTF-16 code units. One accented letter before the target and the blank landed inside the
next word: `Jó éjszakát, Timmy!` rendered as `Jó é_____immy!`. The split is now done where the
bytes are, in `srs.splitTarget`, and the field is absent when the note has no located span. It
is sent for every direction, not only cloze: `recognize` underlines the target so the word gloss
has a visible referent.

**The queue is scoped by where its material came from.** `?scope=course` is notes enrolled from
a course unit (`note.source` matching `course:%`); `?scope=reader` is everything else, defined as
the complement so a source added later cannot vanish from every tab; `?scope=text:<id>` is the
words mined from one library text — a SUB-slice of `reader` via `note_origin`, never a fourth
pile ([ADR-022](../.claude/DECISIONS.md#adr-022)). The response echoes the `scope` it actually
served and carries `scopes`: one **`Breakdown`** per pile
(`{scope, dueNow, dueLaterToday, nextDueAt?, buriedToday, newCards, newNotes}` —
[ADR-021](../.claude/DECISIONS.md#adr-021)), always the fixed three plus the text scope when one
was served, because the segmented control has to label the tab the learner is not standing on.
Every figure is **post-burial and day-granular**: `dueNow` is what a queue built now will
actually serve (one card per word; graduated cards count for the whole local day, learning
steps once elapsed), `dueLaterToday`/`nextDueAt` name the learning steps still maturing today,
and `buriedToday` is where due-but-hidden cards are counted instead of inside a bigger number.
**The daily new-card budget is global, not per scope** — `introducedToday` and `newPerDay`
count the whole deck however the queue is filtered, or a learner would take the day's ration
twice over. [ADR-016](../.claude/DECISIONS.md#adr-016).

**`expected` on an answer depends on the direction.** `produce` prompts with the target word's
gloss, so it grades the **dictionary form** — prompting «день; солнце» and demanding `napokra`
asked for a case the card never named. `cloze` and `spell` grade the surface form: cloze blanks
that exact span and is the only direction that grades morphology.
[ADR-017](../.claude/DECISIONS.md#adr-017).

**A queue item carries `memory`** — what the card knows about itself, for the learner to see
while answering ([ADR-026](../.claude/DECISIONS.md#adr-026), [R18]): `stage` (`new` · `learning` ·
`young` · `mature` · `relearning`, Anki's 21-day convention for young/mature), `retrievability`
(the library's own forgetting curve, the same figure the words browser shows), `stabilityDays`,
`intervalDays`, `reps`, `lapses`, `correct`/`total` for the card and `wordCorrect`/`wordTotal`
for every card of the word, `history` (the last eight outcomes, oldest first), and `preview` —
what each grade would schedule (`again`/`hard`/`good`/`easy` as instants, from
`Scheduler.Preview`, approximate because interval fuzz is seeded at answer time). Practice items
carry the same object **without `preview`**: nothing there is scheduled ([ADR-019](../.claude/DECISIONS.md#adr-019)).
Every figure is measured or computed by the library; none is a score.

**Sets are the client's.** The queue is dealt out in sets of a per-device size (default 15) by
the review screen; the server serves the same list as ever, and every number in `counts` and
`scopes` is untouched. A card missed in a set is re-served at the set's end once the `nextDue`
the answer returned has elapsed — the client waits on the scheduler's own step, never asks
early.

**A queue item also carries `sentenceRu` / `sentenceEn`.** These translate the WHOLE SENTENCE and
are distinct from `glossRu` / `glossEn`, which gloss the target WORD. Confusing the two is what
made the card unreadable — `Jó hírt hoztunk.` under «хороший; добрый», the entry for `jó`, with
nothing saying which of the four words it belonged to. Filled from `sentence_trans` via the
note's `sentence_id`, in one query for the whole queue, and absent when the pack has no
translation for that sentence.

**Free practice is a separate mode, not a second queue.** `/api/practice/*` exists so a learner
can train at any moment — with the queue cleared, or with a deck of nothing but new cards. A
practice answer changes **no card state**, writes **nothing to `morph_state`**, and writes
**nothing to `review_log`**: undo is delete-and-replay over that log and replay must reproduce
stored FSRS state bit for bit, so a row that never fed the scheduler cannot live in it. It
records `practice_log` and `daily_log.active_ms` only — never `due_left`, so however long a
session runs it cannot earn the day's streak. `PracticeAnswerResponse` deliberately carries no
`nextDue` and no `gradedFeatures` rather than zeroed ones: a `0` interval would render as a
schedule that does not exist. The verdict itself is shared with the review path
(`gradeTyped`/`expectedAnswer`), so the two modes can never disagree about what is correct.
Volume and accuracy come back on `GET /api/stats/overview` as `practice`, reported separately
from `today.reviews`. [ADR-019](../.claude/DECISIONS.md#adr-019).

**And `sentenceModel`, which names the machine — or is empty.** A note mined from the reader has
no `sentence_id`: its sentence came from the learner's own text and the pack has no human
translation for it. Its crib is read from `translation_cache` and carries the model that wrote
it, printed beside the text on the card. `sentenceModel` is **empty for a pack translation**,
which a person wrote. The queue **never calls the MT sidecar** — it reads the cache and nothing
else, so the screen is identical whether the translator is running, off, or still loading.
[ADR-018](../.claude/DECISIONS.md#adr-018).

**A queue item carries `expected`, the string that card is graded against.** It is on the wire
so «Подсказка» can reveal the answer *without* grading the card — revealing is the whole point of
a hint, and on a typed card there was nothing to reveal. No secret is spent: `targetForm` and
`lemma` are both already sent, so the answer was always derivable client-side. What was missing
was WHICH of the two a direction wants, and the client must never re-derive that — `produce`
grades the dictionary form and `cloze` the form in the sentence, and a second copy of that rule
is how the hint comes to show one word while the grader wants another. Server and client share
one `expectedFor`. [ADR-017](../.claude/DECISIONS.md#adr-017).

**`options` (optional) — tap-to-answer candidates for `cloze` and `produce` cards.**
`"options": ["házban","házat","házba","ház"]`: 3-5 strings, shuffled, containing `expected`
exactly once; omitted when fewer than three can be built (the client types). The rest are
real: for `cloze`, other **attested** forms of the same lemma (the two closest in suffix, then a
seeded draw; topped up with same-POS neighbours if the lemma has too few forms); for `produce`,
same-POS lemmas near in frequency rank and length, never one with the same first-sense gloss.
No option is accepted by the grader as the answer (case/diacritics-insensitive equal).
Deterministic per (card, local day). Built for the review queue and `/api/practice/queue`,
including reader-mined and custom notes (it needs only the note's lemma and target form).
**Grading does not change**: the client submits the chosen string as `typed`, optionally with
`via: "choice"` (`"typed"` default, anything else → 400; `choice` only for `cloze`/`produce`).
`via` is not stored; on `choice` a `usedHint` flag is ignored, and a correct choice rates
exactly what a correct typed answer without a hint does (Good — never Easy, never a recognition
bonus). Accepted by `/api/review/answer` and `/api/practice/answer`.
[ADR-060](../.claude/DECISIONS.md#adr-060).

**`QueueCounts` describes the session that was built, in the Breakdown vocabulary.** `due` is
the served scope's `dueNow`; `dueLaterToday`/`nextDueAt` are the learning steps the done screen
counts down to; the fate of every new word this build considered comes on ONE basis —
`newShownNotes` / `newDeferredNotes` (a sibling was already due or answered today; NOT the
limit's doing) / `newBacklogNotes` (held back by the budget alone) — with the global pace as
`introducedToday`/`newPerDay`/`newBudgetLeft`, and `reviewsCapped` when `maxReviewsPerDay`
trimmed the build. A note spawns three to five cards, so the word figures lead: "59 new cards
waiting" told a learner with 27 new words nothing they could act on.
| `GET` | `/api/practice/queue` | ✅ | free practice from `?scope=` (text scope **and `trash`** included), ignoring due dates and the daily cap. `?size=` (10–50, clamped, default 20) and `?mode=random\|weak` — weak leads with the lowest-retrievability words and falls back to random on a virgin deck; the response's `mode` echoes what was actually served ([ADR-024](../.claude/DECISIONS.md#adr-024)). Also returns `pool` |
| `POST` | `/api/practice/answer` | ✅ | `{cardId, rating?, typed?, via?, durationMs}` → `{correct, expected?, diff?}`. **Schedules nothing** — see below |
| `GET/PUT/DELETE` | `/api/notes/:id` | ⏳ | editing and deletion |

**The trash is practised, never scheduled.** `POST /api/notes/:id/trash` takes a word out of the
scheduler and nothing else: the note, its cards, their FSRS state and every `review_log` row stay
exactly as they were, which is what makes restore a restore rather than a re-add at state zero —
and what keeps undo (delete-and-replay over that log) working across a trashing. Free practice
accepts `scope=trash` because practice records nothing (ADR-019); the review queue refuses it,
and answering a card trashed since the queue was built is a **409**. Both trash and restore
return fresh per-scope `Breakdown`s so the tab strip updates from the server rather than by doing
its own arithmetic (ADR-021); pass the caller's own `?scope=text:<id>` to get that chip's numbers
back alongside the fixed three. Re-adding a trashed word — from the reader, or by re-enrolling
its course unit — restores it, because otherwise «добавить» reported success on a word that
stayed invisible. See [ADR-025](../.claude/DECISIONS.md#adr-025).

## Texts & mining

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET/POST/DELETE` | `/api/texts[/:id]` | ✅ | ingest paste/URL/`.srt`/`.epub`. The list also carries each text's `level`/`levelSource` and `noteCount` (words in the deck via `note_origin`) |
| `GET` | `/api/texts/:id` | ✅ | pre-analysed tokens, per-token knowledge state and both coverage numbers |
| `GET` | `/api/texts/:id/next` | ✅ | the `n+1` rail |
| `PUT` | `/api/texts/:id/level` | ✅ | the learner's own CEFR verdict (`{level}`, strict enum) — stored as `manual`, never overwritten by a machine ([ADR-023](../.claude/DECISIONS.md#adr-023)) |
| `POST` | `/api/texts/:id/level` | ✅ | ask the configured `services.llm` for a second opinion. 503 when the slot is off; a garbage answer is a 502 and the stored estimate stays; the model's name travels in `levelDetail`. The first consumer of the llm slot — `/api/llm/*` below remains unbuilt |
| `POST` | `/api/knowledge/lemma` | ✅ | mark known / learning / ignored |
| `GET/PUT/DELETE` | `/api/glosses/:form` | ✅ | the learner's own dictionary, for words the pack lacks |
| `POST` | `/api/translate` | ✅ | machine translation of sentences — **503 unless `services.mt` is up**  Callers: the reader's word rail, the review card's crib and the word breakdown's example sentences — each offers it only when the learner's language is missing and `mt` is reachable ([ADR-035](../.claude/DECISIONS.md#adr-035)); the review queue itself never calls it |
| `GET` | `/api/mt/status` | ✅ | what the translation sidecar is doing, in its own words |
| `POST` | `/api/mt/warmup` | ✅ | admin — fetch the model, or retry a failed download |

### `GET /api/library/sets` · `POST /api/library/sets/:code/import`

Ready-made reading sets, «Живые примеры» ([ADR-056](../.claude/DECISIONS.md#adr-056)). Signed-in.

`GET` → a bare array (never `null`), core units first by order, then themes:
`[{code, level, track, title, titleRu, titleEn, theme, sentences, imported, textId?}]`.
`title`/`theme` follow `?lang=ru|en`, else the configured UI locale; `textId` is present only when
`imported`. A pack without the tables answers `200 []` — not an error.

`POST …/import` → `{textId, created}`. Builds the text from the set's pack sentences, one per
paragraph (a blank line always ends a sentence), and stores it through the same ingestion path as
`POST /api/texts` (`kind: paste`, title = the set title in the UI locale, `source_url = "set:<code>"`
— a marker, not a link). `201 created:true` the first time; `200 created:false` with the existing
`textId` thereafter (per learner; deleting the text allows a fresh import). `404` unknown code,
`503 errors.packMissing` when the pack has no sets table. It creates **no** note, card or state:
mining words stays the learner's act in the reader. The pack's Russian translation is not stored
with the text — the text model has no per-sentence translation slot.

### `GET/PUT/DELETE /api/glosses/:form`

```jsonc
// PUT /api/glosses/Kinyitom
{ "gloss": "Я открываю.", "source": "machine", "model": "facebook/nllb-200-distilled-600M" }
// GET -> the same, or {"form": "kinyitom"} when nothing is saved
```

The pack is read-only and finite; `Kinyitom` and a long tail behind it resolve to no lemma. This
is the overlay that lets the reader say something about them anyway — the same shape as
`morph_correction`, which corrects an analysis rather than adding a meaning.

`form` is **lowercased on write and on read**, so a lookup is an exact primary-key hit;
`COLLATE NOCASE` would throw the index away on the hot path of every word the reader draws. A
miss is an empty entry, not a 404: almost every word is a miss.

**`source` is the point.** `manual` is a meaning the learner wrote; `machine` is one a
translation model produced, stored with the model so the label can name it. The distinction
survives storage and is re-rendered on every view, and anything not explicitly `machine` defaults
to `manual` — mislabelling a person's note as machine output is a cosmetic insult, the reverse
passes off a guess as a person's understanding. A `machine` entry does **not** reach a card until
the learner accepts it, which re-saves it as `manual`. See
[ADR-014](../.claude/DECISIONS.md#adr-014).

### `POST /api/translate`

```jsonc
// ->
{ "texts": ["Reggel hét órakor felkelek."], "target": "ru" }   // target: ru | en, blank = locale.gloss
// <-
{ "translations": [
    { "source": "Reggel hét órakor felkelek.",
      "text":   "Утром я встаю в семь часов.",
      "model":  "facebook/nllb-200-distilled-600M",
      "cached": false }
]}
```

### `GET /api/mt/status` · `POST /api/mt/warmup`

```jsonc
// GET /api/mt/status
{ "configured": true, "reachable": true, "state": "loading",
  "model": "facebook/nllb-200-distilled-600M",
  "downloadedBytes": 1240000000, "totalBytes": 2482655255 }
```

**Why this is not `/api/capabilities`.** That probe treats any status under 500 as reachable,
which is right for four services and wrong for this one: a translator downloading 2.4 GB of
weights answers `/health` with 503 **by design**, for several minutes. Through the probe,
"downloading", "never deployed" and "died on a full disk" are one grey row. This route passes
the service's own body through so Settings can tell them apart and offer the remedy that fits.

`configured` and `reachable` are separate because they send the learner different places —
switch the provider on, versus bring the container up. `state` is absent when we could not ask.
`detail` on a failure is the service's **own** text, verbatim; a sentence we invented about it
would drop the one thing that lets anybody fix it. The byte counts are omitted rather than
zeroed when the service cannot measure them, and the client draws an indeterminate bar rather
than "0%".

`POST /api/mt/warmup` is **admin-only** (it costs bandwidth and disk on a shared install) and
returns the same shape. It is what makes a failed download retryable without restarting the
container — which on a deployed instance means without SSH. It is idempotent: a load already
running is reported, not started twice. 503 `translation.notConfigured` when the provider is
off, 503 `translation.unreachable` when nothing answers. Both are i18n keys.

Neither route can START the sidecar: the core has no shell and no Docker socket. See
[ADR-015](../.claude/DECISIONS.md#adr-015) for why that boundary is stated rather than papered
over.

The only endpoint that returns text no person and no corpus stands behind, so it is the only one
that returns its author: **`model` travels with every row and the UI prints it beside the text**,
on the first view and on every cached one. Nothing here becomes a note, a gloss or a graded
answer — it is a crib for a line the learner is looking at ([ADR-014](../.claude/DECISIONS.md#adr-014)).

- **`503 translation.notConfigured`** — no translator is set up. **`503 translation.unreachable`** —
  it is not answering, which on a fresh deploy usually means the model is still loading. Both are
  i18n keys: only the client knows the learner's language.
- **A fully cached batch is answered with the sidecar stopped.** Those sentences were already
  paid for and serving them needs no model.
- Batched, capped at 40 sentences / 8000 characters, and **refused with the numbers** rather than
  truncated. Duplicates within one batch cost one model run.
- A short answer from the sidecar is an error, never a partial result: dropping element three
  would caption sentence four with sentence three's meaning.

## The guided course

The grammar DAG fixes the order *rules* arrive in and says nothing about words; the miner finds
good sentences but only once there is a vocabulary to score them against. Between them sits the
question every beginner actually asks, and "mine your own sentences" is not an answer to it. So
the pack ships an ordered syllabus in three finite tiers — `course_unit` + `course_word` — and these three
endpoints serve it, then hand over.

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/course` | ✅ | the syllabus resolved against this learner's deck: `{plan, enrolled, totals, unlock}`. `plan.current` is the unit to take NEXT and follows enrolment ([ADR-048](../.claude/DECISIONS.md#adr-048)): the first unit enrolled and not done, else the first neither enrolled nor done; reader words can finish a unit but never move it. 503 when there is no pack, or when the pack carries no course. Each `units[].unit` has a `track` (`"core"` or `"theme"`; `"core"` for a pack built before the column existed) and a `tier` (`"basic"` | `"intermediate"` | `"advanced"`; `"basic"` for an older pack and for every theme unit; [ADR-057](../.claude/DECISIONS.md#adr-057)). `unit.month` (1-18) is an internal ordering/checkpoint field, not a learner-facing label. Each `units[]` also has `learned`: how many of the unit's words are *counted lemmas* (the `/api/stats/level` rule). `wordsInPlay` says a word is in the deck; `learned` says it has been learned. `totals` tallies the core words, in all and per tier: `{words, inDeck, learned, settled, perTier: {basic|intermediate|advanced: {words, inDeck, learned, settled}}}`; themes are excluded. Each `units[]` also has `settled` ([ADR-058](../.claude/DECISIONS.md#adr-058)): how many of the unit's words are *settled* — lemma `known`, or a non-suspended, non-trashed card at stage `mature` (interval >= 21 days). `unlock: {share: 0.7, settledDays: 21}` is the soft gate's rule so the client never hard-codes it: a unit after one with `settled < share * wordsTotal` is DRAWN closed. It is advice; no endpoint refuses enrolling or practising such a unit. Theme units are optional side branches ([ADR-050](../.claude/DECISIONS.md#adr-050)): they carry their own `status` (`done`/`ahead`) and `enrolled`, but never become `current`, and `total`/`completed`/`handedOver` count the 64 core units only (all three tiers; `handedOver` only after the last tier's last unit is done) |
| `POST` | `/api/course/:code/enrol` | ✅ | takes a unit up: one sentence note per word, into the review deck. `{unitCode, notesMade, duplicate, skipped[]}`. 404 unknown unit; 400 when every review direction is disabled |
| `GET` | `/api/resources` | ✅ | `{resources: [{kind, title, url, level, blurbRu, blurbEn, free}]}` — the hand-off catalogue, read from the pack. 503 without a pack; a pack predating the table yields `{"resources": []}` rather than an error |

```json
{ "plan": {
    "units": [{
      "unit": {"code": "m2-u3", "month": 2, "ord": 7,
               "tier": "basic", "titleRu": "…", "titleEn": "At the doctor",
               "goalRu": "…", "goalEn": "I can register at a GP and describe a symptom",
               "situationRu": "…", "situationEn": "orvosi rendelő",
               "topicCodes": ["locative-grid", "past-tense"],
               "words": [{"lemma": "orvos", "upos": "NOUN", "glossRu": "…",
                          "glossEn": "doctor", "note": ""}]},
      "status": "current", "enrolled": true, "wordsTotal": 34, "wordsInPlay": 12, "learned": 3, "settled": 1
    }],
    "current": "m2-u3", "handedOver": false, "completed": 6, "total": 64 },
  "unlock": {"share": 0.7, "settledDays": 21},
  "totals": {"words": 3106, "inDeck": 140, "learned": 31, "settled": 9,
             "perTier": {"basic":        {"words": 1399, "inDeck": 140, "learned": 31},
                         "intermediate": {"words": 1012, "inDeck": 0,   "learned": 0},
                         "advanced":     {"words": 695,  "inDeck": 0,   "learned": 0}}},
  "enrolled": {"m2-u3": {"unitCode": "m2-u3", "enrolled": true,
                         "notesMade": 31, "skipped": 3}} }
```

**A unit is `done` when its words are in the review deck — a fact, not a button.**
`status` ∈ `done | current | ahead`, and `done` means every word of the unit is in that
learner's `lemma_state` as `learning` or `known` (`course.Resolve`, joined to the deck in
`Server.inPlaySet`). **That is "in the deck", not "learned"**: a unit is `done` the moment it is
enrolled, so the UI shows `learned`/`wordsTotal` instead and does not call `done` «пройден». There is no completion flag anywhere in the schema — `course_progress`
records enrolment and nothing else — so the plan screen cannot drift from the deck, and marking
a unit finished without learning it is not a thing the API can express.

**`ahead` is not `locked`.** Nothing stops a learner jumping to month five, and calling it
locked would be a lie about a restriction the app does not enforce. The order is advice.

`enrolled` is separate from `done` for the same reason: taking a unit up creates cards, and
whether the learner then *learned* them is the deck's answer, not the enrolment's.

**`skipped` is surfaced, never hidden, and carries the gloss.** A card must be a sentence — a
bare lemma teaches nothing about the morphology that carries the meaning — so a unit word the
pack has no attested sentence for gets no card at all, and the enrolment response names it as
`{lemma, glossRu, glossEn}`. It used to be a bare string, which named a word to the learner and
then declined to say what it meant; refusing to invent an example is a principle, withholding a
gloss that is sitting beside the word in the pack is not. A unit that quietly taught 26 of its
30 words would be a unit whose progress bar lies.

**Finding that sentence takes two searches, not one.** The corpus files each token under a
lemma id chosen by the tagger that built the pack; the course names its words with the
*dictionary's* lemma. They disagree often enough to matter — `nagyon` is lemmatised to the
adjective `nagy`, so a join on lemma id returned nothing for 820 sentences that plainly contain
the word, and the UI reported a corpus gap that did not exist. Enrolment now falls back to
searching by attested SURFACE FORM (`morph.CaseVariants` over `sentence_token.form`, which is
BINARY-collated like every other index in the pack). Both paths run the same check afterwards:
the runtime analyser has to agree the token really reads as this lemma. On hu-v1 this took the
unteachable set from 5% + a long tail of false gaps down to 42 of 853 words, all genuinely
absent from Tatoeba — mostly administrative vocabulary (`adóbevallás`, `tértivevény`,
`önéletrajz`). `duplicate` counts words that
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
| `POST` | `/api/hvpt/answer` | ✅ | `{trialId, chosen, rtMs}` → reveals `played` **plus `audioA`/`audioB`**, both members' clips from the trial's own talker, so the UI can play the two words back to back — the comparison is the corrective feedback, and it exists only after the answer (ADR-008 holds before it). Updates the contrast's FSRS + accuracy. Replay of an id is 410 |
| `GET` | `/api/hvpt/contrasts` | ✅ | per-contrast state; **no contrast is ever complete** [R7] |
| `POST` | `/api/listen/material` | ✅ | multipart `transcript` + `title`, and **`audio` is optional**; the transcript is ingested as a normal text (coverage pre-check for free) and the response carries the verdict |
| `GET` | `/api/listen/:id/state` | ✅ | the three-pass position [R13] |
| `POST` | `/api/listen/:id/progress` | ✅ | `{pass, positionS, secondsHeard}` — seconds are clamped to 60 per report: **hours accumulate, they cannot be claimed**. Feeds the word-order listening gate [R15] and the daily ledger |

HVPT trials never carry the played member — it is held server-side like a drill's expected
answer (ADR-008 applies). The audio URL, the talker id and the rate are in the payload, because
talker variability is the mechanism and must be auditable on screen.

**A listening material does not need an audio file.** Requiring one meant the loop could not
start until the learner had found a recording *and* a matching transcript, which nobody does, so
on a fresh install the screen was an empty list above an upload form. With no file the material
is stored with an empty `audio_path` and the learner's own browser reads the transcript aloud,
sentence by sentence. That is worse input than a native recording and the UI labels it as such
(`listen.deviceVoiceCaveat`); it is enormously better than silence. `positionS` is reported as
`0` for such a material — there is no timeline to store, and a sentence index is not seconds.

## Progress

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/stats/overview` | ✅ | lemma counts + cumulative curve, streak, today's ledger, listening hours, checkpoint history + the next spec with live evidence, blocked levels, exam date. Also: `queue` — the whole deck's `Breakdown`, the SAME numbers the review tabs show ([ADR-021](../.claude/DECISIONS.md#adr-021); `dueNow` duplicates `queue.dueNow` for older clients); `days` — the ledger, newest first, `?days=7\|30\|90\|365`, **default 90** — the chart offers 7/30/90 and switches between them client-side, so the response has to carry the widest of them; anything unrecognised gets the default rather than an error. The STREAK is always computed over the full ledger, never over this window; each row carries `readS` beside `listenS` so the activity chart can say where an hour went ([ADR-044](../.claude/DECISIONS.md#adr-044)); `practiceDays` + `practiceByDirection` — practice series, still scheduling nothing |
| `GET` | `/api/stats/mastery` | ✅ | per-feature mastery; each row adds `topics: [code]` — the grammar topics that teach the feature (`[]` if none), so a heatmap cell can link to its lesson |
| `GET` | `/api/stats/level` | ✅ | the **estimated level** and text coverage ([ADR-047](../.claude/DECISIONS.md#adr-047)), computed per request, never stored: `{label:"A2.2", band, sublevel, confidence:"low"\|"estimate", coverage:0.71, countedLemmas, bands:[{band,counted,total,share,target:0.8,grammarReady,grammarDrillable,reached}], cappedBy:"m3"\|"", methodKey:"level.method.v1"}`. See ARCHITECTURE §5.12 for the model. `label` is `C1+` once every measured band is reached |
| `GET` | `/api/stats/deck` | ✅ | the learner's WORDS (lemmas) by stage, the Words list's own rule (`known` status outranks; else the strongest card's stage): `{total, stages:{learning,relearning,young,mature,known}, new}`. `new` = in the deck with no reviewed card yet (the list shows these as learning, «ещё не повторялось») and is NOT also counted in `stages.learning`, which means "answered at least once and in its learning steps". `total` = sum of all six = the Words list total for `status=all` (an ignored word with no card has no stage and is in neither). Trashed/suspended cards excluded. A word count, never a due count (those come from `srs.Breakdowns`). Cached 30 s with the level scan, dropped by `recordInteraction` |
| `GET` | `/api/stats/level/band/:band` | ✅ | `?limit=50` (1–200) → `{words:[{lemmaId,lemma,glossRu,glossEn,freqRank}]}`: the highest-frequency lemmas of band `A1`–`C1` the learner does not yet count. Homonyms are collapsed by lemma text (case-insensitive): the best-ranked row, preferring one with a gloss in `?lang=` (default the UI locale); `limit` counts distinct lemmas. Read-only — adds no note. 400 on an unknown band |
| `POST` | `/api/checkpoints` | ✅ | `{code, attested[], notes}` → verdict. Measured criteria the server computes; attested criteria are the learner's claims, stored as such. A failure returns the diagnostic against the three documented failure modes and **locks curriculum levels** at/above the checkpoint's gate (`gate.locked.checkpoint`) until a retake passes |
| `POST` | `/api/today/human` | ✅ | Loop 4 attestation. Recorded, shown, **ignored by the streak** — the app cannot verify a conversation and does not pretend to |

The streak in `/api/stats/overview` counts **obligations met** — due queue cleared AND the
stated daily minutes reached — never activity. A day with no work breaks it; that is design,
not accident ([R14], and missed days are failure mode №1 in the source plan). "Queue cleared"
means the ACTIONABLE count: `daily_log.due_left` stores `srs.ActionableDue` — post-burial, what
a queue built now would serve — because the raw card count it used to store included siblings
buried until tomorrow, which made the streak literally unwinnable for any learner whose words
carry more than one due card. Learning steps maturing later today deliberately do not block the
day: a 10-minute step at 23:55 is not a debt ([ADR-021](../.claude/DECISIONS.md#adr-021)).

> **There is no export or backup API.** `/api/export/apkg`, `/api/export/csv`,
> `/api/export/backup` and `/api/import/backup` were removed along with `internal/export`; no
> route, package or UI for them exists. The learner's data is one self-contained directory, so
> backup is a filesystem operation and restore is putting the directory back — see
> [SELFHOSTING.md](SELFHOSTING.md#backup-and-restore). Do not re-document these endpoints
> without re-implementing them.

## Profile export & import

Added by [ADR-040](../.claude/DECISIONS.md#adr-040), which supersedes ADR-011's *"the app has no
export"* in the shape ADR-011 itself named as honest. **Not** an `.apkg`: that refusal stands.

| Method | Path | Status | Notes |
| --- | --- | --- | --- |
| `GET` | `/api/profile/export` | ✅ | streams `tanulom-profile-<date>.zip`: `profile.json` (every user-scoped table — both FSRS state spaces, the review log, texts and their tokens, glosses, the daily ledger, tutor threads) plus `media/` for the learner's own audio. Columns are **discovered** via `PRAGMA table_info`, so a migration cannot silently drop one. `settings` rows with `secret = 1` are **omitted**, and the screen says so. Acting user only — there is no route that reads another account |
| `GET` | `/api/profile/size` | ✅ | `{notes, cards, reviews, texts, words, threads, days}` — what an import is about to replace, in the learner's own units, for the confirm dialog |
| `POST` | `/api/profile/import` | ✅ | multipart `profile=<zip>` → the report: `{tables, totalRows, mediaFiles, skippedColumns?, replayOk, replayChecked, replayWarning?, secretsOmitted}`. **REPLACES**, in one transaction, for the acting user only. 400 on a `formatVersion` newer than this binary's, on an archive that is not a profile, and on a failed integrity check. That check is the **replay property**: every card's log is replayed through `internal/srs/replay.go` and compared to the state it arrived with — a mismatch on the same architecture is corruption and rolls back, while one across two architectures is a *warning* naming the fuzz/FMA reason (ARCHITECTURE.md §5.7). Surrogate ids are re-assigned and every reference rewritten |

## Reading time

| Method | Path | Status | Notes |
| --- | --- | --- | --- |
| `POST` | `/api/reading/progress` | ✅ | `{seconds}` → `{seconds, activeMs}` — what was actually credited, echoed because the clamp means the two can differ. Clamped to 60 s per report, the same bound listening has. The reader was the one loop that reported nothing, so an hour with a text left the day's minutes unmoved and the streak unmet ([ADR-044](../.claude/DECISIONS.md#adr-044)). Zero is accepted and credits nothing — an idle tick is the normal case |

## A dictionary entry from the model

[ADR-042](../.claude/DECISIONS.md#adr-042). Deliberately **not** a tutor route: the tutor mints
nothing and a test counts the tables to prove it.

| Method | Path | Status | Notes |
| --- | --- | --- | --- |
| `GET` | `/api/words/ai-entry/status` | ✅ | `{available, reason?, model?}` — asked BEFORE the button is offered, so an unavailable model is a missing button and not a failed press |
| `POST` | `/api/words/:form/entry` | ✅ | `?lang=` → `{form, model, unknown?, lemma?, pos?, meanings?, morphemes?, examples?, raw?, unattested}`. Offered only where the pack has nothing. `unattested` lists the Hungarian the dictionary has never seen — every token is checked, **including the ones the model forgot to backtick**, and the asked-about form is excluded. `raw` carries the model's text when it did not answer in the schema, shown as prose rather than discarded. `unknown` is the model's own admission, passed through. **Writes nothing**: the meaning reaches `user_gloss` only via `PUT /api/glosses/:form` with `source: "machine"`, and a card's back only after «Принять как своё» re-saves it `manual`. 503 with the tutor's own key when no model is configured; 502 `errors.llmFailed` when the call fails |

## The grammar tutor

A thread is a conversation with an optional pinned topic ([ADR-029](../.claude/DECISIONS.md#adr-029)).

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/tutor` | ✅ | `?kind=chat\|review\|word` (absent = every kind) → `{threads: [{id, title, topic?, kind?, noteId?, titleRu?, titleEn?, lastTopic?, lastTopicRu?, lastTopicEn?, messages, lastRole, lastContent, updatedAt}], counts: {chat?, review?, word?}}`, most recently active first. `counts` always covers EVERY kind regardless of the filter — a tab strip has to carry the number on the tab you are not looking at. `chat` is the wire name for the ordinary kind, whose stored value is `''` and which a query string cannot carry ([ADR-045](../.claude/DECISIONS.md#adr-045)) |
| `DELETE` | `/api/tutor` | ✅ | `?kind=` clears one tab, absent clears every chat → `{deleted}`. An unrecognised kind is **400**, never silently "all". A review chat is one per word per local day, so a month of reviewing leaves hundreds and one-at-a-time was the only way out ([ADR-045](../.claude/DECISIONS.md#adr-045)) |
| `POST` | `/api/tutor` | ✅ | `?lang=` · `{topic?, kind?}` → **201** with the thread response; 404 on an unknown topic. `kind: "review"` with `note` asks for the review chat about that WORD for the local day: **200** with it when it exists, 201 when created and named by the word ([ADR-034](../.claude/DECISIONS.md#adr-034)). `kind: "word"` with `word` does the same keyed on a surface form, lowercased server-side the way `user_gloss` keys it ([ADR-042](../.claude/DECISIONS.md#adr-042)); 400 without a word. 400 on any other kind, 404 on a note that is not the learner's |
| `GET` | `/api/tutor/review` | ✅ | `?lang=&note=` → that word's chat for the local day with its messages, or the thread response with `thread: null` — never created by asking. Without `note`, the day's newest review chat |
| `GET` | `/api/tutor/word/:form` | ✅ | `?lang=` → the day's chat about that FORM with its messages, or `thread: null` — never created by asking. Keyed on the form rather than a note, because the words worth asking about are often the ones the pack cannot resolve at all ([ADR-042](../.claude/DECISIONS.md#adr-042)) |
| `GET` | `/api/tutor/new` | ✅ | `?lang=` → the thread response with `thread: null` — availability and sources for a chat not started yet |
| `GET` | `/api/tutor/topic/:code` | ✅ | `?lang=` → the latest thread pinned to a lesson, or `thread: null` with `topic` set; 404 on an unknown topic |
| `GET` | `/api/tutor/:id` | ✅ | `?lang=ru\|en` → `{available, reason?, model?, lang, thread: {id, title, topic?, kind?, noteId?, titleRu?, titleEn?, createdAt, updatedAt} \| null, topic?, messages, sources}`. 404 `errors.tutorNoThread` (another learner's, or none). `available: false` with `reason: "errors.llmOff"` when `services.llm` is off, or `"errors.llmNoModel"` when it is on but names no model — the panel says which and links to Settings |
| `POST` | `/api/tutor/:id` | ✅ | `{question?, lang?, replace?, card?}` → **`text/event-stream`**: `event: asked` `{id, question}` first (the stored question's id and text — composed by the server when `card.template` is set, so the client can show it and offer an edit of it), then `event: delta` `{text}` per fragment, then `event: done` `{message}` whenever any words arrived — `message.interrupted` when the answer was cut by the budget, the timeout, a stop or a lost connection; `event: error` `{error}` only when nothing arrived (`errors.tutorFailed`, or `errors.tutorStopped` after a stop). `replace` names a question of the learner's own to edit: it and everything after it are deleted first; 404 `errors.tutorNoMessage` for any other turn ([ADR-031](../.claude/DECISIONS.md#adr-031)). **`card`** `{cardId, phase: "question"\|"answer", template?, typed?}` makes the question about a review card ([ADR-034](../.claude/DECISIONS.md#adr-034)): a `template` (`breakdown`, `whyForm`, `whyNotMine`, `translateWord`, `spelling`, `explainTask`; before the answer `hintForm`, `hintWord`, `hintRule`, `explainTask`) has the server compose the question and the answer's shape — `question` may then be blank — and 400 refuses a template outside its phase; 404 `errors.tutorNoCard` for a card that is not the learner's or is in the trash; in the `question` phase the prompt withholds the answer. 400 blank or over 1,000 **characters**; 404 no such thread; 503 no model or no corpus; **409** while the learner's previous question is still streaming |
| `POST` | `/api/tutor/:id/stop` | ✅ | ends the learner's in-flight answer on this thread → **204**, idempotent; the stream then delivers the partial as `done`. 404 on another learner's thread |
| `DELETE` | `/api/tutor/:id` | ✅ | deletes the thread and its turns → 204 |

A `message` is `{id, role, content, lang, model, promptVersion, topic?, topicTitle?, unattested,
interrupted, cutReason?, promptTokens?, answerTokens?, contextTokens?, cardId?, cardPhase?,
template?, createdAt}`; a thread header carries `kind` (`review` for a review chat, whose
`title` is the word it is about and `noteId` the note that word came from — chats from before
migration `00019` carry neither and the client names them by their date). **`model` is not
decoration** — it is the label the honesty rules
require on every machine answer ([ADR-014](../.claude/DECISIONS.md#adr-014),
[ADR-027](../.claude/DECISIONS.md#adr-027)). **`topic`** on an assistant turn is the lesson the
answer was grounded in — the thread's pin, or the one inferred from the question — and the
panel prints it; absent means the answer came from the reference alone, and the panel says
that. **`unattested`** lists the backticked Hungarian words the pack's dictionary does not carry,
and the panel prints them in the uncertainty hue — a backticked *sentence* is checked word by
word, so a translation ([ADR-030](../.claude/DECISIONS.md#adr-030)) is covered. **`interrupted`**
is set when the text ended before the answer did: the stream timeout, a stop, a lost connection,
or the answer budget (`finish_reason: length`, surfaced as `llm.ErrTruncated`); **`cutReason`**
says which — `budget`, `timeout`, `stop`, `connection`, `echo` (the model began reciting the prompt's own headings, `tutor.EchoAt`) — and the panel prints that under the
answer rather than guessing at two causes. **`promptTokens` / `answerTokens`** are the server's own usage figures for the
answer and **`contextTokens`** the window the model ran with (Ollama's `/api/ps`); absent when
not reported — the context meter under the composer prints them, and says "not reported" rather
than 0 ([ADR-031](../.claude/DECISIONS.md#adr-031)). **Nothing on these routes reaches a note, a
card, a gloss or either state space** — pinned by `handlers_tutor_test.go`. A thread keeps its
last 200 messages. The answer language is the client's rendered locale, falling back to
`locale.ui`. Full design: [`TUTOR.md`](TUTOR.md).

## Speech & LLM *(v1.1)*

| Method | Path | Status | Notes |
|---|---|---|---|
| `POST` | `/api/speech/tts` | ⏳ P8 | text → audio; content-addressed cache |
| `POST` | `/api/speech/asr` | ⏳ P8 | audio → transcript + confidence |
| `POST` | `/api/speech/score` | ⏳ P8 | target + audio → **explicit segmental feedback** + enqueued HVPT contrasts |
| `POST` | `/api/llm/generate` | ⏳ P9 | comprehensible input; returns `attested` per span |
| `POST` | `/api/llm/correct` | ⏳ P9 | staged correction — **server releases stage 3 only after an attempt** |
| `POST` | `/api/llm/roleplay` | ⏳ P9 | scenario turn, optionally voice in/out |

The grammar tutor is **not** on this list either: it shipped as `/api/tutor/*` above,
against the same `llm` slot, and explains rather than generates — see
[ADR-027](../.claude/DECISIONS.md#adr-027).

Sentence translation is **not** on this list: it shipped as `POST /api/translate` above, against a
dedicated translation model rather than the `llm` slot. See
[ADR-014](../.claude/DECISIONS.md#adr-014) for why those are different tools.

## Static

| Path | Serves |
|---|---|
| `/media/*` | cached TTS, HVPT clips, learner recordings |
| `/*` | SPA catch-all from the embedded FS |
