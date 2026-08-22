# Development

A Go backend (`backend/`) serving a Vue 3 SPA (`web-ui/`). In production the built SPA is
embedded into the binary; in development the two run side by side with Vite proxying to Go.

## Prerequisites

- **Go** 1.26+
- **Node** 22+ (24 recommended — see `web-ui/package.json` `engines`)
- **Python** 3.11+ *only if* you rebuild the language pack
- **Docker** for image builds

## Two toolchains, two directories

Run `npm` from `web-ui/`. Run `go` from `backend/`. They are separate projects that ship as one
binary.

## Running locally without Docker

The `Makefile` at the repo root drives both toolchains together. **Docker is not needed for any
of it** — only for building the published image. `make` on its own prints every target:

```bash
git clone https://github.com/antlko/tanulom.git
cd tanulom
make deps    # go mod download + npm ci + the pipeline's Python requirements
make dev     # backend and Vite together, in the foreground
```

| Target | What it does |
|---|---|
| `make dev` | Runs the Go API **and** the Vite dev server in one terminal. Ctrl-C stops both. |
| `make backend` / `make frontend` | Either half alone, when you want two terminals. |
| `make spa` | Builds the SPA and embeds it into `backend/internal/web/dist`. |
| `make build` | `spa`, then the static `CGO_ENABLED=0` binary into `bin/tanulom`. |
| `make run` | `build`, then serves UI + API from that one binary against `./data`. |
| `make test` | Everything CI gates on: Go vet/build/test, then type-check, lint, SPA build. |
| `make test-backend` / `make test-frontend` | Either half. |
| `make lint` / `make fmt` | gofmt + `go vet` + oxlint/eslint; or rewrite in place. |
| `make langpack` | Rebuilds `data/langpack-hu-v1.sqlite` (see the caveats below). |
| `make clean` | Removes build output. **Never touches `./data`.** |

Overridable: `DATA_DIR`, `PACK`, `PYTHON`, `VERSION`, `DEV_ADDR`, `GO_TEST_FLAGS` — e.g.
`make test GO_TEST_FLAGS='-race -count=1'` to match CI exactly, or
`make deps PYTHON=langpack/.venv/bin/python` to keep pip out of the system interpreter.

Open **http://localhost:5173** for `make dev`. Vite proxies `/api` and `/media` to the backend
(see `web-ui/vite.config.ts`), so cookies and requests work end to end. Both sides hot-reload.

Two things the Makefile knows that are easy to get wrong by hand:

- **`make dev` pins the backend to `:8012`** (`DEV_ADDR`), because the Vite proxy target is
  written literally in `vite.config.ts` and the two have to agree. It happens to equal
  `config.DefaultAddr` today, but it is not read from it — change one and you must change the
  other, or every `/api` call from the dev SPA 502s with no clue why. `make run` passes no
  `--addr` at all, so the single binary uses the configured default.
- **`//go:embed all:dist` does not compile when `backend/internal/web/dist` is absent**, which is
  every fresh clone. Any Go target writes a placeholder there first, for the same reason
  `ci.yml` has a "Stub embedded SPA" step. It never overwrites a real `make spa` build.

## Doing it by hand

The Makefile is a convenience, not a dependency. The underlying commands:

```bash
# Two dev servers, two terminals
cd backend  && go run ./cmd/tanulom --data-dir ../data --addr :8012
cd web-ui   && npm install && npm run dev

# The single binary
cd web-ui && npm ci && npm run build && cd ..
rm -rf backend/internal/web/dist && cp -r web-ui/dist backend/internal/web/dist
cd backend && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../bin/tanulom ./cmd/tanulom
cd .. && ./bin/tanulom --data-dir ./data
```

The `Dockerfile` automates exactly these steps across build stages.

## Quality gates

**Frontend** (from `web-ui/`):

```bash
npm run type-check   # vue-tsc, no emit
npm run lint         # oxlint → eslint, both --fix
npm run build        # type-check + production build
```

**Backend** (from `backend/`):

```bash
go vet ./...
go test ./...
go build ./...
```

Both run in `.github/workflows/ci.yml` on every PR, and `make test` runs the same set locally.

## Language pack

You do **not** need to build the pack to develop, and there is nothing to fetch: the binary
installs its embedded starter pack into the data directory on first boot. It is a real pack —
5,132 lemmas, the whole 24-unit course, every minimal pair — so every screen works against it. To rebuild — see [`LANGPACK.md`](LANGPACK.md) — expect hours and tens
of GB of scratch space; it runs in CI on tag, not on a laptop.

## Optional sidecars

```bash
docker compose --profile nlp --profile speech up -d
```

The app probes them at startup and via `GET /api/capabilities`. **Develop with them stopped at
least some of the time** — the invariant that the core works alone is easy to break by accident
and hard to notice.

## Testing conventions

- Backend uses stdlib `testing`: table-driven, `httptest` for handlers, a temp-file SQLite per
  test. Add tests beside the package.
- **Golden tests** for morphology live in `internal/morph/testdata/` against a held-out UD
  Hungarian sample. These are the product's quality floor — if they regress, `n+1` and the
  heatmap are wrong.
- **Real-pack tests** (`*_realpack_test.go` in `internal/drill` and `internal/morph`) run the
  generators and the segmenter against the actual `data/langpack-hu-v1.sqlite` when present and
  skip cleanly in CI. Point them elsewhere with `TANULOM_TEST_PACK=/path/to/pack.sqlite`.
- **Pipeline tests** are plain functions under `langpack/tests/`; run them with `pytest` if you
  have it, or import-and-call (the repo's Python needs no extra deps for these).
- **Property test:** replaying a `review_log` through FSRS must reproduce stored card state
  exactly. If this fails, review history is not trustworthy.
- **No frontend test framework is wired.** Verify UI changes by running the dev servers. Do not
  invent an `npm test` command.

## Conventions that bite

- **Icons: `@lucide/vue`**, not `lucide-vue-next` (deprecated). Some names changed in v1
  (`Home`→`House`); verify with `type-check`.
- **Tailwind v4 resets buttons to `cursor: default`.** Reuse `buttonVariants`, `TabsTrigger`,
  `SelectTrigger`, `DropdownMenuItem` rather than hand-rolling a bare `<button>`.
- **shadcn-vue primitives in `src/components/ui` are single-word by design** — `eslint.config.ts`
  disables `vue/multi-word-component-names` for that directory only.
- **Migrations:** add `internal/db/migrations/NNNNN_name.sql` with `-- +goose Up` / `-- +goose
  Down`, dialect `sqlite3`. **Never edit an applied migration.** They are embedded and run inside
  `db.Open`; there is no migrate step.
- **`config.yaml` is mutated only through `Server.updateConfig`.** New `Config` fields must also
  be added to `config/clone.go`.
- **Frontend types mirror `dto.go`** (camelCase). Change both together.
- **Fiber handlers** return `fiber.NewError(code, msg)`; the central `errorHandler` renders
  `{"error": msg}` — the shape the frontend expects. Success via `c.JSON`.
- **Both locales, always.** A PR adding an `en` key without `ru` fails review.
- **Pedagogical code cites its note:**
  `// [R5] blocked-first ... see .claude/research/05-interleaving.md`
- **Behaviour change → doc change in the same commit.** `docs/` is the contract.
