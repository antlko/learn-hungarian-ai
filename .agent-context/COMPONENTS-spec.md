# COMPONENTS-spec — install optional services (MT, HuSpaCy) from the app, in ONE container

Owner request (Settings → Components screenshots): "How will it download, where to? I want everything in one container:
press Install here, it downloads, then (if needed) asks to restart. What will it install? Will Settings → Services update
automatically? What if the ports are taken on my server, or are they container-local? What is `--profile`? I will install
everything with docker compose anyway — can the MAIN BINARY run the python services itself (e.g. `mt`) once downloaded?
Check if the install can work this way. Keep docker / docker compose as TABS for choosing the install method."

## Feasibility (orchestrator's reading of the repo)
- services/mt (NLLB-200 600M, ctranslate2 + transformers, torch only for the one-time conversion) and services/nlp (spaCy 3.8 +
  hu_core_news_md wheel downloaded on /warmup) are plain Python/uvicorn apps: `python app.py`, env-configured
  (MT_MODEL_DIR, MT_PORT, NLP_MODEL_DIR, NLP_PORT), `/health` + `/warmup` + work endpoints. They hard-code host 0.0.0.0.
- The release image is `gcr.io/distroless/static` (no shell, no python), so the Go binary cannot run them THERE. A native run
  (`make dev`/`make run`, user's laptop) can, using the system python3 (this is exactly what `make mt` does).
- => Design: a **managed component** feature in the Go binary: embed the two service sources (go:embed), and on an admin's click
  create a venv under `<data>/components/<id>/`, `pip install` the pinned requirements, prepare the model, then run
  `python app.py` as a supervised CHILD PROCESS bound to 127.0.0.1 on a free ephemeral port; point the app at it
  automatically. Needs python3≥3.10 with venv+pip and outbound network. In Docker this requires a **`-full` image variant**
  (debian-slim + python3 + venv + ca-certificates; same binary, same non-root user, /data volume); the default distroless
  image reports "no Python in this build — use the full image or Docker Compose".
- LLM (Ollama) is NOT embedded (multi-GB, GPU libs): it stays external (own server) or compose.
- tts/asr: not built (unchanged).

## User-visible model: install METHOD TABS per component
`Встроенный (в этом контейнере/на этой машине)` | `Docker Compose` | `Свой сервер (адрес)`.
- Встроенный: states absent → installing (steps with progress + live log tail) → installed/starting → running (shows «порт 41873, только внутри»), buttons Установить / Запустить / Остановить / Удалить (frees disk; asks confirm). No restart of the app is needed (the app starts the child itself); say so. If python is missing: disabled with the reason and a pointer to the other tabs.
- Docker Compose: copyable `docker compose --profile mt up -d` + a one-line InfoTip explaining «profile = необязательная служба в compose-файле, которая не стартует, пока вы её не запросите»; note that in compose the services reach each other by service name and ports need NOT be published to the host (default compose does not publish them) so host port clashes are impossible unless the user publishes them.
- Свой сервер: endpoint field (existing Services row) for a service the learner already runs elsewhere.
- Settings → Services rows update AUTOMATICALLY: choosing/finishing Встроенный sets `services.<id>.provider = "managed"` (via updateConfig); the Services row then shows «встроенный · порт N» read-only instead of an endpoint field. Docker/own-server tabs keep the endpoint field.

## Backend contract (M1)
Config: `services.mt.provider` and `services.nlp.provider` accept the new value `managed` (validate; `openai` stays refused for mt/nlp). Endpoint for `managed` is NEVER stored: it is resolved at runtime by the manager (`http://127.0.0.1:<port>`). All existing clients (internal/mt, internal/nlp, capability probes, components status) resolve endpoints through ONE function that knows about managed ports.

`GET /api/admin/components` (existing list) — each of `mt` and `nlp` gains:
```
managed: {
  available: bool,            // python3>=3.10 with venv+pip found, data dir writable
  unavailableKey?: string,    // errors.componentNoPython | errors.componentReadOnly
  pythonVersion?: string,
  state: "absent"|"installing"|"installed"|"starting"|"running"|"failed"|"stopped",
  step?: string,              // i18n key: component.step.venv|pip|model|start
  progress?: number,          // 0..1 overall, monotonic within a job
  port?: number,              // 127.0.0.1 only
  errorKey?: string, errorDetail?: string,   // detail = short tail of the failing command (no secrets)
  log?: string[],             // last ≤40 lines of the install/run log (redacted)
  diskNeed: string, ramNeed: string          // human strings from constants (documented estimates)
}
```
Endpoints (admin-only; 202 on start; 409 `errors.componentBusy` if a job runs; 404 unknown id; ids: mt|nlp only):
`POST /api/admin/components/:id/install` · `/start` · `/stop` · `/uninstall`.
Rules: single-flight per component; install steps = (1) extract embedded source to `<data>/components/<id>/src` (sha-verified against the embedded copy on every start; never run anything else), (2) `python3 -m venv <data>/components/<id>/venv`, (3) `pip install --no-input --disable-pip-version-check -r requirements.txt` into the venv with progress parsed from pip output (coarse is fine), (4) model prep: MT = `python -c "import app; app.prepare_model()"` with torch installed from the CPU index first and uninstalled afterwards to reclaim ~800 MB; NLP = POST /warmup after start (progress from /health), (5) set provider=managed via updateConfig, (6) start. Preflight: free disk ≥ (mt 6 GB, nlp 1.5 GB) else `errors.diskFull`; outbound network error → `errors.componentNetwork`. Cancel on shutdown. Atomic: installs into `<id>.installing/` and renames on success; a failed install leaves nothing half-installed.
Process supervision: `exec.Command(venv/bin/python, src/app.py)` with a CLEAN env (PATH, HOME=<data>/components/<id>/home, LANG, plus MT_*/NLP_* settings: HOST=127.0.0.1, PORT=<free port>, MODEL_DIR=<data>/components/<id>/models) — never inherit TANULOM_* or any secret; own process group; stdout/stderr → ring buffer + rotating log file (cap size); restart with exponential backoff (max 5 tries / 10 min then `failed`); SIGTERM then SIGKILL after 5 s on stop/shutdown; autostart at boot when provider=managed and installed. `app.py` files read `MT_HOST`/`NLP_HOST` (default 0.0.0.0 so compose is unchanged).
Security: admin-only; embedded sources only; pinned requirement versions; no user-supplied URLs/args; model download URLs fixed in the sources; the log is redacted of paths outside data dir. Tests with a fake python (a shell script standing in for venv/pip/python) — no network in tests.
Docs: SELFHOSTING (methods, requirements, disk/RAM, full image, no host ports), CONFIGURATION (`provider: managed`), API.md, ARCHITECTURE (manager), CLAUDE.md invariants (the app stays fully usable with components absent; managed children never block boot; the distroless image has none), new ADR (next free) "Managed components: embedded python sidecars supervised by the binary".

## Image (M2)
Dockerfile: new final stage `full` = `python:3.12-slim-bookworm` (or debian-slim + python3, python3-venv, ca-certificates), non-root uid 65532, same binary/ENV/VOLUME/ENTRYPOINT; default final stage stays distroless (so `docker build .` behaviour is unchanged; select with `--target full`). docker-compose.yml: commented alternative `tanulom: build: {context: ., target: full}`; CI workflow publishes `:latest` (distroless) and `:latest-full` tags (only edit the workflow if it exists and the change is obviously safe; never run it).

## UI (M3) — see brief.
