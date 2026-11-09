# Findings: install, sidecars, HuSpaCy (researcher, 2026-11-08; no shell, nothing run)

## Install
- Fresh clone runs with no download: binary embeds starter pack; lang.EnsurePack installs it on first boot (backend/internal/lang/starter.go:59-99; server.go:178). Full pack = optional content upgrade, no feature depends on it.
- README Quick start (README.md:111-117) pulls ghcr.io/antlko/learn-hungarian-ai but README.md:163 says it exists only after a v* tag; no local tag -> expect denied. docker-compose.yml:26 pulls the image, `build: .` commented out (:29).
- Working paths: `docker build -t tanulom . && docker run -d -p 8012:8012 -v tanulom-data:/data tanulom` (README.md:219-223); source: `make build && ./bin/tanulom --data-dir ./data` (Makefile:108-120). `make deps` only for pipeline Python.
- Full pack adds (docs/LANGPACK.md:417-426): lemmas 5,132->35,311; wordforms 502,480->1,980,147; sentences 12,000->60,000; RU glosses 2,290->8,620. Curriculum/course/minimal pairs/audio identical. README.md:120-122 numbers (5,059/517,837/11,400) disagree with LANGPACK.md -> reconcile.
- HuSpaCy is NOT used by the pipeline (no analyse.py, no spaCy import; build.py:365-375 links tokens by dictionary, empty feats). langpack/requirements-build.txt unused. LANGPACK.md:208,331-332 and Makefile:160 ("hours") describe a nonexistent stage.
- The ~40 min is likely `make subtitles` (LANGPACK.md:273), full build downloads ~2 GB OpenSubtitles + 580 MB Wiktionary (pipeline/sources.py:81-131). Not measured.
- No prebuilt-pack download exists (no fetch code in backend/internal/lang). LANGPACK.md:343-347 describes download/BAKE_LANGPACK modes that do not exist. Hosting a full pack with OpenSubtitles conflicts with langpack/LICENSES.md:31-42; option: publish pack built with `--subtitle-sentences 0` (Tatoeba+Wiktionary only).
- A `make langpack` output named langpack-hu-v1.sqlite dropped in the data volume works today with a restart (lang.Detect lang.go:37-65).

## Sidecars today
- mt: works if deployed (services/mt/app.py NLLB-600M CTranslate2; /health /warmup /translate). Go: handlers_translate.go:101, handlers_mt.go:64-102, route server.go:446. Settings progress bar SettingsView.vue:570-650 (3 s polling :216-217). This is the template for "install with progress".
- nlp: STUB. Config config.go:243 default off (363-367); only probed (handlers_capabilities.go:51)/serialised (respond.go:111); no Go code calls /analyze; services/nlp has only README; compose profile fails (no Dockerfile, docker-compose.yml:49-52). Row shows "connected" if anything answers :5002/health (SettingsView.vue:181).
- tts/asr: config rows only, no call sites (only audioAvailable() handlers_review.go:658-668). Real audio = browser speech (ADR-012).
- "not implemented" string not in web-ui; likely in README.md:255-257, docker-compose.yml, services/README.md. Dead-end copy handlers_analyze.go:50 "download one from Settings → Data" (no such screen; CLAUDE.md forbids).
- docs/CONFIGURATION.md:98 example `nlp: provider: local` but default off.

## HuSpaCy
- Planned contract (services/README.md:27-32): HTTP :5002, /health, /analyze.
- Published figures (huspacy.github.io/models, 2024-10-28): md 127 MB, 2.4 GB RAM, 4474 tok/s CPU, lemma 97.41 POS 96.85 morph 94.32 LAS 74.25; lg 402 MB, 3.3 GB RAM, 1410 tok/s, lemma 97.62 POS 96.60 morph 93.41 LAS 78.12; trf 1.27 GB GPU; no sm. md = smallest viable, ~equal to lg. HF md v3.8.1 requires spaCy >=3.8.0,<3.9.0; CC BY-SA 4.0. spaCy 3.8.x needs Python >=3.10. Container size/RAM unverified (plan >=2.4 GB).
- No Go dependency on it. Value: context disambiguation only (ADR-002 keeps runtime morphology Python-free; FST port rejected DECISIONS.md:53).

## Install-from-UI
- No pack/sidecar install endpoint exists. Only /mt/warmup installs (admin-only).
- In-app pack install feasible in pure Go: stream to /data/langpack-hu-v1.sqlite.part, SHA-256, rename. Needs hosted asset + pinned checksum. Live swap infeasible (loadPack once at boot server.go:121,172-193) -> UI says "restart", process exits, `restart: unless-stopped` relaunches.
- Sidecars cannot be started from distroless; docker socket = host root, reject. Piper/CTranslate2/spaCy cannot run in-process with CGO_ENABLED=0. Use compose profiles + UI command + probe, like mt.
- Config: services.{tts,asr,nlp,llm,mt} (config.go:240-257), only via updateConfig.
